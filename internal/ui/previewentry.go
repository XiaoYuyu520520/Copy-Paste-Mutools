package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// previewEntry 是只读的「合并预览」输入框。
//
// 为什么覆盖右键：Fyne 的 Entry 在禁用状态下 TappedSecondary 只给
// 「Copy / Select all」，文案未走本地化，且禁用后选区操作不可用。
// 预览框实际只需要「复制全部」，这里用固定中文文案。
type previewEntry struct {
	widget.Entry
	onCopyAll func()
}

func newPreviewEntry(onCopyAll func()) *previewEntry {
	e := &previewEntry{onCopyAll: onCopyAll}
	// 与原来的 makeCodeEntry 保持一致：多行 + 等宽字体。
	e.MultiLine = true
	e.TextStyle = fyne.TextStyle{Monospace: true}
	e.Wrapping = fyne.TextWrapWord
	e.ExtendBaseWidget(e)
	return e
}

// TappedSecondary 覆盖：弹出预览框专用菜单。
func (e *previewEntry) TappedSecondary(pe *fyne.PointEvent) {
	app := fyne.CurrentApp()
	if app == nil || pe == nil {
		return
	}
	c := app.Driver().CanvasForObject(e)
	if c == nil {
		return
	}
	copyAll := fyne.NewMenuItem("复制全部", func() {
		if e.onCopyAll != nil {
			e.onCopyAll()
		}
	})
	showContextMenuByCanvas(c, app.Driver().AbsolutePositionForObject(e).Add(pe.Position), copyAll)
}

// 编译期断言：previewEntry 必须保持 SecondaryTappable（覆盖 Entry 自带的实现）。
var _ fyne.SecondaryTappable = (*previewEntry)(nil)
