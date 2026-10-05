// buffer.go — 文本缓冲：piece table 存储 + 行索引。
// SPDX-License-Identifier: MIT

// Package buffer 提供编辑器使用的可编辑文本缓冲。
//
// 存储采用 piece table：初始文件内容放在不可变的 original 缓冲里，
// 之后所有插入的文本按顺序追加到 add 缓冲。文档内容由一组片段（piece）
// 拼接而成，每个片段指向两个缓冲中的一段区间。
//
// 相比把内容存成切片或按行存成字符串数组，piece table 让「在文件中间插入一行」
// 只改动片段数组的局部，不移动其余内容；行索引则把「第几行」定位到 O(log n)。
//
// 本包不依赖任何上层包，也不依赖 UI，可独立测试。
package buffer

import (
	"bytes"
	"fmt"
	"unicode/utf8"
)

// bufKind 标识片段指向哪个底层缓冲。
type bufKind uint8

const (
	// bufOriginal 指向初始文件内容，构造后不再修改。
	bufOriginal bufKind = iota
	// bufAdd 指向后续插入的文本，只在尾部追加。
	bufAdd
)

// piece 是文档中一段连续内容的引用。
type piece struct {
	kind   bufKind
	start  int // 在所属缓冲内的起始偏移
	length int // 长度
}

// Buffer 是可编辑的文本缓冲。零值不可用，必须用 New 构造。
type Buffer struct {
	original []byte
	add      []byte
	pieces   []piece
	total    int
	lines    lineIndex
}

// New 用初始内容构造缓冲。data 会被复制，调用方可以继续使用原切片。
func New(data []byte) *Buffer {
	b := &Buffer{
		original: bytes.Clone(data),
		pieces:   make([]piece, 0, 8),
		total:    len(data),
	}
	if len(data) > 0 {
		b.pieces = append(b.pieces, piece{kind: bufOriginal, start: 0, length: len(data)})
	}
	b.lines.init(b)
	return b
}

// collectNewlines 返回 [from, to) 区间内每个换行符之后的位置。
func (b *Buffer) collectNewlines(from, to int) []int {
	var found []int
	for pos := from; pos < to; {
		idx, off := b.locate(pos)
		if idx >= len(b.pieces) {
			break
		}
		p := b.pieces[idx]
		// off 是片段内偏移，源缓冲的索引还要加上片段自己的起点。
		base := p.start + off
		available := p.length - off
		stop := to - pos
		if available < stop {
			stop = available
		}
		chunk := b.source(p)[base : base+stop]
		for i, c := range chunk {
			if c == '\n' {
				found = append(found, pos+i+1)
			}
		}
		pos += stop
	}
	return found
}

// Len 返回文档的字节长度。
func (b *Buffer) Len() int {
	return b.total
}

// LineCount 返回行数。空文档也算一行。
func (b *Buffer) LineCount() int {
	return b.lines.count()
}

// LineStart 返回第 i 行第一个字节的偏移。i 从 0 开始。
func (b *Buffer) LineStart(i int) int {
	return b.lines.start(i)
}

// LineEnd 返回第 i 行最后一个字节之后、含行尾换行符的偏移。i 从 0 开始。
func (b *Buffer) LineEnd(i int) int {
	return b.lines.end(i, b.total)
}

// LineText 返回第 i 行的内容，不含行尾换行符。
// 行尾的 CRLF 视为一个换行符，因此结尾的 \r 也会被去掉。
func (b *Buffer) LineText(i int) []byte {
	text := b.slice(b.lines.start(i), b.lines.end(i, b.total))
	return bytes.TrimSuffix(bytes.TrimSuffix(text, []byte("\n")), []byte("\r"))
}

// LineRuneLen 返回第 i 行的字符数（按 rune 计，不含行尾换行符）。
func (b *Buffer) LineRuneLen(i int) int {
	return utf8.RuneCount(b.LineText(i))
}

