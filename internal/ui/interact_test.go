package ui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/data"
)

// TestShortcutCatalog_Unique 快捷键目录里不允许重复登记同一个组合。
func TestShortcutCatalog_Unique(t *testing.T) {
	seen := map[string]string{}
	for _, s := range shortcutCatalog {
		key := s.shortcutName()
		if prev, ok := seen[key]; ok {
			t.Errorf("快捷键 %s 重复：%q 与 %q", key, prev, s.Label)
		}
		seen[key] = s.Label
	}
}

// TestShortcutCatalog_HelpCoversCatalog 一览弹窗的条目数应与目录一致。
func TestShortcutCatalog_HelpCoversCatalog(t *testing.T) {
	rows := shortcutHelp()
	if len(rows) != len(shortcutCatalog) {
		t.Fatalf("一览条目数 = %d，目录条目数 = %d", len(rows), len(shortcutCatalog))
	}
	for _, r := range rows {
		if r[0] == "" || r[1] == "" {
			t.Errorf("条目缺少文案：%v", r)
		}
	}
}

// TestShortcutCatalog_HasModifierForMost 检查除 F1 外都有修饰键，
// 避免「单键快捷键」在输入时误触发。
func TestShortcutCatalog_HasModifierForMost(t *testing.T) {
	for _, s := range shortcutCatalog {
		if s.Label == "快捷键一览" {
			continue
		}
		if s.Mod == 0 {
			t.Errorf("快捷键 %q 缺少修饰键", s.Label)
		}
	}
}

// TestRowMenuItems 区块菜单条目随位置变化：首块不能上移、末块不能下移。
func TestRowMenuItems(t *testing.T) {
	cfg := data.DefaultConfig()
	ed := NewTabEditor("Tab", data.ResizeTab(cfg.Tabs[0], 3), nil)

	first := ed.rowMenuItems(0)
	up := findMenuItem(first, "上移")
	if up == nil {
		t.Fatal("菜单缺少「上移」")
	}
	if !up.Disabled {
		t.Error("首块的「上移」应被禁用")
	}

	last := ed.rowMenuItems(2)
	down := findMenuItem(last, "下移")
	if down == nil {
		t.Fatal("菜单缺少「下移」")
	}
	if !down.Disabled {
		t.Error("末块的「下移」应被禁用")
	}

	mid := ed.rowMenuItems(1)
	if u := findMenuItem(mid, "上移"); u == nil || u.Disabled {
		t.Error("中间块的「上移」应可用")
	}
	if d := findMenuItem(mid, "下移"); d == nil || d.Disabled {
		t.Error("中间块的「下移」应可用")
	}

	// 删除在只有一个区块时禁用
	single := NewTabEditor("Tab", data.ResizeTab(cfg.Tabs[0], 1), nil)
	del := findMenuItem(single.rowMenuItems(0), "删除本区块")
	if del == nil || !del.Disabled {
		t.Error("只剩一个区块时「删除本区块」应被禁用")
	}
}

// TestRowMenuItems_HasSeparator 菜单里应至少有一条分隔线，避免动作混在一起。
func TestRowMenuItems_HasSeparator(t *testing.T) {
	cfg := data.DefaultConfig()
	ed := NewTabEditor("Tab", data.ResizeTab(cfg.Tabs[0], 2), nil)
	items := ed.rowMenuItems(0)
	seps := 0
	for _, it := range items {
		if it.IsSeparator {
			seps++
		}
	}
	if seps == 0 {
		t.Error("区块菜单应包含分隔线")
	}
}

// TestEditorHasContent 空标签与有内容的标签要能区分（关闭确认依赖它）。
func TestEditorHasContent(t *testing.T) {
	cfg := data.DefaultConfig()
	empty := NewTabEditor("Tab", data.ResizeTab(cfg.Tabs[0], 2), nil)
	if empty.hasContent() {
		t.Error("空标签不应判定为有内容")
	}

	filled := NewTabEditor("Tab", data.ResizeTab(cfg.Tabs[0], 2), nil)
	filled.Config.Prompts[0].Text = "有一段提示词"
	if !filled.hasContent() {
		t.Error("有文本的标签应判定为有内容")
	}

	named := NewTabEditor("Tab", data.ResizeTab(cfg.Tabs[0], 2), nil)
	named.Config.BlockNames[1] = "标题"
	if !named.hasContent() {
		t.Error("只有区块标题也算有内容")
	}
}

