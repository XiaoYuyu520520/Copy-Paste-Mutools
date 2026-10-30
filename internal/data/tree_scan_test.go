package data

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTreeScanner_ResumesWithoutRestart(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 25; i++ {
		name := filepath.Join(root, "file-"+strings.Repeat("x", i%3)+string(rune('a'+i%26))+".txt")
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	scanner, err := NewTreeScanner(root, TreeOptions{})
	if err != nil {
		t.Fatal(err)
	}

	var out strings.Builder
	batches := 0
	for {
		result, err := scanner.Next(4)
		if err != nil {
			t.Fatal(err)
		}
		batches++
		out.WriteString(result.Text)
		if !result.Done && !result.Truncated {
			t.Fatal("a paused scan must be marked truncated")
		}
		if result.Done {
			if result.Truncated {
				t.Fatal("a completed scan must not be marked truncated")
			}
			break
		}
		if batches > 50 {
			t.Fatal("scanner did not make progress")
		}
	}

	want, wantStat, err := BuildTree(root, TreeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != want {
		t.Fatalf("resumed output differs from full scan\nGOT:\n%s\nWANT:\n%s", got, want)
	}
	if batches < 2 {
		t.Fatalf("expected multiple bounded batches, got %d", batches)
	}
	if got := scanner.stat.Total(); got != wantStat.Total() {
		t.Fatalf("scanner total=%d, BuildTree total=%d", got, wantStat.Total())
	}
}
