package handler

import (
	"net/http"
	"sync"

	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/netspeed/loglib"
	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/netspeed/model"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  65536,
	WriteBufferSize: 65536,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var connCounter int

type WSHandler struct {
	connections map[*websocket.Conn]*SpeedTester
	mu          sync.Mutex
}

func NewWSHandler() *WSHandler {
	return &WSHandler{
		connections: make(map[*websocket.Conn]*SpeedTester),
	}
}

func (h *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	connCounter++
	connID := connCounter
	remoteAddr := r.RemoteAddr
	loglib.Info("[conn-%d] New WebSocket connection from %s (User-Agent: %s)", connID, remoteAddr, r.UserAgent())

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		loglib.Error("[conn-%d] WebSocket upgrade error: %v", connID, err)
		return
	}

	tester := NewSpeedTester(conn)

	h.mu.Lock()
	h.connections[conn] = tester
	h.mu.Unlock()
	loglib.Info("[conn-%d] Connection upgraded, total connections: %d", connID, len(h.connections))

	defer func() {
		h.mu.Lock()
		delete(h.connections, conn)
		h.mu.Unlock()
		tester.Stop()
		conn.Close()
		loglib.Info("[conn-%d] Connection closed, total connections: %d", connID, len(h.connections))
	}()

	readCh := make(chan []byte, 100)
	var testWg sync.WaitGroup
	msgCount := 0

	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			loglib.Warn("[conn-%d] Read error: %v (total msgs processed: %d)", connID, err, msgCount)
			tester.Stop()
			return
		}
		msgCount++

		if msgType == websocket.BinaryMessage {
			select {
			case readCh <- data:
				loglib.Debug("[conn-%d] Received binary message, buffered: %d/%d", connID, len(readCh), cap(readCh))
			default:
				loglib.Debug("[conn-%d] Binary message dropped (readCh full, size=%d)", connID, len(data))
			}
			continue
		}

		msg, err := model.ParseMessage(data)
		if err != nil {
			loglib.Warn("[conn-%d] Parse error for message #%d: %v, raw: %s", connID, msgCount, err, string(data))
			continue
		}

		loglib.Debug("[conn-%d] Received message #%d: type=%s mode=%s duration=%d", connID, msgCount, msg.Type, msg.Mode, msg.Duration)

		switch msg.Type {
		case model.MsgStart:
			loglib.Info("[conn-%d] START test: mode=%s duration=%ds", connID, msg.Mode, msg.Duration)

			tester.Stop()
			testWg.Wait()
			tester.Prepare()

			testWg.Add(1)
			go func(mode model.TestMode, duration int) {
				defer testWg.Done()

				drained := 0
			drain:
				for {
					select {
					case <-readCh:
						drained++
					default:
						break drain
					}
				}
				loglib.Debug("[conn-%d] Drained %d stale binary messages from readCh", connID, drained)
				loglib.Info("[conn-%d] Starting %s test (%ds)...", connID, mode, duration)

				switch mode {
				case model.ModeDownload:
					tester.RunDownload(duration)
				case model.ModeUpload:
					tester.RunUpload(duration, readCh)
				case model.ModeBidirectional:
					tester.RunBidirectional(duration, readCh)
				}

				loglib.Info("[conn-%d] %s test completed", connID, mode)
			}(msg.Mode, msg.Duration)

		case model.MsgStop:
			loglib.Info("[conn-%d] STOP requested by client", connID)
			tester.Stop()

		case model.MsgPing:
			tester.writeJSON(model.Message{
				Type:      model.MsgPong,
				Timestamp: msg.Timestamp,
			})
			loglib.Debug("[conn-%d] Pong response sent", connID)

		default:
			loglib.Warn("[conn-%d] Unknown message type: %s", connID, msg.Type)
		}
	}
}
