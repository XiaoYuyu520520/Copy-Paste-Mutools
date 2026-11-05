package data

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// 本文件统一「写盘」策略，避免各处直接 os.WriteFile 覆盖导致的半截文件：
//
//  1. 先写到同目录的 <name>.tmp 临时文件；
//  2. 落盘完成后，把现有文件备份为 <name>.bak（若存在且可读）；
//  3. 用临时文件原子重命名为目标文件。
//
// 这样即使写入过程中进程被杀 / 断电，目标文件要么是旧的完整内容，
// 要么是新的完整内容，不会留下「写了一半」的 JSON；
// 万一新内容损坏，<name>.bak 还能恢复上一份配置。
//
// 桌面 config.json / templates.json / history.json、Android 的
// mobile-config.json / chat-sessions.json 全部走这一套逻辑。

// bakSuffix、tmpSuffix 是原子写入使用的文件后缀。
const (
	tmpSuffix = ".tmp"
	bakSuffix = ".bak"
)

// SaveJSONAtomic 把 v 序列化为缩进 JSON 并原子写入 path（保留 .bak 备份）。
//
// 目录不存在时自动创建。返回值仅表示「最终目标文件已成功替换」。
func SaveJSONAtomic(path string, v any) error {
	buf, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 JSON 失败: %w", err)
	}
	return SaveFileAtomic(path, buf)
}

// SaveFileAtomic 把已序列化的字节原子写入 path（保留 .bak 备份）。
func SaveFileAtomic(path string, data []byte) error {
	if path == "" {
		return errors.New("保存路径为空")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 1) 写临时文件。用固定后缀便于失败后清理，也方便用户识别。
	tmp := path + tmpSuffix
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("写入临时文件失败: %w", err)
	}

	// 2) 备份旧文件（失败不阻断主流程：备份只是可恢复性增强）。
	if err := os.Rename(path, path+bakSuffix); err != nil && !errors.Is(err, os.ErrNotExist) {
		// 某些文件系统不允许覆盖已有 .bak，先删后重试一次。
		_ = os.Remove(path + bakSuffix)
		_ = os.Rename(path, path+bakSuffix)
	}

	// 3) 原子替换。Rename 在同目录内是原子操作（Windows 使用 ReplaceFile 语义）。
	if err := os.Rename(tmp, path); err != nil {
		// 回滚：把备份挪回去，避免目标文件消失。
		_ = os.Rename(path+bakSuffix, path)
		_ = os.Remove(tmp)
		return fmt.Errorf("替换目标文件失败: %w", err)
	}
	return nil
}

// LoadJSONWithBackup 先读主文件，失败或损坏时尝试 .bak 备份。
//
// 返回是否使用了备份（true 表示主文件缺失/损坏，数据来自 .bak）。
func LoadJSONWithBackup(path string, v any) (usedBackup bool, err error) {
	if e := loadJSONFile(path, v); e == nil {
		return false, nil
	}
	if e := loadJSONFile(path+bakSuffix, v); e == nil {
		return true, nil
	}
	return false, fmt.Errorf("读取 %s 及其备份均失败", filepath.Base(path))
}

func loadJSONFile(path string, v any) error {
	buf, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(buf, v)
}
