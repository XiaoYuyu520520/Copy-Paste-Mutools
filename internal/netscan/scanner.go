package netscan

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// DefaultProbePorts is the built-in port list used when neither the HTTP
// request nor the config file supplies one. 只探测该列表里的端口：不在列表中的
// 端口（例如 3000/5000/8000…）即使开放也不会显示，可在页面「探测端口」里补充。
var DefaultProbePorts = []int{
	21, 22, 23, 25, 53, 80, 110, 139, 143, 443, 445,
	1433, 3306, 3389, 5432, 5900, 6379, 8022, 8080, 8443,
	8888, 9000, 9090, 9200, 9999, 27017,
}

// Options controls scan behaviour.
type Options struct {
	PingTimeout    time.Duration
	TCPPortTimeout time.Duration
	TCPConcurrency int
	WorkerCount    int
	UseTCPProbe    bool
	ProbePorts     []int
	LookupMAC      bool
	LookupHost     bool
	MaxHosts       int
}

func (o Options) withDefaults() Options {
	if o.PingTimeout <= 0 {
		o.PingTimeout = 800 * time.Millisecond
	}
	if o.TCPPortTimeout <= 0 {
		o.TCPPortTimeout = 400 * time.Millisecond
	}
	if o.TCPConcurrency <= 0 {
		o.TCPConcurrency = 16
	}
	if o.WorkerCount <= 0 {
		o.WorkerCount = 64
	}
	if o.MaxHosts <= 0 {
		o.MaxHosts = 4096
	}
	if o.ProbePorts == nil {
		o.ProbePorts = DefaultProbePorts
	}
	return o
}

