package data

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Item 与 merger.go 保持一致，本文件复用。
// 注意：merger.go 中的 Item 是这里的别名引用，所以这里不再重复定义。

const DefaultThemeMode = "light"

// AIConfig AI 聊天的 OpenAI 兼容接口配置。
type AIConfig struct {
	BaseURL string `json:"base_url"`                // 例如 https://api.openai.com/v1
	APIKey  string `json:"api_key,omitempty"`       // Bearer Key（本地兼容服务可留空）
	Model   string `json:"model"`                   // 例如 gpt-4o-mini / deepseek-chat / qwen-plus
	System  string `json:"system_prompt,omitempty"` // 可选的系统提示词
}

// Config 整个应用的持久化配置。
type Config struct {
	Tabs      []TabConfig `json:"tabs"`
	ActiveTab int         `json:"active_tab"`
	DataDir   string      `json:"data_dir,omitempty"`
	ThemeMode string      `json:"theme_mode"`
	AI        *AIConfig   `json:"ai,omitempty"`
}

// DefaultConfig 返回开箱即用的默认配置：3 个空标签 + 亮色主题。
func DefaultConfig() Config {
	return Config{
		Tabs: []TabConfig{
			blankTab(),
			blankTab(),
			blankTab(),
		},
		ActiveTab: 0,
		ThemeMode: DefaultThemeMode,
	}
}

func blankTab() TabConfig {
	return TabConfig{
		Prompts:   blankItems(),
		Sentences: blankItems(),
	}
}

func blankItems() []Item {
	return DefaultItems()
}

// DefaultItems 返回 MaxRows 个默认启用项（仅 Enabled = true）。
func DefaultItems() []Item {
	out := make([]Item, MaxRows)
	for i := range out {
		out[i] = Item{Enabled: true}
	}
	return out
}

// DefaultPrompts 返回一列空白提示词项。
func DefaultPrompts() []Item { return DefaultItems() }

// DefaultSentences 返回一列空白语句项。
func DefaultSentences() []Item { return DefaultItems() }

// ExecDir 返回当前可执行文件所在目录；不可获得则返回 "."。
func ExecDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

// ConfigPath 获取 config.json 路径，始终位于 exe 同级。
func ConfigPath() string {
	return filepath.Join(ExecDir(), "config.json")
}

// HistoryPath 获取 history.json 路径，可被 DataDir 覆盖。
func HistoryPath(dataDir string) string {
	dir := dataDir
	if dir == "" {
		dir = ExecDir()
	}
	return filepath.Join(dir, "history.json")
}

// padItems 将任意长度的 item 切片对齐为 MaxRows 长度。
func padItems(items []Item) []Item {
	out := make([]Item, MaxRows)
	for i := range out {
		if i < len(items) {
			t := items[i].Text
			if len(t) > 200000 {
				t = t[:200000]
			}
			out[i] = Item{Text: t, Enabled: items[i].Enabled}
		} else {
			out[i] = Item{Enabled: true}
		}
	}
	return out
}

// LoadConfig 加载 config.json。
//
// 兼容性：
//  1. 旧版（web 版）只含 prompts/sentences 单组，自动迁移到一个新标签。
//  2. 多标签新版按原样读取。
func LoadConfig() (Config, error) {
	cfg := DefaultConfig()
	raw, err := os.ReadFile(ConfigPath())
	if err != nil {
		return cfg, err
	}

	var probe map[string]json.RawMessage
	if jerr := json.Unmarshal(raw, &probe); jerr != nil {
		return cfg, jerr
	}

	if _, hasTabs := probe["tabs"]; !hasTabs {
		var legacy struct {
			Prompts   []Item `json:"prompts"`
			Sentences []Item `json:"sentences"`
			DataDir   string `json:"data_dir"`
		}
		if jerr := json.Unmarshal(raw, &legacy); jerr == nil {
			cfg.Tabs = []TabConfig{{
				Prompts:   padItems(legacy.Prompts),
				Sentences: padItems(legacy.Sentences),
			}}
			cfg.DataDir = legacy.DataDir
			return cfg, nil
		}
		return cfg, nil
	}

	if jerr := json.Unmarshal(raw, &cfg); jerr != nil {
		return cfg, jerr
	}

	if len(cfg.Tabs) == 0 {
		cfg.Tabs = []TabConfig{blankTab()}
	}
	for i := range cfg.Tabs {
		cfg.Tabs[i].Prompts = padItems(cfg.Tabs[i].Prompts)
		cfg.Tabs[i].Sentences = padItems(cfg.Tabs[i].Sentences)
	}
	if cfg.ActiveTab < 0 || cfg.ActiveTab >= len(cfg.Tabs) {
		cfg.ActiveTab = 0
	}
	return cfg, nil
}

// SaveConfig 将配置写回磁盘。
func SaveConfig(c Config) error {
	buf, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigPath(), buf, 0644)
}
