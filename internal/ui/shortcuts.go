package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// editorContent 返回当前激活标签的编辑器，没有则返回 nil。
func (m *MainApp) editorContent() *TabEditor {
	idx := m.tabBar.Selected()
	if idx < 0 || idx >= len(m.bundles) {
		return nil
	}
	return m.bundles[idx].editor
}

// installShortcuts 注册全局快捷键。组合键一览见 shortcut_catalog.go，
// F1 会弹出同一份目录，菜单与弹窗不会各自漂移。
//
// 覆盖：
//   - Ctrl/Cmd+N 新建标签；Ctrl+Shift+N 新增区块
//   - Ctrl/Cmd+S 立即保存；Ctrl+W 关闭当前标签
//   - Ctrl+Tab / Ctrl+Shift+Tab 下一个 / 上一个标签（循环）
//   - Ctrl/Cmd+1..9 切换标签（9 = 最后一个）
//   - Ctrl/Cmd+Enter 复制合并结果
//   - Ctrl+F 聚焦当前输入框；Ctrl+L 定位标签名
//   - Ctrl+Shift+R 重命名当前标签
//   - Alt+↑ / Alt+↓ 当前区块上移 / 下移
//   - Ctrl+J AI 聊天；Ctrl+Shift+M Markdown；Ctrl+Shift+L 局域网扫描
//   - F1 快捷键一览
func (m *MainApp) installShortcuts(win fyne.Window) {
	c := win.Canvas()

	// ctrl / super 双注册：macOS 上 Cmd 与 Windows/Linux 上 Ctrl 等价。
	pair := func(key fyne.KeyName, mod fyne.KeyModifier, fn func(fyne.Shortcut)) {
		c.AddShortcut(&desktop.CustomShortcut{KeyName: key, Modifier: mod}, fn)
		c.AddShortcut(&desktop.CustomShortcut{KeyName: key, Modifier: mod | fyne.KeyModifierSuper}, fn)
	}

	pair(fyne.KeyN, fyne.KeyModifierControl, func(fyne.Shortcut) {
		m.addBlankTab()
		m.setStatus("已新建标签")
	})
	pair(fyne.KeyN, fyne.KeyModifierControl|fyne.KeyModifierShift, func(fyne.Shortcut) {
		if e := m.editorContent(); e != nil && m.feature == featEditor {
			e.AddBlockAtEnd()
			m.setStatus("已在末尾新增区块")
		}
	})
	pair(fyne.KeyS, fyne.KeyModifierControl, func(fyne.Shortcut) {
		m.saveWithFeedback()
	})
	pair(fyne.KeyW, fyne.KeyModifierControl, func(fyne.Shortcut) {
		m.confirmCloseTab()
	})

	// Ctrl+Tab / Ctrl+Shift+Tab：循环切换标签。
	nextTab := func(delta int) {
		if n := len(m.bundles); n > 0 {
			m.selectTab(((m.tabBar.Selected()+delta)%n + n) % n)
		}
	}
	c.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyTab, Modifier: fyne.KeyModifierControl},
		func(fyne.Shortcut) { nextTab(1) })
	c.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyTab, Modifier: fyne.KeyModifierControl | fyne.KeyModifierShift},
		func(fyne.Shortcut) { nextTab(-1) })

	// Ctrl+Enter：复制当前合并结果。
	copyShortcut := func(fyne.Shortcut) {
		if e := m.editorContent(); e != nil {
			text := e.preview.Text
			if text == "" {
				m.setStatus("暂无合并内容")
				return
			}
			fyne.CurrentApp().Clipboard().SetContent(text)
			m.setStatus(fmt.Sprintf("已复制合并结果（%d 字符）", len([]rune(text))))
		}
	}
	pair(fyne.KeyEnter, fyne.KeyModifierControl, copyShortcut)
	pair(fyne.KeyReturn, fyne.KeyModifierControl, copyShortcut)

	// Ctrl+1..9：切换标签；9 跳到最后一个（浏览器惯例）。
	for i := 0; i < 9; i++ {
		key := digitKey(i)
		if key == "" {
			continue
		}
		idx := i
		win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: key, Modifier: fyne.KeyModifierControl},
			func(fyne.Shortcut) {
				if idx == 8 {
					idx = len(m.bundles) - 1
				}
				if idx >= 0 && idx < len(m.bundles) {
					m.selectTab(idx)
				}
			})
	}

	// Ctrl+F：聚焦「当前区块」的提示词输入框。
	pair(fyne.KeyF, fyne.KeyModifierControl, func(fyne.Shortcut) {
		if e := m.editorContent(); e != nil {
			if row := e.activeRow(); row != nil && row.prompt != nil {
				win.Canvas().Focus(row.prompt)
				return
			}
			m.setStatus("没有可聚焦的输入框")
		}
	})

	// Ctrl+L：定位到标签名输入框。
	pair(fyne.KeyL, fyne.KeyModifierControl, func(fyne.Shortcut) {
		if e := m.editorContent(); e != nil && e.nameEntry != nil {
			win.Canvas().Focus(e.nameEntry)
		}
	})

	// Ctrl+Shift+R：重命名当前标签（弹输入框，影响标签栏标题）。
	pair(fyne.KeyR, fyne.KeyModifierControl|fyne.KeyModifierShift, func(fyne.Shortcut) {
		m.showRenameDialog()
	})

	// Alt+↑ / Alt+↓：移动当前（最后聚焦的）区块。
	pair(fyne.KeyUp, fyne.KeyModifierAlt, func(fyne.Shortcut) {
		if e := m.editorContent(); e != nil {
			e.moveActiveBlock(-1)
		}
	})
	pair(fyne.KeyDown, fyne.KeyModifierAlt, func(fyne.Shortcut) {
		if e := m.editorContent(); e != nil {
			e.moveActiveBlock(1)
		}
	})

	// Ctrl+J / Ctrl+Shift+M / Ctrl+Shift+L：打开浏览器工具。
	pair(fyne.KeyJ, fyne.KeyModifierControl, func(fyne.Shortcut) { m.openAiChat() })
	pair(fyne.KeyM, fyne.KeyModifierControl|fyne.KeyModifierShift, func(fyne.Shortcut) { m.openMarkdown() })
	pair(fyne.KeyL, fyne.KeyModifierControl|fyne.KeyModifierShift, func(fyne.Shortcut) { m.openLanScan() })

	// F1：快捷键一览。
	c.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyF1}, func(fyne.Shortcut) {
		m.showShortcutHelp()
	})
}

