// undo_test.go — 撤销栈的合并、撤销、重做行为测试。
// SPDX-License-Identifier: MIT

package buffer

import (
	"testing"
	"time"
)

// fakeClock 是可控时钟，用来确定性地测试合并窗口。
type fakeClock struct {
	now time.Time
}

// Advance 把时钟向前拨。
func (c *fakeClock) Advance(d time.Duration) {
	c.now = c.now.Add(d)
}

// Now 返回当前时间。
func (c *fakeClock) Now() time.Time {
	return c.now
}

func TestUndoRedoRoundTrip(t *testing.T) {
	b := New([]byte("hello"))
	stack := NewUndoStack(0)

	stack.Push(Change{Offset: 5, Inserted: []byte(" world")})
	b.Replace(5, 0, []byte(" world"))
	if b.String() != "hello world" {
		t.Fatalf("编辑后 = %q", b.String())
	}

	change, ok := stack.Undo()
	if !ok {
		t.Fatal("Undo 应返回可撤销的变更")
	}
	b.Undo(change)
	if b.String() != "hello" {
		t.Errorf("撤销后 = %q, want %q", b.String(), "hello")
	}

	stack.PushRedo(change)
	change, ok = stack.Redo()
	if !ok {
		t.Fatal("Redo 应返回可重做的变更")
	}
	b.Redo(change)
	if b.String() != "hello world" {
		t.Errorf("重做后 = %q, want %q", b.String(), "hello world")
	}
}

func TestUndoEmptyStack(t *testing.T) {
	stack := NewUndoStack(0)
	if _, ok := stack.Undo(); ok {
		t.Error("空栈 Undo 应返回 false")
	}
	if _, ok := stack.Redo(); ok {
		t.Error("空栈 Redo 应返回 false")
	}
}

func TestNewChangeClearsRedo(t *testing.T) {
	stack := NewUndoStack(0)
	stack.Push(Change{Offset: 0, Inserted: []byte("a")})
	change, _ := stack.Undo()
	stack.PushRedo(change)
	if got := stack.RedoDepth(); got != 1 {
		t.Fatalf("RedoDepth = %d, want 1", got)
	}

	// 编辑器语义：撤销之后做了新编辑，重做栈必须作废。
	stack.Push(Change{Offset: 0, Inserted: []byte("b")})
	if got := stack.RedoDepth(); got != 0 {
		t.Errorf("新变更后 RedoDepth = %d, want 0", got)
	}
}

func TestCoalesceConsecutiveInserts(t *testing.T) {
	clock := &fakeClock{}
	stack := NewUndoStack(time.Second)
	stack.now = clock.Now

	// 模拟逐字符输入 "abc"。
	for i, c := range []string{"a", "b", "c"} {
		stack.Push(Change{Offset: i, Inserted: []byte(c)})
		clock.Advance(50 * time.Millisecond)
	}

	if got := stack.Depth(); got != 1 {
		t.Fatalf("Depth = %d, want 1（三次连续输入应合并成一条）", got)
	}

	change, _ := stack.Undo()
	if got := string(change.Inserted); got != "abc" {
		t.Errorf("合并后的插入内容 = %q, want %q", got, "abc")
	}
}

func TestNoCoalesceAcrossTimeGap(t *testing.T) {
	clock := &fakeClock{}
	stack := NewUndoStack(100 * time.Millisecond)
	stack.now = clock.Now

	stack.Push(Change{Offset: 0, Inserted: []byte("a")})
	clock.Advance(200 * time.Millisecond) // 超过合并窗口
	stack.Push(Change{Offset: 1, Inserted: []byte("b")})

	if got := stack.Depth(); got != 2 {
		t.Errorf("Depth = %d, want 2（超时后应另起一条）", got)
	}
}

