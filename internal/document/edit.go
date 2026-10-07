// edit.go — 文本编辑：插入、删除、缩进、撤销重做。
// SPDX-License-Identifier: MIT

package document

import (
	"bytes"

	"github.com/29anan29/Go-Studio/internal/buffer"
)

// applyEdit 是唯一改动缓冲的入口：它负责登记撤销、同步版本号、作废重做。
//
// removed 是要删除的字节数，inserted 是插入的内容，语义与 buffer.Replace 一致。
func (d *Document) applyEdit(offset, removed int, inserted []byte) {
	before := d.rev
	// 必须在替换之前问出行号：插入换行后行号会整体平移。
	line, _ := d.buf.LineCol(offset)
	d.dirtyFrom = line
	removedText := d.buf.Replace(offset, removed, inserted)
	merged := d.undo.Push(buffer.Change{
		Offset:   offset,
		Removed:  removedText,
		Inserted: inserted,
	})
	// 新编辑会让撤销栈顶变化，重做栈则整体作废；
	// 版本区间记录必须同步，否则脏标记会算错。
	d.redoRevs = d.redoRevs[:0]
	d.recordEdit(before, merged)
}

// cursorAtOffset 返回字节偏移对应的光标位置，越界时截断。
func (d *Document) cursorAtOffset(offset int) Cursor {
	line, col := d.buf.LineCol(offset)
	return d.clamp(line, col)
}

// InsertRune 在光标处插入一个字符，光标随之右移。
// 换行符会走 Newline 以获得自动缩进。
func (d *Document) InsertRune(r rune) bool {
	if d.readonly {
		return false
	}
	if r == '\n' {
		return d.Newline()
	}
	offset := d.MustOffset()
	d.applyEdit(offset, 0, []byte(string(r)))
	d.horizontal(d.cursor.Line, d.cursor.Col+1)
	return true
}

// InsertText 在光标处插入一段文本（粘贴、输入法整词上屏），光标落在插入内容之后。
func (d *Document) InsertText(text string) bool {
	if d.readonly || text == "" {
		return false
	}
	offset := d.MustOffset()
	d.applyEdit(offset, 0, []byte(text))
	d.cursor = d.cursorAtOffset(offset + len(text))
	d.desiredCol = d.cursor.Col
	return true
}

// Newline 在光标处换行，并把当前行的行首空白复制到新行，实现基础自动缩进。
//
// 这里只复制空白，不做花括号分析：猜错了比不猜更烦人，
// 括号感知的缩进留给后续与语法结构配合的阶段。
func (d *Document) Newline() bool {
	if d.readonly {
		return false
	}
	indent := d.leadingIndent()
	inserted := make([]byte, 0, 1+len(indent))
	inserted = append(inserted, '\n')
	inserted = append(inserted, indent...)

	offset := d.MustOffset()
	d.applyEdit(offset, 0, inserted)
	d.horizontal(d.cursor.Line+1, len([]rune(string(indent))))
	return true
}

// leadingIndent 返回当前行行首的连续空白。
func (d *Document) leadingIndent() []byte {
	text := d.buf.LineText(d.cursor.Line)
	i := 0
	for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
		i++
	}
	return text[:i]
}

// InsertTab 插入缩进。tabWidth 大于 0 时补足到下一个制表位（软制表符），
// 否则插入一个真正的制表符。
func (d *Document) InsertTab(tabWidth int) bool {
	if d.readonly {
		return false
	}
	var inserted []byte
	if tabWidth <= 0 {
		inserted = []byte{'\t'}
	} else {
		n := tabWidth - d.cursor.Col%tabWidth
		inserted = bytes.Repeat([]byte{' '}, n)
	}
	offset := d.MustOffset()
	d.applyEdit(offset, 0, inserted)
	d.horizontal(d.cursor.Line, d.cursor.Col+len(inserted))
	return true
}

// Backspace 删除光标左侧的一个字符；在行首则把本行并入上一行。
func (d *Document) Backspace() bool {
	if d.readonly {
		return false
	}
	line, col := d.cursor.Line, d.cursor.Col

	if col > 0 {
		end := d.MustOffset()
		start, err := d.buf.OffsetOf(line, col-1)
		if err != nil {
			return false
		}
		d.applyEdit(start, end-start, nil)
		d.horizontal(line, col-1)
		return true
	}

	if line == 0 {
		return false
	}
	// 吃掉上一行末尾的换行符：从上一行行尾删到本行行首。
	prevLen := d.buf.LineRuneLen(line - 1)
	start, err := d.buf.OffsetOf(line-1, prevLen)
	if err != nil {
		return false
	}
	end := d.MustOffset()
	d.applyEdit(start, end-start, nil)
	d.horizontal(line-1, prevLen)
	return true
}

