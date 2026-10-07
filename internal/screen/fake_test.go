// fake_test.go — 测试替身 Fake 的单元测试。
// SPDX-License-Identifier: MIT

// 本文件不涉及 termios，不需要平台约束。

package screen

import (
	"strings"
	"testing"
)

func TestFakeCaps(t *testing.T) {
	want := Caps{Width: 100, Height: 30, Terminal: "xterm"}
	f := NewFake(want)

	if got := f.Caps(); got != want {
		t.Errorf("Caps() = %+v, want %+v", got, want)
	}
}

// TestFakeSetCaps 覆盖能力值用于模拟 resize 之后的变化。
func TestFakeSetCaps(t *testing.T) {
	f := NewFake(Caps{Width: 80, Height: 24})
	f.SetCaps(Caps{Width: 120, Height: 40})

	if got := f.Caps(); got.Width != 120 || got.Height != 40 {
		t.Errorf("Caps() = %+v, want 120x40", got)
	}
}

// TestFakeLastFrameOnEmptyFrames 一帧都没送过时必须返回零值而不是 panic。
func TestFakeLastFrameOnEmptyFrames(t *testing.T) {
	f := NewFake(Caps{})

	frame := f.LastFrame()
	if frame.Content != "" {
		t.Errorf("空 Fake 的帧内容 = %q, want 空", frame.Content)
	}
	if frame.Cursor != nil {
		t.Errorf("空 Fake 的光标 = %+v, want nil", frame.Cursor)
	}
}

func TestFakeFramesIsACopy(t *testing.T) {
	f := NewFake(Caps{})
	f.Render("first", nil)

	frames := f.Frames()
	if len(frames) != 1 {
		t.Fatalf("帧数 = %d, want 1", len(frames))
	}
	frames[0].Content = "tampered"

	if got := f.LastFrame().Content; got != "first" {
		t.Errorf("修改 Frames() 的返回值污染了记录: %q", got)
	}
}

// TestFakeCloseIsIdempotent 重复关闭不该 panic，事件通道也只关一次。
func TestFakeCloseIsIdempotent(t *testing.T) {
	f := NewFake(Caps{})
	if err := f.Close(); err != nil {
		t.Fatalf("Close() 返回错误: %v", err)
	}
	// 第二次 Close 若重复关闭通道会 panic。
	if err := f.Close(); err != nil {
		t.Errorf("第二次 Close() 返回错误: %v", err)
	}
}

// TestFakeEmitAfterClose 关闭后投递事件必须失败而不是 panic 在已关闭的通道上。
func TestFakeEmitAfterClose(t *testing.T) {
	f := NewFake(Caps{})
	if !f.EmitKey("a") {
		t.Fatal("关闭前 EmitKey 应成功")
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if f.EmitKey("a") {
		t.Error("关闭后 EmitKey 应返回 false")
	}
	if f.Emit(Event{Kind: EventResize}) {
		t.Error("关闭后 Emit 应返回 false")
	}
	// 关闭也不该再记录帧。
	f.Render("after close", nil)
	if got := len(f.Frames()); got != 0 {
		t.Errorf("关闭后记录了 %d 帧，want 0", got)
	}
}

// TestFakeEmitRespectsBackpressure 通道满时返回 false，不阻塞。
func TestFakeEmitRespectsBackpressure(t *testing.T) {
	f := NewFake(Caps{})
	// 容量 64，填满后必须返回 false。
	for i := 0; i < 64; i++ {
		if !f.EmitKey("a") {
			t.Fatalf("第 %d 次投递失败，通道不该这么满", i)
		}
	}
	if f.EmitKey("a") {
		t.Error("通道满时 EmitKey 应返回 false")
	}
}

func TestFakeRenderWithNilCursor(t *testing.T) {
	f := NewFake(Caps{})
	f.Render("frame", nil)

	if got := f.LastFrame().Cursor; got != nil {
		t.Errorf("光标 = %+v, want nil", got)
	}
}

func TestFakeCursorCopiesForEachFrame(t *testing.T) {
	f := NewFake(Caps{})
	cursor := &CursorSpec{X: 1, Y: 2}
	f.Render("a", cursor)

	cursor.X = 99
	if got := f.LastFrame().Cursor.X; got != 1 {
		t.Errorf("修改传入的光标污染了已记录的帧: X = %d, want 1", got)
	}
}

func TestFakeString(t *testing.T) {
	f := NewFake(Caps{})
	if got := f.String(); !strings.Contains(got, "frames: 0") {
		t.Errorf("String() = %q, want 含 frames: 0", got)
	}
	f.Render("a", nil)
	if got := f.String(); !strings.Contains(got, "frames: 1") {
		t.Errorf("String() = %q, want 含 frames: 1", got)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if got := f.String(); !strings.Contains(got, "closed: true") {
		t.Errorf("String() = %q, want 含 closed: true", got)
	}
}

// TestFakeEventsChannelClosesOnClose 关闭后事件通道必须被关闭，
// 上层事件循环靠这个收到「终端已断开」。
func TestFakeEventsChannelClosesOnClose(t *testing.T) {
	f := NewFake(Caps{})
	events := f.Events()
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	for range events {
		// 排空残留事件，直到通道关闭。
	}
	// 再次 range 会立即结束，不会阻塞。
	select {
	case _, ok := <-events:
		if ok {
			t.Error("关闭后事件通道仍在投递事件")
		}
	default:
		t.Error("关闭后事件通道没有被关闭")
	}
}
