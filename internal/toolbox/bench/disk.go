package bench

import (
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// DiskOptions 控制磁盘 I/O 基准测试。
type DiskOptions struct {
	// FilePath 是测试文件路径；为空时使用临时目录下的临时文件。
	FilePath string
	// SizeBytes 是测试文件大小。
	SizeBytes int64
	// BlockBytes 是读写块大小。
	BlockBytes int64
	// KeepFile 为 true 时保留测试文件（用于排查）。
	KeepFile bool
	// DropCaches 为 true 时尝试清除系统页缓存（仅 Linux，需 root）。
	DropCaches bool
}

// DefaultDiskOptions 返回默认磁盘测试参数：1 GiB 文件、64 MiB 块。
func DefaultDiskOptions() DiskOptions {
	return DiskOptions{
		SizeBytes:  1 << 30,
		BlockBytes: 64 << 20,
		DropCaches: true,
	}
}

// DiskResult 是磁盘测试结果。
type DiskResult struct {
	Path         string
	SizeBytes    int64
	WriteTime    time.Duration
	WriteSpeed   float64
	ReadTime     time.Duration
	ReadSpeed    float64
	CacheDropped bool
	CacheNote    string
}

// RunDisk 执行顺序写 + fsync + 顺序读测试。
func RunDisk(opts DiskOptions) (DiskResult, error) {
	if opts.SizeBytes <= 0 {
		opts.SizeBytes = DefaultDiskOptions().SizeBytes
	}
	if opts.BlockBytes <= 0 {
		opts.BlockBytes = DefaultDiskOptions().BlockBytes
	}

	path := opts.FilePath
	if path == "" {
		f, err := os.CreateTemp("", "toolbox-diskbench-*.tmp")
		if err != nil {
			return DiskResult{}, err
		}
		path = f.Name()
		f.Close()
		opts.KeepFile = false
	}

	result := DiskResult{Path: path, SizeBytes: opts.SizeBytes}

	if !opts.KeepFile {
		defer os.Remove(path)
	}

	block := make([]byte, opts.BlockBytes)
	if _, err := rand.Read(block); err != nil {
		return result, fmt.Errorf("生成随机数据失败: %w", err)
	}

	// ---- 写入 ----
	f, err := os.Create(path)
	if err != nil {
		return result, fmt.Errorf("创建测试文件失败: %w", err)
	}
	writeStart := time.Now()
	remaining := opts.SizeBytes
	for remaining > 0 {
		n := int64(len(block))
		if remaining < n {
			n = remaining
		}
		written, werr := f.Write(block[:n])
		if werr != nil {
			f.Close()
			return result, fmt.Errorf("写入失败: %w", werr)
		}
		remaining -= int64(written)
	}
	// 关键：把缓冲区刷到物理磁盘，否则测的是内存速度。
	if err := f.Sync(); err != nil {
		f.Close()
		return result, fmt.Errorf("刷盘失败: %w", err)
	}
	f.Close()
	result.WriteTime = time.Since(writeStart)
	result.WriteSpeed = bytesPerSecond(opts.SizeBytes, result.WriteTime)

	// ---- 清缓存 ----
	if opts.DropCaches {
		result.CacheDropped, result.CacheNote = dropCaches()
	}

	// ---- 读取 ----
	rf, err := os.Open(path)
	if err != nil {
		return result, fmt.Errorf("打开测试文件失败: %w", err)
	}
	defer rf.Close()
	readStart := time.Now()
	buf := make([]byte, opts.BlockBytes)
	var total int64
	for {
		n, rerr := rf.Read(buf)
		total += int64(n)
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return result, fmt.Errorf("读取失败: %w", rerr)
		}
	}
	result.ReadTime = time.Since(readStart)
	result.ReadSpeed = bytesPerSecond(opts.SizeBytes, result.ReadTime)

	return result, nil
}

// bytesPerSecond 计算吞吐量（字节/秒）。
func bytesPerSecond(size int64, d time.Duration) float64 {
	if d <= 0 {
		return 0
	}
	return float64(size) / d.Seconds()
}

// dropCaches 尝试清除 Linux 页缓存，返回是否成功以及给用户的说明。
func dropCaches() (bool, string) {
	if runtime.GOOS != "linux" {
		return false, "非 Linux 系统不支持清缓存，读取速度可能包含页缓存影响"
	}
	// 先同步，再尝试直接写（需要 root）。
	_ = exec.Command("sync").Run()
	if err := os.WriteFile("/proc/sys/vm/drop_caches", []byte("3\n"), 0644); err == nil {
		return true, "缓存清除成功"
	}
	// 退回到 sudo（会提示用户输入密码）。
	cmd := exec.Command("sudo", "sh", "-c", "echo 3 > /proc/sys/vm/drop_caches")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err == nil {
		return true, "缓存清除成功（通过 sudo）"
	}
	return false, "缓存清除失败，读取速度可能虚高（建议以 root 运行）"
}

// FormatBytes 把字节数格式化成人类可读形式，例如 "1.50 GB"。
func FormatBytes(b float64) string {
	if b <= 0 {
		return "0 B"
	}
	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	idx := 0
	value := b
	for value >= 1024 && idx < len(units)-1 {
		value /= 1024
		idx++
	}
	// 字节级别不带小数位，其余保留两位。
	if idx == 0 {
		return fmt.Sprintf("%.0f %s", value, units[idx])
	}
	return fmt.Sprintf("%.2f %s", value, units[idx])
}

// FormatDiskResult 把磁盘测试结果渲染成报告。
func FormatDiskResult(r DiskResult) string {
	var b strings.Builder
	b.WriteString("=== 磁盘 I/O 性能测试 ===\n")
	b.WriteString(fmt.Sprintf("文件路径: %s\n", r.Path))
	b.WriteString(fmt.Sprintf("测试大小: %s\n", FormatBytes(float64(r.SizeBytes))))
	b.WriteString("---------------------------------\n")
	b.WriteString("写入测试\n")
	b.WriteString(fmt.Sprintf("  耗时: %.2f 秒\n", r.WriteTime.Seconds()))
	b.WriteString(fmt.Sprintf("  速度: %s/s\n", FormatBytes(r.WriteSpeed)))
	b.WriteString("---------------------------------\n")
	b.WriteString(fmt.Sprintf("缓存: %s\n", r.CacheNote))
	b.WriteString("读取测试\n")
	b.WriteString(fmt.Sprintf("  耗时: %.2f 秒\n", r.ReadTime.Seconds()))
	b.WriteString(fmt.Sprintf("  速度: %s/s\n", FormatBytes(r.ReadSpeed)))
	b.WriteString("=================================\n")
	return b.String()
}
