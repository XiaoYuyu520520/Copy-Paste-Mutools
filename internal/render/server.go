package render

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"sync"
	"time"
)

// LocalServer 管理一个监听在本机未占用端口的临时 HTTP 服务。
// 生命周期：每次 Start 自动选一个空闲端口；Stop 释放绑定；程序退出前应调用 Stop。
type LocalServer struct {
	mu  sync.Mutex
	srv *http.Server
	ln  net.Listener
	URL string
}

// Start 停止旧服务并在随机空闲端口上启动 handler，返回访问地址。
func (s *LocalServer) Start(handler http.Handler) (string, error) {
	s.Stop()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("无法监听本机端口: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	url := fmt.Sprintf("http://127.0.0.1:%d", port)

	srv := &http.Server{Handler: handler}

	s.mu.Lock()
	s.srv = srv
	s.ln = ln
	s.URL = url
	s.mu.Unlock()

	go func() {
		// Serve 返回属于正常关闭或错误；错误丢弃，服务已被显式 Stop。
		_ = srv.Serve(ln)
	}()
	return url, nil
}

// Stop 关闭服务并释放端口（可安全重复调用）。
func (s *LocalServer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.srv == nil {
		return
	}
	srv, ln := s.srv, s.ln
	s.srv, s.ln = nil, nil
	s.URL = ""
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	if ln != nil {
		_ = ln.Close()
	}
}

// IsRunning 当前是否有服务在跑。
func (s *LocalServer) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.srv != nil
}

// URLSafe 返回最近一次启动的地址（无则空串）。
func (s *LocalServer) URLSafe() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.URL
}

// OpenBrowser 用系统默认浏览器打开 URL。
func OpenBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
