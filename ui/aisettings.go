package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"prompt-merger-native/chat"
)

// AISettingsPanel 独立「AI 设置」页：配置 OpenAI 兼容接口的
// API 地址 / API Key / 模型 / 系统提示词，保存到 config.json。
// 供左侧页面栏进入；AI 聊天（浏览器版）实时读取这里保存的值。
type AISettingsPanel struct {
	root fyne.CanvasObject

	baseURL *widget.Entry
	apiKey  *widget.Entry
	model   *widget.Entry
	system  *widget.Entry
	status  *widget.Label

	OnSave func(chat.Settings)
}

// NewAISettingsPanel 构造 AI 设置页；onBack 返回编辑器。
func NewAISettingsPanel(win fyne.Window, onBack func()) *AISettingsPanel {
	p := &AISettingsPanel{}

	back := widget.NewButtonWithIcon("返回编辑器", theme.NavigateBackIcon(), onBack)
	back.Importance = widget.LowImportance
	title := widget.NewLabelWithStyle("AI 设置", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	header := container.NewBorder(nil, nil,
		container.NewHBox(container.NewPadded(back), container.NewPadded(title)), nil, nil)

	p.baseURL = widget.NewEntry()
	p.baseURL.SetPlaceHolder("https://api.openai.com/v1（也可填 DeepSeek / Qwen / 本地 Ollama·LM Studio 等 OpenAI 兼容地址）")
	p.apiKey = widget.NewPasswordEntry()
	p.apiKey.SetPlaceHolder("sk-...（本地兼容服务可留空）")
	p.model = widget.NewEntry()
	p.model.SetPlaceHolder("例如 gpt-4o-mini / deepseek-chat / qwen-plus")
	p.system = widget.NewMultiLineEntry()
	p.system.Wrapping = fyne.TextWrapWord
	p.system.SetPlaceHolder("可选的系统提示词，例如：你是一个贴心的助手，用简体中文回答。")

	capStyle := fyne.TextStyle{Bold: true}
	form := container.NewVBox(
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
	p.status = widget.NewLabel("填写后点「保存」；底部「AI 聊天」会读取这里的配置。")
	p.status.Wrapping = fyne.TextWrapWord

	foot := container.NewBorder(nil, nil,
		container.NewHBox(container.NewPadded(save)),
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

// Value 读取当前表单内容。
func (p *AISettingsPanel) Value() chat.Settings {
	return chat.Settings{
		BaseURL: strings.TrimSpace(p.baseURL.Text),
		APIKey:  strings.TrimSpace(p.apiKey.Text),
		Model:   strings.TrimSpace(p.model.Text),
		System:  strings.TrimSpace(p.system.Text),
	}
}

// Save 校验并回调 OnSave 落盘。
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
	if p.OnSave != nil {
		p.OnSave(s)
		p.setStatus("已保存到 config.json ✓ 底部点「AI 聊天」即可使用")
		return
	}
	p.setStatus("保存回调未注册")
}

func (p *AISettingsPanel) setStatus(s string) {
	if p.status != nil {
		p.status.SetText(fmt.Sprintf("状态：%s", s))
	}
}
