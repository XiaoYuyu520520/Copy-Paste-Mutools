package cli

import (
	"fmt"
	"os"
	"runtime"
	"time"
)

// systemSummary 返回一行系统摘要，用于基准测试报告。
func systemSummary() string {
	host, err := os.Hostname()
	if err != nil {
		host = "unknown"
	}
	return fmt.Sprintf("%s/%s %s", runtime.GOOS, runtime.GOARCH, host)
}

// durationSeconds 把秒数转成 time.Duration。
func durationSeconds(sec int) time.Duration {
	if sec <= 0 {
		sec = 1
	}
	return time.Duration(sec) * time.Second
}

// nowString 返回当前时间的字符串形式。
func nowString() string {
	return time.Now().Format("15:04:05")
}

// timeAfter 返回一个在 d 之后触发的 channel。
func timeAfter(d time.Duration) <-chan time.Time {
	return time.After(d)
}
