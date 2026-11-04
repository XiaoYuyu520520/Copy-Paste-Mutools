package data

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Template 是一个可复用的提示词模板。
type Template struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Category    string    `json:"category,omitempty"`
	Description string    `json:"description,omitempty"`
	Tab         TabConfig `json:"tab"`
	CreatedAt   string    `json:"created_at,omitempty"`
	UpdatedAt   string    `json:"updated_at,omitempty"`
}

// TemplateStore 是本地模板集合（templates.json）。
type TemplateStore struct {
	Templates []Template `json:"templates"`
}

// LoadTemplates 读取模板文件。
//
// 文件不存在时返回空集合且 err 为 nil（属于正常状态）；
// 文件损坏或读取失败时返回可识别的错误，避免调用方「静默丢模板」。
func LoadTemplates(path string) (TemplateStore, error) {
	var st TemplateStore
	buf, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return st, nil
		}
		return st, fmt.Errorf("读取模板文件失败: %w", err)
	}
	if err := json.Unmarshal(buf, &st); err != nil {
		return TemplateStore{}, fmt.Errorf("模板文件解析失败（%s）: %w", filepath.Base(path), err)
	}
	return st, nil
}

// SaveTemplates 写入模板文件。
func SaveTemplates(path string, st TemplateStore) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	buf, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, buf, 0o644)
}

// NewTemplate 用当前标签快照创建模板。
func NewTemplate(name, category, description string, tab TabConfig) Template {
	now := time.Now().Format(time.RFC3339)
	return Template{
		ID:          newTemplateID(),
		Name:        strings.TrimSpace(name),
		Category:    strings.TrimSpace(category),
		Description: strings.TrimSpace(description),
		Tab:         NormalizeTab(tab),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// UpsertTemplate 新增或按 ID 更新模板。
func UpsertTemplate(st *TemplateStore, t Template) {
	t = NormalizeTemplate(t)
	for i := range st.Templates {
		if st.Templates[i].ID == t.ID && t.ID != "" {
			t.CreatedAt = st.Templates[i].CreatedAt
			st.Templates[i] = t
			return
		}
	}
	if t.ID == "" {
		t.ID = newTemplateID()
	}
	st.Templates = append(st.Templates, t)
}

// NormalizeTemplate 补齐模板字段（ID/时间/标签结构）。
func NormalizeTemplate(t Template) Template {
	t.Name = strings.TrimSpace(t.Name)
	if t.Name == "" {
		t.Name = "未命名模板"
	}
	t.Tab = NormalizeTab(t.Tab)
	if t.ID == "" {
		t.ID = newTemplateID()
	}
	now := time.Now().Format(time.RFC3339)
	if t.CreatedAt == "" {
		t.CreatedAt = now
	}
	t.UpdatedAt = now
	return t
}

// DeleteTemplate 按 ID 删除模板，返回是否删除。
func DeleteTemplate(st *TemplateStore, id string) bool {
	for i := range st.Templates {
		if st.Templates[i].ID == id {
			st.Templates = append(st.Templates[:i], st.Templates[i+1:]...)
			return true
		}
	}
	return false
}

// FindTemplate 按 ID 查找。
func FindTemplate(st TemplateStore, id string) (Template, bool) {
	for _, t := range st.Templates {
		if t.ID == id {
			return t, true
		}
	}
	return Template{}, false
}

// FilterTemplates 按分类与关键词筛选（关键词匹配名称/分类/描述）。
func FilterTemplates(st TemplateStore, category, keyword string) []Template {
	category = strings.TrimSpace(category)
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	out := make([]Template, 0, len(st.Templates))
	for _, t := range st.Templates {
		if category != "" && t.Category != category {
			continue
		}
		if keyword != "" {
			hit := strings.Contains(strings.ToLower(t.Name), keyword) ||
				strings.Contains(strings.ToLower(t.Category), keyword) ||
				strings.Contains(strings.ToLower(t.Description), keyword)
			if !hit {
				continue
			}
		}
		out = append(out, t)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// TemplateCategories 返回去重后的分类列表。
func TemplateCategories(st TemplateStore) []string {
	set := map[string]struct{}{}
	for _, t := range st.Templates {
		if c := strings.TrimSpace(t.Category); c != "" {
			set[c] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for c := range set {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

var templateVarRe = regexp.MustCompile(`\{\{\s*([^{}]+?)\s*\}\}`)

// ExtractVariables 提取模板中出现的 {{变量}}，去重并排序。
func ExtractVariables(t TabConfig) []string {
	set := map[string]struct{}{}
	collect := func(s string) {
		for _, m := range templateVarRe.FindAllStringSubmatch(s, -1) {
			name := strings.TrimSpace(m[1])
			if name != "" {
				set[name] = struct{}{}
			}
		}
	}
	for i := range t.Prompts {
		collect(t.Prompts[i].Text)
	}
	for i := range t.Sentences {
		collect(t.Sentences[i].Text)
	}
	for _, n := range t.BlockNames {
		collect(n)
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ApplyVariables 用给定值替换 {{变量}}，未提供的变量保持原样。
func ApplyVariables(t TabConfig, vars map[string]string) TabConfig {
	replace := func(s string) string {
		return templateVarRe.ReplaceAllStringFunc(s, func(m string) string {
			sub := templateVarRe.FindStringSubmatch(m)
			if len(sub) < 2 {
				return m
			}
			key := strings.TrimSpace(sub[1])
			if v, ok := vars[key]; ok {
				return v
			}
			return m
		})
	}
	out := NormalizeTab(t)
	for i := range out.Prompts {
		out.Prompts[i].Text = replace(out.Prompts[i].Text)
	}
	for i := range out.Sentences {
		out.Sentences[i].Text = replace(out.Sentences[i].Text)
	}
	for i := range out.BlockNames {
		out.BlockNames[i] = replace(out.BlockNames[i])
	}
	return out
}

// TemplateExport 是 JSON 导入导出的外层结构。
type TemplateExport struct {
	Version   int        `json:"version"`
	Templates []Template `json:"templates"`
}

// ExportTemplatesJSON 导出模板为 JSON（版本化，便于未来兼容）。
func ExportTemplatesJSON(ts []Template) ([]byte, error) {
	return json.MarshalIndent(TemplateExport{Version: 1, Templates: ts}, "", "  ")
}

// ImportTemplatesJSON 解析 JSON；兼容裸数组与版本化对象。
func ImportTemplatesJSON(raw []byte) ([]Template, error) {
	var exp TemplateExport
	if err := json.Unmarshal(raw, &exp); err == nil && exp.Templates != nil {
		return exp.Templates, nil
	}
	var list []Template
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, err
	}
	return list, nil
}

func newTemplateID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
