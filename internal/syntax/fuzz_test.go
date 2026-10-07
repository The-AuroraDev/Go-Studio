// fuzz_test.go — 用随机输入持续压语法高亮器。
// SPDX-License-Identifier: MIT

package syntax

import (
	"testing"
)

// FuzzLexLine 用随机字节与 rune 压每一门语言的切分函数。
//
// 词法高亮的风险全在「语法不完整的中间状态」上：敲到一半的引号、
// 只开了头的注释、还没闭合的围栏。这些状态在真实编辑里每敲一键都会出现，
// 随机输入能穷举出我没想到的组合。
//
// 断言只有一条：不能 panic，且必须满足 token 契约。
// 至于「切得对不对」，随机输入无从判断——那由前面的定向测试负责。
func FuzzLexLine(f *testing.F) {
	seeds := []string{
		"", " ", "\t", "/*", "*/", "//", "\"", "'", "`", "'''", `"""`,
		"<a href=", "<!--", "-->", "[==[", "]]", "r#\"", "$", "${",
		"0x1F", "1e+", "'\\''", "=begin", "=cut", "```go", "@dec", "\x00",
		"func f() { s := \"x\" }", "print(len(x))", "#include <stdio.h>",
		"key: value", "[package]", "<a href=\"x\">", "中文 := 1",
	}
	for _, seed := range seeds {
		// 三行填同一个种子：这样每次迭代既覆盖单行也覆盖续行，
		// 而续行分支的 bug 历来比首行多。
		f.Add(seed, seed, seed)
	}
	f.Add("/*", "中间", "*/ x")
	f.Add("```", "code", "```")
	f.Add("x = [[a", "b]]", "y = 1")

	f.Fuzz(func(t *testing.T, a, b, c string) {
		lines := []string{a, b, c}

		// 语言不进参数：fuzzer 会连它一起变异，
		// 变异出来的名字当然查不到，等于白跑。
		// 放进循环反而更好——每次迭代把所有语言都压一遍。
		for _, lang := range Languages() {
			lex := ForLanguage(lang)
			assertLexInvariants(t, lang, lex, lines)
		}
	})
}

// assertLexInvariants 对一段行序列检查切分函数的不变量。
func assertLexInvariants(t *testing.T, lang string, lex Lexer, lines []string) {
	t.Helper()
	var state State
	for _, line := range lines {
		{
			runes := []rune(line)
			toks, next := lex.LexLine(runes, state)

			// 契约一：区间必须合法、不重叠、按 Start 递增。
			prevEnd := 0
			for i, tok := range toks {
				if tok.Start < 0 || tok.End > len(runes) || tok.End <= tok.Start {
					t.Fatalf("%s: token %d 非法 %+v，行 %q", lang, i, tok, line)
				}
				if tok.Start < prevEnd {
					t.Fatalf("%s: token %d 与前一个重叠 %+v，行 %q", lang, i, tok, line)
				}
				if int(tok.Kind) >= KindCount {
					t.Fatalf("%s: token %d 的 Kind 越界 %d", lang, i, tok.Kind)
				}
				prevEnd = tok.End
			}

			// 契约二：切片与 token 必须对得上，否则渲染层会取到越界字符。
			for _, tok := range toks {
				_ = runes[tok.Start:tok.End]
			}

			// 契约三：纯函数。同一行同样输入必须给同样结果，
			// 否则缓存会把上一个文件的高亮带到下一个文件。
			again, _ := lex.LexLine(runes, state)
			if kindsOf(runes, toks) != kindsOf(runes, again) {
				t.Fatalf("%s: 两次切分结果不一致 %q：%s vs %s",
					lang, line, kindsOf(runes, toks), kindsOf(runes, again))
			}

			state = next
		}
	}
}
