package c2engine

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

// newTestEngine 构造一个带临时 SQLite 的独立引擎（不碰全局单例）。
func newTestEngine(t *testing.T) *Engine {
	t.Helper()
	e := &Engine{
		config:      DefaultConfig(),
		clients:     make(map[int64]*Client),
		listeners:   make(map[int64]*Listener),
		tunnels:     make(map[int64]*Tunnel),
		hosts:       make(map[int64]*Host),
		tasks:       make(map[int64]*Task),
		vkeyIndex:   make(map[string]int64),
		blockedKeys: make(map[string]bool),
		storage:     NewStorage(t.TempDir() + "/data.db"),
	}
	t.Cleanup(func() { _ = e.storage.Close() })
	return e
}

// TestNewClientMarksOnline 覆盖 B0.1：签到即在线（原版客户端 +0x50 状态位）。
// 此前 Status 没有任何写入点，导致面板所有下发接口返回 "client is close"。
func TestNewClientMarksOnline(t *testing.T) {
	e := newTestEngine(t)

	c, err := e.NewClient("vk-online", "http", "127.0.0.1:1", "10.0.0.1", "u", "h", "linux", "p", "amd64")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if !c.IsOnline() || !c.Status {
		t.Fatalf("first check-in should mark the client online: %+v", c)
	}

	// 重新签到（同一 verify key）同样保持在线。
	c2, err := e.NewClient("vk-online", "http", "127.0.0.1:2", "", "", "", "", "", "")
	if err != nil {
		t.Fatalf("NewClient (re-checkin): %v", err)
	}
	if !c2.IsOnline() {
		t.Fatalf("re-check-in should keep the client online")
	}
	if c2.ID != c.ID {
		t.Fatalf("re-check-in should reuse the client id: %d != %d", c2.ID, c.ID)
	}
}

// TestSweepOffline 覆盖 B0.1 的另一半：静默超时后下线。
func TestSweepOffline(t *testing.T) {
	e := newTestEngine(t)
	c, err := e.NewClient("vk-sweep", "http", "127.0.0.1:3", "", "", "", "", "", "")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	// 未超时：不应下线。
	if n := e.SweepOffline(time.Hour); n != 0 {
		t.Fatalf("sweep with 1h timeout should not touch fresh clients, got %d", n)
	}
	if !c.IsOnline() {
		t.Fatalf("client went offline too early")
	}

	// 超时：下线。
	if n := e.SweepOffline(time.Nanosecond); n != 1 {
		t.Fatalf("sweep should mark 1 client offline, got %d", n)
	}
	if c.IsOnline() {
		t.Fatalf("client should be offline after sweep")
	}

	// 再签到一起复活。
	if _, err := e.NewClient("vk-sweep", "http", "127.0.0.1:3", "", "", "", "", "", ""); err != nil {
		t.Fatalf("re-checkin: %v", err)
	}
	if !c.IsOnline() {
		t.Fatalf("re-checkin should mark offline client online again")
	}
}

// TestDecodeAgentBody covers B0.2: the four body shapes the two sides produce.
func TestDecodeAgentBody(t *testing.T) {
	const salt = "e2e-salt"
	payload := []byte(`{"verify_key":"vk","hostname":"h"}`)
	frame := mustFrame(t, payload, salt)
	b64Frame := base64Std(frame)
	b64JSON := base64Std(payload)

	cases := []struct {
		name   string
		body   []byte
		framed bool
	}{
		{"plain json", payload, false},
		{"raw frame", frame, true},
		{"base64 frame", b64Frame, true},
		{"base64 json", b64JSON, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, framed, err := DecodeAgentBody(tc.body, salt)
			if err != nil {
				t.Fatalf("DecodeAgentBody: %v", err)
			}
			if framed != tc.framed {
				t.Fatalf("framed = %v, want %v", framed, tc.framed)
			}
			var m map[string]interface{}
			if err := json.Unmarshal(got, &m); err != nil {
				t.Fatalf("payload not JSON: %v (%q)", err, got)
			}
			if m["verify_key"] != "vk" {
				t.Fatalf("payload lost its content: %v", m)
			}
		})
	}

	if _, _, err := DecodeAgentBody([]byte("not a body"), salt); err == nil {
		t.Fatalf("garbage body should be rejected")
	}
}

// TestResultRequestAcceptsBothSpellings covers B0.5: agent 用 snake_case、
// 服务端结构体用首字母大写标签，Go 的 JSON 解码不做下划线归一化，结果此前
// 永远落不到对应任务上（CommandID 恒 0）。
func TestResultRequestAcceptsBothSpellings(t *testing.T) {
	cases := []string{
		`{"ClientID":7,"CommandID":9,"Result":"ok","Status":"completed","VerifyKey":"vk"}`,
		`{"client_id":7,"command_id":9,"result":"ok","status":"completed","verify_key":"vk"}`,
		`{"clientid":7,"commandid":9,"result":"ok","status":"completed"}`,
	}
	for _, body := range cases {
		var r ResultRequest
		if err := json.Unmarshal([]byte(body), &r); err != nil {
			t.Fatalf("unmarshal %s: %v", body, err)
		}
		if r.ClientID != 7 || r.CommandID != 9 || r.Result != "ok" || r.Status != "completed" {
			t.Fatalf("%s -> %+v", body, r)
		}
	}
}

// helpers ---------------------------------------------------------------

func mustFrame(t *testing.T, payload []byte, salt string) []byte {
	t.Helper()
	f, err := FrameEncrypt(payload, salt)
	if err != nil {
		t.Fatalf("FrameEncrypt: %v", err)
	}
	return f
}

func base64Std(b []byte) []byte {
	return []byte(base64.StdEncoding.EncodeToString(b))
}
