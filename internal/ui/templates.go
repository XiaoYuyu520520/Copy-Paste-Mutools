package ui

import (
	"io"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/data"
)

// showTemplates 打开模板管理对话框：应用/保存/删除/导入/导出。
func (e *TabEditor) showTemplates() {
	path := data.TemplatesPath("")
	st, loadErr := data.LoadTemplates(path)
	if loadErr != nil {
		dialog.ShowError(loadErr, e.window)
	}

	var filtered []data.Template
	selected := -1

	categorySel := widget.NewSelect([]string{"全部分类"}, nil)
	search := widget.NewEntry()
	search.SetPlaceHolder("搜索模板名称 / 分类 / 描述")

	var listSel *widget.Select
	listSel = widget.NewSelect(nil, func(string) { selected = listSel.SelectedIndex() })
	meta := widget.NewLabel("选择一个模板查看详情")
	meta.Wrapping = fyne.TextWrapWord

	refresh := func() {
		// 分类选项
		cats := append([]string{"全部分类"}, data.TemplateCategories(st)...)
		categorySel.Options = cats
		if categorySel.Selected == "" || !contains(cats, categorySel.Selected) {
			categorySel.SetSelected("全部分类")
		}
		cat := ""
		if categorySel.Selected != "" && categorySel.Selected != "全部分类" {
			cat = categorySel.Selected
		}
		filtered = data.FilterTemplates(st, cat, search.Text)
		opts := make([]string, len(filtered))
		for i, t := range filtered {
			label := t.Name
			if t.Category != "" {
				label += " · " + t.Category
			}
			opts[i] = label
		}
		selected = -1
		listSel.Options = opts
		listSel.ClearSelected()
		if len(opts) == 0 {
			meta.SetText("没有匹配的模板")
		} else {
			meta.SetText("共 " + itoa(len(opts)) + " 个模板")
		}
	}
	categorySel.OnChanged = func(string) { refresh() }
	search.OnChanged = func(string) { refresh() }
	listSel.OnChanged = func(string) {
		selected = listSel.SelectedIndex()
		if selected < 0 || selected >= len(filtered) {
			return
		}
		t := filtered[selected]
		vars := data.ExtractVariables(t.Tab)
		desc := t.Description
		if desc == "" {
			desc = "（无描述）"
		}
		meta.SetText("名称：" + t.Name + "\n分类：" + orDash(t.Category) +
			"\n变量：" + orDash(strings.Join(vars, ", ")) + "\n" + desc)
	}
	refresh()

	applyBtn := widget.NewButtonWithIcon("应用", theme.ConfirmIcon(), func() {
		if selected < 0 || selected >= len(filtered) {
			dialog.ShowInformation("模板", "请先选择一个模板", e.window)
			return
		}
		e.applyTemplate(filtered[selected])
	})
	applyBtn.Importance = widget.HighImportance

	saveBtn := widget.NewButtonWithIcon("保存当前为模板", theme.DocumentSaveIcon(), func() {
		nameEntry := widget.NewEntry()
		nameEntry.SetText(e.Title)
		catEntry := widget.NewEntry()
		descEntry := widget.NewMultiLineEntry()
		items := []*widget.FormItem{
			widget.NewFormItem("名称", nameEntry),
			widget.NewFormItem("分类", catEntry),
			widget.NewFormItem("描述", descEntry),
		}
		dialog.ShowForm("保存为模板", "保存", "取消", items, func(ok bool) {
			if !ok {
				return
			}
			t := data.NewTemplate(nameEntry.Text, catEntry.Text, descEntry.Text, e.Config)
			data.UpsertTemplate(&st, t)
			if err := data.SaveTemplates(path, st); err != nil {
				dialog.ShowError(err, e.window)
				return
			}
			refresh()
		}, e.window)
	})

	delBtn := widget.NewButtonWithIcon("删除", theme.DeleteIcon(), func() {
		if selected < 0 || selected >= len(filtered) {
			return
		}
		t := filtered[selected]
		dialog.ShowConfirm("删除模板", "确认删除「"+t.Name+"」？", func(ok bool) {
			if !ok {
				return
			}
			data.DeleteTemplate(&st, t.ID)
			_ = data.SaveTemplates(path, st)
			refresh()
		}, e.window)
	})
	delBtn.Importance = widget.DangerImportance

	importBtn := widget.NewButtonWithIcon("导入 JSON", theme.UploadIcon(), func() {
		dialog.ShowFileOpen(func(r fyne.URIReadCloser, err error) {
			if err != nil || r == nil {
				return
			}
			defer r.Close()
			raw, err := io.ReadAll(r)
			if err != nil {
				dialog.ShowError(err, e.window)
				return
			}
			list, err := data.ImportTemplatesJSON(raw)
			if err != nil {
				dialog.ShowError(err, e.window)
				return
			}
			for _, t := range list {
				t.ID = ""
				data.UpsertTemplate(&st, t)
			}
			if err := data.SaveTemplates(path, st); err != nil {
				dialog.ShowError(err, e.window)
				return
			}
			refresh()
		}, e.window)
	})

	exportBtn := widget.NewButtonWithIcon("导出 JSON", theme.DownloadIcon(), func() {
		raw, err := data.ExportTemplatesJSON(st.Templates)
		if err != nil {
			dialog.ShowError(err, e.window)
			return
		}
		dialog.ShowFileSave(func(w fyne.URIWriteCloser, err error) {
			if err != nil || w == nil {
				return
			}
			defer w.Close()
			_, _ = w.Write(raw)
		}, e.window)
	})

	content := container.NewBorder(
		container.NewVBox(
			container.NewBorder(nil, nil, widget.NewLabel("分类"), categorySel, nil),
			search,
			listSel,
			meta,
			widget.NewSeparator(),
		),
		container.NewVBox(widget.NewSeparator(), container.NewGridWithColumns(3,
			applyBtn, saveBtn, delBtn,
		), container.NewGridWithColumns(2,
			importBtn, exportBtn,
		)),
		nil, nil,
		widget.NewLabel(""),
	)
	d := dialog.NewCustom("模板管理", "关闭", content, e.window)
	d.Resize(fyne.NewSize(560, 560))
	d.Show()
}

