package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
)

// NewCardWith 把 inner 包成一张带圆角 + 细边框的卡片容器。
//
// 用途：给面板、预览区提供统一的视觉边界（圆角、1 px 边框、内边距），
// 替代「一个矩形 + 一个 entry」的粗糙组合。pad/corner 由调用方指定。
func NewCardWith(inner fyne.CanvasObject, pad, corner float32) fyne.CanvasObject {
	return newCardPanel(inner, pad, corner).root
}

func newCardPanel(inner fyne.CanvasObject, pad, corner float32) *cardPanel {
	pal := CurrentPalette()

	bg := canvas.NewRectangle(pal.Card)
	bg.CornerRadius = corner

	// 1 px 描边矩形：StrokeColor + CornerRadius 让边框和卡片同形。
	stroke := canvas.NewRectangle(color.NRGBA{R: 0, G: 0, B: 0, A: 0})
	stroke.StrokeColor = pal.Border
	stroke.StrokeWidth = 1
	stroke.CornerRadius = corner

	padded := container.NewPadded(inner)

	// 自定义 layout：背景与描边拉伸到与 padded 相同位置和大小，
	// 内容按内边距缩进。
	c := container.New(&cardLayout{pad: pad, body: padded}, bg, stroke, padded)
	c.Refresh()
	return &cardPanel{root: c, bg: bg, stroke: stroke}
}

// cardPanel 保存卡片的可重着色图层，供运行中主题切换使用。
type cardPanel struct {
	root   *fyne.Container
	bg     *canvas.Rectangle
	stroke *canvas.Rectangle
}

// Recolor 按当前主题刷新卡片背景与边框颜色。
func (c *cardPanel) Recolor() {
	if c == nil {
		return
	}
	pal := CurrentPalette()
	if c.bg != nil {
		c.bg.FillColor = pal.Card
		c.bg.Refresh()
	}
	if c.stroke != nil {
		c.stroke.StrokeColor = pal.Border
		c.stroke.Refresh()
	}
	if c.root != nil {
		c.root.Refresh()
	}
}

type cardLayout struct {
	pad  float32
	body fyne.CanvasObject // index 2
}

func (l *cardLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) < 3 {
		return fyne.NewSize(0, 0)
	}
	body := objects[2].MinSize()
	return fyne.NewSize(body.Width+l.pad*2, body.Height+l.pad*2)
}

func (l *cardLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) < 3 {
		return
	}
	bg, stroke, body := objects[0], objects[1], objects[2]
	bg.Move(fyne.NewPos(0, 0))
	bg.Resize(size)
	stroke.Move(fyne.NewPos(0, 0))
	stroke.Resize(size)
	body.Move(fyne.NewPos(l.pad, l.pad))
	body.Resize(fyne.NewSize(size.Width-l.pad*2, size.Height-l.pad*2))
}
