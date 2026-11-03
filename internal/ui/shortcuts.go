package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
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

// installShortcuts 注册全局快捷键，避免主线程阻塞任务。
//
// 支持：
//   - Ctrl/Cmd+N 新建标签
//   - Ctrl/Cmd+S 立即保存
//   - Ctrl/Cmd+W 关闭当前标签
//   - Ctrl/Cmd+1..9 切换标签
//   - Ctrl/Cmd+Enter 在编辑器页立即复制合并结果
//   - Ctrl/Cmd+F 聚焦编辑器（提示可用模板搜索）
func (m *MainApp) installShortcuts(win fyne.Window) {
	win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyN, Modifier: desktop.ControlModifier}, func(fyne.Shortcut) {
		m.addBlankTab()
		m.setStatus("已新建标签")
	})
	win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyN, Modifier: desktop.SuperModifier}, func(fyne.Shortcut) {
		m.addBlankTab()
		m.setStatus("已新建标签")
	})
	win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyS, Modifier: desktop.ControlModifier}, func(fyne.Shortcut) {
		m.saveConfigNow()
		m.setStatus("配置已保存")
	})
	win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyS, Modifier: desktop.SuperModifier}, func(fyne.Shortcut) {
		m.saveConfigNow()
		m.setStatus("配置已保存")
	})
	win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyW, Modifier: desktop.ControlModifier}, func(fyne.Shortcut) {
		m.confirmCloseTab()
	})

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
	win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyEnter, Modifier: desktop.ControlModifier}, copyShortcut)
	win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyEnter, Modifier: desktop.SuperModifier}, copyShortcut)

	// Ctrl+1..9：切换标签。
	for i := 0; i < 9; i++ {
		idx := i
		key := digitKey(idx)
		if key == "" {
			continue
		}
		handler := func(fyne.Shortcut) {
			if idx < len(m.bundles) {
				m.selectTab(idx)
			}
		}
		win.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: key, Modifier: desktop.ControlModifier}, handler)
	}
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

// 编译期断言：确保 widget 包被使用。
var _ = widget.NewLabel
