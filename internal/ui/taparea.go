package ui

import (
	"fyne.io/fyne/v2"
)

// tapArea 是一个铺满父容器并接收点击的透明命中层。
//
// 它必须保存布局写入的位置和尺寸，否则 Fyne 的命中测试会把 0×0 的对象忽略掉。
type tapArea struct {
	onTap func()
	// onSecondary 处理右键：参数是相对窗口画布的点击位置，
	// 菜单需要用它定位。为 nil 时右键不做任何事。
	onSecondary func(pos fyne.Position)
	position    fyne.Position
	size        fyne.Size
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

func (t *tapArea) TappedSecondary(pe *fyne.PointEvent) {
	if t.onSecondary == nil || pe == nil {
		return
	}
	t.onSecondary(pe.AbsolutePosition)
}

// secondaryArea 是只响应右键的透明命中层。
//
// 与 tapArea 的区别：它**不实现** fyne.Tappable。
// Fyne 的命中测试按控件树顺序找第一个匹配对象，把本层放在兄弟控件
// 之后（绘制在最上层）时：
//   - 左键：本层不匹配 Tappable，事件继续找到内部的 Entry / Button；
//   - 右键：Entry 也实现 SecondaryTappable，会先于本层命中——所以
//     可编辑输入框上弹的是 Fyne 原生的剪切/复制/粘贴菜单，
//     而行内空白处（无其它 SecondaryTappable）才会落到本层。
type secondaryArea struct {
	onSecondary func(pos fyne.Position)
	position    fyne.Position
	size        fyne.Size
}

func (s *secondaryArea) MinSize() fyne.Size      { return fyne.NewSize(0, 0) }
func (s *secondaryArea) Move(pos fyne.Position)  { s.position = pos }
func (s *secondaryArea) Position() fyne.Position { return s.position }
func (s *secondaryArea) Resize(size fyne.Size)   { s.size = size }
func (s *secondaryArea) Size() fyne.Size         { return s.size }
func (s *secondaryArea) Hide()                   {}
func (s *secondaryArea) Show()                   {}
func (s *secondaryArea) Visible() bool           { return true }
func (s *secondaryArea) Refresh()                {}

func (s *secondaryArea) TappedSecondary(pe *fyne.PointEvent) {
	if s.onSecondary == nil || pe == nil {
		return
	}
	s.onSecondary(pe.AbsolutePosition)
}

// 编译期断言：tapArea 同时是 Tappable 与 SecondaryTappable；
// secondaryArea 只允许是 SecondaryTappable。
var (
	_ fyne.Tappable          = (*tapArea)(nil)
	_ fyne.SecondaryTappable = (*tapArea)(nil)

	_ fyne.SecondaryTappable = (*secondaryArea)(nil)
)
