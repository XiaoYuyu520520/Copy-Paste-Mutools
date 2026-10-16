//go:build !windows

package ui

import (
	"errors"
	"time"
)

// simulateKeys 非 Windows 平台的占位实现。
func simulateKeys(keys []simKey, startDelay, perKey time.Duration, repeats int, stopped func() bool, step func(int)) (bool, error) {
	return false, errors.New("当前平台不支持模拟键盘输入（仅 Windows）")
}
