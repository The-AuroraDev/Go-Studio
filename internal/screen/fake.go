// fake.go — 测试用 Screen 实现，把帧与事件留在内存里供断言。
// SPDX-License-Identifier: MIT

package screen

import (
	"fmt"
	"sync"
)

// Frame 是一次 Render 的记录。
type Frame struct {
	// Content 是送出的帧内容。
	Content string
	// Cursor 是当时的光标位置，nil 表示隐藏。
	Cursor *CursorSpec
}

// Fake 是可编程的 Screen 替身，用于脱离真实终端测试上层逻辑。
type Fake struct {
	events chan Event
	frames []Frame

	mu     sync.Mutex
	caps   Caps
	closed bool
}

// NewFake 创建一个 Fake。caps 是 Caps 将要返回的值。
func NewFake(caps Caps) *Fake {
	return &Fake{
		events: make(chan Event, 64),
		caps:   caps,
	}
}

// Caps 返回设定的能力值。
func (f *Fake) Caps() Caps {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.caps
}

// SetCaps 覆盖能力值，用于模拟 resize 之后的能力变化。
func (f *Fake) SetCaps(caps Caps) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.caps = caps
}

// Render 记录一帧。
func (f *Fake) Render(frame string, cursor *CursorSpec) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return
	}
	f.frames = append(f.frames, Frame{Content: frame, Cursor: cursorOrNil(cursor)})
}

// Frames 返回已记录的帧。调用方不得修改返回值。
func (f *Fake) Frames() []Frame {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]Frame, len(f.frames))
	copy(out, f.frames)
	return out
}

// LastFrame 返回最后一帧，帧数为零时返回零值。
func (f *Fake) LastFrame() Frame {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.frames) == 0 {
		return Frame{}
	}
	return f.frames[len(f.frames)-1]
}

// Emit 投递一个事件。通道满时返回 false。
func (f *Fake) Emit(event Event) bool {
	f.mu.Lock()
	closed := f.closed
	f.mu.Unlock()
	if closed {
		return false
	}
	select {
	case f.events <- event:
		return true
	default:
		return false
	}
}

// EmitKey 投递一次按键按下。
func (f *Fake) EmitKey(keystroke string) bool {
	return f.Emit(Event{Kind: EventKeyPress, Keystroke: keystroke})
}

// Events 返回事件通道。
func (f *Fake) Events() <-chan Event {
	return f.events
}

// Close 关闭事件通道，之后的 Render 与 Emit 都会被忽略。
func (f *Fake) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return nil
	}
	f.closed = true
	close(f.events)
	return nil
}

// cursorOrNil 复制一份光标，避免调用方后续修改影响已记录的帧。
func cursorOrNil(cursor *CursorSpec) *CursorSpec {
	if cursor == nil {
		return nil
	}
	copied := *cursor
	return &copied
}

// String 让 Fake 在测试失败信息里可读。
func (f *Fake) String() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return fmt.Sprintf("Fake{frames: %d, closed: %t}", len(f.frames), f.closed)
}
