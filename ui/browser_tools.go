package ui

import (
	"fmt"
	"net/http"
	"path/filepath"
	"sync"

	"prompt-merger-native/chat"
	"prompt-merger-native/data"
	"prompt-merger-native/lanscan"
	"prompt-merger-native/render"
)

// BrowserTool 管理一个「启动本地随机空闲端口服务 + 系统浏览器打开」的工具
// （局域网扫描 / Markdown 渲染 / AI 聊天）。
//
// 首次点击启动并打开浏览器；服务已在跑时再次点击只重新打开浏览器，
// 避免打断页面中正在进行的扫描/预览。程序退出由 flushAll 调用 Stop 释放。
type BrowserTool struct {
	mu       sync.Mutex
	svc      *render.LocalServer
	handler  http.Handler
	stopHook func()
	url      string
}

func newBrowserTool(handler http.Handler, stopHook func()) *BrowserTool {
	return &BrowserTool{svc: &render.LocalServer{}, handler: handler, stopHook: stopHook}
}

// newLanScanTool 局域网设备扫描浏览器版。
func newLanScanTool() *BrowserTool {
	srv := lanscan.New(lanscan.Assets(), lanscan.ScanDefaults{})
	// 网页选项（TCP 回退/MAC/主机名/SSH 模式等）记忆到 exe 同目录，跨随机端口生效
	srv.SetPrefsPath(filepath.Join(data.ExecDir(), "lanscan-options.json"))
	return newBrowserTool(srv.Handler(), srv.CancelAll)
}

// newMarkdownTool Markdown 渲染浏览器版：
// 拖入 / 打开 .md 即全屏渲染（左右对照、亮暗主题在页内切换），.pgm 走地图编辑器。
// docs 由原生窗口的拖放回调写入，页面通过 /api/events 热切换到新文档。
func newMarkdownTool(docs *render.DocStore) *BrowserTool {
	return newBrowserTool(render.NewMarkdownEditorHandler(docs), nil)
}

// newAiChatTool AI 聊天浏览器版；get 每次请求读取最新设置（AI 设置页保存）。
// 会话历史记忆到 exe 同目录 aichat-history.json，跨随机端口可恢复。
func newAiChatTool(get func() chat.Settings) *BrowserTool {
	chat.SetHistoryPath(filepath.Join(data.ExecDir(), "aichat-history.json"))
	return newBrowserTool(chat.New(render.Assets(), get), nil)
}

// Open 确保服务在跑并打开浏览器。返回 (是否首次启动, 访问地址, error)。
func (t *BrowserTool) Open() (bool, string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.url != "" {
		if err := render.OpenBrowser(t.url); err != nil {
			return false, t.url, fmt.Errorf("打开浏览器失败: %w", err)
		}
		return false, t.url, nil
	}

	url, err := t.svc.Start(t.handler)
	if err != nil {
		return false, "", err
	}
	t.url = url

	if err := render.OpenBrowser(url); err != nil {
		return true, url, fmt.Errorf("服务已启动但打开浏览器失败: %w", err)
	}
	return true, url, nil
}

// URL 返回当前地址；未启动为空。
func (t *BrowserTool) URL() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.url
}

// Stop 取消内部活动（扫描会话）并关闭服务、释放端口。
func (t *BrowserTool) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.stopHook != nil {
		t.stopHook()
	}
	if t.svc != nil {
		t.svc.Stop()
	}
	t.url = ""
}
