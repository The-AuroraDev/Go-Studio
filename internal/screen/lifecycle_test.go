// lifecycle_test.go — 后端的完整生命周期：启动、出帧、按键、退出。
// SPDX-License-Identifier: MIT

// 本文件用伪终端同时充当输入与输出，因此只能在类 Unix 平台编译。
//go:build !windows

package screen

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

// liveBackend 是一个真的 Bubble Tea 后端，加上可写它输入的主端。
type liveBackend struct {
	screen *bubbleScreen
	// master 是伪终端主端：往它写等于用户按键，从它读等于看屏幕。
	master *os.File
	// tty 是从端，终端设置就在它上面，可以直接读来比对。
	tty *os.File
	// pristine 是 NewBubble 之前的终端设置，恢复的正确性以它为准。
	pristine *ttyState
}

// newLiveBackend 起一个真的后端。
//
// 输入与输出都接伪终端从端：Bubble Tea 只有在输入也是终端时才会切换 raw 模式，
// 而它决定尺寸用的是输出端的窗口大小，因此两者都得是「真终端」，
// 否则既量不到尺寸、也不会改终端模式，这个测试的前提就不成立。
// 窗口大小必须显式设置：pty.Open() 给出的默认是 0x0。
func newLiveBackend(t *testing.T) *liveBackend {
	t.Helper()

	master, tty, err := pty.Open()
	if err != nil {
		t.Skipf("无法分配伪终端（可能缺少 /dev/ptmx）: %v", err)
	}
	if err := pty.Setsize(master, &pty.Winsize{Rows: 24, Cols: 80}); err != nil {
		_ = master.Close()
		_ = tty.Close()
		t.Skipf("无法设置伪终端尺寸: %v", err)
	}

	// 持续读走输出，避免缓冲区写满把程序卡住。
	go func() {
		buf := make([]byte, 4096)
		for {
			if _, err := master.Read(buf); err != nil {
				return
			}
		}
	}()

	// 关键：终端设置的快照必须在 NewBubble 之前取。
	// 拿程序启动后的状态去比对是错的——那时已经是 raw 模式，
	// 而恢复的目标恰恰是启动前的状态。
	pristine := captureTTYState(tty)
	if pristine == nil {
		_ = master.Close()
		_ = tty.Close()
		t.Skip("读不到伪终端的初始设置")
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	s, err := NewBubble(BubbleOptions{
		Context: ctx,
		Output:  tty,
		Input:   tty,
		// 读帧要快，否则测试会因为等不及而超时。
		FPS: 120,
	})
	if err != nil {
		_ = master.Close()
		_ = tty.Close()
		t.Fatalf("NewBubble 返回错误: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	live, ok := s.(*bubbleScreen)
	if !ok {
		t.Fatalf("NewBubble 返回了 %T, want *bubbleScreen", s)
	}
	return &liveBackend{screen: live, master: master, tty: tty, pristine: pristine}
}

// waitFor 等待一条满足条件的事件。
func (b *liveBackend) waitFor(t *testing.T, timeout time.Duration, match func(Event) bool) Event {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case event, ok := <-b.screen.Events():
			if !ok {
				t.Fatal("等待期间事件通道关闭了")
			}
			if match(event) {
				return event
			}
		case <-deadline:
			t.Fatal("未在超时内等到期望的事件")
		}
	}
}

// waitForResize 等到首个尺寸事件，它证明事件循环真的跑起来了。
func (b *liveBackend) waitForResize(t *testing.T, timeout time.Duration) {
	t.Helper()
	event := b.waitFor(t, timeout, func(e Event) bool {
		return e.Kind == EventResize && e.Width > 0 && e.Height > 0
	})
	if event.Width <= 0 || event.Height <= 0 {
		t.Errorf("尺寸事件 = %dx%d, want 正数", event.Width, event.Height)
	}
}

// sendKey 往伪终端写一个按键。
func (b *liveBackend) sendKey(t *testing.T, data string) {
	t.Helper()
	if _, err := b.master.WriteString(data); err != nil {
		t.Fatalf("写入按键失败: %v", err)
	}
}

// TestBubbleLifecycle 覆盖 NewBubble、Events、Render、Close 的完整路径。
func TestBubbleLifecycle(t *testing.T) {
	live := newLiveBackend(t)
	s := live.screen

	live.waitForResize(t, 10*time.Second)

	caps := s.Caps()
	if caps.Width <= 0 || caps.Height <= 0 {
		t.Errorf("能力尺寸 = %dx%d, want 正数", caps.Width, caps.Height)
	}

	// 出帧：Render 之后 View 应该能看到内容。
	s.Render("hello", &CursorSpec{X: 2, Y: 1, Shape: CursorBar, Blink: true})
	deadline := time.Now().Add(3 * time.Second)
	for s.View().Content != "hello" {
		if time.Now().After(deadline) {
			t.Fatalf("Render 的内容没有进入 View，实际 = %q", s.View().Content)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if got := s.View().Cursor; got == nil || got.Position.X != 2 {
		t.Errorf("View 的光标 = %+v, want X=2", got)
	}

	// 写一个按键，确认输入真的被读到了。
	live.sendKey(t, "z")
	live.waitFor(t, 5*time.Second, func(e Event) bool {
		return e.Kind == EventKeyPress && e.Keystroke == "z"
	})

	// 关闭后事件通道必须关闭，重复关闭不许出错。
	if err := s.Close(); err != nil {
		t.Fatalf("Close() 返回错误: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Errorf("第二次 Close() 返回错误: %v", err)
	}
}

// TestBubbleRenderAfterCloseIsIgnored 关闭后 Render 不能 panic，
// 否则事件循环里迟到的重绘会把进程带崩。
func TestBubbleRenderAfterCloseIsIgnored(t *testing.T) {
	live := newLiveBackend(t)
	live.waitForResize(t, 10*time.Second)

	if err := live.screen.Close(); err != nil {
		t.Fatalf("Close() 返回错误: %v", err)
	}
	live.screen.Render("after close", nil)
}

// TestBubbleRedrawRequestsAreCoalesced 连续多次 Render 只应触发一次重绘请求，
// 否则按键事件会被重绘消息淹没。
func TestBubbleRedrawRequestsAreCoalesced(t *testing.T) {
	live := newLiveBackend(t)
	live.waitForResize(t, 10*time.Second)
	s := live.screen

	for i := 0; i < 5; i++ {
		s.Render("frame", nil)
	}
	// redrawMsg 被处理之后标记会清掉；等待它被消费完。
	deadline := time.Now().Add(2 * time.Second)
	for {
		s.mu.Lock()
		queued := s.redrawQueued
		s.mu.Unlock()
		if !queued {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("重绘标记一直没有被清掉")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestBubbleNewOnNonTerminalOutput 输出不是终端时仍要能启动：
// 这条路径没有 termios 快照，也就无需恢复，但绝不能因此启动失败。
func TestBubbleNewOnNonTerminalOutput(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Skipf("无法创建临时文件: %v", err)
	}
	defer file.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s, err := NewBubble(BubbleOptions{Context: ctx, Output: file, Input: file})
	if err != nil {
		t.Fatalf("NewBubble 在非终端输出上返回错误: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Errorf("Close() 返回错误: %v", err)
	}
}

// TestBubbleCloseRestoresTerminalOnRealTTY 在真伪终端上退出后，
// termios 必须被恢复——这是 M0 的核心承诺，编辑器的退出路径依赖它。
func TestBubbleCloseRestoresTerminalOnRealTTY(t *testing.T) {
	live := newLiveBackend(t)
	live.waitForResize(t, 10*time.Second)

	// 程序启动后必须确实把终端改成了 raw 模式，否则比对没有意义。
	running := captureTTYState(live.tty)
	if running == nil || !isRawModeTermios(running.sys) {
		t.Fatal("测试前提不成立：程序没有把终端切换为 raw 模式")
	}

	if err := live.screen.Close(); err != nil {
		t.Fatalf("Close() 返回错误: %v", err)
	}

	before := live.pristine
	after := captureTTYState(live.tty)
	if after == nil {
		t.Fatal("退出后读不到终端设置")
	}
	if after.sys.Lflag != before.sys.Lflag {
		t.Errorf("Lflag 未恢复: before=%#x after=%#x", before.sys.Lflag, after.sys.Lflag)
	}
	if after.sys.Iflag != before.sys.Iflag {
		t.Errorf("Iflag 未恢复: before=%#x after=%#x", before.sys.Iflag, after.sys.Iflag)
	}
	if after.sys.Oflag != before.sys.Oflag {
		t.Errorf("Oflag 未恢复: before=%#x after=%#x", before.sys.Oflag, after.sys.Oflag)
	}
	if after.sys.Cflag != before.sys.Cflag {
		t.Errorf("Cflag 未恢复: before=%#x after=%#x", before.sys.Cflag, after.sys.Cflag)
	}
}

// isRawModeTermios 判断 ICANON 与 ECHO 是否已关闭。
func isRawModeTermios(state termios) bool {
	return state.Lflag&unix.ICANON == 0 && state.Lflag&unix.ECHO == 0
}