func TestNoCoalesceWhenNotAdjacent(t *testing.T) {
	clock := &fakeClock{}
	stack := NewUndoStack(time.Hour)
	stack.now = clock.Now

	stack.Push(Change{Offset: 0, Inserted: []byte("a")})
	stack.Push(Change{Offset: 5, Inserted: []byte("b")}) // 位置不连续

	if got := stack.Depth(); got != 2 {
		t.Errorf("Depth = %d, want 2（位置不连续不应合并）", got)
	}
}

func TestNoCoalesceForDelete(t *testing.T) {
	clock := &fakeClock{}
	stack := NewUndoStack(time.Hour)
	stack.now = clock.Now

	// 先删除，再紧接着插入。删除之后的新输入应当独立成一条。
	stack.Push(Change{Offset: 0, Removed: []byte("x")})
	stack.Push(Change{Offset: 0, Inserted: []byte("y")})

	if got := stack.Depth(); got != 2 {
		t.Errorf("Depth = %d, want 2（删除之后的输入不应并入删除）", got)
	}
}

func TestCoalesceDisabledWithZeroWindow(t *testing.T) {
	clock := &fakeClock{}
	stack := NewUndoStack(0)
	stack.now = clock.Now

	stack.Push(Change{Offset: 0, Inserted: []byte("a")})
	stack.Push(Change{Offset: 1, Inserted: []byte("b")})

	if got := stack.Depth(); got != 2 {
		t.Errorf("Depth = %d, want 2（窗口为 0 时不应合并）", got)
	}
}

func TestPushClonesCallerBuffers(t *testing.T) {
	stack := NewUndoStack(0)
	inserted := []byte("hello")
	stack.Push(Change{Offset: 0, Inserted: inserted})

	// 调用方复用切片，栈内记录不能被影响。
	copy(inserted, "xxxxx")

	change, _ := stack.Undo()
	if got := string(change.Inserted); got != "hello" {
		t.Errorf("栈内插入内容 = %q, want %q（必须与调用方切片隔离）", got, "hello")
	}
}

func TestResetClearsEverything(t *testing.T) {
	stack := NewUndoStack(0)
	stack.Push(Change{Offset: 0, Inserted: []byte("a")})
	change, _ := stack.Undo()
	stack.PushRedo(change)

	stack.Reset()
	if stack.Depth() != 0 || stack.RedoDepth() != 0 {
		t.Errorf("Reset 后 Depth=%d RedoDepth=%d, want 0 0", stack.Depth(), stack.RedoDepth())
	}
}

func TestChangeIsInsert(t *testing.T) {
	if !(Change{Inserted: []byte("x")}).IsInsert() {
		t.Error("纯插入的 Change 应 IsInsert 为 true")
	}
	if (Change{Removed: []byte("x")}).IsInsert() {
		t.Error("含删除的 Change 应 IsInsert 为 false")
	}
}

// TestUndoRestoresBufferAfterManyEdits 端到端验证撤销能把缓冲还原。
func TestUndoRestoresBufferAfterManyEdits(t *testing.T) {
	b := New([]byte("package main\n\nfunc main() {}\n"))
	stack := NewUndoStack(0)

	edits := []struct {
		offset, removed int
		inserted        string
	}{
		{0, 0, "// 注释\n"},
		{len(b.Text()) - 1, 0, "\n\n// 结尾\n"},
		{0, len("// 注释\n"), ""},
		{5, 3, "PACKAGE"},
	}

	for _, edit := range edits {
		stack.Push(Change{
			Offset:   edit.offset,
			Removed:  b.Text()[edit.offset : edit.offset+edit.removed],
			Inserted: []byte(edit.inserted),
		})
		b.Replace(edit.offset, edit.removed, []byte(edit.inserted))
	}

	for want := len(edits) - 1; want >= 0; want-- {
		change, ok := stack.Undo()
		if !ok {
			t.Fatalf("第 %d 次撤销失败", want)
		}
		b.Undo(change)
	}

	if got := b.String(); got != "package main\n\nfunc main() {}\n" {
		t.Errorf("全部撤销后 = %q, want 原文", got)
	}
}
