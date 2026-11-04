package data

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestTemplateVariableExtractAndApply(t *testing.T) {
	tb := TabConfig{
		Prompts:    []Item{{Text: "项目：{{项目名称}}", Enabled: true}},
		Sentences:  []Item{{Text: "目标 {{目标}}\n{{项目名称}}", Enabled: true}},
		BlockNames: []string{"{{项目名称}} 说明"},
	}
	vars := ExtractVariables(tb)
	if len(vars) != 2 || vars[0] != "目标" || vars[1] != "项目名称" {
		t.Fatalf("vars = %v", vars)
	}

	out := ApplyVariables(tb, map[string]string{"项目名称": "工具箱", "目标": "发布"})
	if out.Prompts[0].Text != "项目：工具箱" {
		t.Errorf("prompt = %q", out.Prompts[0].Text)
	}
	if out.Sentences[0].Text != "目标 发布\n工具箱" {
		t.Errorf("sentence = %q", out.Sentences[0].Text)
	}
	if out.BlockNames[0] != "工具箱 说明" {
		t.Errorf("name = %q", out.BlockNames[0])
	}

	// 未提供的变量保持原样
	partial := ApplyVariables(tb, map[string]string{"项目名称": "X"})
	if partial.Sentences[0].Text != "目标 {{目标}}\nX" {
		t.Errorf("partial = %q", partial.Sentences[0].Text)
	}
}

func TestTemplateCRUDAndFilter(t *testing.T) {
	var st TemplateStore
	t1 := NewTemplate("代码审查", "开发", "审查 Go 代码", TabConfig{})
	t2 := NewTemplate("周报", "写作", "每周总结", TabConfig{})
	UpsertTemplate(&st, t1)
	UpsertTemplate(&st, t2)
	if len(st.Templates) != 2 {
		t.Fatalf("len = %d", len(st.Templates))
	}

	if got := FilterTemplates(st, "开发", ""); len(got) != 1 || got[0].Name != "代码审查" {
		t.Fatalf("filter category = %+v", got)
	}
	if got := FilterTemplates(st, "", "周"); len(got) != 1 || got[0].Name != "周报" {
		t.Fatalf("filter keyword = %+v", got)
	}
	if cats := TemplateCategories(st); len(cats) != 2 {
		t.Fatalf("categories = %v", cats)
	}

	t1.Name = "代码审查 v2"
	UpsertTemplate(&st, t1)
	if len(st.Templates) != 2 {
		t.Fatal("upsert should update not add")
	}
	if !DeleteTemplate(&st, t2.ID) {
		t.Fatal("delete failed")
	}
	if len(st.Templates) != 1 {
		t.Fatal("delete left extra")
	}
}

func TestTemplateJSONRoundTrip(t *testing.T) {
	st := TemplateStore{Templates: []Template{NewTemplate("A", "c", "d", TabConfig{})}}
	raw, err := ExportTemplatesJSON(st.Templates)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ImportTemplatesJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "A" {
		t.Fatalf("roundtrip = %+v", got)
	}

	// 兼容裸数组
	bare, _ := json.Marshal(st.Templates)
	got, err = ImportTemplatesJSON(bare)
	if err != nil || len(got) != 1 {
		t.Fatalf("bare import = %+v, %v", got, err)
	}
}

func TestLoadTemplatesReturnsError(t *testing.T) {
	dir := t.TempDir()

	// 文件不存在：属于正常情况，不算错误
	if _, err := LoadTemplates(filepath.Join(dir, "missing.json")); err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}

	// 文件损坏：必须返回可识别的错误
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadTemplates(bad); err == nil {
		t.Fatal("损坏的模板文件应返回错误")
	}

	// 正常文件
	ok := filepath.Join(dir, "ok.json")
	if err := os.WriteFile(ok, []byte(`{"templates":[{"id":"x","name":"T"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := LoadTemplates(ok)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(st.Templates) != 1 || st.Templates[0].Name != "T" {
		t.Fatalf("templates = %+v", st.Templates)
	}
}
