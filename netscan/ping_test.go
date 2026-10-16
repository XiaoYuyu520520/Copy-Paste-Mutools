package netscan

import (
	"testing"
	"time"
)

// GBK 中文系统 ping 输出示例（时间=…），ASCII 的 TTL/ms 仍是判活与取延迟的关键。
func TestRepliedDetectsAsciiMarkers(t *testing.T) {
	samples := []string{
		"来自 127.0.0.1 的回复: 字节=32 时间<1ms TTL=128",
		"Reply from 127.0.0.1: bytes=32 time=1ms TTL=64",
		"64 bytes from 127.0.0.1: icmp_seq=1 ttl=64 time=0.035 ms",
		"请求超时。",
		"Request timed out.",
		"一般故障。",
	}
	for i, s := range samples {
		want := i < 3
		if got := replied(s); got != want {
			t.Errorf("replied(%q) = %v, want %v", s, got, want)
		}
	}
}

func TestParseLatencyCrossLocale(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"Reply from 192.168.1.1: bytes=32 time=3ms TTL=64", 3 * time.Millisecond},
		{"来自 192.168.1.1 的回复: 字节=32 时间=3ms TTL=64", 3 * time.Millisecond},
		{"64 bytes from 1.1.1.1: icmp_seq=1 ttl=56 time=0.42 ms", 420 * time.Microsecond},
		{"无匹配输出", 0},
	}
	for _, c := range cases {
		if got := parseLatency(c.in); got != c.want {
			t.Errorf("parseLatency(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
