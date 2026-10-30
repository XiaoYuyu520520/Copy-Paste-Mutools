package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// LeftRail 是左侧竖向页面切换，取代旧版的历史栏。
//
// 视觉（参考 JetBrains / VSCode 的侧边导航）：
//   - 顶部：分区标题 + 分隔线
//   - 每个导航项：[图标] + [文字]，固定行高 36 px
//   - 选中项：圆角 10 px 的蓝色渐变卡片 + 加粗主文字
//   - 未选中：无底色 + 柔和浅灰文字
type LeftRail struct {
	items    []string
	icons    []fyne.Resource
	selected int
	onSelect func(int)

	root fyne.CanvasObject
	rows []*leftRailRow
}

// leftRailRow 单个导航项：
//
//	root = Container[ leftRailRowLayout ](
//	         bg       canvas.Image（圆角 + 水平渐变，未选中时隐藏）
//	         content  HBox(图标 + RichText 文字)（左内边距 12 px，垂直居中）
//	         tap      透明 Tappable（铺满整行接收点击）
//	       )
type leftRailRow struct {
	root   *fyne.Container
	bg     *canvas.Image
	text   *widget.RichText
	label  string
	active bool

	// 上一次生成背景位图的尺寸，用于避免每帧重复栅格化。
	bw, bh int
}

// NewLeftRail 构造左侧导航（无图标）。
func NewLeftRail(items []string, onSelect func(int)) *LeftRail {
	return NewLeftRailWithIcons(items, nil, onSelect)
}

