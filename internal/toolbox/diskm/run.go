package diskm

import (
	"context"
	"fmt"
	"time"
)

// Run 在终端里持续打印磁盘 I/O 采样，直到 count 用尽或用户按 Ctrl+C。
func Run(opts Options) error {
	if opts.Interval <= 0 {
		opts.Interval = time.Second
	}

	ctx := context.Background()
	if opts.Count > 0 {
		// 有限次数时用超时兜底，避免调用方忘记取消。
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Interval*time.Duration(opts.Count)+time.Second)
		defer cancel()
	}

	fmt.Printf("正在监控磁盘 I/O（间隔 %s，Ctrl+C 退出）\n", opts.Interval)
	fmt.Printf("%-14s %-12s %-12s %-12s\n", "设备", "读MB/s", "写MB/s", "繁忙率%")
	fmt.Println("------------------------------------------------")

	return Monitor(ctx, opts, func(samples []Sample) {
		for _, s := range samples {
			fmt.Printf("%-14s %-12.2f %-12.2f %-12.1f\n", s.Device, s.ReadMBs, s.WriteMBs, s.UtilPct)
		}
	})
}

// Start 在后台启动监控，把采样打到标准输出。
//
// 用于被其它命令（如 toolbox）以「服务」形式调用。
func Start(ctx context.Context, opts Options) {
	go func() {
		_ = Run(opts)
	}()
}
