// e2e.go — 完整使用测试：像真人一样在伪终端里把编辑器用一遍，逐项断言。
// SPDX-License-Identifier: MIT

// 依赖伪终端与 termios，只能在类 Unix 平台编译。
//go:build !windows

// 用法：
//
//	go run ./scripts/e2e                       # 全部场景
//	go run ./scripts/e2e -only 编辑            # 只跑名字含「编辑」的场景
//	go run ./scripts/e2e -keep                 # 失败时打印累计输出，便于排查
//	go run ./scripts/e2e -v                   # 打印每个步骤
//
// 与包内单测的分工：internal/* 的测试验证「某个函数对不对」，
// 这里验证「一个真人坐下来能不能用」，两者互补，不能互相替代。
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

func main() {
	var (
		only    = flag.String("only", "", "只跑名字包含该子串的场景")
		verbose = flag.Bool("v", false, "打印每个步骤")
		keep    = flag.Bool("keep", false, "失败时保留并打印工作目录")
		binary  = flag.String("bin", "", "直接使用已有的二进制，默认现编")
		list    = flag.Bool("list", false, "只列出场景名，不执行")
	)
	flag.Parse()

	if *list {
		for _, name := range names() {
			fmt.Println(name)
		}
		return
	}

	bin := *binary
	if bin == "" {
		var err error
		bin, err = buildBinary()
		if err != nil {
			fatalf("编译二进制失败: %v", err)
		}
	}

	scenarios := allScenarios()
	if *only != "" {
		var filtered []scenario
		for _, sc := range scenarios {
			if strings.Contains(sc.name, *only) {
				filtered = append(filtered, sc)
			}
		}
		scenarios = filtered
	}
	if len(scenarios) == 0 {
		fatalf("没有匹配的场景: %q", *only)
	}

	fmt.Printf("完整使用测试：%d 个场景\n\n", len(scenarios))

	var passed, failed int
	var failures []string
	start := time.Now()

	for _, sc := range scenarios {
		// 每个场景用独立的工作目录，互不干扰。
		dir, err := os.MkdirTemp("", "go-studio-e2e-*")
		must(err)

		res := sc.run(dir, bin, *verbose)
		if res.err != nil {
			failed++
			failures = append(failures, sc.name)
			fmt.Printf("  FAIL  %s\n", sc.name)
			for _, line := range strings.Split(res.err.Error(), "\n") {
				fmt.Printf("          %s\n", line)
			}
			if *keep {
				fmt.Printf("          工作目录保留在: %s\n", dir)
			} else {
				_ = os.RemoveAll(dir)
			}
			continue
		}
		passed++
		fmt.Printf("  ok    %s  (%d 步, %s)\n", sc.name, res.steps, res.elapsed.Round(time.Millisecond))
		_ = os.RemoveAll(dir)
	}

	fmt.Printf("\n通过 %d，失败 %d，总耗时 %s\n", passed, failed, time.Since(start).Round(time.Millisecond))
	if failed > 0 {
		fmt.Printf("失败的场景: %s\n", strings.Join(failures, ", "))
		os.Exit(1)
	}
}

// ---- 场景框架 ----

// scenario 是一个完整的使用场景。
type scenario struct {
	// name 是场景名，出现在报告里。
	name string
	// steps 是要依次执行的步骤。
	steps []step
}

// step 是一个步骤。
type step struct {
	// name 描述这一步在做什么。
	name string
	// fn 执行这一步。
	fn func(e *env) error
}

// result 是场景的执行结果。
type result struct {
	err     error
	steps   int
	elapsed time.Duration
}

// run 执行一个场景。
func (s scenario) run(dir, bin string, verbose bool) result {
	start := time.Now()
	e, err := newEnv(dir, bin)
	if err != nil {
		return result{err: err, elapsed: time.Since(start)}
	}
	defer e.close()

	for i, st := range s.steps {
		if verbose {
			fmt.Printf("        [%d/%d] %s\n", i+1, len(s.steps), st.name)
		}
		if err := st.fn(e); err != nil {
			return result{
				err:     fmt.Errorf("第 %d 步「%s」失败: %w", i+1, st.name, err),
				steps:   i,
				elapsed: time.Since(start),
			}
		}
	}
	return result{steps: len(s.steps), elapsed: time.Since(start)}
}

