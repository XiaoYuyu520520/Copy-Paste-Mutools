package ui

import (
	"os"
	"runtime"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
)

// TestMain 为 CI 提供稳定的 UTF-8 语言环境。
//
// GitHub Actions 的 runner 默认 LANG=C，Fyne 会报 "Error parsing user locale C"，
// 并在 CJK 文本测量时触发 go-text/harfbuzz 的 slice panic（与本项目代码无关）。
// 这里在测试启动前换成一个合法 locale，避免随机失败。
func TestMain(m *testing.M) {
	if v := os.Getenv("LANG"); v == "" || v == "C" || v == "POSIX" {
		_ = os.Setenv("LANG", "en_US.UTF-8")
	}
	if v := os.Getenv("LC_ALL"); v == "" || v == "C" || v == "POSIX" {
		_ = os.Setenv("LC_ALL", "en_US.UTF-8")
	}

	// 与 Fyne 默认测试路径保持一致：单线程渲染，避免 text shaping 缓存被并发访问。
	runtime.GOMAXPROCS(1)
	lang.SystemLocale()
	test.NewApp()
	fyne.CurrentApp().Settings().SetTheme(theme.DefaultTheme())

	os.Exit(m.Run())
}
