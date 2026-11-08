package data

import (
	"strconv"
	"strings"
)

// DefaultRows 是新建标签时的默认区块数（旧版固定 5 行的兼容值）。
const DefaultRows = 5

// MaxRows 保留旧常量名，表示默认区块数；区块数量本身已不再受限。
//
// Deprecated: 新代码请使用 DefaultRows。
const MaxRows = DefaultRows

// 合并模式。
const (
	MergeModeNatural  = "natural"  // 自然语言拼接（旧版行为）
	MergeModeRaw      = "raw"      // 原样拼接，不添加任何标点
	MergeModeMarkdown = "markdown" // Markdown 标题 + 段落
	MergeModeCustom   = "custom"   // 区块之间使用自定义分隔符
)

// Item 描述单条输入文本及其是否启用。
type Item struct {
	Text    string `json:"text"`
	Enabled bool   `json:"enabled"`
}

// TabConfig 单个标签的全部数据。
//
// Prompts/Sentences 保持旧版结构以兼容历史配置；区块数量不再固定，
// 允许任意长度（两列长度由 NormalizeTab 对齐）。
type TabConfig struct {
	Name string `json:"name,omitempty"` // 用户自定义标签名

	Prompts   []Item `json:"prompts"`
	Sentences []Item `json:"sentences"`

	BlockNames []string `json:"block_names,omitempty"` // 每个区块的自定义标题
	Collapsed  []bool   `json:"collapsed,omitempty"`   // 每个区块的折叠状态

	Mode      string `json:"merge_mode,omitempty"` // 合并模式，空 = natural
	Separator string `json:"separator,omitempty"`  // custom 模式的分隔符

	// KeepEmptyHeadings 为 true 时，Markdown 模式保留空区块的标题
	//（适合需要固定模板结构的场景）；默认跳过没有内容的区块。
	KeepEmptyHeadings bool `json:"keep_empty_headings,omitempty"`
}

// NormalizeTab 对齐两列长度并补齐元数据，保持旧配置可读。
func NormalizeTab(t TabConfig) TabConfig {
	n := len(t.Prompts)
	if len(t.Sentences) > n {
		n = len(t.Sentences)
	}
	if n < 1 {
		n = 1
	}
	prompts := make([]Item, n)
	sentences := make([]Item, n)
	for i := 0; i < n; i++ {
		if i < len(t.Prompts) {
			prompts[i] = t.Prompts[i]
		} else {
			prompts[i] = Item{Enabled: true}
		}
		if i < len(t.Sentences) {
			sentences[i] = t.Sentences[i]
		} else {
			sentences[i] = Item{Enabled: true}
		}
	}
	// Prompts / Sentences 已经是新切片，Item 为值类型，
	// 因此返回值与入参不共享底层数组（可安全用于撤销快照）。
	names := make([]string, n)
	copy(names, t.BlockNames)
	collapsed := make([]bool, n)
	copy(collapsed, t.Collapsed)

	t.Prompts = prompts
	t.Sentences = sentences
	t.BlockNames = names
	t.Collapsed = collapsed
	t.Mode = EffectiveMode(t.Mode)
	return t
}

// ResizeTab 把标签调整为 n 个区块（n>=1），保留已有内容。
func ResizeTab(t TabConfig, n int) TabConfig {
	if n < 1 {
		n = 1
	}
	t = NormalizeTab(t)
	if n < len(t.Prompts) {
		t.Prompts = t.Prompts[:n]
		t.Sentences = t.Sentences[:n]
		t.BlockNames = t.BlockNames[:n]
		t.Collapsed = t.Collapsed[:n]
		return t
	}
	for len(t.Prompts) < n {
		t.Prompts = append(t.Prompts, Item{Enabled: true})
		t.Sentences = append(t.Sentences, Item{Enabled: true})
		t.BlockNames = append(t.BlockNames, "")
		t.Collapsed = append(t.Collapsed, false)
	}
	return t
}

// AddBlock 在末尾追加一个空区块。
func AddBlock(t TabConfig) TabConfig {
	return ResizeTab(t, len(t.Prompts)+1)
}

