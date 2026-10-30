// Package monitor 实现开发板性能监视器的后端。
//
// 迁移自 legacy/python/Linux开发板性能监视器/app.py（Flask + SSE）：
//   - GET /                前端页面
//   - GET /api/status      一次性状态快照
//   - GET /api/stream      SSE 持续推送系统状态
//   - GET /api/ping/stream SSE 持续推送到目标的 ping 延迟
//
// 改成 Go 原生实现后不再需要 Flask/Python 运行时，单文件即可部署到
// RDK / Jetson 等 Linux 板子上。
package monitor

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
)

// Options 是监视器配置。
type Options struct {
	// Addr 是监听地址。
	Addr string
	// Interval 是 SSE 推送间隔。
	Interval time.Duration
	// PingInterval 是 ping SSE 的推送间隔。
	PingInterval time.Duration
	// PingTimeout 是单次 ping 的超时。
	PingTimeout time.Duration
	// FrontendFS 是前端静态资源。
	FrontendFS http.FileSystem
}

// Status 是系统状态快照。
type Status struct {
	Timestamp string       `json:"timestamp"`
	CPU       CPUSample    `json:"cpu"`
	Memory    MemSample    `json:"memory"`
	Disk      []DiskSample `json:"disk"`
	Host      HostSample   `json:"host"`
	GPU       []GPUSample  `json:"gpu"`
}

// CPUSample 是 CPU 使用率采样。
type CPUSample struct {
	UsagePercent float64 `json:"usage_percent"`
	LogicalCores int     `json:"logical_cores"`
}

