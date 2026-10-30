package cli

import (
	"flag"
	"io/fs"
	"net/http"
)

// flagSet 创建一个子命令的 FlagSet，输出统一走 stderr。
func flagSet(name string) *flag.FlagSet {
	return flag.NewFlagSet(name, flag.ContinueOnError)
}

// httpFS 把 fs.FS 包装成 http.FileSystem。
func httpFS(f fs.FS) http.FileSystem {
	return http.FS(f)
}
