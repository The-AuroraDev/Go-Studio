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

	// Undo 已经把变更移入重做栈，不需要也不应该再手动放一次。
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
	stack.Undo()
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
	stack.Undo()

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

// ---- 连续删除的合并 ----

// pushDelete 往栈里记录一次删除，offset 是被删内容在文档中的起点。
func pushDelete(stack *UndoStack, offset int, removed string) {
	stack.Push(Change{Offset: offset, Removed: []byte(removed)})
}

// TestCoalesceBackspace 模拟连续退格：「abc|」连按三次退格。
// 每次删除的区间都紧贴在上一段的左边，应合并成一条。
func TestCoalesceBackspace(t *testing.T) {
	clock := &fakeClock{}
	stack := NewUndoStack(time.Second)
	stack.now = clock.Now

	// 文档 "abc"，光标在末尾。
	pushDelete(stack, 2, "c")
	clock.Advance(50 * time.Millisecond)
	pushDelete(stack, 1, "b")
	clock.Advance(50 * time.Millisecond)
	pushDelete(stack, 0, "a")

	if got := stack.Depth(); got != 1 {
		t.Fatalf("Depth = %d, want 1（三次退格应合并成一条）", got)
	}
	change, ok := stack.Undo()
	if !ok {
		t.Fatal("Undo 失败")
	}
	// 合并后应从起点 0 一次插回 "abc"，顺序必须与原文档一致。
	if change.Offset != 0 {
		t.Errorf("Offset = %d, want 0", change.Offset)
	}
	if got := string(change.Removed); got != "abc" {
		t.Errorf("Removed = %q, want %q", got, "abc")
	}
}

// TestCoalesceForwardDelete 模拟连续 Delete：「|abc」连按三次。
// 每次删除的区间都从同一位置开始，应合并成一条。
func TestCoalesceForwardDelete(t *testing.T) {
	clock := &fakeClock{}
	stack := NewUndoStack(time.Second)
	stack.now = clock.Now

	pushDelete(stack, 0, "a")
	clock.Advance(50 * time.Millisecond)
	pushDelete(stack, 0, "b")
	clock.Advance(50 * time.Millisecond)
	pushDelete(stack, 0, "c")

	if got := stack.Depth(); got != 1 {
		t.Fatalf("Depth = %d, want 1（三次前向删除应合并成一条）", got)
	}
	change, _ := stack.Undo()
	if change.Offset != 0 {
		t.Errorf("Offset = %d, want 0", change.Offset)
	}
	if got := string(change.Removed); got != "abc" {
		t.Errorf("Removed = %q, want %q", got, "abc")
	}
}

// TestCoalesceDeleteRespectsWindow 停顿超过窗口后必须另起一条。
func TestCoalesceDeleteRespectsWindow(t *testing.T) {
	clock := &fakeClock{}
	stack := NewUndoStack(100 * time.Millisecond)
	stack.now = clock.Now

	pushDelete(stack, 1, "b")
	clock.Advance(200 * time.Millisecond)
	pushDelete(stack, 0, "a")

	if got := stack.Depth(); got != 2 {
		t.Errorf("Depth = %d, want 2（超出合并窗口不应合并）", got)
	}
}

// TestNoCoalesceDeleteWhenNotAdjacent 中间隔了别的编辑就不该合并。
func TestNoCoalesceDeleteWhenNotAdjacent(t *testing.T) {
	clock := &fakeClock{}
	stack := NewUndoStack(time.Hour)
	stack.now = clock.Now

	pushDelete(stack, 5, "x")
	clock.Advance(time.Millisecond)
	pushDelete(stack, 0, "y")

	if got := stack.Depth(); got != 2 {
		t.Errorf("Depth = %d, want 2（不相邻的删除不应合并）", got)
	}
}

// TestNoCoalesceDeleteWithInsert 删除与插入类型不同，绝不合并。
func TestNoCoalesceDeleteWithInsert(t *testing.T) {
	clock := &fakeClock{}
	stack := NewUndoStack(time.Hour)
	stack.now = clock.Now

	pushDelete(stack, 3, "x")
	clock.Advance(time.Millisecond)
	// 紧挨着删除区间的右侧插入，位置相邻但类型不同。
	stack.Push(Change{Offset: 3, Inserted: []byte("y")})

	if got := stack.Depth(); got != 2 {
		t.Errorf("Depth = %d, want 2（删除与插入不应合并）", got)
	}
}

