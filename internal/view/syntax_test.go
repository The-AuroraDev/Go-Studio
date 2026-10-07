// syntax_test.go — 语法高亮在渲染层的接线测试。
// SPDX-License-Identifier: MIT

package view

import (
	"strings"
	"testing"

	"github.com/29anan29/Go-Studio/internal/document"
	"github.com/29anan29/Go-Studio/internal/syntax"
)

// stubTokens 是可控的 token 来源，用来把渲染层与具体词法器解耦。
// 这里要验的是「颜色有没有落在正确的位置」，不是「切分对不对」——
// 后者由 syntax 包自己的测试负责。
type stubTokens map[int][]syntax.Token

func (s stubTokens) Tokens(line int) []syntax.Token { return s[line] }

// syntaxOpts 返回一组带语法配色的渲染选项。
func syntaxOpts(doc *document.Document, tokens TokenSource) Options {
	opts := basicOpts(doc)
	opts.Theme = SyntaxTheme()
	opts.Tokens = tokens
	return opts
}

// codeTokens 把整行标成一种 kind，最省事的接缝测试。
func wholeLine(line int, kind syntax.Kind, length int) []syntax.Token {
	if length == 0 {
		return nil
	}
	return []syntax.Token{{Kind: kind, Start: 0, End: length}}
}

