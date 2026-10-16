package ui

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"prompt-merger-native/data"
)

func colorsEqual(a, b color.Color) bool {
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}

func findTapArea(root fyne.CanvasObject) *tapArea {
	var found *tapArea
	walk(root, func(obj fyne.CanvasObject) {
		if found != nil {
			return
		}
		if tap, ok := obj.(*tapArea); ok {
			found = tap
		}
	})
	return found
}

func TestTopTabTapAreaIsHittable(t *testing.T) {
	app := test.NewApp()
	app.Settings().SetTheme(NewModernTheme(false))

	picked := -1
	bar := NewCustomTabBar(func(idx int) {
		picked = idx
	})
	cfg := data.DefaultConfig()
	bundles := make([]*tabBundle, 3)
	for i := range bundles {
		bundles[i] = &tabBundle{editor: NewTabEditor("T", cfg.Tabs[0], nil)}
	}
	bar.SetBundles(bundles)
	bar.SetSelected(0)

	win := test.NewWindow(bar.View())
	win.Resize(fyne.NewSize(600, 100))
	win.Show()

	tabs := stripTabs(bar)
	if len(tabs) != 3 {
		t.Fatalf("expected 3 tabs, got %d", len(tabs))
	}
	tap := findTapArea(tabs[1])
	if tap == nil {
		t.Fatal("second tab has no tap area")
	}
	if tap.Size().Width <= 0 || tap.Size().Height <= 0 {
		t.Fatalf("tab tap area must be hittable, got %v", tap.Size())
	}

	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(tap)
	pos = pos.Add(fyne.NewPos(tap.Size().Width/2, tap.Size().Height/2))
	test.TapCanvas(win.Canvas(), pos)

	if picked != 1 {
		t.Fatalf("second tab tap picked %d, want 1", picked)
	}
}

func TestLeftRailTapAreaIsHittable(t *testing.T) {
	app := test.NewApp()
	app.Settings().SetTheme(NewModernTheme(false))

	picked := -1
	rail := NewLeftRail([]string{"one", "two", "three"}, func(idx int) {
		picked = idx
	})
	win := test.NewWindow(rail.View())
	win.Resize(fyne.NewSize(300, 300))
	win.Show()

	taps := findAll(rail.View(), func(obj fyne.CanvasObject) bool {
		_, ok := obj.(*tapArea)
		return ok
	})
	if len(taps) != 3 {
		t.Fatalf("expected 3 rail tap areas, got %d", len(taps))
	}
	tap, ok := taps[1].(*tapArea)
	if !ok {
		t.Fatalf("second rail row has unexpected tap type %T", taps[1])
	}
	if tap.Size().Width <= 0 || tap.Size().Height <= 0 {
		t.Fatalf("rail tap area must be hittable, got %v", tap.Size())
	}

	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(tap)
	pos = pos.Add(fyne.NewPos(tap.Size().Width/2, tap.Size().Height/2))
	test.TapCanvas(win.Canvas(), pos)

	if picked != 1 {
		t.Fatalf("second rail row tap picked %d, want 1", picked)
	}
}

func TestCardPanelRecolorsOnThemeChange(t *testing.T) {
	app := test.NewApp()
	app.Settings().SetTheme(NewModernTheme(false))

	root := NewCardWith(widget.NewLabel("card"), 8, 6)
	if _, ok := root.(*fyne.Container); !ok {
		t.Fatalf("NewCardWith returned %T, want *fyne.Container", root)
	}
	panel := newCardPanel(widget.NewLabel("card"), 8, 6)
	light := CurrentPalette()
	if !colorsEqual(panel.bg.FillColor, light.Card) {
		t.Fatalf("initial card fill = %v, want light card %v", panel.bg.FillColor, light.Card)
	}

	app.Settings().SetTheme(NewModernTheme(true))
	panel.Recolor()
	dark := CurrentPalette()
	if !colorsEqual(panel.bg.FillColor, dark.Card) {
		t.Fatalf("recolored card fill = %v, want dark card %v", panel.bg.FillColor, dark.Card)
	}
	if !colorsEqual(panel.stroke.StrokeColor, dark.Border) {
		t.Fatalf("recolored card border = %v, want dark border %v", panel.stroke.StrokeColor, dark.Border)
	}
}
