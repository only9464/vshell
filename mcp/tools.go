package mcp

// MCP 工具实现：全部通过本机面板 API 调用，与前端/CLI 走同一条业务路径。
//
// 为什么走 HTTP 而不是直接调引擎：面板的鉴权、参数校验、错误文案、超时处理
// 都在控制器里，MCP 复用它们可以避免"同一功能两套语义"。token 用配置里的
// 用户名现场签发（utils.GenerateToken），不落盘、不外传。

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"vshell/utils"
)

// Tool 是一个 MCP 工具定义。
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
	handler     func(args map[string]interface{}) (string, bool)
}

// builtinTools 返回工具清单。
func builtinTools() []Tool {
	tools := []Tool{
		{
			Name:        "list_clients",
			Description: "列出所有受控端（客户端），含在线状态、主机名、系统、通道与地址。",
			InputSchema: obj(map[string]interface{}{}),
			handler: func(args map[string]interface{}) (string, bool) {
				return apiJSON("POST", "/client/list", map[string]interface{}{"page": 1, "pageSize": 200})
			},
		},
		{
			Name:        "list_listeners",
			Description: "列出监听器（模式、监听地址、Vkey、运行状态）。",
			InputSchema: obj(map[string]interface{}{}),
			handler: func(args map[string]interface{}) (string, bool) {
				return apiJSON("POST", "/listener/list", map[string]interface{}{"page": 1, "pageSize": 200})
			},
		},
		{
			Name:        "create_listener",
			Description: "创建监听器（创建后自动启动）。mode: http/https/kcp/websocket/dns/cdn_websocket。",
			InputSchema: obj(map[string]interface{}{
				"mode":          map[string]interface{}{"type": "string", "description": "传输模式"},
				"vkey":          map[string]interface{}{"type": "string", "description": "监听器密钥"},
				"host":          map[string]interface{}{"type": "string", "description": "监听地址，默认 127.0.0.1"},
				"port":          map[string]interface{}{"type": "integer", "description": "监听端口"},
				"remark":        map[string]interface{}{"type": "string"},
				"dns_domain":    map[string]interface{}{"type": "string", "description": "DNS 模式的域名后缀"},
				"ping_interval": map[string]interface{}{"type": "integer", "description": "心跳秒数"},
			}, "mode", "vkey"),
			handler: func(a map[string]interface{}) (string, bool) {
				host := str(a, "host", "127.0.0.1")
				return apiJSON("POST", "/listener/add", map[string]interface{}{
					"Mode":         str(a, "mode", "http"),
					"Vkey":         str(a, "vkey", ""),
					"Host":         host,
					"Port":         num(a, "port", 0),
					"Remark":       str(a, "remark", "mcp"),
					"DNSDomain":    str(a, "dns_domain", ""),
					"PingInterval": num(a, "ping_interval", 1),
				})
			},
		},
		{
			Name:        "run_command",
			Description: "在指定客户端上执行一条 shell 命令并返回输出（同步等待，最多 20 秒）。",
			InputSchema: obj(map[string]interface{}{
				"client_id": map[string]interface{}{"type": "integer", "description": "客户端 ID"},
				"command":   map[string]interface{}{"type": "string", "description": "要执行的命令"},
			}, "client_id", "command"),
			handler: func(a map[string]interface{}) (string, bool) {
				return apiJSON("POST", "/terminal/shell", map[string]interface{}{
					"id":      num(a, "client_id", 0),
					"command": str(a, "command", ""),
				})
			},
		},
		{
			Name:        "list_files",
			Description: "列出客户端上某个目录的内容。",
			InputSchema: obj(map[string]interface{}{
				"client_id": map[string]interface{}{"type": "integer"},
				"path":      map[string]interface{}{"type": "string", "description": "目录路径"},
			}, "client_id", "path"),
			handler: func(a map[string]interface{}) (string, bool) {
				return apiJSON("POST", "/file/ls", map[string]interface{}{
					"id": num(a, "client_id", 0), "path": str(a, "path", "/"),
				})
			},
		},
		{
			Name:        "read_file",
			Description: "读取客户端上的文本文件内容。",
			InputSchema: obj(map[string]interface{}{
				"client_id": map[string]interface{}{"type": "integer"},
				"path":      map[string]interface{}{"type": "string", "description": "完整文件路径"},
			}, "client_id", "path"),
			handler: func(a map[string]interface{}) (string, bool) {
				dir, name := splitPath(str(a, "path", ""))
				return apiJSON("POST", "/file/cat", map[string]interface{}{
					"id": num(a, "client_id", 0), "path": dir, "target": name,
				})
			},
		},
		{
			Name:        "write_file",
			Description: "把文本内容写入客户端上的文件（覆盖）。",
			InputSchema: obj(map[string]interface{}{
				"client_id": map[string]interface{}{"type": "integer"},
				"path":      map[string]interface{}{"type": "string"},
				"content":   map[string]interface{}{"type": "string"},
			}, "client_id", "path", "content"),
			handler: func(a map[string]interface{}) (string, bool) {
				dir, name := splitPath(str(a, "path", ""))
				return apiJSON("POST", "/file/edit", map[string]interface{}{
					"id": num(a, "client_id", 0), "path": dir, "target": name,
					"content": str(a, "content", ""),
				})
			},
		},
		{
			Name:        "upload_file",
			Description: "把本机文件上传到客户端指定目录（multipart → agent 落盘）。",
			InputSchema: obj(map[string]interface{}{
				"client_id":  map[string]interface{}{"type": "integer"},
				"local_path": map[string]interface{}{"type": "string", "description": "服务器本机文件路径"},
				"remote_dir": map[string]interface{}{"type": "string", "description": "客户端目标目录"},
			}, "client_id", "local_path", "remote_dir"),
			handler: func(a map[string]interface{}) (string, bool) {
				return apiUpload(num(a, "client_id", 0), str(a, "local_path", ""), str(a, "remote_dir", "/tmp"))
			},
		},
		{
			Name:        "download_file",
			Description: "从客户端下载文件到服务器本机（返回保存路径与字节数）。",
			InputSchema: obj(map[string]interface{}{
				"client_id":   map[string]interface{}{"type": "integer"},
				"remote_path": map[string]interface{}{"type": "string"},
				"local_path":  map[string]interface{}{"type": "string", "description": "保存到本机的路径"},
			}, "client_id", "remote_path", "local_path"),
			handler: func(a map[string]interface{}) (string, bool) {
				return apiDownload(num(a, "client_id", 0), str(a, "remote_path", ""), str(a, "local_path", ""))
			},
		},
		{
			Name:        "dashboard",
			Description: "获取服务端概览（客户端/监听器计数、CPU/磁盘、版本、许可）。",
			InputSchema: obj(map[string]interface{}{}),
			handler: func(args map[string]interface{}) (string, bool) {
				return apiJSON("GET", "/dashboard/info", nil)
			},
		},
		{
			Name:        "list_tunnels",
			Description: "列出隧道（端口转发）配置。",
			InputSchema: obj(map[string]interface{}{}),
			handler: func(args map[string]interface{}) (string, bool) {
				return apiJSON("POST", "/tunnel/list", map[string]interface{}{"page": 1, "pageSize": 200})
			},
		},
		{
			Name:        "start_tunnel",
			Description: "创建并启动一条隧道：服务器在 port 监听，流量经 agent 转发到 target。",
			InputSchema: obj(map[string]interface{}{
				"client_id":   map[string]interface{}{"type": "integer"},
				"port":        map[string]interface{}{"type": "integer", "description": "服务器监听端口"},
				"target":      map[string]interface{}{"type": "string", "description": "agent 侧可达的目标 host:port"},
				"server_addr": map[string]interface{}{"type": "string", "description": "agent 拨号用的服务器地址"},
			}, "client_id", "port", "target"),
			handler: func(a map[string]interface{}) (string, bool) {
				if text, bad := apiJSON("POST", "/tunnel/add", map[string]interface{}{
					"clientId": num(a, "client_id", 0),
					"Port":     num(a, "port", 0),
					"Mode":     "tcp",
					"Target":   str(a, "target", ""),
					"Remark":   "mcp",
				}); bad {
					return text, true
				}
				// 找出刚创建的那条（按端口匹配），再启动。
				list, _ := apiRaw("POST", "/tunnel/list", map[string]interface{}{"page": 1, "pageSize": 200})
				id := findTunnelID(list, num(a, "port", 0))
				if id == 0 {
					return "tunnel created but id not found", true
				}
				addr := str(a, "server_addr", fmt.Sprintf("127.0.0.1:%d", num(a, "port", 0)))
				return apiJSON("POST", "/tunnel/start", map[string]interface{}{"id": id, "serverAddr": addr})
			},
		},
		{
			Name:        "screenshot",
			Description: "抓取客户端屏幕截图，保存为 PNG 到服务器本机。",
			InputSchema: obj(map[string]interface{}{
				"client_id":  map[string]interface{}{"type": "integer"},
				"local_path": map[string]interface{}{"type": "string", "description": "PNG 保存路径"},
				"quality":    map[string]interface{}{"type": "string", "description": "big/normal/small"},
			}, "client_id", "local_path"),
			handler: func(a map[string]interface{}) (string, bool) {
				return apiScreenshot(num(a, "client_id", 0), str(a, "local_path", ""), str(a, "quality", "normal"))
			},
		},
	}
	return tools
}

