package data

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
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
	// ID 是服务商的稳定唯一标识，用于删除/选中时精确定位，
	// 避免依赖列表下标（删除前面的项后下标整体偏移导致选错）。
	ID      string `json:"id,omitempty"`
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key,omitempty"`
	Model   string `json:"model"`
	System  string `json:"system,omitempty"`
}

// ProviderID 返回服务商 ID，空 ID 时基于内容生成一个稳定标识（不回写）。
func (p AIProvider) ProviderID() string {
	if p.ID != "" {
		return p.ID
	}
	return providerIDFrom(p.BaseURL, p.Model, p.Name)
}

// providerIDFrom 由若干字段构造确定性 ID，同一内容在不同次运行中一致。
func providerIDFrom(parts ...string) string {
	h := fnv.New64a()
	for i, s := range parts {
		if i > 0 {
			_, _ = h.Write([]byte("\x00"))
		}
		_, _ = h.Write([]byte(strings.TrimSpace(s)))
	}
	return fmt.Sprintf("p%016x", h.Sum64())
}

// ensureProviderIDs 给缺少 ID 的服务商补上 ID（兼容旧配置）。
func (c *Config) ensureProviderIDs() {
	used := map[string]struct{}{}
	for i := range c.Providers {
		id := c.Providers[i].ID
		if id == "" {
			id = c.Providers[i].ProviderID()
		}
		// 极端碰撞兜底：加后缀保证唯一。
		base := id
		for n := 1; ; n++ {
			if _, taken := used[id]; !taken {
				break
			}
			id = fmt.Sprintf("%s-%d", base, n)
		}
		c.Providers[i].ID = id
		used[id] = struct{}{}
	}
}

// FindProvider 按 ID 返回服务商下标，未找到返回 -1。
func (c *Config) FindProvider(id string) int {
	for i := range c.Providers {
		if c.Providers[i].ProviderID() == id {
			return i
		}
	}
	return -1
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
	c.ensureProviderIDs()
	if c.ActiveProvider < 0 || c.ActiveProvider >= len(c.Providers) {
		c.ActiveProvider = 0
	}
	ai := c.Providers[c.ActiveProvider].ToAIConfig()
	c.AI = &ai
}

// SetProviders 整体替换服务商列表（用于设置页保存）。
//
// active 可以是下标（向后兼容）或服务商 ID（字符串形式，经 SelectProvidersByID 使用）；
// 这里仍按下标处理，但会确保每个条目都有稳定 ID。
func (c *Config) SetProviders(list []AIProvider, active int) {
	if len(list) == 0 {
		c.Providers = nil
		c.AI = nil
		c.EnsureProviders()
		return
	}
	c.Providers = append([]AIProvider(nil), list...)
	c.ensureProviderIDs()
	if active < 0 || active >= len(c.Providers) {
		active = 0
	}
	c.ActiveProvider = active
	ai := c.Providers[active].ToAIConfig()
	c.AI = &ai
}

// SelectProvidersByID 整体替换服务商列表并按 ID 选取当前生效项。
//
// 相比 SetProviders 的下标方式，删除/重排后仍能保持同一个服务商被选中。
// 找不到该 ID 时回落到下标 0。
func (c *Config) SelectProvidersByID(list []AIProvider, activeID string) {
	if len(list) == 0 {
		c.Providers = nil
		c.AI = nil
		c.EnsureProviders()
		return
	}
	c.Providers = append([]AIProvider(nil), list...)
	c.ensureProviderIDs()
	active := 0
	if idx := c.FindProvider(activeID); idx >= 0 {
		active = idx
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
		if p.ID == "" {
			p.ID = providerIDFrom(p.BaseURL, p.Model, p.Name)
		}
		c.Providers = append(c.Providers, p)
		c.ActiveProvider = len(c.Providers) - 1
	} else {
		// 更新时保留原 ID，避免编辑导致 ID 漂移。
		if p.ID == "" {
			p.ID = c.Providers[idx].ProviderID()
		}
		c.Providers[idx] = p
		c.ActiveProvider = idx
	}
	c.ensureProviderIDs()
	ai := p.ToAIConfig()
	c.AI = &ai
}

// DeleteProvider 删除服务商（至少保留一个）。
//
// 选中项按服务商唯一 ID 保持，而不是按下标：
// 例 A,B,C,D 选中 C 删除 A 后仍选中 C（而非被下标偏移误选到 D）。
func (c *Config) DeleteProvider(idx int) {
	c.EnsureProviders()
	if len(c.Providers) <= 1 || idx < 0 || idx >= len(c.Providers) {
		return
	}
	activeID := c.Providers[c.ActiveProvider].ProviderID()
	c.Providers = append(c.Providers[:idx], c.Providers[idx+1:]...)

	active := 0
	if a := c.FindProvider(activeID); a >= 0 {
		active = a
	} else if idx < len(c.Providers) {
		// 删除的正是当前选中项：选中原位置的下一项（末尾则选前一项）。
		active = idx
	} else {
		active = len(c.Providers) - 1
	}
	c.ActiveProvider = active
	ai := c.Providers[active].ToAIConfig()
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
		// 主文件缺失或不可读时尝试 .bak 备份，尽量不丢用户配置。
		if bak, e := os.ReadFile(ConfigPath() + bakSuffix); e == nil {
			raw = bak
			err = nil
		}
	}
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

// SaveConfig 将配置写回磁盘（原子写入，保留 .bak 备份）。
func SaveConfig(c Config) error {
	if err := EnsureStorageDir(); err != nil {
		return err
	}
	return SaveJSONAtomic(ConfigPath(), c)
}
