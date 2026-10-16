package ui

import (
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"prompt-merger-native/data"
)

const treeSettingsWidth = 360

// treeScanChunkSize bounds each background scan batch so huge roots cannot
// monopolize the UI thread. The continue button resumes from the saved cursor.
const treeScanChunkSize = 2000

// TreePanel 独立的「文件夹 Tree」页面。
//
// 选择一个文件夹后按类似 tree 命令的缩进格式打印结构，可：
//   - 开关屏蔽隐藏项与常见构建/依赖目录（build、node_modules…）
//   - 添加需要整棵屏蔽的自定义文件夹名
//   - 添加关键词，名称含该词的条目被忽略
type TreePanel struct {
	root fyne.CanvasObject
	win  fyne.Window

	folder string

	pathEntry  *widget.Entry
	folderBtn  *widget.Button
	refreshBtn *widget.Button

	hideHiddenCheck *widget.Check
	hideBuildCheck  *widget.Check

	extraNames []string // 自定义屏蔽文件夹名
	kwNames    []string // 自定义屏蔽关键词
	extraInput *widget.Entry
	kwInput    *widget.Entry
	extraList  *fyne.Container // 各屏蔽项行（名称 + 删除）
	kwList     *fyne.Container

	out     *widget.Entry // 结果
	summary *widget.Label
	copyBtn *widget.Button

	continueBtn *widget.Button
	scanner     *data.TreeScanner
	scanID      uint64
	scanning    bool

	copyTimer *time.Timer
}