// callTool 按名字执行工具，返回文本与是否出错。
func (s *Server) callTool(name string, args map[string]interface{}) (string, bool) {
	for _, t := range s.tools {
		if t.Name == name {
			if t.handler == nil {
				return "tool has no handler", true
			}
			return t.handler(args)
		}
	}
	return "unknown tool: " + name, true
}

// ---- 参数辅助 ----

func obj(props map[string]interface{}, required ...string) map[string]interface{} {
	m := map[string]interface{}{"type": "object", "properties": props}
	if len(required) > 0 {
		m["required"] = required
	}
	return m
}

func str(a map[string]interface{}, key, def string) string {
	if v, ok := a[key].(string); ok && v != "" {
		return v
	}
	return def
}

func num(a map[string]interface{}, key string, def int) int {
	switch v := a[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case json.Number:
		if n, err := v.Int64(); err == nil {
			return int(n)
		}
	}
	return def
}

func splitPath(p string) (dir, name string) {
	name = filepath.Base(p)
	dir = filepath.Dir(p)
	if dir == "." {
		dir = "/"
	}
	return dir, name
}

// ---- HTTP 调用 ----

// baseURL 返回本机面板地址（MCP 与面板同机同端口）。
func baseURL() string {
	cfg := utils.GetFullSettings()
	ip := cfg.WebIP
	if ip == "" || ip == "0.0.0.0" || ip == "::" {
		ip = "127.0.0.1"
	}
	return fmt.Sprintf("http://%s:%d/api", ip, cfg.WebPort)
}

