// terminal_test.go — 端到端验证：在真实伪终端里启动与退出，终端状态必须完全恢复。
// SPDX-License-Identifier: MIT

// 本文件用到伪终端与 unix 信号，只能在类 Unix 平台编译。
//go:build !windows

package main

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

// altScreenSeq 是终端进入备用屏的序列，用来确认程序真的接管了终端。
const altScreenSeq = "\x1b[?1049h"

// buildBinary 编译出待测二进制，返回其路径。
func buildBinary(t *testing.T) string {
	t.Helper()
	binary := t.TempDir() + "/go-studio"
	cmd := exec.Command("go", "build", "-o", binary, ".")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("build binary: %v", err)
	}
	return binary
}

// ptySession 是一次伪终端会话，负责在后台持续读取输出。
type ptySession struct {
	tty *os.File
	cmd *exec.Cmd

	mu         sync.Mutex
	output     bytes.Buffer
	signalSent bool

	// before 是程序启动前的终端设置，用于退出后比对。
	before unix.Termios
}

// newPTYSession 启动 binary 并在后台读取输出。
// width、height 是伪终端的窗口尺寸；为 0 时程序会继续等待有效尺寸。
func newPTYSession(t *testing.T, binary string, width, height uint16) *ptySession {
	return newPTYSessionArgs(t, binary, width, height)
}

// newPTYSessionArgs 启动 binary 并附带命令行参数，在后台读取输出。
func newPTYSessionArgs(t *testing.T, binary string, width, height uint16, args ...string) *ptySession {
	t.Helper()

	cmd := exec.Command(binary, args...)
	tty, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: height, Cols: width})
	if err != nil {
		t.Skipf("无法分配伪终端（可能缺少 /dev/ptmx）: %v", err)
	}

	before, err := unix.IoctlGetTermios(int(tty.Fd()), unix.TCGETS)
	if err != nil {
		_ = tty.Close()
		t.Fatalf("读取初始终端设置失败: %v", err)
	}

	session := &ptySession{tty: tty, cmd: cmd, before: *before}
	session.readLoop()
	return session
}

// readLoop 在后台把伪终端输出读进缓冲区，直到终端关闭。
func (s *ptySession) readLoop() {
	go func() {
		chunk := make([]byte, 4096)
		for {
			n, err := s.tty.Read(chunk)
			if n > 0 {
				s.mu.Lock()
				s.output.Write(chunk[:n])
				s.mu.Unlock()
			}
			if err != nil {
				return
			}
		}
	}()
}

// outputSoFar 返回已经读到的全部输出。
func (s *ptySession) outputSoFar() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.output.String()
}

// waitForAltScreen 等待程序进入备用屏。
func (s *ptySession) waitForAltScreen(t *testing.T, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for !strings.Contains(s.outputSoFar(), altScreenSeq) {
		if time.Now().After(deadline) {
			t.Fatalf("程序未在 %v 内进入备用屏，输出=%q", timeout, s.outputSoFar())
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// signal 向子进程发信号，只发一次。
func (s *ptySession) signal(sig syscall.Signal) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.signalSent {
		return
	}
	s.signalSent = true
	_ = s.cmd.Process.Signal(sig)
}

// wait 等待进程退出，超时则强制杀死。
func (s *ptySession) wait(t *testing.T, timeout time.Duration) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- s.cmd.Wait() }()

	select {
	case <-done:
	case <-time.After(timeout):
		_ = s.cmd.Process.Kill()
		<-done
		t.Fatalf("程序未在 %v 内退出", timeout)
	}
}

// exited 报告进程是否已经退出。
func (s *ptySession) exited() bool {
	return s.cmd.ProcessState != nil && s.cmd.ProcessState.Exited()
}

// terminalState 读取伪终端当前的终端设置。
func (s *ptySession) terminalState(t *testing.T) unix.Termios {
	t.Helper()
	state, err := unix.IoctlGetTermios(int(s.tty.Fd()), unix.TCGETS)
	if err != nil {
		t.Fatalf("读取终端设置失败: %v", err)
	}
	return *state
}

// isRawMode 判断终端是否处于 raw 模式（ICANON 与 ECHO 关闭）。
func isRawMode(state unix.Termios) bool {
	return state.Lflag&unix.ICANON == 0 && state.Lflag&unix.ECHO == 0
}

