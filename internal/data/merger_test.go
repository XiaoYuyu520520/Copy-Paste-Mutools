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
