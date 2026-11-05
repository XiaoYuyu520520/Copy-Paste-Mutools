// Package ai 提供统一的 OpenAI 兼容接口客户端。
//
// 之前桌面端（internal/chat）与移动端（internal/mobile）各写了一套 SSE 解析：
//   - 桌面端按空行切分事件，但一个事件内的多行 data: 处理不完整，
//     且丢弃了 scanner.Err()，读取中断时用户看到的是「正常结束」；
//   - 移动端逐行解析，遇到多行 data: 或 [DONE] 前的空行行为又不一样。
//
// 结果是同一个模型服务在桌面和手机上的兼容表现不一致。
//
// 本包把「请求 + 解析」统一起来：
//   - SSE 按空行切分事件，事件内多行 data: 会按顺序拼接（兼容部分服务换行输出 JSON）；
//   - 同时支持 content / reasoning_content / reasoning 三种字段名；
//   - 非流式响应（上游忽略 stream 参数）自动回退；
//   - 读取错误（scanner.Err()）会作为错误返回，不再被静默吞掉；
//   - 支持取消（context）与用量（usage）统计。
//
// 桌面端与移动端只负责把回调内容渲染到界面。
package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Message 一条对话消息。
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Config 一次请求需要的上游配置。
type Config struct {
	BaseURL string // OpenAI 兼容根地址，例如 https://api.openai.com/v1
	APIKey  string
	Model   string
	System  string // 可选的系统提示词
}

// Effective 是否已配置可用。
func (c Config) Effective() bool {
	return strings.TrimSpace(c.BaseURL) != "" && strings.TrimSpace(c.Model) != ""
}

// Usage 上游返回的 token 用量。
type Usage struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
}

// Event 流式过程中回调的事件类型。
type Event struct {
	// Type 取值：delta（正文增量）/ reasoning（思维链增量）/ usage（用量）/ error（上游错误）。
	Type string
	// Content 增量文本（delta / reasoning 时有效）。
	Content string
	// Usage 用量信息（usage 时有效）。
	Usage Usage
}

// Event 类型常量。
const (
	EventDelta     = "delta"
	EventReasoning = "reasoning"
	EventUsage     = "usage"
	EventError     = "error"
)

// Options 控制一次流式请求的行为。
type Options struct {
	// Model 覆盖 Config.Model；为空时使用 Config.Model。
	Model string
	// System 覆盖 Config.System；为空时使用 Config.System。
	System string
	// Timeout 单次请求超时；0 表示不限制（由调用方的 context 控制）。
	Timeout time.Duration
	// IncludeUsage 是否请求上游返回 usage（部分服务不支持该字段）。
	IncludeUsage bool
	// FallbackNonStream 是否在上游忽略 stream 参数时回退解析普通 JSON。
	FallbackNonStream bool
}

// Handler 接收流式事件。返回错误可中断后续读取。
type Handler func(ev Event) error

