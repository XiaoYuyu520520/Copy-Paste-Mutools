package netscan

import (
	"fmt"
	"net"
	"sort"
	"strings"
)

// Interface describes a local network interface and its reachable subnets.
type Interface struct {
	Name       string   `json:"name"`
	Index      int      `json:"index"`
	MAC        string   `json:"mac"`
	IPs        []string `json:"ips"`
	Subnets    []string `json:"subnets"`
	IsUp       bool     `json:"is_up"`
	IsLoopback bool     `json:"is_loopback"`
	MTU        int      `json:"mtu"`
}

// ListInterfaces enumerates all non-loopback, up interfaces and converts each
// IPv4 address into a CIDR range so the UI can offer ready-to-scan subnets.
func ListInterfaces() ([]Interface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("枚举网卡失败: %w", err)
	}

	var out []Interface
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		var ips, subnets []string
		for _, a := range addrs {
			var ip net.IP
			var ipnet *net.IPNet
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
				ipnet = v
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() || ip.To4() == nil {
				continue
			}
			ips = append(ips, ip.String())
			if ipnet != nil {
				subnets = append(subnets, ipnet.String())
			}
		}
		if len(ips) == 0 {
			continue
		}
		sort.Strings(ips)
		sort.Strings(subnets)

		out = append(out, Interface{
			Name:       iface.Name,
			Index:      iface.Index,
			MAC:        iface.HardwareAddr.String(),
			IPs:        ips,
			Subnets:    subnets,
			IsUp:       iface.Flags&net.FlagUp != 0,
			IsLoopback: iface.Flags&net.FlagLoopback != 0,
			MTU:        iface.MTU,
		})
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("未发现可用的网络接口")
	}
	return out, nil
}

// FirstUsableSubnet returns the first IPv4 CIDR on the interface.
func (i Interface) FirstUsableSubnet() string {
	if len(i.Subnets) == 0 {
		return ""
	}
	return i.Subnets[0]
}

// DisplayName returns a friendly name for the interface.
func (i Interface) DisplayName() string {
	parts := []string{i.Name}
	if len(i.IPs) > 0 {
		parts = append(parts, strings.Join(i.IPs, ","))
	}
	return strings.Join(parts, " — ")
}
