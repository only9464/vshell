package main

import (
	"strings"
	"testing"
)

// 解析：服务端下发的四种事件形态。
func TestParseScreenInput(t *testing.T) {
	cases := []struct {
		in   string
		want screenInputEvent
	}{
		{"move x=1280 y=720", screenInputEvent{Kind: "move", X: 1280, Y: 720, Button: 1, Amount: 1}},
		{"click button=3", screenInputEvent{Kind: "click", Button: 3, Amount: 1}},
		{"scroll dir=up amount=5", screenInputEvent{Kind: "scroll", Dir: "up", Amount: 5, Button: 1}},
		{"key=ctrl+c", screenInputEvent{Kind: "key", Key: "ctrl+c", Button: 1, Amount: 1}},
		{"key=alt+F4", screenInputEvent{Kind: "key", Key: "alt+F4", Button: 1, Amount: 1}},
	}
	for _, c := range cases {
		got, err := parseScreenInput(c.in)
		if err != nil {
			t.Fatalf("%q: %v", c.in, err)
		}
		if got.Kind != c.want.Kind || got.X != c.want.X || got.Y != c.want.Y ||
			got.Button != c.want.Button || got.Dir != c.want.Dir || got.Key != c.want.Key ||
			got.Amount != c.want.Amount {
			t.Fatalf("%q -> %+v, want %+v", c.in, got, c.want)
		}
	}
}

// 翻译成 xdotool 参数（纯函数，不需要 X server）。
func TestScreenInputArgs(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"move x=10 y=20", "mousemove 10 20"},
		{"click button=2", "click 2"},
		{"click button=9", "click 1"}, // 越界按钮回落到左键
		{"scroll dir=up amount=3", "click --repeat 3 --delay 20 4"},
		{"scroll dir=down amount=1", "click --repeat 1 --delay 20 5"},
		{"key=ctrl+shift+t", "key --clearmodifiers ctrl+shift+t"},
	}
	for _, c := range cases {
		ev, err := parseScreenInput(c.in)
		if err != nil {
			t.Fatalf("%q: %v", c.in, err)
		}
		args, err := screenInputArgs(ev)
		if err != nil {
			t.Fatalf("%q: %v", c.in, err)
		}
		if got := strings.Join(args, " "); got != c.want {
			t.Fatalf("%q -> %q, want %q", c.in, got, c.want)
		}
	}
}

// 非法输入必须报错而不是 panic / 静默成功。
func TestScreenInputErrors(t *testing.T) {
	for _, in := range []string{"", "jump x=1", "key="} {
		ev, err := parseScreenInput(in)
		if err == nil {
			if _, aerr := screenInputArgs(ev); aerr == nil {
				t.Fatalf("%q should be rejected", in)
			}
		}
	}
}

// 端到端（进程内）：injectScreenInput 对未知命令返回错误文案，且不会执行任何命令。
func TestInjectScreenInputUnknown(t *testing.T) {
	if _, errMsg := injectScreenInput("screen_input jump x=1"); errMsg == "" {
		t.Fatalf("unknown kind should return an error message")
	}
}