// env 是一个正在运行的编辑器实例及其伪终端。
type env struct {
	t             testingT
	dir           string
	bin           string
	width, height int
	tty           *os.File
	cmd           *exec.Cmd
	// 互斥锁是指针：场景之间要用值接管整个 env（换启动参数），
	// 锁被复制一次会立刻触发 go vet，也会让两个 goroutine 各有一把锁。
	mu     *sync.Mutex
	buf    bytes.Buffer
	screen *virtualScreen
	term   *termSnapshot
}

// testingT 是 *testing.T 的最小接口，方便断言复用。
type testingT interface {
	Helper()
	Fatalf(format string, args ...any)
}

// termSnapshot 是启动前的终端设置。
type termSnapshot struct {
	Lflag, Iflag, Oflag, Cflag uint32
}

// launch 起一个编辑器进程并连上伪终端。
// 它只管启动，不创建 env：后台读取的 goroutine 必须在 env 建好之后再起，
// 否则它抓到的是另一个对象，输出会写进没人读的缓冲区。
func launch(dir, bin, path string) (*os.File, *exec.Cmd, *termSnapshot, error) {
	var args []string
	if path != "" {
		args = append(args, path)
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	tty, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 30, Cols: 100})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("分配伪终端失败: %w", err)
	}

	// 必须在编辑器动终端之前取快照：恢复的目标是启动前的状态。
	var term *termSnapshot
	if before, err := unix.IoctlGetTermios(int(tty.Fd()), unix.TCGETS); err == nil {
		term = &termSnapshot{
			Lflag: before.Lflag, Iflag: before.Iflag,
			Oflag: before.Oflag, Cflag: before.Cflag,
		}
	}
	return tty, cmd, term, nil
}

// newEnv 不带参数启动编辑器。
func newEnv(dir, bin string) (*env, error) {
	e := &env{t: reporter{dir: dir}, dir: dir, bin: bin, mu: &sync.Mutex{},
		width: 100, height: 30}
	if err := e.startSession(""); err != nil {
		return nil, err
	}
	if err := e.expect("NORMAL", 15*time.Second); err != nil {
		_ = e.tty.Close()
		return nil, err
	}
	return e, nil
}

// newEnvWithFile 启动编辑器并打开一个文件。
func newEnvWithFile(dir, bin, path string) (*env, error) {
	e := &env{t: reporter{dir: dir}, dir: dir, bin: bin, mu: &sync.Mutex{},
		width: 100, height: 30}
	if err := e.startSession(path); err != nil {
		return nil, err
	}
	if err := e.expect("NORMAL", 15*time.Second); err != nil {
		_ = e.tty.Close()
		return nil, err
	}
	return e, nil
}

// startSession 在这个 env 上起进程，并开始后台读取输出。
//
// goroutine 抓的是 e 本身，所以重启时它依然有效——这一点很关键：
// 若换成一个新 env 对象，输出会写进另一个缓冲区，
// 所有断言都会因为读到空内容而失败，而且失败信息极具误导性。
func (e *env) startSession(path string) error {
	tty, cmd, term, err := launch(e.dir, e.bin, path)
	if err != nil {
		return err
	}
	e.tty, e.cmd, e.term = tty, cmd, term

	e.mu.Lock()
	e.buf.Reset()
	e.screen = newVirtualScreen(e.width, e.height)
	e.mu.Unlock()

	go func() {
		chunk := make([]byte, 8192)
		for {
			n, err := tty.Read(chunk)
			if n > 0 {
				e.mu.Lock()
				e.buf.Write(chunk[:n])
				// 同步喂给虚拟屏：断言要看的是「用户看到的画面」，
				// 不是渲染器的字节流。
				e.screen.apply(string(chunk[:n]))
				e.mu.Unlock()
			}
			if err != nil {
				return
			}
		}
	}()
	return nil
}

// reporter 让断言失败时带上工作目录，方便手工复现。
type reporter struct{ dir string }

func (r reporter) Helper() {}

func (r reporter) Fatalf(format string, args ...any) {
	panic(fmt.Sprintf(format, args...))
}

// ---- 终端交互 ----

// send 往伪终端写入一段原始字节。
func (e *env) send(data string) {
	e.t.Helper()
	if _, err := e.tty.WriteString(data); err != nil {
		panic(fmt.Errorf("写入 %q 失败: %w", data, err))
	}
}

// sendSlow 逐字符写入，字符之间留出间隔。
//
// 必须留间隔：一次把整串灌进去会挤在同一个读循环里，
// 主循环还没处理完上一键，下一键就已经到了，断言时序不可靠。
func (e *env) sendSlow(text string, gap time.Duration) {
	e.t.Helper()
	for _, r := range text {
		e.send(string(r))
		time.Sleep(gap)
	}
}

