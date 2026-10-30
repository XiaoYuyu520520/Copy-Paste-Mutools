// Package render 提供把内容「以本地随机端口 HTTP 服务 + 系统浏览器」渲染的能力。
//
// 纯离线：marked / highlight.js / github 风格样式已通过 go:embed 内嵌进二进制，
// 浏览器页面不会访问任何外部 CDN。
//
// 主题策略：亮色与夜间各用一套显式 CSS（github-markdown-light / -dark、github / github-dark），
// 页面手动切换并互斥启用，不依赖 prefers-color-scheme，避免系统配色把正文与代码高亮拆成两套。
package render

import (
	"embed"
	"io"
	"io/fs"
	"net/http"
)

//go:embed assets/*
var assetsFS embed.FS

// Assets 提供内嵌静态资源（/assets/...）的子文件系统。
func Assets() fs.FS {
	sub, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		panic(err)
	}
	return sub
}

// NewPageHandler 把内嵌静态资源（/assets/...）与一个根页面组到同一 mux，离线自足。
func NewPageHandler(page string) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(Assets()))))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, page)
	})
	return mux
}
