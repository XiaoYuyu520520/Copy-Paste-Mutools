//go:build linux

package monitor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// collectGPU 在 Linux 上通过 sysfs 读取显卡型号与利用率。
//
// 利用率来自 /sys/class/drm/card*/device/gpu_busy_percent（NVIDIA 专有），
// 缺失时留 0。
func collectGPU() []GPUSample {
	out := []GPUSample{}
	cards, err := filepath.Glob("/sys/class/drm/card[0-9]*")
	if err != nil {
		return out
	}
	for _, card := range cards {
		device := filepath.Join(card, "device")
		name := readSysString(filepath.Join(device, "device"))
		vendor := readSysString(filepath.Join(device, "vendor"))
		if name == "" && vendor == "" {
			continue
		}
		sample := GPUSample{Name: strings.TrimSpace(vendor + " " + name)}
		if busy := readSysString(filepath.Join(device, "gpu_busy_percent")); busy != "" {
			sample.UsagePct = parseFloat(busy)
		}
		if memInfo := filepath.Join(device, "mem_info_vram_used"); true {
			if b, err := os.ReadFile(memInfo); err == nil {
				sample.MemoryUsed = uint64(parseFloat(strings.TrimSpace(string(b))))
			}
		}
		if memTotal := filepath.Join(device, "mem_info_vram_total"); true {
			if b, err := os.ReadFile(memTotal); err == nil {
				sample.MemoryTotal = uint64(parseFloat(strings.TrimSpace(string(b))))
			}
		}
		out = append(out, sample)
	}
	return out
}

// readSysString 读取 sysfs 文件内容。
func readSysString(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// parseFloat 解析浮点数，失败返回 0。
func parseFloat(s string) float64 {
	var v float64
	if _, err := fmt.Sscanf(s, "%g", &v); err != nil {
		return 0
	}
	return v
}
