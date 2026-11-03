package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/chat"
	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/data"
)

// AISettingsPanel 独立「AI 设置」页：管理多个 OpenAI 兼容服务商。
type AISettingsPanel struct {
	root fyne.CanvasObject

	providerSel *widget.Select
	providers   []data.AIProvider
	active      int
	updating    bool

	name    *widget.Entry
	baseURL *widget.Entry
	apiKey  *widget.Entry
	model   *widget.Entry
	system  *widget.Entry
	status  *widget.Label

	OnSave          func(chat.Settings)
	OnSaveProviders func([]data.AIProvider, int)
}

// NewAISettingsPanel 构造 AI 设置页；onBack 返回编辑器。
func NewAISettingsPanel(win fyne.Window, onBack func()) *AISettingsPanel {
	p := &AISettingsPanel{}

	back := widget.NewButtonWithIcon("返回编辑器", theme.NavigateBackIcon(), onBack)
	back.Importance = widget.LowImportance
	title := widget.NewLabelWithStyle("AI 设置", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	header := container.NewBorder(nil, nil,
		container.NewHBox(container.NewPadded(back), container.NewPadded(title)), nil, nil)

	p.providerSel = widget.NewSelect(nil, func(string) {
		if p.updating {
			return
		}
		idx := p.providerSel.SelectedIndex()
		if idx < 0 || idx >= len(p.providers) {
			return
		}
		p.active = idx
		p.loadProvider(p.providers[idx])
	})
	addBtn := widget.NewButtonWithIcon("新增", theme.ContentAddIcon(), p.addProvider)
	delBtn := widget.NewButtonWithIcon("删除", theme.ContentRemoveIcon(), p.deleteProvider)
	providerRow := container.NewBorder(nil, nil,
		widget.NewLabelWithStyle("服务商", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewHBox(addBtn, delBtn),
		p.providerSel,
	)

	p.name = widget.NewEntry()
	p.name.SetPlaceHolder("服务商名称，例如 OpenAI / DeepSeek / 本地 Ollama")
	p.baseURL = widget.NewEntry()
	p.baseURL.SetPlaceHolder("https://api.openai.com/v1（OpenAI 兼容地址）")
	p.apiKey = widget.NewPasswordEntry()
	p.apiKey.SetPlaceHolder("sk-...（本地兼容服务可留空）")
	p.model = widget.NewEntry()
	p.model.SetPlaceHolder("例如 gpt-4o-mini / deepseek-chat / qwen-plus")
	p.system = widget.NewMultiLineEntry()
	p.system.Wrapping = fyne.TextWrapWord
	p.system.SetPlaceHolder("可选的系统提示词，例如：你是一个贴心的助手，用简体中文回答。")

	capStyle := fyne.TextStyle{Bold: true}
	form := container.NewVBox(
		container.NewPadded(providerRow),
		widget.NewSeparator(),
		container.NewPadded(widget.NewLabelWithStyle("名称", fyne.TextAlignLeading, capStyle)),
		container.NewPadded(p.name),
		widget.NewSeparator(),
		container.NewPadded(widget.NewLabelWithStyle("API 地址 (Base URL)", fyne.TextAlignLeading, capStyle)),
		container.NewPadded(p.baseURL),
		widget.NewSeparator(),
		container.NewPadded(widget.NewLabelWithStyle("API Key", fyne.TextAlignLeading, capStyle)),
		container.NewPadded(p.apiKey),
		widget.NewSeparator(),
		container.NewPadded(widget.NewLabelWithStyle("模型", fyne.TextAlignLeading, capStyle)),
		container.NewPadded(p.model),
		widget.NewSeparator(),
		container.NewPadded(widget.NewLabelWithStyle("系统提示词（可选）", fyne.TextAlignLeading, capStyle)),
		container.NewPadded(WrapFixedHeight(p.system, 120)),
	)

	save := widget.NewButtonWithIcon("保存", theme.DocumentSaveIcon(), p.Save)
	save.Importance = widget.HighImportance
	testBtn := widget.NewButtonWithIcon("测试连接", theme.RadioButtonCheckedIcon(), p.testConnection)
	p.status = widget.NewLabel("选择或新增服务商，填写后点「保存」。")
	p.status.Wrapping = fyne.TextWrapWord

	foot := container.NewBorder(nil, nil,
		container.NewHBox(container.NewPadded(save), container.NewPadded(testBtn)),
		container.NewPadded(p.status), nil)

	body := container.NewBorder(
		container.NewVBox(header, widget.NewSeparator()),
		container.NewVBox(widget.NewSeparator(), foot),
		nil, nil,
		container.NewVScroll(form),
	)
	p.root = body
	p.root.Hide()
	return p
}

func (p *AISettingsPanel) View() fyne.CanvasObject { return p.root }

// Set 用保存的配置填充表单。
func (p *AISettingsPanel) Set(s chat.Settings) {
	p.baseURL.SetText(s.BaseURL)
	p.apiKey.SetText(s.APIKey)
	p.model.SetText(s.Model)
	p.system.SetText(s.System)
}

// SetProviders 载入多个服务商；空列表时用当前 Set 的值兜底。
func (p *AISettingsPanel) SetProviders(list []data.AIProvider, active int) {
	p.updating = true
	p.providers = append([]data.AIProvider(nil), list...)
	if len(p.providers) == 0 {
		p.providers = []data.AIProvider{data.ProviderFromAI("默认服务", data.AIConfig{
			BaseURL: p.baseURL.Text, APIKey: p.apiKey.Text, Model: p.model.Text, System: p.system.Text,
		})}
	}
	if active < 0 || active >= len(p.providers) {
		active = 0
	}
	p.active = active
	p.refreshProviderOptionsLocked()
	p.loadProvider(p.providers[active])
	p.updating = false
}

func (p *AISettingsPanel) refreshProviderOptions() {
	p.updating = true
	p.refreshProviderOptionsLocked()
	p.updating = false
}

func (p *AISettingsPanel) refreshProviderOptionsLocked() {
	opts := make([]string, len(p.providers))
	for i, pr := range p.providers {
		name := pr.Name
		if name == "" {
			name = fmt.Sprintf("服务商 %d", i+1)
		}
		opts[i] = name
	}
	p.providerSel.Options = opts
	if p.active >= 0 && p.active < len(opts) {
		p.providerSel.SetSelectedIndex(p.active)
	}
}

func (p *AISettingsPanel) loadProvider(pr data.AIProvider) {
	p.name.SetText(pr.Name)
	p.baseURL.SetText(pr.BaseURL)
	p.apiKey.SetText(pr.APIKey)
	p.model.SetText(pr.Model)
	p.system.SetText(pr.System)
}

func (p *AISettingsPanel) addProvider() {
	p.providers = append(p.providers, data.AIProvider{Name: fmt.Sprintf("服务商 %d", len(p.providers)+1)})
	p.active = len(p.providers) - 1
	p.refreshProviderOptions()
	p.loadProvider(p.providers[p.active])
	p.setStatus("已新增，请填写地址与模型后保存")
}

func (p *AISettingsPanel) deleteProvider() {
	if len(p.providers) <= 1 {
		p.setStatus("至少保留一个服务商")
		return
	}
	p.providers = append(p.providers[:p.active], p.providers[p.active+1:]...)
	if p.active >= len(p.providers) {
		p.active = len(p.providers) - 1
	}
	p.refreshProviderOptions()
	p.loadProvider(p.providers[p.active])
	p.setStatus("已删除（记得点保存）")
}

// Value 读取当前表单内容。
func (p *AISettingsPanel) Value() chat.Settings {
	return chat.Settings{
		BaseURL: strings.TrimSpace(p.baseURL.Text),
		APIKey:  strings.TrimSpace(p.apiKey.Text),
		Model:   strings.TrimSpace(p.model.Text),
		System:  p.system.Text,
	}
}

// Save 校验并回调 OnSave / OnSaveProviders 落盘。
func (p *AISettingsPanel) Save() {
	s := p.Value()
	if s.BaseURL == "" {
		p.setStatus("请先填写 API 地址（例如 https://api.openai.com/v1）")
		return
	}
	if s.Model == "" {
		p.setStatus("请填写要使用的模型名称")
		return
	}
	if len(p.providers) == 0 {
		p.providers = []data.AIProvider{{Name: "默认服务"}}
		p.active = 0
	}
	if p.active < 0 || p.active >= len(p.providers) {
		p.active = 0
	}
	name := strings.TrimSpace(p.name.Text)
	if name == "" {
		name = "未命名服务"
	}
	p.providers[p.active] = data.AIProvider{
		Name: name, BaseURL: s.BaseURL, APIKey: s.APIKey, Model: s.Model, System: s.System,
	}
	p.refreshProviderOptions()

	if p.OnSave != nil {
		p.OnSave(s)
	}
	if p.OnSaveProviders != nil {
		p.OnSaveProviders(append([]data.AIProvider(nil), p.providers...), p.active)
	}
	p.setStatus(fmt.Sprintf("已保存「%s」✓ 底部点「AI 聊天」即可使用", name))
}

// testConnection 在后台请求 /models，验证地址、Key 与网络连通性。
func (p *AISettingsPanel) testConnection() {
	s := p.Value()
	if s.BaseURL == "" || s.Model == "" {
		p.setStatus("请先填写 API 地址与模型")
		return
	}
	p.setStatus("正在测试连接…")
	go func() {
		url := strings.TrimRight(s.BaseURL, "/") + "/models"
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			p.setStatusLater("测试失败：" + err.Error())
			return
		}
		if s.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+s.APIKey)
		}
		client := &http.Client{Timeout: 8 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			p.setStatusLater("连接失败：" + err.Error())
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			var parsed struct {
				Data []struct {
					ID string `json:"id"`
				} `json:"data"`
			}
			_ = json.Unmarshal(body, &parsed)
			msg := fmt.Sprintf("连接成功（HTTP %d）", resp.StatusCode)
			if len(parsed.Data) > 0 {
				ids := make([]string, 0, 5)
				for i, m := range parsed.Data {
					if i >= 5 {
						break
					}
					ids = append(ids, m.ID)
				}
				msg += "，示例模型：" + strings.Join(ids, ", ")
			}
			p.setStatusLater(msg)
			return
		}
		p.setStatusLater(fmt.Sprintf("服务返回 HTTP %d：%s", resp.StatusCode, strings.TrimSpace(string(body))))
	}()
}

func (p *AISettingsPanel) setStatus(s string) {
	if p.status != nil {
		p.status.SetText(fmt.Sprintf("状态：%s", s))
	}
}

func (p *AISettingsPanel) setStatusLater(s string) {
	fyne.Do(func() { p.setStatus(s) })
}
