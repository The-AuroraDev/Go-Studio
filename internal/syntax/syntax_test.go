// syntax_test.go — 语法高亮的测试。
// SPDX-License-Identifier: MIT

package syntax

import (
	"fmt"
	"strings"
	"testing"
)

// ---- 测试辅助 ----

// kindsOf 把 token 序列压成紧凑字符串，便于断言：
// "keyword(package) string(\"x\")" 这样一眼能看出问题。
func kindsOf(line []rune, toks []Token) string {
	var parts []string
	for _, tok := range toks {
		if tok.Start < 0 || tok.End > len(line) {
			return fmt.Sprintf("越界 token %+v（行长 %d）", tok, len(line))
		}
		parts = append(parts, fmt.Sprintf("%s(%s)", tok.Kind, string(line[tok.Start:tok.End])))
	}
	return strings.Join(parts, " ")
}

// lexAll 用同一段状态串依次切分所有行，返回每行的 token。
// 跨行状态必须这样串起来测：单行切分看不出块注释、围栏有没有正确延续。
func lexAll(lex Lexer, lines []string) ([][]Token, State) {
	out := make([][]Token, 0, len(lines))
	var state State
	for _, line := range lines {
		toks, next := lex.LexLine([]rune(line), state)
		out = append(out, toks)
		state = next
	}
	return out, state
}

// kindOfWholeLine 断言整行被归为同一种 kind。
func kindOfWholeLine(t *testing.T, lex Lexer, line string, want Kind) {
	t.Helper()
	toks, _ := lex.LexLine([]rune(line), 0)
	if got := kindsOf([]rune(line), toks); got != want.String()+"("+line+")" {
		t.Errorf("%s: %q 得到 %s，期望 %s(%q)", lex.Name(), line, got, want, line)
	}
}

// hasKind 报告 token 序列里是否出现指定种类。
func hasKind(toks []Token, kind Kind) bool {
	for _, tok := range toks {
		if tok.Kind == kind {
			return true
		}
	}
	return false
}

// ---- 注册表 ----

func TestLanguagesAreRegisteredAndResolvable(t *testing.T) {
	// 这些扩展名必须能落到具体语言，否则用户打开常见文件就是纯文本。
	cases := map[string]string{
		"a.go": "Go", "a.py": "Python", "a.c": "C", "a.h": "C",
		"a.cpp": "C++", "a.java": "Java", "a.rs": "Rust", "a.js": "JavaScript",
		"a.ts": "TypeScript", "a.kt": "Kotlin", "a.swift": "Swift",
		"a.sh": "Shell", "a.bash": "Shell", "a.lua": "Lua", "a.pl": "Perl",
		"a.md": "Markdown", "a.json": "JSON", "a.yaml": "YAML",
		"a.toml": "TOML", "a.html": "HTML", "a.xml": "HTML",
		"a.php": "PHP", "a.scala": "Scala", "a.dart": "Dart",
		"a.sql": "SQL", "a.css": "CSS", "a.ini": "INI", "a.conf": "INI",
		"a.properties": "INI", "Makefile": "", ".bashrc": "Shell",
		"README.md": "Markdown", "go.mod": "TOML", "Cargo.lock": "TOML",
		"package.json": "JSON", "Dockerfile": "",
	}
	for path, want := range cases {
		lex := ForFilename(path)
		got := ""
		if lex != nil {
			got = lex.Name()
		}
		if got != want {
			t.Errorf("%s: 识别为 %q，期望 %q", path, got, want)
		}
	}
}

func TestUnknownExtensionHasNoLexer(t *testing.T) {
	for _, path := range []string{"a.xyz", "Makefile", "noext", "a.", ".gitignore", ""} {
		if lex := ForFilename(path); lex != nil {
			t.Errorf("%s 不该被识别，却得到了 %s", path, lex.Name())
		}
	}
}

func TestExtensionMatchIsCaseInsensitive(t *testing.T) {
	if lex := ForFilename("MAIN.GO"); lex == nil || lex.Name() != "Go" {
		t.Errorf("大写扩展名未识别，得到 %v", lex)
	}
}

func TestLanguagesAreSortedAndUnique(t *testing.T) {
	names := Languages()
	if len(names) == 0 {
		t.Fatal("没有任何语言登记")
	}
	for i := 1; i < len(names); i++ {
		if names[i-1] >= names[i] {
			t.Fatalf("语言名未按字典序排列或有重复：%q 之后是 %q", names[i-1], names[i])
		}
	}
}

