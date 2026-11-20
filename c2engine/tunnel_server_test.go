package c2engine

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"testing"
	"time"
)

// 端到端：公网连接 → 服务器分配通道号 → OnOpen 回调 → 假 agent 拨回来握手 →
// 双向转发。这里不起 C2 传输，直接验证隧道数据面本身。
func TestTunnelServerRelaysBytes(t *testing.T) {
	// 目标服务：回显一行。
	target, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("target listen: %v", err)
	}
	defer target.Close()
	go func() {
		for {
			conn, err := target.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 64)
				n, _ := c.Read(buf)
				_, _ = c.Write(append([]byte("echo:"), buf[:n]...))
			}(conn)
		}
	}()

	// 选一个空闲端口给隧道。
	probe, _ := net.Listen("tcp", "127.0.0.1:0")
	port := probe.Addr().(*net.TCPAddr).Port
	_ = probe.Close()

	ts, err := StartTunnelServer(9001, port)
	if err != nil {
		t.Fatalf("start tunnel server: %v", err)
	}
	defer StopTunnelServer(9001)

	opened := make(chan uint32, 1)
	ts.OnOpen = func(chanID uint32) error {
		opened <- chanID
		return nil
	}

	// 公网客户端连进来。
	pub, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 2*time.Second)
	if err != nil {
		t.Fatalf("public dial: %v", err)
	}
	defer pub.Close()

	var chanID uint32
	select {
	case chanID = <-opened:
	case <-time.After(3 * time.Second):
		t.Fatal("OnOpen was not called for the public connection")
	}

	// 假 agent：带 magic + chanID 拨回来，然后自己连目标并转发。
	agentSide, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 2*time.Second)
	if err != nil {
		t.Fatalf("agent dial: %v", err)
	}
	defer agentSide.Close()

	tgt, err := net.DialTimeout("tcp", target.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatalf("dial target from agent: %v", err)
	}
	defer tgt.Close()

	hdr := append([]byte(TunnelMagic), byte(chanID), byte(chanID>>8), byte(chanID>>16), byte(chanID>>24))
	var lb [2]byte
	binary.LittleEndian.PutUint16(lb[:], uint16(len(target.Addr().String())))
	hdr = append(hdr, lb[:]...)
	hdr = append(hdr, target.Addr().String()...)
	if _, err := agentSide.Write(hdr); err != nil {
		t.Fatalf("handshake write: %v", err)
	}

	// agent 本地把两条连接对接起来。
	go func() { _, _ = io.Copy(agentSide, tgt) }()
	go func() { _, _ = io.Copy(tgt, agentSide) }()

	// 公网客户端发一行，期望拿到目标服务的回显。
	if _, err := pub.Write([]byte("hello\n")); err != nil {
		t.Fatalf("public write: %v", err)
	}
	_ = pub.SetReadDeadline(time.Now().Add(5 * time.Second))
	buf := make([]byte, 64)
	n, err := pub.Read(buf)
	if err != nil {
		t.Fatalf("public read: %v", err)
	}
	if got := string(buf[:n]); got != "echo:hello\n" {
		t.Fatalf("relayed data = %q, want %q", got, "echo:hello\n")
	}
}

// 没有 agent 接入时，公网连接必须超时关闭而不是永久挂住。
func TestTunnelServerClosesUnpairedPublicConn(t *testing.T) {
	probe, _ := net.Listen("tcp", "127.0.0.1:0")
	port := probe.Addr().(*net.TCPAddr).Port
	_ = probe.Close()

	ts, err := StartTunnelServer(9002, port)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer StopTunnelServer(9002)
	ts.OnOpen = func(uint32) error { return nil } // 不派发，模拟 agent 不来

	pub, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer pub.Close()

	// 配对超时是 15s：这里直接等它关闭（上限 20s）。
	_ = pub.SetReadDeadline(time.Now().Add(tunnelPairTimeout + 5*time.Second))
	buf := make([]byte, 1)
	if _, err := pub.Read(buf); err == nil {
		t.Fatalf("unpaired public connection should be closed by the server")
	}
}

