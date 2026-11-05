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

	saveTimer    *time.Timer
	onSaveResult func(error)
	saveState    SaveState
	saveErr      string
}

// newStore 载入配置；文件不存在或损坏时回退到 .bak 备份，再不行用默认配置。
func newStore(a fyne.App) *store {
	s := &store{path: configPath(a), cfg: data.DefaultConfig()}
	var cfg data.Config
	if _, err := data.LoadJSONWithBackup(s.path, &cfg); err == nil {
		s.cfg = normalize(cfg)
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

// cloneConfig 深拷贝配置，隔离切片与指针字段。
//
// 关键点：先整体复制 TabConfig（保留 Name / Mode / Separator /
// BlockNames / Collapsed 等所有标量字段），再深拷贝其中的切片，
// 避免手写字段列表时漏掉新加的字段。
func cloneConfig(c data.Config) data.Config {
	out := c

	out.Tabs = make([]data.TabConfig, len(c.Tabs))
	for i, t := range c.Tabs {
		tab := t // 先整体复制，保留 Name/Mode/Separator 等字段
		tab.Prompts = append([]data.Item(nil), t.Prompts...)
		tab.Sentences = append([]data.Item(nil), t.Sentences...)
		tab.BlockNames = append([]string(nil), t.BlockNames...)
		tab.Collapsed = append([]bool(nil), t.Collapsed...)
		out.Tabs[i] = tab
	}

	if c.AI != nil {
		ai := *c.AI
		out.AI = &ai
	}
	if c.Providers != nil {
		out.Providers = append([]data.AIProvider(nil), c.Providers...)
	}
	return out
}

// SaveState 表示配置的落盘状态，用于给用户明确反馈。
type SaveState int

const (
	// SaveStateIdle 表示当前没有待保存的改动。
	SaveStateIdle SaveState = iota
	// SaveStatePending 表示有改动尚未写盘（等待去抖定时器）。
	SaveStatePending
	// SaveStateSaving 表示正在写盘。
	SaveStateSaving
	// SaveStateSaved 表示最近一次写盘成功。
	SaveStateSaved
	// SaveStateFailed 表示写盘失败（details 为原因）。
	SaveStateFailed
)

// Label 返回状态的中文描述。
func (s SaveState) Label() string {
	switch s {
	case SaveStatePending:
		return "有未保存的改动"
	case SaveStateSaving:
		return "正在保存…"
	case SaveStateSaved:
		return "已保存"
	case SaveStateFailed:
		return "保存失败"
	default:
		return "就绪"
	}
}

// Save 立即落盘（原子写入，保留 .bak 备份）。
func (s *store) Save() error {
	s.setSaveState(SaveStateSaving, "")
	err := data.SaveJSONAtomic(s.path, s.Config())
	if err != nil {
		s.setSaveState(SaveStateFailed, err.Error())
		return err
	}
	s.setSaveState(SaveStateSaved, "")
	return nil
}

// SaveDebounced 合并短时间内的多次修改，1 秒后统一落盘。
//
// 编辑器每敲一个字符都会触发保存，直接写盘在手机上会造成明显卡顿。
// 定时器回调里的写入错误会通过 onSaveResult 回调交给界面提示，
// 不再静默吞掉（之前 `_ = s.Save()` 导致写盘失败用户毫不知情）。
func (s *store) SaveDebounced() {
	s.setSaveState(SaveStatePending, "")
	s.mu.Lock()
	if s.saveTimer != nil {
		s.saveTimer.Stop()
	}
	s.saveTimer = time.AfterFunc(time.Second, func() { s.runDebouncedSave() })
	fn := s.onSaveResult
	s.mu.Unlock()
	if fn != nil {
		fn(nil) // 通知界面刷新为「有未保存的改动」
	}
}

// runDebouncedSave 执行一次去抖写入，并把结果上报给已注册的回调。
func (s *store) runDebouncedSave() {
	err := s.Save()
	s.mu.Lock()
	fn := s.onSaveResult
	s.mu.Unlock()
	if fn != nil {
		fn(err)
	}
}

// SetSaveResultHandler 注册保存结果回调（nil 表示不关心）。
//
// 回调可能在定时器 goroutine 中触发，界面层需自行切回 UI 线程。
func (s *store) SetSaveResultHandler(fn func(error)) {
	s.mu.Lock()
	s.onSaveResult = fn
	s.mu.Unlock()
}

// SaveStatus 返回当前落盘状态与失败原因。
func (s *store) SaveStatus() (SaveState, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveState, s.saveErr
}

// setSaveState 记录落盘状态（不触发回调；回调由 runDebouncedSave 统一上报）。
func (s *store) setSaveState(state SaveState, detail string) {
	s.mu.Lock()
	s.saveState = state
	s.saveErr = detail
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
