package ui

import (
	"fmt"
	"sync"
	"time"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/data"
)

const (
	mergeDebounce  = 200 * time.Millisecond
	rowMinHeight   = 90
	previewMinSize = 120
)

// TabEditor 单个标签页的全部 UI 与数据。
//
// 区块数量不再固定：支持新增/删除/复制/上移下移/折叠/重命名。
type TabEditor struct {
	Title  string
	Config data.TabConfig

	rows    []*editorRow
	rowsBox *fyne.Container

	preview     *widget.Entry
	previewCard *cardPanel
	rowsScroll  *container.Scroll

	nameEntry *widget.Entry
	modeSel   *widget.Select
	sepEntry  *widget.Entry
	sepWrap   fyne.CanvasObject
	countLbl  *widget.Label

	// undoStack / redoStack 实现多级撤销与重做。
	undoStack []data.TabConfig
	redoStack []data.TabConfig
	undoBtn   *widget.Button
	redoBtn   *widget.Button

	// 圆角遮罩：Fyne 的 Entry 无圆角 API，用遮罩层把四角「咬」圆。

	OnMerged   func(text string)
	OnDirty    func()
	OnDirtyNow func()
	OnRename   func()

	mergeTimer *time.Timer
	saveTimer  *time.Timer
	prevOutput string

	// mergeSeq 是合并请求的版本号：每次 scheduleMerge 自增。
	// 后台回调携带自己发起时的版本号，回到 UI 线程后若版本已过期
	// 就直接丢弃，避免「旧回调覆盖新预览」。
	mergeSeq uint64
	// mergeMu 保护 mergeSeq 与配置快照的读取。
	mergeMu sync.Mutex
	// restoring 为 true 时，控件回填不再回写 Config（撤销/重做时避免互相覆盖）。
	restoring bool

	window fyne.Window
}

// editorRow 是单个可折叠区块的控件集合。
type editorRow struct {
	root       fyne.CanvasObject
	body       fyne.CanvasObject
	name       *widget.Entry
	promptOn   *widget.Check
	sentenceOn *widget.Check
	prompt     *widget.Entry
	sentence   *widget.Entry
}

// NewTabEditor 创建一个新的标签编辑器。
func NewTabEditor(title string, tab data.TabConfig, win fyne.Window) *TabEditor {
	e := &TabEditor{Title: title, Config: data.NormalizeTab(tab), window: win}
	if e.Title == "" {
		e.Title = "标签"
	}
	if e.Config.Name != "" {
		e.Title = e.Config.Name
	}
	e.build()
	return e
}

