package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// showContextMenu 在 pos（窗口画布绝对坐标）处弹出右键菜单。
//
// 所有右键菜单都走这里，保证：
//   - 菜单为空时不弹（避免出现空面板）；
//   - 使用当前窗口的画布，而不是 fyne.CurrentApp 的全部画布。
func showContextMenu(win fyne.Window, pos fyne.Position, items ...*fyne.MenuItem) {
	if win == nil || win.Canvas() == nil {
		return
	}
	showContextMenuByCanvas(win.Canvas(), pos, items...)
}

// showContextMenuByCanvas 是 showContextMenu 的画布版本，
// 供拿到的是画布而非窗口时使用（例如控件覆盖方法内）。
func showContextMenuByCanvas(c fyne.Canvas, pos fyne.Position, items ...*fyne.MenuItem) {
	if c == nil || len(items) == 0 {
		return
	}
	widget.ShowPopUpMenuAtPosition(fyne.NewMenu("", items...), c, pos)
}

// menuSeparator 返回一条菜单分隔线。
func menuSeparator() *fyne.MenuItem {
	return fyne.NewMenuItemSeparator()
}

// menuItemWithShortcut 构造带快捷键提示但**不注册**快捷键的菜单项。
//
// fyne.MenuItem.Shortcut 在窗口主菜单里会自动注册全局快捷键；
// 右键菜单里我们希望只显示提示文字，实际按键由 installShortcuts 统一注册，
// 否则同一组合键会被注册两次。
func menuItemWithShortcut(label string, key fyne.KeyName, mod fyne.KeyModifier, action func()) *fyne.MenuItem {
	item := fyne.NewMenuItem(label, action)
	item.Shortcut = &desktopShortcut{key: key, mod: mod}
	return item
}

// desktopShortcut 是只用于显示的快捷键描述。
type desktopShortcut struct {
	key fyne.KeyName
	mod fyne.KeyModifier
}

func (s *desktopShortcut) ShortcutName() string {
	prefix := ""
	if s.mod&fyne.KeyModifierControl != 0 {
		prefix += "Ctrl+"
	}
	if s.mod&fyne.KeyModifierAlt != 0 {
		prefix += "Alt+"
	}
	if s.mod&fyne.KeyModifierShift != 0 {
		prefix += "Shift+"
	}
	return prefix + string(s.key)
}

// buildMenu 按「项目 — nil 分隔 — 项目」的扁平列表构造菜单。
// 传入 nil 代表一条分隔线。
func buildMenu(entries ...*fyne.MenuItem) []*fyne.MenuItem {
	out := make([]*fyne.MenuItem, 0, len(entries))
	for _, e := range entries {
		if e == nil {
			if len(out) == 0 || out[len(out)-1].IsSeparator {
				continue // 开头或连续的分隔线不重复添加
			}
			out = append(out, menuSeparator())
			continue
		}
		out = append(out, e)
	}
	// 去掉末尾分隔线
	if n := len(out); n > 0 && out[n-1].IsSeparator {
		out = out[:n-1]
	}
	return out
}
