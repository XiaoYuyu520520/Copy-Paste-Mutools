package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// PasteAsContent 把整段文本塞进第 1 行的提示词，便于在编辑器内继续拆分。
func (e *TabEditor) PasteAsContent(content string) {
	e.Config.Prompts[0].Text = content
	e.Config.Prompts[0].Enabled = true
	e.Config.Sentences[0].Text = ""
	e.Config.Sentences[0].Enabled = true
	e.prompts[0].SetText(content)
	e.sentences[0].SetText("")
	fyne.Do(func() { e.scheduleMerge() })
}

// PasteAsPrompt 仅填充到第 1 行的提示词。
func (e *TabEditor) PasteAsPrompt(content string) {
	e.Config.Prompts[0].Text = content
	e.Config.Prompts[0].Enabled = true
	e.prompts[0].SetText(content)
	fyne.Do(func() { e.scheduleMerge() })
}

// 保留对 widget 包的依赖
var _ = widget.NewLabel