func (e *TabEditor) build() {
	e.rowsBox = container.NewVBox()

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

// rebuildRows 按当前 Config 重建全部区块控件。
//
// 纯文本修改不会走这里（Entry 自己维护内容），只有区块数量/顺序变化
// 才需要重建控件，避免每次输入都重排整个列表。
func (e *TabEditor) rebuildRows() {
	if e.rowsBox == nil {
		return
	}
	e.Config = data.NormalizeTab(e.Config)
	// 回填控件时禁止 OnChanged 回写，否则会把刚恢复的快照覆盖掉。
	prevRestoring := e.restoring
	e.restoring = true
	defer func() { e.restoring = prevRestoring }()
	e.rowsBox.Objects = nil
	e.rows = e.rows[:0]
	for i := range e.Config.Prompts {
		row := e.makeRow(i)
		e.rows = append(e.rows, row)
		if i > 0 {
			e.rowsBox.Add(widget.NewSeparator())
		}
		e.rowsBox.Add(row.root)
	}
	e.rowsBox.Refresh()
	e.updateCount()
}

// syncRowsText 把 Config 中的文本回填到已存在的控件（不重建控件树）。
func (e *TabEditor) syncRowsText() {
	prev := e.restoring
	e.restoring = true
	defer func() { e.restoring = prev }()
	for i, row := range e.rows {
		if i >= len(e.Config.Prompts) || i >= len(e.Config.Sentences) {
			break
		}
		if row.prompt != nil && row.prompt.Text != e.Config.Prompts[i].Text {
			row.prompt.SetText(e.Config.Prompts[i].Text)
		}
		if row.sentence != nil && row.sentence.Text != e.Config.Sentences[i].Text {
			row.sentence.SetText(e.Config.Sentences[i].Text)
		}
		if row.name != nil && i < len(e.Config.BlockNames) && row.name.Text != e.Config.BlockNames[i] {
			row.name.SetText(e.Config.BlockNames[i])
		}
	}
}

// View 返回整个编辑器页面。
func (e *TabEditor) View() fyne.CanvasObject {
	// 延迟到真正展示时再创建区块控件，减少构造阶段的无谓布局与字体测量。
	if len(e.rows) == 0 {
		e.rebuildRows()
	}
	e.rowsScroll = container.NewVScroll(e.rowsBox)
	previewPanel := e.previewPanel()
	top := e.toolbar()

	split := container.NewBorder(
		top,
		previewPanel,
		nil,
		nil,
		e.rowsScroll,
	)
	return container.NewPadded(split)
}

func (e *TabEditor) toolbar() fyne.CanvasObject {
	e.nameEntry = widget.NewEntry()
	e.nameEntry.SetPlaceHolder("标签名称")
	e.nameEntry.SetText(e.Title)
	e.nameEntry.OnChanged = func(s string) {
		if e.restoring {
			return
		}
		e.Title = s
		e.Config.Name = s
		e.scheduleSave()
		if e.OnRename != nil {
			e.OnRename()
		}
	}

	modeOpts := []string{
		mergeModeLabel(data.MergeModeNatural),
		mergeModeLabel(data.MergeModeRaw),
		mergeModeLabel(data.MergeModeMarkdown),
		mergeModeLabel(data.MergeModeCustom),
	}
	e.modeSel = widget.NewSelect(modeOpts, func(opt string) {
		e.Config.Mode = mergeModeValue(opt)
		e.updateSeparatorVisibility()
		e.scheduleMerge()
		e.scheduleSave()
	})
	e.modeSel.SetSelected(mergeModeLabel(data.EffectiveMode(e.Config.Mode)))

	e.sepEntry = widget.NewEntry()
	e.sepEntry.SetText(e.Config.Separator)
	e.sepEntry.SetPlaceHolder(`区块分隔符，例如 "\n\n" 或 "---"`)
	e.sepEntry.OnChanged = func(s string) {
		e.Config.Separator = s
		e.scheduleMerge()
		e.scheduleSave()
	}
	sepBox := container.NewVBox(
		widget.NewLabelWithStyle("自定义分隔符（仅自定义模式生效）", fyne.TextAlignLeading, fyne.TextStyle{Italic: true}),
		e.sepEntry,
	)
	e.sepWrap = sepBox
	e.updateSeparatorVisibility()

	addBtn := widget.NewButtonWithIcon("新增区块", theme.ContentAddIcon(), func() {
		e.Config = data.AddBlock(e.Config)
		e.Config.BlockNames[len(e.Config.BlockNames)-1] = ""
		e.pushUndo()
		e.rebuildRows()
		e.scheduleMerge()
		e.scheduleSave()
	})
	addBtn.Importance = widget.MediumImportance

	tplBtn := widget.NewButtonWithIcon("模板", theme.DocumentIcon(), e.showTemplates)

	e.countLbl = widget.NewLabel("")

	titleRow := container.NewBorder(nil, nil,
		widget.NewLabelWithStyle("标签名", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewHBox(widget.NewLabel("拼接模式"), e.modeSel),
		e.nameEntry,
	)
	actionRow := container.NewBorder(nil, nil, nil,
		container.NewHBox(e.countLbl),
		container.NewHBox(addBtn, tplBtn, layout.NewSpacer()),
	)

	return container.NewVBox(
		titleRow,
		actionRow,
		sepBox,
		widget.NewSeparator(),
	)
}

func (e *TabEditor) updateSeparatorVisibility() {
	if e.sepWrap == nil {
		return
	}
	if data.EffectiveMode(e.Config.Mode) == data.MergeModeCustom {
		e.sepWrap.Show()
	} else {
		e.sepWrap.Hide()
	}
}

func mergeModeLabel(m string) string {
	switch m {
	case data.MergeModeRaw:
		return "原样拼接（不加标点）"
	case data.MergeModeMarkdown:
		return "Markdown（标题+段落）"
	case data.MergeModeCustom:
		return "自定义分隔符"
	default:
		return "自然语言（旧版行为）"
	}
}

func mergeModeValue(label string) string {
	switch label {
	case mergeModeLabel(data.MergeModeRaw):
		return data.MergeModeRaw
	case mergeModeLabel(data.MergeModeMarkdown):
		return data.MergeModeMarkdown
	case mergeModeLabel(data.MergeModeCustom):
		return data.MergeModeCustom
	default:
		return data.MergeModeNatural
	}
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
		if app := fyne.CurrentApp(); app != nil {
			app.Clipboard().SetContent(text)
			if e.OnMerged != nil {
				e.OnMerged("__copied__")
			}
		}
	})
	copyBtn.Importance = widget.HighImportance

	e.undoBtn = widget.NewButtonWithIcon("撤销", theme.ContentUndoIcon(), e.undoLast)
	e.undoBtn.Disable()
	e.redoBtn = widget.NewButtonWithIcon("重做", theme.ContentRedoIcon(), e.redoLast)
	e.redoBtn.Disable()

	clearBtn := widget.NewButtonWithIcon("清空此标签", theme.ContentClearIcon(), func() {
		dialog.ShowConfirm("清空", "确认清空此标签的全部文本？可点「撤销」恢复。", func(ok bool) {
			if !ok {
				return
			}
			e.Clear()
			e.pushUndo()
			e.rebuildRows()
			e.scheduleMerge()
			e.scheduleSave()
		}, e.window)
	})
	clearBtn.Importance = widget.LowImportance

	buttonBar := container.NewHBox(
		clearBtn,
		e.undoBtn,
		e.redoBtn,
		layout.NewSpacer(),
		copyBtn,
	)

	previewEntry := WrapExactHeight(e.preview, previewMinSize)

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

// Recolor refreshes theme-dependent card layers without rebuilding the editor.
func (e *TabEditor) Recolor() {
	if e != nil && e.previewCard != nil {
		e.previewCard.Recolor()
	}
}

// makeRow 单个区块：可折叠标题行 + 提示词/语句两列。
func (e *TabEditor) makeRow(i int) *editorRow {
	row := &editorRow{}

	collapsed := i < len(e.Config.Collapsed) && e.Config.Collapsed[i]
	collapseIcon := theme.MenuDropDownIcon()
	if collapsed {
		collapseIcon = theme.MenuExpandIcon()
	}
	collapseBtn := widget.NewButtonWithIcon("", collapseIcon, func() {
		e.Config.Collapsed[i] = !e.Config.Collapsed[i]
		e.rebuildRows()
		e.scheduleSave()
	})
	collapseBtn.Importance = widget.LowImportance

	row.name = widget.NewEntry()
	name := ""
	if i < len(e.Config.BlockNames) {
		name = e.Config.BlockNames[i]
	}
	row.name.SetText(name)
	row.name.SetPlaceHolder(fmt.Sprintf("区块 %d 标题（可选）", i+1))
	row.name.OnChanged = func(s string) {
		if i < len(e.Config.BlockNames) {
			e.Config.BlockNames[i] = s
			e.scheduleMerge()
			e.scheduleSave()
		}
	}

	upBtn := widget.NewButtonWithIcon("", theme.MoveUpIcon(), func() {
		e.Config = data.MoveBlock(e.Config, i, i-1)
		e.pushUndo()
		e.rebuildRows()
		e.scheduleMerge()
		e.scheduleSave()
	})
	upBtn.Importance = widget.LowImportance
	if i == 0 {
		upBtn.Disable()
	}

	downBtn := widget.NewButtonWithIcon("", theme.MoveDownIcon(), func() {
		e.Config = data.MoveBlock(e.Config, i, i+1)
		e.pushUndo()
		e.rebuildRows()
		e.scheduleMerge()
		e.scheduleSave()
	})
	downBtn.Importance = widget.LowImportance
	if i == len(e.Config.Prompts)-1 {
		downBtn.Disable()
	}

	copyBtn := widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() {
		e.Config = data.DuplicateBlock(e.Config, i)
		e.pushUndo()
		e.rebuildRows()
		e.scheduleMerge()
		e.scheduleSave()
	})
	copyBtn.Importance = widget.LowImportance

	delBtn := widget.NewButtonWithIcon("", theme.ContentRemoveIcon(), func() {
		if len(e.Config.Prompts) <= 1 {
			return
		}
		e.Config = data.RemoveBlock(e.Config, i)
		e.pushUndo()
		e.rebuildRows()
		e.scheduleMerge()
		e.scheduleSave()
	})
	delBtn.Importance = widget.LowImportance
	if len(e.Config.Prompts) <= 1 {
		delBtn.Disable()
	}

	header := container.NewBorder(nil, nil,
		container.NewHBox(collapseBtn, row.name),
		container.NewHBox(upBtn, downBtn, copyBtn, delBtn),
		nil,
	)

	row.root = container.NewVBox(header)
	if collapsed {
		return row
	}

	row.promptOn = widget.NewCheck("提示词", nil)
	row.promptOn.SetChecked(e.Config.Prompts[i].Enabled)
	row.sentenceOn = widget.NewCheck("语句", nil)
	row.sentenceOn.SetChecked(e.Config.Sentences[i].Enabled)

	row.prompt = makeCodeEntry()
	row.prompt.SetPlaceHolder(fmt.Sprintf("提示词 %d（可粘贴多行）", i+1))
	row.prompt.SetText(e.Config.Prompts[i].Text)
	row.prompt.Wrapping = fyne.TextWrapWord

	row.sentence = makeCodeEntry()
	row.sentence.SetPlaceHolder(fmt.Sprintf("语句 %d（可粘贴多行）", i+1))
	row.sentence.SetText(e.Config.Sentences[i].Text)
	row.sentence.Wrapping = fyne.TextWrapWord

	row.prompt.OnChanged = func(s string) {
		if e.restoring {
			return
		}
		if i < len(e.Config.Prompts) {
			e.Config.Prompts[i].Text = s
			e.scheduleMerge()
			e.scheduleSave()
		}
	}
	row.sentence.OnChanged = func(s string) {
		if e.restoring {
			return
		}
		if i < len(e.Config.Sentences) {
			e.Config.Sentences[i].Text = s
			e.scheduleMerge()
			e.scheduleSave()
		}
	}
	row.promptOn.OnChanged = func(b bool) {
		if e.restoring {
			return
		}
		if i < len(e.Config.Prompts) {
			e.Config.Prompts[i].Enabled = b
			e.scheduleMerge()
			e.scheduleSave()
		}
	}
	row.sentenceOn.OnChanged = func(b bool) {
		if e.restoring {
			return
		}
		if i < len(e.Config.Sentences) {
			e.Config.Sentences[i].Enabled = b
			e.scheduleMerge()
			e.scheduleSave()
		}
	}

	rowLabel := widget.NewLabelWithStyle(
		fmt.Sprintf("第 %d 块", i+1),
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)
	leftCol := container.New(&editorLeftLayout{}, rowLabel, row.promptOn, row.sentenceOn)
	promptArea := WrapExactHeight(row.prompt, rowMinHeight)
	sentenceArea := WrapExactHeight(row.sentence, rowMinHeight)
	row.body = container.New(
		&editorRowLayout{LeftWidth: 96, Gap: 8},
		leftCol, promptArea, sentenceArea,
	)
	row.root.(*fyne.Container).Add(row.body)
	return row
}

