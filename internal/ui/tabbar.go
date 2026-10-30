package ui

import (
	"image/color"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// 标签栏固定高度：每个 tab 的按钮 + 文字高度约 40 px，加上 2 px 下划线。
const tabBarStripHeight = 42

// 2 px 下划线粗细，对应 IDE 顶部 tab 选中态标记。
const tabUnderlineHeight = 2

// 切换标签时的「下划线滑动」动画时长。
const tabSwitchAnimate = 220 * time.Millisecond

// customTabBar 自定义标签栏：
//
//   - 左侧：tab 横向列表，未选文字 = FgMuted，选中文字 = Fg（白），下方 2px Accent 下划线
//   - 右侧：[+ 新增] [关闭当前标签]
//   - 切换 tab 时，下划线水平滑动到新位置（ease-out）
//
// 设计原因：Fyne 的 widget.Button 文字色无法单实例覆盖，所以这里用 *widget.Label
// 配 fyne.Tappable 自定义控件，确保选中 / 未选 / hover 三态有清晰视觉差异。
type customTabBar struct {
	bundles  []*tabBundle
	selected int

	mu       sync.Mutex
	onSelect func(idx int) // idx=-1 表示新增

	root     fyne.CanvasObject
	stripBox *fyne.Container
	scroll   *container.Scroll
	addBtn   *widget.Button
	closeBtn *widget.Button
	onClose  func()

	underline *canvas.Rectangle // 选中态下划线
	slideAnim *fyne.Animation
	curX      float32
	curW      float32
	ulnSet    bool

	// tabItems 与 bundles 顺序对应，用于按 index 找到对应的 Label / Tappable，
	// 便于刷新选中态颜色与重定位下划线。
	tabItems []*tabItem
}

// tabItem 单个 tab 的内容：RichText（可控文字色）+ Tappable。
//
// 用 RichText 而非 Label：Label 不支持按住选中态改文字颜色，而标签栏要求
// 「未选 = 柔和灰、选中 = 白 / 高亮」。
type tabItem struct {
	text *widget.RichText
	name string
}

// NewCustomTabBar 构造空标签栏。
func NewCustomTabBar(onSelect func(int)) *customTabBar {
	b := &customTabBar{
		onSelect: onSelect,
	}

	b.addBtn = widget.NewButtonWithIcon("新增", theme.ContentAddIcon(), func() {
		if b.onSelect != nil {
			b.onSelect(-1)
		}
	})
	// 主操作按钮：Azure 填充 + 4 px 圆角（圆角由 ButtonRadius 主题值提供）。
	b.addBtn.Importance = widget.HighImportance

	b.closeBtn = widget.NewButtonWithIcon("", theme.ContentClearIcon(), func() {
		if b.onClose != nil {
			b.onClose()
		}
	})
	b.closeBtn.Importance = widget.LowImportance
	b.closeBtn.Hide()

	pal := CurrentPalette()
	b.underline = canvas.NewRectangle(pal.Accent)

	b.stripBox = container.New(&stripLayout{
		MaxHeight: tabBarStripHeight,
		Float:     b.underline,
	})
	b.stripBox.Objects = []fyne.CanvasObject{b.underline}
	b.scroll = container.NewHScroll(b.stripBox)

	trailing := container.NewHBox(b.closeBtn, b.addBtn)
	b.root = container.NewBorder(
		nil,
		widget.NewSeparator(),
		nil,
		container.NewPadded(trailing),
		b.scroll,
	)

	latestTabBar = b
	return b
}

// SetCloseHandler 注册「关闭当前标签」回调（用于显示右侧关闭按钮）。
func (b *customTabBar) SetCloseHandler(onClose func()) {
	b.onClose = onClose
	if b.closeBtn == nil {
		return
	}
	if onClose != nil {
		b.closeBtn.Show()
	} else {
		b.closeBtn.Hide()
	}
}

// View 返回根容器。
func (b *customTabBar) View() fyne.CanvasObject { return b.root }

// SetBundles 设置标签列表。
func (b *customTabBar) SetBundles(bundles []*tabBundle) {
	b.mu.Lock()
	b.bundles = bundles
	b.mu.Unlock()
	b.refresh()
}

// SetSelected 设置激活标签。
func (b *customTabBar) SetSelected(idx int) {
	b.mu.Lock()
	b.selected = idx
	b.mu.Unlock()
	b.refresh()
}

// Selected 当前激活下标。
func (b *customTabBar) Selected() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.selected
}

