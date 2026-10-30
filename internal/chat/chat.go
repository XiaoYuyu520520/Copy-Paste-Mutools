// Package chat 提供局域网浏览器版 AI 聊天：OpenAI 兼容接口 + SSE 流式转发。
//
// 视觉与交互参考 RikkaHub 嵌入 Web UI 的主对话形态：
// 左侧会话列表 + 顶部模型切换 + 消息气泡 / Markdown / 思维链 + 底部输入框。
// 纯离线：marked / highlight.js / github 样式复用 render 包内嵌资源。
package chat

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"sync"
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
	return os.WriteFile(histStore.path, raw, 0o644)
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

type modelItem struct {
	ID string `json:"id"`
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
			upstream, err := request(r, http.MethodGet, joinURL(cfg.BaseURL, "/models"), cfg, nil)
			if err == nil && upstream.StatusCode == http.StatusOK {
				var body struct {
					Data []modelItem `json:"data"`
				}
				_ = json.NewDecoder(io.LimitReader(upstream.Body, 4<<20)).Decode(&body)
				_ = upstream.Body.Close()
				for _, m := range body.Data {
					if strings.TrimSpace(m.ID) != "" {
						ids = append(ids, m.ID)
					}
				}
			} else if upstream != nil {
				_ = upstream.Body.Close()
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

type upstreamError struct {
	Message string `json:"message"`
}

// streamEvent 返回给页面的统一事件。
type streamEvent struct {
	Type             string `json:"type"` // delta / reasoning / usage / error / done
	Content          string `json:"content,omitempty"`
	PromptTokens     int64  `json:"prompt_tokens,omitempty"`
	CompletionTokens int64  `json:"completion_tokens,omitempty"`
}

type usageInfo struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
}

type upstreamChunk struct {
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
			ReasoningAlt     string `json:"reasoning"`
		} `json:"delta"`
		Message struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
			ReasoningAlt     string `json:"reasoning"`
		} `json:"message"`
	} `json:"choices"`
	Usage *usageInfo     `json:"usage"`
	Error *upstreamError `json:"error"`
}

// handleChat 接收前端请求并把上游 OpenAI 兼容流逐块转发为 SSE。
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

		model := strings.TrimSpace(req.Model)
		if model == "" {
			model = cfg.Model
		}
		payload := map[string]any{
			"model":    model,
			"stream":   true,
			"messages": buildUpstreamMessages(cfg.System, req.Messages),
			// 请求 usage，便于页面统计 token 速度
			"stream_options": map[string]any{"include_usage": true},
		}
		body, err := json.Marshal(payload)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		upstream, err := request(r, http.MethodPost, joinURL(cfg.BaseURL, "/chat/completions"), cfg, body)
		if err != nil {
			http.Error(w, "连接模型服务失败: "+err.Error(), http.StatusBadGateway)
			return
		}
		defer upstream.Body.Close()

		if upstream.StatusCode != http.StatusOK {
			msg := fmt.Sprintf("模型服务返回 %d", upstream.StatusCode)
			raw, _ := io.ReadAll(io.LimitReader(upstream.Body, 8<<10))
			if len(raw) > 0 {
				var ue upstreamError
				if json.Unmarshal(raw, &ue) == nil && ue.Message != "" {
					msg += "：" + ue.Message
				} else {
					msg += "：" + strings.TrimSpace(string(raw))
				}
			}
			http.Error(w, msg, http.StatusBadGateway)
			return
		}

		isStream := strings.Contains(strings.ToLower(upstream.Header.Get("Content-Type")), "event-stream")
		streamToClient(w, upstream.Body, isStream)
	}
}