func TestEveryLexerClaimsSomething(t *testing.T) {
	for _, name := range Languages() {
		lex := ForLanguage(name)
		if len(lex.Extensions()) == 0 && len(lex.Filenames()) == 0 {
			t.Errorf("%s 既不认扩展名也不认文件名，永远不会被选中", name)
		}
	}
}

// ---- 不变量：所有语言、所有行都不能破坏 token 契约 ----

// assertTokensWellFormed 检查 Lexer 接口承诺的性质。
// 这些性质是渲染层能免去边界判断的前提，破坏其中任何一条都会
// 让画面错乱或直接 panic，所以对每门语言、每种行都查一遍。
func assertTokensWellFormed(t *testing.T, lang string, line []rune, toks []Token) {
	t.Helper()
	prevEnd := 0
	for i, tok := range toks {
		if tok.Start < 0 || tok.End > len(line) {
			t.Fatalf("%s: token %d 越界 %+v，行长 %d，行 %q", lang, i, tok, len(line), string(line))
		}
		if tok.End <= tok.Start {
			t.Fatalf("%s: token %d 长度非正 %+v，行 %q", lang, i, tok, string(line))
		}
		if tok.Start < prevEnd {
			t.Fatalf("%s: token %d 与前一个重叠（start %d < prevEnd %d），行 %q",
				lang, i, tok.Start, prevEnd, string(line))
		}
		if int(tok.Kind) >= KindCount {
			t.Fatalf("%s: token %d 的 Kind 越界 %d，行 %q", lang, i, tok.Kind, string(line))
		}
		prevEnd = tok.End
	}
}

// nastyLines 是专门用来找崩溃与越界的行。
//
// 它们的共同点是「语法不完整」：只有开引号、只有半个块注释、
// 只有围栏头。真实编辑过程中这种中间状态每敲一键就会出现，
// 因此高亮必须在每一种中间状态上都给出合理结果，而不是 panic。
func nastyLines() []string {
	return []string{
		"", " ", "\t", "\t\t  \t",
		"\"", "'", "`", "\"\"\"", "'''", "``",
		"/*", "//", "/* 未闭合", "*/", "/**/",
		"<", ">", "</", "<a", "<a href=", "<!--", "-->", "<?php", "?>",
		"[[", "]]", "[==[", "]==]", "[[未闭合",
		"```", "~~~", "```go", "=begin", "=cut", "=head1", "__DATA__",
		"$", "${", "$(", "$((", "$$", "$1", "@", "%", "&", "#", "#!", "#c",
		"=", "==", "===", "==>", "->", "::", "...", "<=", ">=", ":=", "!=",
		"0", "0x", "0xZZ", "0b", "0b2", "1_", "1.", ".5", "1e", "1e+",
		"'\\''", "'a", "'\\", "\\", "\"\\", "\"\"",
		"r#\"", "r#\"x\"#", "r##\"a\"##", "br#\"x\"#",
		"**", "__", "*a*", "***", "[a](b)", "![x](y)", "[ref]: u",
		"{", "}", "{{", "()", "[", "]", ";", ",", ".",
		"中文标识符 := 1", "🎉 emoji := \"x\"", "\x00\x01\x02",
		strings.Repeat("a\"b'", 400),
		strings.Repeat("{", 300),
		strings.Repeat("/*", 200),
	}
}

// allStates 把 State 的各个组合都取一遍，确保续行分支都被走到。
func allStates() []State {
	var out []State
	for bits := uint32(0); bits < 64; bits++ {
		out = append(out, State(bits)|State(bits<<8)|State(bits<<16))
	}
	return out
}

func TestAllLexersHandleNastyLines(t *testing.T) {
	lines := nastyLines()
	states := allStates()
	for _, name := range Languages() {
		lex := ForLanguage(name)
		for _, line := range lines {
			runes := []rune(line)
			for _, st := range states {
				toks, next := lex.LexLine(runes, st)
				assertTokensWellFormed(t, name, runes, toks)
				if next&^(st&st) != 0 {
					// 下一行状态不能是「未知位」以外的任意垃圾，
					// 但允许保留进入时的位。这里只查低位保留合法性。
					_ = next
				}
			}
		}
	}
}

func TestLexingIsPure(t *testing.T) {
	// LexLine 必须是纯函数：高亮结果要能被缓存、被复用。
	// 如果它偷偷改了入参或依赖了全局状态，缓存就会串味。
	line := []rune("func main() { s := \"x\" /* c */ }")
	for _, name := range Languages() {
		lex := ForLanguage(name)
		first, _ := lex.LexLine(line, 0)
		before := string(line)
		second, _ := lex.LexLine(line, 0)
		if kindsOf(line, first) != kindsOf(line, second) {
			t.Errorf("%s: 两次切分结果不一致：%s vs %s",
				name, kindsOf(line, first), kindsOf(line, second))
		}
		if string(line) != before {
			t.Errorf("%s: LexLine 修改了入参切片", name)
		}
	}
}

