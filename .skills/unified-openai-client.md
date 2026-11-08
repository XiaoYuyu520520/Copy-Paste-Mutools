# 技能：统一 OpenAI 兼容接口客户端（internal/ai）

适用：桌面浏览器版 AI 聊天（`internal/chat`）、Android AI 聊天（`internal/mobile`），
以及任何要接 OpenAI 兼容 / DeepSeek / Qwen / Ollama 等服务的地方。

## 为什么要有这个包

之前两个端各写了一套 SSE 解析：桌面端按空行切事件但**多行 `data:` 处理不完整**、
**丢掉了 `scanner.Err()`**（网络中断被当成正常结束）；Android 端则是另一套逐行解析。
同一个模型服务在两个端表现不一致。`internal/ai` 把协议部分统一，UI 层只负责显示。

## 用法

```go
err := ai.Stream(ctx,
    ai.Config{BaseURL: "https://api.openai.com/v1", APIKey: "sk-...", Model: "gpt-4o-mini", System: "你是助手"},
    []ai.Message{{Role: "user", Content: "你好"}},
    func(delta string) { /* 正文增量 */ },
    func(reasoning string) { /* 思维链增量，可为 nil */ },
    func(u ai.Usage) { /* token 用量，可为 nil */ },
)
```

可配置版：

```go
ai.StreamWithOptions(ctx, cfg, msgs, ai.Options{
    Timeout:           5 * time.Minute,
    IncludeUsage:      true,     // 请求 stream_options.include_usage
    FallbackNonStream: true,     // 上游忽略 stream 时回退解析普通 JSON
    Model:             "override-model",
}, onDelta, onReasoning, onUsage)
```

拉取模型列表（设置页「测试连接」）：`ai.ListModels(ctx, cfg)`。

## 客户端已处理的行为

| 场景 | 行为 |
| --- | --- |
| 一个事件内多行 `data:` | 按顺序拼接后解析（部分服务会把一段 JSON 换行输出） |
| 思维链字段名差异 | `delta.reasoning_content` 与 `delta.reasoning` 都支持 |
| 非流式响应 | `Content-Type` 不是 `text/event-stream` 时解析普通 JSON |
| 读取中断 | `scanner.Err()` 作为 error 返回，不会静默结束 |
| 用户取消 | 依赖 `ctx`（Android「停止」按钮、桌面端断连都走 context） |
| `[DONE]` / 注释行 / 非 JSON 负载 | 安全跳过，不中断整条流 |
| 上游错误块 | `{"error":{"message":...}}` 转成可展示的错误 |

## 消息整理

`ai.BuildMessages(system, msgs)`：系统提示词前置、跳过空消息、合并连续同角色、
非法角色归一为 `user`。桌面端的 `buildUpstreamMessages` 与它等价（保留给旧测试）。

## 加测试时的提醒

用 `httptest.NewServer` 模拟上游，`Content-Type: text/event-stream`，
注意「读取中断」这类用例要用 `http.Hijacker` 手动断开连接，
普通 `w.Write` 后返回会被 Go 的 httptest 自动补上结尾，测不出中断。
