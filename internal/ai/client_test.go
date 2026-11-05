package ai

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// collect 把各回调的输出收集起来，便于断言。
type collect struct {
	delta     strings.Builder
	reasoning strings.Builder
	usage     *Usage
	err       error
}

func runStream(body string, contentType string, status int) (*collect, error) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	c := &collect{}
	cfg := Config{BaseURL: srv.URL, Model: "test-model"}
	err := Stream(context.Background(), cfg,
		[]Message{{Role: "user", Content: "hi"}},
		func(s string) { c.delta.WriteString(s) },
		func(s string) { c.reasoning.WriteString(s) },
		func(u Usage) { c.usage = &u },
	)
	c.err = err
	return c, err
}

func TestStreamMultiLineDataEvent(t *testing.T) {
	// 同一事件内两行 data:（部分服务把一段 JSON 换行输出）。
	body := "data: {\"choices\":[{\"delta\":{\"content\":\"你\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":\n" +
		"data: {\"content\":\"好\"}}]}\n\n" +
		"data: [DONE]\n\n"
	c, err := runStream(body, "text/event-stream", http.StatusOK)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if got := c.delta.String(); got != "你好" {
		t.Fatalf("delta = %q, want 你好", got)
	}
}

func TestStreamReasoningAndUsage(t *testing.T) {
	body := "data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"想一想\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"答\"}}]}\n\n" +
		"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5}}\n\n" +
		"data: [DONE]\n\n"
	c, err := runStream(body, "text/event-stream", http.StatusOK)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if got := c.reasoning.String(); got != "想一想" {
		t.Fatalf("reasoning = %q", got)
	}
	if got := c.delta.String(); got != "答" {
		t.Fatalf("delta = %q", got)
	}
	if c.usage == nil || c.usage.PromptTokens != 10 || c.usage.CompletionTokens != 5 {
		t.Fatalf("usage = %+v", c.usage)
	}
	if c.usage.TotalTokens != 15 {
		t.Fatalf("total = %d, want 15", c.usage.TotalTokens)
	}
}

func TestStreamReasoningAltField(t *testing.T) {
	body := "data: {\"choices\":[{\"delta\":{\"reasoning\":\"备选字段\"}}]}\n\n"
	c, err := runStream(body, "text/event-stream", http.StatusOK)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if got := c.reasoning.String(); got != "备选字段" {
		t.Fatalf("reasoning = %q", got)
	}
}

func TestStreamReaderErrorSurfaces(t *testing.T) {
	// 响应中途截断：必须返回错误，而不是当作正常结束。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"部分\"}}]}\n\n"))
		// 直接关闭连接（Hijack 保证没有正常收尾）
		hj, ok := w.(http.Hijacker)
		if !ok {
			panic("no hijack")
		}
		conn, buf, _ := hj.Hijack()
		_ = buf.Flush()
		_ = conn.Close()
	}))
	defer srv.Close()

	cfg := Config{BaseURL: srv.URL, Model: "test-model"}
	err := Stream(context.Background(), cfg,
		[]Message{{Role: "user", Content: "hi"}},
		func(string) {}, nil, nil)
	if err == nil {
		t.Fatal("expected error from truncated stream")
	}
	if !strings.Contains(err.Error(), "读取流失败") && !strings.Contains(err.Error(), "unexpected EOF") {
		t.Fatalf("error = %v", err)
	}
}

func TestStreamUpstreamErrorChunk(t *testing.T) {
	body := "data: {\"error\":{\"message\":\"余额不足\"}}\n\n"
	_, err := runStream(body, "text/event-stream", http.StatusOK)
	if err == nil || !strings.Contains(err.Error(), "余额不足") {
		t.Fatalf("err = %v", err)
	}
}

func TestStreamNonStreamFallback(t *testing.T) {
	body := `{"choices":[{"message":{"content":"完整回答"}}],"usage":{"prompt_tokens":3,"completion_tokens":4}}`
	c, err := runStream(body, "application/json", http.StatusOK)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if got := c.delta.String(); got != "完整回答" {
		t.Fatalf("delta = %q", got)
	}
	if c.usage == nil || c.usage.TotalTokens != 7 {
		t.Fatalf("usage = %+v", c.usage)
	}
}

func TestStreamHTTPError(t *testing.T) {
	_, err := runStream(`{"error":{"message":"key 无效"}}`, "application/json", http.StatusUnauthorized)
	if err == nil || !strings.Contains(err.Error(), "key 无效") {
		t.Fatalf("err = %v", err)
	}
}

func TestStreamCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fl := w.(http.Flusher)
		for i := 0; i < 100; i++ {
			_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"x\"}}]}\n\n"))
			fl.Flush()
			time.Sleep(30 * time.Millisecond)
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	cfg := Config{BaseURL: srv.URL, Model: "test-model"}
	err := Stream(ctx, cfg, []Message{{Role: "user", Content: "hi"}}, func(string) {}, nil, nil)
	if err == nil {
		t.Fatal("expected cancel error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
}

func TestBuildMessagesMergesAndSkipsEmpty(t *testing.T) {
	msgs := BuildMessages("系统", []Message{
		{Role: "user", Content: "a"},
		{Role: "user", Content: "b"},
		{Role: "tool", Content: ""},
		{Role: "assistant", Content: "c"},
	})
	if len(msgs) != 3 {
		t.Fatalf("messages = %d: %+v", len(msgs), msgs)
	}
	if msgs[0].Role != "system" || msgs[0].Content != "系统" {
		t.Fatalf("first = %+v", msgs[0])
	}
	if msgs[1].Content != "a\nb" {
		t.Fatalf("merge = %q", msgs[1].Content)
	}
	if msgs[2].Role != "assistant" {
		t.Fatalf("last = %+v", msgs[2])
	}
}

func TestChatCompletionsURL(t *testing.T) {
	cases := map[string]string{
		"https://api.openai.com/v1":      "https://api.openai.com/v1/chat/completions",
		"https://api.openai.com/v1/":     "https://api.openai.com/v1/chat/completions",
		"https://x/v1/chat/completions":  "https://x/v1/chat/completions",
		"https://x/v1/chat/completions/": "https://x/v1/chat/completions",
	}
	for in, want := range cases {
		if got := ChatCompletionsURL(in); got != want {
			t.Fatalf("ChatCompletionsURL(%q) = %q, want %q", in, got, want)
		}
	}
}