// NewTreePanel 构造文件夹 Tree 页面；win 用于弹「选择文件夹」，onBack 返回编辑器。
func NewTreePanel(win fyne.Window, onBack func()) *TreePanel {
	p := &TreePanel{win: win}

	// ---------------- 顶部：返回 + 标题 ----------------
	backBtn := widget.NewButtonWithIcon("返回编辑", theme.NavigateBackIcon(), onBack)
	backBtn.Importance = widget.LowImportance
	title := widget.NewLabelWithStyle("文件夹 Tree", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	header := container.NewBorder(
		nil, nil,
		container.NewHBox(container.NewPadded(backBtn), container.NewPadded(title)),
		nil, nil,
	)

	// ---------------- 工具行：选择文件夹 / 路径 / 刷新 ----------------
	p.folderBtn = widget.NewButtonWithIcon("选择文件夹", theme.FolderOpenIcon(), p.pickFolder)
	p.folderBtn.Importance = widget.MediumImportance

	p.pathEntry = widget.NewEntry()
	p.pathEntry.Disable()
	p.pathEntry.SetPlaceHolder("尚未选择文件夹…")
	p.pathEntry.Wrapping = fyne.TextWrapOff

	p.refreshBtn = widget.NewButtonWithIcon("生成", theme.ViewRefreshIcon(), p.refreshScan)
	p.refreshBtn.Importance = widget.HighImportance
	p.refreshBtn.Disable()

	p.continueBtn = widget.NewButtonWithIcon("继续探查", theme.NavigateNextIcon(), p.continueScan)
	p.continueBtn.Importance = widget.MediumImportance
	p.continueBtn.Hide()

	toolRow := container.NewBorder(
		nil, nil,
		container.NewPadded(p.folderBtn),
		container.NewHBox(
			container.NewPadded(p.continueBtn),
			container.NewPadded(p.refreshBtn),
		),
		container.NewPadded(p.pathEntry),
	)

	// ---------------- 左：结果 ----------------
	resultCap := widget.NewLabelWithStyle("目录结构", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	p.summary = widget.NewLabel("选择文件夹后点「生成」")
	p.summary.Wrapping = fyne.TextWrapWord

	p.out = widget.NewMultiLineEntry()
	p.out.Wrapping = fyne.TextWrapOff
	p.out.SetPlaceHolder("tree 结果会显示在这里，可复制后粘贴给 AI/编辑器…")
	p.out.Disable()

	p.copyBtn = widget.NewButtonWithIcon("复制结果", theme.ContentCopyIcon(), p.copyResult)
	p.copyBtn.Importance = widget.HighImportance
	p.copyBtn.Disable()

	resultTop := container.NewBorder(
		nil, nil,
		container.NewPadded(resultCap),
		container.NewPadded(p.copyBtn),
		container.NewPadded(p.summary),
	)
	leftBox := container.NewBorder(resultTop, nil, nil, nil, container.NewPadded(p.out))

	// ---------------- 右：过滤设置 ----------------
	settings := container.NewVBox()

	p.hideHiddenCheck = widget.NewCheck("屏蔽隐藏项（.git / .idea / .vscode…）", nil)
	p.hideHiddenCheck.SetChecked(true)
	settings.Add(container.NewPadded(p.hideHiddenCheck))

	p.hideBuildCheck = widget.NewCheck("屏蔽常见构建/依赖目录", nil)
	p.hideBuildCheck.SetChecked(true)
	settings.Add(container.NewPadded(p.hideBuildCheck))
	buildHint := widget.NewLabelWithStyle(
		"内置：build · dist · out · bin · obj · node_modules · vendor · target · __pycache__ · Debug · Release …",
		fyne.TextAlignLeading,
		fyne.TextStyle{Italic: true},
	)
	buildHint.Wrapping = fyne.TextWrapWord
	settings.Add(container.NewPadded(buildHint))

	p.hideHiddenCheck.OnChanged = func(bool) { p.refreshScan() }
	p.hideBuildCheck.OnChanged = func(bool) { p.refreshScan() }

	settings.Add(widget.NewSeparator())

	// 自定义屏蔽文件夹
	settings.Add(container.NewPadded(widget.NewLabelWithStyle(
		"屏蔽文件夹（整棵跳过）", fyne.TextAlignLeading, fyne.TextStyle{Bold: true},
	)))
	p.extraInput = widget.NewEntry()
	p.extraInput.SetPlaceHolder("如：output、migrations")
	addExtra := widget.NewButtonWithIcon("添加", theme.ConfirmIcon(), func() {
		p.addExtra(p.extraInput.Text)
		p.extraInput.SetText("")
	})
	extraRow := container.NewGridWithColumns(2,
		container.NewPadded(p.extraInput),
		container.NewPadded(addExtra),
	)
	settings.Add(extraRow)
	p.extraList = container.NewVBox()
	settings.Add(container.NewPadded(p.extraList))

	settings.Add(widget.NewSeparator())

	// 关键词屏蔽
	settings.Add(container.NewPadded(widget.NewLabelWithStyle(
		"屏蔽关键词（名称包含即忽略）", fyne.TextAlignLeading, fyne.TextStyle{Bold: true},
	)))
	p.kwInput = widget.NewEntry()
	p.kwInput.SetPlaceHolder("如：test、draft、bak")
	addKw := widget.NewButtonWithIcon("添加", theme.ConfirmIcon(), func() {
		p.addKeyword(p.kwInput.Text)
		p.kwInput.SetText("")
	})
	kwRow := container.NewGridWithColumns(2,
		container.NewPadded(p.kwInput),
		container.NewPadded(addKw),
	)
	settings.Add(kwRow)
	p.kwList = container.NewVBox()
	settings.Add(container.NewPadded(p.kwList))

	settings.Add(widget.NewSeparator())
	kwHint := widget.NewLabelWithStyle(
		"文件夹名、文件名只要包含关键词就会被整条忽略（目录连同其全部内容）。",
		fyne.TextAlignLeading,
		fyne.TextStyle{Italic: true},
	)
	kwHint.Wrapping = fyne.TextWrapWord
	settings.Add(container.NewPadded(kwHint))

	settingsScroll := container.NewVScroll(container.NewPadded(settings))
	rightCol := container.New(&fixedWidthLayout{Width: treeSettingsWidth}, settingsScroll)

	// 结果 + 设置
	body := container.NewHSplit(leftBox, rightCol)
	body.SetOffset(0.72)

	topBar := container.NewVBox(header, widget.NewSeparator(), toolRow, widget.NewSeparator())

	p.root = container.NewBorder(topBar, nil, nil, nil, body)
	p.root.Hide()

	p.rebuildLists()
	return p
}

func (p *TreePanel) View() fyne.CanvasObject { return p.root }

// pickFolder 弹出文件夹选择对话框。
func (p *TreePanel) pickFolder() {
	if p.win == nil {
		return
	}
	dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil {
			p.setSummary("选择文件夹出错：" + err.Error())
			return
		}
		if uri == nil {
			return
		}
		p.setFolder(uri.Path())
	}, p.win)
}

