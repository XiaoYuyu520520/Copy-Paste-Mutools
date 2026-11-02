package mobile

import (
	"context"
	"sort"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// chatPage 是手机端的 AI 对话页。
//
// 桌面版把对话界面交给浏览器（内嵌 RikkaHub 风格页面）；
// 手机端没有 WebView 组件，所以直接用 Fyne 控件实现，
// 通过流式回调逐个分片更新最后一条消息。
type chatPage struct {
	store *store
	root  fyne.CanvasObject

	msgBox  *fyne.Container
	scroll  *container.Scroll
	input   *widget.Entry
	sendBtn *widget.Button
	status  *widget.Label

	// mu 保护下方全部共享状态（历史、发送态、取消函数、代次）。
	mu      sync.Mutex
	history []chatMsg

	// streaming 为 true 表示有一个请求在途；
	// cancel 是它的取消函数；gen 是会话代次，
	// 每次发送/清空自增，旧请求完成后若代次不匹配则不再碰 UI 状态。
	streaming bool
	cancel    context.CancelFunc
	gen       int
}

func newChatPage(s *store) *chatPage {
	p := &chatPage{store: s}

	p.msgBox = container.NewVBox()
	p.scroll = container.NewVScroll(p.msgBox)

	p.input = widget.NewMultiLineEntry()
	p.input.SetPlaceHolder("输入消息…")
	p.input.Wrapping = fyne.TextWrapWord

	p.sendBtn = widget.NewButtonWithIcon("发送", theme.MailSendIcon(), p.toggleSend)
	p.status = widget.NewLabel("")

	clearBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), p.clear)
	top := container.NewBorder(nil, nil, widget.NewLabel("AI 对话"), container.NewHBox(clearBtn))

	bottom := container.NewBorder(nil, p.status, nil, p.sendBtn, p.input)

	p.root = container.NewBorder(top, bottom, nil, nil, p.scroll)
	return p
}

// toggleSend 在「发送」与「停止」之间切换。
//
// 发送状态在启动 goroutine 之前同步切换：连续快速点击只会启动一个请求，
// 其余点击走「停止」分支取消当前请求。
func (p *chatPage) toggleSend() {
	text := strings.TrimSpace(p.input.Text)

	p.mu.Lock()
	if p.streaming {
		cancel := p.cancel
		p.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		return
	}
	if text == "" {
		p.mu.Unlock()
		return
	}

	// 在锁内完成：追加历史、占位发送态、生成代次与取消函数。
	p.history = append(p.history, chatMsg{Role: "user", Content: text})
	history := make([]chatMsg, len(p.history))
	copy(history, p.history)
	p.streaming = true
	p.gen++
	gen := p.gen
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.mu.Unlock()

	fyne.Do(func() {
		p.input.SetText("")
		p.addBubble("我", text, true)
		p.status.SetText("正在思考…")
		p.sendBtn.SetText("停止")
		p.sendBtn.SetIcon(theme.CancelIcon())
	})

	go p.send(ctx, cancel, history, gen)
}

// send 发起一次对话请求。只在 toggleSend 确认独占后由 goroutine 调用。
func (p *chatPage) send(ctx context.Context, cancel context.CancelFunc, history []chatMsg, gen int) {
	defer cancel()

	// 助手气泡先占位，流式分片直接往里追加。
	//
	// fyne.Do 只能调度不返回值的函数，因此用 channel 把主线程创建的对象带回来；
	// 这里在 goroutine 里，阻塞等待不会卡住 UI。
	bubbleCh := make(chan fyne.CanvasObject, 1)
	fyne.Do(func() { bubbleCh <- p.addBubble("助手", "", false) })
	bubble := <-bubbleCh

	var sb strings.Builder
	cfg := p.store.AIConfig()

	err := streamChat(ctx, cfg, history,
		func(delta string) {
			sb.WriteString(delta)
			fyne.Do(func() { p.updateBubble(bubble, "助手", sb.String(), false) })
		},
		func(full string) {},
	)

	// 完成时校验代次：期间用户清空过会话/发起新请求，就不回写旧状态。
	current := false
	p.mu.Lock()
	if p.gen == gen {
		current = true
		p.streaming = false
		p.cancel = nil
		if ctx.Err() == nil && sb.Len() > 0 {
			p.history = append(p.history, chatMsg{Role: "assistant", Content: sb.String()})
		}
	}
	p.mu.Unlock()

	if !current {
		return
	}

	fyne.Do(func() {
		p.sendBtn.SetText("发送")
		p.sendBtn.SetIcon(theme.MailSendIcon())
		if err != nil {
			if ctx.Err() != nil {
				p.status.SetText("已停止")
			} else {
				p.status.SetText("出错：" + err.Error())
			}
			p.updateBubble(bubble, "助手", sb.String()+"\n\n[错误] "+err.Error(), false)
			return
		}
		p.status.SetText("")
	})
}

// addBubble 在消息列表末尾追加一个气泡，返回它的容器。
func (p *chatPage) addBubble(who, text string, user bool) fyne.CanvasObject {
	obj := bubbleContent(who, text, user)
	p.msgBox.Add(obj)
	p.msgBox.Refresh()
	p.scroll.ScrollToBottom()
	return obj
}

// updateBubble 原地更新气泡内容（流式追加时用）。
func (p *chatPage) updateBubble(obj fyne.CanvasObject, who, text string, user bool) {
	c, ok := obj.(*fyne.Container)
	if !ok {
		return
	}
	fresh := bubbleContent(who, text, user)
	c.Objects = fresh.(*fyne.Container).Objects
	c.Refresh()
	p.scroll.ScrollToBottom()
}

// bubbleContent 渲染一条消息：用户用普通文本，助手用 Markdown。
func bubbleContent(who, text string, user bool) fyne.CanvasObject {
	var body fyne.CanvasObject
	if user {
		l := widget.NewLabel(text)
		l.Wrapping = fyne.TextWrapWord
		body = l
	} else if strings.TrimSpace(text) == "" {
		body = widget.NewLabel("…")
	} else {
		body = widget.NewRichTextFromMarkdown(text)
	}
	return widget.NewCard(who, "", body)
}

// clear 清空当前会话。
func (p *chatPage) clear() {
	p.mu.Lock()
	if p.cancel != nil {
		p.cancel()
	}
	p.cancel = nil
	p.streaming = false
	p.gen++ // 让在途请求完成时不再回写状态
	p.history = nil
	p.mu.Unlock()

	fyne.Do(func() {
		p.msgBox.Objects = nil
		p.msgBox.Refresh()
		p.sendBtn.SetText("发送")
		p.sendBtn.SetIcon(theme.MailSendIcon())
		p.status.SetText("已清空会话")
	})
}

// 保持 sort 导入（后续会话排序会用到）。
var _ = sort.Strings
