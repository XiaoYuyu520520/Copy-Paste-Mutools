package handler

import (
	"encoding/binary"
	"math/rand"
	"sync"
	"time"

	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/netspeed/loglib"
	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/netspeed/model"

	"github.com/gorilla/websocket"
)

const blockSize = 65536

var rngPool = sync.Pool{
	New: func() interface{} {
		return rand.New(rand.NewSource(time.Now().UnixNano()))
	},
}

func generateBlock() []byte {
	buf := make([]byte, blockSize)
	rng := rngPool.Get().(*rand.Rand)
	binary.LittleEndian.PutUint64(buf[:8], rng.Uint64())
	rngPool.Put(rng)
	return buf
}

type SpeedTester struct {
	conn       *websocket.Conn
	writeMu    sync.Mutex
	stopCh     chan struct{}
	startTime  time.Time
	totalBytes int64
}

func NewSpeedTester(conn *websocket.Conn) *SpeedTester {
	return &SpeedTester{
		conn:   conn,
		stopCh: make(chan struct{}),
	}
}

func (st *SpeedTester) writeJSON(v interface{}) error {
	st.writeMu.Lock()
	defer st.writeMu.Unlock()
	return st.conn.WriteJSON(v)
}

func (st *SpeedTester) writeBinary(data []byte) error {
	st.writeMu.Lock()
	defer st.writeMu.Unlock()
	return st.conn.WriteMessage(websocket.BinaryMessage, data)
}

func (st *SpeedTester) Stop() {
	select {
	case <-st.stopCh:
	default:
		close(st.stopCh)
	}
}

func (st *SpeedTester) Prepare() {
	st.stopCh = make(chan struct{})
}

func (st *SpeedTester) runProgressReporter(direction string, totalBytes *int64, startTime *time.Time, doneCh chan struct{}) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	lastBytes := int64(0)
	lastTime := *startTime
	reportCount := 0

	for {
		select {
		case <-st.stopCh:
			loglib.Debug("[progress-%s] Stopped by stopCh (reports sent: %d)", direction, reportCount)
			return
		case <-doneCh:
			loglib.Debug("[progress-%s] Stopped by doneCh (reports sent: %d)", direction, reportCount)
			return
		case <-ticker.C:
			now := time.Now()
			elapsed := now.Sub(*startTime).Seconds()
			currentBytes := *totalBytes

			if elapsed > 0 && currentBytes > 0 {
				speedMbps := float64(currentBytes-lastBytes) / now.Sub(lastTime).Seconds() / 125000.0
				totalMB := float64(currentBytes) / 1048576.0
				reportCount++

				st.writeJSON(model.Message{
					Type:      model.MsgProgress,
					Direction: direction,
					SpeedMbps: speedMbps,
					TotalMB:   totalMB,
					Elapsed:   elapsed,
				})
				loglib.Debug("[progress-%s] #%d: speed=%.2f Mbps, total=%.2f MB, elapsed=%.2fs",
					direction, reportCount, speedMbps, totalMB, elapsed)
			} else if elapsed > 0 {
				loglib.Debug("[progress-%s] #%d: no bytes yet (currentBytes=%d)", direction, reportCount, currentBytes)
			}
			lastBytes = currentBytes
			lastTime = now
		}
	}
}

func (st *SpeedTester) RunDownload(duration int) {
	block := generateBlock()
	startTime := time.Now()
	endTime := startTime.Add(time.Duration(duration) * time.Second)
	doneCh := make(chan struct{})

	loglib.Info("[download] Starting download test: duration=%ds, blockSize=%d", duration, blockSize)

	go st.runProgressReporter("down", &st.totalBytes, &startTime, doneCh)
	defer close(doneCh)

	maxSpeed := 0.0
	minSpeed := 1e9
	blockCount := 0

	windowBytes := int64(0)
	windowStart := time.Now()

	for time.Now().Before(endTime) {
		select {
		case <-st.stopCh:
			loglib.Warn("[download] Stopped via stopCh after %d blocks", blockCount)
			return
		default:
		}
		if err := st.writeBinary(block); err != nil {
			loglib.Error("[download] Write error at block %d: %v", blockCount, err)
			return
		}
		st.totalBytes += int64(len(block))
		windowBytes += int64(len(block))
		blockCount++

		windowElapsed := time.Since(windowStart).Seconds()
		if windowElapsed >= 0.2 {
			speed := float64(windowBytes) / windowElapsed / 125000.0
			if speed > maxSpeed {
				maxSpeed = speed
			}
			if speed < minSpeed && blockCount > 1 {
				minSpeed = speed
			}
			windowBytes = 0
			windowStart = time.Now()
		}
	}

	if windowBytes > 0 {
		windowElapsed := time.Since(windowStart).Seconds()
		if windowElapsed > 0 {
			speed := float64(windowBytes) / windowElapsed / 125000.0
			if speed > maxSpeed {
				maxSpeed = speed
			}
			if speed < minSpeed && blockCount > 1 {
				minSpeed = speed
			}
		}
	}

	elapsed := time.Since(startTime).Seconds()
	avgMbps := float64(st.totalBytes) / elapsed / 125000.0

	loglib.Info("[download] Test finished: blocks=%d, totalBytes=%d, elapsed=%.2fs, avg=%.2f Mbps, max=%.2f, min=%.2f",
		blockCount, st.totalBytes, elapsed, avgMbps, maxSpeed, minSpeed)

	st.writeJSON(model.Message{
		Type:        model.MsgResult,
		Direction:   "down",
		AvgMbps:     avgMbps,
		MaxMbps:     maxSpeed,
		MinMbps:     minSpeed,
		TotalBytes:  st.totalBytes,
		DurationAct: elapsed,
	})
}

