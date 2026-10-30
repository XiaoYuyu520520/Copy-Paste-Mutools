// Package netscan 把 internal/netscan 的扫描引擎包装成命令行接口。
//
// 迁移自 legacy/python 中的三份扫描脚本：
//   - 局域网设备扫描/network_windows.py（Windows 网关解析 + 多线程 ping）
//   - 局域网设备扫描/network_linux_v2.py（增强版：主机名 + MAC + 报告导出）
//   - wlan_scan/（跨平台高级扫描框架：子网检测、端口扫描、CSV/JSON 导出）
//
// 这些能力在 Go 侧已经由 internal/netscan 实现（ICMP + TCP 回退 + MAC +
// 主机名 + 开放端口），本包只负责 CLI 参数与文本/CSV 输出。
package netscan

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	core "github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/netscan"
)

// Options 是命令行扫描参数。
type Options struct {
	// CIDR 是要扫描的网段；为空时自动推断。
	CIDR string
	// TimeoutMS 是单主机探测超时（毫秒）。
	TimeoutMS int
	// Workers 是并发探测数。
	Workers int
	// PortsCSV 是要探测的 TCP 端口列表，空表示使用默认列表。
	PortsCSV string
	// UseTCPProbe 为 true 时在 ICMP 失败后回退到 TCP 探测。
	UseTCPProbe bool
	// LookupMAC 为 true 时解析 MAC 地址。
	LookupMAC bool
	// LookupHost 为 true 时反查主机名。
	LookupHost bool
}

// DefaultOptions 返回默认扫描参数。
func DefaultOptions() Options {
	return Options{
		TimeoutMS:   800,
		Workers:     64,
		PortsCSV:    "",
		UseTCPProbe: true,
		LookupMAC:   true,
		LookupHost:  true,
	}
}

// ScanRequest 是一次扫描请求。
type ScanRequest struct {
	Options Options
}

// ScanResult 是一次扫描的结果。
type ScanResult struct {
	CIDR       string
	StartedAt  time.Time
	FinishedAt time.Time
	Hosts      []*core.Host
	Scanned    int
	Total      int
}

// Scan 执行一次局域网扫描。
func Scan(req ScanRequest) (*ScanResult, error) {
	opts := req.Options
	if opts.TimeoutMS <= 0 {
		opts.TimeoutMS = 800
	}
	if opts.Workers <= 0 {
		opts.Workers = 64
	}

	cidr := strings.TrimSpace(opts.CIDR)
	if cidr == "" {
		guessed, err := GuessCIDR()
		if err != nil {
			return nil, err
		}
		cidr = guessed
	}

	ports, err := parsePorts(opts.PortsCSV)
	if err != nil {
		return nil, err
	}

	coreOpts := core.Options{
		PingTimeout:    time.Duration(opts.TimeoutMS) * time.Millisecond,
		TCPPortTimeout: time.Duration(opts.TimeoutMS) * time.Millisecond,
		TCPConcurrency: 16,
		WorkerCount:    opts.Workers,
		UseTCPProbe:    opts.UseTCPProbe,
		ProbePorts:     ports,
		LookupMAC:      opts.LookupMAC,
		LookupHost:     opts.LookupHost,
	}

	scanner := core.New(coreOpts)
	result := &ScanResult{CIDR: cidr, StartedAt: time.Now()}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	err = scanner.Scan(ctx, cidr, func(ev core.Event) {
		switch ev.Type {
		case "start":
			result.Total = ev.Total
		case "host":
			if ev.Host != nil {
				result.Hosts = append(result.Hosts, ev.Host)
			}
		case "done", "cancelled":
			result.Scanned = ev.Scanned
		}
	})
	result.FinishedAt = time.Now()

	// 同一次扫描里同一个 IP 可能上报两次（先存活后补充信息），按 IP 去重。
	result.Hosts = dedupe(result.Hosts)
	sort.Slice(result.Hosts, func(i, j int) bool {
		return compareIP(result.Hosts[i].IP, result.Hosts[j].IP) < 0
	})
	if err != nil && len(result.Hosts) == 0 {
		return nil, err
	}
	return result, nil
}

// GuessCIDR 根据本机网卡推断一个可扫描的网段。
func GuessCIDR() (string, error) {
	ifaces, err := core.ListInterfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		if cidr := iface.FirstUsableSubnet(); cidr != "" {
			return cidr, nil
		}
	}
	return "", fmt.Errorf("无法推断网段，请用 -cidr 手动指定")
}

