package mobile

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// streamFlushInterval 是流式回复的界面刷新间隔：
// 太快会频繁重建 Markdown 气泡，太慢则看起来卡顿。
const streamFlushInterval = 80 * time.Millisecond

// chatPage 是手机端的 AI 对话页。
//
// 会话与消息持久化到应用目录（chat-sessions.json），
// 退出/重启后自动恢复最近一次会话。
type chatPage struct {
	store *store
	win   fyne.Window
	root  fyne.CanvasObject

	msgBox     *fyne.Container
	scroll     *container.Scroll
	input      *widget.Entry
	sendBtn    *widget.Button
	status     *widget.Label
	sessionSel *widget.Select

	sessions *chatStore
	updating bool   // UI 线程内防止下拉框回调递归
	filter   string // 会话搜索关键字

	// mu 保护下方全部共享状态（历史、发送态、取消函数、代次、当前会话）。
	mu        sync.Mutex
	history   []chatMsg
	sessionID string

	streaming bool
	cancel    context.CancelFunc
	gen       int
}

func newChatPage(s *store, win fyne.Window) *chatPage {
	p := &chatPage{store: s, win: win}
	p.sessions = newChatStore(filepath.Dir(s.path))

	p.msgBox = container.NewVBox()
	p.scroll = container.NewVScroll(p.msgBox)

	p.input = widget.NewMultiLineEntry()
	p.input.SetPlaceHolder("输入消息…")
	p.input.Wrapping = fyne.TextWrapWord

	p.sendBtn = widget.NewButtonWithIcon("发送", theme.MailSendIcon(), p.toggleSend)
	p.status = widget.NewLabel("")

	p.sessionSel = widget.NewSelect(nil, func(string) {
		if p.updating {
			return
		}
		idx := p.sessionSel.SelectedIndex()
		list, _ := p.sessions.SearchSessions(p.filter)
		if idx < 0 || idx >= len(list) {
			return
		}
		p.openSession(list[idx].ID)
	})

	search := widget.NewEntry()
	search.SetPlaceHolder("搜索会话标题或内容")
	search.OnChanged = func(text string) {
		p.filter = text
		p.refreshSessions()
	}

	// 会话管理收进「⋯」菜单：新建 / 重命名 / 置顶 / 清空 / 删除
	// 五个图标按钮平铺在标题栏上太挤，也容易误触删除。
	var menuBtn *widget.Button
	menuBtn = widget.NewButtonWithIcon("", theme.MoreHorizontalIcon(), func() {
		p.showSessionMenu(menuBtn)
	})
	menuBtn.Importance = widget.LowImportance
	top := container.NewBorder(nil, nil,
		widget.NewLabelWithStyle("AI 对话", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewHBox(menuBtn),
		p.sessionSel,
	)

	bottom := container.NewBorder(nil, p.status, nil, p.sendBtn, p.input)

	p.root = container.NewBorder(container.NewVBox(top, search), bottom, nil, nil, p.scroll)
	p.loadActive()
	p.refreshSessions()
	return p
}

// showSessionMenu 弹出会话管理菜单。
func (p *chatPage) showSessionMenu(anchor fyne.CanvasObject) {
	if p.win == nil {
		return
	}
	pinned := p.isPinned()
	pinLabel := "置顶会话"
	if pinned {
		pinLabel = "取消置顶"
	}
	menu := fyne.NewMenu("",
		fyne.NewMenuItem("新建会话", p.newSession),
		fyne.NewMenuItem("重命名会话", p.renameSession),
		fyne.NewMenuItem(pinLabel, p.togglePin),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("清空当前会话", p.clear),
		fyne.NewMenuItem("删除当前会话", p.deleteSession),
	)
	pop := widget.NewPopUpMenu(menu, p.win.Canvas())
	pop.Resize(pop.MinSize())
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(anchor)
	below := pos.AddXY(0, anchor.Size().Height)
	if below.Y+pop.MinSize().Height > p.win.Canvas().Size().Height {
		below = fyne.NewPos(pos.X, pos.Y-pop.MinSize().Height)
	}
	pop.ShowAtPosition(below)
}

// isPinned 返回当前会话是否已置顶。
func (p *chatPage) isPinned() bool {
	p.mu.Lock()
	id := p.sessionID
	p.mu.Unlock()
	if id == "" {
		return false
	}
	list, _ := p.sessions.SearchSessions(p.filter)
	for _, s := range list {
		if s.ID == id {
			return s.Pinned
		}
	}
	return false
}

// refreshSessions 重建会话下拉框。
func (p *chatPage) refreshSessions() {
	list, active := p.sessions.SearchSessions(p.filter)
	opts := make([]string, len(list))
	activeIdx := 0
	for i, s := range list {
		title := s.Title
		if title == "" {
			title = "新会话"
		}
		// 置顶会话加标记，便于在列表里一眼分辨。
		if s.Pinned {
			title = "★ " + title
		}
		// 附上相对时间，帮助区分同名会话。
		title += " · " + relativeTime(s.UpdatedAt)
		opts[i] = fmt.Sprintf("%s", title)
		if s.ID == active {
			activeIdx = i
		}
	}
	p.updating = true
	p.sessionSel.Options = opts
	p.sessionSel.SetSelectedIndex(activeIdx)
	p.updating = false
}

// loadActive 把当前会话消息渲染到界面。
func (p *chatPage) loadActive() {
	s := p.sessions.Active()
	p.mu.Lock()
	p.sessionID = s.ID
	p.history = append([]chatMsg(nil), s.Messages...)
	p.mu.Unlock()

	p.msgBox.Objects = nil
	for _, m := range s.Messages {
		if m.Role == "assistant" {
			p.addBubble("助手", m.Content, false)
		} else if m.Role == "user" {
			p.addBubble("我", m.Content, true)
		}
	}
	p.msgBox.Refresh()
	p.status.SetText("")
}

// openSession 切换会话（必要时先取消在途请求）。
func (p *chatPage) openSession(id string) {
	if id == "" {
		return
	}
	p.mu.Lock()
	if p.sessionID == id {
		p.mu.Unlock()
		return
	}
	if p.cancel != nil {
		p.cancel()
	}
	p.cancel = nil
	p.streaming = false
	p.gen++ // 旧请求完成后不再回写 UI
	p.mu.Unlock()

	p.sessions.SetActive(id)
	p.loadActive()
	p.refreshSessions()
}

func (p *chatPage) newSession() {
	p.mu.Lock()
	if p.cancel != nil {
		p.cancel()
	}
	p.cancel = nil
	p.streaming = false
	p.gen++
	p.mu.Unlock()

	p.sessions.New()
	p.loadActive()
	p.refreshSessions()
}

func (p *chatPage) renameSession() {
	p.mu.Lock()
	id := p.sessionID
	p.mu.Unlock()
	if id == "" {
		return
	}
	current := p.sessions.Active()
	dialog.ShowEntryDialog("重命名会话", "输入新的会话名称", func(text string) {
		text = strings.TrimSpace(text)
		if text == "" {
			return
		}
		p.sessions.Rename(id, text)
		p.refreshSessions()
	}, p.win)
	p.status.SetText("当前会话：" + current.Title)
}

// relativeTime 把更新时间转成「刚刚 / N 分钟前 / N 小时前 / N 天前」。
func relativeTime(t time.Time) string {
	if t.IsZero() {
		return "未知"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "刚刚"
	case d < time.Hour:
		return fmt.Sprintf("%d 分钟前", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d 小时前", int(d.Hours()))
	default:
		return fmt.Sprintf("%d 天前", int(d.Hours()/24))
	}
}

// togglePin 置顶 / 取消置顶当前会话。
func (p *chatPage) togglePin() {
	p.mu.Lock()
	id := p.sessionID
	p.mu.Unlock()
	if id == "" {
		return
	}
	p.sessions.TogglePin(id)
	p.refreshSessions()
}

func (p *chatPage) deleteSession() {
	p.mu.Lock()
	id := p.sessionID
	p.mu.Unlock()
	if id == "" {
		return
	}
	dialog.ShowConfirm("删除会话", "确认删除当前会话？此操作不可恢复。", func(ok bool) {
		if !ok {
			return
		}
		p.mu.Lock()
		if p.cancel != nil {
			p.cancel()
		}
		p.cancel = nil
		p.streaming = false
		p.gen++
		p.mu.Unlock()
		p.sessions.Delete(id)
		p.loadActive()
		p.refreshSessions()
	}, p.win)
}

// toggleSend 在「发送」与「停止」之间切换。
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

	sid := p.sessionID
	p.history = append(p.history, chatMsg{Role: "user", Content: text})
	p.sessions.Append(sid, chatMsg{Role: "user", Content: text})
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
	p.refreshSessions()

	go p.send(ctx, cancel, history, gen, sid)
}

// send 发起一次对话请求。只在 toggleSend 确认独占后由 goroutine 调用。
//
// 流式渲染做了两层保护：
//  1. 后台只往自己的缓冲区追加，UI 回调读取的是「不可变快照」；
//  2. UI 刷新按 streamFlushInterval 合并，避免每个 token 都重建一次 Markdown。
func (p *chatPage) send(ctx context.Context, cancel context.CancelFunc, history []chatMsg, gen int, sid string) {
	defer cancel()

	bubbleCh := make(chan fyne.CanvasObject, 1)
	fyne.Do(func() { bubbleCh <- p.addBubble("助手", "", false) })
	bubble := <-bubbleCh

	var (
		sb      strings.Builder
		sbMu    sync.Mutex
		lastRaw string // 上一次已提交给 UI 的文本
	)

	// flush 把当前缓冲区内容作为不可变快照提交给 UI。
	flush := func(force bool) {
		sbMu.Lock()
		text := sb.String()
		sbMu.Unlock()
		if !force && text == lastRaw {
			return
		}
		lastRaw = text
		snapshot := text
		fyne.Do(func() { p.updateBubble(bubble, "助手", snapshot, false) })
	}

	cfg := p.store.AIConfig()

	// 合并刷新：后台定时把缓冲区内容刷到界面。
	stopFlush := make(chan struct{})
	flushDone := make(chan struct{})
	go func() {
		defer close(flushDone)
		ticker := time.NewTicker(streamFlushInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stopFlush:
				return
			case <-ticker.C:
				flush(false)
			}
		}
	}()

	err := streamChat(ctx, cfg, history,
		func(delta string) {
			sbMu.Lock()
			sb.WriteString(delta)
			sbMu.Unlock()
		},
		func(full string) {},
	)

	close(stopFlush)
	<-flushDone
	flush(true)

	sbMu.Lock()
	finalText := sb.String()
	sbMu.Unlock()
	if err == nil && ctx.Err() == nil && finalText != "" {
		p.sessions.Append(sid, chatMsg{Role: "assistant", Content: finalText})
	}

	current := false
	p.mu.Lock()
	if p.gen == gen {
		current = true
		p.streaming = false
		p.cancel = nil
		if ctx.Err() == nil && finalText != "" {
			p.history = append(p.history, chatMsg{Role: "assistant", Content: finalText})
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
			p.updateBubble(bubble, "助手", finalText+"\n\n[错误] "+err.Error(), false)
			return
		}
		p.status.SetText("")
	})
	p.refreshSessions()
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
	p.gen++
	sid := p.sessionID
	p.history = nil
	p.mu.Unlock()

	p.sessions.SetMessages(sid, nil)

	fyne.Do(func() {
		p.msgBox.Objects = nil
		p.msgBox.Refresh()
		p.sendBtn.SetText("发送")
		p.sendBtn.SetIcon(theme.MailSendIcon())
		p.status.SetText("已清空会话")
	})
}
