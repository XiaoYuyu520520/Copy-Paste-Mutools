package data

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// 存储位置策略：
//
//   - 普通安装（默认）：写入操作系统的用户配置目录，
//     避免 Program Files / /usr/bin 等只读目录导致保存失败。
//   - 便携模式：在 exe 同级目录写入，可随 U 盘一起带走。
//
// 便携模式的判定顺序：
//  1. 环境变量 PROMPT_MERGER_PORTABLE=1
//  2. exe 同级存在 portable.txt 文件
type storageMode int

const (
	storageUserDir  storageMode = iota // 普通安装：用户配置目录
	storagePortable                    // 便携模式：exe 同级目录
)

const portableMarker = "portable.txt"

var (
	storageOnce      sync.Once
	storageModeValue storageMode
)

// AppDataDir 返回本应用的数据目录名（用于用户配置目录）。
const AppDataDir = "prompt-merger-toolbox"

// DetectStorageMode 返回当前的存储策略；可在测试中重置。
func DetectStorageMode() storageMode {
	storageOnce.Do(func() {
		storageModeValue = detectStorageMode()
	})
	return storageModeValue
}

// ResetStorageModeForTest 重置缓存的存储策略（仅测试使用）。
func ResetStorageModeForTest(mode storageMode) {
	storageOnce.Do(func() {})
	storageModeValue = mode
	storageOnce = sync.Once{}
	storageOnce.Do(func() { storageModeValue = mode })
}

func detectStorageMode() storageMode {
	if v := strings.TrimSpace(os.Getenv("PROMPT_MERGER_PORTABLE")); v != "" && v != "0" && !strings.EqualFold(v, "false") {
		return storagePortable
	}
	if _, err := os.Stat(filepath.Join(ExecDir(), portableMarker)); err == nil {
		return storagePortable
	}
	return storageUserDir
}

// StorageDir 返回数据目录（不含文件名）。
//
// 普通安装：<用户配置目录>/prompt-merger-toolbox
// 便携模式：<exe 同级目录>
// 失败时回落到 exe 同级目录，保证至少能写入。
func StorageDir() string {
	if DetectStorageMode() == storagePortable {
		return ExecDir()
	}
	dir, err := os.UserConfigDir()
	if err != nil || strings.TrimSpace(dir) == "" {
		return ExecDir()
	}
	return filepath.Join(dir, AppDataDir)
}

// StoragePath 返回数据目录下的文件路径。
func StoragePath(name string) string {
	return filepath.Join(StorageDir(), name)
}

// EnsureStorageDir 确保数据目录存在。
func EnsureStorageDir() error {
	return os.MkdirAll(StorageDir(), 0o755)
}

// ConfigPath 返回 config.json 路径。
//
// 普通安装放在用户配置目录，便携模式放在 exe 同级目录。
// 兼容升级：若用户配置目录还没有文件、但 exe 同级存在旧的 config.json，
// 会自动迁移过去，避免升级后「配置消失」。
func ConfigPath() string {
	return StoragePath("config.json")
}

// legacyConfigPath 是旧版本把配置写在 exe 同级的位置。
func legacyConfigPath() string {
	return filepath.Join(ExecDir(), "config.json")
}

// migrateLegacyConfig 在用户配置目录为空时，把 exe 同级的旧配置迁过来。
// 返回是否发生了迁移。
func migrateLegacyConfig() bool {
	if DetectStorageMode() == storagePortable {
		return false
	}
	target := ConfigPath()
	if _, err := os.Stat(target); err == nil {
		return false // 已有新配置，不动
	}
	old := legacyConfigPath()
	raw, err := os.ReadFile(old)
	if err != nil {
		return false
	}
	if err := EnsureStorageDir(); err != nil {
		return false
	}
	if err := os.WriteFile(target, raw, 0o644); err != nil {
		return false
	}
	return true
}

// HistoryPath 获取 history.json 路径，可被 dataDir 覆盖。
func HistoryPath(dataDir string) string {
	dir := dataDir
	if dir == "" {
		dir = StorageDir()
	}
	return filepath.Join(dir, "history.json")
}

// TemplatesPath 返回模板文件路径；baseDir 为空时使用数据目录。
func TemplatesPath(baseDir string) string {
	if baseDir == "" {
		baseDir = StorageDir()
	}
	return filepath.Join(baseDir, "templates.json")
}

// IsWindows 报告当前是否运行在 Windows 上（供路径/行为差异使用）。
func IsWindows() bool { return runtime.GOOS == "windows" }
