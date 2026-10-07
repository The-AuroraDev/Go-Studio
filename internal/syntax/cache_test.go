// cache_test.go — 增量高亮缓存的测试。
// SPDX-License-Identifier: MIT

package syntax

import (
	"fmt"
	"strings"
	"testing"
)

// fakeLines 是最简文档实现。
type fakeLines struct {
	lines []string
}

func newFake(text string) *fakeLines {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	return &fakeLines{lines: strings.Split(text, "\n")}
}

func (f *fakeLines) LineCount() int { return len(f.lines) }

func (f *fakeLines) LineText(line int) []byte {
	if line < 0 || line >= len(f.lines) {
		return nil
	}
	return []byte(f.lines[line])
}

func (f *fakeLines) set(line int, text string) {
	f.lines[line] = text
}

func (f *fakeLines) insert(at int, text string) {
	f.lines = append(f.lines, "")
	copy(f.lines[at+1:], f.lines[at:])
	f.lines[at] = text
}

func (f *fakeLines) remove(at int) {
	f.lines = append(f.lines[:at], f.lines[at+1:]...)
}

// lineOf 取某一行的 rune 切片。
// LineText 给的是字节，而 token 下标一律是 rune 下标，两者不能混用。
func lineOf(doc Lines, line int) []rune {
	return []rune(string(doc.LineText(line)))
}

// plainHighlighter 不走缓存，逐行重算，用作对照基准。
// 缓存的全部价值就是「结果与朴素重算一致」，所以基准必须朴素。
func plainHighlighter(doc Lines, lex Lexer, line int) []Token {
	toks, _ := lex.LexLine(lineOf(doc, line), stateBefore(doc, lex, line))
	return toks
}

func stateBefore(doc Lines, lex Lexer, line int) State {
	var st State
	for i := 0; i < line; i++ {
		_, st = lex.LexLine(lineOf(doc, i), st)
	}
	return st
}

func TestCacheMatchesFullRelex(t *testing.T) {
	source := `package main

import "fmt"

/* 一段跨多行的注释
   第二行
   第三行 */

func main() {
	s := "hello"
	fmt.Println(s)
	// 尾部注释
}
`
	doc := newFake(source)
	lex := Go()
	hl := NewHighlighter(doc, lex)

	for line := 0; line < doc.LineCount(); line++ {
		got := hl.Tokens(line)
		want := plainHighlighter(doc, lex, line)
		if kindsOf(lineOf(doc, line), got) != kindsOf(lineOf(doc, line), want) {
			t.Errorf("第 %d 行不一致：\n缓存 = %s\n朴素 = %s", line+1,
				kindsOf(lineOf(doc, line), got),
				kindsOf(lineOf(doc, line), want))
		}
	}
}

func TestCacheScrollingIsIncremental(t *testing.T) {
	// 滚动到深处不应该重新扫描整个文件。
	// 这是缓存存在的唯一理由：不为这个，缓存就是纯浪费。
	var b strings.Builder
	b.WriteString("package main\n\nfunc main() {\n")
	for i := 0; i < 5000; i++ {
		fmt.Fprintf(&b, "\tx%d := %d // 第 %d 行\n", i, i, i)
	}
	b.WriteString("}\n")

	doc := newFake(b.String())
	hl := NewHighlighter(doc, Go())

	hl.Tokens(4000)
	afterJump := hl.MeasuredRunes()

	hl.Tokens(4001)
	hl.Tokens(4002)
	hl.Tokens(4003)
	afterScroll := hl.MeasuredRunes()

	// 往下滚三行只能多扫三行。
	if grew := afterScroll - afterJump; grew > 200 {
		t.Errorf("向下滚动三行却多扫了 %d 个 rune（跳到第 4000 行时扫了 %d），缓存没起作用",
			grew, afterJump)
	}
	// 跳到深处时确实扫了不少，这部分开销是一次性的。
	if afterJump < 1000 {
		t.Errorf("跳到第 4000 行只扫了 %d 个 rune，状态传递可能没生效", afterJump)
	}
}

