package sysinfo

// GPU 采集按平台拆分：
//   - Windows：通过 PowerShell 的 CIM/WMI 查询 Win32_VideoController
//   - Linux：读取 /sys/class/drm/card*/device 下的 vendor/device
//
// 之所以不引入第三方 GPU 库：跨平台直读 sysfs / CIM 足够覆盖「显卡型号 +
// 显存」这类基础展示需求，且不增加 cgo 依赖。

// CollectGPU 采集显卡信息。采集失败时返回空切片而不是错误，
// 与 Python 原脚本「拿不到就跳过」的行为一致。
func CollectGPU() []GPUInfo {
	return collectGPU()
}
