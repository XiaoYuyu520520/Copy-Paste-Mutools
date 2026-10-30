package netscan

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ProbeMethod describes how a host was confirmed alive.
type ProbeMethod string

const (
	MethodICMP ProbeMethod = "ping"
	MethodTCP  ProbeMethod = "tcp"
)

// Host represents a discovered device on the LAN.
type Host struct {
	IP        string      `json:"ip"`
	Hostname  string      `json:"hostname,omitempty"`
	MAC       string      `json:"mac,omitempty"`
	Vendor    string      `json:"vendor,omitempty"`
	Alive     bool        `json:"alive"`
	LatencyMs int64       `json:"latency_ms,omitempty"`
	Method    ProbeMethod `json:"method,omitempty"`
	OpenPorts []int       `json:"open_ports,omitempty"`
	FirstSeen time.Time   `json:"first_seen"`
}

// SystemPing uses the OS `ping` command to check reachability. The supplied
// context is honoured: when it is cancelled the spawned ping process is
// killed, which lets Scan stop immediately even if many hosts are queued.
func SystemPing(ctx context.Context, ip string, timeout time.Duration) (time.Duration, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout+500*time.Millisecond)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		ms := int(timeout / time.Millisecond)
		if ms < 100 {
			ms = 100
		}
		cmd = exec.CommandContext(ctx, "ping", "-n", "1", "-w", strconv.Itoa(ms), ip)
	} else {
		sec := int(timeout / time.Second)
		if sec < 1 {
			sec = 1
		}
		cmd = exec.CommandContext(ctx, "ping", "-c", "1", "-W", strconv.Itoa(sec), ip)
	}
	hideChildWindow(cmd)

	out, err := cmd.CombinedOutput()
	text := string(out)
	// 存活判定用 ASCII 特征（各语言版本都带 TTL= / time= / time<），
	// 中文系统 ping 输出是 GBK 编码的「时间=…」，不能用中文字面量匹配。
	if replied(text) {
		return parseLatency(text), nil
	}
	// Treat context cancellation as a clean abort instead of an error.
	if ctx.Err() != nil {
		return -1, ctx.Err()
	}
	msg := strings.TrimSpace(text)
	if msg == "" && err != nil {
		msg = err.Error()
	}
	return -1, fmt.Errorf("ping 失败: %s", msg)
}

// replied 通过输出里的 ASCII 标记判断是否收到回复（跨语言：TTL / time= / time<）。
func replied(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "ttl=") ||
		strings.Contains(lower, "time=") ||
		strings.Contains(lower, "time<")
}

var msRe = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)\s*ms`)

// parseLatency 提取文本中第一处 "<数字>ms" 作为往返延迟（ASCII 数字，跨语言可用）。
func parseLatency(s string) time.Duration {
	m := msRe.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0
	}
	return time.Duration(v * float64(time.Millisecond))
}

// ProbeTCPAny probes a set of ports concurrently and returns the first port
// that accepts a connection, or 0 if none do. Used for fast TCP-based
// liveness detection: unlike ProbeTCPPorts it does not wait for every port to
// be checked, so a firewall that silently drops traffic only costs one dial
// timeout instead of one per port.
func ProbeTCPAny(ctx context.Context, host string, ports []int, timeout time.Duration, concurrency int) int {
	if len(ports) == 0 {
		return 0
	}
	if concurrency <= 0 {
		concurrency = 16
	}
	if concurrency > len(ports) {
		concurrency = len(ports)
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	work := make(chan int, len(ports))
	results := make(chan int, 1)
	var wg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range work {
				if ProbeTCP(ctx, host, p, timeout) {
					select {
					case results <- p:
					default:
					}
					cancel()
					return
				}
			}
		}()
	}

	for _, p := range ports {
		select {
		case work <- p:
		case <-ctx.Done():
		}
	}
	close(work)

	go func() {
		wg.Wait()
		close(results)
	}()

	for p := range results {
		return p
	}
	return 0
}

// ProbeTCP attempts a single TCP handshake against host:port. The supplied
// context can abort a pending dial immediately.
func ProbeTCP(ctx context.Context, host string, port int, timeout time.Duration) bool {
	if ctx.Err() != nil {
		return false
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	d := &net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// ProbeTCPPorts probes a list of ports concurrently and returns the open ones.
func ProbeTCPPorts(ctx context.Context, host string, ports []int, timeout time.Duration, concurrency int) []int {
	if len(ports) == 0 {
		return nil
	}
	type result struct {
		port int
		open bool
	}
	ch := make(chan int, len(ports))
	for _, p := range ports {
		ch <- p
	}
	close(ch)
	results := make(chan result, len(ports))
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range ch {
				if ctx.Err() != nil {
					return
				}
				open := ProbeTCP(ctx, host, p, timeout)
				results <- result{p, open}
			}
		}()
	}
	go func() {
		wg.Wait()
		close(results)
	}()
	var opens []int
	for r := range results {
		if r.open {
			opens = append(opens, r.port)
		}
	}
	return opens
}

// ReverseLookup returns the first PTR record for the IP, or empty string.
func ReverseLookup(ctx context.Context, ip string) string {
	if ctx.Err() != nil {
		return ""
	}
	rctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	names, err := net.DefaultResolver.LookupAddr(rctx, ip)
	if err != nil || len(names) == 0 {
		return ""
	}
	return strings.TrimSuffix(names[0], ".")
}

// LookupMAC returns the MAC address for an IP from the OS ARP table.
func LookupMAC(ip string) string {
	switch runtime.GOOS {
	case "windows":
		return lookupMACWindows(ip)
	case "linux", "darwin":
		return lookupMACUnix(ip)
	}
	return ""
}

var (
	arpCacheMu sync.Mutex
	arpCache   map[string]string
	arpCacheAt time.Time
)

func lookupMACWindows(ip string) string {
	arpCacheMu.Lock()
	if arpCache != nil && time.Since(arpCacheAt) < 5*time.Second {
		c := arpCache[ip]
		arpCacheMu.Unlock()
		return c
	}
	arpCacheMu.Unlock()

	cmd := exec.Command("arp", "-a")
	hideChildWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	m := parseArpTable(string(out))
	arpCacheMu.Lock()
	arpCache = m
	arpCacheAt = time.Now()
	arpCacheMu.Unlock()
	return m[ip]
}

// parseArpTable parses both `arp -a` (Windows) and `/proc/net/arp` (Linux).
func parseArpTable(out string) map[string]string {
	res := make(map[string]string)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		ip := fields[0]
		if net.ParseIP(ip) == nil {
			continue
		}
		mac := fields[1]
		lower := strings.ToLower(mac)
		if strings.Contains(lower, "incomplete") || strings.HasPrefix(lower, "ff:") {
			continue
		}
		if _, ok := res[ip]; !ok {
			res[ip] = strings.ToUpper(mac)
		}
	}
	return res
}

func lookupMACUnix(ip string) string {
	out, err := exec.Command("cat", "/proc/net/arp").Output()
	if err == nil {
		m := parseArpTable(string(out))
		if mac, ok := m[ip]; ok {
			return mac
		}
		return ""
	}
	out2, err := exec.Command("arp", "-n", ip).Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out2), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[0] == ip {
			mac := fields[2]
			if !strings.Contains(strings.ToLower(mac), "incomplete") {
				return strings.ToUpper(mac)
			}
		}
	}
	return ""
}
