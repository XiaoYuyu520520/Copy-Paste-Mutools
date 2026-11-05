package mobile

import (
	"crypto/rand"
	"encoding/hex"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/data"
)

// chatSession 是一段可持久化的 AI 对话。
type chatSession struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Messages  []chatMsg `json:"messages"`
	UpdatedAt time.Time `json:"updated_at"`
	Pinned    bool      `json:"pinned,omitempty"`
}

type chatStoreFile struct {
	ActiveID string        `json:"active_id"`
	Sessions []chatSession `json:"sessions"`
}

// chatStore 负责手机端 AI 会话的本地持久化。
//
// 每次完成一条消息都会落盘，进程被系统杀掉后再次打开即可恢复。
type chatStore struct {
	mu       sync.Mutex
	path     string
	activeID string
	sessions []chatSession
}

func newChatStore(dir string) *chatStore {
	c := &chatStore{path: filepath.Join(dir, "chat-sessions.json")}
	c.load()
	if len(c.sessions) == 0 {
		c.createLocked()
	}
	if _, ok := c.findLocked(c.activeID); !ok {
		c.activeID = c.sessions[0].ID
	}
	return c
}

func (c *chatStore) load() {
	var f chatStoreFile
	if _, err := data.LoadJSONWithBackup(c.path, &f); err != nil {
		return
	}
	c.activeID = f.ActiveID
	c.sessions = f.Sessions
}

func (c *chatStore) saveLocked() error {
	return data.SaveJSONAtomic(c.path, chatStoreFile{ActiveID: c.activeID, Sessions: c.sessions})
}

func (c *chatStore) save() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.saveLocked()
}

// SessionList 返回会话快照与当前会话 ID。
//
// 排序规则：置顶的会话排在最前，其余按最近使用时间（UpdatedAt）倒序，
// 因此「刚刚聊过」的会话总是能在列表最上面找到。
func (c *chatStore) SessionList() ([]chatSession, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]chatSession, len(c.sessions))
	copy(out, c.sessions)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Pinned != out[j].Pinned {
			return out[i].Pinned
		}
		if !out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].UpdatedAt.After(out[j].UpdatedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out, c.activeID
}

// SearchSessions 按关键字过滤会话（匹配标题与消息正文）。
func (c *chatStore) SearchSessions(keyword string) ([]chatSession, string) {
	list, active := c.SessionList()
	kw := strings.ToLower(strings.TrimSpace(keyword))
	if kw == "" {
		return list, active
	}
	out := make([]chatSession, 0, len(list))
	for _, s := range list {
		if sessionMatches(s, kw) {
			out = append(out, s)
		}
	}
	return out, active
}

// sessionMatches 判断会话是否命中关键字（标题或任一条消息正文）。
func sessionMatches(s chatSession, lowerKeyword string) bool {
	if strings.Contains(strings.ToLower(s.Title), lowerKeyword) {
		return true
	}
	for _, m := range s.Messages {
		if strings.Contains(strings.ToLower(m.Content), lowerKeyword) {
			return true
		}
	}
	return false
}

// TogglePin 置顶 / 取消置顶指定会话，返回新的置顶状态。
func (c *chatStore) TogglePin(id string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	pinned := false
	for i := range c.sessions {
		if c.sessions[i].ID == id {
			c.sessions[i].Pinned = !c.sessions[i].Pinned
			pinned = c.sessions[i].Pinned
			break
		}
	}
	_ = c.saveLocked()
	return pinned
}

// Active 返回当前会话快照。
func (c *chatStore) Active() chatSession {
	c.mu.Lock()
	defer c.mu.Unlock()
	if s, ok := c.findLocked(c.activeID); ok {
		return cloneSession(s)
	}
	return chatSession{}
}

// SetActive 切换当前会话并落盘。
func (c *chatStore) SetActive(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.findLocked(id); ok {
		c.activeID = id
		_ = c.saveLocked()
	}
}

// New 新建会话并切换为当前会话。
func (c *chatStore) New() chatSession {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.createLocked()
	_ = c.saveLocked()
	return cloneSession(s)
}

// Delete 删除会话；若删的是当前会话则切到剩下最新的一条。
func (c *chatStore) Delete(id string) chatSession {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.sessions {
		if c.sessions[i].ID == id {
			c.sessions = append(c.sessions[:i], c.sessions[i+1:]...)
			break
		}
	}
	if len(c.sessions) == 0 {
		c.createLocked()
	}
	if _, ok := c.findLocked(c.activeID); !ok {
		c.activeID = c.sessions[0].ID
	}
	_ = c.saveLocked()
	if s, ok := c.findLocked(c.activeID); ok {
		return cloneSession(s)
	}
	return chatSession{}
}

// Rename 重命名会话。
func (c *chatStore) Rename(id, title string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.sessions {
		if c.sessions[i].ID == id {
			c.sessions[i].Title = strings.TrimSpace(title)
			c.sessions[i].UpdatedAt = time.Now()
			break
		}
	}
	_ = c.saveLocked()
}

// Append 追加一条消息；首条用户消息自动生成会话标题。
func (c *chatStore) Append(id string, msg chatMsg) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.sessions {
		if c.sessions[i].ID != id {
			continue
		}
		c.sessions[i].Messages = append(c.sessions[i].Messages, msg)
		c.sessions[i].UpdatedAt = time.Now()
		if c.sessions[i].Title == "" || c.sessions[i].Title == "新会话" {
			if msg.Role == "user" {
				c.sessions[i].Title = titleFrom(msg.Content)
			}
		}
		break
	}
	_ = c.saveLocked()
}

// SetMessages 覆盖整个会话消息列表（用于清空）。
func (c *chatStore) SetMessages(id string, msgs []chatMsg) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.sessions {
		if c.sessions[i].ID != id {
			continue
		}
		c.sessions[i].Messages = append([]chatMsg(nil), msgs...)
		c.sessions[i].UpdatedAt = time.Now()
		break
	}
	_ = c.saveLocked()
}

func (c *chatStore) createLocked() chatSession {
	s := chatSession{
		ID:        newChatID(),
		Title:     "新会话",
		UpdatedAt: time.Now(),
	}
	c.sessions = append(c.sessions, s)
	c.activeID = s.ID
	return s
}

func (c *chatStore) findLocked(id string) (chatSession, bool) {
	for _, s := range c.sessions {
		if s.ID == id {
			return s, true
		}
	}
	return chatSession{}, false
}

func cloneSession(s chatSession) chatSession {
	out := s
	out.Messages = append([]chatMsg(nil), s.Messages...)
	return out
}

func titleFrom(text string) string {
	t := strings.TrimSpace(text)
	t = strings.ReplaceAll(t, "\n", " ")
	r := []rune(t)
	if len(r) > 18 {
		return string(r[:18]) + "…"
	}
	if t == "" {
		return "新会话"
	}
	return t
}

func newChatID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
