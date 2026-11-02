package mobile

import (
	"context"
	"fmt"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/netscan"
)

// mobileProbePorts 是手机端用于判断主机存活的 TCP 端口列表。
//
// 手机上没有可用的 ping（ICMP 需要 CAP_NET_RAW），只能靠 TCP 握手判断存活。
// 列表刻意选短：每多一个端口就多一轮并发拨号，扫 /24 时差别很明显。
var mobileProbePorts = []int{80, 443, 22, 445, 3389, 8080, 8000, 8888, 5000, 9090}

// scanState 是扫描页的状态机，避免 cancel 在 goroutine 间裸读写。
type scanState uint8

const (
	scanIdle     scanState = iota // 空闲，可开始
	scanRunning                   // 扫描进行中
	scanStopping                  // 用户点了停止，等待扫描 goroutine 收尾
)

// scanPage 是局域网扫描页。
type scanPage struct {
	root fyne.CanvasObject

	cidrEntry *widget.Entry
	startBtn  *widget.Button
	status    *widget.Label
	list      *widget.List

	mu     sync.Mutex
	hosts  []*netscan.Host
	state  scanState
	cancel context.CancelFunc
}

func newScanPage() *scanPage {
	p := &scanPage{}

	p.cidrEntry = widget.NewEntry()
	p.cidrEntry.SetPlaceHolder("192.168.1.0/24")
	p.startBtn = widget.NewButtonWithIcon("开始扫描", theme.SearchIcon(), p.toggle)
	p.status = widget.NewLabel("")

	p.list = widget.NewList(
		func() int {
			p.mu.Lock()
			defer p.mu.Unlock()
			return len(p.hosts)
		},
		func() fyne.CanvasObject { return widget.NewLabel("template") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			p.mu.Lock()
			if i < 0 || i >= len(p.hosts) {
				p.mu.Unlock()
				return
			}
			h := p.hosts[i]
			p.mu.Unlock()
			label := o.(*widget.Label)
			label.SetText(fmt.Sprintf("%s  %s  %s", h.IP, orDash(h.Hostname), openPortsText(h.OpenPorts)))
		},
	)

	top := container.NewVBox(
		container.NewBorder(nil, nil, nil, p.startBtn, p.cidrEntry),
		p.status,
	)
	p.root = container.NewBorder(top, nil, nil, nil, p.list)

	p.guessCIDR()
	return p
}

// guessCIDR 用本机网卡推断一个可扫描网段填进输入框。
func (p *scanPage) guessCIDR() {
	ifaces, err := netscan.ListInterfaces()
	if err != nil {
		return
	}
	for _, iface := range ifaces {
		if cidr := iface.FirstUsableSubnet(); cidr != "" {
			p.cidrEntry.SetText(cidr)
			return
		}
	}
}

// toggle 开始或停止扫描。
func (p *scanPage) toggle() {
	cidr := p.cidrEntry.Text

	p.mu.Lock()
	switch p.state {
	case scanRunning:
		// 只允许取消一次；后续点击在收尾前忽略。
		p.state = scanStopping
		cancel := p.cancel
		p.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		return
	case scanStopping:
		p.mu.Unlock()
		return
	}

	if cidr == "" {
		p.mu.Unlock()
		p.status.SetText("请先填写网段，例如 192.168.1.0/24")
		return
	}

	// 在启动 goroutine 前同步切换状态，防止连点启动多个扫描。
	ctx, cancel := context.WithCancel(context.Background())
	p.state = scanRunning
	p.cancel = cancel
	p.hosts = nil
	p.mu.Unlock()

	go p.scan(ctx, cancel, cidr)
}

// scan 执行一次扫描。
func (p *scanPage) scan(ctx context.Context, cancel context.CancelFunc, cidr string) {
	defer cancel()

	fyne.Do(func() {
		p.startBtn.SetText("停止")
		p.startBtn.SetIcon(theme.CancelIcon())
		p.status.SetText("正在扫描 " + cidr + " …")
		p.list.Refresh()
	})

	scanner := netscan.New(netscan.Options{
		PingTimeout:    mobileTimeout,
		TCPPortTimeout: mobileTimeout,
		TCPConcurrency: 24,
		WorkerCount:    96,
		UseTCPProbe:    true, // Android 上这是唯一可行的探测方式
		ProbePorts:     mobileProbePorts,
		LookupMAC:      netscan.MACExtractionSupported(),
		LookupHost:     true,
	})

	var scanned, total int
	err := scanner.Scan(ctx, cidr, func(ev netscan.Event) {
		switch ev.Type {
		case "start":
			total = ev.Total
		case "host":
			if ev.Host != nil && ev.Host.Alive {
				p.upsert(ev.Host)
			}
			scanned = ev.Scanned
		case "done", "cancelled":
			scanned = ev.Scanned
		}
		fyne.Do(func() {
			p.status.SetText(fmt.Sprintf("已扫 %d/%d，发现 %d 台", scanned, total, p.count()))
		})
	})

	cancelled := ctx.Err() != nil
	p.mu.Lock()
	p.cancel = nil
	p.state = scanIdle
	p.mu.Unlock()

	fyne.Do(func() {
		p.startBtn.SetText("开始扫描")
		p.startBtn.SetIcon(theme.SearchIcon())
		switch {
		case err != nil && cancelled:
			p.status.SetText(fmt.Sprintf("已停止，发现 %d 台", p.count()))
		case err != nil:
			p.status.SetText("扫描失败：" + err.Error())
		default:
			p.status.SetText(fmt.Sprintf("扫描完成，发现 %d 台", p.count()))
		}
	})
}

// upsert 插入或更新一台主机（扫描分两阶段，同一 IP 会来两次）。
func (p *scanPage) upsert(h *netscan.Host) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, old := range p.hosts {
		if old.IP == h.IP {
			p.hosts[i] = h
			fyne.Do(func() { p.list.Refresh() })
			return
		}
	}
	p.hosts = append(p.hosts, h)
	fyne.Do(func() { p.list.Refresh() })
}

// count 返回当前发现的主机数。
func (p *scanPage) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.hosts)
}

// orDash 空值显示成短横线。
func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// openPortsText 把端口列表渲染成文本。
func openPortsText(ports []int) string {
	if len(ports) == 0 {
		return ""
	}
	out := ""
	for i, p := range ports {
		if i > 0 {
			out += ","
		}
		out += fmt.Sprint(p)
	}
	return "[" + out + "]"
}