// keys 是命名键到字节序列的映射。
//
// 这些序列不是猜的，是用 `go test -v -run TestProbeSequences ./internal/screen/`
// 打印出来核对过的——同��个功能在不同终端上编码不一样：
// End 可能是 CSI F、4~ 或 8~；Ctrl+End 必须是 CSI 1;5 F，
// 光发 CSI F 的话编辑器收到的是普通 <end>，Ctrl 那一下根本没带上。
var keys = map[string]string{
	"enter":      "\r",
	"esc":        "\x1b",
	"backspace":  "\x7f",
	"tab":        "\t",
	"space":      " ",
	"delete":     "\x1b[3~",
	"up":         "\x1b[A",
	"down":       "\x1b[B",
	"right":      "\x1b[C",
	"left":       "\x1b[D",
	"home":       "\x1b[H",
	"end":        "\x1b[F",
	"pgup":       "\x1b[5~",
	"pgdown":     "\x1b[6~",
	"ctrl+a":     "\x01",
	"ctrl+c":     "\x03",
	"ctrl+g":     "\x07",
	"ctrl+s":     "\x13",
	"ctrl+r":     "\x12",
	"ctrl+home":  "\x1b[1;5H",
	"ctrl+end":   "\x1b[1;5F",
	"ctrl+right": "\x1b[1;5C",
	"ctrl+left":  "\x1b[1;5D",
	"ctrl+space": "\x00",
	"alt+1":      "\x1b1",
	"alt+2":      "\x1b2",
	"alt+9":      "\x1b9",
}

// key 按名字发送一个键。
func (e *env) key(name string) {
	e.t.Helper()
	data, ok := keys[name]
	if !ok {
		panic(fmt.Sprintf("未定义的键名 %q", name))
	}
	e.send(data)
}

// chord 发送一组按键，键之间留出间隔让主循环处理。
func (e *env) chord(names ...string) {
	e.t.Helper()
	for i, name := range names {
		e.key(name)
		if i < len(names)-1 {
			time.Sleep(120 * time.Millisecond)
		}
	}
}

// ctrlA 发送 C-a 前缀。
func (e *env) ctrlA() { e.chord("ctrl+a") }

// ---- 断言 ----

// expect 等待**重建后的屏幕**上出现 want。
//
// 断言屏幕而不是字节流：渲染器只重写变化的单元格，
// 光标从 1:2 走到 1:6 时字节流里只有 "e3 s4 i5 z6" 这样的碎片，
// "1:6" 作为连续字符串永远不出现。屏幕上的内容才是用户看到的。
func (e *env) expect(want string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if e.screenText().contains(want) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%s 内屏幕上没有 %q\n当前屏幕:\n%s\n原始输出末尾:\n%s",
				timeout, want, e.dump(), e.rawTail(600))
		}
		time.Sleep(40 * time.Millisecond)
	}
}

// expectGone 确认屏幕上**没有** unwanted。
//
// 用于「不该发生的事」：比如按了 C-space 之后文档里不该多出一个字符，
// 或者按 Esc 之后输入浮层不该还留在屏幕上。
func (e *env) expectGone(unwanted string) error {
	if text := e.screenText(); text.contains(unwanted) {
		return fmt.Errorf("屏幕上不该有 %q 却有\n当前屏幕:\n%s\n原始输出末尾(转义可见):\n%s",
			unwanted, e.dump(), e.rawTailEscaped(400))
	}
	return nil
}

// dump 返回带行号的屏幕转储，排查布局问题用。
func (e *env) dump() string {
	s := e.screenText()
	var b strings.Builder
	for r := 0; r < s.height; r++ {
		fmt.Fprintf(&b, "%2d|%s\n", r, s.line(r))
	}
	return b.String()
}

// rawTail 返回原始输出的末尾，用来对照屏幕。
func (e *env) rawTail(n int) string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return tail(e.buf.String(), n)
}

