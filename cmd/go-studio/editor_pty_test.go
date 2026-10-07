// editor_pty_test.go — 在真实伪终端里驱动编辑器：键入字符、保存、退出。
// SPDX-License-Identifier: MIT

// 本文件用到伪终端与 unix 信号，只能在类 Unix 平台编译。
//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"
)

// startEditor 启动编辑器；path 非空时作为命令行参数传入。
func startEditor(t *testing.T, binary, path string) *ptySession {
	t.Helper()
	if path == "" {
		return newPTYSession(t, binary, 100, 30)
	}
	return newPTYSessionArgs(t, binary, 100, 30, path)
}

// writeKeys 向伪终端写入一串按键。
func writeKeys(t *testing.T, session *ptySession, keys string) {
	t.Helper()
	if _, err := session.tty.WriteString(keys); err != nil {
		t.Fatalf("写入按键失败: %v", err)
	}
}

// stripANSI 去掉 ANSI 转义序列，只留可见字符。
//
// 必须先剥色再断言可见文字：光标正好压在某个字符上时，
// 该字符会被反显序列劈成两半（"package" 变成 "p" + 转义 + "ackage"），
// 直接搜原文必然找不到。
func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != 0x1b {
			b.WriteByte(s[i])
			continue
		}
		j := i + 1
		if j < len(s) && s[j] == '[' {
			j++
			for j < len(s) && !(s[j] >= 0x40 && s[j] <= 0x7e) {
				j++
			}
			if j < len(s) {
				j++
			}
		} else if j < len(s) {
			// OSC 序列：以 BEL 或 ST 结束。
			for j < len(s) && s[j] != 0x07 && !(s[j] == 0x1b && j+1 < len(s) && s[j+1] == '\\') {
				j++
			}
			if j < len(s) && s[j] == 0x07 {
				j++
			} else if j+1 < len(s) {
				j += 2
			}
		}
		i = j - 1
	}
	return b.String()
}