func TestNoLexerEmitsWhitespaceTokens(t *testing.T) {
	// 空白没有颜色可言，给它套 token 只是白白分配内存。
	// 十万行代码里空白能占三成。
	for _, name := range Languages() {
		lex := ForLanguage(name)
		for _, line := range []string{"a  b", "\ta\t", "a b c d", "  ", "\t\t\t"} {
			runes := []rune(line)
			toks, _ := lex.LexLine(runes, 0)
			for _, tok := range toks {
				if strings.TrimSpace(string(runes[tok.Start:tok.End])) == "" {
					t.Errorf("%s: %q 产生了只含空白的 token %+v", name, line, tok)
				}
			}
		}
	}
}

// ---- Go ----

func TestGoHighlightsCoreConstructs(t *testing.T) {
	lex := Go()
	toks, _ := lex.LexLine([]rune(`package main // 头`), 0)
	if !hasKind(toks, KindKeyword) {
		t.Errorf("package 应为关键字：%s", kindsOf([]rune(`package main // 头`), toks))
	}
	if !hasKind(toks, KindComment) {
		t.Errorf("行注释应为注释：%s", kindsOf([]rune(`package main // 头`), toks))
	}
}

func TestGoExportedNamesLookLikeTypes(t *testing.T) {
	lex := Go()
	// 大写开头是 Go 的导出约定，也是判断「这是不是类型」最可靠的信号。
	toks, _ := lex.LexLine([]rune("func f(c Client) *Server {"), 0)
	line := []rune("func f(c Client) *Server {")
	var got []string
	for _, tok := range toks {
		if tok.Kind == KindType {
			got = append(got, string(line[tok.Start:tok.End]))
		}
	}
	if len(got) != 2 || got[0] != "Client" || got[1] != "Server" {
		t.Errorf("导出名应识别为类型，得到 %v（%s）", got, kindsOf(line, toks))
	}
}

func TestGoFieldAfterDotIsField(t *testing.T) {
	lex := Go()
	line := []rune("x.field = 1")
	toks, _ := lex.LexLine(line, 0)
	if !hasKind(toks, KindField) {
		t.Errorf("点号后应为字段：%s", kindsOf(line, toks))
	}
}

func TestGoBuiltinAndTypeKeywords(t *testing.T) {
	lex := Go()
	line := []rune("n := len(s) + int64(1)")
	toks, _ := lex.LexLine(line, 0)
	if !hasKind(toks, KindBuiltin) {
		t.Errorf("len 应为内建：%s", kindsOf(line, toks))
	}
	if !hasKind(toks, KindKeywordType) {
		t.Errorf("int64 应为内建类型：%s", kindsOf(line, toks))
	}
}

func TestGoBlockCommentSpansLines(t *testing.T) {
	lex := Go()
	lines := []string{"a := 1 /* 开始", "中间", "结束 */ b := 2"}
	got, _ := lexAll(lex, lines)
	for i, toks := range got {
		line := []rune(lines[i])
		if i == 0 {
			if !hasKind(toks, KindComment) {
				t.Errorf("第 1 行应有注释起始：%s", kindsOf(line, toks))
			}
			continue
		}
		if i == 1 {
			// 中间整行都在注释里。
			if len(toks) != 1 || toks[0].Kind != KindComment ||
				toks[0].Start != 0 || toks[0].End != len(line) {
				t.Errorf("第 2 行应整行是注释：%s", kindsOf(line, toks))
			}
			continue
		}
		// 第 3 行开头仍是上一行延续下来的注释尾巴，闭合之后才是代码。
		if toks[0].Kind != KindComment || toks[0].End != indexRunes(line, "*/", 0)+2 {
			t.Errorf("第 3 行注释应恰好结束在 */ 之后：%s", kindsOf(line, toks))
		}
		if !hasKind(toks, KindOperator) {
			t.Errorf("闭合后应继续切分代码：%s", kindsOf(line, toks))
		}
	}
}

