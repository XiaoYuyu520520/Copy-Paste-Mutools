// 提示词复合工具箱 · 统一命令行入口。
//
// 一个可执行文件承载全部小工具，避免「一个工具一个 exe」的碎片化。
// 每个子命令都是独立的纯逻辑包，本文件只负责参数解析与分发。
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/toolbox/cli"
)

func main() {
	if err := cli.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "错误:", err)
		os.Exit(1)
	}
}

// usage 由 cli 包提供，这里仅在无参数时打印顶层帮助。
func init() {
	if len(os.Args) == 1 {
		fmt.Fprint(os.Stderr, cli.Usage())
		os.Exit(0)
	}
	_ = strings.TrimSpace
}
