package mobile

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/data"
)

// chatMsg 是发给上游的一条消息。
type chatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// streamRequest 是 OpenAI 兼容的流式请求体。
type streamRequest struct {
	Model    string    `json:"model"`
	Messages []chatMsg `json:"messages"`
	Stream   bool      `json:"stream"`
}

// streamChunk 是流式响应的一个分片（只取需要字段）。
type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// streamChat 调用 OpenAI 兼容接口并把增量文本回调出去。
//
// 与桌面版共同点：一样的请求格式、一样的 SSE 解析规则。
// 不同点：桌面版把页面交给浏览器渲染，这里直接驱动 Fyne 控件。
func streamChat(ctx context.Context, cfg data.AIConfig, msgs []chatMsg, onDelta func(string), onDone func(string)) error {
	if strings.TrimSpace(cfg.BaseURL) == "" || strings.TrimSpace(cfg.Model) == "" {
		return fmt.Errorf("请先在「更多 → AI 设置」里配置 API 地址与模型")
	}

	if cfg.System != "" {
		// 显式新建切片，避免 append 复用调用方底层数组、把 system 消息写进历史。
		withSystem := make([]chatMsg, 0, len(msgs)+1)
		withSystem = append(withSystem, chatMsg{Role: "system", Content: cfg.System})
		withSystem = append(withSystem, msgs...)
		msgs = withSystem
	}

	body, err := json.Marshal(streamRequest{Model: cfg.Model, Messages: msgs, Stream: true})
	if err != nil {
		return err
	}

	url := strings.TrimRight(cfg.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// 上游错误一般是一段 JSON，尽量把 message 抽出来给人看。
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		var e struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(raw, &e) == nil && e.Error.Message != "" {
			return fmt.Errorf("上游返回 %d: %s", resp.StatusCode, e.Error.Message)
		}
		return fmt.Errorf("上游返回 %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	// 非流式响应（部分兼容服务忽略 stream 参数）也要能处理。
	if !strings.Contains(resp.Header.Get("Content-Type"), "event-stream") {
		raw, _ := io.ReadAll(resp.Body)
		var full struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(raw, &full); err != nil {
			return fmt.Errorf("解析响应失败: %w", err)
		}
		if len(full.Choices) == 0 {
			return fmt.Errorf("上游没有返回内容")
		}
		text := full.Choices[0].Message.Content
		onDelta(text)
		onDone(text)
		return nil
	}

	var sb strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			break
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		if chunk.Error != nil && chunk.Error.Message != "" {
			return fmt.Errorf("上游错误: %s", chunk.Error.Message)
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		if delta := chunk.Choices[0].Delta.Content; delta != "" {
			sb.WriteString(delta)
			onDelta(delta)
		}
	}
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		return fmt.Errorf("读取流失败: %w", err)
	}

	onDone(sb.String())
	return nil
}
