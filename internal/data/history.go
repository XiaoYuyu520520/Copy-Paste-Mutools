package data

import (
	"os"
	"time"
)

// HistoryEntry 单条历史记录。
type HistoryEntry struct {
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
}

// MaxHistory 历史记录最大保留条数。
const MaxHistory = 200

// LoadHistory 读取 history.json；主文件缺失/损坏时回退到 .bak。失败返回 nil。
func LoadHistory(dataDir string) []HistoryEntry {
	path := HistoryPath(dataDir)
	var entries []HistoryEntry
	if _, err := LoadJSONWithBackup(path, &entries); err != nil {
		return nil
	}
	return entries
}

// SaveHistory 写入 history.json（原子写入，保留 .bak 备份）。
func SaveHistory(entries []HistoryEntry, dataDir string) error {
	if dataDir == "" {
		dataDir = ExecDir()
	}
	_ = os.MkdirAll(dataDir, 0755)
	return SaveJSONAtomic(HistoryPath(dataDir), entries)
}

// AppendHistory 在尾部追加一条，超过 MaxHistory 自动丢弃最旧。
func AppendHistory(entries []HistoryEntry, content string) []HistoryEntry {
	if content == "" {
		return entries
	}
	entry := HistoryEntry{
		Content:   content,
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
	}
	entries = append(entries, entry)
	if len(entries) > MaxHistory {
		entries = entries[len(entries)-MaxHistory:]
	}
	return entries
}
