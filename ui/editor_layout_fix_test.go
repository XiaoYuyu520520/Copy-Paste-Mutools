package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"prompt-merger-native/data"
)

func TestEditorPreviewAndRowsUseBoundedLayout(t *testing.T) {
	app := test.NewApp()
	app.Settings().SetTheme(NewModernTheme(false))
	editor := NewTabEditor("T", data.DefaultConfig().Tabs[0], nil)

	win := test.NewWindow(editor.View())
	win.Resize(fyne.NewSize(1240, 820))
	win.Show()

	if editor.previewCard == nil {
		t.Fatal("preview card was not captured")
	}
	if _, ok := interface{}(editor.previewCard.root).(*fyne.Container); !ok {
		t.Fatalf("preview root has type %T, want *fyne.Container", editor.previewCard.root)
	}
	if !editor.previewCard.root.Visible() || editor.previewCard.root.Size().Height <= 0 {
		t.Fatalf("preview card is not renderable: visible=%v size=%v", editor.previewCard.root.Visible(), editor.previewCard.root.Size())
	}
	if got := editor.preview.Size().Height; got != previewMinSize {
		t.Fatalf("preview height=%v, want fixed %v", got, previewMinSize)
	}

	if editor.rowsScroll == nil {
		t.Fatal("rows scroll was not captured")
	}
	contentHeight := editor.rowsScroll.Content.MinSize().Height
	viewportHeight := editor.rowsScroll.Size().Height
	if contentHeight <= viewportHeight {
		t.Fatalf("five editor rows should scroll: content=%v viewport=%v", contentHeight, viewportHeight)
	}
	if contentHeight < data.MaxRows*rowMinHeight {
		t.Fatalf("rows content height=%v, want at least %v", contentHeight, data.MaxRows*rowMinHeight)
	}
}