// OffsetOf 返回第 line 行第 col 个字符的字节偏移。
// col 按 rune 计数；超出该行长度时会被截断到行尾。
// 行号越界返回错误。
func (b *Buffer) OffsetOf(line, col int) (int, error) {
	start, err := b.lineStartChecked(line)
	if err != nil {
		return 0, err
	}
	if col < 0 {
		return 0, fmt.Errorf("buffer: col %d is negative", col)
	}
	end := b.lines.end(line, b.total)
	return start + runeOffset(b.slice(start, end), col), nil
}

// LineCol 返回给定字节偏移所在的行号与列号。列号按 rune 计数。
// 偏移越界时截断到文档末尾。
func (b *Buffer) LineCol(offset int) (line, col int) {
	if offset < 0 {
		offset = 0
	}
	if offset > b.total {
		offset = b.total
	}
	line = b.lines.lineOf(offset)
	return line, b.runeCountInRange(b.lines.start(line), offset)
}

// runeCountInRange 统计 [from, to) 之间的 rune 数，不复制任何字节。
//
// 取行前缀再数 rune 会为每次调用分配一份拷贝，而行列转换在渲染路径上
// 每格都要用到，因此这里直接按片段走源缓冲做子切片。
func (b *Buffer) runeCountInRange(from, to int) int {
	if to <= from {
		return 0
	}

	count := 0
	pos := 0
	for i := range b.pieces {
		p := b.pieces[i]
		if p.length == 0 {
			continue
		}
		pieceStart := pos
		pos += p.length

		// 与目标区间求交。
		lo, hi := max(pieceStart, from), min(pos, to)
		if lo >= hi {
			continue
		}
		// lo、hi 是全局偏移，换算回源缓冲的字节下标。
		base := p.start + (lo - pieceStart)
		count += utf8.RuneCount(b.source(p)[base : base+(hi-lo)])
	}
	return count
}

// Replace 把 [offset, offset+removed) 替换为 inserted，返回被删除的内容。
// inserted 会被复制，调用方可以继续使用原切片。
//
// offset 或 removed 越界会 panic：这属于调用方的逻辑错误，必须立刻暴露，
// 静默修正会让后续所有渲染与定位都基于错误的前提。
func (b *Buffer) Replace(offset, removed int, inserted []byte) []byte {
	if offset < 0 || offset > b.total {
		panic(fmt.Sprintf("buffer: offset %d out of range [0, %d]", offset, b.total))
	}
	if removed < 0 || removed > b.total-offset {
		panic(fmt.Sprintf("buffer: removed %d out of range [0, %d]", removed, b.total-offset))
	}

	removedText := b.slice(offset, offset+removed)

	// 先把新文本追加到 add 缓冲，得到指向它的片段。
	var fresh piece
	if len(inserted) > 0 {
		fresh = piece{kind: bufAdd, start: len(b.add), length: len(inserted)}
		b.add = append(b.add, inserted...)
	}

	oldTotal := b.total

	startIdx := b.splitAt(offset)
	endIdx := b.splitAt(offset + removed)
	b.splicePieces(startIdx, endIdx, fresh)
	b.total = oldTotal - removed + len(inserted)

	// 行索引必须在片段拼接之后更新：它要扫描的是编辑后的新内容。
	b.lines.adjust(b, offset, removed, len(inserted), oldTotal)
	return removedText
}

// Undo 应用一条变更的逆操作。返回被删除的内容。
func (b *Buffer) Undo(change Change) []byte {
	return b.Replace(change.Offset, len(change.Inserted), change.Removed)
}

// Redo 重新应用一条变更。返回被删除的内容。
func (b *Buffer) Redo(change Change) []byte {
	return b.Replace(change.Offset, len(change.Removed), change.Inserted)
}

// Text 返回文档全部内容的副本。仅用于测试与小文件场景，
// 大文件请用 LineText 逐行读取。
func (b *Buffer) Text() []byte {
	return b.slice(0, b.total)
}

// String 返回文档内容的字符串形式，便于调试输出。
func (b *Buffer) String() string {
	return string(b.slice(0, b.total))
}

