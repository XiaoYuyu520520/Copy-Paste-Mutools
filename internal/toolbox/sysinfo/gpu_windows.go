//go:build windows

package sysinfo

import (
	"encoding/json"
	"os/exec"
	"strings"
)

// winGPU 是 CIM 查询 Win32_VideoController 得到的字段子集。
type winGPU struct {
	Name                 string `json:"Name"`
	AdapterRAM           uint64 `json:"AdapterRAM"`
	DriverVersion        string `json:"DriverVersion"`
	VideoProcessor       string `json:"VideoProcessor"`
	AdapterCompatibility string `json:"AdapterCompatibility"`
}

// collectGPU 通过 PowerShell 调用 CIM 查询显卡。
func collectGPU() []GPUInfo {
	const script = `Get-CimInstance Win32_VideoController | Select-Object Name,AdapterRAM,DriverVersion | ConvertTo-Json -Compress`
	out, err := exec.Command("powershell", "-NoProfile", "-Command", script).Output()
	if err != nil {
		return []GPUInfo{}
	}
	text := strings.TrimSpace(string(out))
	if text == "" {
		return []GPUInfo{}
	}

	// ConvertTo-Json 在单条记录时返回对象，多条时返回数组，两种都要处理。
	var many []winGPU
	if err := json.Unmarshal([]byte(text), &many); err == nil {
		return convertWinGPU(many)
	}
	var one winGPU
	if err := json.Unmarshal([]byte(text), &one); err == nil {
		return convertWinGPU([]winGPU{one})
	}
	return []GPUInfo{}
}

func convertWinGPU(list []winGPU) []GPUInfo {
	out := []GPUInfo{}
	for _, g := range list {
		if strings.TrimSpace(g.Name) == "" {
			continue
		}
		out = append(out, GPUInfo{
			Source:      "CIM",
			Name:        g.Name,
			MemoryTotal: g.AdapterRAM,
			Driver:      g.DriverVersion,
		})
	}
	return out
}
