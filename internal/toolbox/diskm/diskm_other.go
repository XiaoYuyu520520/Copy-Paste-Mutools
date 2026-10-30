//go:build !linux

package diskm

// collect 在非 Linux 平台返回空集合。
//
// Windows 的磁盘性能计数器需要 Performance Counter API（PDH），
// 当前版本暂未实现；此时命令会提示用户改用 Linux 或使用系统自带的
// 资源监视器。
func collect() map[string]Sample {
	return map[string]Sample{}
}
