package mcp

import (
	"bytes"
	"time"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"vshell/utils"
)

// newPipe 返回预置 stdin 的 Server，执行后返回逐行响应。
func runMCP(t *testing.T, reqs ...string) []map[string]interface{} {
	t.Helper()
	in := strings.NewReader(strings.Join(reqs, "\n") + "\n")
	var out bytes.Buffer
	srv := NewServer(&out)
	if err := srv.Serve(in); err != nil {
		t.Fatalf("serve: %v", err)
	}
	var resps []map[string]interface{}
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if line == "" {
			continue
		}
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("bad response line %q: %v", line, err)
		}
		resps = append(resps, m)
	}
	return resps
}

// initialize / tools/list / ping 三个基础方法。
func TestMCPHandshakeAndTools(t *testing.T) {
	resps := runMCP(t,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"ping"}`,
	)
	if len(resps) != 3 {
		t.Fatalf("got %d responses, want 3", len(resps))
	}
	res, _ := resps[0]["result"].(map[string]interface{})
	if res["protocolVersion"] != ProtocolVersion {
		t.Fatalf("protocolVersion = %v", res["protocolVersion"])
	}
	tools, _ := resps[1]["result"].(map[string]interface{})["tools"].([]interface{})
	if len(tools) < 10 {
		t.Fatalf("tools/list returned %d tools, want >= 10", len(tools))
	}
	// 每个工具必须有 name/description/inputSchema
	for _, raw := range tools {
		tl := raw.(map[string]interface{})
		for _, k := range []string{"name", "description", "inputSchema"} {
			if _, ok := tl[k]; !ok {
				t.Fatalf("tool %v missing %q", tl["name"], k)
			}
		}
	}
}

// 未知方法与坏 JSON 要返回标准错误码。
func TestMCPErrors(t *testing.T) {
	resps := runMCP(t,
		`{"jsonrpc":"2.0","id":1,"method":"no/such"}`,
		`not-json`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"nope","arguments":{}}}`,
	)
	if len(resps) != 3 {
		t.Fatalf("got %d responses", len(resps))
	}
	if e, _ := resps[0]["error"].(map[string]interface{}); e == nil || e["code"].(float64) != -32601 {
		t.Fatalf("unknown method error = %v", resps[0]["error"])
	}
	if e, _ := resps[1]["error"].(map[string]interface{}); e == nil || e["code"].(float64) != -32700 {
		t.Fatalf("parse error = %v", resps[1]["error"])
	}
	// 未知工具走 tools/call 的 isError 通道，而不是协议错误
	res, _ := resps[2]["result"].(map[string]interface{})
	if res == nil || res["isError"] != true {
		t.Fatalf("unknown tool result = %v", resps[2])
	}
}

// MCP 通过 /api/login 取 token（跨进程签名不一致的修复），再用它调工具。
func TestMCPLogsInForToken(t *testing.T) {
	var sawToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/login":
			_, _ = w.Write([]byte(`{"code":0,"result":{"token":"T-LOGIN"},"type":"success"}`))
		case "/api/dashboard/info":
			sawToken = r.Header.Get("Token")
			_, _ = w.Write([]byte(`{"code":0,"result":{"version":"4.9.3"},"type":"success"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	hostPort := strings.TrimPrefix(srv.URL, "http://")
	host, portStr, _ := strings.Cut(hostPort, ":")
	port, _ := strconv.Atoi(portStr)
	cfg := utils.GetFullSettings()
	prevIP, prevPort, prevUser, prevPass := cfg.WebIP, cfg.WebPort, cfg.WebUsername, cfg.WebPassword
	cfg.WebIP, cfg.WebPort, cfg.WebUsername, cfg.WebPassword = host, port, "admin", "pw"
	// 清掉可能缓存的 token，确保走登录路径
	tokenMu.Lock()
	tokenValue, tokenExpiry = "", time.Time{}
	tokenMu.Unlock()
	t.Cleanup(func() {
		cfg.WebIP, cfg.WebPort, cfg.WebUsername, cfg.WebPassword = prevIP, prevPort, prevUser, prevPass
		tokenMu.Lock()
		tokenValue, tokenExpiry = "", time.Time{}
		tokenMu.Unlock()
	})

	resps := runMCP(t, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"dashboard","arguments":{}}}`)
	res, _ := resps[0]["result"].(map[string]interface{})
	if res == nil || res["isError"] == true {
		t.Fatalf("dashboard via login token failed: %v", resps[0])
	}
	if sawToken != "T-LOGIN" {
		t.Fatalf("panel saw token %q, want T-LOGIN", sawToken)
	}
}

// 工具真正打到面板 API：用 httptest 假面板验证 dashboard 工具。
func TestMCPToolCallsPanelAPI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/dashboard/info" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Token") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","result":{"version":"4.9.3","clientCount":3},"type":"success"}`))
	}))
	defer srv.Close()

	hostPort := strings.TrimPrefix(srv.URL, "http://")
	host, portStr, _ := strings.Cut(hostPort, ":")
	port, _ := strconv.Atoi(portStr)
	cfg := utils.GetFullSettings()
	prevIP, prevPort := cfg.WebIP, cfg.WebPort
	cfg.WebIP, cfg.WebPort = host, port
	t.Cleanup(func() { cfg.WebIP, cfg.WebPort = prevIP, prevPort })

	resps := runMCP(t, `{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"dashboard","arguments":{}}}`)
	res, _ := resps[0]["result"].(map[string]interface{})
	if res == nil || res["isError"] == true {
		t.Fatalf("dashboard tool failed: %v", resps[0])
	}
	content := res["content"].([]interface{})[0].(map[string]interface{})
	text := content["text"].(string)
	if !strings.Contains(text, "4.9.3") || !strings.Contains(text, "clientCount") {
		t.Fatalf("dashboard output = %q", text)
	}
}

// MCP 资源：列表与读取（白名单外的 uri 必须拒绝）。
func TestMCPResources(t *testing.T) {
	resps := runMCP(t,
		`{"jsonrpc":"2.0","id":1,"method":"resources/list"}`,
		`{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"vshell://docs/STATUS.md"}}`,
		`{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"vshell://docs/../../etc/passwd"}}`,
	)
	list, _ := resps[0]["result"].(map[string]interface{})["resources"].([]interface{})
	if len(list) < 4 {
		t.Fatalf("resources/list returned %d items", len(list))
	}
	read, _ := resps[1]["result"].(map[string]interface{})["contents"].([]interface{})
	if len(read) != 1 || !strings.Contains(read[0].(map[string]interface{})["text"].(string), "还原状态") {
		t.Fatalf("resources/read failed: %v", resps[1])
	}
	if e, _ := resps[2]["error"].(map[string]interface{}); e == nil {
		t.Fatalf("path traversal must be rejected: %v", resps[2])
	}
}
