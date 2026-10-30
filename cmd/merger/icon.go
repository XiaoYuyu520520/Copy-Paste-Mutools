package main

import (
	_ "embed"
	"os"
	"path/filepath"
)

// ico/icon.png 是应用图标源文件，构建时内嵌进可执行文件，
// 供 Fyne 在运行时设置窗口/任务栏图标。
//
//go:embed ico/icon.png
var appIconPNG []byte

// iconBytes 返回内嵌的 PNG 图标数据。
func iconBytes() []byte { return appIconPNG }

// iconSourcePath 返回磁盘上 ico/icon.png 的候选路径。
// 打包后图标可能与可执行文件分离存放，因此按多个位置依次查找。
func iconSourcePath() string {
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)
		candidates := []string{
			filepath.Join(dir, "ico", "icon.png"),
			filepath.Join(dir, "icon.png"),
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				return c
			}
		}
	}
	return "ico/icon.png"
}