// InsertBlock 在 idx 处插入一个空区块（idx 之后的原有区块整体后移）。
//
// idx 越界时按边界处理：<=0 插到最前，>= 区块数插到最后，
// 因此调用方既可以用「在第 i 块上方插入」= InsertBlock(t, i)，
// 也可以用「在末尾追加」= InsertBlock(t, len(t.Prompts))。
func InsertBlock(t TabConfig, idx int) TabConfig {
	t = NormalizeTab(t)
	n := len(t.Prompts)
	if idx < 0 {
		idx = 0
	}
	if idx > n {
		idx = n
	}
	prompts := make([]Item, 0, n+1)
	prompts = append(prompts, t.Prompts[:idx]...)
	prompts = append(prompts, Item{Enabled: true})
	prompts = append(prompts, t.Prompts[idx:]...)

	sentences := make([]Item, 0, n+1)
	sentences = append(sentences, t.Sentences[:idx]...)
	sentences = append(sentences, Item{Enabled: true})
	sentences = append(sentences, t.Sentences[idx:]...)

	names := make([]string, 0, n+1)
	names = append(names, t.BlockNames[:idx]...)
	names = append(names, "")
	names = append(names, t.BlockNames[idx:]...)

	collapsed := make([]bool, 0, n+1)
	collapsed = append(collapsed, t.Collapsed[:idx]...)
	collapsed = append(collapsed, false)
	collapsed = append(collapsed, t.Collapsed[idx:]...)

	t.Prompts = prompts
	t.Sentences = sentences
	t.BlockNames = names
	t.Collapsed = collapsed
	return t
}

// RemoveBlock 删除指定区块（至少保留一个）。
func RemoveBlock(t TabConfig, idx int) TabConfig {
	t = NormalizeTab(t)
	if len(t.Prompts) <= 1 || idx < 0 || idx >= len(t.Prompts) {
		return t
	}
	t.Prompts = append(t.Prompts[:idx], t.Prompts[idx+1:]...)
	t.Sentences = append(t.Sentences[:idx], t.Sentences[idx+1:]...)
	t.BlockNames = append(t.BlockNames[:idx], t.BlockNames[idx+1:]...)
	t.Collapsed = append(t.Collapsed[:idx], t.Collapsed[idx+1:]...)
	return t
}

// MoveBlock 把区块从 from 移到 to（用于上移/下移/拖拽排序）。
func MoveBlock(t TabConfig, from, to int) TabConfig {
	t = NormalizeTab(t)
	n := len(t.Prompts)
	if from < 0 || from >= n || to < 0 || to >= n || from == to {
		return t
	}
	// 通用切片移动（from<to 与 from>to 两个方向）。
	rotate := func(vals []Item) {
		v := vals[from]
		if from < to {
			copy(vals[from:to], vals[from+1:to+1])
		} else {
			copy(vals[to+1:from+1], vals[to:from])
		}
		vals[to] = v
	}
	rotate(t.Prompts)
	rotate(t.Sentences)

	names := t.BlockNames
	name := names[from]
	if from < to {
		copy(names[from:to], names[from+1:to+1])
	} else {
		copy(names[to+1:from+1], names[to:from])
	}
	names[to] = name

	col := t.Collapsed
	c := col[from]
	if from < to {
		copy(col[from:to], col[from+1:to+1])
	} else {
		copy(col[to+1:from+1], col[to:from])
	}
	col[to] = c
	return t
}

// DuplicateBlock 复制指定区块到其后。
func DuplicateBlock(t TabConfig, idx int) TabConfig {
	t = NormalizeTab(t)
	if idx < 0 || idx >= len(t.Prompts) {
		return t
	}
	insert := func(vals []Item, v Item) []Item {
		out := make([]Item, 0, len(vals)+1)
		out = append(out, vals[:idx+1]...)
		out = append(out, v)
		out = append(out, vals[idx+1:]...)
		return out
	}
	t.Prompts = insert(t.Prompts, t.Prompts[idx])
	t.Sentences = insert(t.Sentences, t.Sentences[idx])

	names := make([]string, 0, len(t.BlockNames)+1)
	names = append(names, t.BlockNames[:idx+1]...)
	names = append(names, t.BlockNames[idx])
	names = append(names, t.BlockNames[idx+1:]...)
	t.BlockNames = names

	col := make([]bool, 0, len(t.Collapsed)+1)
	col = append(col, t.Collapsed[:idx+1]...)
	col = append(col, t.Collapsed[idx])
	col = append(col, t.Collapsed[idx+1:]...)
	t.Collapsed = col
	return t
}

