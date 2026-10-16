package lanscan

import (
	"context"
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"prompt-merger-native/netscan"
)

// ScanDefaults 保存服务默认扫描参数。调用方（Fyne 界面）未提供时用内置默认值。
type ScanDefaults struct {
	Workers    int
	TimeoutMs  int
	MaxHosts   int
	ProbePorts []int
	UseTCP     bool
	LookupMAC  bool
	LookupHost bool
}

// resolvedScanDefaults 补齐内置兜底，保证零值 ScanDefaults 也有合理默认。
func (s *Server) resolvedScanDefaults() ScanDefaults {
	d := s.scanDefaults
	if d.Workers <= 0 {
		d.Workers = 128
	}
	if d.TimeoutMs <= 0 {
		d.TimeoutMs = 800
	}
	if d.MaxHosts <= 0 {
		d.MaxHosts = 4096
	}
	if len(d.ProbePorts) == 0 {
		d.ProbePorts = netscan.DefaultProbePorts
	}
	return d
}

// Server 聚合「局域网设备扫描」浏览器版的 HTTP 路由与扫描会话管理。
type Server struct {
	mux          *http.ServeMux
	webFS        fs.FS
	scanDefaults ScanDefaults
	sessions     sync.Map // id(string) -> *Session

	prefsMu   sync.Mutex
	prefsPath string
	prefs     map[string]any
}

// New 构造 Server。webFS 需包含 index.html / style.css / app.js。
func New(webFS fs.FS, defaults ScanDefaults) *Server {
	s := &Server{
		webFS:        webFS,
		mux:          http.NewServeMux(),
		scanDefaults: defaults,
	}
	s.routes()
	return s
}

// Handler 返回可直接交给 http.Server 的路由。
func (s *Server) Handler() http.Handler {
	return s.mux
}

// CancelAll 取消并关闭所有仍在进行的扫描会话（窗口退出/停止服务时调用）。
func (s *Server) CancelAll() {
	s.sessions.Range(func(_, v any) bool {
		if sess, ok := v.(*Session); ok {
			sess.Close()
		}
		return true
	})
}

// SetPrefsPath 启用网页选项记忆：把页面里的选项保存到指定文件，跨会话/跨随机端口生效。
// 传空路径则不启用。
func (s *Server) SetPrefsPath(path string) {
	s.prefsMu.Lock()
	defer s.prefsMu.Unlock()
	s.prefsPath = path
	s.prefs = loadPrefsFile(path)
}

func loadPrefsFile(path string) map[string]any {
	if path == "" {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return nil
	}
	return m
}

func (s *Server) routes() {
	s.mux.HandleFunc("/api/health", s.handleHealth)
	s.mux.HandleFunc("/api/config", s.handleConfig)
	s.mux.HandleFunc("/api/prefs", s.handlePrefs)
	s.mux.HandleFunc("/api/interfaces", s.handleInterfaces)
	s.mux.HandleFunc("/api/scan", s.handleScanStart)
	s.mux.HandleFunc("/api/scan/", s.handleScanByID)
	s.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(s.webFS))))
	s.mux.HandleFunc("/", s.handleIndex)
}

// handlePrefs 读写网页选项记忆（GET 读取 / POST 保存），跨随机端口持久化。
func (s *Server) handlePrefs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.prefsMu.Lock()
		out := s.prefs
		if out == nil {
			out = map[string]any{}
		}
		s.prefsMu.Unlock()
		writeJSON(w, http.StatusOK, out)
	case http.MethodPost:
		var body map[string]any
		if err := json.NewDecoder(io.LimitReader(r.Body, 16<<10)).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		s.prefsMu.Lock()
		s.prefs = body
		path := s.prefsPath
		s.prefsMu.Unlock()
		if path != "" {
			raw, err := json.MarshalIndent(body, "", "  ")
			if err == nil {
				_ = os.WriteFile(path, raw, 0o644)
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		methodNotAllowed(w, http.MethodGet)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"version": "1.0.0",
		"time":    time.Now(),
	})
}

// handleConfig 上报解析后的默认扫描参数，前端据此预填 + 判断本地记忆是否过期。
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	d := s.resolvedScanDefaults()
	writeJSON(w, http.StatusOK, map[string]any{
		"workers":     d.Workers,
		"timeout_ms":  d.TimeoutMs,
		"max_hosts":   d.MaxHosts,
		"probe_ports": d.ProbePorts,
		"use_tcp":     d.UseTCP,
		"lookup_mac":  d.LookupMAC,
		"lookup_host": d.LookupHost,
	})
}

