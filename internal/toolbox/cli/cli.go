// Package cli 实现 toolbox 命令行工具的参数解析与子命令分发。
package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/data"
	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/toolbox/bench"
	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/toolbox/netscan"
	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/toolbox/shortcut"
	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/toolbox/sysinfo"
	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/toolbox/usb"
)

// Usage 返回顶层帮助文本。
func Usage() string {
	var b strings.Builder
	b.WriteString("提示词复合工具箱 · toolbox\n\n")
	b.WriteString("用法: toolbox <子命令> [参数]\n\n")
	b.WriteString("可用子命令:\n")
	b.WriteString("  tree        打印目录树（自动忽略构建/依赖目录）\n")
	b.WriteString("  cpubench    CPU 单核性能基准测试\n")
	b.WriteString("  diskbench   磁盘顺序读写吞吐测试\n")
	b.WriteString("  netscan     局域网设备扫描（ICMP/TCP/MAC/主机名）\n")
	b.WriteString("  sysinfo     系统硬件与资源信息（支持 --json）\n")
	b.WriteString("  shortcut    生成桌面网址快捷方式\n")
	b.WriteString("  usb         列出 USB 设备（可生成 Markdown 快照）\n")
	b.WriteString("  share       启动局域网文件共享服务（浏览器访问）\n")
	b.WriteString("  monitor     启动开发板性能监视器（SSE 实时推送）\n")
	b.WriteString("  netspeed    启动局域网网速测试服务\n")
	b.WriteString("  diskm       实时监控磁盘 I/O 吞吐\n")
	b.WriteString("\n用 toolbox <子命令> -h 查看子命令帮助。\n")
	return b.String()
}

// Run 解析并执行子命令。
func Run(args []string) error {
	if len(args) == 0 {
		fmt.Print(Usage())
		return nil
	}

	cmd, rest := args[0], args[1:]
	switch cmd {
	case "tree":
		return runTree(rest)
	case "cpubench":
		return runCPUBench(rest)
	case "diskbench":
		return runDiskBench(rest)
	case "netscan":
		return runNetScan(rest)
	case "sysinfo":
		return runSysInfo(rest)
	case "shortcut":
		return runShortcut(rest)
	case "usb":
		return runUSB(rest)
	case "share":
		return runShare(rest)
	case "monitor":
		return runMonitor(rest)
	case "netspeed":
		return runNetSpeed(rest)
	case "diskm":
		return runDiskMonitor(rest)
	case "help", "-h", "--help":
		fmt.Print(Usage())
		return nil
	default:
		return fmt.Errorf("未知子命令 %q\n\n%s", cmd, Usage())
	}
}

// runTree 打印目录树。
func runTree(args []string) error {
	fs := flag.NewFlagSet("tree", flag.ContinueOnError)
	ignoreHidden := fs.Bool("hidden", false, "同时显示隐藏文件（默认隐藏 . 开头的项）")
	ignoreDefaults := fs.Bool("no-default-ignore", false, "不忽略常见的构建/依赖目录")
	extraDirs := fs.String("ignore", "", "额外忽略的目录名，逗号分隔")
	keywords := fs.String("keyword", "", "额外忽略的关键词（名称包含即忽略），逗号分隔")
	maxDepth := fs.Int("depth", 0, "最大递归深度，0 表示不限")
	if err := fs.Parse(args); err != nil {
		return err
	}

	root := "."
	if fs.NArg() > 0 {
		root = fs.Arg(0)
	}

	text, stat, err := buildTree(root, *ignoreHidden, *ignoreDefaults, splitList(*extraDirs), splitList(*keywords), *maxDepth)
	if err != nil {
		return err
	}
	fmt.Print(text)
	fmt.Fprintf(os.Stderr, "\n目录 %d 个，文件 %d 个，已忽略 %d 项\n", stat.Dirs, stat.Files, stat.Ignored)
	return nil
}