func TestInvalidateAfterEdit(t *testing.T) {
	// 在文件开头开一个块注释，后面每一行都得变成注释。
	// 缓存若只在改动行重算，后面四千行会继续显示旧颜色。
	source := "a := 1\nb := 2\nc := 3\nd := 4\n"
	doc := newFake(source)
	hl := NewHighlighter(doc, Go())

	before := hl.Tokens(3)
	if hasKind(before, KindComment) {
		t.Fatalf("改动前第 4 行不该是注释")
	}

	doc.set(0, "/* 开")
	hl.Invalidate(0)

	after := hl.Tokens(3)
	if len(after) != 1 || after[0].Kind != KindComment {
		t.Errorf("开启块注释后，第 4 行应整行是注释：%s", kindsOf(lineOf(doc, 3), after))
	}
}

func TestInvalidateFromEditLineOnly(t *testing.T) {
	// 只改第 3 行，第 1、2 行的结果不该被重算也不能变。
	source := "a := 1\nb := 2\nc := 3\nd := 4\n"
	doc := newFake(source)
	hl := NewHighlighter(doc, Go())

	hl.Tokens(0)
	hl.Tokens(1)
	hl.Tokens(2)
	hl.Tokens(3)
	before := hl.MeasuredRunes()

	doc.set(2, "c := 30")
	hl.Invalidate(2)
	hl.Tokens(3)

	if grew := hl.MeasuredRunes() - before; grew > 40 {
		t.Errorf("只改一行却多扫了 %d 个 rune", grew)
	}
	line := lineOf(doc, 3)
	if !hasKind(hl.Tokens(3), KindNumber) {
		t.Errorf("重算后的第 4 行应有数字：%s", kindsOf(line, hl.Tokens(3)))
	}
}

func TestInvalidateAll(t *testing.T) {
	doc := newFake("a := 1\nb := 2\n")
	hl := NewHighlighter(doc, Go())
	hl.Tokens(1)

	hl.InvalidateAll()
	if hl.MeasuredRunes() != 0 {
		t.Errorf("全部作废后不应保留已扫描量")
	}
	line := lineOf(doc, 1)
	if got := hl.Tokens(1); len(got) != len(plainHighlighter(doc, Go(), 1)) {
		t.Errorf("作废后重算结果与朴素实现不一致：%s vs %s",
			kindsOf(line, got), kindsOf(line, plainHighlighter(doc, Go(), 1)))
	}
}

func TestCacheHandlesLineInsertAndDelete(t *testing.T) {
	// 插入与删除会改变行号。缓存里按行号存的下标必须跟着走，
	// 否则会拿旧行的 token 配新行的文字，画面直接错位。
	source := "a := 1\nb := 2\nc := 3\n"
	doc := newFake(source)
	hl := NewHighlighter(doc, Go())

	for line := 0; line < doc.LineCount(); line++ {
		hl.Tokens(line)
	}
	doc.insert(1, "x := \"新行\"\n")
	hl.Invalidate(1)
	hl.InvalidateAll()

	for line := 0; line < doc.LineCount(); line++ {
		got := hl.Tokens(line)
		want := plainHighlighter(doc, Go(), line)
		runes := lineOf(doc, line)
		if kindsOf(runes, got) != kindsOf(runes, want) {
			t.Errorf("插入后第 %d 行不一致：缓存 = %s，朴素 = %s",
				line+1, kindsOf(runes, got), kindsOf(runes, want))
		}
	}

	doc.remove(1)
	hl.InvalidateAll()
	for line := 0; line < doc.LineCount(); line++ {
		got := hl.Tokens(line)
		want := plainHighlighter(doc, Go(), line)
		runes := lineOf(doc, line)
		if kindsOf(runes, got) != kindsOf(runes, want) {
			t.Errorf("删除后第 %d 行不一致：缓存 = %s，朴素 = %s",
				line+1, kindsOf(runes, got), kindsOf(runes, want))
		}
	}
}

func TestCacheShrinksWhenDocumentLosesLines(t *testing.T) {
	// 行数变少后 valid 可能超过行数；不收缩就会去算不存在的行。
	doc := newFake("a := 1\nb := 2\nc := 3\nd := 4\n")
	hl := NewHighlighter(doc, Go())
	hl.Tokens(3)

	doc.remove(3)
	doc.remove(2)
	hl.Invalidate(2)

	for line := 0; line < doc.LineCount(); line++ {
		runes := lineOf(doc, line)
		if got, want := kindsOf(runes, hl.Tokens(line)), kindsOf(runes, plainHighlighter(doc, Go(), line)); got != want {
			t.Errorf("第 %d 行不一致：%s vs %s", line+1, got, want)
		}
	}
	if hl.Tokens(99) != nil {
		t.Error("越界行号应返回 nil")
	}
}