func (st *SpeedTester) RunUpload(duration int, readCh chan []byte) {
	startTime := time.Now()
	endTime := startTime.Add(time.Duration(duration) * time.Second)
	doneCh := make(chan struct{})

	loglib.Info("[upload] Starting upload test: duration=%ds", duration)

	go st.runProgressReporter("up", &st.totalBytes, &startTime, doneCh)
	defer close(doneCh)

	maxSpeed := 0.0
	minSpeed := 1e9
	chunkCount := 0

	windowBytes := int64(0)
	windowStart := time.Now()

	for time.Now().Before(endTime) {
		select {
		case <-st.stopCh:
			loglib.Warn("[upload] Stopped via stopCh after %d chunks", chunkCount)
			return
		case data, ok := <-readCh:
			if !ok {
				loglib.Warn("[upload] readCh closed after %d chunks", chunkCount)
				return
			}
			st.totalBytes += int64(len(data))
			windowBytes += int64(len(data))
			chunkCount++

			windowElapsed := time.Since(windowStart).Seconds()
			if windowElapsed >= 0.2 {
				speed := float64(windowBytes) / windowElapsed / 125000.0
				if speed > maxSpeed {
					maxSpeed = speed
				}
				if speed < minSpeed && chunkCount > 1 {
					minSpeed = speed
				}
				windowBytes = 0
				windowStart = time.Now()
			}

			if chunkCount%100 == 0 {
				loglib.Debug("[upload] Received %d chunks, totalBytes=%d, speed=%.2f Mbps",
					chunkCount, st.totalBytes, float64(st.totalBytes)/time.Since(startTime).Seconds()/125000.0)
			}
		case <-time.After(100 * time.Millisecond):
			if time.Now().After(endTime) {
				goto done
			}
		}
	}
done:

	if windowBytes > 0 {
		windowElapsed := time.Since(windowStart).Seconds()
		if windowElapsed > 0 {
			speed := float64(windowBytes) / windowElapsed / 125000.0
			if speed > maxSpeed {
				maxSpeed = speed
			}
			if speed < minSpeed && chunkCount > 1 {
				minSpeed = speed
			}
		}
	}

	elapsed := time.Since(startTime).Seconds()
	avgMbps := 0.0
	if elapsed > 0 {
		avgMbps = float64(st.totalBytes) / elapsed / 125000.0
	}

	loglib.Info("[upload] Test finished: chunks=%d, totalBytes=%d, elapsed=%.2fs, avg=%.2f Mbps, max=%.2f, min=%.2f",
		chunkCount, st.totalBytes, elapsed, avgMbps, maxSpeed, minSpeed)

	st.writeJSON(model.Message{
		Type:        model.MsgResult,
		Direction:   "up",
		AvgMbps:     avgMbps,
		MaxMbps:     maxSpeed,
		MinMbps:     minSpeed,
		TotalBytes:  st.totalBytes,
		DurationAct: elapsed,
	})
}

