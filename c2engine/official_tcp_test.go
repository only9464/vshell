package c2engine

import (
	"bufio"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"testing"
	"time"
)

// fakeOfficialAgent 用与官方抓包一致的字节序列驱动服务端，验证：
// 认证 → 通道注册 → conf 上报 → 任务下发 → 结果回传。
type fakeAgent struct {
	conn net.Conn
	rd   *bufio.Reader
	salt string
}

func (a *fakeAgent) send(plain []byte) error {
	wire, err := FrameEncrypt(plain, a.salt)
	if err != nil {
		return err
	}
	_, err = a.conn.Write(wire)
	return err
}

func (a *fakeAgent) read() ([]byte, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(a.rd, hdr[:]); err != nil {
		return nil, err
	}
	n := int(binary.LittleEndian.Uint32(hdr[:]))
	body := make([]byte, n)
	if _, err := io.ReadFull(a.rd, body); err != nil {
		return nil, err
	}
	return Unframe(body, a.salt)
}

func (a *fakeAgent) version() error {
	// 与抓包一致：[u32 LE 5]["4.9.3"]，同一连接连发两条
	payload := []byte(OfficialVersion)
	rec := make([]byte, 4+len(payload))
	binary.LittleEndian.PutUint32(rec[:4], uint32(len(payload)))
	copy(rec[4:], payload)
	if err := a.send(rec); err != nil {
		return err
	}
	return a.send(rec)
}

