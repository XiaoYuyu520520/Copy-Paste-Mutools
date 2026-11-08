package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// 窗口最小可用尺寸：
//   - 宽度要放下「左侧导航 + 96 px 区块左列 + 两列输入框 + 状态栏三个工具按钮」；
//   - 高度要放下标签栏、一行区块与固定高度的合并预览。
const (
	minWindowWidth  float32 = 960
	minWindowHeight float32 = 640
)

// minSizeContainer 给内容一个最小尺寸：
// 窗口被拖得比它更小时，内容仍按最小尺寸排版（超出部分被窗口裁切），
// 而不是把两列输入框挤到不可用。
//
// Fyne 的 fyne.Window 没有 SetMinSize；用这个容器达到同样的「布局不被压坏」效果。
//
// 注意：必须实现 fyne.Widget（BaseWidget + Renderer），
// 只实现 fyne.CanvasObject 的话 Fyne 不会渲染它（窗口会一片空白）。
type minSizeContainer struct {
	widget.BaseWidget
	minW, minH float32
	inner      fyne.CanvasObject
}

func newMinSizeContainer(inner fyne.CanvasObject, w, h float32) *minSizeContainer {
	c := &minSizeContainer{inner: inner, minW: w, minH: h}
	c.ExtendBaseWidget(c)
	return c
}

func (c *minSizeContainer) CreateRenderer() fyne.WidgetRenderer {
	return &minSizeRenderer{inner: c.inner, minW: c.minW, minH: c.minH}
}

// minSizeRenderer 把内部对象撑到不小于最小尺寸。
type minSizeRenderer struct {
	inner      fyne.CanvasObject
	minW, minH float32
}

func (r *minSizeRenderer) Layout(size fyne.Size) {
	r.inner.Resize(fyne.NewSize(maxFloat(size.Width, r.minW), maxFloat(size.Height, r.minH)))
}

func (r *minSizeRenderer) MinSize() fyne.Size {
	s := r.inner.MinSize()
	return fyne.NewSize(maxFloat(s.Width, r.minW), maxFloat(s.Height, r.minH))
}

func (r *minSizeRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.inner} }

func (r *minSizeRenderer) Refresh() { r.inner.Refresh() }

func (r *minSizeRenderer) Destroy() {}

func maxFloat(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

var _ fyne.CanvasObject = (*minSizeContainer)(nil)
