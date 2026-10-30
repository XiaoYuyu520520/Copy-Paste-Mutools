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
	if s.cfg.AI == nil {
		s.cfg.AI = &data.AIConfig{BaseURL: "https://api.openai.com/v1", Model: "gpt-4o-mini"}
	}
	return s
}

// Config 返回配置指针（调用方修改后应调用 Save 或 SaveDebounced）。
func (s *store) Config() *data.Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return &s.cfg
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
	return cfg
}

// normalizeTab 保证两个列表都有 MaxRows 项。
func normalizeTab(t data.TabConfig) data.TabConfig {
	t.Prompts = padItems(t.Prompts)
	t.Sentences = padItems(t.Sentences)
	return t
}

// padItems 把项目列表补齐/截断到 MaxRows。
func padItems(items []data.Item) []data.Item {
	out := make([]data.Item, data.MaxRows)
	for i := 0; i < data.MaxRows; i++ {
		if i < len(items) {
			out[i] = items[i]
		} else {
			out[i] = data.Item{Enabled: true}
		}
	}
	return out
}

// blankTab 返回一个空标签。
func blankTab() data.TabConfig {
	return data.TabConfig{
		Prompts:   data.DefaultPrompts(),
		Sentences: data.DefaultSentences(),
	}
}
