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
// 连续同类编辑会被合并：一段连续输入、或一段连续退格，都合成一条，
// 这样按一次撤销就能退掉刚敲的一整串，而不是一次只退一个字符。
//
// 返回值为真表示这次变更被并入了栈顶，没有新增历史条目。
// 调用方若在与撤销栈平行地记录元信息（编辑器的脏标记就依赖这一点），
// 必须据此更新栈顶那条而不是追加新的一条。
func (s *UndoStack) Push(change Change) bool {
	change.Removed = bytes.Clone(change.Removed)
	change.Inserted = bytes.Clone(change.Inserted)

	// 任何新变更都会作废重做栈，这是所有编辑器的通用行为。
	s.redo = s.redo[:0]

	if merged, ok := s.tryCoalesce(change); ok {
		s.undo[len(s.undo)-1] = merged
		s.lastChange = merged
		s.mergedAt = s.now()
		return true
	}

	s.undo = append(s.undo, change)
	s.lastChange = change
	s.hasLast = true
	s.mergedAt = s.now()
	return false
}

// tryCoalesce 判断新变更能否与栈顶合并，能则返回合并后的变更。
//
// 合并的三种前提必须同时成立：类型相同（都是插入或都是删除）、
// 位置紧密相邻、且距上次合并没超过窗口。三者缺一不可——
// 放宽位置条件会把两次不相干的编辑并成一条，撤销时删掉用户没想删的内容。
func (s *UndoStack) tryCoalesce(change Change) (Change, bool) {
	if !s.hasLast || s.coalesceWindow <= 0 || len(s.undo) == 0 {
		return Change{}, false
	}
	// 合并窗口从栈顶最后一次被扩展的时刻算起，这样持续输入会一直合并，
	// 停顿超过窗口后才另起一条。
	if s.now().Sub(s.mergedAt) > s.coalesceWindow {
		return Change{}, false
	}
	isInsert := func(c Change) bool { return len(c.Removed) == 0 && len(c.Inserted) > 0 }
	isDelete := func(c Change) bool { return len(c.Inserted) == 0 && len(c.Removed) > 0 }

	switch {
	case isInsert(s.lastChange) && isInsert(change):
		return mergeInserts(s.lastChange, change)
	case isDelete(s.lastChange) && isDelete(change):
		return mergeDeletes(s.lastChange, change)
	default:
		// 删除后接着输入、或替换后接着编辑，都不算同一次操作。
		return Change{}, false
	}
}

// mergeInserts 把两次相邻插入并成一条：新插入必须紧接在上一段之后。
func mergeInserts(last, next Change) (Change, bool) {
	if next.Offset != last.Offset+len(last.Inserted) {
		return Change{}, false
	}
	merged := Change{Offset: last.Offset}
	merged.Inserted = make([]byte, 0, len(last.Inserted)+len(next.Inserted))
	merged.Inserted = append(merged.Inserted, last.Inserted...)
	merged.Inserted = append(merged.Inserted, next.Inserted...)
	return merged, true
}

// mergeDeletes 把两次相邻删除并成一条，分退格与前向删除两种方向。
//
// 退格是从光标往左删：新删的区间紧贴在上一段的左边，合并后起点取新的。
// 前向删除是从光标往右删：新删的区间紧贴在上一段的右边，起点不变。
// Removed 始终按文档顺序拼接，撤销时要原样插回去。
func mergeDeletes(last, next Change) (Change, bool) {
	switch {
	case next.Offset+len(next.Removed) == last.Offset:
		merged := Change{Offset: next.Offset}
		merged.Removed = make([]byte, 0, len(next.Removed)+len(last.Removed))
		merged.Removed = append(merged.Removed, next.Removed...)
		merged.Removed = append(merged.Removed, last.Removed...)
		return merged, true
	case next.Offset == last.Offset:
		merged := Change{Offset: last.Offset}
		merged.Removed = make([]byte, 0, len(last.Removed)+len(next.Removed))
		merged.Removed = append(merged.Removed, last.Removed...)
		merged.Removed = append(merged.Removed, next.Removed...)
		return merged, true
	default:
		return Change{}, false
	}
}

// Undo 取出最近一条变更并把它移入重做栈，没有可撤销的则返回 false。
// 调用方拿到变更后应执行 Buffer.Undo 把它应用回缓冲。
func (s *UndoStack) Undo() (Change, bool) {
	if len(s.undo) == 0 {
		return Change{}, false
	}
	change := s.undo[len(s.undo)-1]
	s.undo = s.undo[:len(s.undo)-1]
	// 撤销过的变更必须留在重做栈里，否则重做一次之后再也没法撤销它。
	s.redo = append(s.redo, change)
	s.hasLast = false
	return change, true
}

// Redo 取出最近一条已撤销的变更并把它移回撤销栈，没有可重做的则返回 false。
func (s *UndoStack) Redo() (Change, bool) {
	if len(s.redo) == 0 {
		return Change{}, false
	}
	change := s.redo[len(s.redo)-1]
	s.redo = s.redo[:len(s.redo)-1]
	s.undo = append(s.undo, change)
	s.hasLast = false
	return change, true
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