// TestTabBatchOperations 标签的复制、移动与批量关闭。
func TestTabBatchOperations(t *testing.T) {
	cfg := data.DefaultConfig()
	cfg.Tabs = []data.TabConfig{
		data.ResizeTab(data.TabConfig{}, 1),
		data.ResizeTab(data.TabConfig{}, 1),
		data.ResizeTab(data.TabConfig{}, 1),
	}
	cfg.Tabs[0].Name = "甲"
	cfg.Tabs[1].Name = "乙"
	cfg.Tabs[2].Name = "丙"

	a := test.NewApp()
	win := NewMainWindow(a, &cfg)
	defer win.Close()

	m := latestMainApp
	if m == nil {
		t.Fatal("无法取回 MainApp")
	}
	if got := len(m.bundles); got != 3 {
		t.Fatalf("标签数 = %d，期望 3", got)
	}

	// 移动：把第 3 个标签（丙）插到第 1 位 → 丙 甲 乙
	m.moveTab(2, 0)
	if m.cfg.Tabs[0].Name != "丙" || m.cfg.Tabs[1].Name != "甲" || m.cfg.Tabs[2].Name != "乙" {
		t.Fatalf("移动后顺序错误: %s %s %s", m.cfg.Tabs[0].Name, m.cfg.Tabs[1].Name, m.cfg.Tabs[2].Name)
	}
	// 控件顺序与配置顺序必须一致
	if m.bundles[0].editor.Title != "丙" {
		t.Errorf("控件顺序与配置不一致: %s", m.bundles[0].editor.Title)
	}
	// 控件顺序与配置顺序必须一致：contentBox 第 i 个对象的标题应以编辑器的
	// Title 对应（View() 每次新建容器，所以按标题核对）。
	for i, b := range m.bundles {
		if i >= len(m.contentBox.Objects) {
			t.Fatalf("contentBox 少了第 %d 个对象", i)
		}
		if got, want := m.cfg.Tabs[i].Name, b.editor.Config.Name; got != want {
			t.Errorf("第 %d 个标签配置名 %q 与控件名 %q 不一致", i, got, want)
		}
	}

	// 复制：在第 1 个标签后插入副本
	m.duplicateTab(0)
	if len(m.bundles) != 4 {
		t.Fatalf("复制后标签数 = %d，期望 4", len(m.bundles))
	}
	if !strings.Contains(m.bundles[1].editor.Title, "副本") {
		t.Errorf("副本标题 = %q", m.bundles[1].editor.Title)
	}

	// 关闭右侧：保留前两个
	m.closeTabsRightOf(0)
	if len(m.bundles) != 1 {
		t.Fatalf("关闭右侧后标签数 = %d，期望 1", len(m.bundles))
	}

	// 关闭其他：保留第 0 个
	m.addBlankTab()
	m.addBlankTab()
	m.closeTabsExcept(0)
	if len(m.bundles) != 1 {
		t.Fatalf("关闭其他后标签数 = %d，期望 1", len(m.bundles))
	}
	if m.cfg.Tabs[0].Name != "丙" {
		t.Errorf("保留下来的标签 = %q，期望 丙", m.cfg.Tabs[0].Name)
	}
}

// TestDuplicateTab_CopiesContent 复制标签要带上内容，而不只是空壳。
func TestDuplicateTab_CopiesContent(t *testing.T) {
	cfg := data.DefaultConfig()
	cfg.Tabs = []data.TabConfig{data.ResizeTab(data.TabConfig{}, 1)}
	cfg.Tabs[0].Name = "原稿"
	cfg.Tabs[0].Prompts[0].Text = "原始提示词"

	a := test.NewApp()
	win := NewMainWindow(a, &cfg)
	defer win.Close()

	m := latestMainApp
	m.duplicateTab(0)
	if len(m.bundles) != 2 {
		t.Fatalf("标签数 = %d，期望 2", len(m.bundles))
	}
	if got := m.bundles[1].editor.Config.Prompts[0].Text; got != "原始提示词" {
		t.Errorf("副本内容 = %q，期望 原始提示词", got)
	}
}

// findMenuItem 按标题查找菜单项。
func findMenuItem(items []*fyne.MenuItem, label string) *fyne.MenuItem {
	for _, it := range items {
		if it.Label == label {
			return it
		}
	}
	return nil
}
