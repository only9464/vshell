package main

// 屏幕远控的输入注入（鼠标移动/点击/滚轮/键盘）。
//
// 服务端（controllers/screen.go 的 screenViewerCommand）把浏览器事件编码成
// 文本命令下发：
//
//	screen_input move x=<X> y=<Y>          鼠标绝对坐标移动
//	screen_input click button=<1|2|3>      左/中/右键单击
//	screen_input scroll dir=<up|down> amount=<N>
//	screen_input key=<KEYSYM>              按键/组合键（xdotool 语法，如 ctrl+c）
//
// 之前 agent 侧**完全没有** screen_input 处理器：命令先被 executeCommand 落到
// shell（因为前缀不是 screen_capture），即使进来 dispatchScreenCommand 也返回
// "unknown screen capture command"。也就是说屏幕直播看得到、点不动。
//
// 注入后端：Linux/X11 用 xdotool（本机已有；Wayland 下走 XWayland 同样可用）。
// Windows 侧留 TODO（ConPTY/SendInput 一起做），返回明确错误而不是静默失败。

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// screenInputEvent 是解析后的输入事件。
type screenInputEvent struct {
	Kind   string // move / click / scroll / key
	X, Y   int
	Button int
	Amount int
	Dir    string
	Key    string
}

var (
	errBadScreenInput   = errors.New("malformed screen_input command")
	errUnknownScreenCmd = errors.New("unknown screen_input kind")
	errNoInputBackend   = errors.New("no input backend on this platform")
)

// parseScreenInput 解析 "move x=10 y=20" / "click button=1" /
// "scroll dir=down amount=3" / "key=ctrl+c" 这类参数串。
func parseScreenInput(args string) (screenInputEvent, error) {
	args = strings.TrimSpace(args)
	if args == "" {
		return screenInputEvent{}, errBadScreenInput
	}
	ev := screenInputEvent{Amount: 1, Button: 1}

	// 服务端的键盘事件没有独立 kind：整条命令就是 "key=<KEYSYM>"。
	if strings.HasPrefix(args, "key=") {
		ev.Kind = "key"
		ev.Key = strings.TrimSpace(strings.TrimPrefix(args, "key="))
		if ev.Key == "" {
			return ev, errBadScreenInput
		}
		return ev, nil
	}

	// 其余形态：第一个词是 kind（move/click/scroll），其后是 k=v。
	fields := strings.Fields(args)
	ev.Kind = fields[0]
	for _, f := range fields[1:] {
		k, v, ok := strings.Cut(f, "=")
		if !ok {
			continue
		}
		switch k {
		case "x":
			ev.X, _ = strconv.Atoi(v)
		case "y":
			ev.Y, _ = strconv.Atoi(v)
		case "button":
			ev.Button, _ = strconv.Atoi(v)
		case "amount":
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				ev.Amount = n
			}
		case "dir":
			ev.Dir = strings.ToLower(v)
		case "key":
			ev.Key = v
		}
	}
	return ev, nil
}

// screenInputArgs 把事件翻译成 xdotool 参数（纯函数，便于单测）。
func screenInputArgs(ev screenInputEvent) ([]string, error) {
	switch ev.Kind {
	case "move":
		return []string{"mousemove", strconv.Itoa(ev.X), strconv.Itoa(ev.Y)}, nil
	case "click":
		b := ev.Button
		if b < 1 || b > 3 {
			b = 1
		}
		return []string{"click", strconv.Itoa(b)}, nil
	case "scroll":
		// X11 滚轮：button 4 = 上滚，5 = 下滚。
		b := "5"
		if ev.Dir == "up" {
			b = "4"
		}
		n := ev.Amount
		if n < 1 {
			n = 1
		}
		return []string{"click", "--repeat", strconv.Itoa(n), "--delay", "20", b}, nil
	case "key":
		key := strings.TrimSpace(ev.Key)
		if key == "" {
			return nil, errBadScreenInput
		}
		return []string{"key", "--clearmodifiers", key}, nil
	default:
		return nil, errUnknownScreenCmd
	}
}

// injectScreenInput 解析并执行一条 screen_input 命令。
func injectScreenInput(cmdStr string) (string, string) {
	args := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(cmdStr), "screen_input"))
	ev, err := parseScreenInput(args)
	if err != nil {
		return "", err.Error()
	}
	xargs, err := screenInputArgs(ev)
	if err != nil {
		return "", err.Error()
	}
	if runtime.GOOS == "windows" {
		return "", errNoInputBackend.Error() + " (windows sendinput not implemented yet)"
	}
	if _, lookErr := exec.LookPath("xdotool"); lookErr != nil {
		return "", "xdotool not found: " + lookErr.Error()
	}
	if out, runErr := exec.Command("xdotool", xargs...).CombinedOutput(); runErr != nil {
		return "", fmt.Sprintf("xdotool %v: %v (%s)", xargs, runErr, strings.TrimSpace(string(out)))
	}
	return "input ok: " + strings.Join(xargs, " "), ""
}
