package cli

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/netspeed"
	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/toolbox/diskm"
	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/toolbox/monitor"
	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/toolbox/share"
	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/toolbox/webassets"
)

// runShare 启动局域网文件共享服务。
func runShare(args []string) error {
	fs := flagSet("share")
	dir := fs.String("dir", ".", "要共享的目录")
	addr := fs.String("addr", "0.0.0.0:8000", "监听地址")
	if err := fs.Parse(args); err != nil {
		return err
	}

	frontend, err := webassets.ShareFS()
	if err != nil {
		return err
	}

	srv, err := share.New(share.Options{
		SharedDir:  *dir,
		Addr:       *addr,
		FrontendFS: httpFS(frontend),
	})
	if err != nil {
		return err
	}

	return serveUntilSignal("局域网文件共享", *addr, srv.ListenAndServe)
}

// runMonitor 启动开发板性能监视器。
func runMonitor(args []string) error {
	fs := flagSet("monitor")
	addr := fs.String("addr", "0.0.0.0:8888", "监听地址")
	interval := fs.Int("interval", 1, "状态推送间隔（秒）")
	if err := fs.Parse(args); err != nil {
		return err
	}

	frontend, err := webassets.MonitorFS()
	if err != nil {
		return err
	}

	srv := monitor.New(monitor.Options{
		Addr:       *addr,
		Interval:   durationSeconds(*interval),
		FrontendFS: httpFS(frontend),
	})

	return serveUntilSignal("开发板性能监视器", *addr, srv.ListenAndServe)
}

// runNetSpeed 启动局域网网速测试服务。
func runNetSpeed(args []string) error {
	fs := flagSet("netspeed")
	port := fs.String("port", netspeed.EnvPort("7777"), "监听端口")
	quiet := fs.Bool("quiet", false, "不写日志文件")
	if err := fs.Parse(args); err != nil {
		return err
	}

	addr := ":" + *port
	srv, err := netspeed.New(netspeed.Options{
		Addr:   addr,
		Quiet:  *quiet,
		LogDir: "logs",
	})
	if err != nil {
		return err
	}

	return serveUntilSignal("局域网网速测试", addr, srv.ListenAndServe)
}

// serveUntilSignal 在后台启动服务，打印访问地址，收到 Ctrl+C 后退出。
//
// 关键点：先起 goroutine 再等信号。若先等信号，服务永远不会开始监听。
func serveUntilSignal(title, addr string, serve func() error) error {
	errCh := make(chan error, 1)
	go func() { errCh <- serve() }()

	// 给监听一个很短的窗口，若立即报错（端口占用等）就直接返回。
	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("%s 启动失败: %w", title, err)
		}
		return nil
	case <-timeAfter(150):
	}

	printBanner(title, addr, localIPs())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case err := <-errCh:
		if err != nil {
			return err
		}
	case <-sigCh:
		fmt.Println("\n正在停止服务...")
	}
	return nil
}

// localIPs 返回本机可用于局域网访问的 IPv4 地址。
func localIPs() []string {
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

// printBanner 打印服务启动横幅。
func printBanner(title, addr string, ips []string) {
	suffix := portOf(addr)
	fmt.Println()
	fmt.Println("=======================================================")
	fmt.Printf("   %s\n", title)
	fmt.Println("=======================================================")
	fmt.Printf("   监听地址: %s\n", addr)
	fmt.Println("-------------------------------------------------------")
	fmt.Printf("   本机访问: http://localhost%s\n", suffix)
	for _, ip := range ips {
		fmt.Printf("   局域网  : http://%s%s\n", ip, suffix)
	}
	fmt.Println("-------------------------------------------------------")
	fmt.Println("   按 Ctrl+C 停止服务")
	fmt.Println("=======================================================")
	fmt.Println()
}

// portOf 从监听地址里取出 ":端口" 部分。
func portOf(addr string) string {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[i:]
		}
	}
	return ""
}

// runDiskMonitor 实时监控磁盘 I/O。
func runDiskMonitor(args []string) error {
	fs := flagSet("diskm")
	device := fs.String("device", "", "只监控指定设备（如 sda / nvme0n1）")
	interval := fs.Int("interval", 1, "采样间隔（秒）")
	count := fs.Int("count", 0, "采样次数，0 表示一直监控")
	if err := fs.Parse(args); err != nil {
		return err
	}

	opts := diskm.DefaultOptions()
	opts.Device = *device
	opts.Interval = durationSeconds(*interval)
	opts.Count = *count

	return diskm.Run(opts)
}
