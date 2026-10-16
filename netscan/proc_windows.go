//go:build windows

package netscan

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000 // CREATE_NO_WINDOW

// hideChildWindow 让 GUI(无控制台)程序启动的控制台子进程（ping / arp 等）
// 完全不创建控制台窗口。不能用「隐藏窗口」代替：GUI 程序若只是把窗口隐藏，
// Windows 仍会为每个子进程分配隐藏控制台 + conhost，几百个并发进程会把
// CPU 打满；CREATE_NO_WINDOW 则根本不分配控制台，与独立版(共享控制台)开销一致。
func hideChildWindow(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= createNoWindow
}
