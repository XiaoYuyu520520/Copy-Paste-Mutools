// Package webassets 提供 toolbox 各子命令使用的前端静态资源。
//
// 所有资源都通过 go:embed 内嵌进可执行文件，保证「下载即用」，
// 不需要在目标机器上另外部署前端目录。
package webassets

import (
	"embed"
	"io/fs"
)

//go:embed share monitor
var assets embed.FS

// ShareFS 返回局域网文件共享的前端资源。
func ShareFS() (fs.FS, error) {
	return fs.Sub(assets, "share")
}

// MonitorFS 返回性能监视器的前端资源。
func MonitorFS() (fs.FS, error) {
	return fs.Sub(assets, "monitor")
}
