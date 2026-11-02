package ui

// enterKeyMode 定义文本中的换行在模拟输入时如何处理。
type enterKeyMode uint8

const (
	// enterKeyModeSend 原样发送 Enter（默认），并保留空行。
	enterKeyModeSend enterKeyMode = iota
	// enterKeyModeCtrlEnter 替换为 Ctrl+Enter。
	enterKeyModeCtrlEnter
	// enterKeyModeSpace 替换为一个空格。
	enterKeyModeSpace
	// enterKeyModeBlock 屏蔽换行：整段中的 Enter 一律不发送。
	enterKeyModeBlock
)

func enterModeName(m enterKeyMode) string {
	switch m {
	case enterKeyModeCtrlEnter:
		return "替换为 Ctrl+Enter"
	case enterKeyModeSpace:
		return "替换为空格"
	case enterKeyModeBlock:
		return "屏蔽（不发送 Enter）"
	default:
		return "发送 Enter（保留空行）"
	}
}

type simKeyKind uint8

const (
	simKeyChar simKeyKind = iota
	simKeyEnter
	simKeyTab
	simKeyCtrlEnter
)

// simKey 一次「按键」：要么是普通可输入字符，要么是特殊按键。
type simKey struct {
	kind simKeyKind
	r    rune
}

// planText 把要输入的一段文本按 enterMode 拆成按键序列。
//
// 换行处理规则（保留原始结构，不再折叠空行）：
//   - \r\n 视为一次换行；单独的 \r 或 \n 也各算一次换行
//   - 连续空行会产生多次 Enter，避免破坏代码、Markdown 等结构化文本
//   - 换行按下述 enterMode 处理：发送 Enter / Ctrl+Enter / 空格 / 屏蔽
//   - \t 发送 Tab；其余字符（含中文）原样保留，交由系统按 Unicode 键入
func planText(s string, enterMode enterKeyMode) []simKey {
	runes := []rune(s)
	out := make([]simKey, 0, len(runes))

	emitBreak := func() {
		switch enterMode {
		case enterKeyModeCtrlEnter:
			out = append(out, simKey{kind: simKeyCtrlEnter})
		case enterKeyModeSpace:
			out = append(out, simKey{kind: simKeyChar, r: ' '})
		case enterKeyModeBlock:
			// 屏蔽：不产生任何按键
		default: // enterKeyModeSend
			out = append(out, simKey{kind: simKeyEnter})
		}
	}

	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '\r' || r == '\n' {
			// CRLF 整体只算一次换行。
			if r == '\r' && i+1 < len(runes) && runes[i+1] == '\n' {
				i++
			}
			emitBreak()
			continue
		}
		if r == '\t' {
			out = append(out, simKey{kind: simKeyTab})
			continue
		}
		out = append(out, simKey{kind: simKeyChar, r: r})
	}
	return out
}
