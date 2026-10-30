package bench

import (
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
