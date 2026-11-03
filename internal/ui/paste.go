package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/data"
)

// ensureFirstBlock 保证至少存在一个区块，供粘贴入口使用。
func ensureFirstBlock(t data.TabConfig) data.TabConfig {
	t = data.NormalizeTab(t)
	return t
}

// PasteAsContent 把整段文本塞进第 1 个区块的提示词，便于在编辑器内继续拆分。
func (e *TabEditor) PasteAsContent(content string) {
	e.Config = ensureFirstBlock(e.Config)
	e.Config.Prompts[0].Text = content
	e.Config.Prompts[0].Enabled = true
	e.Config.Sentences[0].Text = ""
	e.Config.Sentences[0].Enabled = true
	e.rebuildRows()
	fyne.Do(func() { e.scheduleMerge() })
}

// PasteAsPrompt 仅填充到第 1 个区块的提示词。
func (e *TabEditor) PasteAsPrompt(content string) {
	e.Config = ensureFirstBlock(e.Config)
	e.Config.Prompts[0].Text = content
	e.Config.Prompts[0].Enabled = true
	e.rebuildRows()
	fyne.Do(func() { e.scheduleMerge() })
}

// 保留对 widget 包的依赖
var _ = widget.NewLabel