// streamChunk 是 OpenAI 兼容流式响应的一个分片。
//
// 不同服务商字段命名略有差异，这里同时容纳：
//   - delta.content / message.content：正文
//   - delta.reasoning_content / delta.reasoning：思维链
type streamChunk struct {
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
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *Usage `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// Stream 发起一次流式请求并把增量内容回调给 onDelta。
//
// 参数：
//   - ctx：用于取消（移动端「停止」按钮与桌面端断连都会走这里）
//   - cfg：上游配置
//   - msgs：对话消息（不含系统提示词，系统提示词由 cfg.System 提供）
//   - onDelta：正文增量回调
//   - onReasoning：思维链增量回调，可为 nil
//   - onUsage：用量回调，可为 nil
//
// 返回的错误一定是可展示给用户的文本（已包含上游返回的原因）。
func Stream(
	ctx context.Context,
	cfg Config,
	msgs []Message,
	onDelta func(string),
	onReasoning func(string),
	onUsage func(Usage),
) error {
	return StreamWithOptions(ctx, cfg, msgs, Options{IncludeUsage: true, FallbackNonStream: true}, onDelta, onReasoning, onUsage)
}

// StreamWithOptions 是 Stream 的可配置版本。
func StreamWithOptions(
	ctx context.Context,
	cfg Config,
	msgs []Message,
	opts Options,
	onDelta func(string),
	onReasoning func(string),
	onUsage func(Usage),
) error {
	if !cfg.Effective() {
		return errors.New("请先配置 API 地址与模型")
	}

	model := strings.TrimSpace(opts.Model)
	if model == "" {
		model = strings.TrimSpace(cfg.Model)
	}
	system := opts.System
	if system == "" {
		system = cfg.System
	}

	body, err := json.Marshal(buildRequest(model, system, msgs, opts))
	if err != nil {
		return err
	}

	url := ChatCompletionsURL(cfg.BaseURL)
	reqCtx := ctx
	var cancel context.CancelFunc
	if opts.Timeout > 0 {
		reqCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if key := strings.TrimSpace(cfg.APIKey); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return upstreamError(resp)
	}

	isStream := strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "event-stream")
	if !isStream {
		if !opts.FallbackNonStream {
			return errors.New("上游未返回流式响应")
		}
		return readNonStream(resp.Body, onDelta, onReasoning, onUsage)
	}

	return readStream(resp.Body, onDelta, onReasoning, onUsage)
}

// readStream 解析 SSE 流。
//
// 事件按空行切分；同一个事件内的多行 data: 会被拼接（部分服务把一段 JSON
// 换行输出），这正是桌面端此前缺失的处理。
func readStream(body io.Reader, onDelta func(string), onReasoning func(string), onUsage func(Usage)) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	var data strings.Builder
	dispatch := func() error {
		payload := strings.TrimSpace(data.String())
		data.Reset()
		if payload == "" || payload == "[DONE]" {
			return nil
		}
		return handleChunk([]byte(payload), onDelta, onReasoning, onUsage)
	}

	for scanner.Scan() {
		line := scanner.Text()
		// 去掉行尾 \r（部分服务用 CRLF）
		line = strings.TrimSuffix(line, "\r")

		if strings.TrimSpace(line) == "" {
			// 空行 = 事件结束
			if err := dispatch(); err != nil {
				return err
			}
			continue
		}
		// 注释行（以 : 开头）忽略
		if strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			// event: / id: / retry: 等字段不携带负载
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data.Len() > 0 {
			data.WriteByte('\n')
		}
		data.WriteString(payload)
	}
	// 流末尾可能没有空行收尾
	if data.Len() > 0 {
		if err := dispatch(); err != nil {
			return err
		}
	}
	// 关键：读取错误必须反馈给调用方，否则网络中断会被当成「正常结束」。
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("读取流失败: %w", err)
	}
	return nil
}

// handleChunk 解析一个 JSON 分片并分发到各回调。
func handleChunk(raw []byte, onDelta func(string), onReasoning func(string), onUsage func(Usage)) error {
	var chunk streamChunk
	if err := json.Unmarshal(raw, &chunk); err != nil {
		// 单个分片解析失败不应中断整条流（部分服务会发心跳/非 JSON 负载）。
		return nil
	}
	if chunk.Error != nil && chunk.Error.Message != "" {
		return fmt.Errorf("上游错误: %s", chunk.Error.Message)
	}
	for _, c := range chunk.Choices {
		if text := reasonText(c.Delta.ReasoningContent, c.Delta.ReasoningAlt); text != "" && onReasoning != nil {
			onReasoning(text)
		}
		if c.Delta.Content != "" && onDelta != nil {
			onDelta(c.Delta.Content)
		}
	}
	if chunk.Usage != nil && onUsage != nil {
		u := *chunk.Usage
		if u.TotalTokens == 0 {
			u.TotalTokens = u.PromptTokens + u.CompletionTokens
		}
		onUsage(u)
	}
	return nil
}

// readNonStream 解析上游忽略 stream 参数时返回的普通 JSON。
func readNonStream(body io.Reader, onDelta func(string), onReasoning func(string), onUsage func(Usage)) error {
	raw, err := io.ReadAll(io.LimitReader(body, 64<<20))
	if err != nil {
		return fmt.Errorf("读取响应失败: %w", err)
	}
	var resp struct {
		Choices []struct {
			Message struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
				ReasoningAlt     string `json:"reasoning"`
			} `json:"message"`
		} `json:"choices"`
		Usage *Usage `json:"usage"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}
	if resp.Error != nil && resp.Error.Message != "" {
		return fmt.Errorf("上游错误: %s", resp.Error.Message)
	}
	if len(resp.Choices) == 0 {
		return errors.New("上游没有返回内容")
	}
	c := resp.Choices[0].Message
	if text := reasonText(c.ReasoningContent, c.ReasoningAlt); text != "" && onReasoning != nil {
		onReasoning(text)
	}
	if c.Content != "" && onDelta != nil {
		onDelta(c.Content)
	}
	if resp.Usage != nil && onUsage != nil {
		u := *resp.Usage
		if u.TotalTokens == 0 {
			u.TotalTokens = u.PromptTokens + u.CompletionTokens
		}
		onUsage(u)
	}
	return nil
}

