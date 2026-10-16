package chat

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"prompt-merger-native/render"
)

func newTS(configured bool) http.Handler {
	get := func() Settings {
		s := Settings{Model: "gpt-4o-mini"}
		if configured {
			s.BaseURL = "http://127.0.0.1:1/v1"
		}
		return s
	}
	return New(render.Assets(), get)
}

func TestIndexPage(t *testing.T) {
	rec := httptest.NewRecorder()
	newTS(true).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("index = %d, want 200", rec.Code)
	}
	for _, want := range []string{"AI 对话", "marked.min.js", "/api/chat", "modelSel"} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("页面缺少 %q", want)
		}
	}
}

func TestConfigUnconfigured(t *testing.T) {
	rec := httptest.NewRecorder()
	newTS(false).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("config = %d, want 200", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["configured"] != false {
		t.Fatalf("configured = %v, want false", body["configured"])
	}
}

func TestModelsUnconfiguredNoNetwork(t *testing.T) {
	rec := httptest.NewRecorder()
	newTS(false).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/models", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("models = %d, want 200", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	list := body["models"].([]any)
	if len(list) != 0 {
		t.Fatalf("models 应为空，got %v", list)
	}
}

func TestChatRequiresConfig(t *testing.T) {
	rec := httptest.NewRecorder()
	body := `{"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(body))
	newTS(false).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("chat = %d, want 400", rec.Code)
	}
}

func TestBuildUpstreamMessages(t *testing.T) {
	msgs := buildUpstreamMessages("你是助手", []chatMessage{
		{Role: "user", Content: "  你好  "},
		{Role: "assistant", Content: ""},
		{Role: "user", Content: "继续"},
	})
	if len(msgs) != 2 || msgs[0].Role != "system" || msgs[0].Content != "你是助手" {
		t.Fatalf("unexpected messages: %+v", msgs)
	}
	if msgs[1].Role != "user" || msgs[1].Content != "你好\n继续" {
		t.Fatalf("空 assistant 应被跳过且相邻 user 合并: %+v", msgs)
	}
}

func TestHistoryRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hist.json")
	SetHistoryPath(path)
	defer SetHistoryPath("")

	h := newTS(true)
	body := `[{"id":"a","title":"hi","updated":1,"messages":[{"role":"user","content":"hello"}]}]`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/history", strings.NewReader(body))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST history = %d, want 200", rec.Code)
	}

	// 模拟重启：重新 SetHistoryPath 后仍能读到
	SetHistoryPath(path)
	defer SetHistoryPath("")
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/api/history", nil))
	if rec2.Code != http.StatusOK {
		t.Fatalf("GET history = %d, want 200", rec2.Code)
	}
	if !strings.Contains(rec2.Body.String(), `"id":"a"`) {
		t.Fatalf("历史未持久化: %s", rec2.Body.String())
	}
}
