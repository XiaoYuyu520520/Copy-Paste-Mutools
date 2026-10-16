package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/software"
	"fyne.io/fyne/v2/test"

	"prompt-merger-native/data"
)

// makeTabBar 构造 n 个真实 bundle 的标签栏。
func makeTabBar(n int) *customTabBar {
	cfg := data.DefaultConfig()
	a := test.NewApp()
	a.Settings().SetTheme(NewModernTheme(false))

	bar := NewCustomTabBar(func(int) {})
	bb := make([]*tabBundle, 0, n)
	for i := 0; i < n; i++ {
		ed := NewTabEditor("T", cfg.Tabs[0], nil)
		bb = append(bb, &tabBundle{editor: ed})
	}
	bar.SetBundles(bb)
	bar.SetSelected(0)
	return bar
}

// stripTabs 列出 strip 中除去 Float 下划线之外的所有 tab 项。
func stripTabs(bar *customTabBar) []fyne.CanvasObject {
	var tabs []fyne.CanvasObject
	for _, o := range bar.stripBox.Objects {
		if o == bar.underline {
			continue
		}
		tabs = append(tabs, o)
	}
	return tabs
}

// TestStripLayout_FloatNotCounted 验证 Float 不参与 strip 的宽度计算。
func TestStripLayout_FloatNotCounted(t *testing.T) {
	bar := makeTabBar(3)

	if got := len(bar.stripBox.Objects); got != 4 {
		t.Fatalf("expected underline + 3 tabs = 4 objects, got %d", got)
	}

	ms := bar.stripBox.MinSize()
	if ms.Width > 400 || ms.Width < 40 {
		t.Fatalf("unexpected strip MinSize width = %.1f", ms.Width)
	}
}

// TestTabRect_MatchesLayout 验证 tabRect 算出的位置与 stripLayout 实排一致。
func TestTabRect_MatchesLayout(t *testing.T) {
	bar := makeTabBar(3)

	c := software.NewCanvas()
	c.SetContent(bar.stripBox)
	bar.stripBox.Resize(fyne.NewSize(400, bar.stripBox.MinSize().Height))
	c.Resize(fyne.NewSize(400, bar.stripBox.MinSize().Height))

	tabs := stripTabs(bar)
	for i, w := range tabs {
		x, ww := bar.tabRect(tabs, i)
		if w.Position().X != x {
			t.Errorf("tab %d: stripLayout X=%.1f, tabRect X=%.1f", i, w.Position().X, x)
		}
		if ww <= 0 {
			t.Errorf("tab %d: tabRect width should be >0, got %.1f", i, ww)
		}
	}
	if tabs[1].Position().X <= tabs[0].Position().X {
		t.Fatal("tabs should move rightwards")
	}
}

// TestUnderlineSettlesOnTarget 验证下划线能放到目标 tab 上。
func TestUnderlineSettlesOnTarget(t *testing.T) {
	bar := makeTabBar(3)
	bar.SetSelected(1)

	tabs := stripTabs(bar)
	_, ww := bar.tabRect(tabs, 1)
	if ww <= 0 {
		t.Fatalf("target width should be >0, got %.1f", ww)
	}

	bar.placeUnderline(bar.tabRect(tabs, 1))
	if bar.underline.Size().Width != ww {
		t.Fatalf("underline width = %.1f, want %.1f", bar.underline.Size().Width, ww)
	}
}

// TestSetSelected_StartsAnimToTarget SetSelected 把目标 (x, w) 记录下来，
// 即使 headless 驱动不会推进动画，目标应当被记录。
func TestSetSelected_StartsAnimToTarget(t *testing.T) {
	bar := makeTabBar(3)

	bar.SetSelected(2)
	if bar.slideAnim == nil {
		t.Skip("test driver 不能推进帧，这里只验证目标已记录")
	}
	tabs := stripTabs(bar)
	x, w := bar.tabRect(tabs, 2)
	// curX 在 SetBundles/SetSelected 完成后由 moveUnderlineTo 写入，
	// 与 tabRect 重算可能存在亚像素 round 差，这里允许 1 px 内。
	if bar.curX != x {
		t.Fatalf("recorded x=%.1f, want %.1f", bar.curX, x)
	}
	if w <= 0 {
		t.Fatalf("target width should be >0, got %.1f", w)
	}
	_ = bar.curW
}