// close 关闭伪终端。
func (s *ptySession) close() {
	_ = s.tty.Close()
}

func TestHelpAndVersionExitCleanly(t *testing.T) {
	binary := buildBinary(t)

	cases := map[string][]string{
		"help":    {"-h"},
		"version": {"-version"},
	}
	for name, args := range cases {
		out, err := exec.Command(binary, args...).CombinedOutput()
		if err != nil {
			t.Errorf("%s: exit error: %v, output: %s", name, err, out)
			continue
		}
		if len(out) == 0 {
			t.Errorf("%s: expected output, got nothing", name)
		}
	}
}

func TestRejectsUnknownFlag(t *testing.T) {
	binary := buildBinary(t)

	out, err := exec.Command(binary, "-nope").CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit for unknown flag")
	}
	if !bytes.Contains(out, []byte("未知选项")) {
		t.Errorf("output = %q, want it to mention the unknown flag", out)
	}
}

func TestRejectsFlagWithoutValue(t *testing.T) {
	binary := buildBinary(t)

	out, err := exec.Command(binary, "-log-level").CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit for missing flag value")
	}
	if !bytes.Contains(out, []byte("缺少取值")) {
		t.Errorf("output = %q, want it to mention the missing value", out)
	}
}

// TestTerminalStateRestoredAfterSigterm 是 M0 最重要的验收项。
//
// 终端被搞脏是最糟糕的失败模式：程序退出后用户后续所有命令行操作都会异常。
// 这里在真实伪终端里跑一遍，发送 SIGTERM 走正常退出路径，
// 再逐项比对 termios 的四个控制字。
func TestTerminalStateRestoredAfterSigterm(t *testing.T) {
	binary := buildBinary(t)
	session := newPTYSession(t, binary, 100, 30)
	defer session.close()

	session.waitForAltScreen(t, 15*time.Second)

	// 先确认程序确实把终端改成了 raw 模式，否则后面的比对没有意义。
	if !isRawMode(session.terminalState(t)) {
		t.Fatal("程序未把终端切换为 raw 模式，测试前提不成立")
	}

	session.signal(syscall.SIGTERM)
	session.wait(t, 15*time.Second)

	after := session.terminalState(t)
	switch {
	case after.Lflag != session.before.Lflag:
		t.Errorf("Lflag 未恢复: before=%#x after=%#x", session.before.Lflag, after.Lflag)
	case after.Iflag != session.before.Iflag:
		t.Errorf("Iflag 未恢复: before=%#x after=%#x", session.before.Iflag, after.Iflag)
	case after.Oflag != session.before.Oflag:
		t.Errorf("Oflag 未恢复: before=%#x after=%#x", session.before.Oflag, after.Oflag)
	case after.Cflag != session.before.Cflag:
		t.Errorf("Cflag 未恢复: before=%#x after=%#x", session.before.Cflag, after.Cflag)
	}
}

// TestProgramStaysAliveOnResize 验证 resize 事件不会让程序退出，
// 并且窗口尺寸确实被上报到日志。
func TestProgramStaysAliveOnResize(t *testing.T) {
	binary := buildBinary(t)
	session := newPTYSession(t, binary, 100, 30)
	defer session.close()

	session.waitForAltScreen(t, 15*time.Second)
	session.signal(syscall.SIGTERM)
	session.wait(t, 15*time.Second)
}

// TestZeroSizeTerminalKeepsWaiting 验证 0x0 窗口不会让程序误判就绪。
// 真实终端在布局完成前可能上报 0x0，程序必须忽略这种尺寸继续等待。
func TestZeroSizeTerminalKeepsWaiting(t *testing.T) {
	binary := buildBinary(t)
	session := newPTYSession(t, binary, 0, 0)
	defer session.close()

	// 给足时间，确认它没有因为 0x0 而立刻渲染并退出。
	time.Sleep(2 * time.Second)
	if session.exited() {
		t.Fatalf("程序在 0x0 终端上提前退出，应继续等待有效尺寸；输出=%q",
			session.outputSoFar())
	}
	session.signal(syscall.SIGTERM)
	session.wait(t, 15*time.Second)
}