// Event is emitted by Scanner.Scan for the consumer to forward (e.g. SSE).
type Event struct {
	Type       string    `json:"type"`
	Scanned    int       `json:"scanned"`
	Total      int       `json:"total"`
	Alive      int       `json:"alive"`
	Host       *Host     `json:"host,omitempty"`
	Error      string    `json:"error,omitempty"`
	StartedAt  time.Time `json:"started_at,omitempty"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
	CIDR       string    `json:"cidr,omitempty"`
}

// Scanner orchestrates concurrent ping + tcp probing.
type Scanner struct {
	opts Options
}

// New returns a Scanner with the supplied options (defaults applied).
func New(opts Options) *Scanner {
	return &Scanner{opts: opts.withDefaults()}
}

// ExpandCIDR returns the list of usable host IPs in a CIDR (network and
// broadcast are excluded for prefixes <= /30).
func ExpandCIDR(cidr string) ([]string, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("解析 CIDR 失败: %w", err)
	}

	ip4 := ipnet.IP.To4()
	if ip4 == nil {
		return nil, fmt.Errorf("仅支持 IPv4 地址段")
	}

	ones, bits := ipnet.Mask.Size()
	if bits != 32 {
		return nil, fmt.Errorf("非 IPv4 掩码")
	}

	start := binary.BigEndian.Uint32(ip4.Mask(ipnet.Mask))
	end := binary.BigEndian.Uint32(ip4) | (^binary.BigEndian.Uint32(ipnet.Mask))

	if ones <= 30 {
		start++
		end--
	}
	if end < start {
		return nil, fmt.Errorf("CIDR 太小")
	}

	total := int(end - start + 1)
	if total > 65536 {
		return nil, fmt.Errorf("子网过大 (%d 个地址)，请缩小范围或使用更长的前缀", total)
	}

	out := make([]string, 0, total)
	for i := start; ; i++ {
		b := make(net.IP, 4)
		binary.BigEndian.PutUint32(b, i)
		out = append(out, b.String())
		if i == end {
			break
		}
	}
	return out, nil
}

// Scan concurrently probes every host in cidr and reports results through
// onEvent. It runs in two phases, both designed for responsiveness:
//
//  1. Liveness: each host is pinged (with a fast TCP fallback) and a "host"
//     event is emitted immediately so the UI can render devices as soon as
//     they are found. When every host has been probed a "done" event is
//     emitted, marking device discovery complete.
//
//  2. Enrichment: alive hosts are then enriched in the background (hostname
//     lookup, MAC resolution, open-port scanning) and an updated "host" event
//     is emitted for each one as it finishes. The slow work therefore never
//     delays device display.
//
// The onEvent callback is invoked from worker goroutines; the caller is
// responsible for serialising if needed.
//
// If ctx is cancelled the function returns promptly: each probe checks the
// context before starting work, so in-flight SystemPing/ProbeTCP calls are
// aborted via the spawned process's context.
func (s *Scanner) Scan(ctx context.Context, cidr string, onEvent func(Event)) error {
	hosts, err := ExpandCIDR(cidr)
	if err != nil {
		return err
	}
	if len(hosts) > s.opts.MaxHosts {
		return fmt.Errorf("扫描主机数 %d 超过上限 %d", len(hosts), s.opts.MaxHosts)
	}

	startedAt := time.Now()
	onEvent(Event{
		Type:      "start",
		Total:     len(hosts),
		StartedAt: startedAt,
		CIDR:      cidr,
	})

	var (
		scanned int64
		alive   int64
		wg      sync.WaitGroup
		sem     = make(chan struct{}, s.opts.WorkerCount)
		mu      sync.Mutex

		aliveHosts = make([]*Host, 0, len(hosts))
	)

	// Phase 1: liveness only. Emit every host the moment it is probed so
	// devices appear immediately, regardless of enrichment still to come.
	for _, ip := range hosts {
		select {
		case <-ctx.Done():
			// Cancellation: drain remaining goroutines (they will exit fast
			// because each probe checks ctx), then emit a final cancelled
			// event.
			go func() { wg.Wait() }()
			finishedAt := time.Now()
			onEvent(Event{
				Type:       "cancelled",
				Scanned:    int(atomic.LoadInt64(&scanned)),
				Total:      len(hosts),
				Alive:      int(atomic.LoadInt64(&alive)),
				FinishedAt: finishedAt,
				CIDR:       cidr,
			})
			return ctx.Err()
		case sem <- struct{}{}:
		}

		wg.Add(1)
		go func(target string) {
			defer wg.Done()
			defer func() { <-sem }()

			h := s.probeLiveness(ctx, target)
			n := atomic.AddInt64(&scanned, 1)
			if h.Alive {
				atomic.AddInt64(&alive, 1)
				mu.Lock()
				aliveHosts = append(aliveHosts, &h)
				mu.Unlock()
			}

			mu.Lock()
			ev := Event{
				Type:    "host",
				Scanned: int(n),
				Total:   len(hosts),
				Alive:   int(atomic.LoadInt64(&alive)),
			}
			if h.Alive {
				ev.Host = &h
			}
			onEvent(ev)
			mu.Unlock()
		}(ip)
	}

	wg.Wait()

	// Phase 1 complete: every device is known. Announce completion so the UI
	// reaches 100% progress right away; enrichment continues below and each
	// updated host is streamed as soon as it is ready.
	onEvent(Event{
		Type:       "done",
		Scanned:    len(hosts),
		Total:      len(hosts),
		Alive:      int(atomic.LoadInt64(&alive)),
		FinishedAt: time.Now(),
		CIDR:       cidr,
	})

	// Phase 2: enrich alive hosts in the background.
	if len(aliveHosts) > 0 {
		var ewg sync.WaitGroup
		for _, h := range aliveHosts {
			select {
			case <-ctx.Done():
				return s.emitCancelled(len(hosts), &alive, cidr, onEvent)
			case sem <- struct{}{}:
			}

			ewg.Add(1)
			go func(h *Host) {
				defer ewg.Done()
				defer func() { <-sem }()

				s.enrich(ctx, h)
				mu.Lock()
				onEvent(Event{
					Type:    "host",
					Scanned: len(hosts),
					Total:   len(hosts),
					Alive:   int(atomic.LoadInt64(&alive)),
					Host:    h,
				})
				mu.Unlock()
			}(h)
		}
		ewg.Wait()

		if ctx.Err() != nil {
			return s.emitCancelled(len(hosts), &alive, cidr, onEvent)
		}
	}
	return nil
}

func (s *Scanner) emitCancelled(total int, alive *int64, cidr string, onEvent func(Event)) error {
	onEvent(Event{
		Type:       "cancelled",
		Scanned:    total,
		Total:      total,
		Alive:      int(atomic.LoadInt64(alive)),
		FinishedAt: time.Now(),
		CIDR:       cidr,
	})
	return context.Canceled
}

// probeLiveness runs only the fast reachability checks (ICMP, then a
// concurrent TCP fallback) and returns the host without any enrichment.
func (s *Scanner) probeLiveness(ctx context.Context, ip string) Host {
	h := Host{IP: ip, FirstSeen: time.Now()}

	if ctx.Err() != nil {
		return h
	}

	lat, err := SystemPing(ctx, ip, s.opts.PingTimeout)
	if err == nil {
		h.Alive = true
		h.LatencyMs = lat.Milliseconds()
		h.Method = MethodICMP
		return h
	}

	if s.opts.UseTCPProbe {
		port := ProbeTCPAny(ctx, ip, s.opts.ProbePorts, s.opts.PingTimeout, s.opts.TCPConcurrency)
		if port > 0 {
			h.Alive = true
			h.Method = MethodTCP
		}
	}
	return h
}

// enrich fills in the slow-to-compute details for an already-known alive host.
func (s *Scanner) enrich(ctx context.Context, h *Host) {
	if ctx.Err() != nil {
		return
	}
	if s.opts.LookupHost {
		h.Hostname = ReverseLookup(ctx, h.IP)
	}
	if s.opts.LookupMAC {
		h.MAC = LookupMAC(h.IP)
	}
	if s.opts.UseTCPProbe {
		opens := ProbeTCPPorts(ctx, h.IP, s.opts.ProbePorts, s.opts.TCPPortTimeout, s.opts.TCPConcurrency)
		sort.Ints(opens)
		h.OpenPorts = opens
	}
}

// Summary bundles the final state of a scan.
type Summary struct {
	CIDR       string        `json:"cidr"`
	StartedAt  time.Time     `json:"started_at"`
	FinishedAt time.Time     `json:"finished_at"`
	Scanned    int           `json:"scanned"`
	Total      int           `json:"total"`
	Alive      int           `json:"alive"`
	Duration   time.Duration `json:"-"`
	DurationMs int64         `json:"duration_ms"`
}

// MarshalJSON ensures DurationMs is always emitted even when zero.
func (s Summary) MarshalJSON() ([]byte, error) {
	type alias Summary
	a := alias(s)
	if a.DurationMs == 0 {
		a.DurationMs = s.Duration.Milliseconds()
	}
	return json.Marshal(a)
}
