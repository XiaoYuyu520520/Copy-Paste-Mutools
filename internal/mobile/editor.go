package mobile

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/data"
)

// editorPage 是提示词 + 语句的合并编辑器。
//
// 手机屏幕窄，因此每一行做成「两行」：
//
//	[☑] 提示词 ______________________
//	[☑] 语句   ______________________
//
// 底部是实时合并预览 + 复制按钮。
type editorPage struct {
	store   *store
	root    fyne.CanvasObject
	body    *fyne.Container
	select1 *widget.Select

	rows    []*editorRow
	preview *widget.Entry
	status  *widget.Label

	updating bool
}

type editorRow struct {
	promptOn, sentenceOn     *widget.Check
	promptText, sentenceText *widget.Entry
}

func newEditorPage(s *store) *editorPage {
	p := &editorPage{store: s}

	p.select1 = widget.NewSelect(nil, func(string) { p.onTabChanged() })
	addBtn := widget.NewButtonWithIcon("", theme.ContentAddIcon(), p.addTab)
	delBtn := widget.NewButtonWithIcon("", theme.ContentRemoveIcon(), p.deleteTab)

	top := container.NewBorder(nil, nil, nil, container.NewHBox(addBtn, delBtn), p.select1)

	for i := 0; i < data.MaxRows; i++ {
		p.rows = append(p.rows, p.newRow(i))
	}
	p.preview = widget.NewMultiLineEntry()
	p.preview.Wrapping = fyne.TextWrapWord
	p.preview.SetPlaceHolder("合并结果会显示在这里")
	// 预览只需要读，不需要编辑。
	p.preview.OnChanged = func(string) {}

	p.status = widget.NewLabel("")

	copyBtn := widget.NewButtonWithIcon("复制结果", theme.ContentCopyIcon(), p.copyResult)
	clearBtn := widget.NewButtonWithIcon("清空本站", theme.DeleteIcon(), p.clearTab)

	form := container.NewVBox()
	for _, r := range p.rows {
		form.Add(container.NewVBox(
			container.NewBorder(nil, nil, r.promptOn, nil, r.promptText),
			container.NewBorder(nil, nil, r.sentenceOn, nil, r.sentenceText),
			widget.NewSeparator(),
		))
	}

	scroll := container.NewVScroll(form)
	bottom := container.NewVBox(
		widget.NewLabelWithStyle("合并预览", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		p.preview,
		container.NewGridWithColumns(2, copyBtn, clearBtn),
		p.status,
	)

	p.root = container.NewBorder(top, bottom, nil, nil, scroll)
	p.refreshTabs()
	return p
}

// newRow 创建一行输入。
func (p *editorPage) newRow(i int) *editorRow {
	r := &editorRow{
		promptOn:     widget.NewCheck("提示词", func(bool) { p.onEdit(i) }),
		sentenceOn:   widget.NewCheck("语句", func(bool) { p.onEdit(i) }),
		promptText:   widget.NewEntry(),
		sentenceText: widget.NewEntry(),
	}
	r.promptText.SetPlaceHolder(fmt.Sprintf("提示词 %d", i+1))
	r.sentenceText.SetPlaceHolder(fmt.Sprintf("语句 %d", i+1))
	r.promptText.OnChanged = func(string) { p.onEdit(i) }
	r.sentenceText.OnChanged = func(string) { p.onEdit(i) }
	return r
}

// refreshTabs 从配置重建标签下拉框并载入当前标签内容。
func (p *editorPage) refreshTabs() {
	cfg := p.store.Config()
	names := make([]string, len(cfg.Tabs))
	for i := range cfg.Tabs {
		names[i] = fmt.Sprintf("标签 %d", i+1)
	}
	if cfg.ActiveTab >= len(names) || cfg.ActiveTab < 0 {
		p.store.Update(func(c *data.Config) { c.ActiveTab = 0 })
	}
	p.updating = true
	p.select1.SetOptions(names)
	idx := p.store.Config().ActiveTab
	if idx >= len(names) {
		idx = 0
	}
	p.select1.SetSelectedIndex(idx)
	p.updating = false
	p.loadTab()
}

// loadTab 把当前标签的数据填进控件。
func (p *editorPage) loadTab() {
	cfg := p.store.Config()
	if cfg.ActiveTab < 0 || cfg.ActiveTab >= len(cfg.Tabs) {
		return
	}
	tab := cfg.Tabs[cfg.ActiveTab]

	p.updating = true
	for i, r := range p.rows {
		if i < len(tab.Prompts) {
			r.promptText.SetText(tab.Prompts[i].Text)
			r.promptOn.SetChecked(tab.Prompts[i].Enabled)
		}
		if i < len(tab.Sentences) {
			r.sentenceText.SetText(tab.Sentences[i].Text)
			r.sentenceOn.SetChecked(tab.Sentences[i].Enabled)
		}
	}
	p.updating = false
	p.updatePreview()
}

// onEdit 把控件内容写回配置并刷新预览。
func (p *editorPage) onEdit(i int) {
	if p.updating {
		return
	}
	promptText := p.rows[i].promptText.Text
	promptOn := p.rows[i].promptOn.Checked
	sentenceText := p.rows[i].sentenceText.Text
	sentenceOn := p.rows[i].sentenceOn.Checked

	p.store.Update(func(cfg *data.Config) {
		if cfg.ActiveTab < 0 || cfg.ActiveTab >= len(cfg.Tabs) {
			return
		}
		tab := &cfg.Tabs[cfg.ActiveTab]
		if i < len(tab.Prompts) {
			tab.Prompts[i].Text = promptText
			tab.Prompts[i].Enabled = promptOn
		}
		if i < len(tab.Sentences) {
			tab.Sentences[i].Text = sentenceText
			tab.Sentences[i].Enabled = sentenceOn
		}
	})
	p.store.SaveDebounced()
	p.updatePreview()
}

// updatePreview 重新计算合并结果。
func (p *editorPage) updatePreview() {
	cfg := p.store.Config()
	if cfg.ActiveTab < 0 || cfg.ActiveTab >= len(cfg.Tabs) {
		p.preview.SetText("")
		return
	}
	p.preview.SetText(data.MergeTab(cfg.Tabs[cfg.ActiveTab]))
}

// onTabChanged 处理标签切换。
func (p *editorPage) onTabChanged() {
	if p.updating {
		return
	}
	idx := p.select1.SelectedIndex()
	if idx < 0 {
		return
	}
	p.store.Update(func(cfg *data.Config) { cfg.ActiveTab = idx })
	p.store.SaveDebounced()
	p.loadTab()
}

// addTab 新建一个空标签。
func (p *editorPage) addTab() {
	p.store.Update(func(cfg *data.Config) {
		cfg.Tabs = append(cfg.Tabs, blankTab())
		cfg.ActiveTab = len(cfg.Tabs) - 1
	})
	p.store.SaveDebounced()
	p.refreshTabs()
}

// deleteTab 删除当前标签（至少保留一个）。
func (p *editorPage) deleteTab() {
	cfg := p.store.Config()
	if len(cfg.Tabs) <= 1 {
		// 只剩一个时退化为「清空此标签」，与桌面版行为一致。
		p.clearTab()
		return
	}
	idx := cfg.ActiveTab
	p.store.Update(func(c *data.Config) {
		if idx < 0 || idx >= len(c.Tabs) {
			return
		}
		c.Tabs = append(c.Tabs[:idx], c.Tabs[idx+1:]...)
		if c.ActiveTab >= len(c.Tabs) {
			c.ActiveTab = len(c.Tabs) - 1
		}
	})
	p.store.SaveDebounced()
	p.refreshTabs()
}

// clearTab 清空当前标签的所有输入。
func (p *editorPage) clearTab() {
	cleared := false
	p.store.Update(func(cfg *data.Config) {
		if cfg.ActiveTab < 0 || cfg.ActiveTab >= len(cfg.Tabs) {
			return
		}
		cfg.Tabs[cfg.ActiveTab] = blankTab()
		cleared = true
	})
	if !cleared {
		return
	}

	p.updating = true
	for _, r := range p.rows {
		r.promptText.SetText("")
		r.sentenceText.SetText("")
		r.promptOn.SetChecked(true)
		r.sentenceOn.SetChecked(true)
	}
	p.updating = false

	p.store.SaveDebounced()
	p.updatePreview()
	p.status.SetText("已清空当前标签")
}

// copyResult 把合并结果写入剪贴板。
func (p *editorPage) copyResult() {
	text := p.preview.Text
	if text == "" {
		p.status.SetText("没有可复制的内容")
		return
	}
	if app := fyne.CurrentApp(); app != nil {
		app.Clipboard().SetContent(text)
	}
	p.status.SetText("已复制到剪贴板")
}

// 避免未使用导入（dialog 在后续版本会用于确认框）。
var _ = dialog.ShowInformation