// tokenCache 缓存面板登录拿到的 JWT。
var (
	tokenMu     sync.Mutex
	tokenValue  string
	tokenExpiry time.Time
)

// token 返回可用的面板 token。
//
// 优先**真正登录一次**（/api/login）：登录由服务端自己签发，能避免"本地按配置
// 再算一遍签名"与服务端密钥不一致的问题（实测：MCP 进程与面板进程的 JWT 密钥
// 可能不同，客户端自签的 token 会被 401）。只有在密码为空时才回落到本地签发。
func token() (string, error) {
	tokenMu.Lock()
	defer tokenMu.Unlock()
	if tokenValue != "" && time.Now().Before(tokenExpiry) {
		return tokenValue, nil
	}

	cfg := utils.GetFullSettings()
	if cfg.WebPassword != "" {
		body, _ := json.Marshal(map[string]string{"username": cfg.WebUsername, "password": cfg.WebPassword})
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Post(baseURL()+"/login", "application/json", bytes.NewReader(body))
		if err == nil {
			defer resp.Body.Close()
			var env struct {
				Result struct {
					Token string `json:"token"`
				} `json:"result"`
			}
			if json.NewDecoder(resp.Body).Decode(&env) == nil && env.Result.Token != "" {
				tokenValue = env.Result.Token
				tokenExpiry = time.Now().Add(20 * time.Hour)
				return tokenValue, nil
			}
		}
	}

	tok, err := utils.GenerateToken(cfg.WebUsername)
	if err != nil {
		return "", err
	}
	tokenValue = tok
	tokenExpiry = time.Now().Add(10 * time.Minute)
	return tok, nil
}

func doRequest(method, path string, body interface{}) (*http.Response, error) {
	resp, err := doRequestOnce(method, path, body)
	if err == nil && resp != nil && resp.StatusCode == http.StatusUnauthorized {
		// 缓存的 token 失效（改密码 / 服务端换了 JWT 密钥）：清缓存重登一次
		resp.Body.Close()
		invalidateToken()
		return doRequestOnce(method, path, body)
	}
	return resp, err
}

// invalidateToken 清掉缓存的 token（下次调用会重新登录）。
func invalidateToken() {
	tokenMu.Lock()
	tokenValue = ""
	tokenExpiry = time.Time{}
	tokenMu.Unlock()
}

// doRequestOnce 发一次请求（不复用 401 重试逻辑）。
func doRequestOnce(method, path string, body interface{}) (*http.Response, error) {
	tok, err := token()
	if err != nil {
		return nil, err
	}
	url := baseURL() + path
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Token", tok)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: 90 * time.Second}
	return client.Do(req)
}

// apiRaw 返回面板的完整信封（code/message/result）。
func apiRaw(method, path string, body interface{}) (map[string]interface{}, error) {
	resp, err := doRequest(method, path, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var env map[string]interface{}
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("non-JSON response (%d): %s", resp.StatusCode, truncate(data))
	}
	return env, nil
}

