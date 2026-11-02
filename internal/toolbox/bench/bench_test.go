package bench

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScore(t *testing.T) {
	cases := []struct {
		duration float64
		baseline float64
		want     int
	}{
		{1.25, 1.25, 1000},
		{0.625, 1.25, 2000},
		{2.5, 1.25, 500},
		{0, 1.25, 0},
	}
	for _, c := range cases {
		if got := score(c.duration, c.baseline); got != c.want {
			t.Errorf("score(%v, %v) = %d, want %d", c.duration, c.baseline, got, c.want)
		}
	}
}

func TestBenchmarkIntIsPositive(t *testing.T) {
	d := benchmarkInt(2000)
	if d <= 0 {
		t.Fatalf("benchmarkInt returned %v, want > 0", d)
	}
}

func TestBenchmarkFloatIsPositive(t *testing.T) {
	d := benchmarkFloat(10000)
	if d <= 0 {
		t.Fatalf("benchmarkFloat returned %v, want > 0", d)
	}
}

func TestRunCPUWithSmallLimits(t *testing.T) {
	opts := DefaultCPUOptions()
	opts.IntLimit = 2000
	opts.FloatIters = 20000
	opts.Warmup = false

	res := RunCPU(opts)
	if res.IntScore <= 0 {
		t.Errorf("IntScore = %d, want > 0", res.IntScore)
	}
	if res.FloatScore <= 0 {
		t.Errorf("FloatScore = %d, want > 0", res.FloatScore)
	}
	if res.OverallScore <= 0 {
		t.Errorf("OverallScore = %d, want > 0", res.OverallScore)
	}
}

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		in   float64
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
			t.Errorf("FormatBytes(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRunDiskRoundTrip(t *testing.T) {
	opts := DefaultDiskOptions()
	opts.SizeBytes = 1 << 20 // 1 MiB
	opts.BlockBytes = 64 << 10
	opts.DropCaches = false

	res, err := RunDisk(opts)
	if err != nil {
		t.Fatalf("RunDisk: %v", err)
	}
	if res.WriteSpeed <= 0 {
		t.Errorf("WriteSpeed = %v, want > 0", res.WriteSpeed)
	}
	if res.ReadSpeed <= 0 {
		t.Errorf("ReadSpeed = %v, want > 0", res.ReadSpeed)
	}
	if res.WriteTime <= 0 || res.ReadTime <= 0 {
		t.Errorf("expected non-zero durations, got write=%v read=%v", res.WriteTime, res.ReadTime)
	}
}

func TestBytesPerSecond(t *testing.T) {
	got := bytesPerSecond(1024, time.Second)
	if got != 1024 {
		t.Errorf("bytesPerSecond = %v, want 1024", got)
	}
	if got := bytesPerSecond(1024, 0); got != 0 {
		t.Errorf("bytesPerSecond with zero duration = %v, want 0", got)
	}
}

func TestRunDiskRefusesExistingFileWithoutOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.bin")
	original := []byte("珍贵数据，不能被截断")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}

	opts := DefaultDiskOptions()
	opts.FilePath = path
	opts.SizeBytes = 1 << 20
	opts.BlockBytes = 64 << 10

	_, err := RunDisk(opts)
	if !errors.Is(err, ErrFileExists) {
		t.Fatalf("RunDisk err = %v, want ErrFileExists", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatalf("原文件被修改: %q", got)
	}
}

func TestRunDiskOverwriteExplicit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.bin")
	if err := os.WriteFile(path, []byte("旧内容"), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := DefaultDiskOptions()
	opts.FilePath = path
	opts.Overwrite = true
	opts.KeepFile = true
	opts.SizeBytes = 1 << 20
	opts.BlockBytes = 64 << 10

	res, err := RunDisk(opts)
	if err != nil {
		t.Fatalf("RunDisk: %v", err)
	}
	if res.WriteSpeed <= 0 || res.ReadSpeed <= 0 {
		t.Fatalf("speed should be positive: %+v", res)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != opts.SizeBytes {
		t.Fatalf("size = %d, want %d", info.Size(), opts.SizeBytes)
	}
}

func TestDefaultDiskOptionsDoesNotDropCaches(t *testing.T) {
	if DefaultDiskOptions().DropCaches {
		t.Fatal("默认不应清除系统页缓存（会影响同机其他程序）")
	}
}