// buildTree 生成目录树文本，受 maxDepth 限制。
func buildTree(root string, ignoreHidden, ignoreDefaults bool, extraDirs, keywords []string, maxDepth int) (string, *data.TreeStat, error) {
	opts := data.TreeOptions{
		IgnoreHidden:      ignoreHidden,
		IgnoreDefaultDirs: ignoreDefaults,
		ExtraDirs:         extraDirs,
		Keywords:          keywords,
	}
	if maxDepth <= 0 {
		return data.BuildTree(root, opts)
	}
	return data.BuildTreeLimited(root, opts, maxDepth)
}

// splitList 把逗号分隔的字符串切成列表。
func splitList(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// runCPUBench 执行 CPU 基准测试。
func runCPUBench(args []string) error {
	fs := flag.NewFlagSet("cpubench", flag.ContinueOnError)
	intLimit := fs.Int("int-limit", 0, "整数测试上限（默认 50000）")
	floatIters := fs.Int("float-iters", 0, "浮点测试迭代次数（默认 3000000）")
	noWarmup := fs.Bool("no-warmup", false, "跳过 CPU 预热")
	if err := fs.Parse(args); err != nil {
		return err
	}

	opts := bench.DefaultCPUOptions()
	if *intLimit > 0 {
		opts.IntLimit = *intLimit
	}
	if *floatIters > 0 {
		opts.FloatIters = *floatIters
	}
	opts.Warmup = !*noWarmup

	result := bench.RunCPU(opts)
	fmt.Print(bench.FormatCPUResult(systemSummary(), runtime.NumCPU(), result))
	return nil
}

// runDiskBench 执行磁盘基准测试。
func runDiskBench(args []string) error {
	fs := flag.NewFlagSet("diskbench", flag.ContinueOnError)
	path := fs.String("file", "", "测试文件路径（默认使用临时文件）")
	sizeMB := fs.Int("size", 1024, "测试文件大小（MB）")
	blockMB := fs.Int("block", 64, "读写块大小（MB）")
	keep := fs.Bool("keep", false, "保留测试文件")
	overwrite := fs.Bool("overwrite", false, "允许覆盖已存在的测试文件（默认拒绝，防止丢数据）")
	dropCaches := fs.Bool("drop-caches", false, "尝试清除系统页缓存（仅 Linux，需 root，会影响同机其他程序）")
	if err := fs.Parse(args); err != nil {
		return err
	}

	opts := bench.DefaultDiskOptions()
	if *path != "" {
		opts.FilePath = *path
	}
	if *sizeMB > 0 {
		opts.SizeBytes = int64(*sizeMB) << 20
	}
	if *blockMB > 0 {
		opts.BlockBytes = int64(*blockMB) << 20
	}
	opts.KeepFile = *keep
	opts.Overwrite = *overwrite
	opts.DropCaches = *dropCaches

	result, err := bench.RunDisk(opts)
	if err != nil {
		if errors.Is(err, bench.ErrFileExists) {
			return fmt.Errorf("%w\n提示：换一个 -file 路径，或加 -overwrite 明确允许覆盖", err)
		}
		return err
	}
	fmt.Print(bench.FormatDiskResult(result))
	return nil
}

// runNetScan 扫描局域网设备。
func runNetScan(args []string) error {
	opts := netscan.DefaultOptions()
	fs := flag.NewFlagSet("netscan", flag.ContinueOnError)
	cidr := fs.String("cidr", "", "要扫描的网段，如 192.168.1.0/24（默认自动推断）")
	timeout := fs.Int("timeout", opts.TimeoutMS, "单主机探测超时（毫秒）")
	workers := fs.Int("workers", opts.Workers, "并发探测数")
	ports := fs.String("ports", opts.PortsCSV, "要探测的 TCP 端口，逗号分隔（空表示只探常用端口）")
	out := fs.String("out", "", "把结果导出为 CSV 文件")
	if err := fs.Parse(args); err != nil {
		return err
	}

	opts.CIDR = *cidr
	opts.TimeoutMS = *timeout
	opts.Workers = *workers
	opts.PortsCSV = *ports

	result, err := netscan.Scan(netscan.ScanRequest{Options: opts})
	if err != nil {
		return err
	}
	fmt.Print(result.Text())
	if *out != "" {
		if err := result.WriteCSV(*out); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "\n结果已写入 %s\n", *out)
	}
	return nil
}