// upstreamError 把上游的非 200 响应转成可读错误。
func upstreamError(resp *http.Response) error {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
	msg := fmt.Sprintf("上游返回 %d", resp.StatusCode)
	var ue struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(raw, &ue) == nil {
		if ue.Error.Message != "" {
			msg += ": " + ue.Error.Message
			return errors.New(msg)
		}
		if ue.Message != "" {
			msg += ": " + ue.Message
			return errors.New(msg)
		}
	}
	if text := strings.TrimSpace(string(raw)); text != "" {
		msg += ": " + text
	}
	return errors.New(msg)
}

// reasonText 兼容不同服务商对思维链字段的命名。
func reasonText(content, alt string) string {
	if strings.TrimSpace(content) != "" {
		return content
	}
	return alt
}

// buildRequest 组装请求体。
func buildRequest(model, system string, msgs []Message, opts Options) map[string]any {
	payload := map[string]any{
		"model":    model,
		"stream":   true,
		"messages": BuildMessages(system, msgs),
	}
	if opts.IncludeUsage {
		payload["stream_options"] = map[string]any{"include_usage": true}
	}
	return payload
}

// BuildMessages 把系统提示词前置，跳过空消息、合并连续同角色。
func BuildMessages(system string, msgs []Message) []Message {
	out := make([]Message, 0, len(msgs)+1)
	if strings.TrimSpace(system) != "" {
		out = append(out, Message{Role: "system", Content: system})
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
		out = append(out, Message{Role: role, Content: content})
	}
	return out
}

// ChatCompletionsURL 由根地址拼出 /chat/completions。
func ChatCompletionsURL(baseURL string) string {
	base := strings.TrimSpace(strings.TrimSuffix(baseURL, "/"))
	if strings.HasSuffix(base, "/chat/completions") {
		return base
	}
	return base + "/chat/completions"
}

// ModelsURL 由根地址拼出 /models。
func ModelsURL(baseURL string) string {
	base := strings.TrimSpace(strings.TrimSuffix(baseURL, "/"))
	if strings.HasSuffix(base, "/models") {
		return base
	}
	return base + "/models"
}

// ListModels 拉取上游可用模型列表（用于设置页测试连接）。
func ListModels(ctx context.Context, cfg Config) ([]string, error) {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return nil, errors.New("请先填写 API 地址")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ModelsURL(cfg.BaseURL), nil)
	if err != nil {
		return nil, err
	}
	if key := strings.TrimSpace(cfg.APIKey); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, upstreamError(resp)
	}
	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&body); err != nil {
		return nil, fmt.Errorf("解析模型列表失败: %w", err)
	}
	ids := make([]string, 0, len(body.Data))
	for _, m := range body.Data {
		if strings.TrimSpace(m.ID) != "" {
			ids = append(ids, m.ID)
		}
	}
	return ids, nil
}