// TestCoalesceDeleteReplaceNotMerged 替换含插入，不与纯删除合并。
func TestCoalesceDeleteReplaceNotMerged(t *testing.T) {
	clock := &fakeClock{}
	stack := NewUndoStack(time.Hour)
	stack.now = clock.Now

	pushDelete(stack, 3, "x")
	clock.Advance(time.Millisecond)
	stack.Push(Change{Offset: 3, Removed: []byte("y"), Inserted: []byte("z")})

	if got := stack.Depth(); got != 2 {
		t.Errorf("Depth = %d, want 2（替换不与删除合并）", got)
	}
}

// TestUndoRedoRedoRoundTrip 固化重做后仍可再次撤销。
// 曾经的行为是 Redo 把变更从重做栈弹掉却不放回撤销栈，
// 于是重做一次之后就再也撤销不了它——这是编辑器里非常刺眼的 bug。
func TestUndoRedoRedoRoundTrip(t *testing.T) {
	b := New([]byte("x"))
	stack := NewUndoStack(0)
	stack.Push(Change{Offset: 1, Inserted: []byte("y")})
	b.Replace(1, 0, []byte("y"))

	// 撤销 → 重做 → 再撤销，文本必须回到重做前的状态。
	change, _ := stack.Undo()
	b.Undo(change)
	if b.String() != "x" {
		t.Fatalf("撤销后 = %q, want %q", b.String(), "x")
	}

	change, _ = stack.Redo()
	b.Redo(change)
	if b.String() != "xy" {
		t.Fatalf("重做后 = %q, want %q", b.String(), "xy")
	}
	if got := stack.Depth(); got != 1 {
		t.Errorf("重做后 Depth = %d, want 1（变更应回到撤销栈）", got)
	}

	change, _ = stack.Undo()
	b.Undo(change)
	if b.String() != "x" {
		t.Errorf("再次撤销后 = %q, want %q", b.String(), "x")
	}
	if got := stack.RedoDepth(); got != 1 {
		t.Errorf("再次撤销后 RedoDepth = %d, want 1", got)
	}
}

// TestRedoStackLifoOrder 多次撤销后，重做必须按后进先出还原。
func TestRedoStackLifoOrder(t *testing.T) {
	b := New([]byte(""))
	stack := NewUndoStack(0)
	for _, text := range []string{"a", "b", "c"} {
		change := Change{Offset: b.Len(), Inserted: []byte(text)}
		stack.Push(change)
		b.Replace(change.Offset, len(change.Removed), change.Inserted)
	}
	if b.String() != "abc" {
		t.Fatalf("编辑后 = %q, want %q", b.String(), "abc")
	}

	for _, want := range []string{"ab", "a", ""} {
		change, ok := stack.Undo()
		if !ok {
			t.Fatalf("撤销到 %q 时栈空", want)
		}
		b.Undo(change)
		if b.String() != want {
			t.Fatalf("撤销后 = %q, want %q", b.String(), want)
		}
	}

	// 重做应逆序还原："a"、"ab"、"abc"。
	for _, want := range []string{"a", "ab", "abc"} {
		change, ok := stack.Redo()
		if !ok {
			t.Fatalf("重做到 %q 时重做栈空", want)
		}
		b.Redo(change)
		if b.String() != want {
			t.Fatalf("重做后 = %q, want %q", b.String(), want)
		}
	}
}

// TestRedoDepthCountsEveryUndo 撤销栈全部退空后，重做深度应等于原深度。
func TestRedoDepthCountsEveryUndo(t *testing.T) {
	stack := NewUndoStack(0)
	for i := 0; i < 5; i++ {
		stack.Push(Change{Offset: i, Inserted: []byte("x")})
	}
	for stack.Depth() > 0 {
		if _, ok := stack.Undo(); !ok {
			t.Fatal("Undo 失败")
		}
	}
	if got := stack.RedoDepth(); got != 5 {
		t.Errorf("RedoDepth = %d, want 5", got)
	}
}
