package c2engine

// 隧道数据面（端口转发）。
//
// 设计取舍：原版把隧道流量走 C2 链路的 LinkMsgChan 帧（见 kcp.go/channel_frame.go），
// 那要求控制通道是**长连接**且 agent 有异步读循环；复刻端的四条传输都是
// 请求/响应式（HTTP 轮询、KCP 轮询、WS 轮询、DNS 查询），直接搬运会退化成
// "每条数据等一个轮询周期"。
//
// 因此复刻端采用**反向端口转发**：agent 收到 tunnel_open 任务后主动向服务器的
// 隧道端口建一条独立 TCP 连接，握手带 magic + 通道号；服务器把这条连接与
// "公网进来的一条连接"配对，然后双向转发。好处：
//   - 与 C2 传输解耦（HTTP/KCP/WS/DNS 都能用，只要 agent 能出网到隧道端口）；
//   - 延迟与吞吐接近裸 TCP；
//   - 不依赖未恢复的 LinkMsgChan 语义。
//
// 握手（agent → 服务器隧道端口）：
//
//	"VST1" | chanID u32 LE | targetLen u16 LE | target
//
// 服务器按首字节判断：读到 "VST" 前缀 = agent 注册；否则视为公网客户端。
// 公网客户端与 agent 连接的配对顺序由 tunnel_open 下发的 chanID 决定。

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

const (
	// TunnelMagic 是 agent 注册隧道通道的握手前缀。
	TunnelMagic = "VST1"
	// tunnelPairTimeout 是公网连接等待 agent 接入的上限。
	tunnelPairTimeout = 15 * time.Second
	// tunnelPeekTimeout 是判断"agent 注册 vs 公网客户端"的短探测窗口。
	tunnelPeekTimeout = 300 * time.Millisecond
)

// TunnelServer 是一条隧道的服务端数据面。
type TunnelServer struct {
	mu       sync.Mutex
	id       int64
	port     int
	listener net.Listener
	// waiting 是"已接入但还没配对的 agent 通道"：chanID → conn。
	waiting map[uint32]net.Conn
	// pending 是"已接受但还没等到 agent 的公网连接"：chanID → conn。
	pending  map[uint32]net.Conn
	nextChan uint32
	stopCh   chan struct{}
	running  bool
	relayed  int64
	// OnOpen 在公网连接到达、分配好通道号后调用：由上层把 tunnel_open 任务
	// 下发给 agent（agent 收到后才来 dial 隧道端口）。
	OnOpen func(chanID uint32) error
}

// NewTunnelServer 创建隧道数据面。
func NewTunnelServer(id int64, port int) *TunnelServer {
	return &TunnelServer{
		id:       id,
		port:     port,
		waiting:  make(map[uint32]net.Conn),
		pending:  make(map[uint32]net.Conn),
		nextChan: 1,
		stopCh:   make(chan struct{}),
	}
}

// Start 在 port 上监听公网连接。
func (ts *TunnelServer) Start() error {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.running {
		return fmt.Errorf("tunnel %d already running", ts.id)
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", ts.port))
	if err != nil {
		return err
	}
	ts.listener = ln
	ts.running = true
	go ts.acceptLoop(ln)
	Logf("Tunnel %d listening on :%d", ts.id, ts.port)
	return nil
}

// Stop 停止监听并关闭所有挂起连接。
func (ts *TunnelServer) Stop() error {
	ts.mu.Lock()
	if !ts.running {
		ts.mu.Unlock()
		return nil
	}
	ts.running = false
	close(ts.stopCh)
	ln := ts.listener
	waiting := ts.waiting
	pending := ts.pending
	ts.waiting = make(map[uint32]net.Conn)
	ts.pending = make(map[uint32]net.Conn)
	ts.mu.Unlock()

	if ln != nil {
		_ = ln.Close()
	}
	for _, c := range waiting {
		_ = c.Close()
	}
	for _, c := range pending {
		_ = c.Close()
	}
	Logf("Tunnel %d stopped", ts.id)
	return nil
}

// RelayCount 返回已转发的字节数（用于面板展示）。
func (ts *TunnelServer) RelayCount() int64 {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.relayed
}

// NextChanID 分配一个通道号（供控制器下发给 agent）。
func (ts *TunnelServer) NextChanID() uint32 {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	id := ts.nextChan
	ts.nextChan++
	return id
}

func (ts *TunnelServer) acceptLoop(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ts.stopCh:
				return
			default:
			}
			continue
		}
		go ts.handleConn(conn)
	}
}

// bufferedConn 让"已经 peek 过"的连接仍能完整读出（避免丢字节）。
type bufferedConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *bufferedConn) Read(b []byte) (int, error) { return c.r.Read(b) }

// handleConn 判断这是 agent 注册还是公网客户端，并尝试配对。
//
// 判定用"短 peek"：agent 一建连就先发 magic，公网客户端通常不会立刻发 4 字节；
// peek 超时按公网处理。已读到的字节通过 bufferedConn 保留，不会丢。
func (ts *TunnelServer) handleConn(conn net.Conn) {
	br := bufio.NewReader(conn)
	_ = conn.SetReadDeadline(time.Now().Add(tunnelPeekTimeout))
	peek, err := br.Peek(len(TunnelMagic))
	_ = conn.SetReadDeadline(time.Time{})

	if err == nil && string(peek) == TunnelMagic {
		ts.handleAgentConn(conn, br)
		return
	}
	ts.handlePublicConn(&bufferedConn{Conn: conn, r: br})
}

