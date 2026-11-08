package ui

import "fyne.io/fyne/v2"

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
type minSizeContainer struct {
	minW, minH float32
	inner      fyne.CanvasObject
}

func newMinSizeContainer(inner fyne.CanvasObject, w, h float32) *minSizeContainer {
	return &minSizeContainer{inner: inner, minW: w, minH: h}
}

func (c *minSizeContainer) MinSize() fyne.Size {
	s := c.inner.MinSize()
	return fyne.NewSize(maxFloat(s.Width, c.minW), maxFloat(s.Height, c.minH))
}

func (c *minSizeContainer) Resize(size fyne.Size) {
	c.inner.Resize(fyne.NewSize(maxFloat(size.Width, c.minW), maxFloat(size.Height, c.minH)))
}

func (c *minSizeContainer) Move(pos fyne.Position)  { c.inner.Move(pos) }
func (c *minSizeContainer) Position() fyne.Position { return c.inner.Position() }
func (c *minSizeContainer) Size() fyne.Size         { return c.inner.Size() }
func (c *minSizeContainer) Show()                   { c.inner.Show() }
func (c *minSizeContainer) Hide()                   { c.inner.Hide() }
func (c *minSizeContainer) Visible() bool           { return c.inner.Visible() }
func (c *minSizeContainer) Refresh()                { c.inner.Refresh() }

func maxFloat(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

var _ fyne.CanvasObject = (*minSizeContainer)(nil)
