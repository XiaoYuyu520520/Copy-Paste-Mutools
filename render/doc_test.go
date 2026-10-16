package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDocStoreLoadsMarkdown(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "readme.md")
	// 带 UTF-8 BOM，验证读取时被剥掉
	body := append([]byte{0xEF, 0xBB, 0xBF}, []byte("# 标题\n")...)
	if err := os.WriteFile(p, body, 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewDocStore()
	events, cancel := s.Subscribe()
	defer cancel()

	doc, err := s.LoadPaths([]string{p})
	if err != nil {
		t.Fatal(err)
	}
	if doc.Kind != KindMarkdown || doc.Name != "readme.md" || doc.Version != 1 {
		t.Fatalf("文档快照不对: %+v", doc)
	}
	if !strings.HasPrefix(doc.Content, "# 标题") {
		t.Errorf("BOM 应被剥掉，得到 %q", doc.Content)
	}
	if got := s.Current(); got.Version != doc.Version || got.Name != doc.Name {
		t.Errorf("Current 与返回值不一致: %+v", got)
	}

	select {
	case ev := <-events:
		if ev.Version != doc.Version || ev.Kind != KindMarkdown {
			t.Errorf("订阅事件不对: %+v", ev)
		}
	default:
		t.Error("订阅者应收到文档变更")
	}
}

func TestDocStorePairsMapFiles(t *testing.T) {
	dir := t.TempDir()
	pgm := filepath.Join(dir, "lab.pgm")
	head := []byte("P5\n4 4\n255\n")
	if err := os.WriteFile(pgm, append(head, make([]byte, 16)...), 0o644); err != nil {
		t.Fatal(err)
	}
	yaml := filepath.Join(dir, "lab.yaml")
	yamlBody := "image: lab.pgm\nresolution: 0.05\norigin: [0,0,0]\n"
	if err := os.WriteFile(yaml, []byte(yamlBody), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewDocStore()
	// 只拖 .pgm：应自动配上同目录同名 .yaml
	doc, err := s.LoadPaths([]string{pgm})
	if err != nil {
		t.Fatal(err)
	}
	if doc.Kind != KindMap {
		t.Fatalf("应为地图文档: %+v", doc)
	}
	if !strings.Contains(doc.Name, "lab.pgm") || !strings.Contains(doc.Name, "lab.yaml") {
		t.Errorf("应自动配对 .yaml，得到 %q", doc.Name)
	}
	page, ok := s.MapHTML()
	if !ok || !strings.Contains(page, "SLAM 地图") {
		t.Error("应缓存可内嵌的完整地图页")
	}

	// 只拖 .yaml：同样回配同名 .pgm，且版本递增
	doc2, err := s.LoadPaths([]string{yaml})
	if err != nil {
		t.Fatal(err)
	}
	if doc2.Kind != KindMap || doc2.Version != doc.Version+1 {
		t.Errorf("按 .yaml 载入失败: %+v", doc2)
	}
}

func TestDocStoreRejectsUnsupported(t *testing.T) {
	s := NewDocStore()
	doc, err := s.LoadPaths([]string{filepath.Join(t.TempDir(), "a.zip")})
	if err == nil {
		t.Fatal("不支持的类型应报错")
	}
	if doc.Kind != KindEmpty || doc.Error == "" {
		t.Errorf("失败时应广播带错误信息的空文档: %+v", doc)
	}
	if _, ok := s.MapHTML(); ok {
		t.Error("非地图文档不应残留地图页")
	}
	if s.Current().Kind != KindEmpty {
		t.Error("当前文档应回到空态")
	}
	if _, err := s.LoadPaths(nil); err == nil {
		t.Error("空输入应报错")
	}
}

// TestDocStoreKeepsLatestForSlowSubscriber 慢订阅者不应阻塞发布，且最终拿到最新文档。
func TestDocStoreKeepsLatestForSlowSubscriber(t *testing.T) {
	s := NewDocStore()
	events, cancel := s.Subscribe()
	defer cancel()

	dir := t.TempDir()
	for i, name := range []string{"a.md", "b.md", "c.md"} {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(strings.Repeat("# t\n", i+1)), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := s.LoadPaths([]string{p}); err != nil {
			t.Fatal(err)
		}
	}
	if got := <-events; got.Name != "c.md" {
		t.Errorf("慢订阅者应拿到最新文档，得到 %q", got.Name)
	}
}

// TestDocStoreUnsubscribeIsSafe 取消订阅可重复调用，且取消后不再收到推送。
func TestDocStoreUnsubscribeIsSafe(t *testing.T) {
	s := NewDocStore()
	events, cancel := s.Subscribe()
	cancel()
	cancel()

	p := writeTemp(t, t.TempDir(), "x.md", "# x")
	if _, err := s.LoadPaths([]string{p}); err != nil {
		t.Fatal(err)
	}
	if _, ok := <-events; ok {
		t.Error("取消订阅后通道应已关闭")
	}
}