// Recolor 切换主题后刷新下划线颜色与所有 tab 文字颜色。
func (b *customTabBar) Recolor() {
	if b.underline != nil {
		b.underline.FillColor = accentColor()
		b.underline.Refresh()
	}
	// 重新应用每个 tab 文字色

	for i, item := range b.tabItems {
		applyTabStyle(item, i == b.selected)
	}
	if b.stripBox != nil {
		b.stripBox.Refresh()
	}
}

// refresh 重建 tab 列表并定位下划线。
func (b *customTabBar) refresh() {
	b.mu.Lock()
	selected := b.selected
	bundles := append([]*tabBundle(nil), b.bundles...)
	b.mu.Unlock()

	b.underline.FillColor = accentColor()
	if len(bundles) == 0 {
		b.underline.Hide()
		b.tabItems = nil
		b.stripBox.Objects = []fyne.CanvasObject{b.underline}
		b.stripBox.Refresh()
		return
	}
	b.underline.Show()

	b.tabItems = make([]*tabItem, 0, len(bundles))
	items := make([]fyne.CanvasObject, 0, len(bundles))
	for i, bundle := range bundles {
		idx := i
		title := bundle.editor.Title

		lbl := widget.NewLabel(title)
		lbl.Alignment = fyne.TextAlignCenter
		lbl.TextStyle = fyne.TextStyle{}

		item := &tabItem{text: widget.NewRichText(), name: title}
		applyTabStyle(item, i == selected)

		tap := &tapArea{onTap: func() {
			if b.onSelect != nil {
				b.onSelect(idx)
			}
		}}
		// 自定义布局让文字居中，Tappable 覆盖整块区域
		wrap := container.New(&tabItemLayout{text: item.text, tap: tap}, item.text, tap)

		b.tabItems = append(b.tabItems, item)
		items = append(items, wrap)
	}

	b.stripBox.Objects = append([]fyne.CanvasObject{b.underline}, items...)
	b.stripBox.Refresh()

	x, w := b.tabRect(items, selected)
	b.moveUnderlineTo(x, w)
	b.ensureVisible(x, w)

	// 文字色阶
	for i, item := range b.tabItems {
		applyTabStyle(item, i == selected)
	}
	b.TryFocusSelected()
}

// applyTabStyle 应用标签文字色阶：
//   - 选中：主前景色（柔和浅灰 / 白）+ 加粗
//   - 未选中：柔和灰（占位色），保持安静
func applyTabStyle(item *tabItem, active bool) {
	if item == nil || item.text == nil {
		return
	}
	colorName := theme.ColorNamePlaceHolder
	style := fyne.TextStyle{}
	if active {
		colorName = theme.ColorNameForeground
		style = fyne.TextStyle{Bold: true}
	}
	item.text.Segments = []widget.RichTextSegment{
		&widget.TextSegment{
			Text: item.name,
			Style: widget.RichTextStyle{
				Alignment: fyne.TextAlignCenter,
				ColorName: colorName,
				TextStyle: style,
			},
		},
	}
	item.text.Refresh()
}

// tabItemLayout 让标签文字居中，Tappable 覆盖整块区域用于点击。
type tabItemLayout struct {
	text *widget.RichText
	tap  *tapArea
}

func (l *tabItemLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) == 0 {
		return fyne.NewSize(0, tabBarStripHeight)
	}
	label := objects[0].MinSize()
	// 24 px 左右内边距 + 自适应文字宽度
	w := label.Width + 24
	return fyne.NewSize(w, tabBarStripHeight)
}

func (l *tabItemLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) < 2 {
		return
	}
	label, tap := objects[0], objects[1]
	lblSize := label.MinSize()
	label.Move(fyne.NewPos((size.Width-lblSize.Width)/2, (size.Height-lblSize.Height)/2))
	label.Resize(lblSize)
	tap.Move(fyne.NewPos(0, 0))
	tap.Resize(size)
}

