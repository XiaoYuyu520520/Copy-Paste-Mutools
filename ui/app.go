package ui

import (
	"fmt"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"prompt-merger-native/chat"
	"prompt-merger-native/data"
	"prompt-merger-native/render"
)

const saveDebounce = 1500 * time.Millisecond

type tabBundle struct {
	editor *TabEditor
}

// 左侧页面索引
const (
	featEditor = iota // 多标签编辑器
	featSim           // 模拟键盘输入
	featTree          // 文件夹 Tree
	featAISet         // AI 设置（API/模型）
)

// MainApp 持有整个窗口与所有 UI 状态。
//
// 布局树（参考用户文档）：
//
//	Window
//	├── body             (Border.Center)
//	│   ├── leftRail      (Border.Left)   竖向页面切换（替代原历史栏）
//	│   └── contentStack  (Border.Center) 编辑器/模拟输入/文件夹 Tree/AI 设置 层叠
//	│       ├── mainWorkspace：tabBar(顶) + editorHost(多标签编辑器)
//	│       ├── simPanel
//	│       └── treePanel
//	└── statusBar        (Border.Bottom) 状态文本 + [AI 聊天][Markdown渲染][局域网扫描] + 主题/标签数
type MainApp struct {
	a   fyne.App
	win fyne.Window
	cfg *data.Config

	bundles []*tabBundle
	tabBar  *customTabBar

	rail       *LeftRail
	status     *widget.Label
	countLabel *widget.Label
	configBtn  *widget.Button
	contentBox *fyne.Container // 多标签编辑器 Stack，只 Show 激活标签的 editor

	simPanel  *SimInputPanel
	treePanel *TreePanel
	aiPanel   *AISettingsPanel
	mainWS    fyne.CanvasObject // 编辑器工作区（Border: tabBar + contentBox）

	lanTool *BrowserTool // 局域网设备扫描（浏览器版）
	mdTool  *BrowserTool // Markdown 渲染（浏览器版）
	aiTool  *BrowserTool // AI 聊天（浏览器版，OpenAI 兼容流式）

	// mdDocs 是 Markdown 渲染页的文档源：原生窗口拖入 .md/.pgm 后写入这里，
	// 已打开的页面通过 SSE 热切换到新文档。
	mdDocs *render.DocStore

	aiMu  sync.RWMutex // 保护 aiSettings
	aiCfg chat.Settings

	feature int // 当前左侧页面：featEditor / featSim / featTree / featAISet

	timerMu   sync.Mutex
	saveTimer *time.Timer
}

