// Package netscan 提供局域网设备扫描能力。
//
// 代码改编自 MIT 许可的开源项目「LAN Scan · 局域网设备扫描工具」
// （局域网设备扫描/ 目录，README 声明 MIT），仅重命名包路径、未改探测逻辑：
//   - 系统 ping 探测（含 Windows/Linux/macOS 参数差异）
//   - 无 ICMP 时的并发 TCP 回退探测
//   - ARP 表 MAC、反向 DNS、开放端口等补充信息
package netscan