// maxUndoDepth 是撤销栈的最大层数。
const maxUndoDepth = 50

// pushUndo 保存当前快照（多级撤销栈）。
//
// 语义：栈里保存的是「操作之后」的状态，因此调用方应在完成一次
// 结构操作后调用（例如新增区块后）。撤销时恢复它的前一项，
// 栈底是最初状态；重做则按相反顺序前进。
func (e *TabEditor) pushUndo() {
	e.undoStack = append(e.undoStack, data.NormalizeTab(e.Config))
	if len(e.undoStack) > maxUndoDepth {
		e.undoStack = e.undoStack[len(e.undoStack)-maxUndoDepth:]
	}
	// 产生新操作后，重做栈失效。
	e.redoStack = e.redoStack[:0]
	e.updateHistoryButtons()
}

// undoLast 撤销一步。
func (e *TabEditor) undoLast() {
	if len(e.undoStack) == 0 {
		return
	}
	// 栈顶是「最近一次操作之后」的状态：撤销 = 弹出栈顶并恢复它的前一项。
	last := len(e.undoStack) - 1
	cur := data.NormalizeTab(e.Config)
	e.undoStack = e.undoStack[:last]
	e.redoStack = append(e.redoStack, cur)
	if last == 0 {
		// 没有更早的快照：回到空标签。
		e.Config = data.NormalizeTab(data.TabConfig{})
	} else {
		e.Config = data.NormalizeTab(e.undoStack[last-1])
	}
	e.afterHistoryChange()
}