// TryFocusSelected 已脱焦的标签在重新进入时获得焦点描边。
func (b *customTabBar) TryFocusSelected() {
	// 已脱焦处理：Fyne 默认无可见 focus 描边差异，跳过。
}

// tabRect 按 stripLayout 同样的算法计算第 selected 个 tab 的 (x, width)。
func (b *customTabBar) tabRect(items []fyne.CanvasObject, selected int) (float32, float32) {
	gap := theme.Padding()
	x := float32(0)
	for i, o := range items {
		w := o.MinSize().Width
		if i == selected {
			return x, w
		}
		x += w + gap
	}
	return 0, 0
}

// placeUnderline 把下划线安到 (x, w)。
func (b *customTabBar) placeUnderline(x, w float32) {
	if b.underline == nil {
		return
	}
	stripH := b.stripBox.Size().Height
	if stripH <= 0 {
		stripH = tabBarStripHeight
	}
	y := stripH - tabUnderlineHeight
	if y < 0 {
		y = 0
	}
	b.underline.Move(fyne.NewPos(x, y))
	b.underline.Resize(fyne.NewSize(w, tabUnderlineHeight))
}

// moveUnderlineTo 把下划线从当前位置动画到 (x, w)。
func (b *customTabBar) moveUnderlineTo(x, w float32) {
	if !b.ulnSet {
		b.ulnSet = true
		b.curX, b.curW = x, w
		b.placeUnderline(x, w)
		return
	}
	if b.curX == x && b.curW == w {
		b.placeUnderline(x, w)
		return
	}
	if b.slideAnim != nil {
		b.slideAnim.Stop()
	}
	fromX, fromW := b.curX, b.curW
	b.curX, b.curW = x, w
	a := fyne.NewAnimation(tabSwitchAnimate, func(v float32) {
		b.placeUnderline(fromX+(x-fromX)*v, fromW+(w-fromW)*v)
	})
	a.Curve = fyne.AnimationEaseOut
	b.slideAnim = a
	a.Start()
}

// ensureVisible 让选中标签滚动到可视区域。
func (b *customTabBar) ensureVisible(x, w float32) {
	if b.scroll == nil {
		return
	}
	view := b.scroll.Size().Width
	if view <= 0 {
		return
	}
	off := b.scroll.Offset.X
	pad := theme.Padding()
	switch {
	case x < off:
		b.scroll.ScrollToOffset(fyne.Position{X: x - pad, Y: b.scroll.Offset.Y})
	case x+w > off+view:
		b.scroll.ScrollToOffset(fyne.Position{X: x + w - view + pad, Y: b.scroll.Offset.Y})
	}
}

// accentColor returns the current theme primary color.
func accentColor() color.Color {
	if app := fyne.CurrentApp(); app != nil {
		if th := app.Settings().Theme(); th != nil {
			return th.Color(theme.ColorNamePrimary, app.Settings().ThemeVariant())
		}
	}
	if t := theme.DarkTheme(); t != nil {
		return t.Color(theme.ColorNamePrimary, fyne.ThemeVariant(1))
	}
	return color.Black
}

// latestTabBar 是最近一次 NewCustomTabBar 构造的对象，用于单元测试读取 layout。
var latestTabBar *customTabBar

// GlobalTabBarStripHeight 返回当前标签栏的 MinSize.Height（供测试）。
func GlobalTabBarStripHeight() float32 {
	if latestTabBar == nil {
		return 0
	}
	return latestTabBar.stripBox.MinSize().Height
}

// GlobalTabBarStripWidth 返回当前标签栏的 MinSize.Width（供测试）。
func GlobalTabBarStripWidth() float32 {
	if latestTabBar == nil {
		return 0
	}
	return latestTabBar.stripBox.MinSize().Width
}

// MakePillVisible 强制把下划线放到当前位置（无动画），用于外部尺寸变化后。
func (b *customTabBar) MakePillVisible() {
	if b.ulnSet {
		b.placeUnderline(b.curX, b.curW)
	}
}
