package main

// 交互式终端的进程抽象。
//
// 原版的终端是"真 TTY"：能 resize、有回显、能跑 vim/top 这类需要终端语义的程序。
// 复刻端此前用 `exec.Command` + 三条管道（stdin/stdout/stderr），虽然能执行命令，
// 但没有 TTY 语义 —— resize 无效、程序检测不到终端、行编辑/进度条/颜色都不对。
//
// 这里把"启动一个 shell"抽象成 shellProc：
//
//	terminal_unix.go    → pty（creack/pty），真 TTY，支持 resize
//	terminal_windows.go → 管道回退（ConPTY 尚未实现，resize 为空操作）
//
// main.go 的 startTerminal/terminalInput/terminalClose 只依赖这个接口。

import (
	"io"
	"os/exec"
)

// shellProc 是一段可交互的 shell 进程。
type shellProc interface {
	io.Reader              // 合并后的输出（TTY 只有一路）
	io.Writer              // 键盘输入
	Resize(rows, cols int) error
	Close() error
	Wait() error
}

// newShellProc 按平台创建 shell 进程（unix 走 PTY）。
func newShellProc(shell string, rows, cols int) (shellProc, error) {
	cmd := exec.Command(shell)
	hideCmdWindow(cmd)
	return startShellProc(cmd, rows, cols)
}