// stripColors 去掉全部 ANSI 序列，只留可见字符。
func stripColors(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// ---- 基本接线 ----

func TestSyntaxColorReachesFrame(t *testing.T) {
	doc := docOf("keyword here")
	frame := Render(syntaxOpts(doc, stubTokens{
		0: wholeLine(0, syntax.KindKeyword, len("keyword here")),
	}))
	line := strings.Split(frame.Content, "\n")[0]

	if !strings.Contains(line, SyntaxTheme().Syntax[syntax.KindKeyword]) {
		t.Errorf("帧里没有关键字色：%q", line)
	}
	// 加色不能改变可见内容。
	if got := strings.TrimRight(stripColors(line), " "); got != "keyword here" {
		t.Errorf("上色后可见文本变了：%q", got)
	}
}

func TestSyntaxColorPerTokenPosition(t *testing.T) {
	doc := docOf("aa bb cc")
	frame := Render(syntaxOpts(doc, stubTokens{0: []syntax.Token{
		{Kind: syntax.KindKeyword, Start: 0, End: 2},
		{Kind: syntax.KindString, Start: 3, End: 4},
		{Kind: syntax.KindNumber, Start: 6, End: 7},
	}}))
	line := strings.Split(frame.Content, "\n")[0]

	theme := SyntaxTheme()
	for _, kind := range []syntax.Kind{syntax.KindKeyword, syntax.KindString, syntax.KindNumber} {
		if !strings.Contains(line, theme.Syntax[kind]) {
			t.Errorf("缺少 %s 的颜色：%q", kind, line)
		}
	}
	if got := strings.TrimRight(stripColors(line), " "); got != "aa bb cc" {
		t.Errorf("上色后可见文本变了：%q", got)
	}
}

func TestNoTokensMeansNoColor(t *testing.T) {
	// 没给 token 来源就是纯文本模式，绝不能自己造颜色。
	doc := docOf("plain text")
	frame := Render(syntaxOpts(doc, nil))

	if strings.Contains(frame.Content, "\x1b[38;5;") {
		t.Errorf("无 token 时不该有前景色：%q", strings.Split(frame.Content, "\n")[0])
	}
	if got := strings.TrimRight(stripColors(strings.Split(frame.Content, "\n")[0]), " "); got != "plain text" {
		t.Errorf("可见文本变了：%q", got)
	}
}

func TestPlainThemeIgnoresTokens(t *testing.T) {
	// 关掉配色就该真的没有颜色：Terminal 不支持颜色时最关键。
	doc := docOf("keyword")
	opts := basicOpts(doc)
	opts.Tokens = stubTokens{0: wholeLine(0, syntax.KindKeyword, len("keyword"))}
	frame := Render(opts)

	if strings.Contains(strings.Split(frame.Content, "\n")[0], "\x1b[38;5;") {
		t.Errorf("PlainTheme 下不该上色：%q", strings.Split(frame.Content, "\n")[0])
	}
}

func TestUnmappedKindFallsBackToText(t *testing.T) {
	// 配色表里没配的种类要回落到普通文本色，而不是留上一段的颜色。
	doc := docOf("abcdef")
	theme := SyntaxTheme()
	theme.Syntax[syntax.KindField] = noColor

	opts := basicOpts(doc)
	opts.Theme = theme
	opts.Tokens = stubTokens{0: []syntax.Token{
		{Kind: syntax.KindKeyword, Start: 0, End: 3},
		{Kind: syntax.KindField, Start: 3, End: 6},
	}}
	line := strings.Split(Render(opts).Content, "\n")[0]

	if got := strings.TrimRight(stripColors(line), " "); got != "abcdef" {
		t.Errorf("可见文本变了：%q", got)
	}
	// 第二段不能继续带第一段的颜色。
	if idx := strings.Index(stripColors(line), "abcdef"); idx >= 0 {
		_ = idx
	}
	keywordCount := strings.Count(line, theme.Syntax[syntax.KindKeyword])
	if keywordCount != 1 {
		t.Errorf("关键字色出现了 %d 次，回落到 Text 的那段不该带色：%q", keywordCount, line)
	}
}

// ---- 光标与颜色的交互 ----

func TestCursorResetsSyntaxColor(t *testing.T) {
	// 光标块用反显实现，会把当前颜色清掉。
	// 若不复位，光标之后的所有字符都会丢掉自己的颜色——
	// 表现为「光标后面一片白」，在长代码行上非常显眼。
	doc := docOf("    keyword tail")
	doc.SetCursor(0, 0)
	frame := Render(syntaxOpts(doc, stubTokens{
		0: wholeLine(0, syntax.KindKeyword, len("    keyword tail")),
	}))
	line := strings.Split(frame.Content, "\n")[0]
	theme := SyntaxTheme()

	cursorAt := strings.Index(line, theme.Cursor)
	colorAt := strings.Index(line, theme.Syntax[syntax.KindKeyword])
	if cursorAt < 0 {
		t.Fatalf("没找到光标：%q", line)
	}
	if colorAt < 0 {
		t.Fatalf("没找到关键字色：%q", line)
	}
	// 光标之后必须重新写一次颜色，否则那一段是无色的。
	after := line[cursorAt:]
	if !strings.Contains(after, theme.Syntax[syntax.KindKeyword]) {
		t.Errorf("光标之后没有恢复颜色：%q", line)
	}
	if got := strings.TrimRight(stripColors(line), " "); got != "    keyword tail" {
		t.Errorf("可见文本变了：%q", got)
	}
}

func TestColorResetAtEndOfLine(t *testing.T) {
	// 每行结尾必须复位。漏掉的话，颜色会漏到下一行的行号槽上。
	doc := docOf("keyword")
	frame := Render(syntaxOpts(doc, stubTokens{0: wholeLine(0, syntax.KindKeyword, 7)}))
	for i, line := range strings.Split(frame.Content, "\n") {
		if i == 0 {
			continue
		}
		if strings.Contains(line, SyntaxTheme().Syntax[syntax.KindKeyword]) {
			t.Errorf("第 %d 行继承了上一行的颜色：%q", i, line)
		}
	}
}

// ---- 鲁棒性 ----

func TestMalformedTokensDoNotBreakRender(t *testing.T) {
	// token 是外部数据，越界或倒序都不该让渲染崩掉——
	// 崩掉就意味着整个编辑器白屏。
	cases := map[string][]syntax.Token{
		"越界终点":     {{Kind: syntax.KindString, Start: 0, End: 999}},
		"负起点":      {{Kind: syntax.KindString, Start: -5, End: 3}},
		"反向区间":     {{Kind: syntax.KindString, Start: 5, End: 2}},
		"零长度":      {{Kind: syntax.KindString, Start: 3, End: 3}},
		"重叠":       {{Kind: syntax.KindString, Start: 0, End: 5}, {Kind: syntax.KindNumber, Start: 2, End: 7}},
		"未排序":      {{Kind: syntax.KindNumber, Start: 6, End: 7}, {Kind: syntax.KindString, Start: 0, End: 3}},
		"Kind越界":   {{Kind: syntax.Kind(200), Start: 0, End: 3}},
		"空token集":  {},
		"超长行token": {{Kind: syntax.KindComment, Start: 0, End: 1 << 30}},
	}
	for name, toks := range cases {
		doc := docOf("keyword here")
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s: 渲染 panic 了：%v", name, r)
				}
			}()
			frame := Render(syntaxOpts(doc, stubTokens{0: toks}))
			// 不崩还不够，可见文本必须还在。
			got := strings.TrimRight(stripColors(strings.Split(frame.Content, "\n")[0]), " ")
			if got != "keyword here" {
				t.Errorf("%s: 可见文本 = %q", name, got)
			}
		}()
	}
}

func TestTokenSourceThatReturnsNilForVisibleLine(t *testing.T) {
	// 高亮器对没算过的行返回 nil，渲染层必须照常画纯文本。
	doc := docOf("one\ntwo\nthree")
	frame := Render(syntaxOpts(doc, stubTokens{1: wholeLine(1, syntax.KindKeyword, 3)}))
	lines := frameLines(frame)
	if len(lines) < 3 {
		t.Fatalf("帧行数不足：%d", len(lines))
	}
	if got := strings.TrimRight(lines[0], " "); got != "one" {
		t.Errorf("第 1 行 = %q", got)
	}
	if got := strings.TrimRight(lines[2], " "); got != "three" {
		t.Errorf("第 3 行 = %q", got)
	}
}

