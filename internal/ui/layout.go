package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
)

// fixedHeightLayout 把内部对象裁到指定高度，宽度填满父容器。
// 用于把 MultiLineEntry 等「内容决定高度」的小部件拉到一个最小可读高度。
type fixedHeightLayout struct {
	MinHeight float32
}

func (l *fixedHeightLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) == 0 {
		return fyne.NewSize(0, l.MinHeight)
	}
	w, h := float32(0), float32(0)
	for _, o := range objects {
		s := o.MinSize()
		if s.Width > w {
			w = s.Width
		}
		if s.Height > h {
			h = s.Height
		}
	}
	if h < l.MinHeight {
		h = l.MinHeight
	}
	return fyne.NewSize(w, h)
}

func (l *fixedHeightLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		h := l.MinHeight
		if o.MinSize().Height > h {
			h = o.MinSize().Height
		}
		if h > size.Height {
			h = size.Height
		}
		y := float32(0)
		if size.Height > h {
			y = (size.Height - h) / 2
		}
		o.Move(fyne.NewPos(0, y))
		o.Resize(fyne.NewSize(size.Width, h))
	}
}

// WrapFixedHeight 用 fixedHeightLayout 包裹一个对象，返回新的 CanvasObject。
func WrapFixedHeight(inner fyne.CanvasObject, h float32) fyne.CanvasObject {
	return container.New(&fixedHeightLayout{MinHeight: h}, inner)
}

// exactHeightLayout 强制内部对象只占指定高度，不跟随内容 MinSize 长高。
// 编辑器的行与预览区使用该布局，避免多行 Entry 的文本高度把整页撑开。
type exactHeightLayout struct {
	Height float32
}

func (l *exactHeightLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	w := float32(0)
	for _, o := range objects {
		if s := o.MinSize(); s.Width > w {
			w = s.Width
		}
	}
	return fyne.NewSize(w, l.Height)
}

func (l *exactHeightLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	h := l.Height
	if h > size.Height {
		h = size.Height
	}
	y := (size.Height - h) / 2
	if y < 0 {
		y = 0
	}
	for _, o := range objects {
		o.Move(fyne.NewPos(0, y))
		o.Resize(fyne.NewSize(size.Width, h))
	}
}

func WrapExactHeight(inner fyne.CanvasObject, h float32) fyne.CanvasObject {
	return container.New(&exactHeightLayout{Height: h}, inner)
}

// editorRowLayout 编辑器单行布局：第一个对象（复选框列）占固定宽度，
// 其余两个对象（提示词列、语句列）均分剩余宽度；对象间保留 Gap 间隔。
type editorRowLayout struct {
	LeftWidth float32
	Gap       float32
}

func (l *editorRowLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) == 0 {
		return fyne.Size{}
	}
	leftW := l.LeftWidth
	h := float32(0)
	for i, o := range objects {
		s := o.MinSize()
		if s.Height > h {
			h = s.Height
		}
		if i == 0 && s.Width > leftW {
			leftW = s.Width
		}
	}
	w := leftW
	for _, o := range objects[1:] {
		w += o.MinSize().Width
	}
	w += l.Gap * float32(len(objects)-1)
	return fyne.NewSize(w, h)
}

func (l *editorRowLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) == 0 {
		return
	}
	gap := l.Gap
	// 宽度不够时先牺牲间隙，保证列可用
	if l.LeftWidth+float32(len(objects)-1)*gap > size.Width {
		gap = 0
	}
	leftW := l.LeftWidth
	if m := objects[0].MinSize().Width; m > leftW {
		leftW = m
	}
	if leftW > size.Width {
		leftW = size.Width
	}
	left := objects[0]
	left.Move(fyne.NewPos(0, 0))
	left.Resize(fyne.NewSize(leftW, size.Height))

	if len(objects) < 2 {
		return
	}
	rest := size.Width - leftW - gap*float32(len(objects)-1)
	if rest < 0 {
		rest = 0
	}
	cols := len(objects) - 1
	x := leftW + gap
	for _, o := range objects[1:] {
		colW := rest / float32(cols)
		o.Move(fyne.NewPos(x, 0))
		o.Resize(fyne.NewSize(colW, size.Height))
		x += colW + gap
	}
}

// stripLayout 把若干子对象横向流式排列，并约束每个对象的高度不超过 maxH，
// 避免被父容器（比如 Border.Center / Scroll）拉到全高。
// Float 指定的对象不参与流式布局（不占位置、不被移动），
// 由调用方自行用 Move/Resize 定位 —— 用于标签栏的滑动高亮胶囊。
// editorLeftLayout packs row label and checkboxes without the default vertical
// box padding, keeping rows compact while preserving hit areas.
type editorLeftLayout struct{}

func (*editorLeftLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	w, h := float32(0), float32(0)
	for _, o := range objects {
		s := o.MinSize()
		if s.Width > w {
			w = s.Width
		}
		h += s.Height
	}
	return fyne.NewSize(w, h)
}

func (*editorLeftLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	y := float32(0)
	for _, o := range objects {
		h := o.MinSize().Height
		if y+h > size.Height {
			h = size.Height - y
		}
		if h < 0 {
			h = 0
		}
		o.Move(fyne.NewPos(0, y))
		o.Resize(fyne.NewSize(size.Width, h))
		y += h
	}
}

type stripLayout struct {
	MaxHeight float32
	Gap       float32
	Float     fyne.CanvasObject
}

func (l *stripLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	w, h := float32(0), float32(0)
	gap := l.Gap
	if gap <= 0 {
		gap = theme.Padding()
	}
	first := true
	for _, o := range objects {
		if o == l.Float {
			continue
		}
		s := o.MinSize()
		if s.Width > 0 {
			if !first {
				w += gap
			}
			w += s.Width
			first = false
		}
		if s.Height > h {
			h = s.Height
		}
	}
	if h > l.MaxHeight {
		h = l.MaxHeight
	}
	if h < l.MaxHeight {
		h = l.MaxHeight
	}
	return fyne.NewSize(w, h)
}

func (l *stripLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	gap := l.Gap
	if gap <= 0 {
		gap = theme.Padding()
	}
	maxH := l.MaxHeight
	if maxH > size.Height {
		maxH = size.Height
	}
	x := float32(0)
	y := float32(0)
	if size.Height > maxH {
		y = (size.Height - maxH) / 2
	}
	first := true
	for _, o := range objects {
		if o == l.Float {
			continue
		}
		s := o.MinSize()
		if !first {
			x += gap
		}
		first = false
		h := s.Height
		if h > maxH {
			h = maxH
		}
		o.Move(fyne.NewPos(x, y))
		o.Resize(fyne.NewSize(s.Width, maxH))
		x += s.Width
	}
}