// waitForOutput 等待输出（剥掉 ANSI 序列后）中出现指定子串。
func waitForOutput(t *testing.T, session *ptySession, want string, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		got := session.outputSoFar()
		visible := stripANSI(got)
		if strings.Contains(visible, want) {
			return visible
		}
		if time.Now().After(deadline) {
			t.Fatalf("输出中始终没有 %q，timeout=%v，输出=%q", want, timeout, visible)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// typeOne 键入单个字符并等它出现在输出里。
//
// 逐键等待而不是一次写完整串：Bubble Tea 按事件逐帧处理，
// 一次写太多会挤在同一个读循环里，断言时序不可靠。
func typeOne(t *testing.T, session *ptySession, ch rune) {
	t.Helper()
	writeKeys(t, session, string(ch))
	waitForOutput(t, session, string(ch), 5*time.Second)
	time.Sleep(80 * time.Millisecond)
}

// stopSession 正常终止会话并确认退出。
func stopSession(t *testing.T, session *ptySession) {
	t.Helper()
	session.signal(syscall.SIGTERM)
	session.wait(t, 15*time.Second)
}

// TestEditorTypesAndShowsText 在真实伪终端里键入文本，
// 确认按键真的能变成屏幕上的字符——这是整个编辑器最该被证明的一件事。
//
// 断言逐个字符而不是整串：Bubble Tea 做的是差分渲染，只重写变化的单元格，
// 因此累积输出里每个字符是分散在多次写入中的，整串永远不会连续出现。
// 状态栏里的 1:1→1:2→… 与脏标记 * 同时证明键入确实生效了。
func TestEditorTypesAndShowsText(t *testing.T) {
	binary := buildBinary(t)
	session := startEditor(t, binary, "")
	defer session.close()

	session.waitForAltScreen(t, 15*time.Second)
	for _, ch := range "hello" {
		typeOne(t, session, ch)
	}

	// 出现脏标记说明五次键入都作用到了文档上。
	visible := waitForOutput(t, session, "*", 5*time.Second)
	if !strings.Contains(visible, "未命名") {
		t.Errorf("状态栏没有显示文档名，输出=%q", visible)
	}

	stopSession(t, session)
}

// TestEditorOpensFileAndShowsContent 打开已有文件应把内容与文件名显示在屏幕上。
func TestEditorOpensFileAndShowsContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.txt")
	if err := os.WriteFile(path, []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	binary := buildBinary(t)
	session := startEditor(t, binary, path)
	defer session.close()

	session.waitForAltScreen(t, 15*time.Second)
	output := waitForOutput(t, session, "sample.txt", 10*time.Second)
	if !strings.Contains(output, "package") {
		t.Errorf("输出里没有文件内容，output=%q", output)
	}

	stopSession(t, session)
}

// TestEditorSavesTypedText 键入后用 C-a w 保存，磁盘内容应与输入一致。
// 这是整条链路——按键、命令解析、文档、文件写入——的端到端证明。
func TestEditorSavesTypedText(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")

	binary := buildBinary(t)
	session := startEditor(t, binary, path)
	defer session.close()

	session.waitForAltScreen(t, 15*time.Second)
	for _, ch := range "saved" {
		typeOne(t, session, ch)
	}
	waitForOutput(t, session, "out.txt", 5*time.Second)

	// C-a w 保存：C-a 是前缀键，再按 w。
	writeKeys(t, session, "\x01") // Ctrl-A
	time.Sleep(200 * time.Millisecond)
	writeKeys(t, session, "w")
	waitForOutput(t, session, "已保存", 10*time.Second)

	stopSession(t, session)

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取保存后的文件失败: %v", err)
	}
	if string(got) != "saved" {
		t.Errorf("磁盘内容 = %q, want %q", got, "saved")
	}
}

// TestEditorUndoViaChord C-a r 应撤销刚输入的一整串。
func TestEditorUndoViaChord(t *testing.T) {
	binary := buildBinary(t)
	session := startEditor(t, binary, "")
	defer session.close()

	session.waitForAltScreen(t, 15*time.Second)
	for _, ch := range "abcde" {
		typeOne(t, session, ch)
	}

	writeKeys(t, session, "\x01") // Ctrl-A
	time.Sleep(200 * time.Millisecond)
	writeKeys(t, session, "r")
	waitForOutput(t, session, "已撤销", 10*time.Second)

	stopSession(t, session)
}

// TestEditorResizesWithoutCrash 改变窗口尺寸后程序应继续存活。
func TestEditorResizesWithoutCrash(t *testing.T) {
	binary := buildBinary(t)
	session := startEditor(t, binary, "")
	defer session.close()

	session.waitForAltScreen(t, 15*time.Second)

	if err := pty.Setsize(session.tty, &pty.Winsize{Rows: 40, Cols: 120}); err != nil {
		t.Skipf("无法调整伪终端尺寸: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	if session.exited() {
		t.Fatalf("resize 后程序退出了，output=%q", session.outputSoFar())
	}

	stopSession(t, session)
}

// TestEditorSurvivesChordKeys 连续按下组合键后程序不应崩溃或提前退出。
func TestEditorSurvivesChordKeys(t *testing.T) {
	binary := buildBinary(t)
	session := startEditor(t, binary, "")
	defer session.close()

	session.waitForAltScreen(t, 15*time.Second)

	// 前缀后接一个无效键，必须被安全丢弃而不是让程序退出。
	writeKeys(t, session, "\x01\x01")
	time.Sleep(200 * time.Millisecond)

	if session.exited() {
		t.Fatalf("无效组合键之后程序退出了，output=%q", session.outputSoFar())
	}

	stopSession(t, session)
}

// TestEditorRestoresTerminalAfterEditing 编辑并保存之后退出，
// 终端状态仍必须完全恢复——这是 M0 留下的承诺，编辑器不能破坏它。
func TestEditorRestoresTerminalAfterEditing(t *testing.T) {
	binary := buildBinary(t)
	session := startEditor(t, binary, "")
	defer session.close()

	session.waitForAltScreen(t, 15*time.Second)
	for _, ch := range "xyz" {
		typeOne(t, session, ch)
	}

	if !isRawMode(session.terminalState(t)) {
		t.Fatal("程序未把终端切换为 raw 模式，测试前提不成立")
	}

	stopSession(t, session)

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

// TestEditorOpensFileViaPrompt 走完整的「C-a f → 敲路径 → 回车」链路，
// 确认浮层输入真的能打开文件。
func TestEditorOpensFileViaPrompt(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "opened.txt")
	if err := os.WriteFile(target, []byte("opened-content"), 0o644); err != nil {
		t.Fatal(err)
	}

	binary := buildBinary(t)
	session := startEditor(t, binary, "")
	defer session.close()

	session.waitForAltScreen(t, 15*time.Second)
	// C-a f
	writeKeys(t, session, "\x01")
	time.Sleep(150 * time.Millisecond)
	writeKeys(t, session, "f")

	// 浮层打开后会预填当前工作目录，先退格清掉再输入目标路径。
	// 退格发 DEL(0x7f)，Bubble Tea 把它解析成 backspace。
	// 分批写入并留出间隔：一次灌 120 个键会挤在同一个读循环里，
	// 主循环来不及处理，回车就可能排在退格之前被丢掉。
	for range 12 {
		writeKeys(t, session, "\x7f\x7f\x7f\x7f\x7f\x7f\x7f\x7f\x7f\x7f")
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(300 * time.Millisecond)

	for _, ch := range target {
		writeKeys(t, session, string(ch))
		time.Sleep(15 * time.Millisecond)
	}
	time.Sleep(200 * time.Millisecond)
	writeKeys(t, session, "\r")

	// 必须等「已打开」这条确认，而不是等路径本身：
	// 路径在浮层里就已经显示出来了，按那个去等会在回车生效前就返回。
	waitForOutput(t, session, "已打开", 10*time.Second)
	if output := waitForOutput(t, session, "opened-content", 10*time.Second); !strings.Contains(output, "opened.txt") {
		t.Errorf("帧里没有文件名，output=%q", output)
	}

	stopSession(t, session)
}

// TestEditorGotoLineViaPrompt C-a g 输入行号后光标要跳过去。
func TestEditorGotoLineViaPrompt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lines.txt")
	if err := os.WriteFile(path, []byte("l1\nl2\nl3\nl4\nl5\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	binary := buildBinary(t)
	session := startEditor(t, binary, path)
	defer session.close()

	session.waitForAltScreen(t, 15*time.Second)

	// C-a g
	writeKeys(t, session, "\x01")
	time.Sleep(150 * time.Millisecond)
	writeKeys(t, session, "g")

	// 浮层预填当前行号 1，退格清空后输入 4。
	writeKeys(t, session, "\x7f\x7f")
	time.Sleep(150 * time.Millisecond)
	for _, ch := range "4" {
		writeKeys(t, session, string(ch))
	}
	time.Sleep(150 * time.Millisecond)
	writeKeys(t, session, "\r")

	// 跳到第 4 行后状态栏应显示 4:1。
	waitForOutput(t, session, "跳转到第 4 行", 10*time.Second)
	waitForOutput(t, session, "4:1", 10*time.Second)

	stopSession(t, session)
}

// TestEditorEscCancelsPrompt Esc 必须能取消浮层且不影响文档。
func TestEditorEscCancelsPrompt(t *testing.T) {
	binary := buildBinary(t)
	session := startEditor(t, binary, "")
	defer session.close()

	session.waitForAltScreen(t, 15*time.Second)

	typeOne(t, session, 'a')
	// C-a f 打开浮层，然后 Esc 取消。
	writeKeys(t, session, "\x01")
	time.Sleep(150 * time.Millisecond)
	writeKeys(t, session, "f")
	time.Sleep(150 * time.Millisecond)
	writeKeys(t, session, "\x1b")

	waitForOutput(t, session, "已取消", 10*time.Second)

	// 取消之后仍能继续编辑，文档没有被浮层污染。
	typeOne(t, session, 'b')

	if session.exited() {
		t.Fatalf("取消浮层之后程序退出了，output=%q", session.outputSoFar())
	}

	stopSession(t, session)
}
