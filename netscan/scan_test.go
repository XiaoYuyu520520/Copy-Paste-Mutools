package netscan

import "testing"

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
