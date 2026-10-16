// Package lanscan 提供「局域网设备扫描」的浏览器版服务。
//
// 复用 LAN Scan(MIT) 的 Web 界面与 SSE/会话协议，探测逻辑直接用本仓库的
// netscan 包。UI 侧只需在随机空闲端口启动本服务并打开系统浏览器即可，
// 不再依赖内嵌的 Fyne 表格页。
package lanscan

import (
	"embed"
	"io/fs"
)

//go:embed assets/*
var assetsFS embed.FS

// Assets 返回内嵌的浏览器静态资源文件系统（index.html / style.css / app.js）。
func Assets() fs.FS {
	sub, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		panic(err)
	}
	return sub
}
