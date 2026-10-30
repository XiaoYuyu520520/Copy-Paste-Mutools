package share

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "b.txt"), []byte("world"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	s, err := New(Options{SharedDir: dir, Addr: "127.0.0.1:0"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s, dir
}

func TestNewRejectsMissingDir(t *testing.T) {
	if _, err := New(Options{SharedDir: filepath.Join(t.TempDir(), "nope")}); err == nil {
		t.Fatal("New() = nil error, want error for missing directory")
	}
}

func TestListFiles(t *testing.T) {
	s, _ := newTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/files", nil)
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var entries []Entry
	if err := json.Unmarshal(rec.Body.Bytes(), &entries); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	// 目录应排在前面。
	if !entries[0].IsDirectory {
		t.Errorf("first entry = %q, want a directory", entries[0].Name)
	}
}

func TestSafeResolveBlocksTraversal(t *testing.T) {
	s, root := newTestServer(t)

	if _, err := s.safeResolve(".."); err == nil {
		t.Error("safeResolve(\"..\") = nil error, want traversal rejected")
	}
	if _, err := s.safeResolve("../.."); err == nil {
		t.Error("safeResolve(\"../..\") = nil error, want traversal rejected")
	}
	got, err := s.safeResolve("sub")
	if err != nil {
		t.Fatalf("safeResolve(\"sub\"): %v", err)
	}
	if got != filepath.Join(root, "sub") {
		t.Errorf("safeResolve(\"sub\") = %q", got)
	}
}

func TestBatchDownload(t *testing.T) {
	s, _ := newTestServer(t)

	body := strings.NewReader(`{"paths":["a.txt","sub"]}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/download/batch", body)
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/zip" {
		t.Errorf("Content-Type = %q, want application/zip", ct)
	}
	// ZIP 魔数
	data := rec.Body.Bytes()
	if len(data) < 4 || string(data[:2]) != "PK" {
		t.Errorf("response is not a zip: %q", data[:min(8, len(data))])
	}
}

func TestBatchDownloadRejectsEmptyPaths(t *testing.T) {
	s, _ := newTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/download/batch", strings.NewReader(`{"paths":[]}`))
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHealth(t *testing.T) {
	s, _ := newTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	s.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	b, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(b), `"status":"ok"`) {
		t.Errorf("body = %q", string(b))
	}
}

// min 是 Go 1.21+ 的内建函数，这里保留一个兜底以兼容旧工具链。
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
