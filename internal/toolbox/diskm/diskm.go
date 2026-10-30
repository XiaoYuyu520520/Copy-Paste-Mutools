// Package diskm 实现磁盘 I/O 实时监控。
//
// 迁移自 legacy/python/.../磁盘工具/disk_monitor.py：原脚本靠解析
// `iostat -x -m 1 <dev>` 的输出。这里改成直接读系统计数器：
//   - Windows：GetDiskPerformanceCounter（性能计数器）
//   - Linux：/proc/diskstats
//
// 因此不依赖 sysstat，也不需要额外进程。
package diskm

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Sample 是一次磁盘 I/O 采样。
type Sample struct {
	Device     string  `json:"device"`
	ReadMBs    float64 `json:"read_mb_per_s"`
	WriteMBs   float64 `json:"write_mb_per_s"`
	UtilPct    float64 `json:"util_percent"`
	ReadBytes  uint64  `json:"read_bytes"`
	WriteBytes uint64  `json:"write_bytes"`
}

// Options 是监控配置。
type Options struct {
	// Device 是要监控的设备名；为空表示监控全部物理设备。
	Device string
	// Interval 是采样间隔。
	Interval time.Duration
	// Count 是采样次数；0 表示一直监控直到 Ctrl+C。
	Count int
}

// DefaultOptions 返回默认配置。
func DefaultOptions() Options {
	return Options{Interval: time.Second}
}

// Monitor 按固定间隔采集磁盘 I/O 采样并回调。
//
// onSample 在采集 goroutine 里被调用，实现需要自行保证线程安全。
func Monitor(ctx interface{ Done() <-chan struct{} }, opts Options, onSample func([]Sample)) error {
	if opts.Interval <= 0 {
		opts.Interval = time.Second
	}
	prev := collect()

	ticker := time.NewTicker(opts.Interval)
	defer ticker.Stop()

	for i := 0; opts.Count <= 0 || i < opts.Count; i++ {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}

		curr := collect()
		samples := diff(prev, curr, opts.Device, opts.Interval)
		prev = curr
		if len(samples) > 0 && onSample != nil {
			onSample(samples)
		}
	}
	return nil
}

// diff 根据两次累计计数之差算出速率。
//
// 注意：调用方需要保证两次采样之间的间隔已知，这里用固定的 Interval 估算。
func diff(prev, curr map[string]Sample, filter string, elapsed time.Duration) []Sample {
	out := []Sample{}
	keys := make([]string, 0, len(curr))
	for k := range curr {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	seconds := elapsed.Seconds()
	if seconds <= 0 {
		seconds = 1
	}

	for _, k := range keys {
		if filter != "" && !strings.EqualFold(filter, k) {
			continue
		}
		c := curr[k]
		p, ok := prev[k]
		if !ok {
			continue
		}
		readDelta := c.ReadBytes - p.ReadBytes
		writeDelta := c.WriteBytes - p.WriteBytes
		out = append(out, Sample{
			Device:     c.Device,
			ReadBytes:  readDelta,
			WriteBytes: writeDelta,
			ReadMBs:    float64(readDelta) / seconds / (1024 * 1024),
			WriteMBs:   float64(writeDelta) / seconds / (1024 * 1024),
			UtilPct:    c.UtilPct,
		})
	}
	return out
}

// FormatSamples 把采样渲染成文本表格。
func FormatSamples(samples []Sample) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%-14s %-12s %-12s %-12s\n", "设备", "读MB/s", "写MB/s", "繁忙率%"))
	b.WriteString(strings.Repeat("-", 52) + "\n")
	for _, s := range samples {
		b.WriteString(fmt.Sprintf("%-14s %-12.2f %-12.2f %-12.1f\n",
			s.Device, s.ReadMBs, s.WriteMBs, s.UtilPct))
	}
	return b.String()
}
