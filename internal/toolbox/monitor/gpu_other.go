//go:build !linux

package monitor

// collectGPU 在非 Linux 平台不采集 GPU（避免额外依赖）。
func collectGPU() []GPUSample {
	return []GPUSample{}
}
