//go:build windows

package usb

import (
	"encoding/json"
	"os/exec"
	"strings"
)

// pnpDevice 是 Get-PnpDevice 输出的字段子集。
type pnpDevice struct {
	InstanceID   string `json:"InstanceId"`
	Class        string `json:"Class"`
	FriendlyName string `json:"FriendlyName"`
	Status       string `json:"Status"`
}

// List 返回当前连接的 USB 设备列表。
func List() ([]Device, error) {
	const script = `Get-PnpDevice -Class USB -ErrorAction SilentlyContinue | Select-Object InstanceId,Class,FriendlyName,Status | ConvertTo-Json -Compress`
	out, err := exec.Command("powershell", "-NoProfile", "-Command", script).Output()
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(string(out))
	if text == "" {
		return []Device{}, nil
	}

	var many []pnpDevice
	if err := json.Unmarshal([]byte(text), &many); err == nil {
		return convertPnp(many), nil
	}
	var one pnpDevice
	if err := json.Unmarshal([]byte(text), &one); err == nil {
		return convertPnp([]pnpDevice{one}), nil
	}
	return []Device{}, nil
}

// convertPnp 把 PnP 记录转换成 Device。
//
// Windows 的 InstanceId 形如 "USB\VID_1A86&PID_7523\5&1234ABCD&0&2"，
// 从中解析 VID/PID。
func convertPnp(list []pnpDevice) []Device {
	out := []Device{}
	for _, p := range list {
		if !strings.Contains(strings.ToUpper(p.InstanceID), "USB\\") {
			continue
		}
		d := Device{
			Vendor: p.FriendlyName,
			Class:  p.Class,
			Serial: parseSerial(p.InstanceID),
		}
		d.VendorID, d.ProductID = parseVIDPID(p.InstanceID)
		out = append(out, d)
	}
	sortDevices(out)
	return out
}

// parseVIDPID 从 InstanceId 中解析厂商 ID 与产品 ID。
func parseVIDPID(instanceID string) (string, string) {
	upper := strings.ToUpper(instanceID)
	vid := extractHex(upper, "VID_")
	pid := extractHex(upper, "PID_")
	return vid, pid
}

// extractHex 取出标记之后的第一段十六进制。
func extractHex(s, marker string) string {
	idx := strings.Index(s, marker)
	if idx < 0 {
		return ""
	}
	rest := s[idx+len(marker):]
	end := 0
	for end < len(rest) {
		c := rest[end]
		if (c >= '0' && c <= '9') || (c >= 'A' && c <= 'F') {
			end++
			continue
		}
		break
	}
	return rest[:end]
}

// parseSerial 从 InstanceId 最后一段取出序列号。
func parseSerial(instanceID string) string {
	parts := strings.Split(instanceID, "\\")
	if len(parts) == 0 {
		return ""
	}
	last := parts[len(parts)-1]
	// 末段常带 &0&2 之类的位置后缀，截断到第一个 &。
	if idx := strings.Index(last, "&"); idx > 0 {
		last = last[:idx]
	}
	return last
}
