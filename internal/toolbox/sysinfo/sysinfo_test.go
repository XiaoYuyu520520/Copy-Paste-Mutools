package sysinfo

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestQueryUnknownTarget(t *testing.T) {
	if _, err := Query("nope"); err == nil {
		t.Fatal("Query(\"nope\") = nil error, want error")
	}
}

func TestQueryKnownTargets(t *testing.T) {
	for _, target := range []string{"system", "cpu", "gpu", "memory", "disk", "all", ""} {
		v, err := Query(target)
		if err != nil {
			t.Fatalf("Query(%q): %v", target, err)
		}
		if v == nil {
			t.Fatalf("Query(%q) = nil", target)
		}
	}
}

func TestCollectSystemHasFields(t *testing.T) {
	s := CollectSystem()
	if s.OS == "" {
		t.Error("OS is empty")
	}
	if s.Architecture == "" {
		t.Error("Architecture is empty")
	}
}

func TestCollectCPUHasCores(t *testing.T) {
	c := CollectCPU()
	if c.LogicalCores <= 0 {
		t.Errorf("LogicalCores = %d, want > 0", c.LogicalCores)
	}
}

func TestFormatJSON(t *testing.T) {
	text, err := FormatJSON(CollectCPU())
	if err != nil {
		t.Fatalf("FormatJSON: %v", err)
	}
	var probe map[string]any
	if err := json.Unmarshal([]byte(text), &probe); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if _, ok := probe["logical_cores"]; !ok {
		t.Errorf("JSON missing logical_cores: %s", text)
	}
}

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		in   uint64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.00 KB"},
		{1024 * 1024, "1.00 MB"},
		{1024 * 1024 * 1024, "1.00 GB"},
	}
	for _, c := range cases {
		if got := FormatBytes(c.in); got != c.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormatHumanIncludesSections(t *testing.T) {
	out := FormatHuman(CollectAll())
	for _, want := range []string{"系统信息", "CPU 信息", "内存信息", "磁盘信息"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing section %q", want)
		}
	}
}

func TestCollectDiskReturnsSlice(t *testing.T) {
	if got := CollectDisk(); got == nil {
		t.Error("CollectDisk() = nil, want non-nil slice")
	}
}
