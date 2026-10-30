//go:build !windows

package sysinfo

import (
	"os"
	"path/filepath"
	"strings"
)

// collectGPU 在非 Windows 平台读取 /sys/class/drm。
func collectGPU() []GPUInfo {
	out := []GPUInfo{}
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
		out = append(out, GPUInfo{
			Source: "sysfs",
			Name:   strings.TrimSpace(vendor + " " + name),
		})
	}
	return out
}

// readSysString 读取 sysfs 中的一行文本。
func readSysString(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