// rawTailEscaped 同 rawTail，但把 ESC 与控制字符显示出来。
// 直接打印原始字节时 ESC 会被当前终端吃掉，排查时什么都看不见。
func (e *env) rawTailEscaped(n int) string {
	raw := e.rawTail(n)
	var b strings.Builder
	for _, r := range raw {
		switch r {
		case 0x1b:
			b.WriteString("<ESC>")
		case '\r':
			b.WriteString("<CR>")
		case '\n':
			b.WriteString("<LF>\n")
		case '\t':
			b.WriteString("<TAB>")
		case 0x07:
			b.WriteString("<BEL>")
		default:
			if r < 0x20 {
				b.WriteString(fmt.Sprintf("<%02X>", r))
			} else {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

// screenText 返回当前屏幕快照。
func (e *env) screenText() *virtualScreen {
	e.mu.Lock()
	defer e.mu.Unlock()
	clone := *e.screen
	return &clone
}

// expectFG 等待第 row 行出现指定前景色。
//
// 用来断言语法高亮：字符本身看不出高亮，颜色才是唯一可观察的信号。
// fg 为空串说明该 kind 在当前主题里没配色，直接判成功。
func (e *env) expectFG(row int, fg string) error {
	fg = normalizeFG(fg)
	if fg == "" {
		return nil
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		s := e.screenText()
		if s.hasFG(row, fg) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("第 %d 行未出现颜色 %q\n当前屏幕:\n%s\n该行用到的颜色: %q",
				row, fg, s, s.fgRow(row))
		}
		time.Sleep(40 * time.Millisecond)
	}
}

// expectNoFG 断言整屏都没有出现指定前景色。
// 用于「纯文本不该有语法色」这类否定断言。
func (e *env) expectNoFG(fg string) error {
	fg = normalizeFG(fg)
	if fg == "" {
		return nil
	}
	// 留一点余量：断言要发生在绘制完成之后。
	e.quiet(500 * time.Millisecond)
	s := e.screenText()
	for row := 0; row < s.height; row++ {
		if s.hasFG(row, fg) {
			return fmt.Errorf("第 %d 行不该出现颜色 %q，却出现了\n当前屏幕:\n%s", row, fg, s)
		}
	}
	return nil
}

// expectNoFGRow 断言第 row 行没有出现指定前景色。
//
// 与 expectNoFG 的区别是范围：整屏否定常常过强。
// 比如文件里本来就有一行行注释，全屏都该有注释色，
// 这时能说明问题的只有「我刚编辑的那一行还有没有」。
func (e *env) expectNoFGRow(row int, fg string) error {
	fg = normalizeFG(fg)
	if fg == "" {
		return nil
	}
	e.quiet(400 * time.Millisecond)
	s := e.screenText()
	if s.hasFG(row, fg) {
		return fmt.Errorf("第 %d 行不该出现颜色 %q，却出现了\n当前屏幕:\n%s\n该行用到的颜色: %q",
			row, fg, s, s.fgRow(row))
	}
	return nil
}

// ctrlHome 回到文档开头，用于把光标准备到已知位置。
func (e *env) ctrlHome() {
	e.chord("ctrl+home")
}

// expectLine 等待第 row 行（从 0 开始）的内容等于 want。
// 用于验证结构性的位置关系，比如「第 3 行显示的就是文档第 3 行」。
func (e *env) expectLine(row int, want string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		s := e.screenText()
		if got := s.line(row); got == want {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%s 内第 %d 行 = %q, want %q\n当前屏幕:\n%s",
				timeout, row, s.line(row), want, s)
		}
		time.Sleep(40 * time.Millisecond)
	}
}

// quiet 等待一段时间，让主循环把按键全部处理完。
func (e *env) quiet(d time.Duration) { time.Sleep(d) }

// textEventually 轮询磁盘直到内容等于 want。
//
// 用于「保存之后验证磁盘」这类断言：保存完成的信号是瞬时的状态文字，
// 等那个文字再读文件会有竞态——文字已经出现、写入可能还没落盘；
// 而上一次保存留下的同一个文字还在屏幕上时，等它又会立刻返回，
// 于是读到的是旧内容。直接轮询文件本身最可靠，也正是用户关心的东西。
func (e *env) textEventually(path, want string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var last string
	for {
		got, err := os.ReadFile(path)
		if err == nil {
			last = string(got)
			if last == want {
				return nil
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%s 内 %s 的内容 = %q, want %q",
				timeout, filepath.Base(path), last, want)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// text 断言文档内容：打开文件并比对磁盘。
func (e *env) text(path, want string) error {
	got, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取 %s 失败: %w", path, err)
	}
	if string(got) != want {
		return fmt.Errorf("%s 的内容 = %q, want %q", filepath.Base(path), got, want)
	}
	return nil
}

// ---- 收尾 ----

// stop 正常终止编辑器并确认终端已恢复。
func (e *env) stop() error {
	_ = e.cmd.Process.Signal(syscall.SIGTERM)

	done := make(chan error, 1)
	go func() { done <- e.cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			// 被 SIGTERM 杀掉返回非零是正常的。
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				return fmt.Errorf("进程异常退出: %w", err)
			}
		}
	case <-time.After(15 * time.Second):
		_ = e.cmd.Process.Kill()
		<-done
		return errors.New("编辑器未在 15 秒内退出")
	}

	return e.checkTerminalRestored()
}

// checkTerminalRestored 确认终端设置被完全恢复。
//
// 终端留脏是最糟糕的失败模式：程序退出后用户后续所有命令行操作都会异常，
// 所以这条断言在每个场景结束时都要过一遍。
func (e *env) checkTerminalRestored() error {
	if e.term == nil {
		return nil // 没有基线可比，跳过
	}
	after, err := unix.IoctlGetTermios(int(e.tty.Fd()), unix.TCGETS)
	if err != nil {
		return fmt.Errorf("退出后读不到终端设置: %w", err)
	}
	checks := []struct {
		name   string
		before uint32
		after  uint32
	}{
		{"Lflag", e.term.Lflag, after.Lflag},
		{"Iflag", e.term.Iflag, after.Iflag},
		{"Oflag", e.term.Oflag, after.Oflag},
		{"Cflag", e.term.Cflag, after.Cflag},
	}
	for _, c := range checks {
		if c.before != c.after {
			return fmt.Errorf("终端 %s 未恢复: before=%#x after=%#x", c.name, c.before, c.after)
		}
	}
	return nil
}

// start 造一个文件并以它重启编辑器。
//
// 大部分场景都以「打开某个文件」开始。用一个固定的空文档起手再改成带文件启动，
// 多一道不必要的中途重启，失败时也更难判断是哪一步出的问题。
func (e *env) start(name, content string) error {
	path := filepath.Join(e.dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("造文件 %s 失败: %w", name, err)
	}
	return e.restart(path)
}

// restart 关闭当前实例并以新参数重启。
func (e *env) restart(path string) error {
	e.close()
	if err := e.startSession(path); err != nil {
		return err
	}
	if err := e.expect("NORMAL", 15*time.Second); err != nil {
		return err
	}
	return nil
}

// restartEmpty 不带参数重启。
func (e *env) restartEmpty() error { return e.restart("") }

// resize 调整伪终端窗口尺寸，并同步虚拟屏尺寸。
// 尺寸变了之后旧屏幕的内容不再可信，所以要清空重建。
func (e *env) resize(cols, rows int) error {
	if err := pty.Setsize(e.tty, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)}); err != nil {
		return fmt.Errorf("调整窗口尺寸失败: %w", err)
	}
	e.width, e.height = cols, rows
	e.mu.Lock()
	e.screen.resize(cols, rows)
	e.mu.Unlock()
	e.quiet(500 * time.Millisecond)
	return nil
}

