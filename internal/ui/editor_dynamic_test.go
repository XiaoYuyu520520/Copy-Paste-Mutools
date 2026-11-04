package ui

import (
	"testing"
	"time"

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
	ed := NewTabEditor("测试", cfg, nil)
	ed.rebuildRows()
	return ed
}

func stopEditorTimers(ed *TabEditor) {
	if ed == nil {
		return
	}
	if ed.mergeTimer != nil {
		ed.mergeTimer.Stop()
	}
	if ed.saveTimer != nil {
		ed.saveTimer.Stop()
	}
}

func TestEditorDynamicRows(t *testing.T) {
	ed := newTestEditor(t, 5)
	defer stopEditorTimers(ed)
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
	defer stopEditorTimers(ed)
	ed.Config.Prompts[0].Text = "KEEP"
	ed.pushUndo() // 快照：KEEP
	ed.Clear()
	ed.pushUndo() // 快照：清空之后（空）
	ed.rebuildRows()
	if ed.Config.Prompts[0].Text != "" {
		t.Fatal("clear failed")
	}
	// 撤销恢复到「清空之前」的状态
	ed.undoLast()
	if ed.Config.Prompts[0].Text != "KEEP" {
		t.Fatalf("undo failed: %q", ed.Config.Prompts[0].Text)
	}
}

func TestEditorMergeModesInUI(t *testing.T) {
	ed := newTestEditor(t, 2)
	defer stopEditorTimers(ed)
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

// 快速连续修改后，只有最新一次合并结果能落到预览上（旧回调必须被丢弃）。
func TestEditorStaleMergeDoesNotOverridePreview(t *testing.T) {
	ed := newTestEditor(t, 2)
	defer stopEditorTimers(ed)

	ed.Config = data.ResizeTab(ed.Config, 1)
	ed.Config.Mode = data.MergeModeRaw
	ed.Config.Sentences[0].Text = ""
	ed.Config.Prompts[0].Text = "旧内容"
	ed.scheduleMerge()

	// 在去抖窗口内立刻改成新内容并再次调度。
	ed.Config.Prompts[0].Text = "新内容"
	ed.scheduleMerge()

	time.Sleep(3 * mergeDebounce)
	if got := data.MergeTab(ed.Config); got != "新内容" {
		t.Fatalf("合并结果 = %q, want 新内容", got)
	}
	// 版本号必须已推进到最新一次请求。
	ed.mergeMu.Lock()
	seq := ed.mergeSeq
	ed.mergeMu.Unlock()
	if seq < 2 {
		t.Fatalf("mergeSeq = %d, want >= 2", seq)
	}
}

// 多级撤销/重做：连续多次结构操作后可以逐级回退与前进。
func TestEditorMultiLevelUndoRedo(t *testing.T) {
	ed := newTestEditor(t, 2)
	defer stopEditorTimers(ed)

	// 语义：pushUndo 记录「操作前」的状态，因此先记录再修改。
	ed.pushUndo() // 快照：P（初始）
	ed.Config.Prompts[0].Text = "v1"
	ed.pushUndo() // 快照：v1
	ed.Config.Prompts[0].Text = "v2"
	ed.pushUndo() // 快照：v2

	// 栈顶是「最后一次修改之前」的状态：
	// 当前值 v2 → 撤销回到 v1 → 再撤销回到 P。
	ed.undoLast()
	if ed.Config.Prompts[0].Text != "v1" {
		t.Fatalf("undo 1 -> %q, want v1", ed.Config.Prompts[0].Text)
	}
	ed.undoLast()
	if ed.Config.Prompts[0].Text != "P" {
		t.Fatalf("undo 2 -> %q, want P", ed.Config.Prompts[0].Text)
	}
	// 栈底保留最初状态作为基线，撤销到最底时栈里只剩它
	if len(ed.undoStack) != 1 {
		t.Fatalf("undo stack should keep the baseline, got %d", len(ed.undoStack))
	}

	// 重做按相反顺序恢复。
	ed.redoLast()
	if ed.Config.Prompts[0].Text != "v1" {
		t.Fatalf("redo 1 -> %q, want v1", ed.Config.Prompts[0].Text)
	}
	ed.redoLast()
	if ed.Config.Prompts[0].Text != "v2" {
		t.Fatalf("redo 2 -> %q, want v2", ed.Config.Prompts[0].Text)
	}
}

// 撤销栈有上限，避免长时间编辑无限增长。
func TestEditorUndoStackBounded(t *testing.T) {
	ed := newTestEditor(t, 1)
	defer stopEditorTimers(ed)
	for i := 0; i < maxUndoDepth+20; i++ {
		ed.pushUndo()
	}
	if len(ed.undoStack) > maxUndoDepth {
		t.Fatalf("undo stack = %d, want <= %d", len(ed.undoStack), maxUndoDepth)
	}
}

// 新操作会清空重做栈。
func TestEditorNewOperationClearsRedo(t *testing.T) {
	ed := newTestEditor(t, 1)
	defer stopEditorTimers(ed)
	ed.pushUndo()
	ed.undoLast()
	if len(ed.redoStack) == 0 {
		t.Fatal("redo stack should have 1 item")
	}
	ed.pushUndo()
	if len(ed.redoStack) != 0 {
		t.Fatalf("redo stack should be cleared, got %d", len(ed.redoStack))
	}
}
