package controllers

// 宿主机指标采集（dashboard 用）。
//
// 原版 /api/dashboard/info 的 39 个键里，有一半是宿主机的实时指标。复刻端此前
// 全部硬编码（cpu 10 / disk 70 / load 恒 0 / tcpCount 恒 0），既骗人也让面板失去
// 意义。这里读 /proc 与 Statfs 拿真实值；非 Linux 平台返回零值，不影响契约
// （键集合由 contract_test.go 保证）。
//
// 采集项与原版键的对应：
//
//	cpu        ← load1 / NumCPU × 100（无采样窗口，仅近似占用率）
//	disk       ← Statfs("/") 已用百分比
//	swap_mem   ← /proc/meminfo SwapTotal/SwapFree
//	virtual_mem← /proc/meminfo MemTotal/MemAvailable
//	io_recv/io_send ← /proc/net/dev 所有非 lo 接口的收发字节总数
//	tcp/udp    ← /proc/net/{tcp,tcp6,udp,udp6} 的条目数
//	icmp/icmpmsg ← /proc/net/snmp 的 Icmp InMsgs/OutMsgs
//	load       ← /proc/loadavg，序列化成 JSON 字符串（原版就是字符串）

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// hostMetrics dashboard 需要的宿主机指标。
type hostMetrics struct {
	Load       string // {"load1":x,"load5":y,"load15":z}
	CPU        int
	Disk       int
	SwapMem    int
	VirtualMem int
	IORecv     int64
	IOSend     int64
	TCP        int
	UDP        int
	ICMP       int
	ICMPMsg    int
}

// collectHostMetrics 采集指标；任何读取失败都用零值兜底（不影响面板可用性）。
func collectHostMetrics() hostMetrics {
	m := hostMetrics{Load: `{"load1":0,"load5":0,"load15":0}`}

	// load + cpu
	if b, err := os.ReadFile("/proc/loadavg"); err == nil {
		f := strings.Fields(string(b))
		if len(f) >= 3 {
			m.Load = `{"load1":` + f[0] + `,"load5":` + f[1] + `,"load15":` + f[2] + `}`
			if l1, err := strconv.ParseFloat(f[0], 64); err == nil {
				n := runtime.NumCPU()
				if n < 1 {
					n = 1
				}
				m.CPU = int(l1 / float64(n) * 100)
				if m.CPU > 100 {
					m.CPU = 100
				}
			}
		}
	}

	// disk
	var st syscall.Statfs_t
	if err := syscall.Statfs("/", &st); err == nil && st.Blocks > 0 {
		used := st.Blocks - st.Bavail
		m.Disk = int(used * 100 / st.Blocks)
	}

	// meminfo
	if b, err := os.ReadFile("/proc/meminfo"); err == nil {
		var total, avail, swapTotal, swapFree float64
		for _, line := range strings.Split(string(b), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			v, _ := strconv.ParseFloat(fields[1], 64)
			switch fields[0] {
			case "MemTotal:":
				total = v
			case "MemAvailable:":
				avail = v
			case "SwapTotal:":
				swapTotal = v
			case "SwapFree:":
				swapFree = v
			}
		}
		if total > 0 {
			m.VirtualMem = int((total - avail) * 100 / total)
		}
		if swapTotal > 0 {
			m.SwapMem = int((swapTotal - swapFree) * 100 / swapTotal)
		}
	}

	// net dev
	if b, err := os.ReadFile("/proc/net/dev"); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			i := strings.IndexByte(line, ':')
			if i < 0 {
				continue
			}
			name := strings.TrimSpace(line[:i])
			if name == "lo" || name == "" {
				continue
			}
			f := strings.Fields(line[i+1:])
			if len(f) < 9 {
				continue
			}
			rx, _ := strconv.ParseInt(f[0], 10, 64)
			tx, _ := strconv.ParseInt(f[8], 10, 64)
			m.IORecv += rx
			m.IOSend += tx
		}
	}

	// sockets
	m.TCP = countProcNetEntries("/proc/net/tcp") + countProcNetEntries("/proc/net/tcp6")
	m.UDP = countProcNetEntries("/proc/net/udp") + countProcNetEntries("/proc/net/udp6")

	// icmp
	if in, out, ok := readICMPCounters(); ok {
		m.ICMP, m.ICMPMsg = in, out
	}
	return m
}

// countProcNetEntries 统计 /proc/net/<file> 的条目数（首行是表头）。
func countProcNetEntries(path string) int {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	n := strings.Count(string(b), "\n")
	if n > 0 {
		n-- // 表头
	}
	return n
}

// readICMPCounters 从 /proc/net/snmp 读出 Icmp InMsgs / OutMsgs。
func readICMPCounters() (in, out int, ok bool) {
	b, err := os.ReadFile("/proc/net/snmp")
	if err != nil {
		return 0, 0, false
	}
	lines := strings.Split(string(b), "\n")
	for i := 0; i+1 < len(lines); i++ {
		if !strings.HasPrefix(lines[i], "Icmp:") {
			continue
		}
		hdr := strings.Fields(lines[i])
		val := strings.Fields(lines[i+1])
		if len(hdr) != len(val) {
			continue
		}
		for j, h := range hdr {
			switch h {
			case "InMsgs":
				in, _ = strconv.Atoi(val[j])
			case "OutMsgs":
				out, _ = strconv.Atoi(val[j])
			}
		}
		return in, out, true
	}
	return 0, 0, false
}
