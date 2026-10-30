package ui

import "fyne.io/fyne/v2"

// tapArea 是一个铺满父容器并接收点击的透明命中层。
//
// 它必须保存布局写入的位置和尺寸，否则 Fyne 的命中测试会把 0×0 的对象忽略掉。
type tapArea struct {
	onTap    func()
	position fyne.Position
	size     fyne.Size
}

func (t *tapArea) MinSize() fyne.Size      { return fyne.NewSize(0, 0) }
func (t *tapArea) Move(pos fyne.Position)  { t.position = pos }
func (t *tapArea) Position() fyne.Position { return t.position }
func (t *tapArea) Resize(size fyne.Size)   { t.size = size }
func (t *tapArea) Size() fyne.Size         { return t.size }
func (t *tapArea) Hide()                   {}
func (t *tapArea) Show()                   {}
func (t *tapArea) Visible() bool           { return true }
func (t *tapArea) Refresh()                {}

func (t *tapArea) Tapped(_ *fyne.PointEvent) {
	if t.onTap != nil {
		t.onTap()
	}
}

func (t *tapArea) TappedSecondary(_ *fyne.PointEvent) {}
