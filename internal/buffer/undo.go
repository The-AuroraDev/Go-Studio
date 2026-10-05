// undo.go — 撤销与重做：以变更记录为基础，插入类操作可合并成一条。
// SPDX-License-Identifier: MIT

package buffer

import (
	"bytes"
	"time"
)

// Change 记录一次编辑的逆操作信息。
// Offset 是编辑起点，Removed 是被删除的内容，Inserted 是插入的内容。
// 撤销时执行 Replace(Offset, len(Inserted), Removed) 即可回到编辑前。
type Change struct {
	Offset   int
	Removed  []byte
	Inserted []byte
}

// IsInsert 判断这次变更是否为纯插入。
func (c Change) IsInsert() bool {
	return len(c.Removed) == 0
}

// UndoStack 管理撤销与重做历史。
// 它只记录变更，不直接操作缓冲；由调用方负责把 Change 应用到 Buffer 上。
type UndoStack struct {
	undo []Change
	redo []Change

	// 合并策略
	coalesceWindow time.Duration
	now            func() time.Time

	// 上一条变更的合并判定所需信息
	lastChange Change
	hasLast    bool
	// mergedAt 是栈顶变更最后一次被合并扩展的时刻。
	mergedAt time.Time
}

// NewUndoStack 创建撤销栈。coalesceWindow 是两次输入之间允许合并的最大间隔，
// 小于等于 0 时关闭合并。
func NewUndoStack(coalesceWindow time.Duration) *UndoStack {
	return &UndoStack{
		coalesceWindow: coalesceWindow,
		now:            time.Now,
	}
}

// Push 记录一次变更，并清空重做栈。
// 连续插入会被合并：位置紧邻上一次插入的末尾且间隔在合并窗口内时，
// 两条变更合并成一条，这样按一次 Ctrl+Z 撤掉刚输入的一整串字符。
func (s *UndoStack) Push(change Change) {
	change.Removed = bytes.Clone(change.Removed)
	change.Inserted = bytes.Clone(change.Inserted)

	// 任何新变更都会作废重做栈，这是所有编辑器的通用行为。
	s.redo = s.redo[:0]

	if s.canCoalesce(change) {
		s.lastChange.Inserted = append(s.lastChange.Inserted, change.Inserted...)
		s.undo[len(s.undo)-1] = s.lastChange
		s.mergedAt = s.now()
		return
	}

	s.undo = append(s.undo, change)
	s.lastChange = change
	s.hasLast = true
	s.mergedAt = s.now()
}

// canCoalesce 判断新变更能否与栈顶合并。
func (s *UndoStack) canCoalesce(change Change) bool {
	if !s.hasLast || s.coalesceWindow <= 0 || len(s.undo) == 0 {
		return false
	}
	// 只合并纯插入。删除或替换后接着输入，不应该被当成同一次操作。
	if !s.lastChange.IsInsert() || !change.IsInsert() {
		return false
	}
	// 必须紧接在栈顶插入的末尾，否则是一次不相干的新输入。
	if change.Offset != s.lastChange.Offset+len(s.lastChange.Inserted) {
		return false
	}
	// 合并窗口从栈顶最后一次被扩展的时刻算起，这样持续输入会一直合并，
	// 停顿超过窗口后才另起一条。
	return s.now().Sub(s.mergedAt) <= s.coalesceWindow
}

// Undo 取出并返回最近一条可撤销的变更，没有则返回 false。
func (s *UndoStack) Undo() (Change, bool) {
	if len(s.undo) == 0 {
		return Change{}, false
	}
	change := s.undo[len(s.undo)-1]
	s.undo = s.undo[:len(s.undo)-1]
	s.hasLast = false
	return change, true
}

// Redo 取出并返回最近一条可重做的变更，没有则返回 false。
func (s *UndoStack) Redo() (Change, bool) {
	if len(s.redo) == 0 {
		return Change{}, false
	}
	change := s.redo[len(s.redo)-1]
	s.redo = s.redo[:len(s.redo)-1]
	s.hasLast = false
	return change, true
}

// PushRedo 把一条已撤销的变更重新放回重做栈。
func (s *UndoStack) PushRedo(change Change) {
	s.redo = append(s.redo, change)
	s.hasLast = false
}

// Depth 返回可撤销的层数。
func (s *UndoStack) Depth() int {
	return len(s.undo)
}

// RedoDepth 返回可重做的层数。
func (s *UndoStack) RedoDepth() int {
	return len(s.redo)
}

// Reset 清空全部历史。
func (s *UndoStack) Reset() {
	s.undo = s.undo[:0]
	s.redo = s.redo[:0]
	s.hasLast = false
}