// redoLast 重做一步。
func (e *TabEditor) redoLast() {
	if len(e.redoStack) == 0 {
		return
	}
	// 重做：恢复被撤销掉的状态，并把当前状态放回撤销栈。
	last := len(e.redoStack) - 1
	restored := data.NormalizeTab(e.redoStack[last])
	e.redoStack = e.redoStack[:last]
	e.undoStack = append(e.undoStack, data.NormalizeTab(e.Config))
	e.Config = restored
	e.afterHistoryChange()
}

// afterHistoryChange 撤销/重做后同步界面状态。
func (e *TabEditor) afterHistoryChange() {
	e.Title = e.Config.Name
	if e.Title == "" {
		e.Title = "标签"
	}
	if e.nameEntry != nil {
		e.restoring = true
		e.nameEntry.SetText(e.Title)
		e.restoring = false
	}
	// 区块数量没变时只回填文本，避免整棵控件树重建（增量刷新）。
	if len(e.rows) == len(e.Config.Prompts) {
		e.syncRowsText()
	} else {
		e.rebuildRows()
	}
	e.scheduleMerge()
	e.scheduleSave()
	e.updateHistoryButtons()
	if e.OnRename != nil {
		e.OnRename()
	}
}

// updateHistoryButtons 刷新撤销/重做按钮的可用状态。
func (e *TabEditor) updateHistoryButtons() {
	if e.undoBtn != nil {
		if len(e.undoStack) > 0 {
			e.undoBtn.Enable()
		} else {
			e.undoBtn.Disable()
		}
	}
	if e.redoBtn != nil {
		if len(e.redoStack) > 0 {
			e.redoBtn.Enable()
		} else {
			e.redoBtn.Disable()
		}
	}
}

