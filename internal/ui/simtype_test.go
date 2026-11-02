package ui

import "testing"

func kinds(keys []simKey) []simKeyKind {
	out := make([]simKeyKind, 0, len(keys))
	for _, k := range keys {
		out = append(out, k.kind)
	}
	return out
}

func assertKinds(t *testing.T, name string, keys []simKey, want []simKeyKind) {
	t.Helper()
	got := kinds(keys)
	if len(got) != len(want) {
		t.Fatalf("%s: got %d keys %v, want %d %v", name, len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%s: key[%d]=%v, want %v (all=%v)", name, i, got[i], want[i], got)
			return
		}
	}
}

func TestPlanText_DefaultSendEnter(t *testing.T) {
	keys := planText("A\nB", enterKeyModeSend)
	assertKinds(t, "default", keys, []simKeyKind{simKeyChar, simKeyEnter, simKeyChar})
}

func TestPlanText_BlockEnter(t *testing.T) {
	// 屏蔽后整段没有 Enter 按键，避免误触发「发送」
	keys := planText("A\nB\nC", enterKeyModeBlock)
	assertKinds(t, "block", keys, []simKeyKind{simKeyChar, simKeyChar, simKeyChar})
}

func TestPlanText_ReplaceCtrlEnter(t *testing.T) {
	keys := planText("A\nB", enterKeyModeCtrlEnter)
	assertKinds(t, "ctrl+enter", keys, []simKeyKind{simKeyChar, simKeyCtrlEnter, simKeyChar})
}

func TestPlanText_ReplaceSpace(t *testing.T) {
	keys := planText("A\nB", enterKeyModeSpace)
	assertKinds(t, "space", keys, []simKeyKind{simKeyChar, simKeyChar, simKeyChar})
	if keys[1].r != ' ' {
		t.Errorf("expected a space char, got %q", keys[1].r)
	}
}

func TestPlanText_CRLFCollapsesToSingleBreak(t *testing.T) {
	// \r\n 是同一个换行，只产生一次 Enter
	keys := planText("A\r\nB", enterKeyModeSend)
	assertKinds(t, "crlf-collapse", keys, []simKeyKind{simKeyChar, simKeyEnter, simKeyChar})
}

func TestPlanText_PreservesBlankLines(t *testing.T) {
	// 空行必须保留（用于代码/Markdown），不能被折叠
	keys := planText("A\n\nB", enterKeyModeSend)
	assertKinds(t, "blank-line", keys,
		[]simKeyKind{simKeyChar, simKeyEnter, simKeyEnter, simKeyChar})

	// CRLF 混合空行
	keys = planText("A\r\n\r\nB", enterKeyModeSend)
	assertKinds(t, "crlf-blank-line", keys,
		[]simKeyKind{simKeyChar, simKeyEnter, simKeyEnter, simKeyChar})

	// 以换行结尾的文本会保留末尾 Enter
	keys = planText("A\n", enterKeyModeSend)
	assertKinds(t, "trailing", keys, []simKeyKind{simKeyChar, simKeyEnter})

	// 仅 \r（老式 Mac）也作为独立换行处理
	keys = planText("A\rB", enterKeyModeSend)
	assertKinds(t, "lone-cr", keys, []simKeyKind{simKeyChar, simKeyEnter, simKeyChar})
}

func TestPlanText_TabAndUnicode(t *testing.T) {
	keys := planText("你好\t世界", enterKeyModeSend)
	assertKinds(t, "tab+unicode", keys,
		[]simKeyKind{simKeyChar, simKeyChar, simKeyTab, simKeyChar, simKeyChar})
	if keys[0].r != '你' || keys[3].r != '世' {
		t.Errorf("unicode chars lost: %+v", keys)
	}
}

func TestPlanText_Empty(t *testing.T) {
	if got := planText("", enterKeyModeBlock); len(got) != 0 {
		t.Fatalf("empty text should yield no keys, got %v", got)
	}
	if got := planText("\r\n", enterKeyModeBlock); len(got) != 0 {
		t.Fatalf("blocked newlines should yield no keys, got %v", got)
	}
}
