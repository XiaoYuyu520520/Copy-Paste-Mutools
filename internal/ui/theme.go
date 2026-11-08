package ui

import (
	"fyne.io/fyne/v2"

	shared "github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/theme"
)

// Palette 是 UI 调色板。
//
// 定义已经移到 internal/theme（桌面端与移动端共用），这里保留类型别名，
// 让桌面端既有调用（CurrentPalette()、palette 字段）不用改。
type Palette = shared.Palette

// lightPalette 返回浅色调色板。
func lightPalette() Palette { return shared.Light() }

// darkPalette 返回暗色调色板。
func darkPalette() Palette { return shared.Dark() }

// NewModernTheme 构造一个新的主题实例。
//
// dark=true 给 IDE 风格的暗色面板；false 走柔和浅色面板。
// 与移动端共用 internal/theme 的实现，两端观感一致。
func NewModernTheme(dark bool) fyne.Theme {
	return shared.NewTheme(dark)
}

// CurrentPalette 返回当前主题的调色板，找不到时退回暗色。
//
// 给非主题模块（自定义卡片容器等）使用，避免重复解析 Fyne 的颜色名。
func CurrentPalette() Palette {
	if a := fyne.CurrentApp(); a != nil {
		if th := a.Settings().Theme(); th != nil {
			if p, ok := shared.PaletteOf(th); ok {
				return p
			}
		}
	}
	return darkPalette()
}
