package ui

import (
	"testing"

	"fyne.io/fyne/v2/test"

	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/data"
)

func newTestEditor(t *testing.T, rows int) *TabEditor {
	t.Helper()
	a := test.NewApp()
	_ = a
	cfg := data.DefaultConfig().Tabs[0]
	cfg = data.ResizeTab(cfg, rows)
	for i := 0; i < rows; i++ {
		cfg.Prompts[i].Text = "P"
		cfg.Sentences[i].Text = "S"
	}
	return NewTabEditor("测试", cfg, nil)
}

func TestEditorDynamicRows(t *testing.T) {
	ed := newTestEditor(t, 5)
	if len(ed.rows) != 5 {
		t.Fatalf("rows = %d, want 5", len(ed.rows))
	}

	// 新增区块
	ed.Config = data.AddBlock(ed.Config)
	ed.rebuildRows()
	if len(ed.rows) != 6 {
		t.Fatalf("after add rows = %d", len(ed.rows))
	}

	// 删除区块（保留至少一个）
	for i := 0; i < 6; i++ {
		ed.Config = data.RemoveBlock(ed.Config, 0)
	}
	ed.rebuildRows()
	if len(ed.rows) != 1 {
		t.Fatalf("min rows = %d, want 1", len(ed.rows))
	}

	// 复制与移动保持内容顺序
	ed.Config.Prompts[0].Text = "A"
	ed.Config = data.DuplicateBlock(ed.Config, 0)
	if ed.Config.Prompts[1].Text != "A" {
		t.Fatalf("duplicate failed: %+v", ed.Config.Prompts[:2])
	}
	ed.Config = data.MoveBlock(ed.Config, 1, 0)
	if ed.Config.Prompts[0].Text != "A" {
		t.Fatalf("move failed: %+v", ed.Config.Prompts[:2])
	}
}

func TestEditorUndoAfterClear(t *testing.T) {
	ed := newTestEditor(t, 2)
	ed.Config.Prompts[0].Text = "KEEP"
	ed.pushUndo()
	ed.Clear()
	ed.rebuildRows()
	if ed.Config.Prompts[0].Text != "" {
		t.Fatal("clear failed")
	}
	ed.undoLast()
	if ed.Config.Prompts[0].Text != "KEEP" {
		t.Fatalf("undo failed: %q", ed.Config.Prompts[0].Text)
	}
}

func TestEditorMergeModesInUI(t *testing.T) {
	ed := newTestEditor(t, 2)
	ed.Config.Mode = data.MergeModeRaw
	got := data.MergeTab(ed.Config)
	if got != "P\nS\nP\nS" {
		t.Fatalf("raw merge = %q", got)
	}
	ed.Config.Mode = data.MergeModeCustom
	ed.Config.Separator = "|"
	if got := data.MergeTab(ed.Config); got != "P\nS|P\nS" {
		t.Fatalf("custom merge = %q", got)
	}
}
