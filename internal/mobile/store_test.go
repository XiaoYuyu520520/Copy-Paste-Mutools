package mobile

import (
	"testing"

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
