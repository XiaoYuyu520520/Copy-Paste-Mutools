//go:build !linux && !windows

package usb

import "fmt"

// List 在未支持的平台上返回空列表。
//
// macOS / BSD 等平台的 USB 枚举需要 IOKit / libusb，这里不引入额外依赖，
// 保持工具箱在所有平台都能编译。
func List() ([]Device, error) {
	return []Device{}, fmt.Errorf("当前平台 %s 暂不支持 USB 枚举（仅支持 Windows 与 Linux）", "非 Windows/Linux")
}
