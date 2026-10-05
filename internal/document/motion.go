// motion.go — 光标移动：水平、垂直、按词、按行、按文档。
// SPDX-License-Identifier: MIT

package document

import "unicode"

// isWordRune 判断一个字符是否算「词」的一部分。
// 字母、数字、下划线算词，其余算分隔符——与大多数编辑器的取词一致。
func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

// lineRunes 返回第 i 行的 rune 切片，去掉行尾换行。
func (d *Document) lineRunes(line int) []rune {
	return []rune(string(d.buf.LineText(line)))
}

// horizontal 收尾一次水平移动：期望列要跟着更新，
// 否则用户按了几次左右键再按上下键，会跳回很久以前的列。
func (d *Document) horizontal(line, col int) {
	d.cursor = d.clamp(line, col)
	d.desiredCol = d.cursor.Col
}

// MoveLeft 光标左移一列；在行首则并入上一行行尾。
func (d *Document) MoveLeft() {
	if d.cursor.Col > 0 {
		d.horizontal(d.cursor.Line, d.cursor.Col-1)
		return
	}
	if d.cursor.Line > 0 {
		prev := d.cursor.Line - 1
		d.horizontal(prev, d.buf.LineRuneLen(prev))
	}
}

// MoveRight 光标右移一列；在行尾则并入下一行行首。
func (d *Document) MoveRight() {
	if d.cursor.Col < d.buf.LineRuneLen(d.cursor.Line) {
		d.horizontal(d.cursor.Line, d.cursor.Col+1)
		return
	}
	if d.cursor.Line < d.buf.LineCount()-1 {
		d.horizontal(d.cursor.Line+1, 0)
	}
}

// MoveUp 光标上移一行，尽量保持期望列。
func (d *Document) MoveUp() {
	if d.cursor.Line == 0 {
		return
	}
	line := d.cursor.Line - 1
	d.setCursorKeepDesired(line, min(d.desiredCol, d.buf.LineRuneLen(line)))
}

// MoveDown 光标下移一行，尽量保持期望列。
func (d *Document) MoveDown() {
	if d.cursor.Line >= d.buf.LineCount()-1 {
		return
	}
	line := d.cursor.Line + 1
	d.setCursorKeepDesired(line, min(d.desiredCol, d.buf.LineRuneLen(line)))
}

// PageUp 光标上移一屏，保持期望列。
func (d *Document) PageUp(height int) {
	if height < 1 {
		height = 1
	}
	line := max(d.cursor.Line-height, 0)
	d.setCursorKeepDesired(line, min(d.desiredCol, d.buf.LineRuneLen(line)))
}

// PageDown 光标下移一屏，保持期望列。
func (d *Document) PageDown(height int) {
	if height < 1 {
		height = 1
	}
	line := min(d.cursor.Line+height, d.buf.LineCount()-1)
	d.setCursorKeepDesired(line, min(d.desiredCol, d.buf.LineRuneLen(line)))
}

// MoveLineStart 光标移到行首。
func (d *Document) MoveLineStart() { d.horizontal(d.cursor.Line, 0) }

// MoveLineEnd 光标移到行尾。
func (d *Document) MoveLineEnd() {
	d.horizontal(d.cursor.Line, d.buf.LineRuneLen(d.cursor.Line))
}

// MoveDocStart 光标移到文档开头。
func (d *Document) MoveDocStart() { d.horizontal(0, 0) }

// MoveDocEnd 光标移到文档末尾。
func (d *Document) MoveDocEnd() {
	last := d.buf.LineCount() - 1
	d.horizontal(last, d.buf.LineRuneLen(last))
}

// GotoLine 跳到指定行并把光标放在行首。行号从 0 开始，越界会被截断。
func (d *Document) GotoLine(line int) { d.horizontal(line, 0) }

// MoveWordLeft 光标左移到上一个词的词首。
//
// 先退过光标左侧的分隔符，再退过整个词，落在词首。
// 在行首时先并到上一行行尾再找词，因此可以一路跨行往左。
func (d *Document) MoveWordLeft() {
	line, col := d.cursor.Line, d.cursor.Col
	if col == 0 {
		if line == 0 {
			return
		}
		line--
		col = d.buf.LineRuneLen(line)
	}
	runes := d.lineRunes(line)
	i := min(col, len(runes))
	for i > 0 && !isWordRune(runes[i-1]) {
		i--
	}
	for i > 0 && isWordRune(runes[i-1]) {
		i--
	}
	d.horizontal(line, i)
}

// MoveWordRight 光标右移到下一个词的词首；本行没有下一个词则落到下一行行首。
func (d *Document) MoveWordRight() {
	line, col := d.cursor.Line, d.cursor.Col
	runes := d.lineRunes(line)
	i := min(col, len(runes))

	// 先走出当前所在的词，再走过词之间的分隔符。
	for i < len(runes) && isWordRune(runes[i]) {
		i++
	}
	for i < len(runes) && !isWordRune(runes[i]) {
		i++
	}
	if i < len(runes) {
		d.horizontal(line, i)
		return
	}
	if line < d.buf.LineCount()-1 {
		d.horizontal(line+1, 0)
		return
	}
	d.horizontal(line, len(runes))
}