// setFolder 记录并显示所选路径，随后自动扫描。
func (p *TreePanel) setFolder(dir string) {
	p.folder = dir
	p.pathEntry.SetText(dir)
	p.refreshBtn.Enable()
	p.refreshScan()
}

func (p *TreePanel) setFolderSync(dir string) {
	p.folder = dir
	p.pathEntry.SetText(dir)
	p.refreshBtn.Enable()
	p.refreshScanSync()
}

// addExtra 添加一个要整棵屏蔽的文件夹名。
func (p *TreePanel) addExtra(name string) {
	name = strings.TrimSpace(name)
	if name == "" || p.containsFold(p.extraNames, name) {
		return
	}
	p.extraNames = append(p.extraNames, name)
	p.rebuildLists()
	p.refreshScan()
}

// addKeyword 添加一个关键词。
func (p *TreePanel) addKeyword(kw string) {
	kw = strings.TrimSpace(kw)
	if kw == "" || p.containsFold(p.kwNames, kw) {
		return
	}
	p.kwNames = append(p.kwNames, kw)
	p.rebuildLists()
	p.refreshScan()
}

func (p *TreePanel) containsFold(list []string, s string) bool {
	for _, it := range list {
		if strings.EqualFold(it, s) {
			return true
		}
	}
	return false
}

// removeExtra 移除一项屏蔽文件夹名。
func (p *TreePanel) removeExtra(name string) {
	out := p.extraNames[:0]
	for _, it := range p.extraNames {
		if it != name {
			out = append(out, it)
		}
	}
	p.extraNames = out
	p.rebuildLists()
	p.refreshScan()
}

// removeKeyword 移除一个关键词。
func (p *TreePanel) removeKeyword(kw string) {
	out := p.kwNames[:0]
	for _, it := range p.kwNames {
		if it != kw {
			out = append(out, it)
		}
	}
	p.kwNames = out
	p.rebuildLists()
	p.refreshScan()
}

// rebuildLists 依据当前屏蔽列表重建可删除的行。
func (p *TreePanel) rebuildLists() {
	if p.extraList == nil || p.kwList == nil {
		return
	}
	p.extraList.Objects = p.extraList.Objects[:0]
	for _, name := range p.extraNames {
		p.extraList.Add(p.chipRow(name, p.removeExtra))
	}
	p.kwList.Objects = p.kwList.Objects[:0]
	for _, kw := range p.kwNames {
		p.kwList.Add(p.chipRow(kw, p.removeKeyword))
	}
}

// chipRow 一条「值 + 删除」的过滤器行。
func (p *TreePanel) chipRow(value string, onRemove func(string)) fyne.CanvasObject {
	label := widget.NewLabelWithStyle(value, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	del := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() { onRemove(value) })
	del.Importance = widget.LowImportance
	return container.NewBorder(
		nil, nil,
		container.NewPadded(label),
		container.NewPadded(del),
		nil,
	)
}

// refreshScan 用当前设置重新生成 tree。
func (p *TreePanel) refreshScan() {
	if p.folder == "" {
		return
	}
	scanner, err := p.beginScan()
	if err != nil {
		p.scanner = nil
		p.setSummary("生成失败：" + err.Error())
		return
	}
	p.startScanChunk(p.scanID, scanner)
}

