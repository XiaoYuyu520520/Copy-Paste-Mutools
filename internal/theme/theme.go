// Package theme 提供桌面端与移动端共用的调色板与 Fyne 主题。
//
// 以前桌面端把调色板写在 internal/ui/theme.go 里，移动端直接用 Fyne 默认主题，
// 两端看起来像两个产品。现在把「颜色 + 尺寸」抽到这里：
//   - internal/ui     继续导出 NewModernTheme / CurrentPalette（保持既有调用不变）
//   - internal/mobile 改用 NewTheme，亮暗两套与桌面端一致
//
// 配色基调参考 JetBrains Darcula / VSCode Dark+：
//   - 背景 #1E2333 深墨蓝，卡片 #2C3247，强调色 #3A96FF
//   - 主文本 #E1E4EB 柔和浅灰，避免纯白
package theme

import (
	"image/color"

	"fyne.io/fyne/v2"
	ftheme "fyne.io/fyne/v2/theme"
)

// Palette 是 UI 调色板，集中在主题层维护以便复用与切换。
type Palette struct {
	Bg         color.Color   // 窗口/页面背景
	Card       color.Color   // 卡片/输入框/侧栏背景
	CardAlt    color.Color   // 副卡片 / hover
	Fg         color.Color   // 主前景（文字/图标）
	FgMuted    color.Color   // 次级文字（占位/未选标签）
	FgDisabled color.Color   // 只读/禁用控件文字
	FgOnAcc    color.Color   // 强调色之上的前景
	Accent     color.Color   // 强调色（按钮、复选框、下划线、滑块手柄）
	NavStart   color.Color   // 左侧选中导航渐变起点
	NavEnd     color.Color   // 左侧选中导航渐变终点
	AccentBg   color.Color   // 强调色低透明度底色
	Border     color.Color   // 1px 边框 / 分割线
	Shadow     color.Color   // 阴影
	Success    color.Color   // 成功状态
	Warning    color.Color   // 警告状态
	Code       fyne.Resource // 等宽字体
}

// Light 返回浅色调色板。
func Light() Palette {
	return Palette{
		Bg:         color.NRGBA{R: 0xF5, G: 0xF7, B: 0xFA, A: 0xFF},
		Card:       color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF},
		CardAlt:    color.NRGBA{R: 0xEE, G: 0xF2, B: 0xFF, A: 0xFF},
		Fg:         color.NRGBA{R: 0x1F, G: 0x29, B: 0x37, A: 0xFF},
		FgMuted:    color.NRGBA{R: 0x6B, G: 0x72, B: 0x80, A: 0xFF},
		FgDisabled: color.NRGBA{R: 0x4B, G: 0x55, B: 0x63, A: 0xFF},
		FgOnAcc:    color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF},
		Accent:     color.NRGBA{R: 0x3A, G: 0x96, B: 0xFF, A: 0xFF},
		NavStart:   color.NRGBA{R: 0xDC, G: 0xEB, B: 0xFF, A: 0xFF},
		NavEnd:     color.NRGBA{R: 0xB4, G: 0xD4, B: 0xFF, A: 0xFF},
		AccentBg:   color.NRGBA{R: 0x3A, G: 0x96, B: 0xFF, A: 0x22},
		Border:     color.NRGBA{R: 0xE5, G: 0xE7, B: 0xEB, A: 0xFF},
		Shadow:     color.NRGBA{R: 0x14, G: 0x1B, B: 0x2C, A: 0x18},
		Success:    color.NRGBA{R: 0x10, G: 0xB9, B: 0x81, A: 0xFF},
		Warning:    color.NRGBA{R: 0xF5, G: 0x9D, B: 0x3B, A: 0xFF},
		Code:       ftheme.TextMonospaceFont(),
	}
}

