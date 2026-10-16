package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"prompt-merger-native/data"
)

// 模拟输入参数默认值。
const (
	simDefaultDelaySec   = 3.0 // 点击「开始输入」后等待几秒再键入
	simDefaultIntervalMS = 50  // 每字符间隔（毫秒）
	simIntervalMinMS     = 5
	simIntervalMaxMS     = 500
	simParamsWidth       = 340
)

// SimInputPanel 独立的「模拟键盘输入」页面。
//
// 布局：左 = 可编辑内容，右 = 参数（可滚动），底部 = 常驻状态栏
// （状态文字 + 进度条 + 时钟倒计时 + 开始/停止），主操作无需滚动即可点到。
type SimInputPanel struct {
	root fyne.CanvasObject

	txt        *widget.Entry // 可编辑的待输入内容
	sourceSel  *widget.Select
	sourceHint *widget.Label
	editors    []*TabEditor // 与 sourceSel 选项一一对应

	delaySlider    *widget.Slider
	intervalSlider *widget.Slider

	repeatEntry *widget.Entry
	blockEnter  *widget.Check
	enterMode   *widget.Select
	enterModeH  *widget.Label

	startBtn *widget.Button
	stopBtn  *widget.Button
	progress *widget.ProgressBar
	status   *widget.Label

	cdBox   *fyne.Container // 时钟图标 + 秒数
	cdLabel *widget.Label

	stopCh chan struct{}
}