// NewLeftRailWithIcons 允许为每项指定图标资源（可为 nil）。
func NewLeftRailWithIcons(items []string, icons []fyne.Resource, onSelect func(int)) *LeftRail {
	r := &LeftRail{items: items, icons: icons, onSelect: onSelect}

	title := widget.NewLabelWithStyle("页面", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	header := container.NewVBox(
		container.NewPadded(title),
		widget.NewSeparator(),
	)

	r.rows = make([]*leftRailRow, 0, len(items))
	rowBox := container.NewVBox()
	for i, label := range items {
		idx := i
		row := newLeftRailRow(label, iconOrNil(icons, i), func() {
			if r.onSelect != nil {
				r.onSelect(idx)
			}
		})
		r.rows = append(r.rows, row)
		rowBox.Add(row.root)
	}

	body := container.NewBorder(header, nil, nil, nil, container.NewPadded(rowBox))
	r.root = body
	r.refresh()
	return r
}

func iconOrNil(icons []fyne.Resource, i int) fyne.Resource {
	if i < 0 || i >= len(icons) {
		return nil
	}
	return icons[i]
}

// newLeftRailRow 构造一个导航项。
func newLeftRailRow(label string, iconRes fyne.Resource, onTap func()) *leftRailRow {
	row := &leftRailRow{label: label}
	pal := CurrentPalette()

	// 背景：圆角渐变卡片，默认隐藏（未选中时与侧栏融为一体）。
	row.bg = newRoundedGradientImage(240, leftRailRowHeight, pal.NavStart, pal.NavEnd, 10)
	row.bw, row.bh = 240, leftRailRowHeight
	row.bg.Hide()

	// 图标：没有图标时用 18x18 的透明占位，保证文字左边距一致。
	var iconObj fyne.CanvasObject
	if iconRes != nil {
		iconObj = widget.NewIcon(iconRes)
	} else {
		iconObj = container.New(
			layout.NewGridWrapLayout(fyne.NewSize(18, 18)),
			canvas.NewRectangle(color.NRGBA{R: 0, G: 0, B: 0, A: 0}),
		)
	}

	// 文字用 RichText，以便按选中态精确控制颜色（Label 不支持单实例改色）。
	row.text = widget.NewRichText()
	row.applyTextColor(false)

	tap := &tapArea{onTap: onTap}
	row.root = container.New(
		&leftRailRowLayout{row: row},
		row.bg, container.NewHBox(iconObj, row.text), tap,
	)
	return row
}

// applyTextColor 按选中态切换文字颜色与字重：
// 选中 = 主文字 + 加粗；未选中 = 主文字。
func (row *leftRailRow) applyTextColor(active bool) {
	colorName := theme.ColorNameForeground
	style := fyne.TextStyle{}
	if active {
		style = fyne.TextStyle{Bold: true}
	}
	row.text.Segments = []widget.RichTextSegment{
		&widget.TextSegment{
			Text: row.label,
			Style: widget.RichTextStyle{
				ColorName: colorName,
				TextStyle: style,
			},
		},
	}
	row.text.Refresh()
}

// applyBackground 切换选中背景：选中时显示圆角蓝色渐变，未选中时隐藏。
func (row *leftRailRow) applyBackground(pal Palette, active bool) {
	row.active = active
	if !active {
		row.bg.Hide()
		return
	}
	row.rebuildBg(pal)
	row.bg.Show()
	row.bg.Refresh()
}

// rebuildBg 按当前尺寸重新栅格化圆角渐变背景。
func (row *leftRailRow) rebuildBg(pal Palette) {
	w, h := row.bw, row.bh
	if w <= 0 {
		w = 240
	}
	if h <= 0 {
		h = leftRailRowHeight
	}
	row.bg.Image = roundedGradient(w, h, pal.NavStart, pal.NavEnd, 10)
	row.bg.Refresh()
}

// resizeBg 在行尺寸变化时同步背景位图尺寸（尺寸未变则直接返回）。
func (row *leftRailRow) resizeBg(w, h int) {
	if w <= 0 || h <= 0 {
		return
	}
	if w == row.bw && h == row.bh {
		return
	}
	row.bw, row.bh = w, h
	if row.active {
		row.rebuildBg(CurrentPalette())
	}
}

// leftRailRowLayout 布局单个导航项：
//   - objects[0] 背景（圆角渐变位图）：铺满
//   - objects[1] 内容（图标 + 文字）：左内边距 12 px，垂直居中
//   - objects[2] 点击层：铺满
type leftRailRowLayout struct {
	row *leftRailRow
}

const leftRailRowHeight = 36
const leftRailRowPadX = 12

func (*leftRailRowLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) < 2 {
		return fyne.NewSize(0, leftRailRowHeight)
	}
	s := objects[1].MinSize()
	return fyne.NewSize(s.Width+leftRailRowPadX*2, leftRailRowHeight)
}

func (l *leftRailRowLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) < 3 {
		return
	}
	bg, content, tap := objects[0], objects[1], objects[2]

	bg.Move(fyne.NewPos(0, 0))
	bg.Resize(size)

	cs := content.MinSize()
	content.Move(fyne.NewPos(leftRailRowPadX, (size.Height-cs.Height)/2))
	content.Resize(fyne.NewSize(size.Width-leftRailRowPadX*2, cs.Height))

	tap.Move(fyne.NewPos(0, 0))
	tap.Resize(size)

	if l.row != nil {
		l.row.resizeBg(int(size.Width), int(size.Height))
	}
}

// View 返回根容器。
func (r *LeftRail) View() fyne.CanvasObject { return r.root }

// Select 程序化选中某项并刷新样式（不触发 onSelect）。
func (r *LeftRail) Select(i int) {
	if i < 0 || i >= len(r.rows) {
		return
	}
	r.selected = i
	r.refresh()
}

// Selected 当前选中下标。
func (r *LeftRail) Selected() int { return r.selected }

// Recolor 主题切换后刷新所有行颜色。
func (r *LeftRail) Recolor() { r.refresh() }

func (r *LeftRail) refresh() {
	pal := CurrentPalette()
	for i, row := range r.rows {
		active := i == r.selected
		row.applyBackground(pal, active)
		row.applyTextColor(active)
	}
}
