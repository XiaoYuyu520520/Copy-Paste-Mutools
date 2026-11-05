package mobile

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/data"
	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/netspeed"
	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/toolbox/monitor"
	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/toolbox/share"
	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/toolbox/sysinfo"
	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/toolbox/webassets"
)

const (
	// mobileTimeout 是扫描/探测单主机的超时。
	mobileTimeout = 900 * time.Millisecond
	// mobileInterval 是状态推送与端口就绪轮询的间隔。
	mobileInterval = 500 * time.Millisecond
)

// morePage 收纳低频但仍有用的功能：AI 设置、本地服务、系统信息、关于。
type morePage struct {
	store *store
	app   fyne.App
	root  fyne.CanvasObject

	svc *serviceRunner
}

func newMorePage(s *store, a fyne.App) *morePage {
	p := &morePage{store: s, app: a, svc: &serviceRunner{}}

	acc := widget.NewAccordion(
		widget.NewAccordionItem("AI 设置", p.aiSettings()),
		widget.NewAccordionItem("本地服务", p.services()),
		widget.NewAccordionItem("系统信息", p.systemInfo()),
		widget.NewAccordionItem("关于", p.about()),
	)
	// 默认展开 AI 设置，方便首次配置。
	acc.Open(0)

	p.root = container.NewVScroll(acc)
	return p
}

// aiSettings 返回 API 配置表单（支持多个服务商）。
func (p *morePage) aiSettings() fyne.CanvasObject {
	cfg := p.store.Config()
	cfg.EnsureProviders()
	providers := append([]data.AIProvider(nil), cfg.Providers...)
	active := cfg.ActiveProvider
	if active < 0 || active >= len(providers) {
		active = 0
	}

	name := widget.NewEntry()
	name.SetPlaceHolder("服务商名称，例如 OpenAI / DeepSeek / 本地 Ollama")

	baseURL := widget.NewEntry()
	baseURL.SetPlaceHolder("https://api.openai.com/v1")

	apiKey := widget.NewPasswordEntry()
	apiKey.SetPlaceHolder("sk-...（本地服务可留空）")

	model := widget.NewEntry()
	model.SetPlaceHolder("gpt-4o-mini")

	system := widget.NewMultiLineEntry()
	system.SetPlaceHolder("可选的系统提示词")
	system.Wrapping = fyne.TextWrapWord

	status := widget.NewLabel("")

	load := func(i int) {
		if i < 0 || i >= len(providers) {
			return
		}
		active = i
		name.SetText(providers[i].Name)
		baseURL.SetText(providers[i].BaseURL)
		apiKey.SetText(providers[i].APIKey)
		model.SetText(providers[i].Model)
		system.SetText(providers[i].System)
	}

	var providerSel *widget.Select
	providerSel = widget.NewSelect(nil, func(string) {
		idx := providerSel.SelectedIndex()
		if idx >= 0 && idx < len(providers) {
			load(idx)
		}
	})
	refreshOpts := func() {
		opts := make([]string, len(providers))
		for i, pr := range providers {
			if pr.Name == "" {
				opts[i] = fmt.Sprintf("服务商 %d", i+1)
			} else {
				opts[i] = pr.Name
			}
		}
		providerSel.Options = opts
		if active >= 0 && active < len(opts) {
			providerSel.SetSelectedIndex(active)
		}
	}

	load(active)
	refreshOpts()

	addBtn := widget.NewButton("新增", func() {
		providers = append(providers, data.AIProvider{Name: fmt.Sprintf("服务商 %d", len(providers)+1)})
		active = len(providers) - 1
		refreshOpts()
		load(active)
		status.SetText("已新增服务商，填写后保存")
	})
	delBtn := widget.NewButton("删除", func() {
		if len(providers) <= 1 {
			status.SetText("至少保留一个服务商")
			return
		}
		providers = append(providers[:active], providers[active+1:]...)
		if active >= len(providers) {
			active = len(providers) - 1
		}
		refreshOpts()
		load(active)
		status.SetText("已删除，记得保存")
	})

	save := widget.NewButtonWithIcon("保存配置", theme.DocumentSaveIcon(), func() {
		newProvider := data.AIProvider{
			ID:      providers[active].ID,
			Name:    strings.TrimSpace(name.Text),
			BaseURL: strings.TrimSpace(baseURL.Text),
			APIKey:  strings.TrimSpace(apiKey.Text),
			Model:   strings.TrimSpace(model.Text),
			System:  system.Text,
		}
		if newProvider.Name == "" {
			newProvider.Name = "未命名服务"
		}
		providers[active] = newProvider
		activeID := newProvider.ID
		p.store.Update(func(c *data.Config) {
			c.SelectProvidersByID(providers, activeID)
		})
		if err := p.store.Save(); err != nil {
			status.SetText("保存失败：" + err.Error())
			return
		}
		cfg = p.store.Config()
		providers = append([]data.AIProvider(nil), cfg.Providers...)
		active = cfg.ActiveProvider
		refreshOpts()
		status.SetText("已保存")
	})

	return container.NewVBox(
		container.NewBorder(nil, nil, widget.NewLabel("服务商"), container.NewHBox(addBtn, delBtn), providerSel),
		bold("名称"), name,
		bold("API 地址"), baseURL,
		bold("API Key"), apiKey,
		bold("模型"), model,
		bold("系统提示词"), system,
		save, status,
	)
}

