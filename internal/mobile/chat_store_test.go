package mobile

import (
	"testing"
	"time"
)

func TestChatStorePersistsAndRestores(t *testing.T) {
	dir := t.TempDir()
	c := newChatStore(dir)
	id := c.Active().ID
	if id == "" {
		t.Fatal("default session missing")
	}

	c.Append(id, chatMsg{Role: "user", Content: "你好，这是第一条消息"})
	c.Append(id, chatMsg{Role: "assistant", Content: "收到"})

	reloaded := newChatStore(dir)
	got := reloaded.Active()
	if len(got.Messages) != 2 {
		t.Fatalf("messages = %d", len(got.Messages))
	}
	if got.Messages[0].Content != "你好，这是第一条消息" {
		t.Fatalf("restore content = %q", got.Messages[0].Content)
	}
	if got.Title == "新会话" || got.Title == "" {
		t.Fatalf("title should be derived from first user message, got %q", got.Title)
	}
	if got.ID != id {
		t.Fatalf("active id not restored: %s vs %s", got.ID, id)
	}
}

func TestChatStoreNewSwitchDelete(t *testing.T) {
	dir := t.TempDir()
	c := newChatStore(dir)
	first := c.Active().ID

	second := c.New()
	if second.ID == first {
		t.Fatal("new session should have new id")
	}
	if c.Active().ID != second.ID {
		t.Fatal("new session should become active")
	}

	c.Rename(second.ID, "会话二")
	if c.Active().Title != "会话二" {
		t.Fatalf("rename failed: %q", c.Active().Title)
	}

	c.SetActive(first)
	if c.Active().ID != first {
		t.Fatal("switch failed")
	}

	c.Delete(first)
	if c.Active().ID != second.ID {
		t.Fatalf("after delete active = %q", c.Active().ID)
	}

	list, _ := c.SessionList()
	if len(list) != 1 {
		t.Fatalf("sessions = %d", len(list))
	}

	c.Delete(second.ID)
	if len(c.Active().ID) == 0 {
		t.Fatal("deleting last session should recreate one")
	}
}

func TestChatStoreClearMessages(t *testing.T) {
	dir := t.TempDir()
	c := newChatStore(dir)
	id := c.Active().ID
	c.Append(id, chatMsg{Role: "user", Content: "x"})
	c.SetMessages(id, nil)
	if len(c.Active().Messages) != 0 {
		t.Fatal("messages not cleared")
	}
}

func TestTitleFrom(t *testing.T) {
	if titleFrom("") != "新会话" {
		t.Fatal("empty title")
	}
	long := "1234567890123456789012345"
	if titleFrom(long) == long {
		t.Fatal("long title should be truncated")
	}
}

// TestSessionListSortedByRecency 验证会话按最近使用排序。
//
// 之前 SessionList 直接复制原切片，新会话追加在末尾，
// 用户会话变多后很难找到刚刚聊过的对话。
func TestSessionListSortedByRecency(t *testing.T) {
	dir := t.TempDir()
	c := newChatStore(dir)

	// 手动构造三个更新时间不同的会话
	c.mu.Lock()
	c.sessions = []chatSession{
		{ID: "old", Title: "旧", UpdatedAt: time.Now().Add(-2 * time.Hour)},
		{ID: "mid", Title: "中", UpdatedAt: time.Now().Add(-1 * time.Hour)},
		{ID: "new", Title: "新", UpdatedAt: time.Now()},
	}
	c.activeID = "new"
	c.mu.Unlock()

	list, active := c.SessionList()
	if len(list) != 3 {
		t.Fatalf("sessions = %d", len(list))
	}
	if list[0].ID != "new" || list[1].ID != "mid" || list[2].ID != "old" {
		t.Fatalf("未按最近使用排序: %s %s %s", list[0].ID, list[1].ID, list[2].ID)
	}
	if active != "new" {
		t.Fatalf("active = %q", active)
	}
}

// TestSessionListPinnedFirst 验证置顶会话排在最前。
func TestSessionListPinnedFirst(t *testing.T) {
	dir := t.TempDir()
	c := newChatStore(dir)

	c.mu.Lock()
	c.sessions = []chatSession{
		{ID: "a", Title: "A", UpdatedAt: time.Now().Add(-time.Hour)},
		{ID: "b", Title: "B", UpdatedAt: time.Now(), Pinned: true},
	}
	c.mu.Unlock()

	list, _ := c.SessionList()
	if list[0].ID != "b" {
		t.Fatalf("置顶会话应在最前，实际 %q", list[0].ID)
	}
}

// TestTogglePinPersists 验证置顶状态可持久化。
func TestTogglePinPersists(t *testing.T) {
	dir := t.TempDir()
	c := newChatStore(dir)
	id := c.Active().ID

	if !c.TogglePin(id) {
		t.Fatal("第一次切换应置顶")
	}
	reloaded := newChatStore(dir)
	list, _ := reloaded.SessionList()
	if len(list) == 0 || !list[0].Pinned {
		t.Fatal("置顶状态未持久化")
	}
}

// TestSearchSessions 验证按标题与消息内容搜索。
func TestSearchSessions(t *testing.T) {
	dir := t.TempDir()
	c := newChatStore(dir)

	first := c.Active().ID
	c.Rename(first, "机器人计划")
	c.Append(first, chatMsg{Role: "user", Content: "关于天气的问题"})

	second := c.New()
	c.Rename(second.ID, "购物清单")
	c.Append(second.ID, chatMsg{Role: "user", Content: "买苹果和牛奶"})

	got, _ := c.SearchSessions("机器人")
	if len(got) != 1 || got[0].ID != first {
		t.Fatalf("按标题搜索失败: %+v", got)
	}
	got, _ = c.SearchSessions("牛奶")
	if len(got) != 1 || got[0].ID != second.ID {
		t.Fatalf("按内容搜索失败: %+v", got)
	}
	got, _ = c.SearchSessions("")
	if len(got) != 2 {
		t.Fatalf("空关键字应返回全部: %d", len(got))
	}
	got, _ = c.SearchSessions("不存在的关键字")
	if len(got) != 0 {
		t.Fatalf("无匹配应返回空: %+v", got)
	}
}
