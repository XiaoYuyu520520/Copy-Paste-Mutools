package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/data"
)

// TestContextMenu_RealEventPath 通过 test.TapSecondary 走 Fyne 真实的
// 右键事件分发路径，验证「标签右键」与「区块行右键」能弹出菜单，
// 且菜单里包含预期条目。
//
// 这一层比直接调用方法更能暴露命中测试顺序的问题：
// 覆盖层如果抢走了左键，或被别的控件挡住，右键就收不到。
func TestContextMenu_RealEventPath(t *testing.T) {
	a := test.NewApp()
	a.Settings().SetTheme(NewModernTheme(false))
	cfg := data.DefaultConfig()
	cfg.Tabs = []data.TabConfig{data.ResizeTab(data.TabConfig{}, 2)}
	cfg.Tabs[0].Name = "首个标签"

	win := NewMainWindow(a, &cfg)
	win.Resize(fyne.NewSize(1240, 820))
	win.Show()
	defer win.Close()

	m := latestMainApp
	if m == nil {
		t.Fatal("latestMainApp 未设置")
	}

	// 1) 标签栏：右键第一个标签对应的 tapArea
	tabTap := findTabTap(t)
	test.TapSecondary(tabTap)

	popup := findPopup(t)
	if popup == nil {
		for i, ov := range overlays() {
			t.Logf("overlay[%d] = %T visible=%v", i, ov, ov.Visible())
		}
		t.Fatalf("标签右键没有弹出菜单（找到 %d 个覆盖层）", len(overlays()))
	}
	// 菜单项文本由 TestRowMenuItems 单独覆盖；这里验证接线正确
	// （右键 → 处理函数 → 弹出菜单）且条目数量符合预期。
	if got := countMenuEntries(popup); got < 5 {
		t.Errorf("标签菜单只有 %d 个条目，应至少 5 个", got)
	}
	clearOverlays()

	// 2) 区块行：右键第一行的覆盖层
	ed := m.bundles[0].editor
	if len(ed.rows) == 0 {
		t.Fatal("编辑器没有渲染出区块行")
	}
	rowTap := findSecondaryArea(t, ed.rows[0].root)
	test.TapSecondary(rowTap)

	popup = findPopup(t)
	if popup == nil {
		t.Fatal("区块行右键没有弹出菜单")
	}
	// 区块菜单应有：上移/下移/分隔线/插入×2/分隔线/复制/删除/分隔线/启用×2 = 11 项
	if got := countMenuEntries(popup); got != 11 {
		t.Errorf("区块菜单有 %d 个条目，期望 11", got)
	}
	clearOverlays()
}

// TestContextMenu_LeftClickStillWorks 覆盖层不能抢走左键：
// 点区块里的输入框应当聚焦它，而不是被覆盖层吞掉。
func TestContextMenu_LeftClickStillWorks(t *testing.T) {
	a := test.NewApp()
	a.Settings().SetTheme(NewModernTheme(false))
	cfg := data.DefaultConfig()
	cfg.Tabs = []data.TabConfig{data.ResizeTab(data.TabConfig{}, 2)}

	win := NewMainWindow(a, &cfg)
	win.Resize(fyne.NewSize(1240, 820))
	win.Show()
	defer win.Close()

	m := latestMainApp
	ed := m.bundles[0].editor
	if len(ed.rows) == 0 || ed.rows[0].prompt == nil {
		t.Fatal("缺少区块输入框")
	}
	// 覆盖层的合同：只实现 SecondaryTappable。
	// 只要它没有实现 fyne.Tappable，Fyne 的命中测试在左键时就会继续
	// 深入内部控件，不会把点击吞在覆盖层上。
	sa := findSecondaryArea(t, ed.rows[0].root)
	if _, ok := fyne.CanvasObject(sa).(fyne.Tappable); ok {
		t.Fatal("secondaryArea 实现了 Tappable，会抢走内部控件的左键点击")
	}
	// 输入框本身仍然可以正常获得焦点。
	canvas := fyne.CurrentApp().Driver().CanvasForObject(ed.rows[0].prompt)
	if canvas == nil {
		t.Fatal("输入框不在画布上")
	}
	canvas.Focus(ed.rows[0].prompt)
	if canvas.Focused() != fyne.Focusable(ed.rows[0].prompt) {
		t.Error("输入框无法获得焦点")
	}
}

