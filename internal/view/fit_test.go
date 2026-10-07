// fit_test.go — 宽度裁剪的单元测试。
// SPDX-License-Identifier: MIT

package view

import (
	"strings"
	"testing"
)

func TestTextWidth(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int
	}{
		{"空", "", 0},
		{"ASCII", "abc", 3},
		{"汉字占两列", "中文", 4},
		{"混排", "a中b", 4},
		// 制表符是控制字符，本身为零宽：它必须在渲染阶段按 tab_width
		// 展开成空格之后再交给 TextWidth，交给原始制表符没有意义。
		{"原始制表符是零宽", "\t", 0},
		{"emoji", "😀", 2},
		{"空格", "  ", 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := TextWidth(tc.in); got != tc.want {
				t.Errorf("TextWidth(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

// TestFitKeepsShortText 放得下就原样返回，不该加省略号。
func TestFitKeepsShortText(t *testing.T) {
	cases := []struct {
		in    string
		width int
		want  string
	}{
		{"abc", 10, "abc"},
		{"abc", 3, "abc"},
		{"中文", 4, "中文"},
		{"中文", 5, "中文"},
		{"", 5, ""},
	}
	for _, tc := range cases {
		if got := Fit(tc.in, tc.width); got != tc.want {
			t.Errorf("Fit(%q, %d) = %q, want %q", tc.in, tc.width, got, tc.want)
		}
	}
}

// TestFitTruncatesOverlongText 放不下要截断并加省略号。
func TestFitTruncatesOverlongText(t *testing.T) {
	cases := []struct {
		in    string
		width int
		want  string
	}{
		{"abcdefgh", 5, "ab..."},
		// 宽度 3 恰好放得下三个字符，此时返回原文而不是光秃秃的省略号：
		// 省略号不携带任何信息，而这三个字符是。
		{"abcdef", 3, "abc"},
		{"abcdefghij", 5, "ab..."},
	}
	for _, tc := range cases {
		if got := Fit(tc.in, tc.width); got != tc.want {
			t.Errorf("Fit(%q, %d) = %q, want %q", tc.in, tc.width, got, tc.want)
		}
	}
}

// TestFitNeverExceedsWidth 是这条函数存在的全部理由：
// 结果必须永远不超过给定宽度，否则状态栏会被终端折行，整帧布局就崩了。
func TestFitNeverExceedsWidth(t *testing.T) {
	texts := []string{
		"",
		"a",
		"abc",
		strings.Repeat("x", 100),
		"中文中文中文中文",
		strings.Repeat("很长的中文提示", 30),
		"mixed 中英文 abc 混排内容测试",
		strings.Repeat("😀", 20),
		strings.Repeat("a中", 40),
	}
	for _, text := range texts {
		for width := 0; width <= 40; width++ {
			got := Fit(text, width)
			if w := TextWidth(got); w > width {
				t.Fatalf("Fit(%q, %d) = %q，宽 %d 超出上限", text, width, got, w)
			}
		}
	}
}

// TestFitDoesNotSplitWideCharacter 裁剪不能切出半个宽字符，
// 那会让光标与文字对不齐。
func TestFitDoesNotSplitWideCharacter(t *testing.T) {
	// 一个汉字占两列，宽度 3 只能放 1 个字加省略号之外的余量。
	got := Fit("中文", 3)
	if w := TextWidth(got); w > 3 {
		t.Errorf("Fit(%q, 3) = %q，宽 %d 超出上限", "中文", got, w)
	}
	for _, r := range got {
		if r == '\ufffd' {
			t.Errorf("Fit 产生了替换字符：%q", got)
		}
	}
}

// TestFitWithTinyWidth 宽度小到放不下省略号时也要硬裁到上限内。
func TestFitWithTinyWidth(t *testing.T) {
	for _, width := range []int{0, 1, 2, 3} {
		got := Fit("abcdef", width)
		if w := TextWidth(got); w > width {
			t.Errorf("Fit(%q, %d) = %q，宽 %d 超出上限", "abcdef", width, got, w)
		}
	}
}

func TestClip(t *testing.T) {
	cases := []struct {
		in    string
		width int
		want  string
	}{
		{"", 5, ""},
		{"abc", 5, "abc"},
		{"abcdef", 3, "abc"},
		{"中文", 2, "中"},
		{"中文", 4, "中文"},
		{"中文", 3, "中"}, // 放不下第二个字就停
		{"abc", 0, ""},
		{"abc", -1, ""},
	}
	for _, tc := range cases {
		if got := clip(tc.in, tc.width); got != tc.want {
			t.Errorf("clip(%q, %d) = %q, want %q", tc.in, tc.width, got, tc.want)
		}
	}
}

// TestClipNeverExceedsWidth clip 的结果必须刚好放得下。
func TestClipNeverExceedsWidth(t *testing.T) {
	for _, text := range []string{"abcdef", "中文中文", "a中b文c", "😀😀😀"} {
		for width := 0; width <= 10; width++ {
			if got := clip(text, width); TextWidth(got) > width {
				t.Fatalf("clip(%q, %d) = %q，宽 %d 超出上限", text, width, got, TextWidth(got))
			}
		}
	}
}

// TestTextWidthMatchesRenderedWidth TextWidth 与实际渲染的宽度必须一致，
// 否则「裁剪后放得下」并不能保证「渲染时不折行」。
func TestTextWidthMatchesRenderedWidth(t *testing.T) {
	for _, text := range []string{"abc", "中文", "a中b", "😀", "a\tb"} {
		doc := docOf(text)
		opts := basicOpts(doc)
		opts.Width = 30
		opts.Height = 1

		line := frameLines(Render(opts))[0]
		if got, want := TextWidth(line), 30; got != want {
			t.Errorf("内容 %q 的渲染宽度 = %d, want %d", text, got, want)
		}
	}
}

// TestStripANSIRemovesEscapeSequences 剥转义是 VisibleWidth 的前提，
// 必须把 CSI 与 OSC 两类都剥干净。
func TestStripANSIRemovesEscapeSequences(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"无转义", "hello", "hello"},
		{"SGR 颜色", "\x1b[38;5;236mhi\x1b[0m", "hi"},
		{"反显", "\x1b[7mx\x1b[m", "x"},
		{"光标定位", "abc\x1b[10;1Hdef", "abcdef"},
		{"清屏", "a\x1b[2Jb", "ab"},
		{"OSC 以 BEL 结束", "a\x1b]0;title\x07b", "ab"},
		{"OSC 以 ST 结束", "a\x1b]0;title\x1b\\b", "ab"},
		{"单独一个 ESC", "a\x1bb", "ab"},
		{"末尾的 ESC", "ab\x1b", "ab"},
		{"只有转义", "\x1b[0m", ""},
		{"中文与转义混合", "\x1b[1m中文\x1b[0m", "中文"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := stripANSI(tc.in); got != tc.want {
				t.Errorf("stripANSI(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestVisibleWidthIgnoresEscapeSequences 这条不变量是整个帧宽机制的地基：
// 同样的可见内容，套不套样式都必须量出同样的宽度。
func TestVisibleWidthIgnoresEscapeSequences(t *testing.T) {
	plain := "hello"
	styled := "\x1b[38;5;236mhello\x1b[0m"

	if TextWidth(plain) != VisibleWidth(styled) {
		t.Errorf("无样式宽 %d、带样式宽 %d，want 相等", TextWidth(plain), VisibleWidth(styled))
	}
	// 反过来：如果用 TextWidth 去量带样式的串，结果会被抬高。
	if TextWidth(styled) <= TextWidth(plain) {
		t.Error("TextWidth 本应把转义序列的可打印字符算进去（这正是不能用它量帧宽的原因）")
	}
}

func TestVisibleWidth(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int
	}{
		{"空", "", 0},
		{"纯文本", "abc", 3},
		{"纯中文", "中文", 4},
		{"带样式", "\x1b[1m中文\x1b[0m", 4},
		// 换行不占列，所以是两行之和 2+2。
		{"跨行求和", "ab\ncd", 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := VisibleWidth(tc.in); got != tc.want {
				t.Errorf("VisibleWidth(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

// TestFirstLineWidth 帧宽只看第一行，且要正确忽略转义序列。
func TestFirstLineWidth(t *testing.T) {
	cases := []struct {
		name  string
		frame string
		want  int
	}{
		{"单行", "abc", 3},
		{"多行", "abc\ndefgh", 3},
		{"带样式", "\x1b[38;5;236mabc\x1b[0m\nx", 3},
		{"中文", "中文\nx", 4},
		{"空", "", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := Frame{Content: tc.frame}
			if got := f.FirstLineWidth(); got != tc.want {
				t.Errorf("FirstLineWidth(%q) = %d, want %d", tc.frame, got, tc.want)
			}
		})
	}
}

// TestFirstLineWidthMatchesVisibleWidth 帧宽与直接量可见宽度必须一致，
// 否则补齐逻辑会算错。
func TestFirstLineWidthMatchesVisibleWidth(t *testing.T) {
	for _, content := range []string{
		"abc",
		"\x1b[38;5;236mabc\x1b[0m",
		"中文",
		"a\tb",
		strings.Repeat("x", 50),
	} {
		f := Frame{Content: content}
		want := VisibleWidth(content)
		if got := f.FirstLineWidth(); got != want {
			t.Errorf("内容 %q：FirstLineWidth = %d, VisibleWidth = %d，want 相等",
				content, got, want)
		}
	}
}

// TestSkipEscapeHandlesTruncatedInput 残缺的转义序列不能让函数越界 panic：
// 终端输出可能被截断，也可能被测试的假数据打断。
func TestSkipEscapeHandlesTruncatedInput(t *testing.T) {
	cases := []string{
		"\x1b",
		"\x1b[",
		"\x1b[3",
		"\x1b[38;5;",
		"\x1b]",
		"\x1b]0;title",
		"\x1b]0;title\x1b",
	}
	for _, in := range cases {
		if got := stripANSI(in); got != "" {
			t.Errorf("stripANSI(%q) = %q, want 空（残缺转义应被整个丢弃）", in, got)
		}
	}
}
