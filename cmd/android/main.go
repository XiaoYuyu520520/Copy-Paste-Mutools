// 提示词复合工具箱 · Android 入口。
//
// 与桌面版（cmd/merger）的区别：只加载移动端界面（internal/mobile），
// 不含模拟键盘输入、USB 枚举、桌面快捷方式等依赖桌面系统能力的功能。
//
// 构建（需要 Android SDK + NDK）：
//
//	fyne package -os android/arm64 \
//	  -app-id com.xiaoyuyu.promptmerger \
//	  -name "提示词工具箱" \
//	  -icon ico/icon.png
package main

import (
	"fyne.io/fyne/v2/app"

	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/mobile"
)

func main() {
	a := app.NewWithID("com.xiaoyuyu.promptmerger")
	win := mobile.New(a)
	win.ShowAndRun()
}