// EffectiveMode 返回合法模式；空/未知值回落到 natural。
func EffectiveMode(mode string) string {
	switch mode {
	case MergeModeRaw, MergeModeMarkdown, MergeModeCustom:
		return mode
	default:
		return MergeModeNatural
	}
}

// MergeTab 按标签自身配置计算合并结果（默认自然语言模式）。
func MergeTab(t TabConfig) string {
	return MergeTabMode(t, t.Mode)
}

// MergeTabMode 使用指定模式计算合并结果。
//
// 注意：自然语言模式保留旧版「提示词：换行语句」与中文标点；
// raw/markdown/custom 不会自动添加中文标点，避免破坏代码与结构化文本。
func MergeTabMode(t TabConfig, mode string) string {
	mode = EffectiveMode(mode)
	n := len(t.Prompts)
	if len(t.Sentences) < n {
		n = len(t.Sentences)
	}
	blocks := make([]string, 0, n)
	for i := 0; i < n; i++ {
		p := t.Prompts[i]
		s := t.Sentences[i]
		pHas := p.Enabled && strings.TrimSpace(p.Text) != ""
		sHas := s.Enabled && strings.TrimSpace(s.Text) != ""

		switch mode {
		case MergeModeRaw:
			parts := make([]string, 0, 2)
			if pHas {
				parts = append(parts, p.Text)
			}
			if sHas {
				parts = append(parts, s.Text)
			}
			if len(parts) > 0 {
				blocks = append(blocks, strings.Join(parts, "\n"))
			}
		case MergeModeMarkdown:
			if !pHas && !sHas && !t.KeepEmptyHeadings {
				// 空区块默认跳过，避免生成一串没有内容的标题。
				continue
			}
			title := blockTitle(t, i)
			var b strings.Builder
			b.WriteString("## ")
			b.WriteString(title)
			if pHas {
				b.WriteString("\n\n")
				b.WriteString(strings.TrimSpace(p.Text))
			}
			if sHas {
				b.WriteString("\n\n")
				b.WriteString(strings.TrimSpace(s.Text))
			}
			blocks = append(blocks, b.String())
		case MergeModeCustom:
			parts := make([]string, 0, 2)
			if pHas {
				parts = append(parts, p.Text)
			}
			if sHas {
				parts = append(parts, s.Text)
			}
			if len(parts) > 0 {
				blocks = append(blocks, strings.Join(parts, "\n"))
			}
		default: // natural
			pText := strings.TrimRight(p.Text, " \t\r\n")
			sText := strings.TrimLeft(s.Text, " \t\r\n")
			includeP := p.Enabled && pText != ""
			includeS := s.Enabled && sText != ""
			switch {
			case includeP && includeS:
				blocks = append(blocks, pText+"：\n"+sText)
			case includeP:
				blocks = append(blocks, pText)
			case includeS:
				blocks = append(blocks, sText)
			}
		}
	}
	if len(blocks) == 0 {
		return ""
	}
	switch mode {
	case MergeModeRaw:
		return strings.Join(blocks, "\n")
	case MergeModeMarkdown:
		return strings.Join(blocks, "\n\n")
	case MergeModeCustom:
		sep := t.Separator
		if sep == "" {
			sep = "\n\n"
		}
		return strings.Join(blocks, sep)
	default:
		return strings.Join(blocks, "，\n") + "。"
	}
}

func blockTitle(t TabConfig, i int) string {
	if i < len(t.BlockNames) && strings.TrimSpace(t.BlockNames[i]) != "" {
		return strings.TrimSpace(t.BlockNames[i])
	}
	return "区块 " + strconv.Itoa(i+1)
}
