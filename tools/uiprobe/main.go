// uiprobe 用 Chrome DevTools Protocol 在无头浏览器里执行 JS，用于前端"点击级"冒烟。
//
// 为什么单独写：仓库外没有 puppeteer/playwright，而 CDP 只需要一个 WebSocket 客户端
// （gorilla/websocket 已是依赖）。Runtime.evaluate 足以完成"填表单 → 点按钮 → 读 DOM"。
//
// 用法：
//
//	go run ./tools/uiprobe -url http://127.0.0.1:8082/app/ -script 'document.title' -wait 5s
//
// -script 会在页面加载完成后在页面上下文执行，返回值按 JSON 打印。
package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type target struct {
	Type                 string `json:"type"`
	URL                  string `json:"url"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

func main() {
	url := flag.String("url", "about:blank", "要打开的页面")
	script := flag.String("script", "document.title", "页面加载后执行的 JS")
	wait := flag.Duration("wait", 4*time.Second, "等待页面加载的时间")
	port := flag.Int("port", 9223, "调试端口")
	chrome := flag.String("chrome", "chromium", "浏览器可执行文件")
	keep := flag.Bool("keep", false, "结束后保留浏览器进程（调试用）")
	typeText := flag.String("type", "", "在 -script 之后向聚焦元素输入这段文本并回车（走 CDP Input 域，真实键盘事件）")
	netlog := flag.Bool("net", false, "收集页面加载期间的网络请求（URL + 关键请求头）并打印为 JSON")
	preload := flag.String("preload", "", "在每个新文档加载**之前**执行的 JS（Page.addScriptToEvaluateOnNewDocument），常用来自动登录 / 注入 token")
	shot := flag.String("shot", "", "把当前视口截图为 PNG 写到该路径（-script 已执行完之后）")
	shotW := flag.Int("shotw", 1440, "截图视口宽度（配合 -shot，Emulation.setDeviceMetricsOverride）")
	shotH := flag.Int("shoth", 900, "截图视口高度")
	flag.Parse()

	cmd := exec.Command(*chrome, "--headless=new", "--disable-gpu", "--no-sandbox",
		fmt.Sprintf("--remote-debugging-port=%d", *port), *url)
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	if err := cmd.Start(); err != nil {
		fail("启动浏览器失败: %v", err)
	}
	if !*keep {
		defer func() { _ = cmd.Process.Kill() }()
	}

	var tg target
	base := fmt.Sprintf("http://127.0.0.1:%d", *port)
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		tg = findPage(base)
		if tg.WebSocketDebuggerURL != "" {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if tg.WebSocketDebuggerURL == "" {
		fail("没有找到可调试的页面目标")
	}

	conn, _, err := websocket.DefaultDialer.Dial(tg.WebSocketDebuggerURL, nil)
	if err != nil {
		fail("连接 CDP 失败: %v", err)
	}
	defer conn.Close()

	_ = call(conn, 5, "Page.enable", map[string]interface{}{})
	if *preload != "" {
		// 在页面脚本运行前注入：SPA 读取 localStorage 时就能拿到 token/会话
		if err := call(conn, 6, "Page.addScriptToEvaluateOnNewDocument",
			map[string]interface{}{"source": *preload}); err != nil {
			fail("注入 preload 失败: %v", err)
		}
		// 注入只对**之后**加载的文档生效，因此重新导航一次
		_ = call(conn, 7, "Page.navigate", map[string]interface{}{"url": *url})
	}

	time.Sleep(*wait)

	res, err := evaluate(conn, *script)
	if err != nil {
		fail("执行脚本失败: %v", err)
	}
	if *netlog {
		// 先开 Network 再重新导航，否则事件都在 enable 之前发生、收不到
		_ = call(conn, 19, "Page.enable", map[string]interface{}{})
		_ = call(conn, 20, "Network.enable", map[string]interface{}{})
		_ = call(conn, 21, "Page.navigate", map[string]interface{}{"url": *url})
		printNet(conn, *wait)
		return
	}
	if *typeText != "" {
		// 打字前先"点一下"终端区域：页面在收到 WS 输出后会重渲染，
		// focus 可能已经丢了，单纯 focus() 有时不生效（xterm 需要 mousedown）。
		_, _ = evaluate(conn, `(() => {
			const x = document.querySelector('.xterm');
			if (x) x.dispatchEvent(new MouseEvent('mousedown', {bubbles: true}));
			const ta = document.querySelector('.xterm-helper-textarea');
			if (ta) ta.focus();
			return document.activeElement ? document.activeElement.className : null;
		})()`)
		time.Sleep(300 * time.Millisecond)
		if err := typeAndEnter(conn, *typeText); err != nil {
			fail("输入文本失败: %v", err)
		}
		// 给终端一点时间回显
		time.Sleep(2 * time.Second)
		res2, err := evaluate(conn, *script)
		if err != nil {
			fail("二次执行脚本失败: %v", err)
		}
		fmt.Println(res2)
		return
	}
	// 截图：先按指定尺寸重排视口，再 Page.captureScreenshot
	if *shot != "" {
		_ = call(conn, 30, "Emulation.setDeviceMetricsOverride", map[string]interface{}{
			"width": *shotW, "height": *shotH, "deviceScaleFactor": 1, "mobile": false,
		})
		time.Sleep(800 * time.Millisecond)
		data, err := captureScreenshot(conn)
		if err != nil {
			fail("截图失败: %v", err)
		}
		if err := os.WriteFile(*shot, data, 0o644); err != nil {
			fail("写截图失败: %v", err)
		}
		fmt.Fprintf(os.Stderr, "screenshot -> %s (%d bytes)\n", *shot, len(data))
	}

	fmt.Println(res)
}

// captureScreenshot 通过 CDP 抓一张 PNG。
func captureScreenshot(conn *websocket.Conn) ([]byte, error) {
	req := map[string]interface{}{"id": 31, "method": "Page.captureScreenshot",
		"params": map[string]interface{}{"format": "png"}}
	if err := conn.WriteJSON(req); err != nil {
		return nil, err
	}
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return nil, err
		}
		var resp struct {
			ID     int `json:"id"`
			Result struct {
				Data string `json:"data"`
			} `json:"result"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			continue
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("%s", resp.Error.Message)
		}
		if resp.ID == 31 {
			return base64.StdEncoding.DecodeString(resp.Result.Data)
		}
	}
}