// continueScan resumes the current bounded scan without restarting it.
func (p *TreePanel) continueScan() {
	if p.scanner == nil || p.scanning {
		return
	}
	p.continueBtn.Disable()
	p.refreshBtn.Disable()
	p.startScanChunk(p.scanID, p.scanner)
}

func (p *TreePanel) treeOptions() data.TreeOptions {
	return data.TreeOptions{
		IgnoreHidden:      p.hideHiddenCheck.Checked,
		IgnoreDefaultDirs: p.hideBuildCheck.Checked,
		ExtraDirs:         append([]string(nil), p.extraNames...),
		Keywords:          append([]string(nil), p.kwNames...),
	}
}

func (p *TreePanel) beginScan() (*data.TreeScanner, error) {
	scanner, err := data.NewTreeScanner(p.folder, p.treeOptions())
	if err != nil {
		return nil, err
	}
	p.scanID++
	p.scanner = scanner
	p.scanning = false
	p.out.SetText("")
	p.copyBtn.Disable()
	p.refreshBtn.Disable()
	p.continueBtn.Hide()
	p.setSummary(fmt.Sprintf("正在探查…每批最多检查 %d 项", treeScanChunkSize))
	return scanner, nil
}

func (p *TreePanel) startScanChunk(id uint64, scanner *data.TreeScanner) {
	p.scanning = true
	go func() {
		result, err := scanner.Next(treeScanChunkSize)
		fyne.Do(func() {
			p.applyScanChunk(id, result, err)
		})
	}()
}

func (p *TreePanel) applyScanChunk(id uint64, result data.TreeScanResult, err error) {
	if id != p.scanID {
		return
	}
	p.scanning = false
	if err != nil {
		p.refreshBtn.Enable()
		p.continueBtn.Hide()
		p.setSummary("生成失败：" + err.Error())
		return
	}
	if result.Text != "" {
		p.out.SetText(p.out.Text + result.Text)
		p.out.Refresh()
		p.copyBtn.Enable()
	}
	p.refreshBtn.Enable()

	if result.Done {
		p.continueBtn.Hide()
		p.setSummary(fmt.Sprintf("完成 · 检查 %d 项 · 目录 %d · 文件 %d · 屏蔽 %d",
			result.Visited, result.Stat.Dirs, result.Stat.Files, result.Stat.Ignored))
		return
	}

	p.continueBtn.Show()
	p.continueBtn.Enable()
	p.setSummary(fmt.Sprintf("已检查 %d 项 · 目录 %d · 文件 %d · 屏蔽 %d · 已达单批限制，点击「继续探查」",
		result.Visited, result.Stat.Dirs, result.Stat.Files, result.Stat.Ignored))
}

// refreshScanSync is used by tests and the headless screenshot generator.
func (p *TreePanel) refreshScanSync() {
	if p.folder == "" {
		return
	}
	scanner, err := p.beginScan()
	if err != nil {
		p.scanner = nil
		p.setSummary("生成失败：" + err.Error())
		return
	}
	for {
		result, err := scanner.Next(treeScanChunkSize)
		p.applyScanChunk(p.scanID, result, err)
		if err != nil || result.Done {
			return
		}
	}
}

// copyResult 复制结果到剪贴板并短暂反馈。
func (p *TreePanel) copyResult() {
	if p.out.Text == "" {
		p.setSummary("没有可复制的内容")
		return
	}
	if app := fyne.CurrentApp(); app != nil {
		app.Clipboard().SetContent(p.out.Text)
	}
	p.copyBtn.SetText("已复制 ✓")
	if p.copyTimer != nil {
		p.copyTimer.Stop()
	}
	p.copyTimer = time.AfterFunc(1500*time.Millisecond, func() {
		fyne.Do(func() {
			if p.copyBtn != nil {
				p.copyBtn.SetText("复制结果")
			}
		})
	})
}

func (p *TreePanel) setSummary(s string) {
	if p.summary != nil {
		p.summary.SetText(s)
	}
}