// MemSample 是内存采样。
type MemSample struct {
	TotalBytes     uint64  `json:"total_bytes"`
	UsedBytes      uint64  `json:"used_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	UsagePercent   float64 `json:"usage_percent"`
}

// DiskSample 是磁盘采样。
type DiskSample struct {
	Device       string  `json:"device"`
	Mountpoint   string  `json:"mountpoint"`
	TotalBytes   uint64  `json:"total_bytes"`
	UsedBytes    uint64  `json:"used_bytes"`
	UsagePercent float64 `json:"usage_percent"`
}

// HostSample 是主机信息。
type HostSample struct {
	OS       string `json:"os"`
	Hostname string `json:"hostname"`
	Uptime   string `json:"uptime"`
	Arch     string `json:"arch"`
}

// GPUSample 是显卡采样。
type GPUSample struct {
	Name        string  `json:"name"`
	UsagePct    float64 `json:"usage_percent"`
	MemoryUsed  uint64  `json:"memory_used_bytes"`
	MemoryTotal uint64  `json:"memory_total_bytes"`
	TempCelsius float64 `json:"temperature_celsius"`
}

// Server 是监视器 HTTP 服务。
type Server struct {
	opts Options
	mux  *http.ServeMux
	srv  *http.Server
}

// New 创建监视器服务。
func New(opts Options) *Server {
	if opts.Addr == "" {
		opts.Addr = "0.0.0.0:8888"
	}
	if opts.Interval <= 0 {
		opts.Interval = time.Second
	}
	if opts.PingInterval <= 0 {
		opts.PingInterval = time.Second
	}
	if opts.PingTimeout <= 0 {
		opts.PingTimeout = 3 * time.Second
	}
	s := &Server{opts: opts, mux: http.NewServeMux()}
	s.routes()
	s.srv = &http.Server{Addr: opts.Addr, Handler: s.mux}
	return s
}

// routes 注册路由。
func (s *Server) routes() {
	s.mux.HandleFunc("/api/status", s.handleStatus)
	s.mux.HandleFunc("/api/stream", s.handleStream)
	s.mux.HandleFunc("/api/ping/stream", s.handlePingStream)
	s.mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	if s.opts.FrontendFS != nil {
		s.mux.Handle("/", http.FileServer(s.opts.FrontendFS))
	}
}

// Handler 返回 HTTP 处理器。
func (s *Server) Handler() http.Handler { return s.mux }

// ListenAndServe 启动服务并阻塞。
func (s *Server) ListenAndServe() error {
	return s.srv.ListenAndServe()
}

// Shutdown 关闭服务并释放端口。
func (s *Server) Shutdown() error {
	if s.srv == nil {
		return nil
	}
	return s.srv.Close()
}

// handleStatus 返回一次性状态快照。
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	_ = json.NewEncoder(w).Encode(Collect())
}

// handleStream 通过 SSE 持续推送系统状态。
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "当前连接不支持流式响应", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)

	ticker := time.NewTicker(s.opts.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			payload, err := json.Marshal(Collect())
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", payload)
			flusher.Flush()
		}
	}
}

// pingResult 是一次 ping 的结果。
type pingResult struct {
	Alive     bool    `json:"alive"`
	LatencyMS float64 `json:"latency_ms"`
}

// handlePingStream 通过 SSE 持续推送到目标的 ping 延迟。
func (s *Server) handlePingStream(w http.ResponseWriter, r *http.Request) {
	target := strings.TrimSpace(r.URL.Query().Get("target"))
	if target == "" || !isValidTarget(target) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"error\":\"invalid or no target\"}\n\n"))
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "当前连接不支持流式响应", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)

	ticker := time.NewTicker(s.opts.PingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			payload, err := json.Marshal(Ping(target, s.opts.PingTimeout))
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", payload)
			flusher.Flush()
		}
	}
}

// Collect 采集一次系统状态。
func Collect() Status {
	st := Status{Timestamp: time.Now().Format(time.RFC3339)}

	percs, _ := cpu.Percent(0, false)
	if len(percs) > 0 {
		st.CPU.UsagePercent = percs[0]
	}
	st.CPU.LogicalCores = runtime.NumCPU()

	if vm, err := mem.VirtualMemory(); err == nil {
		st.Memory = MemSample{
			TotalBytes:     vm.Total,
			UsedBytes:      vm.Used,
			AvailableBytes: vm.Available,
			UsagePercent:   vm.UsedPercent,
		}
	}

	parts, _ := disk.Partitions(false)
	for _, p := range parts {
		usage, err := disk.Usage(p.Mountpoint)
		if err != nil {
			continue
		}
		st.Disk = append(st.Disk, DiskSample{
			Device:       p.Device,
			Mountpoint:   p.Mountpoint,
			TotalBytes:   usage.Total,
			UsedBytes:    usage.Used,
			UsagePercent: usage.UsedPercent,
		})
	}

	if h, err := host.Info(); err == nil {
		st.Host = HostSample{
			OS:       h.OS,
			Hostname: h.Hostname,
			Arch:     h.KernelArch,
		}
	}
	if up, err := host.Uptime(); err == nil {
		st.Host.Uptime = (time.Duration(up) * time.Second).Truncate(time.Second).String()
	}

	st.GPU = collectGPU()
	return st
}

// Ping 对目标执行一次 ping，返回是否存活与延迟。
func Ping(target string, timeout time.Duration) pingResult {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("ping", "-n", "1", "-w", strconv.Itoa(int(timeout.Milliseconds())), target)
	} else {
		cmd = exec.Command("ping", "-c", "1", "-W", strconv.Itoa(int(timeout.Seconds())), target)
	}
	out, err := cmd.Output()
	if err != nil {
		return pingResult{Alive: false}
	}
	return pingResult{Alive: true, LatencyMS: parseLatency(string(out))}
}

// latencyPattern 兼容中英文 ping 输出。
var latencyPattern = regexp.MustCompile(`(?i)(?:time=|时间[=<])\s*([0-9.]+)\s*ms`)

// parseLatency 从 ping 输出里提取毫秒延迟。
func parseLatency(text string) float64 {
	m := latencyPattern.FindStringSubmatch(text)
	if len(m) < 2 {
		return 0
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0
	}
	return v
}

// targetPattern 校验目标只能是主机名或 IPv4。
var targetPattern = regexp.MustCompile(`^(([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])\.)*([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9\-]*[A-Za-z0-9])$`)

// isValidTarget 校验 target 是合法的主机名或 IP，防止参数注入。
func isValidTarget(target string) bool {
	return targetPattern.MatchString(target)
}