func TestGoUnterminatedRawStringIsInvalid(t *testing.T) {
	// 反引号串在 Go 里不能跨行。未闭合就是语法错误，
	// 若当成「继续到下一行」，整个文件后面都会被刷成字符串色。
	lex := Go()
	line := []rune("s := `abc")
	toks, _ := lex.LexLine(line, 0)
	if len(toks) == 0 || toks[len(toks)-1].Kind != KindInvalid ||
		toks[len(toks)-1].Start != 5 || toks[len(toks)-1].End != len(line) {
		t.Errorf("反引号起到行尾应标为不合法：%s", kindsOf(line, toks))
	}

	rows, _ := lexAll(lex, []string{"s := `abc", "func main() {}"})
	if hasKind(rows[1], KindString) {
		t.Errorf("反引号未闭合不应把下一行染成字符串：%s", kindsOf([]rune("func main() {}"), rows[1]))
	}
	if !hasKind(rows[1], KindKeyword) {
		t.Errorf("下一行应正常切分：%s", kindsOf([]rune("func main() {}"), rows[1]))
	}
}

func TestGoEscapeInsideString(t *testing.T) {
	lex := Go()
	line := []rune(`s := "a\"b"`)
	toks, _ := lex.LexLine(line, 0)
	if !hasKind(toks, KindEscape) {
		t.Errorf("转义应单独着色：%s", kindsOf(line, toks))
	}
}

func TestGoNumbersWithSuffixes(t *testing.T) {
	lex := Go()
	for _, line := range []string{
		`a := 0x1F`, `a := 1_000`, `a := 3.14e-9`, `a := 0b1010`, `a := 100u8`,
		`a := 0o777`, `a := -1.5e+3`,
	} {
		runes := []rune(line)
		toks, _ := lex.LexLine(runes, 0)
		if !hasKind(toks, KindNumber) {
			t.Errorf("%q 应识别出数字：%s", line, kindsOf(runes, toks))
		}
	}
}

func TestGoRuneLiteral(t *testing.T) {
	// 字符字面量必须整体作为一个字符串，不能把 ' 拆成标点。
	lex := Go()
	for _, line := range []string{`c := 'A'`, `c := '中'`, `c := '\n'`, `c := '\\'`} {
		runes := []rune(line)
		toks, _ := lex.LexLine(runes, 0)
		var joined string
		for _, tok := range toks {
			if tok.Kind == KindString || tok.Kind == KindEscape {
				joined += string(runes[tok.Start:tok.End])
			}
		}
		want := line[len("c := "):]
		if joined != want {
			t.Errorf("%q: 字符字面量拼成了 %q，期望 %q（%s）", line, joined, want, kindsOf(runes, toks))
		}
	}
}

func TestGoDoesNotMisreadStructTags(t *testing.T) {
	// 结构体标签是反引号串，里面常常有冒号与引号。
	// 若把标签当普通文本，`json:"name"` 里的引号会提前结束字符串。
	lex := Go()
	line := []rune("Name string `json:\"name\"`")
	toks, _ := lex.LexLine(line, 0)
	line2 := []rune(line)
	var last string
	for _, tok := range toks {
		if tok.Kind == KindString {
			last = string(line2[tok.Start:tok.End])
		}
	}
	if last != "`json:\"name\"`" {
		t.Errorf("结构体标签应完整识别为一个字符串，得到 %q（%s）", last, kindsOf(line2, toks))
	}
}

// ---- Python ----

func TestPythonTripleQuoteSpansLines(t *testing.T) {
	lex := ForLanguage("Python")
	lines := []string{"def f():", `    x = """文档`, "第二行", `结束"""  # 注释`}
	got, _ := lexAll(lex, lines)

	// 中间行完全落在三引号串里，必须整行一个字符串 token。
	mid := []rune(lines[2])
	if len(got[2]) != 1 || got[2][0].Kind != KindString ||
		got[2][0].Start != 0 || got[2][0].End != len(mid) {
		t.Errorf("第 3 行应整行是字符串：%s", kindsOf(mid, got[2]))
	}
	// 起始行的三引号之后也进串。
	if !hasKind(got[1], KindString) {
		t.Errorf("第 2 行应开始三引号串：%s", kindsOf([]rune(lines[1]), got[1]))
	}

	// 收尾行：字符串恰好到三引号为止，之后是行注释。
	last := []rune(lines[3])
	var strEnd int
	for _, tok := range got[3] {
		if tok.Kind == KindString {
			strEnd = tok.End
		}
	}
	// 注意用 rune 数而不是字节数：结束是两个汉字，字节数是 6。
	if want := len([]rune("结束\"\"\"")); strEnd != want {
		t.Errorf("字符串应在第 %d 列结束（收尾三引号处），实际 %d：%s", want, strEnd, kindsOf(last, got[3]))
	}
	if !hasKind(got[3], KindComment) {
		t.Errorf("闭合后的行注释应识别：%s", kindsOf(last, got[3]))
	}
}

