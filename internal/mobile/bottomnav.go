package mobile

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	shared "github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/theme"
)

// 底栏尺寸：触控目标高度按 Material 建议不小于 48 dp，这里留 60 更宽松。
const (
	bottomNavHeight   float32 = 60
	navIconSize       float32 = 22
	navSelectedRadius float32 = 14
)

// mobilePalette 读取当前主题的调色板；取不到时退回暗色。
func mobilePalette() shared.Palette {
	if a := fyne.CurrentApp(); a != nil {
		if p, ok := shared.PaletteOf(a.Settings().Theme()); ok {
			return p
		}
	}
	return shared.Dark()
}

// tapLayer 是接收点击的透明层。
//
// 与桌面端 ui.tapArea 的差别：这里用 widget.BaseWidget 实现完整控件，
// 移动端驱动需要完整的 widget 才能可靠地命中触摸。
type tapLayer struct {
	widget.BaseWidget
	onTap func()
}

func newTapLayer(onTap func()) *tapLayer {
	t := &tapLayer{onTap: onTap}
	t.ExtendBaseWidget(t)
	return t
}

func (t *tapLayer) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.NRGBA{A: 0}))
}

func (t *tapLayer) Tapped(*fyne.PointEvent) {
	if t.onTap != nil {
		t.onTap()
	}
}

// navButton 是底部导航里的单个入口：竖排图标 + 文字，选中时带圆角底色。
type navButton struct {
	root    fyne.CanvasObject
	bg      *canvas.Rectangle
	icon    *widget.Icon
	label   *widget.RichText
	name    string // 图标名（见 navicons.go）
	text    string // 显示文字
	iconRes fyne.Resource
}

// navButtonLayout 让图标在上、文字在下，整体垂直居中。
type navButtonLayout struct{}

func (*navButtonLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(56, bottomNavHeight)
}

func (*navButtonLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) < 3 {
		return
	}
	bg, icon, label := objects[0], objects[1], objects[2]

	bg.Move(fyne.NewPos(0, 0))
	bg.Resize(size)

	iconSize := fyne.NewSize(navIconSize, navIconSize)
	icon.Move(fyne.NewPos((size.Width-iconSize.Width)/2, 9))
	icon.Resize(iconSize)

	labelSize := label.MinSize()
	label.Move(fyne.NewPos((size.Width-labelSize.Width)/2, 9+iconSize.Height+2))
	label.Resize(fyne.NewSize(size.Width, labelSize.Height))
}

// bottomNav 是自写的底部导航栏。
//
// 为什么不用 container.NewAppTabs：Fyne 2.8 的 AppTabs 在窄屏下会把放不下的
// 标签收进「更多」弹出菜单，5 个入口时常被折叠，拇指点击要够两次；
// 自写底栏保证 5 个入口始终等宽可见，每个都有 60 dp 高的触控目标。
type bottomNav struct {
	root     fyne.CanvasObject
	items    []*navButton
	selected int
}

// bottomNavEntry 是单个入口的静态描述。
type bottomNavEntry struct {
	Icon string // 图标名
	Text string // 文字
}

// newBottomNav 构造底部导航；onSelect 收到被点击的入口下标。
func newBottomNav(entries []bottomNavEntry, onSelect func(int)) *bottomNav {
	n := &bottomNav{}
	pal := mobilePalette()

	for i, e := range entries {
		idx := i
		b := &navButton{name: e.Icon, text: e.Text}
		b.bg = canvas.NewRectangle(color.NRGBA{A: 0})
		b.bg.CornerRadius = navSelectedRadius
		b.bg.Hide()

		b.iconRes = navIcon(e.Icon, pal.FgMuted)
		b.icon = widget.NewIcon(b.iconRes)
		b.label = widget.NewRichText()

		inner := container.New(&navButtonLayout{}, b.bg, b.icon, b.label)
		tap := newTapLayer(func() {
			if onSelect != nil {
				onSelect(idx)
			}
		})
		b.root = container.NewStack(inner, tap)
		n.items = append(n.items, b)
	}

	// 等宽网格：不论屏幕多宽，各入口始终平分整行。
	w := float32(400) / float32(len(entries))
	if len(entries) == 0 {
		w = 0
	}
	row := container.NewGridWrap(fyne.NewSize(w, bottomNavHeight))
	for _, b := range n.items {
		row.Add(b.root)
	}
	n.root = row
	n.applyStyle(0)
	return n
}

// View 返回底栏根容器。
func (n *bottomNav) View() fyne.CanvasObject { return n.root }

// Select 切换选中项并刷新样式（不触发 onSelect 回调）。
func (n *bottomNav) Select(i int) {
	n.selected = i
	n.applyStyle(i)
}

// RefreshStyle 主题切换后重刷（保持上一次的选中下标）。
func (n *bottomNav) RefreshStyle() { n.applyStyle(n.selected) }

// applyStyle 按当前主题与选中下标刷新全部条目。
func (n *bottomNav) applyStyle(selected int) {
	pal := mobilePalette()
	for idx, b := range n.items {
		active := idx == selected
		if active {
			b.bg.FillColor = pal.AccentBg
			b.bg.Show()
			b.icon.SetResource(navIcon(b.name, pal.Accent))
		} else {
			b.bg.Hide()
			b.icon.SetResource(navIcon(b.name, pal.FgMuted))
		}
		colorName := theme.ColorNamePlaceHolder
		style := fyne.TextStyle{}
		if active {
			colorName = theme.ColorNameForeground
			style = fyne.TextStyle{Bold: true}
		}
		b.label.Segments = []widget.RichTextSegment{
			&widget.TextSegment{
				Text: b.text,
				Style: widget.RichTextStyle{
					Alignment: fyne.TextAlignCenter,
					ColorName: colorName,
					TextStyle: style,
				},
			},
		}
		b.label.Refresh()
		b.bg.Refresh()
	}
}

// 编译期断言：tapLayer 是可点击的控件。
var _ fyne.Tappable = (*tapLayer)(nil)
