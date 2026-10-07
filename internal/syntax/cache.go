// cache.go — 按行增量高亮缓存。
// SPDX-License-Identifier: MIT

package syntax

// Highlighter 为一个文档维护增量高亮状态。
//
// 为什么需要缓存：每帧只画可见的几十行，但要拿到第 5000 行的 token，
// 必须先知道第 5000 行开始时的状态——那是靠前 4999 行一行行算出来的。
// 不缓存的话，滚一次动屏就要重扫五千行，光标一移动就掉帧。
//
// 缓存的结构就是两条平行的切片：
//
//	states[i]  第 i 行开始时的状态
//	tokens[i]  第 i 行的 token
//
// valid 表示 states[0..valid) 都已算好。编辑发生后从 dirty 行起作废，
// 于是「往下滚」几乎零成本，「在远处编辑」才会重算到视口。
type Highlighter struct {
	lexer  Lexer
	lines  Lines
	states []State
	tokens [][]Token
	valid  int
	// measured 记录已经量过的行长，供编辑器调参时估算成本。
	runes int
}

// Lines 是高亮所需的最小文档接口。
// 只依赖这两个方法，因此高亮不绑定具体的文档实现。
type Lines interface {
	LineCount() int
	LineText(line int) []byte
}

// NewHighlighter 为文档与语言规则建立缓存。
// lex 为 nil 表示这份文档不高亮。
func NewHighlighter(lines Lines, lex Lexer) *Highlighter {
	if lex == nil || lines == nil {
		return nil
	}
	h := &Highlighter{lexer: lex, lines: lines}
	// states[0] 是第 0 行开始时的状态，恒为零值。
	// valid 必须从 0 起：它表示「已算出的行数」，
	// 预置成 1 会让第 0 行永远算不出来。
	h.states = []State{0}
	return h
}

// Lexer 返回缓存使用的语言规则。
func (h *Highlighter) Lexer() Lexer {
	if h == nil {
		return nil
	}
	return h.lexer
}

// Language 返回语言名，未高亮时返回空串。
func (h *Highlighter) Language() string {
	if h == nil {
		return ""
	}
	return h.lexer.Name()
}

// Invalidate 作废 from 行及之后的全部缓存。
//
// from 行「开始时」的状态仍然是准的——这次编辑发生在该行之内，
// 还没有影响它开头。真正失效的是该行自己的 token 与它之后的每一行。
func (h *Highlighter) Invalidate(from int) {
	if h == nil || from < 0 {
		return
	}
	if from < h.valid {
		h.valid = from
	}
}

// InvalidateAll 丢掉全部缓存。
// 换文件、换语言、另存为改变语言时用。
func (h *Highlighter) InvalidateAll() {
	if h == nil {
		return
	}
	h.valid = 0
	h.runes = 0
	h.states = h.states[:1]
	h.states[0] = 0
	h.tokens = h.tokens[:0]
}

// Tokens 返回第 line 行的 token，行号越界或不高亮时返回 nil。
func (h *Highlighter) Tokens(line int) []Token {
	if h == nil || line < 0 || line >= h.lines.LineCount() {
		return nil
	}
	h.ensure(line)
	if line < len(h.tokens) {
		return h.tokens[line]
	}
	return nil
}

// ensure 把缓存推进到第 line 行已经算好的位置。
//
// 前向推进有一个不变量：算第 idx 行时 states[idx] 一定已经就位——
// 它是上一轮算第 idx-1 行时写下的收尾状态。所以这里不需要回头去找状态，
// 也不能回头找：一旦写成互相调用就会无限递归。
func (h *Highlighter) ensure(line int) {
	// 行数变少时（删行）先收缩，否则会按已经不存在的行继续算。
	if count := h.lines.LineCount(); h.valid > count {
		h.valid = count
	}
	for h.valid <= line {
		idx := h.valid
		// 先补齐切片再读：growTo 会保留已有内容，
		// 只把超出 n 的部分清零，而那些部分本来就已作废。
		h.states = growTo(h.states, idx+2)
		text := []rune(string(h.lines.LineText(idx)))
		toks, next := h.lexer.LexLine(text, h.states[idx])
		h.tokens = growTo(h.tokens, idx+1)
		h.tokens[idx] = toks
		h.states[idx+1] = next
		h.valid++
		h.runes += len(text)
	}
}

// growTo 把切片补零到至少 n 个元素。
func growTo[T any](s []T, n int) []T {
	if cap(s) >= n {
		return s[:n]
	}
	out := make([]T, n, n+n/2+8)
	copy(out, s)
	return out
}

// MeasuredRunes 返回已扫描过的 rune 总数，供编辑器统计高亮成本。
func (h *Highlighter) MeasuredRunes() int {
	if h == nil {
		return 0
	}
	return h.runes
}