// applyTemplate 把模板内容应用到当前标签，必要时先填写变量。
func (e *TabEditor) applyTemplate(t data.Template) {
	vars := data.ExtractVariables(t.Tab)
	if len(vars) == 0 {
		e.replaceConfig(t.Tab, t.Name)
		return
	}

	entries := make(map[string]*widget.Entry, len(vars))
	items := make([]*widget.FormItem, 0, len(vars))
	for _, name := range vars {
		entry := widget.NewEntry()
		entry.SetPlaceHolder("未填写时保留 {{" + name + "}}")
		entries[name] = entry
		items = append(items, widget.NewFormItem(name, entry))
	}
	dialog.ShowForm("填写模板变量", "生成预览", "取消", items, func(ok bool) {
		if !ok {
			return
		}
		values := make(map[string]string, len(entries))
		for k, entry := range entries {
			if v := strings.TrimSpace(entry.Text); v != "" {
				values[k] = v
			}
		}
		e.replaceConfig(data.ApplyVariables(t.Tab, values), t.Name)
	}, e.window)
}

func (e *TabEditor) replaceConfig(tab data.TabConfig, name string) {
	e.pushUndo()
	e.Config = data.NormalizeTab(tab)
	if e.Config.Name == "" && name != "" {
		e.Config.Name = name
		e.Title = name
	}
	if e.nameEntry != nil {
		e.nameEntry.SetText(e.Title)
	}
	e.rebuildRows()
	e.scheduleMerge()
	e.scheduleSave()
	if e.OnRename != nil {
		e.OnRename()
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}
