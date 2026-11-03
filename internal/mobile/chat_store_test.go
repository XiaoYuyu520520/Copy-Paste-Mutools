package mobile

import "testing"

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
