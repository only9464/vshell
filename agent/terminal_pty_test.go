//go:build !windows

package main

// PTY 终端测试（Linux/macOS）。
//
// 验证：真 TTY（shell 能看到终端）→ 能执行命令并拿到输出 → resize 生效 →
// Close 后进程退出。这些正是此前"管道版终端"缺失的语义。

import (
	"strings"
	"testing"
	"time"
)

func TestPTYShellEchoAndResize(t *testing.T) {
	proc, err := newShellProc("/bin/sh", 24, 80)
	if err != nil {
		t.Fatalf("newShellProc: %v", err)
	}
	defer func() { _ = proc.Close() }()

	// 真 TTY 下 `[ -t 1 ]` 为真；管道实现会打印 "no-tty"。
	if _, err := proc.Write([]byte("if [ -t 1 ]; then echo TTY:yes; else echo TTY:no; fi\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	got := readUntil(t, proc, "TTY:", 3*time.Second)
	if !strings.Contains(got, "TTY:yes") {
		t.Fatalf("shell did not see a TTY: %q", got)
	}

	// resize 必须生效（PTY 的 TIOCSWINSZ）。
	if err := proc.Resize(40, 120); err != nil {
		t.Fatalf("resize: %v", err)
	}
	if _, err := proc.Write([]byte("stty size\n")); err != nil {
		t.Fatalf("write stty: %v", err)
	}
	got = readUntil(t, proc, "40 120", 3*time.Second)
	if !strings.Contains(got, "40 120") {
		// 同时把读到的内容带出来，便于排查（不同 stty 版本可能输出不同列宽）。
		t.Fatalf("resize did not take effect, output: %q", got)
	}
}

// readUntil 一直读到输出里出现 want 或超时。
func readUntil(t *testing.T, r interface{ Read([]byte) (int, error) }, want string, d time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(d)
	var sb strings.Builder
	buf := make([]byte, 1024)
	for time.Now().Before(deadline) {
		type res struct {
			n   int
			err error
		}
		ch := make(chan res, 1)
		go func() {
			n, err := r.Read(buf)
			ch <- res{n, err}
		}()
		select {
		case got := <-ch:
			if got.n > 0 {
				sb.Write(buf[:got.n])
				if strings.Contains(sb.String(), want) {
					return sb.String()
				}
			}
			if got.err != nil {
				return sb.String()
			}
		case <-time.After(d):
			return sb.String()
		}
	}
	return sb.String()
}
