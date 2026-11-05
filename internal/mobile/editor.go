package mobile

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/data"
)

// editorPage 是提示词 + 语句的合并编辑器（手机端）。
type editorPage struct {
	store   *store
	root    fyne.CanvasObject
	rowBox  *fyne.Container
	scroll  *container.Scroll
	select1 *widget.Select

	nameEntry *widget.Entry
	modeSel   *widget.Select
	sepEntry  *widget.Entry
	sepWrap   *fyne.Container

	rows    []*mobileEditorRow
	preview *widget.Entry
	status  *widget.Label
	count   *widget.Label

	updating bool
}

type mobileEditorRow struct {
	root fyne.CanvasObject
}

func newEditorPage(s *store) *editorPage {
	p := &editorPage{store: s}

	p.select1 = widget.NewSelect(nil, func(string) { p.onTabChanged() })
	addBtn := widget.NewButtonWithIcon("", theme.ContentAddIcon(), p.addTab)
	delBtn := widget.NewButtonWithIcon("", theme.ContentRemoveIcon(), p.deleteTab)
	top := container.NewBorder(nil, nil, nil, container.NewHBox(addBtn, delBtn), p.select1)

	p.nameEntry = widget.NewEntry()
	p.nameEntry.SetPlaceHolder("标签名称")
	p.nameEntry.OnChanged = func(text string) {
		if p.updating {
			return
		}
		idx := p.selectedTab()
		p.store.Update(func(c *data.Config) {
			if idx >= 0 && idx < len(c.Tabs) {
				c.Tabs[idx].Name = text
			}
		})
		p.store.SaveDebounced()
	}

	p.modeSel = widget.NewSelect([]string{
		mobileModeLabel(data.MergeModeNatural),
		mobileModeLabel(data.MergeModeRaw),
		mobileModeLabel(data.MergeModeMarkdown),
		mobileModeLabel(data.MergeModeCustom),
	}, func(string) {
		if p.updating {
			return
		}
		mode := mobileModeValue(p.modeSel.Selected)
		idx := p.selectedTab()
		p.store.Update(func(c *data.Config) {
			if idx >= 0 && idx < len(c.Tabs) {
				c.Tabs[idx].Mode = mode
			}
		})
		p.updateSepVisibility()
		p.store.SaveDebounced()
		p.updatePreview()
	})

	p.sepEntry = widget.NewEntry()
	p.sepEntry.SetPlaceHolder(`区块分隔符，例如 \n\n`)
	p.sepEntry.OnChanged = func(text string) {
		if p.updating {
			return
		}
		idx := p.selectedTab()
		p.store.Update(func(c *data.Config) {
			if idx >= 0 && idx < len(c.Tabs) {
				c.Tabs[idx].Separator = text
			}
		})
		p.store.SaveDebounced()
		p.updatePreview()
	}
	p.sepWrap = container.NewVBox(widget.NewLabel("自定义分隔符"), p.sepEntry)
	p.sepWrap.Hide()

	addBlockBtn := widget.NewButtonWithIcon("新增区块", theme.ContentAddIcon(), p.addBlock)
	tplBtn := widget.NewButtonWithIcon("模板", theme.DocumentIcon(), p.showTemplates)

	toolbar := container.NewVBox(
		container.NewBorder(nil, nil, widget.NewLabel("标签名"), nil, p.nameEntry),
		container.NewBorder(nil, nil, widget.NewLabel("拼接模式"), nil, p.modeSel),
		p.sepWrap,
		container.NewHBox(addBlockBtn, tplBtn),
		widget.NewSeparator(),
	)

	p.rowBox = container.NewVBox()
	p.scroll = container.NewVScroll(p.rowBox)

	p.preview = widget.NewMultiLineEntry()
	p.preview.Wrapping = fyne.TextWrapWord
	p.preview.SetPlaceHolder("合并结果会显示在这里")
	p.preview.OnChanged = func(string) {}

	p.status = widget.NewLabel("")
	p.count = widget.NewLabel("")

	// 配置保存状态反馈：去抖保存完成后更新「未保存 / 正在保存 / 已保存 / 保存失败」。
	// 回调可能在定时器 goroutine 触发，需切回 UI 线程。
	s.SetSaveResultHandler(func(err error) {
		fyne.Do(func() {
			state, detail := s.SaveStatus()
			if err != nil {
				p.status.SetText("保存失败：" + detail)
				return
			}
			p.status.SetText(state.Label())
		})
	})

	copyBtn := widget.NewButtonWithIcon("复制结果", theme.ContentCopyIcon(), p.copyResult)
	clearBtn := widget.NewButtonWithIcon("清空本站", theme.DeleteIcon(), p.clearTab)

	bottom := container.NewVBox(
		widget.NewLabelWithStyle("合并预览", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		p.preview,
		container.NewBorder(nil, nil, p.count, nil, container.NewHBox(copyBtn, clearBtn)),
		p.status,
	)

	p.root = container.NewBorder(
		container.NewVBox(top, toolbar),
		bottom, nil, nil, p.scroll,
	)
	p.refreshTabs()
	return p
}

func (p *editorPage) selectedTab() int {
	return p.select1.SelectedIndex()
}

// refreshTabs 从配置重建标签下拉框并载入当前标签内容。
func (p *editorPage) refreshTabs() {
	cfg := p.store.Config()
	names := make([]string, len(cfg.Tabs))
	for i := range cfg.Tabs {
		if cfg.Tabs[i].Name != "" {
			names[i] = cfg.Tabs[i].Name
		} else {
			names[i] = fmt.Sprintf("标签 %d", i+1)
		}
	}
	idx := cfg.ActiveTab
	if idx < 0 || idx >= len(names) {
		idx = 0
		p.store.Update(func(c *data.Config) { c.ActiveTab = 0 })
	}
	p.updating = true
	p.select1.SetOptions(names)
	p.select1.SetSelectedIndex(idx)
	p.updating = false
	p.loadTab()
}

// loadTab 把当前标签的数据填进控件。
func (p *editorPage) loadTab() {
	cfg := p.store.Config()
	idx := p.selectedTab()
	if idx < 0 || idx >= len(cfg.Tabs) {
		return
	}
	tab := cfg.Tabs[idx]

	p.updating = true
	p.nameEntry.SetText(tab.Name)
	p.modeSel.SetSelected(mobileModeLabel(data.EffectiveMode(tab.Mode)))
	p.sepEntry.SetText(tab.Separator)
	p.updating = false
	p.updateSepVisibility()

	p.rebuildRows(tab)
	p.updatePreview()
}

func (p *editorPage) updateSepVisibility() {
	if data.EffectiveMode(mobileModeValue(p.modeSel.Selected)) == data.MergeModeCustom {
		p.sepWrap.Show()
	} else {
		p.sepWrap.Hide()
	}
}

// rebuildRows 根据标签内容重建区块控件。
func (p *editorPage) rebuildRows(tab data.TabConfig) {
	p.rowBox.Objects = nil
	p.rows = p.rows[:0]
	for i := range tab.Prompts {
		if i >= len(tab.Sentences) {
			break
		}
		row := p.makeRow(i, tab)
		p.rows = append(p.rows, row)
		if i > 0 {
			p.rowBox.Add(widget.NewSeparator())
		}
		p.rowBox.Add(row.root)
	}
	p.rowBox.Refresh()
}

func (p *editorPage) makeRow(i int, tab data.TabConfig) *mobileEditorRow {
	row := &mobileEditorRow{}

	nameEntry := widget.NewEntry()
	if i < len(tab.BlockNames) {
		nameEntry.SetText(tab.BlockNames[i])
	}
	nameEntry.SetPlaceHolder(fmt.Sprintf("区块 %d 标题（可选）", i+1))
	nameEntry.OnChanged = func(text string) {
		p.updateBlock(i, func(t *data.TabConfig) {
			if i < len(t.BlockNames) {
				t.BlockNames[i] = text
			}
		})
	}

	upBtn := widget.NewButtonWithIcon("", theme.MoveUpIcon(), func() { p.moveBlock(i, i-1) })
	upBtn.Importance = widget.LowImportance
	if i == 0 {
		upBtn.Disable()
	}
	downBtn := widget.NewButtonWithIcon("", theme.MoveDownIcon(), func() { p.moveBlock(i, i+1) })
	downBtn.Importance = widget.LowImportance
	if i == len(tab.Prompts)-1 {
		downBtn.Disable()
	}
	dupBtn := widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() { p.duplicateBlock(i) })
	dupBtn.Importance = widget.LowImportance
	delBtn := widget.NewButtonWithIcon("", theme.ContentRemoveIcon(), func() { p.removeBlock(i) })
	delBtn.Importance = widget.LowImportance
	if len(tab.Prompts) <= 1 {
		delBtn.Disable()
	}

	header := container.NewBorder(nil, nil, nameEntry,
		container.NewHBox(upBtn, downBtn, dupBtn, delBtn), nil)

	promptOn := widget.NewCheck("提示词", nil)
	promptOn.SetChecked(tab.Prompts[i].Enabled)
	promptOn.OnChanged = func(b bool) {
		p.updateBlock(i, func(t *data.TabConfig) {
			if i < len(t.Prompts) {
				t.Prompts[i].Enabled = b
			}
		})
	}
	sentenceOn := widget.NewCheck("语句", nil)
	sentenceOn.SetChecked(tab.Sentences[i].Enabled)
	sentenceOn.OnChanged = func(b bool) {
		p.updateBlock(i, func(t *data.TabConfig) {
			if i < len(t.Sentences) {
				t.Sentences[i].Enabled = b
			}
		})
	}

	promptText := widget.NewMultiLineEntry()
	promptText.SetText(tab.Prompts[i].Text)
	promptText.SetPlaceHolder(fmt.Sprintf("提示词 %d", i+1))
	promptText.Wrapping = fyne.TextWrapWord
	promptText.OnChanged = func(text string) {
		p.updateBlock(i, func(t *data.TabConfig) {
			if i < len(t.Prompts) {
				t.Prompts[i].Text = text
			}
		})
	}
	sentenceText := widget.NewMultiLineEntry()
	sentenceText.SetText(tab.Sentences[i].Text)
	sentenceText.SetPlaceHolder(fmt.Sprintf("语句 %d", i+1))
	sentenceText.Wrapping = fyne.TextWrapWord
	sentenceText.OnChanged = func(text string) {
		p.updateBlock(i, func(t *data.TabConfig) {
			if i < len(t.Sentences) {
				t.Sentences[i].Text = text
			}
		})
	}

	row.root = container.NewVBox(
		header,
		container.NewBorder(nil, nil, promptOn, nil, promptText),
		container.NewBorder(nil, nil, sentenceOn, nil, sentenceText),
		widget.NewSeparator(),
	)
	return row
}

