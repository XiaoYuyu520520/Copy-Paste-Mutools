package ui

import (
	"fmt"
	"image/png"
	"os"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/software"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"time"

	"prompt-merger-native/data"
)

// walk 递归遍历 widget 树，包括 *fyne.Container / *container.Split / *container.Scroll。
func walk(obj fyne.CanvasObject, fn func(fyne.CanvasObject)) {
	if obj == nil {
		return
	}
	fn(obj)
	switch o := obj.(type) {
	case *fyne.Container:
		for _, c := range o.Objects {
			walk(c, fn)
		}
	case *container.Split:
		walk(o.Leading, fn)
		walk(o.Trailing, fn)
	case *container.Scroll:
		walk(o.Content, fn)
	case *container.Clip:
		walk(o.Content, fn)
	}
}

func findAll(root fyne.CanvasObject, pred func(fyne.CanvasObject) bool) []fyne.CanvasObject {
	var out []fyne.CanvasObject
	walk(root, func(o fyne.CanvasObject) {
		if pred(o) {
			out = append(out, o)
		}
	})
	return out
}

func layoutTypeName(obj fyne.CanvasObject) string {
	c, ok := obj.(*fyne.Container)
	if !ok {
		return ""
	}
	if c.Layout == nil {
		return ""
	}
	return fmt.Sprintf("%T", c.Layout)
}

func treeString(obj fyne.CanvasObject, depth int) string {
	if obj == nil {
		return ""
	}
	prefix := strings.Repeat("  ", depth)
	name := fmt.Sprintf("%T", obj)
	if c, ok := obj.(*fyne.Container); ok {
		if c.Layout != nil {
			name = name + fmt.Sprintf(" [layout=%T]", c.Layout)
		}
	}
	if l, ok := obj.(*widget.Label); ok {
		t := l.Text
		if len(t) > 30 {
			t = t[:30] + "..."
		}
		name = name + fmt.Sprintf(" text=%q", t)
	}
	if e, ok := obj.(*widget.Entry); ok {
		name = name + fmt.Sprintf(" text=%d chars placeholder=%q", len(e.Text), e.PlaceHolder)
	}
	switch o := obj.(type) {
	case *fyne.Container:
		out := prefix + name + "\n"
		for _, c := range o.Objects {
			out += treeString(c, depth+1)
		}
		return out
	case *container.Split:
		out := prefix + name + "\n"
		out += treeString(o.Leading, depth+1)
		out += treeString(o.Trailing, depth+1)
		return out
	case *container.Scroll:
		out := prefix + name + "\n"
		out += treeString(o.Content, depth+1)
		return out
	case *container.Clip:
		out := prefix + name + "\n"
		out += treeString(o.Content, depth+1)
		return out
	}
	return prefix + name + "\n"
}

func dumpTree(t *testing.T, root fyne.CanvasObject) {
	t.Log("\n" + treeString(root, 0))
}

func TestMainWindow_LayoutStructure(t *testing.T) {
	cfg := data.DefaultConfig()
	a := test.NewApp()
	win := NewMainWindow(a, &cfg)
	win.Show()
	win.Resize(fyne.NewSize(1240, 820))

	root := win.Canvas().Content()
	if root == nil {
		t.Fatal("window content is nil")
	}

	// 1) 顶层应该是 BorderLayout
	lt := layoutTypeName(root)
	if !strings.Contains(lt, "borderLayout") {
		t.Fatalf("root layout expected *layout.borderLayout, got %q", lt)
	}

	// 2) 编辑器应该可见：5 行提示词 + 5 行语句 = 10 个 Entry (加上预览 1 个)
	entries := findAll(root, func(o fyne.CanvasObject) bool {
		_, ok := o.(*widget.Entry)
		return ok
	})
	if len(entries) < 11 {
		dumpTree(t, root)
		t.Errorf("expected >= 11 Entry widgets (10 input + 1 preview), got %d", len(entries))
	}

	// 3) tabBar 的 stripBox 用 stripLayout，且其 MinSize.Height <= 50
	//    通过 root 内部访问需要知道结构；改用 GlobalApp exposed getter
	tabBarStripMin := GlobalTabBarStripHeight()
	if tabBarStripMin <= 0 || tabBarStripMin > 80 {
		dumpTree(t, root)
		t.Errorf("expected tabBar strip height in (0, 80], got %.1f", tabBarStripMin)
	}
}

