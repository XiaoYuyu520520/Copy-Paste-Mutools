// Package paint 提供一个最小可用的画板。
//
// 迁移自 legacy/python/小工具v1.0/code/最终优化版画板.py（tkinter）。
// 原脚本功能：自由笔画、选颜色、调粗细、清空画布、首次运行选分辨率。
// 这里用 Fyne 重新实现，并把配置存进用户配置目录。
package paint

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// Config 是画板的持久化配置。
type Config struct {
	Resolution string `json:"resolution"`
	BrushColor string `json:"brush_color"`
	BrushSize  int    `json:"brush_size"`
}

// DefaultConfig 返回默认配置。
func DefaultConfig() Config {
	return Config{Resolution: "800x600", BrushColor: "#0000ff", BrushSize: 3}
}

// configPath 返回配置文件的完整路径。
func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir = filepath.Join(dir, "prompt-merger-toolbox")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "paint.json"), nil
}

// LoadConfig 读取配置，文件不存在时返回默认值。
func LoadConfig() Config {
	cfg := DefaultConfig()
	path, err := configPath()
	if err != nil {
		return cfg
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(b, &cfg)
	if cfg.BrushSize <= 0 {
		cfg.BrushSize = DefaultConfig().BrushSize
	}
	if cfg.Resolution == "" {
		cfg.Resolution = DefaultConfig().Resolution
	}
	return cfg
}

// SaveConfig 写入配置。
func SaveConfig(cfg Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}

// ParseResolution 把 "800x600" 解析成宽高。
func ParseResolution(s string) (int, int) {
	var w, h int
	if _, err := fmt.Sscanf(s, "%dx%d", &w, &h); err != nil || w <= 0 || h <= 0 {
		return 800, 600
	}
	return w, h
}

// Pad 是画板控件：把拖拽轨迹画到一张 RGBA 图上。
type Pad struct {
	widget.BaseWidget

	img      *image.RGBA
	imgView  *canvas.Image
	pen      color.Color
	penWidth int

	lastX, lastY float32
	drawing      bool
	onChange     func()
}

// NewPad 创建一块指定像素尺寸的画板。
func NewPad(width, height int, pen color.Color, penWidth int) *Pad {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)
	p := &Pad{img: img, pen: pen, penWidth: penWidth}
	p.ExtendBaseWidget(p)
	return p
}

// CreateRenderer 实现 fyne.Widget。
func (p *Pad) CreateRenderer() fyne.WidgetRenderer {
	p.imgView = canvas.NewImageFromImage(p.img)
	p.imgView.FillMode = canvas.ImageFillStretch
	return widget.NewSimpleRenderer(p.imgView)
}

// Dragged 实现 fyne.Draggable，把拖拽轨迹连成线。
func (p *Pad) Dragged(e *fyne.DragEvent) {
	x, y := p.toImageCoords(e.Position)
	if !p.drawing {
		p.drawing = true
		p.lastX, p.lastY = x, y
		p.dot(x, y)
	} else {
		p.line(p.lastX, p.lastY, x, y)
		p.lastX, p.lastY = x, y
	}
	p.Refresh()
}

// DragEnd 实现 fyne.Draggable。
func (p *Pad) DragEnd() {
	p.drawing = false
	if p.onChange != nil {
		p.onChange()
	}
}

// SetPen 修改画笔颜色与粗细。
func (p *Pad) SetPen(c color.Color, width int) {
	if width <= 0 {
		width = 1
	}
	p.pen = c
	p.penWidth = width
}

// Clear 清空画布。
func (p *Pad) Clear() {
	draw.Draw(p.img, p.img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)
	p.Refresh()
}

// Image 返回底层图像，便于导出。
func (p *Pad) Image() *image.RGBA { return p.img }

// toImageCoords 把控件坐标换算成图像像素坐标。
func (p *Pad) toImageCoords(pos fyne.Position) (float32, float32) {
	size := p.Size()
	if size.Width <= 0 || size.Height <= 0 {
		return 0, 0
	}
	sx := float32(p.img.Bounds().Dx()) / size.Width
	sy := float32(p.img.Bounds().Dy()) / size.Height
	return pos.X * sx, pos.Y * sy
}

// line 用 Bresenham 画一条带粗细的线。
func (p *Pad) line(x0, y0, x1, y1 float32) {
	steps := int(maxf(absf(x1-x0), absf(y1-y0)))
	if steps == 0 {
		p.dot(x1, y1)
		return
	}
	for i := 0; i <= steps; i++ {
		t := float32(i) / float32(steps)
		p.dot(x0+(x1-x0)*t, y0+(y1-y0)*t)
	}
}

// dot 以 (cx, cy) 为圆心填充一个实心圆。
func (p *Pad) dot(cx, cy float32) {
	r := float32(p.penWidth) / 2
	if r < 1 {
		r = 1
	}
	minX := int(cx - r)
	maxX := int(cx + r)
	minY := int(cy - r)
	maxY := int(cy + r)
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			dx := float32(x) - cx
			dy := float32(y) - cy
			if dx*dx+dy*dy <= r*r {
				p.img.Set(x, y, p.pen)
			}
		}
	}
}

// SaveImage 把画布导出成 PNG 文件。
func (p *Pad) SaveImage(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return encodePNG(f, p.img)
}

func absf(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func maxf(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

// ShowDialog 是给上层 GUI 用的薄封装，避免重复引用 dialog 包。
func ShowDialog(w fyne.Window, title, message string, parent fyne.CanvasObject) {
	dialog.ShowCustom(title, "确定", parent, w)
}