func NewMainWindow(a fyne.App, cfg *data.Config) fyne.Window {
	m := &MainApp{
		a:       a,
		cfg:     cfg,
		status:  widget.NewLabel("● 就绪"),
		feature: featEditor,
	}
	if cfg.AI != nil {
		m.aiCfg = chat.Settings{
			BaseURL: cfg.AI.BaseURL,
			APIKey:  cfg.AI.APIKey,
			Model:   cfg.AI.Model,
			System:  cfg.AI.System,
		}
	}

	dark := cfg.ThemeMode == "dark"
	a.Settings().SetTheme(NewModernTheme(dark))

	win := a.NewWindow("提示词+语句自动合并工具 · Pro (Native)")
	win.Resize(fyne.NewSize(1240, 820))
	win.CenterOnScreen()
	m.win = win

	// ---- 左侧竖向页面切换（替代历史栏） ----
	m.rail = NewLeftRail([]string{"编辑器", "模拟键盘输入", "文件夹 Tree", "AI 设置"}, m.onRailPick)

	// ---- 多标签编辑器 ----
	m.tabBar = NewCustomTabBar(m.onTabSelect)
	m.tabBar.SetCloseHandler(m.confirmCloseTab)

	m.contentBox = container.NewStack()
	if len(m.cfg.Tabs) == 0 {
		m.cfg.Tabs = []data.TabConfig{blankTabConfig()}
	}
	for i, tabCfg := range m.cfg.Tabs {
		m.addBundle(fmt.Sprintf("标签 %d", i+1), tabCfg)
	}
	if m.cfg.ActiveTab < 0 || m.cfg.ActiveTab >= len(m.bundles) {
		m.cfg.ActiveTab = 0
	}
	m.tabBar.SetBundles(m.bundles)
	m.tabBar.SetSelected(m.cfg.ActiveTab)
	m.selectTab(m.cfg.ActiveTab)

	m.mainWS = container.NewBorder(m.tabBar.View(), nil, nil, nil, m.contentBox)

	m.simPanel = NewSimInputPanel(func() { m.selectFeature(featEditor) })
	m.treePanel = NewTreePanel(m.win, func() { m.selectFeature(featEditor) })
	m.aiPanel = NewAISettingsPanel(m.win, func() { m.selectFeature(featEditor) })
	m.aiPanel.OnSave = m.onSaveAI
	m.aiPanel.Set(m.chatSettings())

	contentStack := container.NewStack(m.mainWS, m.simPanel.View(), m.treePanel.View(), m.aiPanel.View())

	// ---- 浏览器工具（局域网扫描 / Markdown 渲染 / AI 聊天） ----
	m.lanTool = newLanScanTool()
	m.mdDocs = render.NewDocStore()
	m.mdTool = newMarkdownTool(m.mdDocs)
	m.aiTool = newAiChatTool(m.chatSettings)

	// ---- 底部状态栏：左 = [主题按钮 + 状态文本]，右 = [AI 聊天 / Markdown / LAN] + 标签数 ----
	m.countLabel = widget.NewLabelWithStyle(
		fmt.Sprintf("共 %d 标签", len(m.bundles)),
		fyne.TextAlignTrailing,
		fyne.TextStyle{Italic: true},
	)

	// 左下角「⚙ 配置」入口：弹菜单里集中放主题切换与立即保存。
	configBtn := widget.NewButtonWithIcon("配置", theme.SettingsIcon(), m.openConfigMenu)
	configBtn.Importance = widget.LowImportance
	m.configBtn = configBtn

	// 右侧三个浏览器按钮都带图标 + 短文本。
	aiBtn := widget.NewButtonWithIcon("AI 聊天", theme.MailSendIcon(), m.openAiChat)
	aiBtn.Importance = widget.MediumImportance
	mdBtn := widget.NewButtonWithIcon("Markdown", theme.DocumentIcon(), m.openMarkdown)
	mdBtn.Importance = widget.MediumImportance
	lanBtn := widget.NewButtonWithIcon("LAN", theme.ComputerIcon(), m.openLanScan)
	lanBtn.Importance = widget.MediumImportance

	statusBar := container.NewBorder(
		nil, nil,
		// 左侧：⚙ 配置 + 状态文本
		container.NewHBox(
			container.NewPadded(configBtn),
			container.NewPadded(m.status),
		),
		// 右侧：浏览器工具 + 标签数
		container.NewHBox(
			container.NewPadded(aiBtn),
			container.NewPadded(mdBtn),
			container.NewPadded(lanBtn),
			container.NewPadded(m.countLabel),
		),
		nil,
	)

	// ---- 整体主体：左竖向导航 + 右内容层叠 ----
	body := container.NewBorder(nil, nil, container.NewPadded(m.rail.View()), nil, contentStack)

	win.SetContent(container.NewBorder(
		nil,
		statusBar,
		nil, nil,
		body,
	))

	win.SetOnClosed(func() { m.flushAll() })
	win.SetOnDropped(m.onFilesDropped)

	m.selectFeature(featEditor)

	// 初次进入强制合并一次
	fyne.Do(func() {
		if idx := m.tabBar.Selected(); idx >= 0 && idx < len(m.bundles) {
			m.bundles[idx].editor.ForceMerge()
			m.updateTabCount()
			m.setStatus(fmt.Sprintf("已加载 %d 个标签", len(m.bundles)))
		}
	})

	return win
}

// ---------- 左侧页面切换 ----------

// onRailPick 由左侧导航触发页面切换。
func (m *MainApp) onRailPick(i int) {
	m.selectFeature(i)
}

