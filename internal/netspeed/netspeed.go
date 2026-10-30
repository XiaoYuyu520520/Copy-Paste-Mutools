// Package netspeed 提供局域网网速测试服务。
//
// 这部分原本是 add/tools/局域网测速工具 里的独立 Go 模块（module netspeed），
// 现已并入主模块，前端资源放在 web/netspeed/dist，通过 go:embed 内嵌，
// 因此整个工具只需要一个可执行文件。
package netspeed

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/netspeed/handler"
	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/netspeed/loglib"
)

//go:embed web/netspeed/dist/*
var frontendFS embed.FS

// Options 是测速服务的配置。
type Options struct {
	// Addr 是监听地址，默认 ":7777"。
	Addr string
	// LogDir 是日志目录，默认 "logs"。
	LogDir string
	// Quiet 为 true 时不写日志文件。
	Quiet bool
}

// Server 是测速服务。
type Server struct {
	opts Options
	http *http.Server
}

// New 创建测速服务。
func New(opts Options) (*Server, error) {
	if opts.Addr == "" {
		opts.Addr = ":7777"
	}
	if opts.LogDir == "" {
		opts.LogDir = "logs"
	}
	if !opts.Quiet {
		if err := loglib.Init(opts.LogDir); err != nil {
			return nil, fmt.Errorf("初始化日志失败: %w", err)
		}
	}

	mux := http.NewServeMux()
	mux.Handle("/ws", handler.NewWSHandler())

	sub, err := fs.Sub(frontendFS, "web/netspeed/dist")
	if err != nil {
		return nil, fmt.Errorf("加载前端资源失败: %w", err)
	}
	mux.Handle("/", http.FileServer(http.FS(sub)))

	return &Server{
		opts: opts,
		http: &http.Server{
			Addr:              opts.Addr,
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
		},
	}, nil
}

// ListenAndServe 启动服务并阻塞。
func (s *Server) ListenAndServe() error {
	loglib.Info("NetSpeed 服务已启动: http://0.0.0.0%s", s.opts.Addr)
	return s.http.ListenAndServe()
}

// Shutdown 关闭服务。
func (s *Server) Shutdown() error {
	return s.http.Close()
}

// LocalAddrs 返回本机可用于局域网访问的 IPv4 地址列表。
func LocalAddrs() []string {
	out := []string{}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return out
	}
	for _, a := range addrs {
		ipNet, ok := a.(*net.IPNet)
		if !ok || ipNet.IP.IsLoopback() {
			continue
		}
		if ip4 := ipNet.IP.To4(); ip4 != nil {
			out = append(out, ip4.String())
		}
	}
	return out
}

// Port 从监听地址里解析出端口号。
func Port(addr string) string {
	_, p, err := net.SplitHostPort(addr)
	if err != nil {
		return "7777"
	}
	return p
}

// EnvPort 读取 PORT 环境变量，无效时返回默认值。
func EnvPort(defaultPort string) string {
	v := os.Getenv("PORT")
	if v == "" {
		return defaultPort
	}
	if _, err := strconv.Atoi(v); err != nil {
		log.Printf("PORT=%q 不是合法端口，使用默认 %s", v, defaultPort)
		return defaultPort
	}
	return v
}