// handleAgentConn 读取握手并把连接挂到 waiting 表。
func (ts *TunnelServer) handleAgentConn(conn net.Conn, br *bufio.Reader) {
	var hdr [len(TunnelMagic) + 4 + 2]byte
	if _, err := io.ReadFull(br, hdr[:]); err != nil {
		_ = conn.Close()
		return
	}
	chanID := binary.LittleEndian.Uint32(hdr[len(TunnelMagic) : len(TunnelMagic)+4])
	targetLen := int(binary.LittleEndian.Uint16(hdr[len(TunnelMagic)+4:]))
	target := make([]byte, targetLen)
	if targetLen > 0 {
		if _, err := io.ReadFull(br, target); err != nil {
			_ = conn.Close()
			return
		}
	}

	ts.mu.Lock()
	if pub, ok := ts.pending[chanID]; ok {
		delete(ts.pending, chanID)
		ts.mu.Unlock()
		ts.relay(pub, conn)
		return
	}
	ts.waiting[chanID] = conn
	ts.mu.Unlock()
	Logf("Tunnel %d: agent channel %d registered for %s", ts.id, chanID, string(target))
}

// handlePublicConn 把公网连接挂到 pending，等待 agent 接入。
func (ts *TunnelServer) handlePublicConn(conn net.Conn) {
	ts.mu.Lock()
	chanID := ts.nextChan
	ts.nextChan++
	if agentConn, ok := ts.waiting[chanID]; ok {
		delete(ts.waiting, chanID)
		ts.mu.Unlock()
		ts.relay(conn, agentConn)
		return
	}
	ts.pending[chanID] = conn
	cb := ts.OnOpen
	ts.mu.Unlock()

	// 通知上层把 tunnel_open 下发给 agent（agent 会带着这个 chanID 拨回来）。
	if cb != nil {
		if err := cb(chanID); err != nil {
			Logf("Tunnel %d: dispatch tunnel_open for chan %d failed: %v", ts.id, chanID, err)
			ts.mu.Lock()
			if c, ok := ts.pending[chanID]; ok && c == conn {
				delete(ts.pending, chanID)
				_ = c.Close()
			}
			ts.mu.Unlock()
			return
		}
	}

	// 等 agent 接入；超时则关闭，避免连接堆积。
	select {
	case <-time.After(tunnelPairTimeout):
		ts.mu.Lock()
		if c, ok := ts.pending[chanID]; ok && c == conn {
			delete(ts.pending, chanID)
			_ = c.Close()
		}
		ts.mu.Unlock()
	case <-ts.stopCh:
	}
}

// relay 双向转发两条连接，直到任一方向结束。
func (ts *TunnelServer) relay(a, b net.Conn) {
	Logf("Tunnel %d: relaying %s <-> %s", ts.id, a.RemoteAddr(), b.RemoteAddr())
	var wg sync.WaitGroup
	wg.Add(2)

	pipe := func(dst, src net.Conn) {
		defer wg.Done()
		n, _ := io.Copy(dst, src)
		ts.mu.Lock()
		ts.relayed += n
		ts.mu.Unlock()
		if tcp, ok := dst.(*net.TCPConn); ok {
			_ = tcp.CloseWrite()
		}
	}

	go pipe(a, b)
	go pipe(b, a)
	wg.Wait()
	_ = a.Close()
	_ = b.Close()
}

// ============================================================================
// 隧道管理器（全局单例）
// ============================================================================

var (
	tunnelServers   = make(map[int64]*TunnelServer)
	tunnelServersMu sync.Mutex
)

// StartTunnelServer 启动某条隧道的数据面（幂等）。
func StartTunnelServer(id int64, port int) (*TunnelServer, error) {
	tunnelServersMu.Lock()
	defer tunnelServersMu.Unlock()
	if ts, ok := tunnelServers[id]; ok && ts.running {
		return ts, nil
	}
	ts := NewTunnelServer(id, port)
	if err := ts.Start(); err != nil {
		return nil, err
	}
	tunnelServers[id] = ts
	return ts, nil
}

// StopTunnelServer 停止某条隧道的数据面。
func StopTunnelServer(id int64) {
	tunnelServersMu.Lock()
	ts := tunnelServers[id]
	delete(tunnelServers, id)
	tunnelServersMu.Unlock()
	if ts != nil {
		_ = ts.Stop()
	}
}

// GetTunnelServer 返回某条隧道的数据面（可能为 nil）。
func GetTunnelServer(id int64) *TunnelServer {
	tunnelServersMu.Lock()
	defer tunnelServersMu.Unlock()
	return tunnelServers[id]
}

// ErrTunnelNotRunning 表示隧道数据面未启动。
var ErrTunnelNotRunning = errors.New("tunnel not running")
