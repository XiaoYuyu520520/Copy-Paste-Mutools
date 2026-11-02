package lanscan

import (
	"testing"
	"time"

	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/netscan"
)

func netscanEvent(typ string) netscan.Event { return netscan.Event{Type: typ} }

func newTestOptions() netscan.Options { return netscan.Options{WorkerCount: 1} }

func TestSessionSubscribeUnsubscribe(t *testing.T) {
	sess := newSession(ScanRequest{CIDR: "192.168.0.0/30"}, newTestOptions())

	ch, unsub := sess.Subscribe()
	if n := sess.Subscribers(); n != 1 {
		t.Fatalf("Subscribers = %d, want 1", n)
	}

	sess.broadcastAll(netscanEvent("host"))
	select {
	case ev := <-ch:
		if ev.Type != "host" {
			t.Fatalf("event type = %q", ev.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("未收到广播事件")
	}

	unsub()
	if n := sess.Subscribers(); n != 0 {
		t.Fatalf("注销后 Subscribers = %d, want 0", n)
	}
	// 注销后频道应已关闭
	if _, ok := <-ch; ok {
		t.Fatal("注销后频道应关闭")
	}

	// 重复注销不应 panic
	unsub()
}

func TestSessionSubscribeAfterCloseReturnsClosedChan(t *testing.T) {
	sess := newSession(ScanRequest{CIDR: "192.168.0.0/30"}, newTestOptions())
	sess.Close()

	ch, unsub := sess.Subscribe()
	defer unsub()
	if _, ok := <-ch; ok {
		t.Fatal("会话已关闭时应返回已关闭的频道")
	}
}

func TestServerPurgeExpired(t *testing.T) {
	s := New(Assets(), ScanDefaults{})
	s.SetSessionTTL(time.Hour)

	live := newSession(ScanRequest{CIDR: "192.168.0.0/24"}, newTestOptions())
	s.sessions.Store(live.ID, live)
	// 让会话进入结束态并标记为很久以前结束。
	live.mu.Lock()
	live.status = "done"
	live.finish = time.Now().Add(-2 * time.Hour)
	live.mu.Unlock()

	s.purgeExpired()
	if _, ok := s.sessions.Load(live.ID); ok {
		t.Fatal("过期会话应被清理")
	}
}

func TestServerPurgeKeepsSubscribedSession(t *testing.T) {
	s := New(Assets(), ScanDefaults{})
	s.SetSessionTTL(time.Minute)

	sess := newSession(ScanRequest{CIDR: "192.168.0.0/24"}, newTestOptions())
	s.sessions.Store(sess.ID, sess)
	sess.mu.Lock()
	sess.status = "done"
	sess.finish = time.Now().Add(-time.Hour)
	sess.mu.Unlock()

	_, unsub := sess.Subscribe()
	defer unsub()

	s.purgeExpired()
	if _, ok := s.sessions.Load(sess.ID); !ok {
		t.Fatal("仍有订阅者的会话不应被清理")
	}
}