func TestSyntaxColorWithLineNumbersAndTabs(t *testing.T) {
	// 行号槽与制表符展开都不能被上色逻辑带偏。
	doc := docOf("\tkeyword\tvalue")
	opts := syntaxOpts(doc, stubTokens{0: wholeLine(0, syntax.KindKeyword, len("\tkeyword\tvalue"))})
	opts.ShowLineNumbers = true
	opts.TabWidth = 4
	frame := Render(opts)

	line := strings.Split(frame.Content, "\n")[0]
	if !strings.Contains(line, SyntaxTheme().Syntax[syntax.KindKeyword]) {
		t.Errorf("带行号与制表符时丢了颜色：%q", line)
	}
	// 制表符必须展开成空格：token 下标是 rune 下标，帧里是列。
	if strings.ContainsAny(stripColors(line), "\t") {
		t.Errorf("帧里仍有未展开的制表符：%q", stripColors(line))
	}
	// 展开宽度取决于绝对列（含行号槽），不值得在这里硬编码；
	// 与纯文本渲染逐字比对才是真正要守住的不变量。
	plainOpts := basicOpts(doc)
	plainOpts.ShowLineNumbers = true
	plainLine := strings.Split(Render(plainOpts).Content, "\n")[0]
	if a, b := stripColors(line), stripColors(plainLine); a != b {
		t.Errorf("上色后的可见文本与纯文本不同：%q vs %q", a, b)
	}
}

func TestSyntaxColorWithWideRunes(t *testing.T) {
	// 中文是宽字符：token 下标按 rune 走，帧里按列走，两者必须分开算。
	doc := docOf("变量 := \"值\"")
	frame := Render(syntaxOpts(doc, stubTokens{
		0: wholeLine(0, syntax.KindKeyword, len([]rune("变量 := \"值\""))),
	}))
	got := strings.TrimRight(stripColors(strings.Split(frame.Content, "\n")[0]), " ")
	if got != "变量 := \"值\"" {
		t.Errorf("宽字符行可见文本 = %q", got)
	}
}

func TestSyntaxThemeCoversEveryKind(t *testing.T) {
	// 每种 kind 要么有专属颜色，要么明确回落到 Text。
	// 漏配不会崩，但会出现「某类东西和普通文本同色」这种看不出来的退化。
	theme := SyntaxTheme()
	if theme.Syntax[syntax.KindText] != noColor {
		t.Error("KindText 应当留空，直接用 Text")
	}
	if theme.Syntax[syntax.KindPunct] != noColor {
		t.Error("KindPunct 建议留空：给标点上色会让画面很花")
	}
	// 其余种类至少要有配色，否则高亮等于没做。
	for k := syntax.KindComment; int(k) < syntax.KindCount; k++ {
		if k == syntax.KindPunct {
			continue
		}
		if theme.Syntax[k] == noColor {
			t.Errorf("%s 没有配色", k)
		}
	}
}

func TestSyntaxThemeColorsAreDistinct(t *testing.T) {
	// 两个 kind 用同一个颜色就没有区分意义。
	theme := SyntaxTheme()
	seen := map[string]syntax.Kind{}
	for k := syntax.KindComment; int(k) < syntax.KindCount; k++ {
		c := theme.Syntax[k]
		if c == noColor {
			continue
		}
		if prev, dup := seen[c]; dup {
			t.Errorf("%s 与 %s 用了同一个颜色 %q", k, prev, c)
		}
		seen[c] = k
	}
}

func TestTokensDoNotChangeGeometry(t *testing.T) {
	// 最关键的不变量：上色前后帧的几何完全一致。
	// 一旦上色改变了列数或行数，光标位置就会与文字错开。
	docs := []string{
		"package main",
		"\tindent := 1",
		"中文 := \"值\"",
		"emoji 🎉 here",
		"a := 1\nb := 2\nc := 3",
	}
	for _, content := range docs {
		doc := document.FromString(content)
		plain := Render(basicOpts(doc))
		lex := syntax.ForFilename("x.go")
		if lex == nil {
			t.Fatal("Go 规则缺失")
		}
		hl := syntax.NewHighlighter(doc, lex)
		colored := Render(syntaxOpts(doc, hl))

		gotC, wantC := colored.Cursor, plain.Cursor
		if gotC, wantC := gotC, wantC; (gotC == nil) != (wantC == nil) {
			t.Errorf("%q: 光标存在性变了 %v vs %v", content, gotC, wantC)
			continue
		}
		if gotC != nil && *gotC != *wantC {
			t.Errorf("%q: 光标位置变了 %+v vs %+v", content, gotC, wantC)
		}
		a := strings.Split(stripColors(colored.Content), "\n")
		b := strings.Split(stripColors(plain.Content), "\n")
		if len(a) != len(b) {
			t.Errorf("%q: 行数变了 %d vs %d", content, len(a), len(b))
			continue
		}
		for i := range a {
			if a[i] != b[i] {
				t.Errorf("%q: 第 %d 行可见内容变了 %q vs %q", content, i+1, a[i], b[i])
			}
		}
	}
}

func TestHighlighterSatisfiesTokenSource(t *testing.T) {
	// 编译期保证：*syntax.Highlighter 能直接当 TokenSource 用。
	var _ TokenSource = (*syntax.Highlighter)(nil)
	var _ TokenSource = stubTokens(nil)
}
