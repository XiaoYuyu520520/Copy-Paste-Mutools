package mobile

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/data"
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
}

// New 构建移动端主窗口。
func New(a fyne.App) fyne.Window {
	w := a.NewWindow("提示词工具箱")
	w.Resize(fyne.NewSize(400, 800))

	m := &MainApp{app: a, win: w, store: newStore(a)}
	m.build()
	return w
}

// build 组装底部导航。
func (m *MainApp) build() {
	cfg := m.store.Config()

	m.editor = newEditorPage(m.store)
	m.chat = newChatPage(m.store)
	m.scan = newScanPage()
	m.paint = newPaintPage()
	m.more = newMorePage(m.store, m.app)

	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon("编辑", theme.DocumentCreateIcon(), m.editor.root),
		container.NewTabItemWithIcon("聊天", theme.MailSendIcon(), m.chat.root),
		container.NewTabItemWithIcon("扫描", theme.SearchIcon(), m.scan.root),
		container.NewTabItemWithIcon("画板", theme.ContentAddIcon(), m.paint.root),
		container.NewTabItemWithIcon("更多", theme.MoreHorizontalIcon(), m.more.root),
	)
	// 手机上底部导航比顶部标签更符合拇指操作习惯。
	tabs.SetTabLocation(container.TabLocationBottom)

	m.win.SetContent(tabs)
	_ = cfg

	// 退出时确保配置落盘（Android 上进程可能被系统直接杀掉，
	// 因此编辑页本身也会用去抖保存兜底）。
	m.win.SetOnClosed(func() {
		_ = m.store.Save()
	})
}

// applyTheme 按配置切换亮/暗主题。
func applyTheme(a fyne.App, cfg *data.Config) {
	if cfg.ThemeMode == "dark" {
		a.Settings().SetTheme(theme.DarkTheme())
		return
	}
	a.Settings().SetTheme(theme.LightTheme())
}

// 编译期断言：确保 widget 包被使用（AppTabs 的图标标签依赖它）。
var _ = widget.NewLabel