func TestOfficialTCPHandshakeAndTask(t *testing.T) {
	srv := NewOfficialTCPServer("127.0.0.1:0", "VK-T", "")
	srv.Logf = func(string, ...interface{}) {}
	srv.ReadTimeout = 5 * time.Second
	if err := srv.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer srv.Stop()
	addr := srv.listener.Addr().String()

	// ---- conf 连接
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	agent := &fakeAgent{conn: conn, rd: bufio.NewReader(conn), salt: ""}
	if err := agent.version(); err != nil {
		t.Fatalf("version: %v", err)
	}
	chal, err := agent.read()
	if err != nil {
		t.Fatalf("read challenge: %v", err)
	}
	if string(chal) != OfficialAuthChallenge(OfficialVersion) {
		t.Fatalf("challenge = %q", chal)
	}
	if err := agent.send([]byte(OfficialAuthResponse("VK-T"))); err != nil {
		t.Fatalf("auth: %v", err)
	}
	if resp, err := agent.read(); err != nil || string(resp) != "sucs" {
		t.Fatalf("sucs = %q, %v", resp, err)
	}
	// 注册 conf 通道并上报 client 记录
	if err := agent.send([]byte("conf")); err != nil {
		t.Fatalf("conf: %v", err)
	}
	if ack, err := agent.read(); err != nil || len(ack) != 1 || ack[0] != 0x01 {
		t.Fatalf("conf ack = %v, %v", ack, err)
	}
	clientJSON := `{"Id":0,"HostName":"probe","OsName":"linux_amd64","UserName":"kali"}`
	if err := agent.send([]byte("conf \x01\x00\x00" + clientJSON)); err != nil {
		t.Fatalf("conf json: %v", err)
	}
	if _, err := agent.read(); err != nil { // \x01
		t.Fatalf("ack2: %v", err)
	}
	if sess, err := agent.read(); err != nil || len(sess) != 16 {
		t.Fatalf("session = %q, %v", sess, err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for len(srv.ClientInfo()) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := srv.ClientInfo()["HostName"]; got != "probe" {
		t.Fatalf("client info = %v", srv.ClientInfo())
	}

	// ---- main 连接（任务下发目标）
	mconn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial main: %v", err)
	}
	defer mconn.Close()
	main := &fakeAgent{conn: mconn, rd: bufio.NewReader(mconn), salt: ""}
	if err := main.version(); err != nil {
		t.Fatalf("main version: %v", err)
	}
	if _, err := main.read(); err != nil { // challenge
		t.Fatalf("main challenge: %v", err)
	}
	if err := main.send([]byte(OfficialAuthResponse("VK-T"))); err != nil {
		t.Fatalf("main auth: %v", err)
	}
	if _, err := main.read(); err != nil { // sucs
		t.Fatalf("main sucs: %v", err)
	}
	if err := main.send([]byte("main")); err != nil {
		t.Fatalf("main register: %v", err)
	}
	deadline = time.Now().Add(2 * time.Second)
	for {
		srv.mu.Lock()
		ok := srv.main != nil
		srv.mu.Unlock()
		if ok || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	srv.mu.Lock()
	hasMain := srv.main != nil
	srv.mu.Unlock()
	if !hasMain {
		t.Fatal("main channel not registered")
	}

	// ---- 服务端下发任务，假 agent 读记录并回结果
	done := make(chan struct{})
	go func() {
		defer close(done)
		// 任务由两条记录组成：header(7B) + body(4B len + json)
		hdr, err := main.read()
		if err != nil {
			return
		}
		if hdr[0] != officialMsgData {
			t.Errorf("task header type = %#x", hdr[0])
			return
		}
		body, err := main.read()
		if err != nil {
			return
		}
		n := int(binary.LittleEndian.Uint32(body[:4]))
		var task OfficialTask
		if err := json.Unmarshal(body[4:4+n], &task); err != nil {
			t.Errorf("task json: %v", err)
			return
		}
		if task.Req.Type != OfficialTaskExec || task.Req.Z1 != "id" {
			t.Errorf("task = %+v", task.Req)
			return
		}
		// 与抓包一致：03|chan|len 头记录 + [u32 len][文本] 体记录
		resText := "->|uid=0(root)|<-"
		rhdr := []byte{officialMsgData, 0x01, 0x00, 0x00, 0x00, byte(len(resText)), byte(len(resText) >> 8)}
		if err := main.send(rhdr); err != nil {
			t.Errorf("send result header: %v", err)
			return
		}
		rbody := make([]byte, 4+len(resText))
		binary.LittleEndian.PutUint32(rbody[:4], uint32(len(resText)))
		copy(rbody[4:], resText)
		_ = main.send(rbody)
	}()

	res, err := srv.RunCommand("id", 3*time.Second)
	if err != nil {
		t.Fatalf("RunCommand: %v", err)
	}
	if res != "->|uid=0(root)|<-" {
		t.Fatalf("result = %q", res)
	}
	<-done
}

func TestOfficialTCPServerRejectsBadAuth(t *testing.T) {
	srv := NewOfficialTCPServer("127.0.0.1:0", "VK-X", "")
	srv.Logf = func(string, ...interface{}) {}
	srv.ReadTimeout = 5 * time.Second
	if err := srv.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer srv.Stop()
	conn, err := net.Dial("tcp", srv.listener.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	a := &fakeAgent{conn: conn, rd: bufio.NewReader(conn)}
	if err := a.version(); err != nil {
		t.Fatalf("version: %v", err)
	}
	_, _ = a.read() // challenge
	_ = a.send([]byte(OfficialAuthResponse("WRONG-VKEY")))
	// 服务端应断开：读取应报错
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := a.read(); err == nil {
		t.Fatal("bad auth response should close the connection")
	}
}

func TestOfficialTCPSendTaskWithoutMain(t *testing.T) {
	srv := NewOfficialTCPServer("127.0.0.1:0", "VK-Y", "")
	srv.Logf = func(string, ...interface{}) {}
	if err := srv.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer srv.Stop()
	if _, err := srv.SendTask(NewOfficialTask(OfficialTaskExec, "id")); err == nil {
		t.Fatal("expected error when no main channel")
	}
	fmt.Fprint(io.Discard, "") // 保持 io 导入被使用
}

// conf 之后，agent 用 md5(会话) 认证后续连接（R58 抓包实证：
// md5("IVAoXA2etic7VgbY") == conn2 的应答 a7e788304e325a859b36f9063e8b4a88）。
func TestOfficialSessionAuthOnFollowupConn(t *testing.T) {
	srv := NewOfficialTCPServer("127.0.0.1:0", "SESSION-VK", "")
	srv.Logf = func(string, ...interface{}) {}
	srv.ReadTimeout = 5 * time.Second
	srv.AuthDelay = 0
	if err := srv.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer srv.Stop()
	addr := srv.listener.Addr().String()

	// --- 第一条连接：认证 + conf，拿会话
	c1, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial1: %v", err)
	}
	a1 := &fakeAgent{conn: c1, rd: bufio.NewReader(c1), salt: ""}
	if err := a1.version(); err != nil {
		t.Fatalf("ver1: %v", err)
	}
	if _, err := a1.read(); err != nil { // challenge
		t.Fatalf("challenge1: %v", err)
	}
	if err := a1.send([]byte(OfficialAuthResponse("SESSION-VK"))); err != nil {
		t.Fatalf("resp1: %v", err)
	}
	if got, err := a1.read(); err != nil || string(got) != "sucs" {
		t.Fatalf("sucs1 = %q, %v", got, err)
	}
	if err := a1.send([]byte("conf")); err != nil {
		t.Fatalf("conf: %v", err)
	}
	if _, err := a1.read(); err != nil { // 
		t.Fatalf("ack1: %v", err)
	}
	if err := a1.send([]byte(`conf &#1;&#0;&#0;{"HostName":"h"}`)); err != nil {
		t.Fatalf("conf json: %v", err)
	}
	if _, err := a1.read(); err != nil { // 
		t.Fatalf("ack2: %v", err)
	}
	sess, err := a1.read()
	if err != nil || len(sess) != 16 {
		t.Fatalf("session = %q, %v", sess, err)
	}
	c1.Close()

	// --- 第二条连接：用 md5(会话) 认证，并注册 main
	c2, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial2: %v", err)
	}
	defer c2.Close()
	a2 := &fakeAgent{conn: c2, rd: bufio.NewReader(c2), salt: ""}
	if err := a2.version(); err != nil {
		t.Fatalf("ver2: %v", err)
	}
	if _, err := a2.read(); err != nil { // challenge
		t.Fatalf("challenge2: %v", err)
	}
	sum := md5.Sum(sess)
	if err := a2.send([]byte(hex.EncodeToString(sum[:]))); err != nil {
		t.Fatalf("resp2: %v", err)
	}
	if got, err := a2.read(); err != nil || string(got) != "sucs" {
		t.Fatalf("sucs2 = %q, %v (session auth rejected)", got, err)
	}
	if err := a2.send([]byte("main")); err != nil {
		t.Fatalf("main: %v", err)
	}
	// 用 RunCommand 的"未注册 main"错误消失来间接确认 main 已注册
	deadline := time.Now().Add(2 * time.Second)
	registered := false
	for time.Now().Before(deadline) {
		srv.mu.Lock()
		registered = srv.main != nil
		srv.mu.Unlock()
		if registered {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !registered {
		t.Fatal("main channel not registered after session auth")
	}
}