// showShortcutHelp 弹出快捷键一览窗口。
//
// 两列布局：左动作、右组合键（等宽字体），按分组排好顺序（见 shortcutHelp）。
func (m *MainApp) showShortcutHelp() {
	rows := shortcutHelp()
	box := container.NewVBox()
	for _, row := range rows {
		key := widget.NewLabelWithStyle(row[1], fyne.TextAlignTrailing, fyne.TextStyle{Monospace: true})
		box.Add(container.NewBorder(nil, nil, widget.NewLabel(row[0]), key, nil))
	}
	scroll := container.NewVScroll(box)
	scroll.SetMinSize(fyne.NewSize(380, 360))
	dialog.ShowCustom("快捷键一览（F1）", "关闭", scroll, m.win)
}

// renamePromptText 是重命名对话框的辅助文本。
const renamePromptText = "新的标签名称"

// showRenameDialog 弹出重命名对话框并同步标签栏与配置。
func (m *MainApp) showRenameDialog() {
	idx := m.tabBar.Selected()
	if idx < 0 || idx >= len(m.bundles) {
		m.setStatus("没有可重命名的标签")
		return
	}
	e := m.bundles[idx].editor
	entry := widget.NewEntry()
	entry.SetText(e.Title)
	dialog.ShowForm("重命名标签", "确定", "取消", []*widget.FormItem{
		{Text: renamePromptText, Widget: entry},
	}, func(ok bool) {
		if !ok {
			return
		}
		name := strings.TrimSpace(entry.Text)
		e.Title = name
		e.Config.Name = name
		e.nameEntry.SetText(name)
		e.scheduleSave()
		if e.OnRename != nil {
			e.OnRename()
		}
		m.setStatus("已重命名为「" + name + "」")
	}, m.win)
}

func digitKey(i int) fyne.KeyName {
	switch i {
	case 0:
		return fyne.Key1
	case 1:
		return fyne.Key2
	case 2:
		return fyne.Key3
	case 3:
		return fyne.Key4
	case 4:
		return fyne.Key5
	case 5:
		return fyne.Key6
	case 6:
		return fyne.Key7
	case 7:
		return fyne.Key8
	case 8:
		return fyne.Key9
	}
	return ""
}
