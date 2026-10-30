package data

import "strings"

// MaxRows 每组（提示词+语句）最大行数。
const MaxRows = 5

// Item 描述单条输入文本及其是否启用。
type Item struct {
	Text    string `json:"text"`
	Enabled bool   `json:"enabled"`
}

// TabConfig 单个标签的全部数据。
type TabConfig struct {
	Prompts   []Item `json:"prompts"`
	Sentences []Item `json:"sentences"`
}

// MergeTab 计算单个标签合并结果。
func MergeTab(t TabConfig) string {
	n := len(t.Prompts)
	if n == 0 || n != len(t.Sentences) {
		return ""
	}
	parts := make([]string, 0, n)
	for i := 0; i < n; i++ {
		p := t.Prompts[i]
		s := t.Sentences[i]
		pText := strings.TrimRight(p.Text, " \t\r\n")
		sText := strings.TrimLeft(s.Text, " \t\r\n")
		pHas := pText != ""
		sHas := sText != ""
		includeP := p.Enabled && pHas
		includeS := s.Enabled && sHas
		if !includeP && !includeS {
			continue
		}
		switch {
		case includeP && includeS:
			parts = append(parts, pText+"：\n"+sText)
		case includeP:
			parts = append(parts, pText)
		default:
			parts = append(parts, sText)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "，\n") + "。"
}
