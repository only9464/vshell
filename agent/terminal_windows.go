//go:build windows

package main

// Windows 下的终端实现（管道回退）。
//
// ConPTY 尚未接入：这里保留 exec + 管道的行为（能执行命令，但没有 TTY 语义，
// resize 为空操作）。等 M3 的第二阶段再换成 ConPTY（CreatePseudoConsole）。

import (
	"io"
	"os/exec"
)

// pipeShell 是管道版 shellProc（Windows 回退实现）。
type pipeShell struct {
	stdin  io.WriteCloser
	out    io.Reader
	cmd    *exec.Cmd
	closer func() error
}

func startShellProc(cmd *exec.Cmd, rows, cols int) (shellProc, error) {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	pr, pw := io.Pipe()
	go func() { _, _ = io.Copy(pw, stdout) }()
	go func() { _, _ = io.Copy(pw, stderr) }()
	return &pipeShell{stdin: stdin, out: pr, cmd: cmd, closer: pw.Close}, nil
}

func (p *pipeShell) Read(b []byte) (int, error)  { return p.out.Read(b) }
func (p *pipeShell) Write(b []byte) (int, error) { return p.stdin.Write(b) }

// Resize 在管道实现下无意义（没有窗口大小概念）。
func (p *pipeShell) Resize(rows, cols int) error { return nil }

func (p *pipeShell) Close() error {
	if p.closer != nil {
		_ = p.closer()
	}
	return p.stdin.Close()
}

func (p *pipeShell) Wait() error { return p.cmd.Wait() }