// selectFeature 切换左侧整页功能之一；离开模拟输入页时先停止可能正在进行的键入。
func (m *MainApp) selectFeature(i int) {
	if i < featEditor || i > featAISet {
		return
	}
	if m.feature == featSim && i != featSim && m.simPanel != nil {
		m.simPanel.stopTyping()
	}
	m.feature = i

	if m.mainWS != nil {
		m.mainWS.Hide()
	}
	if m.simPanel != nil {
		m.simPanel.View().Hide()
	}
	if m.treePanel != nil {
		m.treePanel.View().Hide()
	}
	if m.aiPanel != nil {
		m.aiPanel.View().Hide()
	}
	if m.rail != nil {
		m.rail.Select(i)
	}

	switch i {
	case featEditor:
		if m.mainWS != nil {
			m.mainWS.Show()
		}
		m.setStatus(fmt.Sprintf("已返回编辑器（共 %d 个标签）", len(m.bundles)))
	case featSim:
		active := -1
		if idx := m.tabBar.Selected(); idx >= 0 && idx < len(m.bundles) {
			active = idx
		}
		m.simPanel.SetBundles(m.bundles, active)
		m.simPanel.View().Show()
		m.setStatus("模拟键盘输入：选来源标签 → 调整参数 → 点「开始输入」")
	case featTree:
		m.treePanel.View().Show()
		m.setStatus("文件夹 Tree：选一个文件夹生成目录结构")
	case featAISet:
		m.aiPanel.Set(m.chatSettings())
		m.aiPanel.View().Show()
		m.setStatus("AI 设置：填 API 地址 / Key / 模型，保存后点底部「AI 聊天」")
	}
}

// ---------- AI 设置 / 聊天 ----------

// chatSettings 读取当前 AI 设置（供设置页与浏览器聊天服务安全并发读取）。
func (m *MainApp) chatSettings() chat.Settings {
	m.aiMu.RLock()
	defer m.aiMu.RUnlock()
	return m.aiCfg
}

// onSaveAI 从 AI 设置页保存到内存并落盘 config.json。
func (m *MainApp) onSaveAI(s chat.Settings) {
	m.aiMu.Lock()
	m.aiCfg = s
	if m.cfg.AI == nil {
		m.cfg.AI = &data.AIConfig{}
	}
	m.cfg.AI.BaseURL = s.BaseURL
	m.cfg.AI.APIKey = s.APIKey
	m.cfg.AI.Model = s.Model
	m.cfg.AI.System = s.System
	m.aiMu.Unlock()
	if err := data.SaveConfig(*m.cfg); err != nil {
		m.setStatus("AI 设置保存失败：" + err.Error())
		return
	}
}

// openAiChat 打开浏览器版 AI 聊天；未配置时提示先去「AI 设置」页。
func (m *MainApp) openAiChat() {
	if m.aiTool == nil {
		m.setStatus("AI 聊天服务不可用")
		return
	}
	if !m.chatSettings().Effective() {
		m.setStatus("请先在左侧「AI 设置」页配置 API 地址与模型")
		m.selectFeature(featAISet)
		return
	}
	m.openBrowserTool(m.aiTool, "AI 聊天")
}

// ---------- 多标签管理 ----------

func (m *MainApp) addBundle(title string, cfg data.TabConfig) {
	editor := NewTabEditor(title, cfg, m.win)
	editor.OnMerged = func(text string) {
		if text == "__copied__" {
			m.setStatus("已复制 ✓")
			return
		}
		m.setStatus(fmt.Sprintf("合并 %d 行 · %s", editor.ActiveCount(), nowStr()))
	}
	editor.OnDirty = func() { m.scheduleSaveConfig() }
	editor.OnDirtyNow = func() { m.saveConfigNow() }

	bundle := &tabBundle{editor: editor}
	m.bundles = append(m.bundles, bundle)

	view := editor.View()
	m.contentBox.Objects = append(m.contentBox.Objects, view)
	view.Hide()
}

func (m *MainApp) selectTab(idx int) {
	if idx < 0 || idx >= len(m.bundles) {
		return
	}
	for i := range m.bundles {
		view := m.contentBox.Objects[i]
		if i == idx {
			view.Show()
		} else {
			view.Hide()
		}
	}
	m.contentBox.Refresh()

	m.cfg.ActiveTab = idx
	m.tabBar.SetSelected(idx)
	m.bundles[idx].editor.ForceMerge()
	m.scheduleSaveConfig()
}

func (m *MainApp) onTabSelect(idx int) {
	if idx < 0 {
		m.addBlankTab()
		return
	}
	m.selectTab(idx)
}