func findPage(base string) target {
	resp, err := http.Get(base + "/json")
	if err != nil {
		return target{}
	}
	defer resp.Body.Close()
	var list []target
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return target{}
	}
	for _, t := range list {
		if t.Type == "page" {
			return t
		}
	}
	return target{}
}

// evaluate 在页面上下文执行 JS，返回结果（字符串化）。
func evaluate(conn *websocket.Conn, script string) (string, error) {
	req := map[string]interface{}{
		"id":     1,
		"method": "Runtime.evaluate",
		"params": map[string]interface{}{
			"expression":    script,
			"returnByValue": true,
			"awaitPromise":  true,
		},
	}
	if err := conn.WriteJSON(req); err != nil {
		return "", err
	}
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return "", err
		}
		var resp struct {
			ID     int `json:"id"`
			Result struct {
				Result struct {
					Value interface{} `json:"value"`
					Type  string      `json:"type"`
				} `json:"result"`
				ExceptionDetails *struct {
					Text string `json:"text"`
				} `json:"exceptionDetails"`
			} `json:"result"`
		}
		if err := json.Unmarshal(data, &resp); err != nil || resp.ID != 1 {
			continue
		}
		if resp.Result.ExceptionDetails != nil {
			return "", fmt.Errorf("%s", resp.Result.ExceptionDetails.Text)
		}
		v, _ := json.Marshal(resp.Result.Result.Value)
		return strings.Trim(string(v), "\""), nil
	}
}

func fail(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "[uiprobe] "+format+"\n", args...)
	os.Exit(1)
}

// typeAndEnter 通过 CDP Input 域逐字符发送键盘事件（真实键盘行为，可驱动 xterm）。
//
// 不用 Input.insertText：它走组合输入路径，xterm 在无头环境里可能收不到；
// 逐字符 keyDown(text=...) + keyUp 与真实敲键盘一致。
func typeAndEnter(conn *websocket.Conn, text string) error {
	for i, ch := range text {
		params := map[string]interface{}{
			"type":                  "keyDown",
			"key":                   string(ch),
			"text":                  string(ch),
			"unmodifiedText":        string(ch),
			"windowsVirtualKeyCode": int(ch),
			"nativeVirtualKeyCode":  int(ch),
		}
		if err := call(conn, 10+i*2, "Input.dispatchKeyEvent", params); err != nil {
			return err
		}
		if err := call(conn, 11+i*2, "Input.dispatchKeyEvent", map[string]interface{}{
			"type": "keyUp", "key": string(ch), "windowsVirtualKeyCode": int(ch),
		}); err != nil {
			return err
		}
	}
	// Enter：rawKeyDown + char + keyUp，xterm 主要看 keyCode 13
	for _, typ := range []string{"rawKeyDown", "char", "keyUp"} {
		params := map[string]interface{}{
			"type":                  typ,
			"key":                   "Enter",
			"code":                  "Enter",
			"windowsVirtualKeyCode": 13,
			"nativeVirtualKeyCode":  13,
		}
		if typ == "char" {
			params["text"] = "\r"
			params["unmodifiedText"] = "\r"
		}
		if err := call(conn, 11, "Input.dispatchKeyEvent", params); err != nil {
			return err
		}
	}
	return nil
}

// call 发送一条 CDP 命令并等待对应 id 的响应。
func call(conn *websocket.Conn, id int, method string, params map[string]interface{}) error {
	req := map[string]interface{}{"id": id, "method": method, "params": params}
	if err := conn.WriteJSON(req); err != nil {
		return err
	}
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		var resp struct {
			ID    int `json:"id"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(data, &resp); err != nil || resp.ID != id {
			continue
		}
		if resp.Error != nil {
			return fmt.Errorf("%s: %s", method, resp.Error.Message)
		}
		return nil
	}
}

// printNet 打开 Network 域，收集一段时间内的请求（URL + 请求头里的鉴权字段）。
func printNet(conn *websocket.Conn, wait time.Duration) {
	deadline := time.Now().Add(wait)
	type rec struct {
		URL  string            `json:"url"`
		Auth map[string]string `json:"auth,omitempty"`
	}
	var out []rec
	// 注意：gorilla/websocket 在读到超时错误后连接即视为失败，**不能继续读**
	// （会 panic "repeated read on failed websocket connection"）。
	// 因此这里设一个总时限，读到错误就结束收集。
	_ = conn.SetReadDeadline(deadline)
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var ev struct {
			Method string `json:"method"`
			Params struct {
				Request struct {
					URL     string            `json:"url"`
					Headers map[string]string `json:"headers"`
				} `json:"request"`
			} `json:"params"`
		}
		if err := json.Unmarshal(data, &ev); err != nil || ev.Method != "Network.requestWillBeSent" {
			continue
		}
		auth := map[string]string{}
		for k, v := range ev.Params.Request.Headers {
			lk := strings.ToLower(k)
			if strings.Contains(lk, "token") || lk == "authorization" || strings.Contains(lk, "cookie") {
				auth[k] = v
			}
		}
		out = append(out, rec{URL: ev.Params.Request.URL, Auth: auth})
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(b))
}
