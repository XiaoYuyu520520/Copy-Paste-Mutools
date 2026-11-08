package mobile

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/data"
	shared "github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/theme"
)

// MainApp 是移动端的主界面：底部 5 个导航页。
type MainApp struct {
	app   fyne.App
	win   fyne.Window
	store *store

	editor *editorPage
	chat   *chatPage
	scan   *scanPage
	paint  *paintPage
	more   *morePage

	stack   *fyne.Container // 5 个页面叠放，切换时 Show/Hide
	nav     *bottomNav
	saveBar *widget.Label // 全局保存状态（所有页面可见）
	pageIdx int
}

// navEntries 是底部导航的 5 个入口（顺序即显示顺序）。
var navEntries = []bottomNavEntry{
	{Icon: "edit", Text: "编辑"},
	{Icon: "chat", Text: "聊天"},
	{Icon: "scan", Text: "扫描"},
	{Icon: "paint", Text: "画板"},
	{Icon: "more", Text: "更多"},
}

// latestMainApp 是最近一次 New 构造的实例，仅供单元测试使用。
var latestMainApp *MainApp

// New 构建移动端主窗口。
func New(a fyne.App) fyne.Window {
	w := a.NewWindow("提示词工具箱")
	w.Resize(fyne.NewSize(400, 800))

	// 主题在 build 之前设置，避免首帧用默认主题再切换导致闪烁。
	tmp := &MainApp{app: a, win: w, store: newStore(a)}
	a.Settings().SetTheme(shared.NewTheme(tmp.store.Config().ThemeMode == "dark"))

	m := &MainApp{app: a, win: w, store: tmp.store}
	m.build()
	latestMainApp = m
	return w
}

// build 组装页面栈与底部导航。
func (m *MainApp) build() {
	m.editor = newEditorPage(m.store, m.win)
	m.chat = newChatPage(m.store, m.win)
	m.scan = newScanPage()
	m.paint = newPaintPage()
	m.more = newMorePage(m.store, m.app)

	m.stack = container.NewStack(
		m.editor.root,
		m.chat.root,
		m.scan.root,
		m.paint.root,
		m.more.root,
	)

	m.nav = newBottomNav(navEntries, m.switchPage)

	// 全局保存状态条：任何页面改动配置后，保存结果都在这里可见
	// （以前只有编辑页有反馈，聊天/扫描页保存失败用户看不到）。
	m.saveBar = widget.NewLabel("")
	m.saveBar.Alignment = fyne.TextAlignCenter
	m.store.SetSaveResultHandler(func(err error) {
		fyne.Do(func() { m.updateSaveBar(err) })
	})

	navBar := container.NewBorder(nil, nil, nil, nil, m.nav.View())
	root := container.NewBorder(
		nil,
		container.NewVBox(m.saveBar, navBar),
		nil, nil,
		m.stack,
	)

	m.win.SetContent(root)

	// 退出时确保配置落盘（Android 上进程可能被系统直接杀掉，
	// 因此编辑页本身也会用去抖保存兜底）。
	m.win.SetOnClosed(func() {
		_ = m.store.Save()
	})

	m.switchPage(0)
}

// switchPage 切换到第 i 个页面。
func (m *MainApp) switchPage(i int) {
	pages := []fyne.CanvasObject{
		m.editor.root, m.chat.root, m.scan.root, m.paint.root, m.more.root,
	}
	if i < 0 || i >= len(pages) {
		return
	}
	for idx, p := range pages {
		if idx == i {
			p.Show()
		} else {
			p.Hide()
		}
	}
	m.pageIdx = i
	m.nav.Select(i)
}

// updateSaveBar 刷新顶部保存状态条。
func (m *MainApp) updateSaveBar(err error) {
	if m.saveBar == nil {
		return
	}
	if err != nil {
		_, detail := m.store.SaveStatus()
		m.saveBar.SetText("保存失败：" + detail)
		m.saveBar.TextStyle = fyne.TextStyle{Bold: true}
		m.saveBar.Refresh()
		return
	}
	state, _ := m.store.SaveStatus()
	text := state.Label()
	if text == "" {
		m.saveBar.SetText("")
		m.saveBar.TextStyle = fyne.TextStyle{}
		m.saveBar.Refresh()
		return
	}
	m.saveBar.SetText(text)
	m.saveBar.TextStyle = fyne.TextStyle{}
	m.saveBar.Refresh()
}

// applyTheme 按配置切换亮/暗主题。
//
// 与桌面端共用 internal/theme 的调色板，两端观感一致。
func applyTheme(a fyne.App, cfg *data.Config) {
	a.Settings().SetTheme(shared.NewTheme(cfg.ThemeMode == "dark"))
}

// 编译期断言：确保 widget 包被使用（底栏与状态条依赖它）。
var _ = widget.NewLabel
var _ = theme.DocumentCreateIcon