// services 返回本地服务面板：文件共享 / 性能监视 / 网速测试。
func (p *morePage) services() fyne.CanvasObject {
	status := widget.NewLabel("未启动")
	urlEntry := widget.NewEntry()
	urlEntry.SetPlaceHolder("启动后这里显示访问地址")
	urlEntry.Disable()

	openBtn := widget.NewButtonWithIcon("用浏览器打开", theme.ComputerIcon(), func() {
		if u := strings.TrimSpace(urlEntry.Text); u != "" {
			_ = p.app.OpenURL(parseURL(u))
		}
	})
	openBtn.Disable()

	stopBtn := widget.NewButtonWithIcon("停止", theme.CancelIcon(), func() {
		p.svc.stop()
		status.SetText("已停止")
		urlEntry.SetText("")
		openBtn.Disable()
	})

	startShare := widget.NewButton("文件共享", func() {
		shared := p.app.Storage().RootURI().Path()
		fe, err := webassets.ShareFS()
		if err != nil {
			status.SetText("资源加载失败：" + err.Error())
			return
		}
		srv, err := share.New(share.Options{
			SharedDir:  shared,
			Addr:       "127.0.0.1:8000",
			FrontendFS: http.FS(fe),
		})
		if err != nil {
			status.SetText("启动失败：" + err.Error())
			return
		}
		p.svc.start("文件共享", "127.0.0.1:8000", srv.ListenAndServe, srv.Shutdown, status, urlEntry, openBtn)
	})

	startMonitor := widget.NewButton("性能监视", func() {
		fe, err := webassets.MonitorFS()
		if err != nil {
			status.SetText("资源加载失败：" + err.Error())
			return
		}
		srv := monitor.New(monitor.Options{
			Addr:       "127.0.0.1:8888",
			Interval:   time.Second,
			FrontendFS: http.FS(fe),
		})
		p.svc.start("性能监视", "127.0.0.1:8888", srv.ListenAndServe, srv.Shutdown, status, urlEntry, openBtn)
	})

	startSpeed := widget.NewButton("网速测试", func() {
		srv, err := netspeed.New(netspeed.Options{Addr: "127.0.0.1:7777", Quiet: true})
		if err != nil {
			status.SetText("启动失败：" + err.Error())
			return
		}
		p.svc.start("网速测试", "127.0.0.1:7777", srv.ListenAndServe, srv.Shutdown, status, urlEntry, openBtn)
	})

	return container.NewVBox(
		widget.NewLabel("服务只在手机本机回环（127.0.0.1）监听，供本机浏览器打开；\n暂不支持电脑访问手机，跨设备互传需要后续版本做局域网监听与配对。"),
		container.NewGridWithColumns(3, startShare, startMonitor, startSpeed),
		urlEntry, openBtn, stopBtn, status,
	)
}

// systemInfo 返回系统信息面板。
func (p *morePage) systemInfo() fyne.CanvasObject {
	label := widget.NewLabel("点击下方按钮采集")
	label.Wrapping = fyne.TextWrapWord

	refresh := widget.NewButtonWithIcon("采集", theme.ViewRefreshIcon(), func() {
		label.SetText(sysinfo.FormatHuman(sysinfo.CollectAll()))
	})

	return container.NewVBox(refresh, label)
}

// about 返回关于页。
func (p *morePage) about() fyne.CanvasObject {
	text := "提示词复合工具箱 · Android\n\n" +
		"手机端保留：提示词合并、AI 聊天、局域网扫描、画板、本地服务。\n\n" +
		"桌面端独有：模拟键盘输入、USB 枚举、桌面快捷方式、磁盘基准。\n\n" +
		"https://github.com/XiaoYuyu520520/Copy-Paste-Mutools"
	return widget.NewLabelWithStyle(text, fyne.TextAlignLeading, fyne.TextStyle{})
}

// bold 生成小标题。
func bold(s string) *widget.Label {
	return widget.NewLabelWithStyle(s, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
}

// parseURL 把字符串转成 URL，失败时回落到回环地址。
func parseURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil || u.Scheme == "" {
		return &url.URL{Scheme: "http", Host: "127.0.0.1"}
	}
	return u
}

// serviceRunner 管理当前运行中的本地服务（同一时间只允许一个）。
type serviceRunner struct {
	mu     sync.Mutex
	stopFn func() error
}

// start 停止旧服务并启动新服务，端口就绪后更新界面。
func (r *serviceRunner) start(name, addr string, serve func() error, stop func() error,
	status *widget.Label, urlEntry *widget.Entry, openBtn *widget.Button) {

	r.stop()

	r.mu.Lock()
	r.stopFn = stop
	r.mu.Unlock()

	go func() { _ = serve() }()

	go func() {
		if waitPort(addr, 5*time.Second) {
			fyne.Do(func() {
				status.SetText(name + " 已启动")
				urlEntry.SetText("http://" + addr)
				openBtn.Enable()
			})
			return
		}
		fyne.Do(func() { status.SetText(name + " 启动失败：端口未就绪") })
	}()
}

// stop 停止当前服务。
func (r *serviceRunner) stop() {
	r.mu.Lock()
	stop := r.stopFn
	r.stopFn = nil
	r.mu.Unlock()
	if stop != nil {
		_ = stop()
	}
}

// waitPort 在 timeout 内轮询端口是否可连接。
func waitPort(addr string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			conn.Close()
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

var _ = fmt.Sprintf
