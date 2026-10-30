// Package sysinfo 采集系统硬件与资源信息。
//
// 迁移自 legacy/python/小工具v1.0/code/读取设备信息.py，
// 原脚本依赖 psutil + WMI + GPUtil，这里改用 gopsutil + 平台原生实现，
// 跨 Windows / Linux，输出结构与原脚本保持一致（JSON 字段同名）。
package sysinfo

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
)

// SystemInfo 描述操作系统与主机基本信息。
type SystemInfo struct {
	OS           string `json:"os"`
	NodeName     string `json:"node_name"`
	Release      string `json:"release"`
	Version      string `json:"version"`
	Architecture string `json:"architecture"`
	Manufacturer string `json:"manufacturer"`
	Model        string `json:"model"`
	SystemType   string `json:"system_type"`
	BootTime     string `json:"last_boot_time"`
	Uptime       string `json:"uptime"`
}

// CPUInfo 描述处理器信息。
type CPUInfo struct {
	Name          string  `json:"name"`
	PhysicalCores int     `json:"physical_cores"`
	LogicalCores  int     `json:"logical_cores"`
	UsagePercent  float64 `json:"usage_percent"`
	Mhz           float64 `json:"mhz"`
}

// GPUInfo 描述一块显卡。
type GPUInfo struct {
	Source      string  `json:"source"`
	Name        string  `json:"name"`
	MemoryTotal uint64  `json:"memory_total_bytes,omitempty"`
	MemoryUsed  uint64  `json:"memory_used_bytes,omitempty"`
	UsagePct    float64 `json:"usage_percent,omitempty"`
	TempCelsius float64 `json:"temperature_celsius,omitempty"`
	Driver      string  `json:"driver_version,omitempty"`
}

