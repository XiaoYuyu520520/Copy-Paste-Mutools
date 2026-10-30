package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Palette 是 UI 调色板，集中在主题层维护以便复用与切换。
//
// 暗色基调参考 JetBrains Darcula / VSCode Dark+：
//   - 背景：#1E2333 深墨蓝
//   - 控件卡片背景：#2C3247
//   - 强调色（Accent）：#3A96FF 现代蔚蓝
//   - 主文本：#E1E4EB 柔和浅灰，避免纯白
type Palette struct {
	Bg         color.Color   // 窗口/页面背景
	Card       color.Color   // 卡片/输入框/侧栏背景
	CardAlt    color.Color   // 副卡片 / hover
	Fg         color.Color   // 主前景（文字/图标）
	FgMuted    color.Color   // 次级文字（占位/未选标签）
	FgDisabled color.Color   // 只读/禁用控件文字（合并预览、Tree 结果）
	FgOnAcc    color.Color   // 强调色之上的前景
	Accent     color.Color   // 强调色（按钮、复选框、下划线、滑块手柄）
	NavStart   color.Color   // 左侧选中导航渐变起点
	NavEnd     color.Color   // 左侧选中导航渐变终点
	AccentBg   color.Color   // 强调色低透明度底色（标签栏背景/hover）
	Border     color.Color   // 1px 边框 / 分割线
	Shadow     color.Color   // 阴影
	Success    color.Color   // 成功状态
	Warning    color.Color   // 警告状态
	Code       fyne.Resource // 等宽字体（编辑器文本域）
}

func lightPalette() Palette {
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
		Code:       theme.TextMonospaceFont(),
	}
}

func darkPalette() Palette {
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
		Code:       theme.TextMonospaceFont(),
	}
}

// modernTheme 把自定义调色板覆盖到 Fyne 默认主题上。
type modernTheme struct {
	pal  Palette
	base fyne.Theme
}

// NewModernTheme 构造一个新的主题实例。
//
// dark=true 给 IDE 风格的暗色面板；false 走柔和浅色面板。
func NewModernTheme(dark bool) fyne.Theme {
	base := theme.DarkTheme()
	pal := darkPalette()
	if !dark {
		base = theme.LightTheme()
		pal = lightPalette()
	}
	return &modernTheme{pal: pal, base: base}
}

// CurrentPalette 返回当前主题的调色板，找不到时退回暗色。
//
// 给非主题模块（自定义卡片容器等）使用，避免重复解析 Fyne 的颜色名。
func CurrentPalette() Palette {
	if a := fyne.CurrentApp(); a != nil {
		if th := a.Settings().Theme(); th != nil {
			if m, ok := th.(*modernTheme); ok {
				return m.pal
			}
		}
	}
	return darkPalette()
}

func (m *modernTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return m.pal.Bg
	case theme.ColorNameForeground:
		return m.pal.Fg
	case theme.ColorNameDisabled:
		return m.pal.FgDisabled
	case theme.ColorNameError:
		return m.pal.Warning
	case theme.ColorNameFocus:
		return m.pal.Accent
	case theme.ColorNameHover:
		return m.pal.CardAlt
	case theme.ColorNamePlaceHolder:
		return m.pal.FgMuted
	case theme.ColorNamePressed:
		return m.pal.AccentBg
	case theme.ColorNamePrimary:
		return m.pal.Accent
	case theme.ColorNameInputBackground:
		return m.pal.Card
	case theme.ColorNameInputBorder:
		return m.pal.Border
	case theme.ColorNameMenuBackground:
		return m.pal.Card
	case theme.ColorNameOverlayBackground:
		return m.pal.Card
	case theme.ColorNameShadow:
		return m.pal.Shadow
	case theme.ColorNameScrollBar:
		return m.pal.Border
	case theme.ColorNameScrollBarBackground:
		return color.NRGBA{R: 0, G: 0, B: 0, A: 0}
	case theme.ColorNameHyperlink:
		return m.pal.Accent
	case theme.ColorNameButton:
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
	case theme.SizeNamePadding:
		return 8
	case theme.SizeNameInlineIcon:
		return 20
	case theme.SizeNameScrollBar:
		return 10
	case theme.SizeNameScrollBarSmall:
		return 4
	case theme.SizeNameSeparatorThickness:
		return 1
	case theme.SizeNameInputBorder:
		return 1
	case theme.SizeNameInputRadius:
		// 文本域 / 下拉框圆角 4 px（Entry 原生使用该主题值）。
		return 4
	case theme.SizeNameButtonRadius:
		// 按钮圆角 4 px。
		return 4
	case theme.SizeNameSelectionRadius:
		return 4
	case theme.SizeNameCardRadius:
		return 6
	case theme.SizeNamePopupRadius, theme.SizeNameMenuRadius:
		return 6
	case theme.SizeNameDialogRadius, theme.SizeNameInnerWindowRadius:
		return 8
	case theme.SizeNameScrollBarRadius:
		return 5
	}
	return m.base.Size(name)
}