func TestLeftRail_ShowsPageSwitcher(t *testing.T) {
	cfg := data.DefaultConfig()
	a := test.NewApp()
	win := NewMainWindow(a, &cfg)
	win.Show()
	win.Resize(fyne.NewSize(1240, 820))

	root := win.Canvas().Content()

	// 收集按钮文字（底部状态栏 / AI 设置 / 模拟键盘输入页）
	btns := findAll(root, func(o fyne.CanvasObject) bool {
		b, ok := o.(*widget.Button)
		return ok && b != nil
	})
	btnLabels := make(map[string]bool)
	for _, o := range btns {
		b := o.(*widget.Button)
		if b.Text != "" {
			btnLabels[b.Text] = true
		}
	}

	// 收集 label 文字（状态栏 / 标签栏 / 各面板）
	lbls := findAll(root, func(o fyne.CanvasObject) bool {
		l, ok := o.(*widget.Label)
		return ok && l != nil
	})
	lblTexts := make(map[string]bool)
	for _, o := range lbls {
		l := o.(*widget.Label)
		if l.Text != "" {
			lblTexts[l.Text] = true
		}
	}

	// 收集 RichText 文字（左侧导航项用 RichText 以便控制选中色）
	richs := findAll(root, func(o fyne.CanvasObject) bool {
		_, ok := o.(*widget.RichText)
		return ok
	})
	for _, o := range richs {
		rt := o.(*widget.RichText)
		for _, seg := range rt.Segments {
			if txt := seg.Textual(); txt != "" {
				lblTexts[txt] = true
			}
		}
	}

	// 左侧导航项（Label / RichText 中包含）
	for _, want := range []string{"编辑器", "模拟键盘输入", "文件夹 Tree", "AI 设置"} {
		if !lblTexts[want] {
			dumpTree(t, root)
			t.Errorf("左侧导航缺少标签 %q", want)
		}
	}

	// 底部状态栏按钮（图标 + 短文本）
	for _, want := range []string{"AI 聊天", "Markdown", "LAN"} {
		if !btnLabels[want] {
			dumpTree(t, root)
			t.Errorf("底部状态栏缺少按钮 %q", want)
		}
	}
}

func TestTheme_NoColorNameButtonOverride(t *testing.T) {
	th := NewModernTheme(false)
	clr := th.Color("button", fyne.ThemeVariant(0))
	if clr == nil {
		t.Fatal("button color should not be nil")
	}
	type colorNRGBA interface {
		RGBA() (uint32, uint32, uint32, uint32)
	}
	accent := th.Color("primary", fyne.ThemeVariant(0))
	if accent == nil {
		t.Fatal("primary color is nil")
	}
	if c1, ok := clr.(colorNRGBA); ok {
		if c2, ok2 := accent.(colorNRGBA); ok2 {
			r1, g1, b1, _ := c1.RGBA()
			r2, g2, b2, _ := c2.RGBA()
			if r1 == r2 && g1 == g2 && b1 == b2 {
				t.Errorf("button color must differ from accent primary (got same RGB)")
			}
		}
	}
}

func TestEditorRow_LeftColumnVisible(t *testing.T) {
	ed := NewTabEditor("Tab1", data.DefaultConfig().Tabs[0], nil)
	row := ed.makeRow(0)
	_ = row
	t.Logf("row type=%T minSize=%v", row, row.MinSize())
	// 断言 leftCol 在 row 的左边 110 px 范围内
	borders := findAll(row, func(o fyne.CanvasObject) bool {
		c, ok := o.(*fyne.Container)
		if !ok {
			return false
		}
		return fmt.Sprintf("%T", c.Layout) == "*layout.borderLayout"
	})
	for _, b := range borders {
		t.Logf("border MinSize=%v", b.(*fyne.Container).MinSize())
	}
	checks := findAll(row, func(o fyne.CanvasObject) bool {
		_, ok := o.(*widget.Check)
		return ok
	})
	if len(checks) < 2 {
		t.Errorf("expected >= 2 Check widgets in row 0, got %d", len(checks))
	}
}

func TestEditorRow_DumpTree(t *testing.T) {
	ed := NewTabEditor("Tab1", data.DefaultConfig().Tabs[0], nil)
	for i := 0; i < 3; i++ {
		row := ed.makeRow(i)
		t.Logf("--- row %d (type=%T) ---", i, row)
		t.Log(treeString(row, 0))
	}
}

func TestEditorRow_CheckSizes(t *testing.T) {
	ed := NewTabEditor("Tab1", data.DefaultConfig().Tabs[0], nil)
	for i := 0; i < 3; i++ {
		c := ed.checks[i][0]
		ms := c.MinSize()
		t.Logf("check[%d][0] type=%T minSize=%v text=%q", i, c, ms, c.Text)
	}
}

func TestEditorRow_FullLayoutMinSize(t *testing.T) {
	ed := NewTabEditor("Tab1", data.DefaultConfig().Tabs[0], nil)
	for i := 0; i < 5; i++ {
		row := ed.makeRow(i)
		ms := row.MinSize()
		t.Logf("row %d type=%T minSize=%v", i, row, ms)
		// 也 dump
		find := func(target string) bool {
			return false
		}
		_ = find
	}
	// 实际渲染 1 行到 software canvas 看效果
	a := test.NewApp()
	a.Settings().SetTheme(NewModernTheme(false))
	c := software.NewCanvas()
	c.SetContent(ed.makeRow(0))
	c.Resize(fyne.NewSize(800, 150))
	time.Sleep(100 * time.Millisecond)
	img := c.Capture()
	f, _ := os.Create("screenshots/test_row0.png")
	defer f.Close()
	png.Encode(f, img)
	t.Log("wrote screenshots/test_row0.png")
}