// updateBlock 在锁内更新当前标签的第 i 个区块。
func (p *editorPage) updateBlock(i int, fn func(*data.TabConfig)) {
	idx := p.selectedTab()
	p.store.Update(func(c *data.Config) {
		if idx < 0 || idx >= len(c.Tabs) {
			return
		}
		fn(&c.Tabs[idx])
		c.Tabs[idx] = data.NormalizeTab(c.Tabs[idx])
	})
	p.store.SaveDebounced()
	p.updatePreview()
}

func (p *editorPage) moveBlock(from, to int) {
	idx := p.selectedTab()
	p.store.Update(func(c *data.Config) {
		if idx < 0 || idx >= len(c.Tabs) {
			return
		}
		c.Tabs[idx] = data.MoveBlock(c.Tabs[idx], from, to)
	})
	p.store.SaveDebounced()
	p.loadTab()
}

func (p *editorPage) duplicateBlock(i int) {
	idx := p.selectedTab()
	p.store.Update(func(c *data.Config) {
		if idx < 0 || idx >= len(c.Tabs) {
			return
		}
		c.Tabs[idx] = data.DuplicateBlock(c.Tabs[idx], i)
	})
	p.store.SaveDebounced()
	p.loadTab()
}

func (p *editorPage) removeBlock(i int) {
	idx := p.selectedTab()
	p.store.Update(func(c *data.Config) {
		if idx < 0 || idx >= len(c.Tabs) {
			return
		}
		c.Tabs[idx] = data.RemoveBlock(c.Tabs[idx], i)
	})
	p.store.SaveDebounced()
	p.loadTab()
}

