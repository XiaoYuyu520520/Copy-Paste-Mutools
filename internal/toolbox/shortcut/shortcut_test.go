package shortcut

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeName(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"正常名字", "正常名字"},
		{`a/b\c:d*e?f"g<h>i|j`, "abcdefghij"},
		{"", ""},
	}
	for _, c := range cases {
		if got := SafeName(c.in); got != c.want {
			t.Errorf("SafeName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestValidateRejectsBadInput(t *testing.T) {
	dir := t.TempDir()

	cases := []struct {
		name string
		opts Options
	}{
		{"空名称", Options{URL: "https://a.com", Dir: dir}},
		{"空网址", Options{Name: "x", Dir: dir}},
		{"非 http 网址", Options{Name: "x", URL: "ftp://a.com", Dir: dir}},
		{"空目录", Options{Name: "x", URL: "https://a.com"}},
		{"目录不存在", Options{Name: "x", URL: "https://a.com", Dir: filepath.Join(dir, "nope")}},
	}
	for _, c := range cases {
		if err := c.opts.Validate(); err == nil {
			t.Errorf("%s: Validate() = nil, want error", c.name)
		}
	}
}

func TestCreateBat(t *testing.T) {
	dir := t.TempDir()
	res, err := Create(Options{Name: "测试", URL: "https://github.com", Dir: dir, Format: FormatBat})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !strings.HasSuffix(res.Path, ".bat") {
		t.Errorf("path = %q, want .bat suffix", res.Path)
	}
	body, err := os.ReadFile(res.Path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	s := string(body)
	if !strings.Contains(s, `start "" "https://github.com"`) {
		t.Errorf("content = %q, want start command", s)
	}
	if !strings.HasPrefix(s, "@echo off") {
		t.Errorf("content = %q, want @echo off prefix", s)
	}
}

func TestCreateDesktop(t *testing.T) {
	dir := t.TempDir()
	res, err := Create(Options{Name: "GH", URL: "https://github.com", Dir: dir, Format: FormatDesktop})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !strings.HasSuffix(res.Path, ".desktop") {
		t.Errorf("path = %q, want .desktop suffix", res.Path)
	}
	body, _ := os.ReadFile(res.Path)
	if !strings.Contains(string(body), "URL=https://github.com") {
		t.Errorf("content = %q, want URL entry", string(body))
	}
}

func TestCreateOverwriteReportsFlag(t *testing.T) {
	dir := t.TempDir()
	opts := Options{Name: "X", URL: "https://a.com", Dir: dir, Format: FormatURL}

	res, err := Create(opts)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if res.Overwrote {
		t.Errorf("Overwrote = true on first create, want false")
	}

	res2, err := Create(opts)
	if err != nil {
		t.Fatalf("Create (2nd): %v", err)
	}
	if !res2.Overwrote {
		t.Errorf("Overwrote = false on second create, want true")
	}
}
