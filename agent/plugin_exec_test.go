package main

import (
	"encoding/base64"
	"os"
	"strings"
	"testing"
)

// 可执行插件：落盘 → 执行 → 回显输出。
func TestRunPluginExecutable(t *testing.T) {
	// 用系统 shell 当"插件"：读脚本、回显参数。
	script := "#!/bin/sh\necho \"plugin:$1\"\n"
	data := base64.StdEncoding.EncodeToString([]byte(script))
	out, errMsg := runPlugin("demo.elf", data, "hello world")
	if errMsg != "" {
		t.Fatalf("runPlugin err: %s (out=%q)", errMsg, out)
	}
	if strings.TrimSpace(out) != "plugin:hello" {
		t.Fatalf("output = %q, want plugin:hello", out)
	}
}

// 动态库插件：明确返回未实现，不静默失败。
func TestRunPluginRejectsSharedLibrary(t *testing.T) {
	data := base64.StdEncoding.EncodeToString([]byte("\x7fELF fake"))
	for _, name := range []string{"libx.so", "libx.dylib", "x.dll", "x.net"} {
		if _, errMsg := runPlugin(name, data, ""); errMsg == "" {
			t.Fatalf("%s should be rejected as unsupported", name)
		}
	}
}

// 非法入参：空名、坏 base64、坏文件名。
func TestRunPluginRejectsBadInput(t *testing.T) {
	if _, errMsg := runPlugin("", "AAAA", ""); errMsg == "" {
		t.Fatalf("empty name should fail")
	}
	if _, errMsg := runPlugin("x.elf", "not-base64!!", ""); errMsg == "" {
		t.Fatalf("bad base64 should fail")
	}
}

// 插件进程非零退出：输出仍要回传，并带错误信息。
func TestRunPluginNonZeroExit(t *testing.T) {
	script := "#!/bin/sh\necho oops\nexit 3\n"
	data := base64.StdEncoding.EncodeToString([]byte(script))
	out, errMsg := runPlugin("fail.elf", data, "")
	if errMsg == "" {
		t.Fatalf("non-zero exit should report an error")
	}
	if !strings.Contains(out, "oops") {
		t.Fatalf("output should still be captured, got %q", out)
	}
}

// 临时文件必须被清理（不在 /tmp 留下 vshell-plugin-* 目录）。
func TestRunPluginCleansTempDir(t *testing.T) {
	before, _ := os.ReadDir(os.TempDir())
	data := base64.StdEncoding.EncodeToString([]byte("#!/bin/sh\ntrue\n"))
	_, _ = runPlugin("ok.elf", data, "")
	after, _ := os.ReadDir(os.TempDir())
	count := func(list []os.DirEntry) int {
		n := 0
		for _, e := range list {
			if strings.HasPrefix(e.Name(), "vshell-plugin-") {
				n++
			}
		}
		return n
	}
	if count(after) > count(before) {
		t.Fatalf("temp dir leaked: before=%d after=%d", count(before), count(after))
	}
}

// filetouch 的时间解析（面板 /file/modifytime 用）。
func TestParseFileTime(t *testing.T) {
	for _, in := range []string{"2026-11-20T22:00:00Z", "2026-11-20 22:00:00", "2026-11-20"} {
		if _, err := parseFileTime(in); err != nil {
			t.Fatalf("%q: %v", in, err)
		}
	}
	if _, err := parseFileTime("not-a-time"); err == nil {
		t.Fatalf("bad input should fail")
	}
}
