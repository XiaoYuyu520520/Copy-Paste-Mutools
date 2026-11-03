package data

import "testing"

func benchmarkTab(rows int) TabConfig {
	t := NormalizeTab(TabConfig{})
	t = ResizeTab(t, rows)
	for i := range t.Prompts {
		t.Prompts[i].Text = "请根据以下要求编写代码，保持可读性与测试覆盖。"
		t.Sentences[i].Text = "func example() error { return nil }"
	}
	return t
}

func BenchmarkMergeTabNatural_100Rows(b *testing.B) {
	t := benchmarkTab(100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = MergeTabMode(t, MergeModeNatural)
	}
}

func BenchmarkMergeTabRaw_1000Rows(b *testing.B) {
	t := benchmarkTab(1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = MergeTabMode(t, MergeModeRaw)
	}
}

func BenchmarkResizeTabGrow(b *testing.B) {
	base := benchmarkTab(5)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ResizeTab(base, 200)
	}
}

func BenchmarkExtractVariables(b *testing.B) {
	t := benchmarkTab(50)
	for i := range t.Prompts {
		t.Prompts[i].Text += " {{项目名称}}-{{版本}}"
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ExtractVariables(t)
	}
}
