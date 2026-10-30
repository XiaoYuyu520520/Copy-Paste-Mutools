package netscan

import "runtime"

// ICMPProbeSupported 报告当前平台能否用系统 `ping` 命令做 ICMP 探测。
//
// Android 上没有可用的 ping 可执行文件（自 Android 7 起 ICMP 走 raw socket，
// 需要 CAP_NET_RAW 权限，普通应用拿不到），因此只能退化为 TCP 探测。
func ICMPProbeSupported() bool {
	return runtime.GOOS != "android"
}

// MACExtractionSupported 报告当前平台能否从系统 ARP 表读取 MAC 地址。
//
// Android 10 起 /proc/net/arp 对普通应用不可读，`arp` 命令也不存在，
// 因此直接跳过 MAC 解析，避免每个主机都等一次失败的 exec。
func MACExtractionSupported() bool {
	return runtime.GOOS != "android"
}
