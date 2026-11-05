package mobile

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/data"
)

func testStore() *store {
	return &store{cfg: data.DefaultConfig()}
}

func TestStoreConfigReturnsIsolatedSnapshot(t *testing.T) {
	s := testStore()
	s.cfg.AI = &data.AIConfig{BaseURL: "https://example.com/v1", Model: "m1"}

	snap := s.Config()
	snap.ActiveTab = 2
	snap.Tabs[0].Prompts[0].Text = "被篡改"
	snap.AI.BaseURL = "https://evil.example"

	got := s.Config()
	if got.ActiveTab == 2 {
		t.Error("修改快照不应影响内部 ActiveTab")
	}
	if got.Tabs[0].Prompts[0].Text == "被篡改" {
		t.Error("修改快照不应影响内部 Tabs 切片")
	}
	if got.AI == nil || got.AI.BaseURL != "https://example.com/v1" {
		t.Errorf("修改快照不应影响内部 AI 配置: %+v", got.AI)
	}
}

func TestStoreUpdateMutatesUnderLock(t *testing.T) {
	s := testStore()
	s.Update(func(c *data.Config) {
		c.ActiveTab = 1
		c.AI = &data.AIConfig{BaseURL: "https://api.test/v1", Model: "test-model"}
	})
	got := s.Config()
	if got.ActiveTab != 1 {
		t.Fatalf("ActiveTab = %d, want 1", got.ActiveTab)
	}
	if got.AI == nil || got.AI.Model != "test-model" {
		t.Fatalf("AI 未更新: %+v", got.AI)
	}
}

func TestStoreAIConfigDefault(t *testing.T) {
	s := testStore()
	s.cfg.AI = nil
	got := s.AIConfig()
	if got.BaseURL == "" || got.Model == "" {
		t.Fatalf("AI 默认值缺失: %+v", got)
	}

	s.cfg.AI = &data.AIConfig{BaseURL: "u", Model: "m"}
	got = s.AIConfig()
	got.Model = "changed"
	if s.cfg.AI.Model != "m" {
		t.Fatal("AIConfig 应返回副本")
	}
}

func TestStoreSnapshotKeepsAllTabFields(t *testing.T) {
	s := testStore()
	s.Update(func(c *data.Config) {
		c.Tabs = []data.TabConfig{{
			Name:       "我的标签",
			Mode:       data.MergeModeCustom,
			Separator:  "---\n",
			Prompts:    []data.Item{{Text: "P", Enabled: true}},
			Sentences:  []data.Item{{Text: "S", Enabled: true}},
			BlockNames: []string{"区块标题"},
			Collapsed:  []bool{true},
		}}
	})

	snap := s.Config()
	tab := snap.Tabs[0]
	if tab.Name != "我的标签" {
		t.Errorf("Name = %q, want 我的标签", tab.Name)
	}
	if tab.Mode != data.MergeModeCustom {
		t.Errorf("Mode = %q, want %q", tab.Mode, data.MergeModeCustom)
	}
	if tab.Separator != "---\n" {
		t.Errorf("Separator = %q", tab.Separator)
	}
	if len(tab.BlockNames) != 1 || tab.BlockNames[0] != "区块标题" {
		t.Errorf("BlockNames = %v", tab.BlockNames)
	}
	if len(tab.Collapsed) != 1 || !tab.Collapsed[0] {
		t.Errorf("Collapsed = %v", tab.Collapsed)
	}

	// 快照改动不得影响内部数据
	tab.Prompts[0].Text = "改过了"
	tab.BlockNames[0] = "改过了"
	tab.Collapsed[0] = false
	got := s.Config().Tabs[0]
	if got.Prompts[0].Text != "P" || got.BlockNames[0] != "区块标题" || !got.Collapsed[0] {
		t.Errorf("快照被共享: %+v", got)
	}
}

// TestStoreSaveStatusLifecycle 验证保存状态流转：未保存 → 已保存 / 失败。
func TestStoreSaveStatusLifecycle(t *testing.T) {
	dir := t.TempDir()
	s := &store{path: filepath.Join(dir, "mobile-config.json"), cfg: data.DefaultConfig()}

	var mu sync.Mutex
	var results []error
	done := make(chan struct{}, 4)
	s.SetSaveResultHandler(func(err error) {
		mu.Lock()
		results = append(results, err)
		mu.Unlock()
		done <- struct{}{}
	})

	state, _ := s.SaveStatus()
	if state != SaveStateIdle {
		t.Fatalf("初始状态 = %v", state)
	}

	s.SaveDebounced()
	// SaveDebounced 立即上报一次「有未保存的改动」
	mu.Lock()
	n := len(results)
	mu.Unlock()
	if n != 1 {
		t.Fatalf("SaveDebounced 应立即上报一次，实际 %d", n)
	}

	// 等待去抖定时器真正写入完成（第二次回调）
	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-done:
		case <-deadline:
			t.Fatal("去抖保存未完成")
		}
		mu.Lock()
		n = len(results)
		mu.Unlock()
		if n >= 2 {
			break
		}
	}

	state, detail := s.SaveStatus()
	if state != SaveStateSaved {
		t.Fatalf("保存后状态 = %v（%s）", state, detail)
	}
}

// TestStoreSaveErrorReported 验证写盘失败时回调能拿到错误。
func TestStoreSaveErrorReported(t *testing.T) {
	// 指向一个不可能是目录的路径，强制写盘失败。
	s := &store{path: filepath.Join(string(rune(0)), "impossible", "cfg.json"), cfg: data.DefaultConfig()}

	errCh := make(chan error, 1)
	s.SetSaveResultHandler(func(err error) {
		if err != nil {
			select {
			case errCh <- err:
			default:
			}
		}
	})

	if err := s.Save(); err == nil {
		t.Fatal("非法路径应保存失败")
	}
	state, detail := s.SaveStatus()
	if state != SaveStateFailed {
		t.Fatalf("失败状态 = %v", state)
	}
	if detail == "" {
		t.Fatal("失败原因不应为空")
	}
}

// TestSaveStateLabels 验证状态文案。
func TestSaveStateLabels(t *testing.T) {
	cases := map[SaveState]string{
		SaveStateIdle:    "就绪",
		SaveStatePending: "有未保存的改动",
		SaveStateSaving:  "正在保存…",
		SaveStateSaved:   "已保存",
		SaveStateFailed:  "保存失败",
	}
	for state, want := range cases {
		if got := state.Label(); got != want {
			t.Errorf("Label(%v) = %q, want %q", state, got, want)
		}
	}
}