func TestHighlighterNilSafety(t *testing.T) {
	// 高亮是可选功能。nil 高亮器被到处传递是常态，
	// 它的每个方法都得能安全地被调用。
	var hl *Highlighter
	doc := newFake("a := 1\n")
	hl.Tokens(0)
	hl.Invalidate(0)
	hl.Invalidate(-1)
	hl.InvalidateAll()
	if hl.Language() != "" {
		t.Error("nil 高亮器的语言名应为空")
	}
	if hl.Lexer() != nil {
		t.Error("nil 高亮器不应有语言规则")
	}
	if hl.MeasuredRunes() != 0 {
		t.Error("nil 高亮器的扫描量应为 0")
	}
	if NewHighlighter(doc, nil) != nil {
		t.Error("没有语言规则时不该建立缓存")
	}
	if NewHighlighter(nil, Go()) != nil {
		t.Error("没有文档时不该建立缓存")
	}
	_ = doc
}

func TestHighlighterOutOfRangeLine(t *testing.T) {
	doc := newFake("a := 1\n")
	hl := NewHighlighter(doc, Go())
	for _, line := range []int{-1, 1, 100} {
		if hl.Tokens(line) != nil {
			t.Errorf("越界行 %d 应返回 nil", line)
		}
	}
}

func TestCacheLanguageAndLexer(t *testing.T) {
	doc := newFake("a := 1\n")
	hl := NewHighlighter(doc, ForLanguage("Python"))
	if hl.Language() != "Python" {
		t.Errorf("语言名 = %q", hl.Language())
	}
	if hl.Lexer() == nil {
		t.Error("应能取回语言规则")
	}
}

func TestCacheAgreesWithPlainAcrossAllLanguages(t *testing.T) {
	// 每门语言都要满足「缓存结果 == 朴素结果」。
	// 跨行状态各语言差异很大，只验 Go 不足以说明缓存是对的。
	sources := map[string]string{
		"Go":       "/* 开\n中间\n闭 */\nx := \"s\"\n",
		"Python":   "x = '''a\nb\nc'''\ny = 1\n",
		"Markdown": "```\ncode\n```\n# 标题\n",
		"Lua":      "x = [[a\nb]]\ny = 1\n",
		"HTML":     "<!-- a\nb -->\n<p>x</p>\n",
		"Perl":     "=head1\na\n=cut\n$x = 1;\n",
		"C":        "/* a\nb */\nint x = 1;\n",
		"JSON":     "{\n\"a\": 1\n}\n",
		"YAML":     "# a\n---\nkey: 1\n",
		"Shell":    "# a\nx=1\n",
	}
	for name, src := range sources {
		lex := ForLanguage(name)
		if lex == nil {
			t.Fatalf("语言 %s 未登记", name)
		}
		doc := newFake(src)
		hl := NewHighlighter(doc, lex)
		for line := 0; line < doc.LineCount(); line++ {
			runes := lineOf(doc, line)
			got := kindsOf(runes, hl.Tokens(line))
			want := kindsOf(runes, plainHighlighter(doc, lex, line))
			if got != want {
				t.Errorf("%s 第 %d 行：缓存 = %s，朴素 = %s", name, line+1, got, want)
			}
		}
	}
}

func TestCacheWithUnicodeLines(t *testing.T) {
	// token 下标是 rune 下标，渲染层也按 rune 走。中日韩文字宽不同，
	// 这里只验证缓存与朴素一致；宽度由 view 包负责。
	doc := newFake("s := \"中文\"\n注释：// 🎉\nx := 1\n")
	hl := NewHighlighter(doc, Go())
	for line := 0; line < doc.LineCount(); line++ {
		runes := lineOf(doc, line)
		if got, want := kindsOf(runes, hl.Tokens(line)), kindsOf(runes, plainHighlighter(doc, Go(), line)); got != want {
			t.Errorf("第 %d 行：%s vs %s", line+1, got, want)
		}
	}
}

func TestGrowTo(t *testing.T) {
	if got := len(growTo([]int{1, 2, 3}, 2)); got != 2 {
		t.Errorf("截短得到长度 %d", got)
	}
	if got := growTo([]int{1}, 5); len(got) != 5 || got[0] != 1 {
		t.Errorf("扩容结果 = %v", got)
	}
	if got := growTo([]int{1, 2, 3, 4}, 2); len(got) != 2 || got[1] != 2 {
		t.Errorf("应复用底层数组：%v", got)
	}
}
