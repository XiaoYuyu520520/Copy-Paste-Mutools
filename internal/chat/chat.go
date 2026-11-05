// Package chat 提供局域网浏览器版 AI 聊天：OpenAI 兼容接口 + SSE 流式转发。
//
// 视觉与交互参考 RikkaHub 嵌入 Web UI 的主对话形态：
// 左侧会话列表 + 顶部模型切换 + 消息气泡 / Markdown / 思维链 + 底部输入框。
// 纯离线：marked / highlight.js / github 样式复用 render 包内嵌资源。
package chat

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/ai"
	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/data"
)

// Settings 一次请求需要的上游配置。
type Settings struct {
	BaseURL string // OpenAI 兼容根地址，例如 https://api.openai.com/v1
	APIKey  string
	Model   string
	System  string // 可选的系统提示词
}

// Effective 是否已配置可用。
func (s Settings) Effective() bool {
	return strings.TrimSpace(s.BaseURL) != "" && strings.TrimSpace(s.Model) != ""
}

// histStore 会话历史记忆：保存到文件，跨随机端口（每次重启 origin 变化）仍可恢复。
var histStore struct {
	mu   sync.Mutex
	path string
	raw  []byte
}

// SetHistoryPath 指定会话历史文件路径；空则不持久化。
func SetHistoryPath(path string) {
	histStore.mu.Lock()
	defer histStore.mu.Unlock()
	histStore.path = path
	histStore.raw = nil
	if path == "" {
		return
	}
	if b, err := os.ReadFile(path); err == nil {
		histStore.raw = b
	}
}

func historyGET() []byte {
	histStore.mu.Lock()
	defer histStore.mu.Unlock()
	if len(histStore.raw) == 0 {
		return []byte("[]")
	}
	out := make([]byte, len(histStore.raw))
	copy(out, histStore.raw)
	return out
}

func historySave(raw []byte) error {
	histStore.mu.Lock()
	defer histStore.mu.Unlock()
	histStore.raw = append([]byte(nil), raw...)
	if histStore.path == "" {
		return nil
	}
	return data.SaveFileAtomic(histStore.path, raw)
}

// New 构造聊天服务路由。
//
//	assets：静态资源（marked/highlight/github 样式），通常传 render.Assets()
//	get：每次请求时读取最新设置（从桌面端「AI 设置」保存）
func New(assets fs.FS, get func() Settings) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assets))))
	mux.HandleFunc("/api/config", handleConfig(get))
	mux.HandleFunc("/api/models", handleModels(get))
	mux.HandleFunc("/api/history", handleHistory)
	mux.HandleFunc("/api/chat", handleChat(get))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, indexHTML)
	})
	return mux
}

// handleConfig 暴露非敏感信息（不返回 API Key），供页面展示/预填模型。
func handleConfig(get func() Settings) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		cfg := get()
		writeJSON(w, http.StatusOK, map[string]any{
			"configured": cfg.Effective(),
			"base_url":   cfg.BaseURL,
			"model":      cfg.Model,
			"system":     cfg.System != "",
		})
	}
}

// handleHistory 读写会话历史（页面把整份会话数组 POST 回来保存）。
func handleHistory(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write(historyGET())
	case http.MethodPost:
		raw, err := io.ReadAll(io.LimitReader(r.Body, 16<<20))
		if err != nil {
			http.Error(w, "读取失败: "+err.Error(), http.StatusBadRequest)
			return
		}
		if err := historySave(raw); err != nil {
			http.Error(w, "保存历史失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"ok":true}`))
	default:
		methodNotAllowed(w, http.MethodGet)
	}
}

// buildUpstreamMessages 把系统提示词前置，跳过空消息、合并连续同角色。
//
// 实现委托给 internal/ai，桌面端 / 移动端 / 网页端共用同一套消息整理逻辑。
func buildUpstreamMessages(system string, msgs []chatMessage) []chatMessage {
	converted := make([]ai.Message, len(msgs))
	for i, m := range msgs {
		converted[i] = ai.Message{Role: m.Role, Content: m.Content}
	}
	out := ai.BuildMessages(system, converted)
	res := make([]chatMessage, len(out))
	for i, m := range out {
		res[i] = chatMessage{Role: m.Role, Content: m.Content}
	}
	return res
}

// handleModels 从上游 /models 拉取可用模型供页面上方下拉选择；失败时只返回默认模型。
func handleModels(get func() Settings) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		cfg := get()
		ids := []string{}
		if cfg.Effective() {
			if list, err := ai.ListModels(r.Context(), ai.Config{
				BaseURL: cfg.BaseURL, APIKey: cfg.APIKey, Model: cfg.Model,
			}); err == nil {
				ids = list
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"models": ids, "default": cfg.Model})
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

// streamEvent 返回给页面的统一事件。
type streamEvent struct {
	Type             string `json:"type"` // delta / reasoning / usage / error / done
	Content          string `json:"content,omitempty"`
	PromptTokens     int64  `json:"prompt_tokens,omitempty"`
	CompletionTokens int64  `json:"completion_tokens,omitempty"`
}

// handleChat 接收前端请求并把上游 OpenAI 兼容流逐块转发为 SSE。
//
// 上游解析统一交给 internal/ai（SSE 多行 data: 事件、思维链字段兼容、
// 非流式回退、读取错误上报），这里只负责 HTTP 转发与事件编码。
func handleChat(get func() Settings) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}
		cfg := get()
		if !cfg.Effective() {
			http.Error(w, "尚未配置 API 地址与模型，请先在桌面端「AI 设置」中填写", http.StatusBadRequest)
			return
		}

		var req chatRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 4<<20)).Decode(&req); err != nil {
			http.Error(w, "请求体解析失败: "+err.Error(), http.StatusBadRequest)
			return
		}
		if len(req.Messages) == 0 {
			http.Error(w, "缺少消息内容", http.StatusBadRequest)
			return
		}

		msgs := make([]ai.Message, len(req.Messages))
		for i, m := range req.Messages {
			msgs[i] = ai.Message{Role: m.Role, Content: m.Content}
		}

		// 先写 SSE 头，再发起上游请求：错误以 error 事件返回，
		// 页面能拿到具体原因而不是「网络错误」。
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "流式响应不被支持", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)

		send := func(ev streamEvent) bool {
			b, err := json.Marshal(ev)
			if err != nil {
				return false
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
				return false
			}
			flusher.Flush()
			return true
		}
		defer send(streamEvent{Type: "done"})

		err := ai.StreamWithOptions(r.Context(),
			ai.Config{BaseURL: cfg.BaseURL, APIKey: cfg.APIKey, Model: cfg.Model, System: cfg.System},
			msgs,
			ai.Options{
				Model:             req.Model,
				IncludeUsage:      true,
				FallbackNonStream: true,
			},
			func(delta string) { send(streamEvent{Type: "delta", Content: delta}) },
			func(reasoning string) { send(streamEvent{Type: "reasoning", Content: reasoning}) },
			func(u ai.Usage) {
				send(streamEvent{Type: "usage", PromptTokens: u.PromptTokens, CompletionTokens: u.CompletionTokens})
			},
		)
		if err != nil && r.Context().Err() == nil {
			send(streamEvent{Type: "error", Content: err.Error()})
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(body)
}

func methodNotAllowed(w http.ResponseWriter, allowed string) {
	w.Header().Set("Allow", allowed)
	http.Error(w, "仅支持 "+allowed+" 方法", http.StatusMethodNotAllowed)
}