// apiJSON 调用并返回格式化 JSON 文本；业务错误（code != 0）标记为失败。
func apiJSON(method, path string, body interface{}) (string, bool) {
	env, err := apiRaw(method, path, body)
	if err != nil {
		return err.Error(), true
	}
	if code, _ := env["code"].(float64); code != 0 {
		msg, _ := env["message"].(string)
		return msg, true
	}
	result, ok := env["result"]
	if !ok {
		result = env
	}
	return textResult(result)
}

func apiUpload(clientID int, localPath, remoteDir string) (string, bool) {
	f, err := os.Open(localPath)
	if err != nil {
		return "open local file: " + err.Error(), true
	}
	defer f.Close()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("id", fmt.Sprintf("%d", clientID))
	_ = mw.WriteField("path", remoteDir)
	part, err := mw.CreateFormFile("file", filepath.Base(localPath))
	if err != nil {
		return err.Error(), true
	}
	if _, err := io.Copy(part, f); err != nil {
		return err.Error(), true
	}
	_ = mw.Close()

	tok, err := token()
	if err != nil {
		return err.Error(), true
	}
	req, err := http.NewRequest("POST", baseURL()+"/file/upload", &buf)
	if err != nil {
		return err.Error(), true
	}
	req.Header.Set("Token", tok)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err.Error(), true
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	var env map[string]interface{}
	if err := json.Unmarshal(data, &env); err != nil {
		return string(data), true
	}
	if code, _ := env["code"].(float64); code != 0 {
		msg, _ := env["message"].(string)
		return msg, true
	}
	abs, _ := filepath.Abs(localPath)
	return fmt.Sprintf("uploaded %s → %s/%s", abs, remoteDir, filepath.Base(localPath)), false
}

func apiDownload(clientID int, remotePath, localPath string) (string, bool) {
	dir, name := splitPath(remotePath)
	tok, err := token()
	if err != nil {
		return err.Error(), true
	}
	url := fmt.Sprintf("%s/file/downloadtobrowser?id=%d&path=%s&target=%s&token=%s",
		baseURL(), clientID, urlEncode(dir), urlEncode(name), tok)
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err.Error(), true
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Sprintf("download failed: HTTP %d %s", resp.StatusCode, truncate(data)), true
	}
	out := localPath
	if out == "" {
		out = name
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil && filepath.Dir(out) != "." {
		return err.Error(), true
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err.Error(), true
	}
	if err := os.WriteFile(out, data, 0o644); err != nil {
		return err.Error(), true
	}
	abs, _ := filepath.Abs(out)
	return fmt.Sprintf("saved %d bytes → %s", len(data), abs), false
}

func apiScreenshot(clientID int, localPath, quality string) (string, bool) {
	tok, err := token()
	if err != nil {
		return err.Error(), true
	}
	url := fmt.Sprintf("%s/screenshot/get?id=%d&quality=%s&token=%s", baseURL(), clientID, urlEncode(quality), tok)
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err.Error(), true
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || len(body) == 0 || body[0] == '{' {
		return "screenshot failed: " + truncate(body), true
	}
	// 面板返回裸 base64 PNG。
	raw, err := decodeBase64(body)
	if err != nil {
		return "decode screenshot: " + err.Error(), true
	}
	out := localPath
	if out == "" {
		out = fmt.Sprintf("screenshot_%d_%d.png", clientID, time.Now().Unix())
	}
	if err := os.WriteFile(out, raw, 0o644); err != nil {
		return err.Error(), true
	}
	abs, _ := filepath.Abs(out)
	return fmt.Sprintf("saved %d bytes PNG → %s", len(raw), abs), false
}

func findTunnelID(env map[string]interface{}, port int) int {
	res, _ := env["result"].(map[string]interface{})
	if res == nil {
		return 0
	}
	items, _ := res["items"].([]interface{})
	best := 0
	for _, it := range items {
		m, _ := it.(map[string]interface{})
		if m == nil {
			continue
		}
		if p, _ := m["Port"].(float64); int(p) == port {
			if id, _ := m["Id"].(float64); int(id) > best {
				best = int(id)
			}
		}
	}
	return best
}

func truncate(b []byte) string {
	if len(b) > 300 {
		return string(b[:300]) + "…"
	}
	return string(b)
}

// urlEncode 对查询参数做转义（路径里常见 "/" 与空格）。
func urlEncode(s string) string { return url.QueryEscape(s) }

// decodeBase64 容忍换行/空白的标准 base64 解码。
func decodeBase64(b []byte) ([]byte, error) {
	clean := make([]byte, 0, len(b))
	for _, c := range b {
		switch c {
		case '\n', '\r', ' ', '\t':
			continue
		}
		clean = append(clean, c)
	}
	return base64.StdEncoding.DecodeString(string(clean))
}