// streamToClient 把上游响应转成页面可读的 SSE。
func streamToClient(w http.ResponseWriter, body io.Reader, isStream bool) {
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

	reader := bufio.NewReader(body)
	if isStream {
		forwardSSE(reader, send)
		return
	}

	// 兜底：上游忽略 stream 直接返回普通 JSON
	all, _ := io.ReadAll(reader)
	var resp upstreamChunk
	if json.Unmarshal(all, &resp) == nil {
		if resp.Error != nil && resp.Error.Message != "" {
			send(streamEvent{Type: "error", Content: resp.Error.Message})
			return
		}
		for _, c := range resp.Choices {
			if text := reasonText(c.Message.ReasoningContent, c.Message.ReasoningAlt); text != "" {
				send(streamEvent{Type: "reasoning", Content: text})
			}
			if c.Message.Content != "" {
				send(streamEvent{Type: "delta", Content: c.Message.Content})
			}
		}
		if resp.Usage != nil {
			send(streamEvent{Type: "usage",
				PromptTokens: resp.Usage.PromptTokens, CompletionTokens: resp.Usage.CompletionTokens})
		}
		return
	}
	send(streamEvent{Type: "error", Content: "上游返回无法解析: " + strings.TrimSpace(string(all))})
}

// forwardSSE 按空行切分并解析上游事件流，转发 reasoning / content / usage。
func forwardSSE(reader *bufio.Reader, send func(streamEvent) bool) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	var buf bytes.Buffer

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			if buf.Len() == 0 {
				continue
			}
			data := strings.TrimSpace(strings.TrimPrefix(buf.String(), "data:"))
			buf.Reset()
			if data == "" || data == "[DONE]" {
				continue
			}
			var chunk upstreamChunk
			if json.Unmarshal([]byte(data), &chunk) != nil {
				continue
			}
			if chunk.Error != nil && chunk.Error.Message != "" {
				send(streamEvent{Type: "error", Content: chunk.Error.Message})
				return
			}
			for _, c := range chunk.Choices {
				if text := reasonText(c.Delta.ReasoningContent, c.Delta.ReasoningAlt); text != "" &&
					!send(streamEvent{Type: "reasoning", Content: text}) {
					return
				}
				if c.Delta.Content != "" &&
					!send(streamEvent{Type: "delta", Content: c.Delta.Content}) {
					return
				}
			}
			if chunk.Usage != nil && len(chunk.Choices) == 0 &&
				!send(streamEvent{Type: "usage",
					PromptTokens: chunk.Usage.PromptTokens, CompletionTokens: chunk.Usage.CompletionTokens}) {
				return
			}
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), "data:") {
			buf.WriteString(strings.TrimSpace(line))
			buf.WriteByte('\n')
		}
	}
}

// reasonText 兼容不同服务商对思维链字段的命名。
func reasonText(content, alt string) string {
	if strings.TrimSpace(content) != "" {
		return content
	}
	return alt
}

// buildUpstreamMessages 把系统提示词前置，跳过空消息、合并连续同角色。
func buildUpstreamMessages(system string, msgs []chatMessage) []chatMessage {
	out := make([]chatMessage, 0, len(msgs)+1)
	if strings.TrimSpace(system) != "" {
		out = append(out, chatMessage{Role: "system", Content: system})
	}
	for _, m := range msgs {
		role := m.Role
		if role != "user" && role != "assistant" && role != "system" {
			role = "user"
		}
		content := strings.TrimSpace(m.Content)
		if content == "" {
			continue
		}
		if n := len(out); n > 0 && out[n-1].Role == role {
			out[n-1].Content += "\n" + content
			continue
		}
		out = append(out, chatMessage{Role: role, Content: content})
	}
	return out
}

func request(r *http.Request, method, url string, cfg Settings, body []byte) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(r.Context(), method, url, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(cfg.APIKey) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(cfg.APIKey))
	}
	return (&http.Client{}).Do(req)
}

// joinURL 把 base 与相对路径拼接，容忍结尾斜杠与重复路径。
func joinURL(base, path string) string {
	base = strings.TrimSpace(strings.TrimSuffix(base, "/"))
	path = strings.TrimPrefix(path, "/")
	if strings.HasSuffix(base, path) {
		return base
	}
	return base + "/" + path
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
