//go:build linux

package usb

import (
	"os"
	"path/filepath"
	"strings"
)

// sysfsDeviceDir 是 Linux 的 USB 设备目录。
const sysfsDeviceDir = "/sys/bus/usb/devices"

// List 返回当前连接的 USB 设备列表。
func List() ([]Device, error) {
	entries, err := os.ReadDir(sysfsDeviceDir)
	if err != nil {
		return nil, err
	}
	list := []Device{}
	for _, e := range entries {
		name := e.Name()
		// 根枢纽名形如 "usb1"，没有 idVendor 文件，跳过。
		base := filepath.Join(sysfsDeviceDir, name)
		if _, err := os.Stat(filepath.Join(base, "idVendor")); err != nil {
			continue
		}
		d := Device{
			VendorID:  readAttr(base, "idVendor"),
			ProductID: readAttr(base, "idProduct"),
			Vendor:    readAttr(base, "manufacturer"),
			Product:   readAttr(base, "product"),
			Serial:    readAttr(base, "serial"),
			Class:     readAttr(base, "bDeviceClass"),
			Speed:     readAttr(base, "speed"),
			Port:      readAttr(base, "devpath"),
		}
		list = append(list, d)
	}
	sortDevices(list)
	return list, nil
}

// readAttr 读取 sysfs 属性文件的内容。
func readAttr(dir, name string) string {
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