// exitedNow 报告进程是否已经退出。
func (e *env) exitedNow() bool {
	return e.cmd.ProcessState != nil && e.cmd.ProcessState.Exited()
}

// waitExit 等进程退出并回收。
func (e *env) waitExit(timeout time.Duration) error {
	done := make(chan error, 1)
	go func() { done <- e.cmd.Wait() }()
	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		_ = e.cmd.Process.Kill()
		<-done
		return fmt.Errorf("编辑器未在 %s 内退出", timeout)
	}
}

// close 收尾：确保进程被杀掉、伪终端被关掉。
func (e *env) close() {
	if e.cmd != nil && e.cmd.Process != nil && e.cmd.ProcessState == nil {
		_ = e.cmd.Process.Kill()
	}
	if e.tty != nil {
		_ = e.tty.Close()
	}
}

// ---- 工具 ----

// buildBinary 编译出被测二进制。
func buildBinary() (string, error) {
	dir, err := os.MkdirTemp("", "go-studio-e2e-bin-*")
	if err != nil {
		return "", err
	}
	bin := filepath.Join(dir, "go-studio")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/go-studio")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return bin, nil
}

// tail 取字符串末尾 n 个字符，便于失败信息可读。
func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "..." + s[len(s)-n:]
}

func must(err error) {
	if err != nil {
		fatalf("%v", err)
	}
}

// names 返回全部场景名，按字典序，便于文档里列出。
func names() []string {
	var out []string
	for _, sc := range allScenarios() {
		out = append(out, sc.name)
	}
	sort.Strings(out)
	return out
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "e2e: "+format+"\n", args...)
	os.Exit(2)
}