// slice 复制 [from, to) 区间的文档内容。
func (b *Buffer) slice(from, to int) []byte {
	if from < 0 || to < from || to > b.total {
		panic(fmt.Sprintf("buffer: slice [%d, %d) out of range [0, %d]", from, to, b.total))
	}
	out := make([]byte, 0, to-from)
	for pos := from; pos < to; {
		idx, off := b.locate(pos)
		if idx >= len(b.pieces) {
			break
		}
		p := b.pieces[idx]
		// off 是片段内偏移，源缓冲的索引还要加上片段自己的起点。
		base := p.start + off
		available := p.length - off
		want := to - pos
		if available < want {
			want = available
		}
		out = append(out, b.source(p)[base:base+want]...)
		pos += want
	}
	return out
}

// source 返回片段所指向的底层缓冲。
func (b *Buffer) source(p piece) []byte {
	if p.kind == bufOriginal {
		return b.original
	}
	return b.add
}

// locate 返回全局偏移 pos 所在的片段下标，以及该偏移在片段内的相对位置。
// pos 等于文档长度时返回片段数与 0。
func (b *Buffer) locate(pos int) (int, int) {
	remaining := pos
	for i, p := range b.pieces {
		if remaining < p.length {
			return i, remaining
		}
		remaining -= p.length
	}
	return len(b.pieces), 0
}

// growTo 保证切片至少有 need 个容量，并保留原有内容。
func growTo(s []piece, need int) []piece {
	if need <= cap(s) {
		return s
	}
	size := max(need, 2*cap(s), 8)
	grown := make([]piece, len(s), size)
	copy(grown, s)
	return grown
}

// splitAt 在全局偏移 at 处把片段切成两半，返回右半部分的起始下标。
// at 正好落在片段边界时不做切割，直接返回该下标。
func (b *Buffer) splitAt(at int) int {
	idx, off := b.locate(at)
	if off == 0 {
		return idx
	}
	p := b.pieces[idx]
	left := piece{kind: p.kind, start: p.start, length: off}
	right := piece{kind: p.kind, start: p.start + off, length: p.length - off}

	// 原地插入一个槽位。编辑频繁发生，每次都重新分配整段切片会让
	// 长会话的分配量随片段数平方增长。
	b.pieces = growTo(b.pieces, len(b.pieces)+1)
	b.pieces = b.pieces[:len(b.pieces)+1]
	copy(b.pieces[idx+2:], b.pieces[idx+1:])
	b.pieces[idx] = left
	b.pieces[idx+1] = right
	return idx + 1
}

// splicePieces 用 fresh 替换 [start, end) 的片段。fresh 长度为 0 时表示删除。
func (b *Buffer) splicePieces(start, end int, fresh piece) {
	removed := end - start
	added := 0
	if fresh.length > 0 {
		added = 1
	}

	oldLen := len(b.pieces)
	newLen := oldLen - removed + added

	// 搬移必须在截断之前完成：源区间一直延伸到旧长度末尾，
	// 先截断会把它裁掉。先把长度扩到两者较大值，保证读源写目标都在界内。
	span := max(oldLen, newLen)
	b.pieces = growTo(b.pieces, span)
	b.pieces = b.pieces[:span]

	// 尾部片段整体前移或后移，给新片段腾出位置。
	// copy 对重叠区间按 memmove 语义处理，两个方向都安全。
	copy(b.pieces[start+added:], b.pieces[start+removed:oldLen])

	b.pieces = b.pieces[:newLen]
	if added > 0 {
		b.pieces[start] = fresh
	}
}

// lineStartChecked 做带检查的行首查询。
func (b *Buffer) lineStartChecked(line int) (int, error) {
	if line < 0 || line >= b.lines.count() {
		return 0, fmt.Errorf("buffer: line %d out of range [0, %d)", line, b.lines.count())
	}
	return b.lines.start(line), nil
}

// runeOffset 返回 text 中第 col 个 rune 的字节偏移，col 超出长度时截断到末尾。
func runeOffset(text []byte, col int) int {
	if col <= 0 {
		return 0
	}
	count := 0
	for i := range string(text) {
		if count == col {
			return i
		}
		count++
	}
	return len(text)
}