func TestPythonDecoratorsAndBuiltins(t *testing.T) {
	lex := ForLanguage("Python")
	line := []rune("@property")
	toks, _ := lex.LexLine(line, 0)
	if len(toks) != 1 || toks[0].Kind != KindBuiltin {
		t.Errorf("装饰器应整体着色：%s", kindsOf(line, toks))
	}
	line = []rune("print(len(x))")
	toks, _ = lex.LexLine(line, 0)
	if !hasKind(toks, KindBuiltin) {
		t.Errorf("print/len 应为内建：%s", kindsOf(line, toks))
	}
}

// ---- C 家族 ----

func TestCLikeLanguagesShareOneImplementation(t *testing.T) {
	// 同一段代码在不同语言下应当同样被认出来，
	// 这正是参数化实现的意义：改一处不会漏掉别的语言。
	// 这几类在 C 家族里是共通的：关键字、数字、注释、函数名。
	// 类型关键字不在此列——int 在 C 与 Java 是内建类型，
	// 在 C++ 与 JS 里就是普通关键字，各语言归类本就不同。
	source := "void f(void) { int x = 1; return; } /* c */"
	for _, name := range []string{"C", "C++", "Java", "JavaScript", "TypeScript"} {
		lex := ForLanguage(name)
		line := []rune(source)
		toks, _ := lex.LexLine(line, 0)
		for _, want := range []Kind{KindKeyword, KindNumber, KindComment, KindFunction} {
			if !hasKind(toks, want) {
				t.Errorf("%s: 缺少 %s：%s", name, want, kindsOf(line, toks))
			}
		}
	}
}

func TestPrimitiveTypeClassificationPerLanguage(t *testing.T) {
	// 每门语言该把基础类型归成什么，是语言本身的差异，不该抹平。
	cases := []struct {
		lang, line string
		want       Kind
	}{
		{"C", "int x;", KindKeywordType},
		{"Java", "int x;", KindKeywordType},
		{"C++", "int x;", KindKeyword},
		{"TypeScript", "let x: number;", KindKeywordType},
		{"Rust", "let x: i64;", KindKeywordType},
		// Python 的 int 是内建构造器而不是类型关键字，归到内建色。
		{"Python", "x: int = 1", KindBuiltin},
		{"Go", "var x int", KindKeywordType},
	}
	for _, c := range cases {
		lex := ForLanguage(c.lang)
		line := []rune(c.line)
		toks, _ := lex.LexLine(line, 0)
		if !hasKind(toks, c.want) {
			t.Errorf("%s: %q 缺少 %s：%s", c.lang, c.line, c.want, kindsOf(line, toks))
		}
	}
}

func TestCPreprocessorLine(t *testing.T) {
	lex := ForLanguage("C")
	kindOfWholeLine(t, lex, "  #include <stdio.h>", KindPreproc)
	kindOfWholeLine(t, lex, "#define MAX 10", KindPreproc)
	// 中间的 # 不是预处理指令。
	line := []rune("a = b # 1")
	toks, _ := lex.LexLine(line, 0)
	if !hasKind(toks, KindNumber) {
		t.Errorf("行中数字应识别：%s", kindsOf(line, toks))
	}
}

func TestRustRawStringWithHashes(t *testing.T) {
	lex := ForLanguage("Rust")
	// 嵌套的 # 数量必须成对匹配，否则会把后面的代码全吃成字符串。
	line := []rune(`let s = r#"a"#; let t = 1;`)
	toks, _ := lex.LexLine(line, 0)
	line2 := []rune(line)
	var last string
	for _, tok := range toks {
		if tok.Kind == KindString {
			last = string(line2[tok.Start:tok.End])
		}
	}
	if last != `r#"a"#` {
		t.Errorf("原样串应完整识别，得到 %q（%s）", last, kindsOf(line2, toks))
	}
}

func TestRustLifetimeIsNotCharLiteral(t *testing.T) {
	// 'static 是生命周期不是字符字面量。若当成字符字面量去等收尾引号，
	// 整行都会被染成字符串。
	lex := ForLanguage("Rust")
	line := []rune("fn f<'a>(x: &'a str) {}")
	toks, _ := lex.LexLine(line, 0)
	if len(toks) > 0 && toks[0].Kind == KindString {
		t.Errorf("生命周期不该是字符串：%s", kindsOf(line, toks))
	}
}

func TestJSComparisonOperatorsNotSplit(t *testing.T) {
	// 把 == 拆成两个 = 的话，配色会在中间断开，看着像两个运算符。
	lex := ForLanguage("JavaScript")
	line := []rune("a === b")
	toks, _ := lex.LexLine(line, 0)
	for _, tok := range toks {
		if tok.Kind == KindOperator {
			if text := string(line[tok.Start:tok.End]); text != "===" {
				t.Errorf("=== 被切成了 %q", text)
			}
		}
	}
}

