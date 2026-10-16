// 入口：创建 Fyne 应用、按配置加载主题与多标签配置、显示主窗口并启动事件循环。
// 退出时由 ui.MainApp.SetOnClosed 触发 flushAll()，把内存配置同步落盘到 config.json。
package main

import (
	"fyne.io/fyne/v2/app"

	"prompt-merger-native/data"
	"prompt-merger-native/ui"
)

func main() {
	cfg, err := data.LoadConfig()
	if err != nil {
		cfg = data.DefaultConfig()
	}

	a := app.New()
	a.Settings().SetTheme(ui.NewModernTheme(cfg.ThemeMode == "dark"))

	win := ui.NewMainWindow(a, &cfg)
	win.ShowAndRun()
}