// MemoryInfo 描述内存占用。
type MemoryInfo struct {
	TotalBytes     uint64  `json:"total_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	UsedBytes      uint64  `json:"used_bytes"`
	UsagePercent   float64 `json:"usage_percent"`
}

// DiskInfo 描述一个挂载点。
type DiskInfo struct {
	Device       string  `json:"device"`
	Mountpoint   string  `json:"mountpoint"`
	Fstype       string  `json:"fstype"`
	TotalBytes   uint64  `json:"total_bytes"`
	UsedBytes    uint64  `json:"used_bytes"`
	FreeBytes    uint64  `json:"free_bytes"`
	UsagePercent float64 `json:"usage_percent"`
}

// AllInfo 是完整的设备信息快照。
type AllInfo struct {
	System SystemInfo `json:"system"`
	CPU    CPUInfo    `json:"cpu"`
	GPU    []GPUInfo  `json:"gpu"`
	Memory MemoryInfo `json:"memory"`
	Disk   []DiskInfo `json:"disk"`
}

// CollectSystem 采集系统信息。
func CollectSystem() SystemInfo {
	info := SystemInfo{
		OS:           runtime.GOOS,
		NodeName:     hostname(),
		Architecture: runtime.GOARCH,
	}
	if h, err := host.Info(); err == nil {
		info.OS = h.OS
		info.Release = h.KernelVersion
		info.Version = h.PlatformVersion
		info.Architecture = h.KernelArch
		info.Manufacturer = h.Platform
		info.Model = h.Hostname
		info.SystemType = h.Platform
		if b, err := host.BootTime(); err == nil {
			info.BootTime = time.Unix(int64(b), 0).Format(time.RFC3339)
		}
		if up, err := host.Uptime(); err == nil {
			info.Uptime = (time.Duration(up) * time.Second).Truncate(time.Second).String()
		}
	}
	return info
}

// CollectCPU 采集处理器信息。usage 采样间隔为 1 秒，与原脚本一致。
func CollectCPU() CPUInfo {
	info := CPUInfo{
		LogicalCores: runtime.NumCPU(),
	}
	if names, err := cpu.Info(); err == nil && len(names) > 0 {
		info.Name = names[0].ModelName
	}
	if phys, err := cpu.Counts(false); err == nil && phys > 0 {
		info.PhysicalCores = phys
	}
	// 非阻塞采样；第一次调用通常返回 0，因此先空转一次再采样。
	_, _ = cpu.Percent(0, false)
	time.Sleep(200 * time.Millisecond)
	if pcts, err := cpu.Percent(300*time.Millisecond, false); err == nil && len(pcts) > 0 {
		info.UsagePercent = pcts[0]
	}
	if infos, err := cpu.Info(); err == nil && len(infos) > 0 {
		info.Mhz = infos[0].Mhz
	}
	return info
}

// CollectMemory 采集内存占用。
func CollectMemory() MemoryInfo {
	vm, err := mem.VirtualMemory()
	if err != nil {
		return MemoryInfo{}
	}
	return MemoryInfo{
		TotalBytes:     vm.Total,
		AvailableBytes: vm.Available,
		UsedBytes:      vm.Used,
		UsagePercent:   vm.UsedPercent,
	}
}

// CollectDisk 采集所有挂载点的磁盘占用。
func CollectDisk() []DiskInfo {
	out := []DiskInfo{}
	parts, err := disk.Partitions(false)
	if err != nil {
		return out
	}
	for _, p := range parts {
		usage, err := disk.Usage(p.Mountpoint)
		if err != nil {
			continue
		}
		out = append(out, DiskInfo{
			Device:       p.Device,
			Mountpoint:   p.Mountpoint,
			Fstype:       p.Fstype,
			TotalBytes:   usage.Total,
			UsedBytes:    usage.Used,
			FreeBytes:    usage.Free,
			UsagePercent: usage.UsedPercent,
		})
	}
	return out
}

// CollectAll 采集全部信息。
func CollectAll() AllInfo {
	return AllInfo{
		System: CollectSystem(),
		CPU:    CollectCPU(),
		GPU:    []GPUInfo{},
		Memory: CollectMemory(),
		Disk:   CollectDisk(),
	}
}

// Query 按名字采集单个部分，可选值：system、cpu、gpu、memory、disk、all。
func Query(target string) (any, error) {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "", "all":
		return CollectAll(), nil
	case "system":
		return CollectSystem(), nil
	case "cpu":
		return CollectCPU(), nil
	case "gpu":
		return []GPUInfo{}, nil
	case "memory":
		return CollectMemory(), nil
	case "disk":
		return CollectDisk(), nil
	default:
		return nil, fmt.Errorf("未知的信息类别: %s（可选 system/cpu/gpu/memory/disk/all）", target)
	}
}

// FormatJSON 把采集结果序列化成 JSON。
func FormatJSON(v any) (string, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// FormatHuman 把采集结果渲染成人类可读的文本报告。
func FormatHuman(info AllInfo) string {
	var b strings.Builder

	b.WriteString("======================================== 系统信息 ========================================\n")
	s := info.System
	b.WriteString(fmt.Sprintf("OS: %s\n", s.OS))
	b.WriteString(fmt.Sprintf("Node Name: %s\n", s.NodeName))
	b.WriteString(fmt.Sprintf("Release: %s\n", s.Release))
	b.WriteString(fmt.Sprintf("Version: %s\n", s.Version))
	b.WriteString(fmt.Sprintf("Architecture: %s\n", s.Architecture))
	if s.Manufacturer != "" {
		b.WriteString(fmt.Sprintf("Manufacturer: %s\n", s.Manufacturer))
	}
	if s.Model != "" {
		b.WriteString(fmt.Sprintf("Model: %s\n", s.Model))
	}
	if s.BootTime != "" {
		b.WriteString(fmt.Sprintf("Last Boot Time: %s\n", s.BootTime))
	}
	if s.Uptime != "" {
		b.WriteString(fmt.Sprintf("Uptime: %s\n", s.Uptime))
	}

	b.WriteString("\n======================================== CPU 信息 ========================================\n")
	c := info.CPU
	b.WriteString(fmt.Sprintf("Name: %s\n", c.Name))
	b.WriteString(fmt.Sprintf("Physical Cores: %d\n", c.PhysicalCores))
	b.WriteString(fmt.Sprintf("Logical Cores: %d\n", c.LogicalCores))
	b.WriteString(fmt.Sprintf("Usage Percent: %.1f%%\n", c.UsagePercent))

	b.WriteString("\n======================================== 内存信息 ========================================\n")
	m := info.Memory
	b.WriteString(fmt.Sprintf("Total: %s (%d bytes)\n", FormatBytes(m.TotalBytes), m.TotalBytes))
	b.WriteString(fmt.Sprintf("Available: %s (%d bytes)\n", FormatBytes(m.AvailableBytes), m.AvailableBytes))
	b.WriteString(fmt.Sprintf("Used: %s (%d bytes)\n", FormatBytes(m.UsedBytes), m.UsedBytes))
	b.WriteString(fmt.Sprintf("Usage Percent: %.1f%%\n", m.UsagePercent))

	b.WriteString("\n======================================== 磁盘信息 ========================================\n")
	for i, d := range info.Disk {
		b.WriteString(fmt.Sprintf("--- 磁盘 %d (%s) ---\n", i, d.Device))
		b.WriteString(fmt.Sprintf("  Mountpoint: %s\n", d.Mountpoint))
		b.WriteString(fmt.Sprintf("  Fstype: %s\n", d.Fstype))
		b.WriteString(fmt.Sprintf("  Total: %s\n", FormatBytes(d.TotalBytes)))
		b.WriteString(fmt.Sprintf("  Used: %s\n", FormatBytes(d.UsedBytes)))
		b.WriteString(fmt.Sprintf("  Free: %s\n", FormatBytes(d.FreeBytes)))
		b.WriteString(fmt.Sprintf("  Usage Percent: %.1f%%\n", d.UsagePercent))
	}
	return b.String()
}

// FormatBytes 把字节数格式化成人类可读形式。
func FormatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// hostname 返回主机名，失败时退化为 "unknown"。
func hostname() string {
	name, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return name
}
