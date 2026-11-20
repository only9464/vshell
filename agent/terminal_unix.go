//go:build !windows

package main

// Unix 下的 PTY 终端实现（Linux/macOS）。
//
// 用 creack/pty 把 shell 接到一个伪终端上：agent 读到的是"合并后的输出流"
// （不需要再分别读 stdout/stderr），写入的是终端主端，resize 通过 TIOCSWINSZ
// 生效，程序能看到真 TTY（`isatty` 为真），因此 vim/top/进度条/颜色都正常。

import (
	"os"
	"os/exec"

	"github.com/creack/pty"
)

// ptyShell 是基于 PTY 的 shellProc。
type ptyShell struct {
	f   *os.File
	cmd *exec.Cmd
}

// startShellProc 用 PTY 启动 shell，并设置初始窗口大小。
func startShellProc(cmd *exec.Cmd, rows, cols int) (shellProc, error) {
	f, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}
	p := &ptyShell{f: f, cmd: cmd}
	if rows <= 0 {
		rows = 40
	}
	if cols <= 0 {
		cols = 120
	}
	_ = p.Resize(rows, cols)
	return p, nil
}

func (p *ptyShell) Read(b []byte) (int, error)  { return p.f.Read(b) }
func (p *ptyShell) Write(b []byte) (int, error) { return p.f.Write(b) }

// Resize 调整伪终端窗口大小（对应 xterm.js 的 fit 事件）。
func (p *ptyShell) Resize(rows, cols int) error {
	if rows <= 0 || cols <= 0 {
		return nil
	}
	return pty.Setsize(p.f, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
}

// Close 关闭主端（子进程会收到 SIGHUP）。
func (p *ptyShell) Close() error { return p.f.Close() }

// Wait 等待子进程退出。
func (p *ptyShell) Wait() error { return p.cmd.Wait() }
