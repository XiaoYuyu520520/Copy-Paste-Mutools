package render

import (
	"bufio"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTemp 写一个临时文件并返回路径。
func writeTemp(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestMarkdownEditorIndex(t *testing.T) {
	rec := httptest.NewRecorder()
	NewMarkdownEditorHandler(nil).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("index = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Markdown 渲染",
		"全屏渲染", // 默认视图：只显示渲染结果并占满整屏
		"左右对照", // 可选开关
		`id="src"`,
		`id="mdLight"`, `id="mdDark"`, `id="hlLight"`, `id="hlDark"`,
		"github-markdown-light.min.css", "github-markdown-dark.min.css",
		"github.min.css", "github-dark.min.css",
		"marked.min.js", "highlight.min.js",
		"applyTheme", "applyView", "matchMedia", // 手动主题 + 视图开关 + 首次跟随系统
		"dragenter", "handleFiles", // 拖入即渲染
		"EventSource", "applyDoc", // 原生拖入经 SSE 推送
	} {
		if !strings.Contains(body, want) {
			t.Errorf("页面缺少 %q", want)
		}
	}
	if strings.Contains(body, docToken) {
		t.Errorf("页面不应再含占位符 %s", docToken)
	}
}

func TestMarkdownEditorAssets(t *testing.T) {
	h := NewMarkdownEditorHandler(nil)
	for _, p := range []string{
		"/assets/marked.min.js",
		"/assets/highlight.min.js",
		"/assets/github.min.css",
		"/assets/github-dark.min.css",
		"/assets/github-markdown-light.min.css",
		"/assets/github-markdown-dark.min.css",
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s = %d, want 200", p, rec.Code)
		}
	}
}

// TestThemeAssetsAreExplicit 守住「手动主题」的前提：亮/暗两套 Markdown CSS 必须各自确定，
// 不能含 prefers-color-scheme 媒体查询，否则系统配色会盖掉用户选择，
// 出现「深色正文配亮色代码高亮」而看不清代码的老问题。
func TestThemeAssetsAreExplicit(t *testing.T) {
	for _, name := range []string{"github-markdown-light.min.css", "github-markdown-dark.min.css"} {
		data, err := fs.ReadFile(Assets(), name)
		if err != nil {
			t.Fatalf("读取 %s 失败: %v", name, err)
		}
		if strings.Contains(string(data), "prefers-color-scheme") {
			t.Errorf("%s 含 prefers-color-scheme，会盖掉手动主题", name)
		}
		if !strings.Contains(string(data), ".markdown-body") {
			t.Errorf("%s 不是 markdown-body 样式", name)
		}
	}
	for _, name := range []string{"github.min.css", "github-dark.min.css"} {
		data, err := fs.ReadFile(Assets(), name)
		if err != nil {
			t.Fatalf("读取 %s 失败: %v", name, err)
		}
		if !strings.Contains(string(data), ".hljs") {
			t.Errorf("%s 不是 highlight.js 主题", name)
		}
	}
}

func TestMarkdownEditorInlinesDroppedDoc(t *testing.T) {
	docs := NewDocStore()
	h := NewMarkdownEditorHandler(docs)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(rec.Body.String(), `"kind":"empty"`) {
		t.Error("空文档源应在首屏内嵌 kind=empty")
	}

	dir := t.TempDir()
	md := writeTemp(t, dir, "README.md", "# 拖入即渲染\n\n```go\nfunc f() {}\n```\n")
	if _, err := docs.LoadPaths([]string{md}); err != nil {
		t.Fatal(err)
	}

	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/", nil))
	body := rec2.Body.String()
	if !strings.Contains(body, `"kind":"markdown"`) || !strings.Contains(body, "README.md") {
		t.Error("首屏应内嵌拖入的 Markdown 文档")
	}
	if !strings.Contains(body, "拖入即渲染") {
		t.Error("首屏应带 Markdown 正文")
	}

	rec3 := httptest.NewRecorder()
	h.ServeHTTP(rec3, httptest.NewRequest(http.MethodGet, "/api/doc", nil))
	if ct := rec3.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("/api/doc Content-Type = %q", ct)
	}
	if !strings.Contains(rec3.Body.String(), `"name":"README.md"`) {
		t.Error("/api/doc 应返回当前文档")
	}
}

// 首屏把文档内嵌进 <script>，正文里的 </script> 必须被转义，否则会截断页面。
func TestMarkdownEditorInliningEscapesScriptTags(t *testing.T) {
	docs := NewDocStore()
	dir := t.TempDir()
	md := writeTemp(t, dir, "evil.md", "正常文字\n\n</script><script>alert(1)</script>\n")
	if _, err := docs.LoadPaths([]string{md}); err != nil {
		t.Fatal(err)
	}

	page, err := markdownEditorHTML(docs.Current())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(page, "<script>alert(1)</script>") {
		t.Error("正文里的 </script> 未转义，会破坏页面结构")
	}
	if !strings.Contains(page, `\u003c/script\u003e`) {
		t.Error("应当以 u003c 转义形式内嵌")
	}
}

func TestMarkdownEditorDocMethodNotAllowed(t *testing.T) {
	rec := httptest.NewRecorder()
	NewMarkdownEditorHandler(nil).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/doc", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /api/doc = %d, want 405", rec.Code)
	}
}

// readSSE 从事件流里读出下一条 data。
func readSSE(t *testing.T, br *bufio.Reader) string {
	t.Helper()
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			t.Fatalf("读事件流失败: %v", err)
		}
		if strings.HasPrefix(line, "data: ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "data: "))
		}
	}
}

func TestMarkdownEditorEventStream(t *testing.T) {
	docs := NewDocStore()
	srv := httptest.NewServer(NewMarkdownEditorHandler(docs))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("Content-Type = %q", ct)
	}

	br := bufio.NewReader(resp.Body)
	if got := readSSE(t, br); !strings.Contains(got, `"kind":"empty"`) {
		t.Errorf("连接时应先补发当前快照，得到 %s", got)
	}

	dir := t.TempDir()
	md := writeTemp(t, dir, "note.md", "拖入推送测试")
	if _, err := docs.LoadPaths([]string{md}); err != nil {
		t.Fatal(err)
	}
	if got := readSSE(t, br); !strings.Contains(got, "note.md") {
		t.Errorf("文档变更应推送 note.md，得到 %s", got)
	}
}

func TestMarkdownEditorMapPage(t *testing.T) {
	docs := NewDocStore()
	srv := httptest.NewServer(NewMarkdownEditorHandler(docs))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/map/current")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("未载入地图时 = %d, want 404", resp.StatusCode)
	}

	dir := t.TempDir()
	head := []byte("P5\n4 4\n255\n")
	pgm := filepath.Join(dir, "map.pgm")
	if err := os.WriteFile(pgm, append(head, make([]byte, 16)...), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := docs.LoadPaths([]string{pgm}); err != nil {
		t.Fatal(err)
	}

	resp2, err := http.Get(srv.URL + "/api/map/current")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("载入地图后 = %d, want 200", resp2.StatusCode)
	}
	page, _ := io.ReadAll(resp2.Body)
	if !strings.Contains(string(page), "SLAM 地图") {
		t.Error("地图页内容不对")
	}
	if strings.Contains(string(page), mapPageToken) {
		t.Error("地图页不应残留占位符")
	}
}

func TestMapUploadRejectsMissingPGM(t *testing.T) {
	rec := httptest.NewRecorder()
	NewMarkdownEditorHandler(nil).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/map", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("缺失 pgm = %d, want 400", rec.Code)
	}
}
