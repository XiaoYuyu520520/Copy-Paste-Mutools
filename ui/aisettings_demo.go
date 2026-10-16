package ui

import (
	"fyne.io/fyne/v2"

	"prompt-merger-native/chat"
)

// DemoAISettingsObject 供 cmd/screenshot 无头渲染「AI 设置」页面快照用。
// 预填一组示例配置，便于检查表单控件的主题样式。
func DemoAISettingsObject() fyne.CanvasObject {
	p := NewAISettingsPanel(nil, func() {})
	p.Set(chat.Settings{
		BaseURL: "https://api.deepseek.com/v1",
		APIKey:  "sk-demo-key-not-real",
		Model:   "deepseek-chat",
		System:  "你是一个贴心的助手，用简体中文回答，代码给出完整 diff。",
	})
	p.View().Show()
	return p.View()
}