// confirmCloseTab 关闭当前标签前的确认。
func (m *MainApp) confirmCloseTab() {
	idx := m.tabBar.Selected()
	if idx < 0 || idx >= len(m.bundles) {
		m.setStatus("没有可关闭的标签")
		return
	}
	msg := fmt.Sprintf("确认关闭当前标签「%s」吗？", m.bundles[idx].editor.Title)
	if len(m.bundles) <= 1 {
		msg = "这是唯一一个标签，关闭会清空它的全部内容。确认清空吗？"
	}
	dialog.ShowConfirm("关闭标签", msg, func(ok bool) {
		if ok {
			m.onTabClose(idx)
		}
	}, m.win)
}

func (m *MainApp) onTabClose(idx int) {
	if idx < 0 || idx >= len(m.bundles) {
		return
	}
	if len(m.bundles) <= 1 {
		m.bundles[0].editor.Clear()
		m.bundles[0].editor.ForceMerge()
		m.scheduleSaveConfig()
		m.setStatus("已清空当前标签")
		return
	}
	view := m.contentBox.Objects[idx]
	view.Hide()

	m.bundles = append(m.bundles[:idx], m.bundles[idx+1:]...)
	m.cfg.Tabs = append(m.cfg.Tabs[:idx], m.cfg.Tabs[idx+1:]...)
	m.contentBox.Objects = append(m.contentBox.Objects[:idx], m.contentBox.Objects[idx+1:]...)

	if m.cfg.ActiveTab > idx {
		m.cfg.ActiveTab--
	} else if m.cfg.ActiveTab >= len(m.bundles) {
		m.cfg.ActiveTab = len(m.bundles) - 1
	}
	m.tabBar.SetBundles(m.bundles)
	m.selectTab(m.cfg.ActiveTab)
	m.updateTabCount()
	m.scheduleSaveConfig()
}

func (m *MainApp) addBlankTab() {
	title := fmt.Sprintf("标签 %d", len(m.bundles)+1)
	m.cfg.Tabs = append(m.cfg.Tabs, blankTabConfig())
	m.addBundle(title, m.cfg.Tabs[len(m.cfg.Tabs)-1])
	m.tabBar.SetBundles(m.bundles)
	m.selectTab(len(m.bundles) - 1)
	m.updateTabCount()
}

func (m *MainApp) updateTabCount() {
	if m.countLabel == nil {
		return
	}
	m.countLabel.SetText(fmt.Sprintf("共 %d 标签", len(m.bundles)))
}

// ---------- 状态 / 主题 ----------

func (m *MainApp) setStatus(text string) {
	if m.status == nil {
		return
	}
	fyne.Do(func() { m.status.SetText("● " + text) })
}

func (m *MainApp) toggleTheme() {
	if m.cfg.ThemeMode == "dark" {
		m.cfg.ThemeMode = "light"
	} else {
		m.cfg.ThemeMode = "dark"
	}
	m.a.Settings().SetTheme(NewModernTheme(m.cfg.ThemeMode == "dark"))
	if m.tabBar != nil {
		m.tabBar.Recolor()
	}
	if m.rail != nil {
		m.rail.Recolor()
	}
	for _, bundle := range m.bundles {
		if bundle != nil && bundle.editor != nil {
			bundle.editor.Recolor()
		}
	}
	m.scheduleSaveConfig()
	m.setStatus("已切换为 " + m.cfg.ThemeMode + " 主题")
}

// openConfigMenu 在左下角「⚙ 配置」按钮上方弹出菜单：
//   - 切换亮/暗主题
//   - 立即把当前配置写入 config.json
func (m *MainApp) openConfigMenu() {
	if m.configBtn == nil || m.win == nil {
		return
	}

	menu := fyne.NewMenu("",
		fyne.NewMenuItem(switchThemeLabel(m.cfg.ThemeMode == "dark"), m.toggleTheme),
		fyne.NewMenuItem("立即保存配置", func() {
			m.saveConfigNow()
			m.setStatus("配置已保存")
		}),
	)

	pop := widget.NewPopUpMenu(menu, m.win.Canvas())
	pop.Resize(pop.MinSize())

	// 菜单底边对齐按钮顶边，向上弹出。
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(m.configBtn)
	pop.ShowAtPosition(fyne.NewPos(pos.X, pos.Y-pop.MinSize().Height-theme.Padding()))
}

// ---------- 底部浏览器按钮 ----------