// DeleteRight 删除光标右侧的一个字符；在行尾则把下一行并入本行。
func (d *Document) DeleteRight() bool {
	if d.readonly {
		return false
	}
	line, col := d.cursor.Line, d.cursor.Col

	if col < d.buf.LineRuneLen(line) {
		start := d.MustOffset()
		end, err := d.buf.OffsetOf(line, col+1)
		if err != nil {
			return false
		}
		// 光标不动，期望列也不动：删除右侧字符不该让光标跳位。
		d.applyEdit(start, end-start, nil)
		return true
	}

	if line >= d.buf.LineCount()-1 {
		return false
	}
	start := d.MustOffset()
	end, err := d.buf.OffsetOf(line+1, 0)
	if err != nil {
		return false
	}
	d.applyEdit(start, end-start, nil)
	return true
}

// DeleteLine 删除整行，光标移到被删行的行首。
func (d *Document) DeleteLine() bool {
	if d.readonly {
		return false
	}
	line := d.cursor.Line
	start := d.buf.LineStart(line)
	end := d.buf.LineEnd(line)

	// 删的是没有换行符的最后一行时，要连前一行的换行一起删掉，
	// 否则会留下一个多余的空行。
	if end == d.buf.Len() && line > 0 {
		prevLen := d.buf.LineRuneLen(line - 1)
		prevEnd, err := d.buf.OffsetOf(line-1, prevLen)
		if err != nil {
			return false
		}
		start = prevEnd
	}
	if start == end {
		return false
	}
	d.applyEdit(start, end-start, nil)
	d.horizontal(line, 0)
	return true
}

// JoinLine 把下一行接到本行末尾，并吃掉下一行行首的缩进。
func (d *Document) JoinLine() bool {
	if d.readonly {
		return false
	}
	line := d.cursor.Line
	if line >= d.buf.LineCount()-1 {
		return false
	}
	lineLen := d.buf.LineRuneLen(line)
	start, err := d.buf.OffsetOf(line, lineLen)
	if err != nil {
		return false
	}
	indent := leadingWhitespaceRunes(d.buf.LineText(line + 1))
	end, err := d.buf.OffsetOf(line+1, indent)
	if err != nil {
		return false
	}
	if end <= start {
		return false
	}
	d.applyEdit(start, end-start, nil)
	d.horizontal(line, lineLen)
	return true
}

// Indent 在行首插入一个缩进单位，整行右移。
func (d *Document) Indent(tabWidth int) bool {
	if d.readonly {
		return false
	}
	if tabWidth <= 0 {
		tabWidth = 1
	}
	start := d.buf.LineStart(d.cursor.Line)
	d.applyEdit(start, 0, bytes.Repeat([]byte{' '}, tabWidth))
	d.horizontal(d.cursor.Line, d.cursor.Col+tabWidth)
	return true
}

// Unindent 删掉行首的一个缩进单位：优先去掉一个制表符，否则去掉至多 tabWidth 个空格。
func (d *Document) Unindent(tabWidth int) bool {
	if d.readonly {
		return false
	}
	if tabWidth <= 0 {
		tabWidth = 1
	}
	start := d.buf.LineStart(d.cursor.Line)
	text := d.buf.LineText(d.cursor.Line)

	remove := 0
	switch {
	case len(text) > 0 && text[0] == '\t':
		remove = 1
	default:
		for remove < len(text) && remove < tabWidth && text[remove] == ' ' {
			remove++
		}
	}
	if remove == 0 {
		return false
	}
	d.applyEdit(start, remove, nil)
	d.horizontal(d.cursor.Line, max(d.cursor.Col-remove, 0))
	return true
}

// Undo 撤销最近一次编辑，光标落到该次改动的位置。
func (d *Document) Undo() bool {
	change, ok := d.undo.Undo()
	if !ok {
		return false
	}
	d.buf.Undo(change)

	pair := d.undoRevs[len(d.undoRevs)-1]
	d.undoRevs = d.undoRevs[:len(d.undoRevs)-1]
	d.redoRevs = append(d.redoRevs, pair)
	d.rev = pair.before

	d.cursor = d.cursorAtOffset(change.Offset)
	d.desiredCol = d.cursor.Col
	return true
}

// Redo 重做最近一次被撤销的编辑，光标落到该次改动的位置。
func (d *Document) Redo() bool {
	change, ok := d.undo.Redo()
	if !ok {
		return false
	}
	d.buf.Redo(change)

	pair := d.redoRevs[len(d.redoRevs)-1]
	d.redoRevs = d.redoRevs[:len(d.redoRevs)-1]
	d.undoRevs = append(d.undoRevs, pair)
	d.rev = pair.after

	d.cursor = d.cursorAtOffset(change.Offset)
	d.desiredCol = d.cursor.Col
	return true
}

// CanUndo 报告是否还有可撤销的编辑。
func (d *Document) CanUndo() bool { return d.undo.Depth() > 0 }

// CanRedo 报告是否还有可重做的编辑。
func (d *Document) CanRedo() bool { return d.undo.RedoDepth() > 0 }

// leadingWhitespaceRunes 返回行首连续空白的 rune 数。
func leadingWhitespaceRunes(text []byte) int {
	count := 0
	for _, r := range string(text) {
		if r != ' ' && r != '\t' {
			break
		}
		count++
	}
	return count
}