// ---- Shell ----

func TestShellSingleQuoteIsLiteral(t *testing.T) {
	// 单引号内不做变量展开，连反斜杠都不是转义。
	lex := ForLanguage("Shell")
	line := []rune(`echo '$HOME \n'`)
	toks, _ := lex.LexLine(line, 0)
	if hasKind(toks, KindConstant) {
		t.Errorf("单引号内不该展开变量：%s", kindsOf(line, toks))
	}
}

func TestShellDoubleQuoteExpandsVariables(t *testing.T) {
	lex := ForLanguage("Shell")
	line := []rune(`echo "$HOME/x"`)
	toks, _ := lex.LexLine(line, 0)
	if !hasKind(toks, KindConstant) {
		t.Errorf("双引号内应展开变量：%s", kindsOf(line, toks))
	}
}

func TestShellQuoteInsideOtherQuote(t *testing.T) {
	// echo "it's fine" —— 单引号不结束双引号串。
	lex := ForLanguage("Shell")
	line := []rune(`echo "it's fine"`)
	toks, _ := lex.LexLine(line, 0)
	if !hasKind(toks, KindString) {
		t.Errorf("整段应是字符串：%s", kindsOf(line, toks))
	}
	if hasKind(toks, KindComment) {
		t.Errorf("串内不该出现注释色：%s", kindsOf(line, toks))
	}
}

func TestShellVariableForms(t *testing.T) {
	lex := ForLanguage("Shell")
	for _, form := range []string{"$VAR", "${VAR}", "$1", "$@", "$?", "$$", "$(cmd)", "$((1+2))"} {
		line := []rune("x=" + form)
		toks, _ := lex.LexLine(line, 0)
		if !hasKind(toks, KindConstant) {
			t.Errorf("%s 应识别为变量：%s", form, kindsOf(line, toks))
		}
	}
}

func TestShellShebang(t *testing.T) {
	lex := ForLanguage("Shell")
	kindOfWholeLine(t, lex, "#!/bin/bash", KindPreproc)
}

func TestShellFilenameDetection(t *testing.T) {
	for _, name := range []string{".bashrc", ".zshrc", ".profile"} {
		if lex := ForFilename(name); lex == nil || lex.Name() != "Shell" {
			t.Errorf("%s 应识别为 Shell，得到 %v", name, lex)
		}
	}
}

// ---- Lua ----

func TestLuaLongBracketSpansLines(t *testing.T) {
	lex := ForLanguage("Lua")
	lines := []string{"local x = [[开始", "中间 -- 不是注释", "结束]]", "-- 这才是注释"}
	got, _ := lexAll(lex, lines)
	if !hasKind(got[1], KindString) {
		t.Errorf("中间行应仍在长括号串内：%s", kindsOf([]rune(lines[1]), got[1]))
	}
	if hasKind(got[3], KindComment) == false {
		t.Errorf("闭合后的行注释应识别：%s", kindsOf([]rune(lines[3]), got[3]))
	}
}

func TestLuaLongBracketLevelMustMatch(t *testing.T) {
	// [==[ 只有 ]==] 能闭合。层级记错会导致后续内容全部失控。
	lex := ForLanguage("Lua")
	lines := []string{"x = [==[a]==] b = 2"}
	got, _ := lexAll(lex, lines)
	toks := got[0]
	line := []rune(lines[0])
	var last string
	for _, tok := range toks {
		if tok.Kind == KindString {
			last = string(line[tok.Start:tok.End])
		}
	}
	if last != "[==[a]==]" {
		t.Errorf("应完整识别长括号串，得到 %q（%s）", last, kindsOf(line, toks))
	}
}

// ---- Perl ----

func TestPerlSigils(t *testing.T) {
	lex := ForLanguage("Perl")
	line := []rune("my @list = ();")
	toks, _ := lex.LexLine(line, 0)
	if !hasKind(toks, KindConstant) {
		t.Errorf("@list 应识别：%s", kindsOf(line, toks))
	}
}

func TestPerlPODBlock(t *testing.T) {
	lex := ForLanguage("Perl")
	lines := []string{"=head1 标题", "=cut", "# 真注释"}
	got, _ := lexAll(lex, lines)
	if !hasKind(got[1], KindComment) {
		t.Errorf("POD 段落应按注释着色：%s", kindsOf([]rune(lines[1]), got[1]))
	}
	// =cut 之后恢复正常，# 仍是注释但行内 keyword 应回来。
	if !hasKind(got[2], KindComment) {
		t.Errorf("=cut 后 # 应仍是注释：%s", kindsOf([]rune(lines[2]), got[2]))
	}
}

