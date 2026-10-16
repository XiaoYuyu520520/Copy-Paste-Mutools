package ui

import (
	"image"
	"image/color"
	"math"

	"fyne.io/fyne/v2/canvas"
)

// roundedGradient 生成一张「圆角 + 水平渐变」的位图。
//
// 背景：Fyne 的 canvas.LinearGradient 不支持圆角（没有 CornerRadius 字段），
// 而 canvas.Rectangle 支持圆角但不支持渐变。要同时满足「渐变蓝 + 圆角卡片」，
// 这里直接按像素栅格化一张 RGBA 位图，再用 canvas.Image 按拉伸模式显示。
//
// 参数：
//   - w, h   : 位图尺寸（像素）
//   - start  : 左侧颜色
//   - end    : 右侧颜色
//   - radius : 圆角半径（像素）
//
// 边缘用「到圆角矩形边界的距离」做 1 px 抗锯齿，避免锯齿感。
func roundedGradient(w, h int, start, end color.Color, radius float64) image.Image {
	if w <= 0 || h <= 0 {
		return image.NewRGBA(image.Rect(0, 0, 1, 1))
	}
	if radius < 0 {
		radius = 0
	}
	if maxR := math.Min(float64(w), float64(h)) / 2; radius > maxR {
		radius = maxR
	}

	sr, sg, sb, sa := toNRGBA(start)
	er, eg, eb, ea := toNRGBA(end)

	img := image.NewNRGBA(image.Rect(0, 0, w, h))

	// 圆角矩形的「内核矩形」：向内收缩 radius 后的矩形。
	kernelMinX := radius
	kernelMaxX := float64(w) - radius
	kernelMinY := radius
	kernelMaxY := float64(h) - radius

	for y := 0; y < h; y++ {
		py := float64(y) + 0.5
		for x := 0; x < w; x++ {
			px := float64(x) + 0.5

			// 计算 (px, py) 到内核矩形的最短距离，得到圆角覆盖度。
			cx := clampFloat(px, kernelMinX, kernelMaxX)
			cy := clampFloat(py, kernelMinY, kernelMaxY)
			d := math.Hypot(px-cx, py-cy)

			cov := radius + 0.5 - d
			if cov <= 0 {
				continue
			}
			if cov > 1 {
				cov = 1
			}

			// 水平渐变参数 t：0 = 左边，1 = 右边。
			t := 0.0
			if w > 1 {
				t = float64(x) / float64(w-1)
			}

			r := mixChannel(sr, er, t)
			g := mixChannel(sg, eg, t)
			b := mixChannel(sb, eb, t)
			a := mixChannel(sa, ea, t)

			img.SetNRGBA(x, y, color.NRGBA{
				R: r,
				G: g,
				B: b,
				A: uint8(float64(a)*cov + 0.5),
			})
		}
	}
	return img
}

// NewRoundedGradient 把 roundedGradient 包成一个可拉伸的 canvas.Image。
func newRoundedGradientImage(w, h int, start, end color.Color, radius float64) *canvas.Image {
	img := canvas.NewImageFromImage(roundedGradient(w, h, start, end, radius))
	img.FillMode = canvas.ImageFillStretch
	img.ScaleMode = canvas.ImageScaleSmooth
	return img
}

func toNRGBA(c color.Color) (uint8, uint8, uint8, uint8) {
	if c == nil {
		return 0, 0, 0, 0
	}
	n := color.NRGBAModel.Convert(c).(color.NRGBA)
	return n.R, n.G, n.B, n.A
}

func mixChannel(a, b uint8, t float64) uint8 {
	v := float64(a) + (float64(b)-float64(a))*t
	if v < 0 {
		v = 0
	}
	if v > 255 {
		v = 255
	}
	return uint8(v + 0.5)
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