// Dark 返回暗色调色板。
func Dark() Palette {
	return Palette{
		Bg:         color.NRGBA{R: 0x1E, G: 0x23, B: 0x33, A: 0xFF},
		Card:       color.NRGBA{R: 0x2C, G: 0x32, B: 0x47, A: 0xFF},
		CardAlt:    color.NRGBA{R: 0x35, G: 0x3C, B: 0x55, A: 0xFF},
		Fg:         color.NRGBA{R: 0xE1, G: 0xE4, B: 0xEB, A: 0xFF},
		FgMuted:    color.NRGBA{R: 0x9A, G: 0xA3, B: 0xB5, A: 0xFF},
		FgDisabled: color.NRGBA{R: 0xAF, G: 0xB7, B: 0xC8, A: 0xFF},
		FgOnAcc:    color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF},
		Accent:     color.NRGBA{R: 0x3A, G: 0x96, B: 0xFF, A: 0xFF},
		NavStart:   color.NRGBA{R: 0x2C, G: 0x32, B: 0x47, A: 0xFF},
		NavEnd:     color.NRGBA{R: 0x2A, G: 0x62, B: 0xB4, A: 0xFF},
		AccentBg:   color.NRGBA{R: 0x3A, G: 0x96, B: 0xFF, A: 0x1F},
		Border:     color.NRGBA{R: 0x3A, G: 0x41, B: 0x5A, A: 0xFF},
		Shadow:     color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x55},
		Success:    color.NRGBA{R: 0x4A, G: 0xDE, B: 0x80, A: 0xFF},
		Warning:    color.NRGBA{R: 0xFF, G: 0xB8, B: 0x6C, A: 0xFF},
		Code:       ftheme.TextMonospaceFont(),
	}
}

// modernTheme 把调色板覆盖到 Fyne 默认主题上。
type modernTheme struct {
	pal  Palette
	base fyne.Theme
}

// NewTheme 构造主题：dark=true 给 IDE 风格的暗色面板，false 走柔和浅色面板。
func NewTheme(dark bool) fyne.Theme {
	base := ftheme.DarkTheme()
	pal := Dark()
	if !dark {
		base = ftheme.LightTheme()
		pal = Light()
	}
	return &modernTheme{pal: pal, base: base}
}

// Palette 返回主题的调色板（外部只读）。
func PaletteOf(t fyne.Theme) (Palette, bool) {
	if m, ok := t.(*modernTheme); ok {
		return m.pal, true
	}
	return Palette{}, false
}

func (m *modernTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch name {
	case ftheme.ColorNameBackground:
		return m.pal.Bg
	case ftheme.ColorNameForeground:
		return m.pal.Fg
	case ftheme.ColorNameDisabled:
		return m.pal.FgDisabled
	case ftheme.ColorNameError:
		return m.pal.Warning
	case ftheme.ColorNameFocus:
		return m.pal.Accent
	case ftheme.ColorNameHover:
		return m.pal.CardAlt
	case ftheme.ColorNamePlaceHolder:
		return m.pal.FgMuted
	case ftheme.ColorNamePressed:
		return m.pal.AccentBg
	case ftheme.ColorNamePrimary:
		return m.pal.Accent
	case ftheme.ColorNameInputBackground:
		return m.pal.Card
	case ftheme.ColorNameInputBorder:
		return m.pal.Border
	case ftheme.ColorNameMenuBackground:
		return m.pal.Card
	case ftheme.ColorNameOverlayBackground:
		return m.pal.Card
	case ftheme.ColorNameShadow:
		return m.pal.Shadow
	case ftheme.ColorNameScrollBar:
		return m.pal.Border
	case ftheme.ColorNameScrollBarBackground:
		return color.NRGBA{R: 0, G: 0, B: 0, A: 0}
	case ftheme.ColorNameHyperlink:
		return m.pal.Accent
	case ftheme.ColorNameButton:
		return m.pal.CardAlt
	}
	return m.base.Color(name, fyne.ThemeVariant(0))
}

func (m *modernTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return m.base.Icon(name)
}

func (m *modernTheme) Font(style fyne.TextStyle) fyne.Resource {
	if style.Monospace {
		return m.pal.Code
	}
	return m.base.Font(style)
}

func (m *modernTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case ftheme.SizeNamePadding:
		return 8
	case ftheme.SizeNameInlineIcon:
		return 20
	case ftheme.SizeNameScrollBar:
		return 10
	case ftheme.SizeNameScrollBarSmall:
		return 4
	case ftheme.SizeNameSeparatorThickness:
		return 1
	case ftheme.SizeNameInputBorder:
		return 1
	case ftheme.SizeNameInputRadius:
		// 文本域 / 下拉框圆角 4 px（Entry 原生使用该主题值）。
		return 4
	case ftheme.SizeNameButtonRadius:
		return 4
	case ftheme.SizeNameSelectionRadius:
		return 4
	case ftheme.SizeNameCardRadius:
		return 6
	case ftheme.SizeNamePopupRadius, ftheme.SizeNameMenuRadius:
		return 6
	case ftheme.SizeNameDialogRadius, ftheme.SizeNameInnerWindowRadius:
		return 8
	case ftheme.SizeNameScrollBarRadius:
		return 5
	}
	return m.base.Size(name)
}