// NewSimInputPanel 构造模拟输入页面，onBack 返回编辑器页。
func NewSimInputPanel(onBack func()) *SimInputPanel {
	p := &SimInputPanel{}

	// ---------------- 顶部：返回 + 标题 ----------------
	backBtn := widget.NewButtonWithIcon("返回编辑", theme.NavigateBackIcon(), onBack)
	backBtn.Importance = widget.LowImportance

	title := widget.NewLabelWithStyle(
		"模拟键盘输入",
		fyne.TextAlignLeading,
		fyne.TextStyle{Bold: true},
	)
	header := container.NewBorder(
		nil, nil,
		container.NewHBox(container.NewPadded(backBtn), container.NewPadded(title)),
		nil, nil,
	)

	// ---------------- 左：输入内容（预填所选标签的合并结果） ----------------
	sourceCap := widget.NewLabelWithStyle("内容来源", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	p.sourceSel = widget.NewSelect(nil, func(opt string) {
		p.loadSourceByTitle(opt)
	})
	p.sourceSel.PlaceHolder = "选择来源标签（自动载入其合并结果）"
	p.sourceHint = widget.NewLabelWithStyle(
		"可在下方直接修改内容。\n提示：内容中的换行见右侧「Enter 处理」设置。",
		fyne.TextAlignLeading,
		fyne.TextStyle{Italic: true},
	)
	p.sourceHint.Wrapping = fyne.TextWrapWord

	p.txt = widget.NewMultiLineEntry()
	p.txt.Wrapping = fyne.TextWrapWord
	p.txt.SetPlaceHolder("选择来源标签，或在此直接输入要模拟键入的内容…")

	leftTop := container.NewVBox(
		container.NewPadded(sourceCap),
		container.NewPadded(p.sourceSel),
		container.NewPadded(p.sourceHint),
		widget.NewSeparator(),
	)
	leftBox := container.NewBorder(leftTop, nil, nil, nil, container.NewPadded(p.txt))

	// ---------------- 右：参数面板（可滚动，主按钮不在其中） ----------------
	params := container.NewVBox()

	delayRow, delaySlider, _ := p.sliderRow(
		"开始延迟（秒）", 0, 10, simDefaultDelaySec,
		func(v float64) string { return fmt.Sprintf("%.1f", v) },
	)
	p.delaySlider = delaySlider
	params.Add(delayRow)

	params.Add(widget.NewSeparator())

	intervalRow, intervalSlider, _ := p.sliderRow(
		"字符间隔（毫秒）", simIntervalMinMS, simIntervalMaxMS, simDefaultIntervalMS,
		func(v float64) string { return fmt.Sprintf("%.0f", v) },
	)
	p.intervalSlider = intervalSlider
	params.Add(intervalRow)

	params.Add(widget.NewSeparator())

	repeatCap := widget.NewLabelWithStyle("重复次数", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	repeatEntry := widget.NewEntry()
	repeatEntry.SetText("1")
	repeatEntry.Validator = func(s string) error {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil || n < 1 {
			return fmt.Errorf("请输入 >=1 的整数")
		}
		return nil
	}
	p.repeatEntry = repeatEntry
	params.Add(container.NewBorder(nil, nil, repeatCap, nil,
		container.NewPadded(repeatEntry)))

	params.Add(widget.NewSeparator())

	enterCap := widget.NewLabelWithStyle("Enter 处理（防误发）", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	params.Add(container.NewPadded(enterCap))
	p.blockEnter = widget.NewCheck("屏蔽 Enter（不发送换行）", nil)
	p.blockEnter.OnChanged = func(b bool) {
		if p.enterMode == nil {
			return
		}
		if b {
			p.enterMode.Disable()
		} else {
			p.enterMode.Enable()
		}
	}
	params.Add(container.NewPadded(p.blockEnter))

	p.enterModeH = widget.NewLabelWithStyle(
		"未屏蔽时，文本里的换行改为：",
		fyne.TextAlignLeading,
		fyne.TextStyle{Italic: true},
	)
	p.enterModeH.Wrapping = fyne.TextWrapWord
	enterMode := widget.NewSelect(
		[]string{
			enterModeName(enterKeyModeSend),
			enterModeName(enterKeyModeCtrlEnter),
			enterModeName(enterKeyModeSpace),
		},
		func(string) {},
	)
	enterMode.SetSelected(enterModeName(enterKeyModeCtrlEnter))
	p.enterMode = enterMode
	params.Add(container.NewPadded(p.enterModeH))
	params.Add(container.NewPadded(enterMode))

	paramScroll := container.NewVScroll(container.NewPadded(params))
	rightCol := container.New(&fixedWidthLayout{Width: simParamsWidth}, paramScroll)

	// 左内容 + 右参数
	body := container.NewHSplit(leftBox, rightCol)
	body.SetOffset(0.72)

	// ---------------- 底部常驻：状态 / 进度 / 倒计时 / 开始、停止 ----------------
	p.progress = widget.NewProgressBar()
	p.progress.Min = 0
	p.progress.Max = 1
	p.status = widget.NewLabel("就绪：选择来源标签或直接输入，然后点「开始输入」")
	p.status.Wrapping = fyne.TextWrapWord

	p.cdLabel = widget.NewLabelWithStyle("3", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	clockIcon := widget.NewIcon(theme.HistoryIcon())
	cdUnit := widget.NewLabel("秒")
	p.cdBox = container.NewHBox(clockIcon, p.cdLabel, cdUnit)
	p.cdBox.Hide()

	startBtn := widget.NewButtonWithIcon("开始输入", theme.MediaPlayIcon(), p.startTyping)
	startBtn.Importance = widget.HighImportance
	p.startBtn = startBtn

	stopBtn := widget.NewButtonWithIcon("停止", theme.MediaStopIcon(), p.stopTyping)
	stopBtn.Importance = widget.LowImportance
	stopBtn.Disable()
	p.stopBtn = stopBtn

	// 采用两段式底栏：
	// 第一行：状态文字（整行换行）；第二行：进度条铺满左侧 + 倒计时与按钮固定右侧。
	statusRow := container.NewBorder(
		nil, nil,
		container.NewPadded(p.status),
		nil,
		nil,
	)
	controlRow := container.NewBorder(
		nil, nil, nil,
		container.NewHBox(
			container.NewPadded(p.cdBox),
			container.NewPadded(startBtn),
			container.NewPadded(stopBtn),
		),
		container.NewPadded(p.progress),
	)
	bottomBar := container.NewVBox(statusRow, controlRow)

	p.root = container.NewBorder(
		container.NewVBox(header, widget.NewSeparator()),
		container.NewVBox(widget.NewSeparator(), bottomBar),
		nil, nil,
		body,
	)
	p.root.Hide()
	return p
}

func (p *SimInputPanel) View() fyne.CanvasObject { return p.root }

// sliderRow 构造一条「左侧标题 + 右侧数值」的滑块行，返回 (行容器, 滑块, 数值标签)。
func (p *SimInputPanel) sliderRow(caption string, min, max, val float64, format func(float64) string) (fyne.CanvasObject, *widget.Slider, *widget.Label) {
	title := widget.NewLabelWithStyle(caption, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	value := widget.NewLabelWithStyle(format(val), fyne.TextAlignTrailing, fyne.TextStyle{Bold: true})
	slider := widget.NewSlider(min, max)
	slider.Value = val
	slider.OnChanged = func(v float64) {
		value.SetText(format(v))
	}
	head := container.NewBorder(nil, nil, title, container.NewPadded(value), nil)
	row := container.NewVBox(head, slider)
	return row, slider, value
}

// SetBundles 页面被打开时同步来源标签列表并载入当前标签的合并结果。
func (p *SimInputPanel) SetBundles(bundles []*tabBundle, active int) {
	p.editors = p.editors[:0]
	titles := make([]string, 0, len(bundles))
	for i := range bundles {
		p.editors = append(p.editors, bundles[i].editor)
		titles = append(titles, fmt.Sprintf("标签 %d", i+1))
	}
	if len(titles) == 0 {
		return
	}
	if active < 0 || active >= len(titles) {
		active = 0
	}
	p.sourceSel.SetOptions(titles)
	p.sourceSel.SetSelectedIndex(active)
	p.loadSourceByTitle(titles[active])
}

func (p *SimInputPanel) loadSourceByTitle(title string) {
	idx := -1
	for i, t := range p.sourceSel.Options {
		if t == title {
			idx = i
			break
		}
	}
	if idx < 0 || idx >= len(p.editors) {
		return
	}
	text := data.MergeTab(p.editors[idx].Config)
	p.txt.SetText(text)
	if p.sourceHint != nil {
		p.sourceHint.SetText(fmt.Sprintf(
			"已载入「%s」的合并结果（%d 字符），可在下方直接修改。",
			title, len(text),
		))
	}
	p.setStatus("就绪：调整参数后点「开始输入」")
}

// currentEnterMode 依据屏蔽勾选与替换下拉计算最终模式。
func (p *SimInputPanel) currentEnterMode() enterKeyMode {
	if p.blockEnter.Checked {
		return enterKeyModeBlock
	}
	switch p.enterMode.Selected {
	case enterModeName(enterKeyModeCtrlEnter):
		return enterKeyModeCtrlEnter
	case enterModeName(enterKeyModeSpace):
		return enterKeyModeSpace
	default:
		return enterKeyModeSend
	}
}

// setCountdown 显示/隐藏底栏的时钟图标倒计时（n 为剩余整秒）。
func (p *SimInputPanel) setCountdown(n int) {
	if n > 0 {
		p.cdLabel.SetText(strconv.Itoa(n))
		p.cdBox.Show()
	} else {
		p.cdBox.Hide()
	}
}

// startTyping 校验参数并启动模拟输入 goroutine。
func (p *SimInputPanel) startTyping() {
	text := p.txt.Text
	if strings.TrimSpace(text) == "" {
		p.setStatus("输入内容为空：请先选择来源标签或直接输入")
		return
	}
	repeats := 1
	if s := strings.TrimSpace(p.repeatEntry.Text); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 {
			p.setStatus("重复次数必须是 >=1 的整数")
			return
		}
		repeats = n
	}

	mode := p.currentEnterMode()
	keys := planText(text, mode)
	if len(keys) == 0 {
		p.setStatus("屏蔽 Enter 后没有可发送的内容")
		return
	}

	delay := time.Duration(p.delaySlider.Value * float64(time.Second))
	perChar := time.Duration(p.intervalSlider.Value * float64(time.Millisecond))
	total := len(keys) * repeats

	stopCh := make(chan struct{})
	p.stopCh = stopCh
	stopped := func() bool {
		select {
		case <-stopCh:
			return true
		default:
			return false
		}
	}

	p.progress.SetValue(0)
	p.progress.Max = float64(total)
	p.startBtn.Disable()
	p.stopBtn.Enable()
	p.setUIEnabled(false)

	delayWhole := int(delay / time.Second)
	delayFrac := delay - time.Duration(delayWhole)*time.Second
	if delayFrac > 0 {
		delayWhole++ // 显示向上取整的倒计时
	}
	p.setStatus("正在倒计时，请立刻切换到目标程序并点击输入框…")
	if delayWhole > 0 {
		p.setCountdown(delayWhole)
	}

	go func() {
		// 可见的秒级倒计时（时钟图标）
		for i := delayWhole; i > 0 && !stopped(); i-- {
			fyne.Do(func() { p.setCountdown(i) })
			if waitUISecond(stopped) {
				break
			}
		}
		if stopped() {
			p.finishRun(true, nil)
			return
		}
		fyne.Do(func() {
			p.setCountdown(0)
			p.setStatus("开始逐字键入…")
		})

		// 已手动完成倒计时，simulateKeys 内不再等待
		_, err := simulateKeys(
			keys, 0, perChar, repeats, stopped,
			func(done int) {
				fyne.Do(func() {
					p.progress.SetValue(float64(done))
					p.setStatus(fmt.Sprintf("正在模拟键入 %d / %d", done, total))
				})
			},
		)
		p.finishRun(false, err)
	}()
}

// waitUISecond 可中断地等待约 1 秒（倒计时步进），被 stopped() 则返回 true。
func waitUISecond(stopped func() bool) bool {
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if stopped() {
			return true
		}
		time.Sleep(30 * time.Millisecond)
	}
	return false
}

// finishRun 收尾：恢复控件并汇总结果，必须在 UI 线程通过 fyne.Do 调用。
func (p *SimInputPanel) finishRun(wasStopped bool, err error) {
	total := int(p.progress.Max)
	fyne.Do(func() {
		p.startBtn.Enable()
		p.stopBtn.Disable()
		p.setUIEnabled(true)
		p.stopCh = nil
		p.setCountdown(0)
		switch {
		case wasStopped:
			p.progress.SetValue(0)
			p.setStatus("已停止")
		case err != nil:
			p.progress.SetValue(0)
			p.setStatus("出错：" + err.Error())
		default:
			p.progress.SetValue(float64(total))
			p.setStatus(fmt.Sprintf("完成：已键入 %d 个按键", total))
		}
	})
}

// stopTyping 触发停止：只会中断本次输入循环（可重复调用，不会重复 close）。
func (p *SimInputPanel) stopTyping() {
	if p.stopCh == nil {
		return
	}
	select {
	case <-p.stopCh:
		// 已在停止中
	default:
		close(p.stopCh)
	}
}

// setUIEnabled 输入期间锁定大部分控件，避免中途改动参数。
func (p *SimInputPanel) setUIEnabled(on bool) {
	for _, w := range []fyne.Disableable{
		p.txt, p.sourceSel, p.delaySlider, p.intervalSlider,
		p.repeatEntry, p.blockEnter, p.enterMode,
	} {
		if w == nil {
			continue
		}
		if on {
			w.Enable()
		} else {
			w.Disable()
		}
	}
	if on {
		// 恢复 block 勾选对下拉的禁用语义
		p.blockEnter.OnChanged(p.blockEnter.Checked)
	}
}

func (p *SimInputPanel) setStatus(s string) {
	if p.status != nil {
		p.status.SetText(s)
	}
}