// runSysInfo 输出系统信息。
func runSysInfo(args []string) error {
	fs := flag.NewFlagSet("sysinfo", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "以 JSON 格式输出")
	target := fs.String("target", "all", "采集目标：system/cpu/gpu/memory/disk/all")
	if err := fs.Parse(args); err != nil {
		return err
	}

	data, err := sysinfo.Query(*target)
	if err != nil {
		return err
	}
	if *asJSON {
		text, err := sysinfo.FormatJSON(data)
		if err != nil {
			return err
		}
		fmt.Println(text)
		return nil
	}
	if all, ok := data.(sysinfo.AllInfo); ok {
		fmt.Print(sysinfo.FormatHuman(all))
		return nil
	}
	text, err := sysinfo.FormatJSON(data)
	if err != nil {
		return err
	}
	fmt.Println(text)
	return nil
}

// runShortcut 生成网址快捷方式。
func runShortcut(args []string) error {
	fs := flag.NewFlagSet("shortcut", flag.ContinueOnError)
	name := fs.String("name", "", "快捷方式名称")
	url := fs.String("url", "", "目标网址（http:// 或 https:// 开头）")
	dir := fs.String("dir", "", "保存目录（默认桌面）")
	format := fs.String("format", "auto", "输出格式：auto/bat/url/desktop")
	if err := fs.Parse(args); err != nil {
		return err
	}

	opts := shortcut.Options{
		Name:   *name,
		URL:    *url,
		Dir:    *dir,
		Format: shortcut.Format(*format),
	}
	if opts.Dir == "" {
		d, err := shortcut.DefaultDir()
		if err != nil {
			return err
		}
		opts.Dir = d
	}

	result, err := shortcut.Create(opts)
	if err != nil {
		return err
	}
	action := "已创建"
	if result.Overwrote {
		action = "已覆盖"
	}
	fmt.Printf("快捷方式%s: %s\n", action, result.Path)
	return nil
}

// runUSB 列出 USB 设备。
func runUSB(args []string) error {
	fs := flag.NewFlagSet("usb", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "以 JSON 格式输出")
	markdown := fs.String("markdown", "", "把 Markdown 快照写入指定文件")
	monitor := fs.Bool("monitor", false, "持续监控设备插拔")
	interval := fs.Int("interval", 2, "监控轮询间隔（秒）")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *monitor {
		fmt.Println("监控 USB 设备变化中，Ctrl+C 退出...")
		return usb.Monitor(durationSeconds(*interval), func(devices []usb.Device) {
			fmt.Printf("[%s] 设备数量: %d\n", nowString(), len(devices))
			for _, d := range devices {
				fmt.Println("  ", d.String())
			}
		})
	}

	snapshot, err := usb.Snapshot()
	if err != nil {
		return err
	}
	if *asJSON {
		text, err := sysinfo.FormatJSON(snapshot)
		if err != nil {
			return err
		}
		fmt.Println(text)
		return nil
	}
	if *markdown != "" {
		if err := os.WriteFile(*markdown, []byte(snapshot.Markdown()), 0644); err != nil {
			return err
		}
		fmt.Printf("Markdown 快照已写入 %s\n", *markdown)
		return nil
	}
	fmt.Printf("USB 设备 (%s，共 %d 个)\n", snapshot.GeneratedAt, len(snapshot.Devices))
	for _, d := range snapshot.Devices {
		fmt.Println("  ", d.String())
	}
	return nil
}
