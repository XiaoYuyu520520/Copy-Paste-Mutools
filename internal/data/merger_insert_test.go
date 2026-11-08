package data

import "testing"

// TestInsertBlock_Middle 在中间插入空区块，原有区块顺序与内容保持。
func TestInsertBlock_Middle(t *testing.T) {
	tc := ResizeTab(TabConfig{}, 3)
	tc.Prompts[0].Text = "a"
	tc.Prompts[1].Text = "b"
	tc.Prompts[2].Text = "c"
	tc.Sentences[1].Text = "B"
	tc.BlockNames[2] = "第三块"

	got := InsertBlock(tc, 1)
	if n := len(got.Prompts); n != 4 {
		t.Fatalf("区块数 = %d，期望 4", n)
	}
	wantPrompts := []string{"a", "", "b", "c"}
	for i, w := range wantPrompts {
		if got.Prompts[i].Text != w {
			t.Errorf("Prompts[%d] = %q，期望 %q", i, got.Prompts[i].Text, w)
		}
	}
	// 语句列与提示词列同步移位：原第 1 块的 "B" 落到第 2 块。
	if got.Sentences[2].Text != "B" {
		t.Errorf("Sentences[2] = %q，期望 B", got.Sentences[2].Text)
	}
	if got.BlockNames[3] != "第三块" {
		t.Errorf("BlockNames[3] = %q，期望 第三块", got.BlockNames[3])
	}
	if got.BlockNames[1] != "" {
		t.Errorf("新块标题应为空，实际 %q", got.BlockNames[1])
	}
	// 新插入的区块默认启用、未折叠
	if !got.Prompts[1].Enabled || !got.Sentences[1].Enabled {
		t.Error("新插入区块应默认启用")
	}
	if got.Collapsed[1] {
		t.Error("新插入区块应默认展开")
	}
	// 各平行切片长度必须一致，否则 NormalizeTab 会补默认值破坏对齐
	if len(got.Sentences) != 4 || len(got.BlockNames) != 4 || len(got.Collapsed) != 4 {
		t.Fatalf("平行切片长度不一致: %d/%d/%d",
			len(got.Sentences), len(got.BlockNames), len(got.Collapsed))
	}
}

// TestInsertBlock_Boundaries 越界下标按边界处理。
func TestInsertBlock_Boundaries(t *testing.T) {
	tc := ResizeTab(TabConfig{}, 2)
	tc.Prompts[0].Text = "a"
	tc.Prompts[1].Text = "b"

	first := InsertBlock(tc, -5)
	if first.Prompts[0].Text != "" || first.Prompts[1].Text != "a" {
		t.Errorf("idx<0 应插到最前，实际 %q,%q", first.Prompts[0].Text, first.Prompts[1].Text)
	}

	last := InsertBlock(tc, 99)
	if len(last.Prompts) != 3 || last.Prompts[2].Text != "" || last.Prompts[1].Text != "b" {
		t.Errorf("idx 越界应插到最后，实际 %+v", last.Prompts)
	}
}

// TestInsertBlock_NormalizesInput 传入未规范化的配置时先规范化再加块。
func TestInsertBlock_NormalizesInput(t *testing.T) {
	got := InsertBlock(TabConfig{}, 0)
	if len(got.Prompts) != 2 {
		t.Fatalf("空配置应先规范化为 1 块再插入，实际 %d 块", len(got.Prompts))
	}
}

// TestInsertBlock_PreservesEnabledFlags 插入不应改动原有区块的启用状态。
func TestInsertBlock_PreservesEnabledFlags(t *testing.T) {
	tc := ResizeTab(TabConfig{}, 2)
	tc.Prompts[0].Enabled = false
	got := InsertBlock(tc, 1)
	if got.Prompts[0].Enabled {
		t.Error("原有区块的启用状态不应被修改")
	}
}