// scheduleMerge 触发一次合并刷新（去抖）。
//
// 后台线程只读取「发起瞬间的配置快照」，不触碰正在被 UI 修改的 e.Config；
// 回调回到 UI 线程后用版本号校验，过期结果直接丢弃。
func (e *TabEditor) scheduleMerge() {
	if e.mergeTimer != nil {
		e.mergeTimer.Stop()
	}

	e.mergeMu.Lock()
	e.mergeSeq++
	seq := e.mergeSeq
	snapshot := data.NormalizeTab(e.Config)
	e.mergeMu.Unlock()

	e.mergeTimer = time.AfterFunc(mergeDebounce, func() {
		text := data.MergeTab(snapshot)
		fyne.Do(func() {
			e.mergeMu.Lock()
			stale := e.mergeSeq != seq
			e.mergeMu.Unlock()
			if stale {
				// 已经有更新的合并请求在路上，本次结果作废。
				return
			}
			e.prevOutput = text
			e.preview.SetText(text)
			e.updateCount()
			if e.OnMerged != nil {
				e.OnMerged(text)
			}
		})
	})
}

func (e *TabEditor) updateCount() {
	if e.countLbl == nil {
		return
	}
	n := e.ActiveCount()
	chars := utf8.RuneCountInString(e.prevOutput)
	e.countLbl.SetText(fmt.Sprintf("%d 个区块 · %d 字符", n, chars))
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
	e.mergeMu.Lock()
	e.mergeSeq++ // 作废任何在途的旧回调
	e.mergeMu.Unlock()
	text := data.MergeTab(e.Config)
	e.prevOutput = text
	e.preview.SetText(text)
	e.updateCount()
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

// Clear 清空所有文本输入（保留区块结构）。
func (e *TabEditor) Clear() {
	e.Config = data.NormalizeTab(e.Config)
	for i := range e.Config.Prompts {
		e.Config.Prompts[i].Text = ""
		e.Config.Sentences[i].Text = ""
		e.Config.BlockNames[i] = ""
	}
}

// ActiveCount 返回合并后非空的区块数（含勾选状态过滤）。
func (e *TabEditor) ActiveCount() int {
	n := 0
	for i := range e.Config.Prompts {
		if i >= len(e.Config.Sentences) {
			break
		}
		pHas := e.Config.Prompts[i].Enabled && trimSpaces(e.Config.Prompts[i].Text) != ""
		sHas := e.Config.Sentences[i].Enabled && trimSpaces(e.Config.Sentences[i].Text) != ""
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
