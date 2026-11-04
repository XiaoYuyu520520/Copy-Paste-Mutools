package data

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

// AIProvider 是一个可保存的服务商配置（不写死厂商）。
type AIProvider struct {
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key,omitempty"`
	Model   string `json:"model"`
	System  string `json:"system,omitempty"`
}

// ProviderFromAI 把当前生效配置转成服务商条目。
func ProviderFromAI(name string, ai AIConfig) AIProvider {
	if strings.TrimSpace(name) == "" {
		name = "默认服务"
	}
	return AIProvider{Name: name, BaseURL: ai.BaseURL, APIKey: ai.APIKey, Model: ai.Model, System: ai.System}
}

// ToAIConfig 转成请求用的配置。
func (p AIProvider) ToAIConfig() AIConfig {
	return AIConfig{BaseURL: p.BaseURL, APIKey: p.APIKey, Model: p.Model, System: p.System}
}

// EnsureProviders 保证服务商列表与当前生效配置一致，并兼容旧版单配置。
func (c *Config) EnsureProviders() {
	if len(c.Providers) == 0 {
		if c.AI != nil {
			c.Providers = []AIProvider{ProviderFromAI("默认服务", *c.AI)}
		} else {
			c.Providers = []AIProvider{{
				Name:    "OpenAI",
				BaseURL: "https://api.openai.com/v1",
				Model:   "gpt-4o-mini",
			}}
		}
		c.ActiveProvider = 0
	}
	if c.ActiveProvider < 0 || c.ActiveProvider >= len(c.Providers) {
		c.ActiveProvider = 0
	}
	ai := c.Providers[c.ActiveProvider].ToAIConfig()
	c.AI = &ai
}

// SetProviders 整体替换服务商列表（用于设置页保存）。
func (c *Config) SetProviders(list []AIProvider, active int) {
	if len(list) == 0 {
		c.Providers = nil
		c.AI = nil
		c.EnsureProviders()
		return
	}
	c.Providers = append([]AIProvider(nil), list...)
	if active < 0 || active >= len(c.Providers) {
		active = 0
	}
	c.ActiveProvider = active
	ai := c.Providers[active].ToAIConfig()
	c.AI = &ai
}

// SelectProvider 切换当前服务商，并同步 AI 为它的配置。
func (c *Config) SelectProvider(idx int) {
	c.EnsureProviders()
	if idx < 0 || idx >= len(c.Providers) {
		return
	}
	c.ActiveProvider = idx
	ai := c.Providers[idx].ToAIConfig()
	c.AI = &ai
}

// UpsertProvider 新增（idx<0）或更新服务商。
func (c *Config) UpsertProvider(idx int, p AIProvider) {
	if strings.TrimSpace(p.Name) == "" {
		p.Name = "未命名服务"
	}
	if idx < 0 || idx >= len(c.Providers) {
		c.Providers = append(c.Providers, p)
		c.ActiveProvider = len(c.Providers) - 1
	} else {
		c.Providers[idx] = p
		c.ActiveProvider = idx
	}
	ai := p.ToAIConfig()
	c.AI = &ai
}

// DeleteProvider 删除服务商（至少保留一个）。
func (c *Config) DeleteProvider(idx int) {
	c.EnsureProviders()
	if len(c.Providers) <= 1 || idx < 0 || idx >= len(c.Providers) {
		return
	}
	c.Providers = append(c.Providers[:idx], c.Providers[idx+1:]...)
	if c.ActiveProvider >= len(c.Providers) {
		c.ActiveProvider = len(c.Providers) - 1
	}
	ai := c.Providers[c.ActiveProvider].ToAIConfig()
	c.AI = &ai
}

// Config 整个应用的持久化配置。
type Config struct {
	Tabs      []TabConfig `json:"tabs"`
	ActiveTab int         `json:"active_tab"`
	DataDir   string      `json:"data_dir,omitempty"`
	ThemeMode string      `json:"theme_mode"`
	AI        *AIConfig   `json:"ai,omitempty"`

	// Providers 是可保存的多个 OpenAI 兼容服务商；AI 始终镜像当前生效项。
	Providers      []AIProvider `json:"providers,omitempty"`
	ActiveProvider int          `json:"active_provider,omitempty"`

	// WindowWidth/Height 记录桌面端窗口尺寸（<=0 表示未记录，用默认值）。
	WindowWidth  float32 `json:"window_width,omitempty"`
	WindowHeight float32 `json:"window_height,omitempty"`
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

// padItems 把任意长度的 item 切片补齐到至少 DefaultRows，并保留超出部分。
// 旧版固定 5 行配置仍可读；新版允许超过 5 行。
func padItems(items []Item) []Item {
	n := len(items)
	if n < DefaultRows {
		n = DefaultRows
	}
	out := make([]Item, n)
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
	// 升级兼容：把 exe 同级的旧 config.json 迁到用户配置目录。
	migrateLegacyConfig()
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
			cfg.Tabs = []TabConfig{NormalizeTab(TabConfig{
				Prompts:   padItems(legacy.Prompts),
				Sentences: padItems(legacy.Sentences),
			})}
			cfg.DataDir = legacy.DataDir
			cfg.EnsureProviders()
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
		cfg.Tabs[i] = NormalizeTab(cfg.Tabs[i])
	}
	if cfg.ActiveTab < 0 || cfg.ActiveTab >= len(cfg.Tabs) {
		cfg.ActiveTab = 0
	}
	cfg.EnsureProviders()
	return cfg, nil
}

// SaveConfig 将配置写回磁盘。
func SaveConfig(c Config) error {
	buf, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := EnsureStorageDir(); err != nil {
		return err
	}
	return os.WriteFile(ConfigPath(), buf, 0644)
}
