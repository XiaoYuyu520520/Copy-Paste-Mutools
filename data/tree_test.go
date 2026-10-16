package data

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeTreeFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := []string{
		"main.go",
		"readme.md",
		"build/out.exe",       // 默认目录 build，应整棵屏蔽
		"src/app.go",
		"src/util/helper.go",
		"vendor/pkg/v.go",     // 默认目录 vendor，应整棵屏蔽
		".git/config",         // 隐藏目录，应屏蔽
		"output/tmp.txt",      // 自定义屏蔽文件夹 output
		"assets/icon_draft.png", // 关键词 draft 命中（文件名）
		"scripts/run.bat",
	}
	for _, f := range files {
		p := filepath.Join(root, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestBuildTree_Filters(t *testing.T) {
	root := makeTreeFixture(t)
	got, st, err := BuildTree(root, TreeOptions{
		IgnoreHidden:      true,
		IgnoreDefaultDirs: true,
		ExtraDirs:         []string{"output"},
		Keywords:          []string{"draft"},
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"main.go", "readme.md", "src", "app.go", "util", "helper.go",
		"scripts", "run.bat",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("tree 应包含 %q\n%s", want, got)
		}
	}
	for _, ban := range []string{
		"build", "out.exe", "vendor", "pkg", ".git", "config",
		"output", "tmp.txt", "icon_draft.png",
	} {
		if strings.Contains(got, ban) {
			t.Errorf("tree 不应包含被屏蔽的 %q\n%s", ban, got)
		}
	}
	// 计入统计：Files = main.go, readme.md, app.go, helper.go, run.bat = 5
	// Dirs = assets, scripts, src, util = 4（assets 仅子文件被屏蔽，目录本身保留）
	if st.Files != 5 || st.Dirs != 4 {
		t.Errorf("统计错误 files=%d dirs=%d ignored=%d", st.Files, st.Dirs, st.Ignored)
	}
}

func TestBuildTree_NoFilterShowsEverything(t *testing.T) {
	root := makeTreeFixture(t)
	got, st, err := BuildTree(root, TreeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"build", "out.exe", "vendor", ".git", "output", "icon_draft.png"} {
		if !strings.Contains(got, want) {
			t.Errorf("无过滤时应包含 %q\n%s", want, got)
		}
	}
	if st.Ignored != 0 {
		t.Errorf("无过滤时不应有忽略项，got %d", st.Ignored)
	}
}

func TestBuildTree_NotDir(t *testing.T) {
	root := makeTreeFixture(t)
	someFile := filepath.Join(root, "main.go")
	if _, _, err := BuildTree(someFile, TreeOptions{}); err == nil {
		t.Error("传入文件应返回错误")
	}
	if _, _, err := BuildTree(filepath.Join(root, "nope"), TreeOptions{}); err == nil {
		t.Error("不存在的路径应返回错误")
	}
}

func TestBuildTree_SortsAndConnectors(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "b.go"), []byte("x"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "a.go"), []byte("x"), 0o644)
	got, _, err := BuildTree(root, TreeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	ai := strings.Index(got, "a.go")
	bi := strings.Index(got, "b.go")
	if ai < 0 || bi < 0 || ai > bi {
		t.Errorf("条目应按名称排序\n%s", got)
	}
	if !strings.Contains(got, "├── a.go") || !strings.Contains(got, "└── b.go") {
		t.Errorf("应为 tree 连接符样式\n%s", got)
	}
}