// parsePorts 解析端口列表，空字符串返回默认端口。
func parsePorts(csvText string) ([]int, error) {
	csvText = strings.TrimSpace(csvText)
	if csvText == "" {
		return core.DefaultProbePorts, nil
	}
	out := []int{}
	for _, part := range strings.Split(csvText, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		p, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("非法端口号: %s", part)
		}
		if p < 1 || p > 65535 {
			return nil, fmt.Errorf("端口号超出范围: %d", p)
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		return core.DefaultProbePorts, nil
	}
	return out, nil
}

// dedupe 按 IP 去重，保留信息更完整的一条。
func dedupe(hosts []*core.Host) []*core.Host {
	byIP := map[string]*core.Host{}
	order := []string{}
	for _, h := range hosts {
		if h == nil {
			continue
		}
		existing, ok := byIP[h.IP]
		if !ok {
			byIP[h.IP] = h
			order = append(order, h.IP)
			continue
		}
		// 用信息更完整的那条覆盖。
		if len(h.Hostname) > len(existing.Hostname) ||
			(h.MAC != "" && existing.MAC == "") ||
			len(h.OpenPorts) > len(existing.OpenPorts) {
			byIP[h.IP] = h
		}
	}
	out := make([]*core.Host, 0, len(order))
	for _, ip := range order {
		out = append(out, byIP[ip])
	}
	return out
}

// compareIP 比较两个 IPv4 字符串，返回负数/0/正数。
func compareIP(a, b string) int {
	pa := splitIP(a)
	pb := splitIP(b)
	for i := 0; i < 4; i++ {
		if pa[i] != pb[i] {
			if pa[i] < pb[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

// splitIP 把点分十进制 IPv4 转成 4 个整数，非法时返回全 0。
func splitIP(s string) [4]int {
	var out [4]int
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return out
	}
	for i, p := range parts {
		v, err := strconv.Atoi(p)
		if err != nil {
			return [4]int{}
		}
		out[i] = v
	}
	return out
}

// Text 把扫描结果渲染成文本报告。
func (r *ScanResult) Text() string {
	var b strings.Builder
	b.WriteString("============================================================\n")
	b.WriteString("局域网设备扫描\n")
	b.WriteString(fmt.Sprintf("网段: %s\n", r.CIDR))
	b.WriteString(fmt.Sprintf("耗时: %s\n", r.FinishedAt.Sub(r.StartedAt).Truncate(time.Millisecond)))
	b.WriteString("============================================================\n")
	if len(r.Hosts) == 0 {
		b.WriteString("未发现活跃设备（可能被防火墙屏蔽了 ICMP）\n")
		return b.String()
	}
	b.WriteString(fmt.Sprintf("%-16s %-18s %-22s %-10s %s\n", "IP 地址", "MAC 地址", "主机名", "延迟", "开放端口"))
	b.WriteString("------------------------------------------------------------\n")
	for _, h := range r.Hosts {
		ports := "-"
		if len(h.OpenPorts) > 0 {
			strs := make([]string, 0, len(h.OpenPorts))
			for _, p := range h.OpenPorts {
				strs = append(strs, strconv.Itoa(p))
			}
			ports = strings.Join(strs, ",")
		}
		hostname := h.Hostname
		if hostname == "" {
			hostname = "-"
		}
		mac := h.MAC
		if mac == "" {
			mac = "-"
		}
		latency := "-"
		if h.Alive {
			latency = fmt.Sprintf("%dms", h.LatencyMs)
		}
		b.WriteString(fmt.Sprintf("%-16s %-18s %-22s %-10s %s\n", h.IP, mac, hostname, latency, ports))
	}
	b.WriteString("------------------------------------------------------------\n")
	b.WriteString(fmt.Sprintf("共发现 %d 台活跃设备。\n", len(r.Hosts)))
	return b.String()
}

// WriteCSV 把扫描结果导出为 CSV 文件。
func (r *ScanResult) WriteCSV(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write([]string{"ip", "mac", "hostname", "latency_ms", "method", "open_ports"}); err != nil {
		return err
	}
	for _, h := range r.Hosts {
		ports := make([]string, 0, len(h.OpenPorts))
		for _, p := range h.OpenPorts {
			ports = append(ports, strconv.Itoa(p))
		}
		row := []string{
			h.IP,
			h.MAC,
			h.Hostname,
			strconv.Itoa(int(h.LatencyMs)),
			string(h.Method),
			strings.Join(ports, ";"),
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}