// findTabTap 在标签栏控件树里找第一个 tapArea（即第一个标签的命中层）。
//
// tabItemLayout 包装的不是 *fyne.Container，要按宽度遍历所有实现
// fyne.CanvasObject 的值：直接查 latestTabBar.tabItems 对应的 stripBox 对象。
func findTabTap(t *testing.T) *tapArea {
	t.Helper()
	b := latestTabBar
	if b == nil || b.stripBox == nil {
		t.Fatal("latestTabBar 未初始化")
	}
	var found *tapArea
	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		if found != nil {
			return
		}
		if ta, ok := o.(*tapArea); ok {
			found = ta
			return
		}
		if c, ok := o.(*fyne.Container); ok {
			for _, ch := range c.Objects {
				walk(ch)
			}
		}
		if w, ok := o.(fyne.Widget); ok {
			r := w.CreateRenderer()
			for _, ch := range r.Objects() {
				walk(ch)
			}
		}
	}
	walk(b.root)
	if found == nil {
		t.Fatal("标签栏里找不到 tapArea")
	}
	return found
}

// findSecondaryArea 在给定对象树里找 secondaryArea 覆盖层。
func findSecondaryArea(t *testing.T, root fyne.CanvasObject) *secondaryArea {
	t.Helper()
	var found *secondaryArea
	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		if found != nil {
			return
		}
		if sa, ok := o.(*secondaryArea); ok {
			found = sa
			return
		}
		if c, ok := o.(*fyne.Container); ok {
			for _, ch := range c.Objects {
				walk(ch)
			}
		}
	}
	walk(root)
	if found == nil {
		t.Fatal("区块行里找不到 secondaryArea 覆盖层")
	}
	return found
}

// findPopup 找到主窗口覆盖层里正在显示的 PopUpMenu。
func findPopup(t *testing.T) *widget.PopUpMenu {
	t.Helper()
	// 覆盖层实际是 *widget.OverlayContainer（包着 PopUpMenu 的容器），
	// 只认「可见」的那个：上一个弹窗被 Hide() 后仍留在列表里。
	for _, ov := range overlays() {
		if !ov.Visible() {
			continue
		}
		if pm := findPopUpMenuIn(ov); pm != nil {
			return pm
		}
	}
	return nil
}

// findPopUpMenuIn 在覆盖层控件树里找 PopUpMenu。
func findPopUpMenuIn(o fyne.CanvasObject) *widget.PopUpMenu {
	if pm, ok := o.(*widget.PopUpMenu); ok {
		return pm
	}
	if c, ok := o.(*fyne.Container); ok {
		for _, ch := range c.Objects {
			if pm := findPopUpMenuIn(ch); pm != nil {
				return pm
			}
		}
	}
	if w, ok := o.(fyne.Widget); ok {
		for _, ch := range w.CreateRenderer().Objects() {
			if pm := findPopUpMenuIn(ch); pm != nil {
				return pm
			}
		}
	}
	return nil
}

// overlays 返回所有窗口画布上的覆盖层（弹窗可能挂在非第 0 个窗口上）。
func overlays() []fyne.CanvasObject {
	var out []fyne.CanvasObject
	for _, w := range fyne.CurrentApp().Driver().AllWindows() {
		if w.Canvas() != nil {
			out = append(out, w.Canvas().Overlays().List()...)
		}
	}
	return out
}

// clearOverlays 清理残留覆盖层（弹窗会挡住后续的命中测试）。
func clearOverlays() {
	for _, ov := range overlays() {
		ov.Hide()
	}
}

// countMenuEntries 统计弹出菜单里的条目数（含分隔线）。
//
// PopUpMenu 内嵌 *widget.Menu，其 Items 是渲染后的控件列表
// （*widget.menuItem 未导出），无法直接读标题；
// 条目文本已由 TestRowMenuItems 用 rowMenuItems() 单独验证，这里只数数量。
func countMenuEntries(pm *widget.PopUpMenu) int {
	return len(pm.Items)
}
