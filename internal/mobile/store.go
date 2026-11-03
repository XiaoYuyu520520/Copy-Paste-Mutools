// Package mobile 是 Android / 手机端的界面层。
//
// 与桌面版 `internal/ui` 的差别：
//   - 布局面向触屏：底部导航 + 单页全屏，而不是左侧竖栏 + 状态栏
//   - 只保留手机上真正有意义的功能（编辑器 / AI 聊天 / 局域网扫描 / 画板 / 更多）
//   - 逻辑层完全复用 internal/data、internal/chat、internal/netscan、
//     internal/toolbox/{share,monitor,paint}
//
// 之所以单独一套 UI 而不是给桌面版加 if mobile 补丁：两者的信息密度、
// 交互方式（点击 vs 拖拽、长按、软键盘）差别太大，混在一起只会两边都难用。
package mobile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"fyne.io/fyne/v2"

	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/data"
)

// store 负责移动端的配置持久化。
//
// Android 上可执行文件目录不可写，因此一律写进 Fyne 的应用存储目录
// （Android 对应 /data/data/<pkg>/files，桌面调试时是用户配置目录）。
type store struct {
	mu   sync.Mutex
	path string
	cfg  data.Config

	saveTimer *time.Timer
}

// newStore 载入配置；文件不存在或损坏时回落到默认配置。
func newStore(a fyne.App) *store {
	s := &store{path: configPath(a), cfg: data.DefaultConfig()}
	if b, err := os.ReadFile(s.path); err == nil {
		var cfg data.Config
		if err := json.Unmarshal(b, &cfg); err == nil {
			s.cfg = normalize(cfg)
		}
	}
	s.cfg.EnsureProviders()
	return s
}

// Config 返回配置的深拷贝快照。
//
// 调用方对返回值的修改不会影响 store，避免解锁后继续改内部状态、
// 或与定时保存并发读写同一份数据。
func (s *store) Config() data.Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneConfig(s.cfg)
}

// Update 在锁内修改配置。回调里拿到的是内部配置，请只在回调内使用。
func (s *store) Update(fn func(*data.Config)) {
	if fn == nil {
		return
	}
	s.mu.Lock()
	fn(&s.cfg)
	s.mu.Unlock()
}

// AIConfig 返回 AI 配置快照（始终非 nil）。
func (s *store) AIConfig() data.AIConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cfg.AI == nil {
		return data.AIConfig{BaseURL: "https://api.openai.com/v1", Model: "gpt-4o-mini"}
	}
	return *s.cfg.AI
}

// cloneConfig 深拷贝配置，隔离切片与 AI 指针。
func cloneConfig(c data.Config) data.Config {
	out := c
	out.Tabs = make([]data.TabConfig, len(c.Tabs))
	for i, t := range c.Tabs {
		out.Tabs[i] = data.TabConfig{
			Prompts:   append([]data.Item(nil), t.Prompts...),
			Sentences: append([]data.Item(nil), t.Sentences...),
		}
	}
	if c.AI != nil {
		ai := *c.AI
		out.AI = &ai
	}
	return out
}

// Save 立即落盘。
func (s *store) Save() error {
	s.mu.Lock()
	b, err := json.MarshalIndent(s.cfg, "", "  ")
	path := s.path
	s.mu.Unlock()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}

// SaveDebounced 合并短时间内的多次修改，1 秒后统一落盘。
//
// 编辑器每敲一个字符都会触发保存，直接写盘在手机上会造成明显卡顿。
func (s *store) SaveDebounced() {
	s.mu.Lock()
	if s.saveTimer != nil {
		s.saveTimer.Stop()
	}
	s.saveTimer = time.AfterFunc(time.Second, func() { _ = s.Save() })
	s.mu.Unlock()
}

// configPath 返回配置文件路径。
func configPath(a fyne.App) string {
	root := a.Storage().RootURI()
	if root != nil {
		if p := root.Path(); p != "" {
			return filepath.Join(p, "mobile-config.json")
		}
	}
	// 兜底：用户配置目录。
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "prompt-merger-toolbox", "mobile-config.json")
	}
	return "mobile-config.json"
}

// normalize 修正反序列化后可能缺失的字段（行数不足、空标签等）。
func normalize(cfg data.Config) data.Config {
	if len(cfg.Tabs) == 0 {
		cfg.Tabs = []data.TabConfig{blankTab()}
	}
	for i := range cfg.Tabs {
		cfg.Tabs[i] = normalizeTab(cfg.Tabs[i])
	}
	if cfg.ActiveTab < 0 || cfg.ActiveTab >= len(cfg.Tabs) {
		cfg.ActiveTab = 0
	}
	cfg.EnsureProviders()
	return cfg
}

// normalizeTab 对齐两列长度并补齐区块元数据。
func normalizeTab(t data.TabConfig) data.TabConfig {
	return data.NormalizeTab(t)
}

// blankTab 返回一个默认区块数的空标签。
func blankTab() data.TabConfig {
	return data.NormalizeTab(data.TabConfig{
		Prompts:   data.DefaultPrompts(),
		Sentences: data.DefaultSentences(),
	})
}
