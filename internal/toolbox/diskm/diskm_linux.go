//go:build linux

package diskm

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// collect 解析 /proc/diskstats，返回每个设备的累计读写字节。
func collect() map[string]Sample {
	out := map[string]Sample{}
	f, err := os.Open("/proc/diskstats")
	if err != nil {
		return out
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 14 {
			continue
		}
		name := fields[2]
		// 只保留物理设备：过滤掉分区（sda1）与内存设备（ram0/loop0）。
		if isPartitionName(name) {
			continue
		}
		readSectors, err1 := strconv.ParseUint(fields[5], 10, 64)
		writeSectors, err2 := strconv.ParseUint(fields[9], 10, 64)
		if err1 != nil || err2 != nil {
			continue
		}
		// 扇区固定 512 字节。
		out[name] = Sample{
			Device:     name,
			ReadBytes:  readSectors * 512,
			WriteBytes: writeSectors * 512,
		}
	}
	return out
}

// isPartitionName 判断是否是分区设备名（sda1 / nvme0n1p2 / mmcblk0p1）。
func isPartitionName(name string) bool {
	if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") ||
		strings.HasPrefix(name, "zram") || strings.HasPrefix(name, "dm-") {
		return true
	}
	digits := 0
	for i := len(name) - 1; i >= 0; i-- {
		c := name[i]
		if c < '0' || c > '9' {
			break
		}
		digits++
	}
	return digits > 0 && digits < len(name)
}
