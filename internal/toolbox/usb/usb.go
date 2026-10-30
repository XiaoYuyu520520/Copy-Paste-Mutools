// Package usb 枚举系统上的 USB 设备。
//
// 迁移自 legacy/python/.../USB_scan/main.py：原脚本用 pyudev（Linux）/
// WMI（Windows）采集设备信息并生成 Markdown 快照。这里改成：
//   - Linux：读 /sys/bus/usb/devices
//   - Windows：调用 PowerShell 的 Get-PnpDevice
//
// 保持原有两种模式：scan（当前快照）与 monitor（插拔事件轮询）。
package usb

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
	"time"
)

// Device 描述一个 USB 设备。
type Device struct {
	VendorID  string `json:"vendor_id"`
	ProductID string `json:"product_id"`
	Vendor    string `json:"vendor"`
	Product   string `json:"product"`
	Serial    string `json:"serial,omitempty"`
	Class     string `json:"class,omitempty"`
	Speed     string `json:"speed,omitempty"`
	Port      string `json:"port,omitempty"`
}

// String 返回适合人读的一行描述。
func (d Device) String() string {
	name := strings.TrimSpace(strings.TrimSpace(d.Vendor) + " " + strings.TrimSpace(d.Product))
	if name == "" {
		name = "未知设备"
	}
	id := d.VendorID + ":" + d.ProductID
	if id == ":" {
		id = "-"
	}
	return fmt.Sprintf("%-28s %-12s %s", name, id, d.Serial)
}

// Snapshot 采集一次设备快照。
func Snapshot() (SnapshotResult, error) {
	devices, err := List()
	if err != nil {
		return SnapshotResult{}, err
	}
	return SnapshotResult{
		GeneratedAt: time.Now().Format("2006-01-02 15:04:05"),
		OS:          runtime.GOOS,
		Devices:     devices,
	}, nil
}

// SnapshotResult 是一次设备快照。
type SnapshotResult struct {
	GeneratedAt string   `json:"generated_at"`
	OS          string   `json:"os"`
	Devices     []Device `json:"devices"`
}

// Markdown 把快照渲染成 Markdown 表格（与原脚本的输出格式一致）。
func (s SnapshotResult) Markdown() string {
	var b strings.Builder
	b.WriteString("# USB 设备快照\n\n")
	b.WriteString(fmt.Sprintf("- 生成时间: %s\n", s.GeneratedAt))
	b.WriteString(fmt.Sprintf("- 操作系统: %s\n", s.OS))
	b.WriteString(fmt.Sprintf("- 设备数量: %d\n\n", len(s.Devices)))
	b.WriteString("| Vendor ID | Product ID | 名称 | 序列号 | 类 |\n")
	b.WriteString("| --- | --- | --- | --- | --- |\n")
	for _, d := range s.Devices {
		name := strings.TrimSpace(strings.TrimSpace(d.Vendor) + " " + strings.TrimSpace(d.Product))
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
			d.VendorID, d.ProductID, name, d.Serial, d.Class))
	}
	return b.String()
}

// Monitor 轮询 USB 设备变化，并在检测到新增/移除时回调。
//
// 轮询间隔由 interval 指定；onChange 收到的是本次变化后的完整列表。
func Monitor(interval time.Duration, onChange func([]Device)) error {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	previous := map[string]bool{}
	devices, err := List()
	if err != nil {
		return err
	}
	for _, d := range devices {
		previous[deviceKey(d)] = true
	}

	for {
		time.Sleep(interval)
		devices, err := List()
		if err != nil {
			continue
		}
		current := map[string]bool{}
		for _, d := range devices {
			current[deviceKey(d)] = true
		}
		if !sameKeys(previous, current) {
			previous = current
			if onChange != nil {
				onChange(devices)
			}
		}
	}
}

// deviceKey 生成设备唯一键：优先序列号，其次 厂商:产品。
func deviceKey(d Device) string {
	if d.Serial != "" {
		return d.VendorID + ":" + d.ProductID + ":" + d.Serial
	}
	return d.VendorID + ":" + d.ProductID
}

// sameKeys 比较两个设备集合是否相同。
func sameKeys(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

// sortDevices 按厂商 ID、产品 ID、序列号排序，保证输出稳定。
func sortDevices(list []Device) {
	sort.Slice(list, func(i, j int) bool {
		if list[i].VendorID != list[j].VendorID {
			return list[i].VendorID < list[j].VendorID
		}
		if list[i].ProductID != list[j].ProductID {
			return list[i].ProductID < list[j].ProductID
		}
		return list[i].Serial < list[j].Serial
	})
}
