package ui

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"prompt-merger-native/data"
)

const (
	mergeDebounce  = 200 * time.Millisecond
	rowMinHeight   = 90
	previewMinSize = 120
)

// TabEditor 单个标签页的全部 UI 与数据。
type TabEditor struct {
	Title  string
	Config data.TabConfig

	prompts     [data.MaxRows]*widget.Entry
	sentences   [data.MaxRows]*widget.Entry
	checks      [data.MaxRows][2]*widget.Check
	preview     *widget.Entry
	previewCard *cardPanel
	rowsScroll  *container.Scroll

	// 圆角遮罩：Fyne 的 Entry 无圆角 API，用遮罩层把四角「咬」圆。
	// 底色是烘焙进位图的，主题切换时必须重建（见 Recolor）。

	OnMerged   func(text string)
	OnDirty    func()
	OnDirtyNow func()

	mergeTimer *time.Timer
	saveTimer  *time.Timer
	prevOutput string

	window fyne.Window
}

// NewTabEditor 创建一个新的标签编辑器。
func NewTabEditor(title string, tab data.TabConfig, win fyne.Window) *TabEditor {
	e := &TabEditor{Title: title, Config: tab, window: win}
	e.build()
	return e
}

func (e *TabEditor) build() {
	for i := 0; i < data.MaxRows; i++ {
		e.checks[i][0] = widget.NewCheck("提示词", nil)
		e.checks[i][0].SetChecked(e.Config.Prompts[i].Enabled)
		e.checks[i][1] = widget.NewCheck("语句", nil)
		e.checks[i][1].SetChecked(e.Config.Sentences[i].Enabled)

		e.prompts[i] = makeCodeEntry()
		e.prompts[i].SetPlaceHolder(fmt.Sprintf("提示词 %d（可粘贴多行）", i+1))
		e.prompts[i].SetText(e.Config.Prompts[i].Text)
		e.prompts[i].Wrapping = fyne.TextWrapWord

		e.sentences[i] = makeCodeEntry()
		e.sentences[i].SetPlaceHolder(fmt.Sprintf("语句 %d（可粘贴多行）", i+1))
		e.sentences[i].SetText(e.Config.Sentences[i].Text)
		e.sentences[i].Wrapping = fyne.TextWrapWord

		idx := i
		e.prompts[i].OnChanged = func(s string) {
			e.Config.Prompts[idx].Text = s
			e.scheduleMerge()
			e.scheduleSave()
		}
		e.sentences[i].OnChanged = func(s string) {
			e.Config.Sentences[idx].Text = s
			e.scheduleMerge()
			e.scheduleSave()
		}
		e.checks[i][0].OnChanged = func(b bool) {
			e.Config.Prompts[idx].Enabled = b
			e.scheduleMerge()
			e.scheduleSave()
		}
		e.checks[i][1].OnChanged = func(b bool) {
			e.Config.Sentences[idx].Enabled = b
			e.scheduleMerge()
			e.scheduleSave()
		}
	}

	e.preview = makeCodeEntry()
	e.preview.Disable()
	e.preview.Wrapping = fyne.TextWrapWord
}

// makeCodeEntry 构造一个带深色背景 + 等宽字体的多行 Entry，
// 类似 IDE 的输入面板（提示词 / 语句 / 预览）。
func makeCodeEntry() *widget.Entry {
	e := widget.NewMultiLineEntry()
	e.TextStyle = fyne.TextStyle{Monospace: true}
	return e
}

// View 返回整个编辑器页面。
//
// 布局：
//   - Border(top=nil, bottom=previewPanel, center=rowsScroll)
//   - rowsScroll 包了一个 VBox，5 行编辑器行
//   - previewPanel = [卡片：标题 + 预览文本 + 按钮栏]
func (e *TabEditor) View() fyne.CanvasObject {
	e.rowsScroll = container.NewVScroll(e.rowsBox())
	rowsScroll := e.rowsScroll
	previewPanel := e.previewPanel()

	split := container.NewBorder(
		nil,
		previewPanel,
		nil,
		nil,
		rowsScroll,
	)

	return container.NewPadded(split)
}

// rowsBox 5 行编辑器的 VBox，中间加分隔线。
func (e *TabEditor) rowsBox() *fyne.Container {
	rows := make([]fyne.CanvasObject, 0, data.MaxRows*2-1)
	for i := 0; i < data.MaxRows; i++ {
		if i > 0 {
			rows = append(rows, widget.NewSeparator())
		}
		rows = append(rows, e.makeRow(i))
	}
	return container.NewVBox(rows...)
}

