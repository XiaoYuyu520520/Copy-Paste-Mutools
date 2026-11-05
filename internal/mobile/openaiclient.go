package mobile

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/ai"
	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/data"
)

// chatMsg 是发给上游的一条消息。
type chatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// streamChat 调用 OpenAI 兼容接口并把增量文本回调出去。
//
// 与桌面版共用 internal/ai 的客户端：SSE 解析、思维链字段兼容、
// 非流式回退、错误处理完全一致，两个端对同一个模型服务表现相同。
// 不同点：桌面版把页面交给浏览器渲染，这里直接驱动 Fyne 控件。
func streamChat(ctx context.Context, cfg data.AIConfig, msgs []chatMsg, onDelta func(string), onDone func(string)) error {
	if strings.TrimSpace(cfg.BaseURL) == "" || strings.TrimSpace(cfg.Model) == "" {
		return fmt.Errorf("请先在「更多 → AI 设置」里配置 API 地址与模型")
	}

	// 显式新建切片，避免 append 复用调用方底层数组、把 system 消息写进历史。
	converted := make([]ai.Message, 0, len(msgs))
	for _, m := range msgs {
		converted = append(converted, ai.Message{Role: m.Role, Content: m.Content})
	}

	var sb strings.Builder
	err := ai.StreamWithOptions(ctx,
		ai.Config{
			BaseURL: cfg.BaseURL,
			APIKey:  cfg.APIKey,
			Model:   cfg.Model,
			System:  cfg.System,
		},
		converted,
		ai.Options{
			Timeout:           5 * time.Minute,
			IncludeUsage:      true,
			FallbackNonStream: true,
		},
		func(delta string) {
			sb.WriteString(delta)
			onDelta(delta)
		},
		nil, // 移动端暂不单独展示思维链
		nil,
	)
	if err != nil {
		return err
	}
	onDone(sb.String())
	return nil
}
