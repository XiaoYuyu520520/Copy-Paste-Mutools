package data

import (
	"strings"
	"testing"
)

func TestMerge_Empty(t *testing.T) {
	if MergeTab(TabConfig{}) != "" {
		t.Error("empty config should yield empty string")
	}
}

func TestMerge_Both(t *testing.T) {
	tc := TabConfig{
		Prompts:   []Item{{Text: "A1", Enabled: true}, {Text: "B1", Enabled: true}},
		Sentences: []Item{{Text: "a2", Enabled: true}, {Text: "b2", Enabled: true}},
	}
	want := "A1：\na2，\nB1：\nb2。"
	if got := MergeTab(tc); got != want {
		t.Errorf("\n want=%q\n got =%q", want, got)
	}
}

func TestMerge_OnlyPrompt(t *testing.T) {
	tc := TabConfig{
		Prompts:   []Item{{Text: "X", Enabled: true}, {Text: "  ", Enabled: true}},
		Sentences: []Item{{Text: "y", Enabled: false}, {Text: "y2", Enabled: true}},
	}
	want := "X，\ny2。"
	if got := MergeTab(tc); got != want {
		t.Errorf("\n want=%q\n got =%q", want, got)
	}
}

func TestMerge_OnlySentence(t *testing.T) {
	tc := TabConfig{
		Prompts:   []Item{{Text: "", Enabled: true}, {Text: "p", Enabled: true}},
		Sentences: []Item{{Text: "s1", Enabled: true}, {Text: "s2", Enabled: true}},
	}
	want := "s1，\np：\ns2。"
	if got := MergeTab(tc); got != want {
		t.Errorf("\n want=%q\n got =%q", want, got)
	}
}

func TestMerge_AllDisabled(t *testing.T) {
	tc := TabConfig{
		Prompts:   []Item{{Text: "X", Enabled: false}, {Text: "Y", Enabled: false}},
		Sentences: []Item{{Text: "a", Enabled: false}, {Text: "b", Enabled: true}},
	}
	want := "b。"
	if got := MergeTab(tc); got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

// 与原 web 版语义一致：保留提示词前面空白，保留语句后面空白，
// 只裁剪紧靠 "：\n" 这一侧的空白，避免出现连续空行。
func TestMerge_WhitespacePreservesOuterSides(t *testing.T) {
	tc := TabConfig{
		Prompts:   []Item{{Text: "  A  \n", Enabled: true}},
		Sentences: []Item{{Text: "\n  B  ", Enabled: true}},
	}
	got := MergeTab(tc)
	want := "  A：\nB  。"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
	if strings.Contains(got, "\n\n") {
		t.Errorf("should never contain double newline: %q", got)
	}
}

func TestMergeModes(t *testing.T) {
	tc := TabConfig{
		Prompts:   []Item{{Text: "P1", Enabled: true}, {Text: "P2", Enabled: true}},
		Sentences: []Item{{Text: "S1", Enabled: true}, {Text: "S2", Enabled: false}},
		Separator: "---\n",
	}

	if got := MergeTabMode(tc, MergeModeRaw); got != "P1\nS1\nP2" {
		t.Errorf("raw = %q", got)
	}
	wantMD := "## 区块 1\n\nP1\n\nS1\n\n## 区块 2\n\nP2"
	if got := MergeTabMode(tc, MergeModeMarkdown); got != wantMD {
		t.Errorf("markdown = %q, want %q", got, wantMD)
	}
	if got := MergeTabMode(tc, MergeModeCustom); got != "P1\nS1---\nP2" {
		t.Errorf("custom = %q", got)
	}
	if got := MergeTabMode(tc, MergeModeNatural); got != "P1：\nS1，\nP2。" {
		t.Errorf("natural = %q", got)
	}
}

func TestNormalizeAndBlockOps(t *testing.T) {
	tc := TabConfig{Prompts: []Item{{Text: "a"}}, Sentences: []Item{{Text: "b"}}}
	got := NormalizeTab(tc)
	if len(got.Prompts) != 1 || len(got.Sentences) != 1 {
		t.Fatalf("normalize should keep 1 block, got %d/%d", len(got.Prompts), len(got.Sentences))
	}
	if len(got.BlockNames) != 1 || len(got.Collapsed) != 1 {
		t.Fatalf("metadata not aligned")
	}

	got = ResizeTab(got, DefaultRows)
	if len(got.Prompts) != DefaultRows {
		t.Fatalf("resize failed: %d", len(got.Prompts))
	}

	more := AddBlock(got)
	if len(more.Prompts) != DefaultRows+1 {
		t.Fatalf("add block failed: %d", len(more.Prompts))
	}

	dup := DuplicateBlock(got, 0)
	if dup.Prompts[1].Text != "a" || dup.Sentences[1].Text != "b" {
		t.Fatalf("duplicate failed: %+v", dup.Prompts[:2])
	}

	moved := MoveBlock(dup, 0, 2)
	if moved.Prompts[2].Text != "a" || moved.Prompts[1].Text != "" || moved.Prompts[0].Text != "a" {
		t.Fatalf("move failed: %+v", moved.Prompts[:4])
	}

	removed := RemoveBlock(got, 0)
	if len(removed.Prompts) != DefaultRows-1 {
		t.Fatalf("remove failed: %d", len(removed.Prompts))
	}
	// 至少保留一个区块
	one := ResizeTab(got, 1)
	if len(one.Prompts) != 1 {
		t.Fatalf("resize to 1 failed: %d", len(one.Prompts))
	}
	if kept := RemoveBlock(one, 0); len(kept.Prompts) != 1 {
		t.Fatalf("should keep at least 1 block, got %d", len(kept.Prompts))
	}
}

func TestMarkdownModeSkipsEmptyBlocks(t *testing.T) {
	tc := TabConfig{
		Mode:      MergeModeMarkdown,
		Prompts:   []Item{{Text: "", Enabled: true}, {Text: "内容", Enabled: true}},
		Sentences: []Item{{Text: "", Enabled: true}, {Text: "", Enabled: true}},
	}
	got := MergeTab(tc)
	if got != "## 区块 2\n\n内容" {
		t.Errorf("markdown = %q", got)
	}
	if strings.Contains(got, "区块 1") {
		t.Errorf("空区块标题不应出现: %q", got)
	}
}

func TestMarkdownModeAllEmptyReturnsEmptyString(t *testing.T) {
	tc := TabConfig{Mode: MergeModeMarkdown}
	tc = ResizeTab(tc, 5) // 默认 5 个空区块
	if got := MergeTab(tc); got != "" {
		t.Errorf("全空输入应输出空字符串, got %q", got)
	}
}

func TestMarkdownModeKeepEmptyHeadings(t *testing.T) {
	tc := TabConfig{Mode: MergeModeMarkdown, KeepEmptyHeadings: true}
	tc = ResizeTab(tc, 2)
	tc.Prompts[0].Text = "有内容"
	got := MergeTab(tc)
	if !strings.Contains(got, "## 区块 1") || !strings.Contains(got, "## 区块 2") {
		t.Errorf("保留空标题模式应输出两个标题: %q", got)
	}
}
