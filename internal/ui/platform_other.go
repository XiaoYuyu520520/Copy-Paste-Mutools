//go:build !windows

package ui

// simInputSupported 表示当前平台是否真正支持模拟键盘输入。
// 非 Windows 平台直接隐藏入口，而不是等用户点击后才报「不支持」。
const simInputSupported = false
