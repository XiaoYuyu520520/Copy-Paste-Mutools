package netscan

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestExpandCIDR_Basic(t *testing.T) {
	hosts, err := ExpandCIDR("192.168.1.0/24")
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 254 {
		t.Fatalf("期望 254 个可用地址，得到 %d", len(hosts))
	}
	if hosts[0] != "192.168.1.1" || hosts[len(hosts)-1] != "192.168.1.254" {
		t.Errorf("边界错误 first=%s last=%s", hosts[0], hosts[len(hosts)-1])
	}
}

func TestExpandCIDR_Invalid(t *testing.T) {
	for _, cidr := range []string{"", "192.168.1.1", "::1/64", "300.1.1.0/24", "192.168.1.0/99"} {
		if _, err := ExpandCIDR(cidr); err == nil {
			t.Errorf("%q 应返回错误", cidr)
		}
	}
	if _, err := ExpandCIDR("10.0.0.0/8"); err == nil {
		t.Error("超大子网 /8 应返回错误")
	}
}

func TestProbeTCP_UnusedPortClosed(t *testing.T) {
	// 端口 1 一般无服务；此处仅验证函数不 panic 且快速返回
	_ = ProbeTCPPorts(t.Context(), "127.0.0.1", []int{1}, 150, 2)
}

// 取消后 Scan 必须等待在途探测结束：返回后不应再有事件产生。
func TestScanCancelWaitsForWorkers(t *testing.T) {
	sc := New(Options{
		WorkerCount:    8,
		UseTCPProbe:    true,
		ProbePorts:     []int{1, 2, 3},
		PingTimeout:    200 * time.Millisecond,
		TCPPortTimeout: 200 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	var mu sync.Mutex
	var events []Event
	done := make(chan struct{})

	go func() {
		defer close(done)
		_ = sc.Scan(ctx, "192.0.2.0/29", func(ev Event) {
			mu.Lock()
			events = append(events, ev)
			mu.Unlock()
		})
	}()

	time.Sleep(30 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Scan 未在取消后返回")
	}

	// 记录返回时的快照，稍后再等一会儿确认没有新事件冒出来。
	mu.Lock()
	count := len(events)
	mu.Unlock()
	time.Sleep(300 * time.Millisecond)
	mu.Lock()
	after := len(events)
	mu.Unlock()
	if after != count {
		t.Fatalf("Scan 返回后仍有事件产生: %d -> %d", count, after)
	}
}

// 同一个 Scanner 并发发起两次扫描时，第二次必须被拒绝，避免事件交错。
func TestScanRejectsConcurrentRun(t *testing.T) {
	sc := New(Options{
		WorkerCount:    4,
		UseTCPProbe:    true,
		ProbePorts:     []int{1, 2, 3},
		PingTimeout:    300 * time.Millisecond,
		TCPPortTimeout: 300 * time.Millisecond,
	})

	var mu sync.Mutex
	first := make(chan error, 1)
	go func() {
		first <- sc.Scan(context.Background(), "192.0.2.0/28", func(Event) {})
	}()

	time.Sleep(40 * time.Millisecond)
	err := sc.Scan(context.Background(), "192.0.2.0/28", func(Event) {})
	if err == nil {
		t.Fatal("并发第二次扫描应被拒绝")
	}
	mu.Lock()
	mu.Unlock()
}

// 事件应带扫描编号，便于调用方隔离旧任务。
func TestScanEventsCarryScanID(t *testing.T) {
	sc := New(Options{WorkerCount: 2, UseTCPProbe: true, ProbePorts: []int{1}, PingTimeout: 100 * time.Millisecond})
	var ids []uint64
	var mu sync.Mutex
	_ = sc.Scan(context.Background(), "192.0.2.0/30", func(ev Event) {
		mu.Lock()
		ids = append(ids, ev.ScanID)
		mu.Unlock()
	})
	mu.Lock()
	defer mu.Unlock()
	if len(ids) == 0 {
		t.Fatal("没有事件")
	}
	for _, id := range ids {
		if id == 0 {
			t.Fatal("事件缺少 ScanID")
		}
	}
}