// onFilesDropped 处理拖到原生窗口上的文件：
// .md/.markdown/.txt 直接交给 Markdown 渲染页全屏渲染，.pgm(+.yaml) 打开地图编辑器。
// 渲染页未打开时会顺带启动并打开浏览器。
func (m *MainApp) onFilesDropped(_ fyne.Position, uris []fyne.URI) {
	if m.mdTool == nil || m.mdDocs == nil {
		m.setStatus("Markdown 渲染服务不可用")
		return
	}

	paths := make([]string, 0, len(uris))
	for _, u := range uris {
		if p := localFilePath(u); p != "" {
			paths = append(paths, p)
		}
	}
	if len(paths) == 0 {
		m.setStatus("拖入的不是本地文件，已忽略")
		return
	}

	doc, err := m.mdDocs.LoadPaths(paths)
	if err != nil {
		m.setStatus("拖入的文件无法渲染：" + err.Error())
		return
	}
	m.openBrowserTool(m.mdTool, "Markdown 渲染")
	if doc.Kind == render.KindMap {
		m.setStatus("已载入地图 " + doc.Name + "（页内可测量 / 擦除 / 另存）")
		return
	}
	m.setStatus("已载入 " + doc.Name + " · 全屏渲染（Ctrl+E 切换左右对照，Ctrl+D 切换亮暗）")
}

// localFilePath 取出 file:// URI 对应的本地路径；非本地文件返回空串。
func localFilePath(u fyne.URI) string {
	if u == nil || u.Scheme() != "file" {
		return ""
	}
	return u.Path()
}

// openLanScan 单击启动浏览器版局域网扫描服务并打开系统浏览器；已运行则重开。
func (m *MainApp) openLanScan() {
	if m.lanTool == nil {
		m.setStatus("局域网扫描服务不可用")
		return
	}
	m.openBrowserTool(m.lanTool, "局域网扫描")
}

// openMarkdown 单击启动 Markdown 渲染页（粘贴/打开 .md/.pgm）并打开浏览器。
func (m *MainApp) openMarkdown() {
	if m.mdTool == nil {
		m.setStatus("Markdown 渲染服务不可用")
		return
	}
	m.openBrowserTool(m.mdTool, "Markdown 渲染")
}

func (m *MainApp) openBrowserTool(t *BrowserTool, name string) {
	started, url, err := t.Open()
	if err != nil {
		m.setStatus(name + "：" + err.Error())
		return
	}
	if started {
		m.setStatus(name + "已启动 · " + url + "（端口自动分配；再点一次重开浏览器，退出程序自动停止）")
	} else {
		m.setStatus("已重新打开 " + url)
	}
}

// ---------- 落盘 ----------

func (m *MainApp) scheduleSaveConfig() {
	m.timerMu.Lock()
	if m.saveTimer != nil {
		m.saveTimer.Stop()
	}
	m.saveTimer = time.AfterFunc(saveDebounce, func() {
		fyne.Do(func() { m.doSaveConfig() })
	})
	m.timerMu.Unlock()
}

func (m *MainApp) doSaveConfig() {
	m.saveConfigNow()
	m.setStatus(fmt.Sprintf("配置已保存（%d 标签）", len(m.bundles)))
}

func (m *MainApp) saveConfigNow() {
	for i, b := range m.bundles {
		if i < len(m.cfg.Tabs) {
			m.cfg.Tabs[i] = b.editor.Config
		}
	}
	_ = data.SaveConfig(*m.cfg)
}

func (m *MainApp) flushAll() {
	m.timerMu.Lock()
	if m.saveTimer != nil {
		m.saveTimer.Stop()
		m.saveTimer = nil
	}
	m.timerMu.Unlock()

	if m.simPanel != nil {
		m.simPanel.stopTyping()
	}
	for _, b := range m.bundles {
		b.editor.FlushSave()
	}
	m.saveConfigNow()
	if m.lanTool != nil {
		m.lanTool.Stop()
	}
	if m.mdTool != nil {
		m.mdTool.Stop()
	}
	if m.aiTool != nil {
		m.aiTool.Stop()
	}
}

// ---------- 工具 ----------

func blankTabConfig() data.TabConfig {
	return data.TabConfig{Prompts: blankItems(), Sentences: blankItems()}
}

func blankItems() []data.Item {
	out := make([]data.Item, data.MaxRows)
	for i := range out {
		out[i] = data.Item{Enabled: true}
	}
	return out
}

func switchThemeLabel(dark bool) string {
	if dark {
		return "切换亮色"
	}
	return "切换暗色"
}

func nowStr() string {
	return time.Now().Format("15:04:05")
}