// previewPanel 合并预览面板：标题 + 预览框 + 按钮栏，整个装在一个 Card 里。
func (e *TabEditor) previewPanel() fyne.CanvasObject {
	previewHeader := widget.NewLabelWithStyle(
		"合并预览",
		fyne.TextAlignLeading,
		fyne.TextStyle{Bold: true},
	)

	copyBtn := widget.NewButtonWithIcon("复制结果", theme.ContentCopyIcon(), func() {
		text := e.preview.Text
		if text == "" {
			return
		}
		app := fyne.CurrentApp()
		if app != nil {
			app.Clipboard().SetContent(text)
			if e.OnMerged != nil {
				e.OnMerged("__copied__")
			}
		}
	})
	copyBtn.Importance = widget.HighImportance

	clearBtn := widget.NewButtonWithIcon("清空此标签", theme.ContentClearIcon(), func() {
		dialog.ShowConfirm("清空", "确认清空此标签的全部内容？", func(ok bool) {
			if !ok {
				return
			}
			e.Clear()
			e.scheduleMerge()
			e.scheduleSave()
		}, e.window)
	})
	clearBtn.Importance = widget.LowImportance

	buttonBar := container.NewHBox(
		clearBtn,
		layout.NewSpacer(),
		copyBtn,
	)

	previewEntry := WrapExactHeight(e.preview, previewMinSize)

	// 把整个预览面板（标题 + 预览 + 按钮栏）放进一个 Card，让背景色和边框与
	// 编辑器行视觉风格统一。
	body := container.NewBorder(
		previewHeader,
		container.NewVBox(
			widget.NewSeparator(),
			container.NewPadded(buttonBar),
		),
		nil, nil,
		container.NewPadded(previewEntry),
	)
	panel := newCardPanel(body, 12, 6)
	e.previewCard = panel
	return panel.root
}

// makeRow 单行编辑器：左 = 行号 + 两个 checkbox；中 = 提示词；右 = 语句。
// Recolor refreshes theme-dependent card layers without rebuilding the editor.
func (e *TabEditor) Recolor() {
	if e != nil && e.previewCard != nil {
		e.previewCard.Recolor()
	}
}

func (e *TabEditor) makeRow(i int) fyne.CanvasObject {
	rowLabel := widget.NewLabelWithStyle(
		fmt.Sprintf("第 %d 行", i+1),
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)

	leftCol := container.New(&editorLeftLayout{}, rowLabel, e.checks[i][0], e.checks[i][1])

	// 文本域：固定高度；深色底 + 1 px 边框 + 4 px 圆角由主题统一提供
	// （Entry 原生读取 SizeNameInputRadius / InputBorder / InputBackground）。
	promptArea := WrapExactHeight(e.prompts[i], rowMinHeight)
	sentenceArea := WrapExactHeight(e.sentences[i], rowMinHeight)

	return container.New(
		&editorRowLayout{LeftWidth: 96, Gap: 8},
		leftCol, promptArea, sentenceArea,
	)
}

// scheduleMerge 触发一次合并刷新（去抖）。
func (e *TabEditor) scheduleMerge() {
	if e.mergeTimer != nil {
		e.mergeTimer.Stop()
	}
	e.mergeTimer = time.AfterFunc(mergeDebounce, func() {
		text := data.MergeTab(e.Config)
		fyne.Do(func() {
			e.prevOutput = text
			e.preview.SetText(text)
			if e.OnMerged != nil {
				e.OnMerged(text)
			}
		})
	})
}

// scheduleSave 触发一次落盘（去抖）。
func (e *TabEditor) scheduleSave() {
	if e.saveTimer != nil {
		e.saveTimer.Stop()
	}
	e.saveTimer = time.AfterFunc(saveDebounce, func() {
		if e.OnDirty != nil {
			fyne.Do(e.OnDirty)
		}
	})
}

// ForceMerge 同步执行一次合并。
func (e *TabEditor) ForceMerge() {
	if e.mergeTimer != nil {
		e.mergeTimer.Stop()
	}
	text := data.MergeTab(e.Config)
	e.prevOutput = text
	e.preview.SetText(text)
	if e.OnMerged != nil {
		e.OnMerged(text)
	}
}

// FlushSave 立即落盘。
func (e *TabEditor) FlushSave() {
	if e.saveTimer != nil {
		e.saveTimer.Stop()
		e.saveTimer = nil
	}
	if e.OnDirtyNow != nil {
		e.OnDirtyNow()
	} else if e.OnDirty != nil {
		e.OnDirty()
	}
}

// Clear 清空所有文本输入。
func (e *TabEditor) Clear() {
	for i := 0; i < data.MaxRows; i++ {
		e.prompts[i].SetText("")
		e.sentences[i].SetText("")
	}
}

// ActiveCount 返回合并后非空的行数（含勾选状态过滤）。
func (e *TabEditor) ActiveCount() int {
	n := 0
	for i := 0; i < data.MaxRows; i++ {
		pText := trimSpaces(e.Config.Prompts[i].Text)
		sText := trimSpaces(e.Config.Sentences[i].Text)
		pHas := e.Config.Prompts[i].Enabled && pText != ""
		sHas := e.Config.Sentences[i].Enabled && sText != ""
		if pHas || sHas {
			n++
		}
	}
	return n
}

func trimSpaces(s string) string {
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t' ||
		s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' ||
		s[0] == '\n' || s[0] == '\r') {
		s = s[1:]
	}
	return s
}

// fixedWidthLayout 限定子对象的宽度为指定值，高度取最大值。
type fixedWidthLayout struct {
	Width float32
}

func (l *fixedWidthLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) == 0 {
		return fyne.NewSize(l.Width, 0)
	}
	w, h := l.Width, float32(0)
	for _, o := range objects {
		s := o.MinSize()
		if s.Height > h {
			h = s.Height
		}
	}
	return fyne.NewSize(w, h)
}

func (l *fixedWidthLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	w := l.Width
	if w > size.Width {
		w = size.Width
	}
	for _, o := range objects {
		o.Move(fyne.NewPos(0, 0))
		o.Resize(fyne.NewSize(w, size.Height))
	}
}