func (st *SpeedTester) RunBidirectional(duration int, readCh chan []byte) {
	block := generateBlock()

	startTime := time.Now()
	endTime := startTime.Add(time.Duration(duration) * time.Second)
	doneCh := make(chan struct{})

	loglib.Info("[bidirectional] Starting bidirectional test: duration=%ds", duration)

	var upBytes, downBytes int64
	var upMu, downMu sync.Mutex

	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		reportCount := 0
		lastDownBytes := int64(0)
		lastUpBytes := int64(0)
		lastTime := time.Now()
		for {
			select {
			case <-st.stopCh:
				return
			case <-doneCh:
				return
			case now := <-ticker.C:
				elapsed := now.Sub(startTime).Seconds()
				downMu.Lock()
				db := downBytes
				downMu.Unlock()
				upMu.Lock()
				ub := upBytes
				upMu.Unlock()
				reportCount++
				deltaDown := db - lastDownBytes
				deltaUp := ub - lastUpBytes
				deltaTime := now.Sub(lastTime).Seconds()
				lastDownBytes = db
				lastUpBytes = ub
				lastTime = now

				if deltaTime > 0 && deltaDown > 0 {
					speed := float64(deltaDown) / deltaTime / 125000.0
					st.writeJSON(model.Message{
						Type:      model.MsgProgress,
						Direction: "down",
						SpeedMbps: speed,
						TotalMB:   float64(db) / 1048576.0,
						Elapsed:   elapsed,
					})
				}
				if deltaTime > 0 && deltaUp > 0 {
					speed := float64(deltaUp) / deltaTime / 125000.0
					st.writeJSON(model.Message{
						Type:      model.MsgProgress,
						Direction: "up",
						SpeedMbps: speed,
						TotalMB:   float64(ub) / 1048576.0,
						Elapsed:   elapsed,
					})
				}
				loglib.Debug("[bidirectional-progress] #%d elapsed=%.2fs deltaDown=%d deltaUp=%d deltaTime=%.3fs",
					reportCount, elapsed, deltaDown, deltaUp, deltaTime)
			}
		}
	}()

	writeDone := make(chan struct{})
	downBlockCount := 0
	go func() {
		for time.Now().Before(endTime) {
			select {
			case <-st.stopCh:
				close(writeDone)
				return
			default:
			}
			if err := st.writeBinary(block); err != nil {
				loglib.Error("[bidirectional] Write error at down block %d: %v", downBlockCount, err)
				close(writeDone)
				return
			}
			downMu.Lock()
			downBytes += int64(len(block))
			downMu.Unlock()
			downBlockCount++
		}
		close(writeDone)
		loglib.Debug("[bidirectional] Download writer finished: %d blocks, %d bytes", downBlockCount, downBytes)
	}()

	maxUp := 0.0
	minUp := 1e9
	upChunkCount := 0

	upWindowBytes := int64(0)
	upWindowStart := time.Now()

	for time.Now().Before(endTime) {
		select {
		case <-st.stopCh:
			goto bidone
		case data, ok := <-readCh:
			if !ok {
				goto bidone
			}
			upMu.Lock()
			upBytes += int64(len(data))
			upMu.Unlock()
			upWindowBytes += int64(len(data))
			upChunkCount++

			windowElapsed := time.Since(upWindowStart).Seconds()
			if windowElapsed >= 0.2 {
				speed := float64(upWindowBytes) / windowElapsed / 125000.0
				if speed > maxUp {
					maxUp = speed
				}
				if speed < minUp && upChunkCount > 1 {
					minUp = speed
				}
				upWindowBytes = 0
				upWindowStart = time.Now()
			}
		case <-time.After(50 * time.Millisecond):
		}
	}
	<-writeDone

	if upWindowBytes > 0 {
		windowElapsed := time.Since(upWindowStart).Seconds()
		if windowElapsed > 0 {
			speed := float64(upWindowBytes) / windowElapsed / 125000.0
			if speed > maxUp {
				maxUp = speed
			}
			if speed < minUp && upChunkCount > 1 {
				minUp = speed
			}
		}
	}

bidone:
	close(doneCh)

	elapsed := time.Since(startTime).Seconds()

	downMu.Lock()
	db := downBytes
	downMu.Unlock()
	upMu.Lock()
	ub := upBytes
	upMu.Unlock()

	loglib.Info("[bidirectional] Test finished: down_blocks=%d up_chunks=%d down_bytes=%d up_bytes=%d elapsed=%.2fs",
		downBlockCount, upChunkCount, db, ub, elapsed)

	if elapsed > 0 && db > 0 {
		downAvg := float64(db) / elapsed / 125000.0
		st.writeJSON(model.Message{
			Type:        model.MsgResult,
			Direction:   "down",
			AvgMbps:     downAvg,
			TotalBytes:  db,
			DurationAct: elapsed,
		})
		loglib.Info("[bidirectional] Result DOWN: avg=%.2f Mbps, total=%d bytes", downAvg, db)
	}
	if elapsed > 0 && ub > 0 {
		upAvg := float64(ub) / elapsed / 125000.0
		st.writeJSON(model.Message{
			Type:        model.MsgResult,
			Direction:   "up",
			AvgMbps:     upAvg,
			MaxMbps:     maxUp,
			MinMbps:     minUp,
			TotalBytes:  ub,
			DurationAct: elapsed,
		})
		loglib.Info("[bidirectional] Result UP: avg=%.2f Mbps, max=%.2f, min=%.2f, total=%d bytes",
			upAvg, maxUp, minUp, ub)
	}
}
