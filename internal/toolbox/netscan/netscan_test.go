package netscan

import (
	"testing"

	core "github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/netscan"
)

func TestParsePortsDefault(t *testing.T) {
	ports, err := parsePorts("")
	if err != nil {
		t.Fatalf("parsePorts(\"\"): %v", err)
	}
	if len(ports) == 0 {
		t.Fatal("parsePorts(\"\") returned empty list")
	}
}

func TestParsePortsExplicit(t *testing.T) {
	ports, err := parsePorts("22,80, 443")
	if err != nil {
		t.Fatalf("parsePorts: %v", err)
	}
	if len(ports) != 3 || ports[0] != 22 || ports[1] != 80 || ports[2] != 443 {
		t.Errorf("ports = %v, want [22 80 443]", ports)
	}
}

func TestParsePortsRejectsGarbage(t *testing.T) {
	if _, err := parsePorts("abc"); err == nil {
		t.Error("parsePorts(\"abc\") = nil error, want error")
	}
	if _, err := parsePorts("70000"); err == nil {
		t.Error("parsePorts(\"70000\") = nil error, want error")
	}
}

func TestCompareIP(t *testing.T) {
	if compareIP("192.168.1.2", "192.168.1.10") >= 0 {
		t.Error("compareIP(1.2, 1.10) should be < 0")
	}
	if compareIP("10.0.0.1", "10.0.0.1") != 0 {
		t.Error("compareIP of equal IPs should be 0")
	}
	if compareIP("10.0.0.5", "10.0.0.1") <= 0 {
		t.Error("compareIP(5, 1) should be > 0")
	}
}

func TestDedupeKeepsRicherEntry(t *testing.T) {
	hosts := []*core.Host{
		{IP: "1.1.1.1"},
		{IP: "1.1.1.1", Hostname: "host-a", MAC: "AA:BB:CC:DD:EE:FF"},
		{IP: "2.2.2.2"},
	}
	out := dedupe(hosts)
	if len(out) != 2 {
		t.Fatalf("got %d hosts, want 2", len(out))
	}
	for _, h := range out {
		if h.IP == "1.1.1.1" && h.Hostname != "host-a" {
			t.Errorf("kept the poorer entry for 1.1.1.1: %+v", h)
		}
	}
}
