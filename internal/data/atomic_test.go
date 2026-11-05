package data

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveJSONAtomicCreatesFileAndBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.json")

	if err := SaveJSONAtomic(path, map[string]int{"n": 1}); err != nil {
		t.Fatalf("first save: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("target missing: %v", err)
	}

	if err := SaveJSONAtomic(path, map[string]int{"n": 2}); err != nil {
		t.Fatalf("second save: %v", err)
	}
	if _, err := os.Stat(path + bakSuffix); err != nil {
		t.Fatalf("backup missing after overwrite: %v", err)
	}

	// 不应残留临时文件
	if _, err := os.Stat(path + tmpSuffix); !os.IsNotExist(err) {
		t.Fatalf("temp file should be cleaned up: %v", err)
	}
}

func TestLoadJSONWithBackupFallsBackWhenCorrupt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.json")

	if err := SaveJSONAtomic(path, map[string]string{"v": "old"}); err != nil {
		t.Fatal(err)
	}
	if err := SaveJSONAtomic(path, map[string]string{"v": "new"}); err != nil {
		t.Fatal(err)
	}
	// 故意破坏主文件
	if err := os.WriteFile(path, []byte("{ broken"), 0o644); err != nil {
		t.Fatal(err)
	}

	var got map[string]string
	used, err := LoadJSONWithBackup(path, &got)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !used {
		t.Fatal("expected backup to be used")
	}
	if got["v"] != "old" {
		t.Fatalf("backup content = %q, want old", got["v"])
	}
}
