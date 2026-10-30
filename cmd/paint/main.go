// 简易画板：迁移自 legacy/python/小工具v1.0/code/最终优化版画板.py。
package main

import (
	"image/color"
	"os"
	"path/filepath"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/toolbox/paint"
)

func main() {
	cfg := paint.LoadConfig()
	width, height := paint.ParseResolution(cfg.Resolution)

	a := app.NewWithID("com.xiaoyuyu.toolbox.paint")
	w := a.NewWindow("简易画板")

	pen, err := parseColor(cfg.BrushColor)
	if err != nil {
		pen = color.Black
	}

	pad := paint.NewPad(width, height, pen, cfg.BrushSize)

	// ---- 工具栏 ----
	brushSize := widget.NewSlider(1, 50)
	brushSize.SetValue(float64(cfg.BrushSize))
	brushSize.OnChanged = func(v float64) {
		pad.SetPen(pen, int(v))
	}

	colorBtn := widget.NewButton("选择颜色", func() {
		dialog.ShowColorPicker("选择颜色", "请选择画笔颜色", func(c color.Color) {
			pen = c
			pad.SetPen(pen, int(brushSize.Value))
			cfg.BrushColor = hexOf(c)
			_ = paint.SaveConfig(cfg)
		}, w)
	})

	clearBtn := widget.NewButtonWithIcon("清空画布", theme.DeleteIcon(), func() {
		pad.Clear()
	})

	saveBtn := widget.NewButtonWithIcon("保存 PNG", theme.DocumentSaveIcon(), func() {
		saveDialog(w, pad)
	})

	toolbar := container.NewHBox(
		widget.NewLabel("画笔粗细:"),
		brushSize,
		colorBtn,
		clearBtn,
		saveBtn,
	)

	content := container.NewBorder(toolbar, nil, nil, nil, pad)
	w.SetContent(content)
	w.Resize(fyne.NewSize(float32(width), float32(height)+60))
	w.ShowAndRun()
}

// saveDialog 弹出保存对话框并把画布导出成 PNG。
func saveDialog(w fyne.Window, pad *paint.Pad) {
	d := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil || writer == nil {
			return
		}
		defer writer.Close()

		name := writer.URI().Name()
		if filepath.Ext(name) == "" {
			name += ".png"
		}
		path := filepath.Join(filepath.Dir(writer.URI().Path()), name)
		if err := pad.SaveImage(path); err != nil {
			dialog.ShowError(err, w)
			return
		}
		dialog.ShowInformation("已保存", "画布已保存到:\n"+path, w)
	}, w)
	d.SetFilter(storage.NewExtensionFileFilter([]string{".png"}))
	d.SetFileName("drawing.png")
	d.Resize(fyne.NewSize(760, 520))
	d.Show()
}

// parseColor 解析 #RRGGBB 形式的颜色。
func parseColor(s string) (color.Color, error) {
	if len(s) != 7 || s[0] != '#' {
		return nil, os.ErrInvalid
	}
	v, err := strconv.ParseUint(s[1:], 16, 32)
	if err != nil {
		return nil, err
	}
	return color.RGBA{
		R: uint8(v >> 16),
		G: uint8(v >> 8),
		B: uint8(v),
		A: 255,
	}, nil
}

// hexOf 把颜色转成 #RRGGBB。
func hexOf(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return "#" + pad2(r>>8) + pad2(g>>8) + pad2(b>>8)
}

// pad2 把一个字节补成两位十六进制。
func pad2(v uint32) string {
	const hexDigits = "0123456789abcdef"
	return string([]byte{hexDigits[(v>>4)&0xF], hexDigits[v&0xF]})
}