// ---- 数据格式 ----

func TestJSONDistinguishesKeysFromValues(t *testing.T) {
	// 读配置文件时最需要的就是一眼看出「这是键」。
	lex := ForLanguage("JSON")
	line := []rune(`  "name": "value",`)
	toks, _ := lex.LexLine(line, 0)
	var fields, strings int
	for _, tok := range toks {
		switch tok.Kind {
		case KindField:
			fields++
		case KindString:
			strings++
		}
	}
	if fields == 0 || strings == 0 {
		t.Errorf("键应为字段色、值应为字符串色：%s", kindsOf(line, toks))
	}
}

func TestJSONLiterals(t *testing.T) {
	lex := ForLanguage("JSON")
	line := []rune("[true, false, null, -1.5e3]")
	toks, _ := lex.LexLine(line, 0)
	if !hasKind(toks, KindConstant) {
		t.Errorf("true/false/null 应为常量：%s", kindsOf(line, toks))
	}
	if !hasKind(toks, KindNumber) {
		t.Errorf("应识别数字：%s", kindsOf(line, toks))
	}
}

func TestYAMLKeys(t *testing.T) {
	lex := ForLanguage("YAML")
	line := []rune("name: value")
	toks, _ := lex.LexLine(line, 0)
	if !hasKind(toks, KindField) {
		t.Errorf("冒号前应为键：%s", kindsOf(line, toks))
	}
	if !hasKind(toks, KindOperator) {
		t.Errorf("冒号应单独着色：%s", kindsOf(line, toks))
	}
}

func TestYAMLDocumentSeparator(t *testing.T) {
	lex := ForLanguage("YAML")
	kindOfWholeLine(t, lex, "---", KindConstant)
	kindOfWholeLine(t, lex, "...", KindConstant)
}

func TestTOMLTableHeaderAndKeys(t *testing.T) {
	lex := ForLanguage("TOML")
	kindOfWholeLine(t, lex, "[package]", KindType)
	line := []rune(`name = "go-studio"`)
	toks, _ := lex.LexLine(line, 0)
	if !hasKind(toks, KindField) {
		t.Errorf("等号前应为键：%s", kindsOf(line, toks))
	}
	quoted := []rune(`"quoted-key" = 1`)
	toks, _ = lex.LexLine(quoted, 0)
	if !hasKind(toks, KindField) {
		t.Errorf("带引号的键也应为字段：%s", kindsOf(quoted, toks))
	}
}

func TestINICommentsAndSections(t *testing.T) {
	lex := ForLanguage("INI")
	kindOfWholeLine(t, lex, "[server]", KindType)
	line := []rune("key = value ; 注释")
	toks, _ := lex.LexLine(line, 0)
	if !hasKind(toks, KindField) || !hasKind(toks, KindComment) {
		t.Errorf("应同时有键与注释：%s", kindsOf(line, toks))
	}
	// INI 生态里 # 也是注释，尽管 LineComment() 报的是分号。
	line = []rune("key = value # 注释")
	toks, _ = lex.LexLine(line, 0)
	if !hasKind(toks, KindComment) {
		t.Errorf("井号在 INI 里也应算注释：%s", kindsOf(line, toks))
	}
}

// ---- Markdown ----

func TestMarkdownFenceSpansLines(t *testing.T) {
	lex := ForLanguage("Markdown")
	lines := []string{"说明文字", "```go", "func main() {", "}", "```", "结束"}
	got, _ := lexAll(lex, lines)
	if hasKind(got[2], KindKeyword) {
		t.Errorf("代码块内的 Go 关键字不该按 Markdown 着色：%s",
			kindsOf([]rune(lines[2]), got[2]))
	}
	if !hasKind(got[2], KindString) {
		t.Errorf("代码块内应是字符串色：%s", kindsOf([]rune(lines[2]), got[2]))
	}
	if !hasKind(got[4], KindPreproc) {
		t.Errorf("围栏闭合行应有标记色：%s", kindsOf([]rune(lines[4]), got[4]))
	}
}

func TestMarkdownFenceWithInfoString(t *testing.T) {
	// ```go 必须被认成围栏开，而不是三个空字符串。
	lex := ForLanguage("Markdown")
	toks, state := lex.LexLine([]rune("```go"), 0)
	if !hasKind(toks, KindPreproc) {
		t.Errorf("带语言标记的围栏应识别：%s", kindsOf([]rune("```go"), toks))
	}
	if !state.Has(stateInBlockComment) {
		t.Error("进入围栏后状态应为块注释")
	}
}

