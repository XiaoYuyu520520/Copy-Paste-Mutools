//go:build !windows

package netscan

import "os/exec"

// hideChildWindow 非 Windows 平台无窗口隐藏需求。
func hideChildWindow(cmd *exec.Cmd) {}