func (s *Server) handleInterfaces(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	ifaces, err := netscan.ListInterfaces()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"interfaces": ifaces})
}

// ScanRequest 是 POST /api/scan 的载荷。布尔指针字段允许省略时回落服务端默认。
type ScanRequest struct {
	Interface  string `json:"interface"` // 可选，仅用于展示
	CIDR       string `json:"cidr"`      // 必填
	UseTCP     *bool  `json:"use_tcp"`
	LookupMAC  *bool  `json:"lookup_mac"`
	LookupHost *bool  `json:"lookup_host"`
	Workers    int    `json:"workers"`
	TimeoutMs  int    `json:"timeout_ms"`
	MaxHosts   int    `json:"max_hosts"`
	Ports      []int  `json:"ports"`
}

func (s *Server) handleScanStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	var req ScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("请求体解析失败: %w", err))
		return
	}
	if strings.TrimSpace(req.CIDR) == "" {
		writeError(w, http.StatusBadRequest, errors.New("缺少 cidr 参数"))
		return
	}
	d := s.resolvedScanDefaults()
	opts := netscan.Options{
		UseTCPProbe:    d.UseTCP,
		LookupMAC:      d.LookupMAC,
		LookupHost:     d.LookupHost,
		WorkerCount:    d.Workers,
		MaxHosts:       d.MaxHosts,
		ProbePorts:     d.ProbePorts,
		PingTimeout:    time.Duration(d.TimeoutMs) * time.Millisecond,
		TCPPortTimeout: time.Duration(d.TimeoutMs) * time.Millisecond,
	}
	if req.UseTCP != nil {
		opts.UseTCPProbe = *req.UseTCP
	}
	if req.LookupMAC != nil {
		opts.LookupMAC = *req.LookupMAC
	}
	if req.LookupHost != nil {
		opts.LookupHost = *req.LookupHost
	}
	if req.Workers > 0 {
		opts.WorkerCount = req.Workers
	}
	if req.MaxHosts > 0 {
		opts.MaxHosts = req.MaxHosts
	}
	if req.TimeoutMs > 0 {
		opts.PingTimeout = time.Duration(req.TimeoutMs) * time.Millisecond
		opts.TCPPortTimeout = time.Duration(req.TimeoutMs) * time.Millisecond
	}
	if len(req.Ports) > 0 {
		opts.ProbePorts = req.Ports
	}

	sess := newSession(req, opts)
	s.sessions.Store(sess.ID, sess)

	go s.runSession(sess)

	writeJSON(w, http.StatusAccepted, map[string]any{
		"id":     sess.ID,
		"status": sess.Status(),
		"cidr":   sess.CIDR,
	})
}

func (s *Server) handleScanByID(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/scan/")
	if rest == "" {
		writeError(w, http.StatusBadRequest, errors.New("缺少扫描 id"))
		return
	}
	parts := strings.SplitN(rest, "/", 2)
	id := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	v, ok := s.sessions.Load(id)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("扫描不存在或已过期"))
		return
	}
	sess := v.(*Session)

	switch action {
	case "":
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		writeJSON(w, http.StatusOK, sess.Snapshot())
	case "cancel":
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}
		sess.Cancel()
		writeJSON(w, http.StatusOK, map[string]any{"id": id, "status": sess.Status()})
	case "events":
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		s.streamEvents(w, r, sess)
	case "csv":
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		s.serveScanCSV(w, sess)
	default:
		writeError(w, http.StatusNotFound, errors.New("未知操作: "+action))
	}
}

// streamEvents 通过 SSE 推送会话快照与增量事件，浏览器端 EventSource 消费。
func (s *Server) streamEvents(w http.ResponseWriter, r *http.Request, sess *Session) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, errors.New("流式响应不被支持"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ctx := r.Context()
	ch := sess.Subscribe()

	if snap := sess.Snapshot(); snap != nil {
		writeSSE(w, "snapshot", snap)
		flusher.Flush()
	}

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		case ev, ok := <-ch:
			if !ok {
				writeSSE(w, "end", map[string]any{"status": sess.Status()})
				flusher.Flush()
				return
			}
			writeSSE(w, "event", ev)
			flusher.Flush()
		}
	}
}