func (p *editorPage) addBlock() {
	idx := p.selectedTab()
	p.store.Update(func(c *data.Config) {
		if idx < 0 || idx >= len(c.Tabs) {
			return
		}
		c.Tabs[idx] = data.AddBlock(c.Tabs[idx])
	})
	p.store.SaveDebounced()
	p.loadTab()
}

// updatePreview 重新计算合并结果。
func (p *editorPage) updatePreview() {
	cfg := p.store.Config()
	idx := p.selectedTab()
	if idx < 0 || idx >= len(cfg.Tabs) {
		p.preview.SetText("")
		return
	}
	text := data.MergeTab(cfg.Tabs[idx])
	p.preview.SetText(text)
	p.count.SetText(fmt.Sprintf("%d 字符", utf8.RuneCountInString(text)))
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

// clearTab 清空当前标签的所有输入（保留区块结构）。
func (p *editorPage) clearTab() {
	idx := p.selectedTab()
	p.store.Update(func(cfg *data.Config) {
		if idx < 0 || idx >= len(cfg.Tabs) {
			return
		}
		t := data.NormalizeTab(cfg.Tabs[idx])
		for i := range t.Prompts {
			t.Prompts[i].Text = ""
			t.Sentences[i].Text = ""
			t.BlockNames[i] = ""
		}
		cfg.Tabs[idx] = t
	})
	p.store.SaveDebounced()
	p.loadTab()
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

// showTemplates 是手机端模板面板：应用/保存当前。
func (p *editorPage) showTemplates() {
	path := filepath.Join(filepath.Dir(p.store.path), "templates.json")
	st, loadErr := data.LoadTemplates(path)
	if loadErr != nil {
		p.status.SetText("模板读取失败：" + loadErr.Error())
	}

	sel := widget.NewSelect(nil, nil)
	opts := make([]string, len(st.Templates))
	for i, t := range st.Templates {
		label := t.Name
		if t.Category != "" {
			label += " · " + t.Category
		}
		opts[i] = label
	}
	sel.Options = opts

	var apply func()
	apply = func() {
		idx := sel.SelectedIndex()
		if idx < 0 || idx >= len(st.Templates) {
			p.status.SetText("请先选择模板")
			return
		}
		tpl := st.Templates[idx]
		vars := data.ExtractVariables(tpl.Tab)
		tabIdx := p.selectedTab()
		if len(vars) == 0 {
			p.store.Update(func(c *data.Config) {
				if tabIdx >= 0 && tabIdx < len(c.Tabs) {
					c.Tabs[tabIdx] = data.NormalizeTab(tpl.Tab)
				}
			})
			p.store.SaveDebounced()
			p.loadTab()
			return
		}
		entries := make(map[string]*widget.Entry, len(vars))
		items := make([]*widget.FormItem, 0, len(vars))
		for _, name := range vars {
			e := widget.NewEntry()
			e.SetPlaceHolder("{{" + name + "}}")
			entries[name] = e
			items = append(items, widget.NewFormItem(name, e))
		}
		dialog.ShowForm("填写模板变量", "应用", "取消", items, func(ok bool) {
			if !ok {
				return
			}
			vals := map[string]string{}
			for k, e := range entries {
				if v := strings.TrimSpace(e.Text); v != "" {
					vals[k] = v
				}
			}
			p.store.Update(func(c *data.Config) {
				if tabIdx >= 0 && tabIdx < len(c.Tabs) {
					c.Tabs[tabIdx] = data.ApplyVariables(tpl.Tab, vals)
				}
			})
			p.store.SaveDebounced()
			p.loadTab()
		}, fyne.CurrentApp().Driver().AllWindows()[0])
	}

	saveBtn := widget.NewButtonWithIcon("保存当前为模板", theme.DocumentSaveIcon(), func() {
		tabIdx := p.selectedTab()
		cfg := p.store.Config()
		if tabIdx < 0 || tabIdx >= len(cfg.Tabs) {
			return
		}
		e := widget.NewEntry()
		e.SetText(cfg.Tabs[tabIdx].Name)
		dialog.ShowForm("保存为模板", "保存", "取消", []*widget.FormItem{
			widget.NewFormItem("名称", e),
		}, func(ok bool) {
			if !ok {
				return
			}
			tpl := data.NewTemplate(e.Text, "手机端", "", cfg.Tabs[tabIdx])
			data.UpsertTemplate(&st, tpl)
			if err := data.SaveTemplates(path, st); err != nil {
				p.status.SetText("保存模板失败：" + err.Error())
				return
			}
			opts = append(opts, tpl.Name)
			sel.Options = opts
			p.status.SetText("模板已保存")
		}, fyne.CurrentApp().Driver().AllWindows()[0])
	})

	applyBtn := widget.NewButtonWithIcon("应用", theme.ConfirmIcon(), apply)
	content := container.NewVBox(
		widget.NewLabel("选择模板后点「应用」替换当前标签内容"),
		sel,
		applyBtn,
		saveBtn,
	)
	dialog.ShowCustom("模板", "关闭", content, fyne.CurrentApp().Driver().AllWindows()[0])
}

func mobileModeLabel(m string) string {
	switch m {
	case data.MergeModeRaw:
		return "原样拼接"
	case data.MergeModeMarkdown:
		return "Markdown"
	case data.MergeModeCustom:
		return "自定义分隔符"
	default:
		return "自然语言"
	}
}

func mobileModeValue(label string) string {
	switch label {
	case mobileModeLabel(data.MergeModeRaw):
		return data.MergeModeRaw
	case mobileModeLabel(data.MergeModeMarkdown):
		return data.MergeModeMarkdown
	case mobileModeLabel(data.MergeModeCustom):
		return data.MergeModeCustom
	default:
		return data.MergeModeNatural
	}
}