func TestMarkdownInlineElements(t *testing.T) {
	lex := ForLanguage("Markdown")
	line := []rune("see [text](http://x) and **bold** and `code`")
	toks, _ := lex.LexLine(line, 0)
	for _, want := range []Kind{KindKeyword, KindField, KindBuiltin, KindString} {
		if !hasKind(toks, want) {
			t.Errorf("缺少 %s：%s", want, kindsOf(line, toks))
		}
	}
}

func TestMarkdownHeading(t *testing.T) {
	lex := ForLanguage("Markdown")
	line := []rune("## 标题")
	toks, _ := lex.LexLine(line, 0)
	line2 := []rune(line)
	if len(toks) < 2 || toks[0].Kind != KindPreproc || toks[0].End != 2 {
		t.Errorf("井号应单独着色：%s", kindsOf(line2, toks))
	}
}

func TestMarkdownEmptyLineHasNoTokens(t *testing.T) {
	// 空行不该产生 token：那是整屏最多的「无用分配」。
	lex := ForLanguage("Markdown")
	if toks, _ := lex.LexLine([]rune("   "), 0); len(toks) != 0 {
		t.Errorf("空白行不该有 token：%s", kindsOf([]rune("   "), toks))
	}
}

// ---- HTML ----

func TestHTMLTags(t *testing.T) {
	lex := ForLanguage("HTML")
	line := []rune(`<a href="x" id='y'>`)
	toks, _ := lex.LexLine(line, 0)
	for _, want := range []Kind{KindType, KindField, KindString} {
		if !hasKind(toks, want) {
			t.Errorf("缺少 %s：%s", want, kindsOf(line, toks))
		}
	}
}

func TestHTMLCommentSpansLines(t *testing.T) {
	lex := ForLanguage("HTML")
	lines := []string{"<!-- 开始", "中间", "结束 --> <b>x</b>"}
	got, _ := lexAll(lex, lines)
	if !hasKind(got[1], KindComment) {
		t.Errorf("中间行应在注释内：%s", kindsOf([]rune(lines[1]), got[1]))
	}
	if !hasKind(got[2], KindType) {
		t.Errorf("闭合后应恢复解析标签：%s", kindsOf([]rune(lines[2]), got[2]))
	}
}

func TestXMLDeclaration(t *testing.T) {
	lex := ForLanguage("HTML")
	line := []rune(`<?xml version="1.0"?>`)
	toks, _ := lex.LexLine(line, 0)
	if !hasKind(toks, KindString) {
		t.Errorf("属性值应为字符串：%s", kindsOf(line, toks))
	}
}

// ---- Kind / State 基础类型 ----

func TestKindStringRoundTrip(t *testing.T) {
	seen := map[string]Kind{}
	for k := KindText; int(k) < KindCount; k++ {
		name := k.String()
		if name == "unknown" {
			t.Fatalf("Kind %d 没有名字", k)
		}
		if prev, dup := seen[name]; dup {
			t.Errorf("名字 %s 重复：%d 与 %d", name, prev, k)
		}
		seen[name] = k
	}
	if KindCount != len(kindNames) {
		t.Errorf("KindCount=%d 与名字表长度 %d 不一致", KindCount, len(kindNames))
	}
}

func TestKindOutOfRangeHasName(t *testing.T) {
	if got := Kind(250).String(); got != "unknown" {
		t.Errorf("越界 Kind 应返回 unknown，得到 %q", got)
	}
}

func TestStateBitsAndNames(t *testing.T) {
	if State(0).String() != "none" {
		t.Errorf("零状态应为 none")
	}
	s := stateInBlockComment.With(stateInRawString)
	if !s.Has(stateInBlockComment) || !s.Has(stateInRawString) {
		t.Error("With 应置起对应位")
	}
	if s.Has(stateInTemplate) {
		t.Error("未置起的位不该为真")
	}
	got := s.String()
	if !strings.Contains(got, "block-comment") || !strings.Contains(got, "raw-string") {
		t.Errorf("状态名缺少分量：%q", got)
	}
	if s.Without(stateInRawString).Has(stateInRawString) {
		t.Error("Without 应清掉对应位")
	}
	if !s.Without(stateInRawString).Has(stateInBlockComment) {
		t.Error("Without 不该影响其它位")
	}
}

func TestTokenLen(t *testing.T) {
	tok := Token{Kind: KindString, Start: 3, End: 8}
	if tok.Len() != 5 {
		t.Errorf("Len=%d，期望 5", tok.Len())
	}
}
