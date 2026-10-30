package lanscan

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestPrefsRoundTripPersistsToFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prefs.json")

	s := newTestServer()
	s.SetPrefsPath(path)

	body := `{"use_tcp":false,"lookup_mac":false,"lookup_host":false,"ssh_mode":true,"workers":"64","timeout":"500","ports":"22,80"}`
	req := httptest.NewRequest(http.MethodPost, "/api/prefs", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST prefs = %d, want 200", rec.Code)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("prefs 文件未写入: %v", err)
	}

	// 重新构造（模拟程序重启、端口变化），应能从文件恢复
	s2 := newTestServer()
	s2.SetPrefsPath(path)
	rec2 := doGet(t, s2.Handler(), "/api/prefs")
	if rec2.Code != http.StatusOK {
		t.Fatalf("GET prefs = %d, want 200", rec2.Code)
	}
	var m map[string]any
	if err := json.Unmarshal(rec2.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m["ssh_mode"] != true || m["use_tcp"] != false || m["workers"] != "64" {
		t.Fatalf("prefs 恢复异常: %v", m)
	}
}
