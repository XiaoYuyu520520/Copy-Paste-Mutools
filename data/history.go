package data

import (
	"encoding/json"
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

// LoadHistory 读取 history.json。失败返回 nil。
func LoadHistory(dataDir string) []HistoryEntry {
	path := HistoryPath(dataDir)
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var entries []HistoryEntry
	if err := json.Unmarshal(buf, &entries); err != nil {
		return nil
	}
	return entries
}

// SaveHistory 写入 history.json，若 dataDir 为空则用 exe 同级目录。
func SaveHistory(entries []HistoryEntry, dataDir string) error {
	if dataDir == "" {
		dataDir = ExecDir()
	}
	_ = os.MkdirAll(dataDir, 0755)
	buf, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(HistoryPath(dataDir), buf, 0644)
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
