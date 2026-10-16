package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"prompt-merger-native/data"
)

func TestTreePanelPathEntryExpands(t *testing.T) {
	app := test.NewApp()
	app.Settings().SetTheme(NewModernTheme(false))
	panel := NewTreePanel(nil, func() {})
	panel.View().Show()

	win := test.NewWindow(panel.View())
	win.Resize(fyne.NewSize(1240, 820))
	win.Show()

	if got := panel.pathEntry.Size().Width; got < 500 {
		t.Fatalf("path field width=%v, want a long path bar", got)
	}
}

func TestTreePanelContinueButtonAfterBoundedScan(t *testing.T) {
	app := test.NewApp()
	app.Settings().SetTheme(NewModernTheme(false))
	panel := NewTreePanel(nil, func() {})
	panel.View().Show()
	panel.scanID = 1
	panel.scanner = &data.TreeScanner{}
	panel.continueBtn.Hide()

	panel.applyScanChunk(1, data.TreeScanResult{
		Text:      "root\nfirst\n",
		Visited:   treeScanChunkSize,
		Truncated: true,
	}, nil)
	if !panel.continueBtn.Visible() || panel.continueBtn.Disabled() {
		t.Fatal("continue button should be visible and enabled after a bounded batch")
	}
	if panel.out.Text != "root\nfirst\n" {
		t.Fatalf("partial output=%q", panel.out.Text)
	}

	panel.applyScanChunk(1, data.TreeScanResult{
		Text:    "last\n",
		Visited: treeScanChunkSize + 1,
		Done:    true,
	}, nil)
	if panel.continueBtn.Visible() {
		t.Fatal("continue button should hide after completion")
	}
	if panel.out.Text != "root\nfirst\nlast\n" {
		t.Fatalf("complete output=%q", panel.out.Text)
	}
}