// serveScanCSV 导出扫描结果（UTF-8 BOM，Excel 友好）。
func (s *Server) serveScanCSV(w http.ResponseWriter, sess *Session) {
	snap := sess.Snapshot()
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="lanscan-%s.csv"`, time.Now().Format("20060102-150405")))
	w.Write([]byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM

	cw := csv.NewWriter(w)
	defer cw.Flush()
	_ = cw.Write([]string{"IP", "Hostname", "MAC", "Latency(ms)", "Method", "OpenPorts"})
	for _, h := range snap.Hosts {
		lat := ""
		if h.LatencyMs > 0 {
			lat = strconv.FormatInt(h.LatencyMs, 10)
		}
		_ = cw.Write([]string{
			h.IP,
			h.Hostname,
			h.MAC,
			lat,
			string(h.Method),
			joinInts(h.OpenPorts, ";"),
		})
	}
}

// handleIndex 返回页面并把解析后的默认参数内联到 HTML 模板占位符里。
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	b, err := fs.ReadFile(s.webFS, "index.html")
	if err != nil {
		http.Error(w, "index.html not found", http.StatusInternalServerError)
		return
	}
	d := s.resolvedScanDefaults()
	r0 := strings.NewReplacer(
		"{{opt_tcp}}", checkedAttr(d.UseTCP),
		"{{opt_mac}}", checkedAttr(d.LookupMAC),
		"{{opt_host}}", checkedAttr(d.LookupHost),
		"{{opt_workers}}", strconv.Itoa(d.Workers),
		"{{opt_timeout}}", strconv.Itoa(d.TimeoutMs),
		"{{opt_ports}}", joinInts(d.ProbePorts, ","),
	)
	html := r0.Replace(string(b))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(html))
}

func checkedAttr(v bool) string {
	if v {
		return " checked"
	}
	return ""
}

func joinInts(vs []int, sep string) string {
	parts := make([]string, len(vs))
	for i, v := range vs {
		parts[i] = strconv.Itoa(v)
	}
	return strings.Join(parts, sep)
}

// runSession 驱动单个扫描会话，把 netscan 事件流广播给订阅者。
func (s *Server) runSession(sess *Session) {
	defer sess.Close()

	sc := netscan.New(sess.opts)
	err := sc.Scan(sess.Context(), sess.CIDR, sess.OnEvent)
	if err == nil {
		return
	}
	// 取消时 Scanner 内部已发出 "cancelled" 事件并写好状态，这里不要覆盖成 error。
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return
	}
	sess.MarkError(err)
}

// --- helpers ---------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(body)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{"error": err.Error()})
}

func methodNotAllowed(w http.ResponseWriter, allowed string) {
	w.Header().Set("Allow", allowed)
	writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("仅支持 %s 方法", allowed))
}

func writeSSE(w http.ResponseWriter, event string, data any) {
	b, err := json.Marshal(data)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "event: %s\n", event)
	fmt.Fprintf(w, "data: %s\n\n", string(b))
}

func newSessionID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// --- session ---------------------------------------------------------------

// Session 表示一个进行中（或已结束）的扫描。
type Session struct {
	ID        string
	CIDR      string
	Interface string
	opts      netscan.Options

	mu       sync.RWMutex
	hosts    map[string]*netscan.Host
	order    []string
	status   string
	start    time.Time
	finish   time.Time
	summary  netscan.Summary
	errMsg   string
	subMu    sync.Mutex
	subs     []chan netscan.Event
	cancel   context.CancelFunc
	ctx      context.Context
	doneOnce sync.Once
	closed   chan struct{}
}

// Snapshot 是返回给前端的 JSON 形状。
type Snapshot struct {
	ID         string          `json:"id"`
	CIDR       string          `json:"cidr"`
	Interface  string          `json:"interface,omitempty"`
	Status     string          `json:"status"`
	StartedAt  time.Time       `json:"started_at,omitempty"`
	FinishedAt time.Time       `json:"finished_at,omitempty"`
	Hosts      []*netscan.Host `json:"hosts"`
	Summary    netscan.Summary `json:"summary"`
	Error      string          `json:"error,omitempty"`
}

func newSession(req ScanRequest, opts netscan.Options) *Session {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Session{
		ID:        newSessionID(),
		CIDR:      req.CIDR,
		Interface: req.Interface,
		opts:      opts,
		status:    "running",
		start:     time.Now(),
		hosts:     make(map[string]*netscan.Host),
		ctx:       ctx,
		cancel:    cancel,
		closed:    make(chan struct{}),
	}
	s.summary.CIDR = req.CIDR
	s.summary.StartedAt = s.start
	return s
}

func (s *Session) Context() context.Context { return s.ctx }

func (s *Session) Cancel() { s.cancel() }

func (s *Session) MarkError(err error) {
	s.mu.Lock()
	s.status = "error"
	s.errMsg = err.Error()
	s.finish = time.Now()
	s.summary.FinishedAt = s.finish
	s.summary.Duration = s.finish.Sub(s.start)
	s.mu.Unlock()
	s.broadcastAll(netscan.Event{Type: "error", Error: err.Error(), FinishedAt: s.finish})
}

func (s *Session) OnEvent(ev netscan.Event) {
	s.mu.Lock()
	switch ev.Type {
	case "start":
		s.summary.Total = ev.Total
		s.summary.StartedAt = ev.StartedAt
	case "host":
		s.summary.Scanned = ev.Scanned
		s.summary.Total = ev.Total
		s.summary.Alive = ev.Alive
		if ev.Host != nil {
			cp := *ev.Host
			if _, ok := s.hosts[cp.IP]; !ok {
				s.order = append(s.order, cp.IP)
			}
			s.hosts[cp.IP] = &cp
		}
	case "done":
		s.status = "done"
		s.finish = ev.FinishedAt
		s.summary.Scanned = ev.Scanned
		s.summary.Total = ev.Total
		s.summary.Alive = ev.Alive
		s.summary.FinishedAt = ev.FinishedAt
		s.summary.Duration = ev.FinishedAt.Sub(s.summary.StartedAt)
	case "cancelled":
		s.status = "cancelled"
		s.finish = ev.FinishedAt
		s.summary.Scanned = ev.Scanned
		s.summary.Total = ev.Total
		s.summary.Alive = ev.Alive
		s.summary.FinishedAt = ev.FinishedAt
		s.summary.Duration = ev.FinishedAt.Sub(s.summary.StartedAt)
	}
	s.mu.Unlock()

	s.broadcastAll(ev)

	// "done" 只代表设备发现完成；后台 enrichment 事件仍可能继续流入，会话
	// 保持打开直到 Scan 返回（runSession defer Close）。
	if ev.Type == "error" || ev.Type == "cancelled" {
		s.Close()
	}
}

// Close 结束会话：取消扫描、通知所有订阅者（关闭频道）、广播不可再订阅。
func (s *Session) Close() {
	s.doneOnce.Do(func() {
		s.mu.Lock()
		if s.status == "running" {
			s.status = "cancelled"
		}
		s.cancel()
		s.mu.Unlock()
		s.subMu.Lock()
		for _, ch := range s.subs {
			close(ch)
		}
		s.subs = nil
		s.subMu.Unlock()
		close(s.closed)
	})
}

func (s *Session) Subscribe() <-chan netscan.Event {
	ch := make(chan netscan.Event, 64)
	s.subMu.Lock()
	s.subs = append(s.subs, ch)
	s.subMu.Unlock()
	return ch
}

func (s *Session) broadcastAll(ev netscan.Event) {
	s.subMu.Lock()
	for _, ch := range s.subs {
		select {
		case ch <- ev:
		default:
			// 订阅者消费慢时丢弃事件，避免阻塞扫描
		}
	}
	s.subMu.Unlock()
}

func (s *Session) Status() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.status == "" {
		return "running"
	}
	return s.status
}

func (s *Session) Snapshot() *Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	hosts := make([]*netscan.Host, 0, len(s.order))
	for _, ip := range s.order {
		hosts = append(hosts, s.hosts[ip])
	}
	status := s.status
	if status == "" {
		status = "running"
	}
	return &Snapshot{
		ID:         s.ID,
		CIDR:       s.CIDR,
		Interface:  s.Interface,
		Status:     status,
		StartedAt:  s.start,
		FinishedAt: s.finish,
		Hosts:      hosts,
		Summary:    s.summary,
		Error:      s.errMsg,
	}
}
