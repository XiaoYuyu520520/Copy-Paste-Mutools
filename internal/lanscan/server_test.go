package lanscan

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestServer() *Server {
	return New(Assets(), ScanDefaults{
		Workers:    64,
		TimeoutMs:  500,
		MaxHosts:   1024,
		ProbePorts: []int{22, 80, 443},
		UseTCP:     true,
		LookupMAC:  true,
		LookupHost: false,
	})
}

func doGet(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHealth(t *testing.T) {
	rec := doGet(t, newTestServer().Handler(), "/api/health")
	if rec.Code != http.StatusOK {
		t.Fatalf("health = %d, want 200", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析 health 失败: %v", err)
	}
	if body["ok"] != true {
		t.Fatalf("ok = %v, want true", body["ok"])
	}
}

func TestConfigInlinesDefaults(t *testing.T) {
	s := newTestServer()
	rec := doGet(t, s.Handler(), "/api/config")
	if rec.Code != http.StatusOK {
		t.Fatalf("config = %d, want 200", rec.Code)
	}
	var cfg map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("解析 config 失败: %v", err)
	}
	if cfg["workers"] != float64(64) || cfg["timeout_ms"] != float64(500) {
		t.Fatalf("config 默认值错误: %v", cfg)
	}

	// index.html 内联占位符应被替换掉
	page := doGet(t, s.Handler(), "/")
	if page.Code != http.StatusOK {
		t.Fatalf("index = %d, want 200", page.Code)
	}
	html := page.Body.String()
	if strings.Contains(html, "{{opt_") {
		t.Fatalf("index.html 存在未替换的模板占位符")
	}
	if !strings.Contains(html, `value="500"`) || !strings.Contains(html, `value="22,80,443"`) {
		t.Fatalf("index.html 未内联默认值: %v", html)
	}
}

func TestStaticServed(t *testing.T) {
	rec := doGet(t, newTestServer().Handler(), "/static/app.js")
	if rec.Code != http.StatusOK {
		t.Fatalf("static app.js = %d, want 200", rec.Code)
	}
	data, _ := io.ReadAll(rec.Body)
	if len(data) == 0 || !strings.Contains(string(data), "lanscan.options") {
		t.Fatalf("app.js 内容异常")
	}
}

func TestUnknownRoute404(t *testing.T) {
	rec := doGet(t, newTestServer().Handler(), "/api/nope")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown = %d, want 404", rec.Code)
	}
}
