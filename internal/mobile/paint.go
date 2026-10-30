package mobile

import (
	"fmt"
	"image/color"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/toolbox/paint"
)

// paintPage 是手机端的画板页。
//
// 直接复用 internal/toolbox/paint 的 Pad 控件：它内部用 image.RGBA 记录笔画，
// 触摸拖动通过 Fyne 的 Draggable 接口转换成坐标，手指画出来的效果与鼠标一致。
type paintPage struct {
	root  fyne.CanvasObject
	pad   *paint.Pad
	pen   color.Color
	width int
}

func newPaintPage() *paintPage {
	p := &paintPage{pen: color.RGBA{R: 0x3A, G: 0x96, B: 0xFF, A: 0xFF}, width: 6}

	// 手机上按屏幕比例给一个初始画布；Pad 会随控件大小自适应坐标。
	p.pad = paint.NewPad(1080, 1600, p.pen, p.width)

	sizeSlider := widget.NewSlider(1, 40)
	sizeSlider.SetValue(float64(p.width))
	sizeSlider.OnChanged = func(v float64) {
		p.width = int(v)
		p.pad.SetPen(p.pen, p.width)
	}

	colorBtn := widget.NewButtonWithIcon("", theme.ColorChromaticIcon(), func() {
		dialog.ShowColorPicker("画笔颜色", "选择一种颜色", func(c color.Color) {
			p.pen = c
			p.pad.SetPen(p.pen, p.width)
		}, fyne.CurrentApp().Driver().AllWindows()[0])
	})
	clearBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() { p.pad.Clear() })
	saveBtn := widget.NewButtonWithIcon("", theme.DocumentSaveIcon(), p.save)

	toolbar := container.NewVBox(
		container.NewBorder(nil, nil, nil, container.NewHBox(colorBtn, clearBtn, saveBtn), sizeSlider),
	)
	p.root = container.NewBorder(toolbar, nil, nil, nil, p.pad)
	return p
}

// save 把画布写进应用存储目录（Android 上无需额外权限）。
func (p *paintPage) save() {
	app := fyne.CurrentApp()
	name := fmt.Sprintf("drawing-%s.png", time.Now().Format("20060102-150405"))
	path := name
	if root := app.Storage().RootURI(); root != nil && root.Path() != "" {
		path = filepath.Join(root.Path(), name)
	}
	if err := p.pad.SaveImage(path); err != nil {
		dialog.ShowError(err, app.Driver().AllWindows()[0])
		return
	}
	dialog.ShowInformation("已保存", "图片已保存到：\n"+path, app.Driver().AllWindows()[0])
}
