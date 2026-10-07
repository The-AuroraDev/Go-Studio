// view.go — 渲染：把文档状态变成一帧终端输出。
// SPDX-License-Identifier: MIT

// Package view 把编辑状态渲染成一帧 ANSI 文本与一个光标位置。
//
// 它是纯函数式的：不读终端、不写文件、不持有可变全局状态。
// 输入是一帧所需的全部信息，输出就是可以直接送给 screen.Screen 的字符串。
// 因此渲染逻辑可以完全脱离真实终端测试——这正是把渲染与输入分开的原因。
//
// 本包目前只渲染文本区。状态栏、标签栏、浮层由 editor 包在其上叠加，
// 它们各自有独立的测试，不与文本区搅在一起。
package view

import (
	"strconv"
	"strings"

	"github.com/mattn/go-runewidth"

	"github.com/29anan29/Go-Studio/internal/document"
	"github.com/29anan29/Go-Studio/internal/screen"
	"github.com/29anan29/Go-Studio/internal/syntax"
)

// Theme 集中定义配色。零值是一个可用的中性主题。
type Theme struct {
	// LineNumber 是行号颜色。
	LineNumber string
	// LineNumberCurrent 是当前行行号颜色。
	LineNumberCurrent string
	// CurrentLine 是当前行整行高亮。
	CurrentLine string
	// Gutter 是行号与文本之间的分隔区域。
	Gutter string
	// Text 是普通文本。
	Text string
	// Cursor 是插入光标的反显块。
	Cursor string
	// Syntax 是各类 token 的前景色，下标即 syntax.Kind。
	// 用数组而不是 map：每帧要查很多次，数组省掉哈希与分配。
	// 留空的种类回落到 Text，因此关掉配色只要整张表置空即可。
	Syntax [syntax.KindCount]string
}

// tokensOf 安全地取某一行的 token。opts.Tokens 可能是 nil 接口，
// 直接调方法会 panic，而渲染路径不该为「没开高亮」这种常态崩掉。
func (o Options) tokensOf(line int) []syntax.Token {
	if o.Tokens == nil {
		return nil
	}
	return o.Tokens.Tokens(line)
}

// syntaxColor 返回某类 token 的前景色，未配置时回落到普通文本色。
//
// 必须检查 Kind 是否越界。token 来自高亮器，属于外部数据：
// 一个越界的 Kind 会让数组索引 panic，而 panic 发生在渲染路径上，
// 后果是整个编辑器白屏，连纯文本都看不到。
func (t Theme) syntaxColor(kind syntax.Kind) string {
	if int(kind) >= 0 && int(kind) < len(t.Syntax) {
		if c := t.Syntax[kind]; c != noColor {
			return c
		}
	}
	return t.Text
}

// SyntaxTheme 返回带语法配色的深色主题。
//
// 每一种 token 都有各自可辨的颜色：既然切分器花力气把「字段」与
// 「普通标识符」分开，渲染层就不能给它们同一个颜色，否则那一层的
// 工作白做了。标点与普通文本则故意留空——满屏括号都有颜色只会更累眼。
func SyntaxTheme() Theme {
	t := DefaultTheme()
	t.Syntax = [syntax.KindCount]string{
		syntax.KindComment:     "\x1b[38;5;243m",
		syntax.KindString:      "\x1b[38;5;114m",
		syntax.KindEscape:      "\x1b[38;5;150m",
		syntax.KindNumber:      "\x1b[38;5;215m",
		syntax.KindKeyword:     "\x1b[38;5;176m",
		syntax.KindKeywordType: "\x1b[38;5;80m",
		syntax.KindBuiltin:     "\x1b[38;5;74m",
		syntax.KindFunction:    "\x1b[38;5;111m",
		syntax.KindType:        "\x1b[38;5;222m",
		syntax.KindField:       "\x1b[38;5;137m",
		syntax.KindConstant:    "\x1b[38;5;94m",
		syntax.KindOperator:    "\x1b[38;5;250m",
		syntax.KindPunct:       noColor,
		syntax.KindPreproc:     "\x1b[38;5;140m",
		syntax.KindLabel:       "\x1b[38;5;172m",
		syntax.KindInvalid:     "\x1b[1;38;5;203m",
	}
	return t
}

// noColor 是关闭配色时使用的空序列。
// 直接返回空串而不是加 ANSI 复位：没有样式就没有需要复位的东西，
// 混进复位序列反而可能把终端已有样式清掉。
const noColor = ""

// DefaultTheme 返回内置深色主题。
//
// 颜色用 ANSI 256 色而不是 24 位色：256 色在几乎所有终端上表现一致，
// 而 24 位色在不支持的终端上要么被降级、要么显示成乱码。
func DefaultTheme() Theme {
	return Theme{
		LineNumber:        "\x1b[38;5;244m",
		LineNumberCurrent: "\x1b[38;5;252m",
		CurrentLine:       "\x1b[38;5;236m",
		Gutter:            "\x1b[38;5;240m",
		Text:              noColor,
		Cursor:            "\x1b[7m",
	}
}

// PlainTheme 返回无配色主题，用于不支持颜色或用户关闭配色的终端。
func PlainTheme() Theme {
	return Theme{
		LineNumber:        noColor,
		LineNumberCurrent: noColor,
		CurrentLine:       noColor,
		Gutter:            noColor,
		Text:              noColor,
		Cursor:            "\x1b[7m",
	}
}

// reset 是清空所有样式的序列。
const reset = "\x1b[0m"

// CursorShape 是光标形状，取值与 screen.CursorShape 对应。
// 这里定义一份是为了不让 view 的对外接口依赖 screen 的类型。
type CursorShape = screen.CursorShape

// Options 描述一帧渲染所需的全部输入。
//
// 它刻意不含文档指针：调用方把需要的东西按值传进来，
// 这样渲染结果完全由这些值决定，测试里构造 Options 即可断言输出。
type Options struct {
	// Doc 是要渲染的文档，不可为 nil。
	Doc *document.Document
	// Width、Height 是文本区的可用尺寸（单位：终端列/行）。
	Width, Height int
	// Top 是视口显示的第一行行号。
	Top int
	// TabWidth 是制表符展开的列数，小于等于 0 时按 1 处理。
	TabWidth int
	// ShowLineNumbers 为真时显示行号槽。
	ShowLineNumbers bool
	// LineNumbersAbsolute 为真时显示绝对行号，否则当前行显示相对行号。
	LineNumbersAbsolute bool
	// Theme 是配色。
	Theme Theme
	// CursorShape 是光标形状。
	CursorShape CursorShape
	// CursorBlink 控制光标闪烁。
	CursorBlink bool
	// Tokens 按行提供语法 token，为 nil 时不高亮。
	// 只在需要时才问某一行：渲染层取可见行的 token，
	// 取不到就退回无色文本，代价只是少了高亮，不会出错。
	Tokens TokenSource
}

// TokenSource 是渲染层取语法 token 的接口。
// *syntax.Highlighter 直接满足它。
type TokenSource interface {
	Tokens(line int) []syntax.Token
}

// Frame 是一帧渲染的结果。
type Frame struct {
	// Content 是帧内容，不含末尾换行，也不做填充。
	Content string
	// Cursor 是光标位置，nil 表示本帧隐藏光标。
	Cursor *screen.CursorSpec
}

// FirstLineWidth 返回帧里第一行占用的终端列数。
//
// 量一帧的宽度一律用 VisibleWidth 而不是在这里另写一套：
// 内容里带 ANSI 转义序列时，朴素的逐 rune 相加会把转义序列里的
// 可打印字符也算进去，结果凭空多出十几列。
func (f Frame) FirstLineWidth() int {
	if idx := strings.IndexByte(f.Content, '\n'); idx >= 0 {
		return VisibleWidth(f.Content[:idx])
	}
	return VisibleWidth(f.Content)
}

// Render 把文档渲染成一帧。
//
// 输出的每一行都不带换行符——换行由 screen 层按行拼接，
// 这样最后一行的处理与中间行完全一致，不会出现差一个换行的经典错误。
func Render(opts Options) Frame {
	if opts.Doc == nil || opts.Height <= 0 || opts.Width <= 0 {
		return Frame{}
	}
	theme := opts.Theme
	if theme.Text == "" && theme.LineNumber == "" {
		// 未设置主题时退回无配色，保证零值可用。
		theme = PlainTheme()
	}

	doc := opts.Doc
	cursor := doc.Cursor()
	tabWidth := opts.TabWidth
	if tabWidth <= 0 {
		tabWidth = 1
	}

	// 行号槽 = 行号位数 + 两个空格。
	//
	// 这里刻意不用制表框字符（例如 │）来分隔：那些字符的终端宽度是
	// 「东亚歧义宽度」，同一个 U+2502 在不同终端与语言环境下会占 1 列或 2 列。
	// 一旦判断错了，整帧就会比终端宽出一列，终端把每一行都折成两行，
	// 整个编辑器立刻不可用。界面装饰必须用宽度确定的字符，
	// 宁可牺牲一点好看，也不能让帧的宽度变得不确定。
	gutterWidth := 0
	if opts.ShowLineNumbers {
		gutterWidth = numberWidth(doc.LineCount()) + 2
		// 终端窄到连行号都放不下时直接关掉行号：宁可少一列信息，
		// 也不能让整帧比终端宽出一列。
		if gutterWidth+1 > opts.Width {
			gutterWidth = 0
		}
	}

	lines := make([]string, 0, opts.Height)
	var cursorPos *screen.CursorSpec

	for row := 0; row < opts.Height; row++ {
		line := opts.Top + row
		if line >= doc.LineCount() {
			// 文档结束后的行补空白，帧的高度始终等于 opts.Height：
			// 高度不稳会让终端每帧都在滚动。
			lines = append(lines, strings.Repeat(" ", opts.Width))
			continue
		}

		isCurrent := line == cursor.Line
		var b strings.Builder
		x := 0

		// 行号槽连同分隔空白一起画：没有行号就不该留空隙，
		// 否则整帧凭空窄两列，看起来像渲染错位。
		if opts.ShowLineNumbers && gutterWidth > 0 {
			number := lineNumberText(line, isCurrent, doc.LineCount(), opts.LineNumbersAbsolute)
			paint := theme.LineNumber
			if isCurrent {
				paint = theme.LineNumberCurrent
			}
			b.WriteString(paint)
			b.WriteString(number)
			b.WriteString("  ")
			b.WriteString(reset)
			x += gutterWidth
		}

		textX := x
		body, cursorCellX := renderLine(
			doc, line, cursor, isCurrent, opts.Width-textX, tabWidth, theme, opts.tokensOf(line),
		)
		b.WriteString(body)
		if isCurrent && cursorCellX >= 0 {
			cursorPos = &screen.CursorSpec{
				X:     textX + cursorCellX,
				Y:     row,
				Shape: opts.CursorShape,
				Blink: opts.CursorBlink,
			}
		}
		lines = append(lines, b.String())
	}

	return Frame{
		Content: strings.Join(lines, "\n"),
		Cursor:  cursorPos,
	}
}

// renderLine 渲染一行的文本部分，返回文本与光标在本行内的 x。
// 光标不在可视范围内时返回的 x 为 -1。
// tokens 是该行的语法 token，可以为 nil（不高亮）。
// 颜色按 token 成段写入，不是每字符写一次：ANSI 转义序列有 5 个字节，
// 逐字符写会让一行的输出膨胀三倍。
func renderLine(
	doc *document.Document,
	line int,
	cursor document.Cursor,
	isCurrent bool,
	avail int,
	tabWidth int,
	theme Theme,
	tokens []syntax.Token,
) (string, int) {
	if avail <= 0 {
		return "", -1
	}

	text := []rune(string(doc.LineText(line)))
	var b strings.Builder
	x := 0
	cursorX := -1

	// ti 指向覆盖当前位置的 token，用单调指针推进。
	// 没有 token 覆盖的位置按普通文本处理：syntax.KindText 会映射回 theme.Text。
	ti := 0
	lastColor := noColor
	colored := false

	for i := 0; i < len(text); i++ {
		r := text[i]
		w := runeCells(r, tabWidth, x)
		// 宽字符放不下就整格截断，绝不让它只占半格——
		// 那会让后续所有列都错位，光标与文字也对不上。
		if w > 0 && x+w > avail {
			break
		}

		if isCurrent && i == cursor.Col {
			cursorX = x
			if lastColor != noColor {
				b.WriteString(reset)
			}
			b.WriteString(theme.Cursor)
			// 制表符在光标下也必须展开：光标框住的是一个真实空格，
			// 直接把制表符字节写进帧里会破坏整行的列对齐。
			b.WriteString(expandedRune(r, w))
			b.WriteString(reset)
			// 复位之后终端不再带任何样式，下一个字符必须重新写自己的颜色。
			lastColor = noColor
			colored = false
			x += w
			continue
		}

		color := theme.Text
		for ti < len(tokens) && tokens[ti].End <= i {
			ti++
		}
		if ti < len(tokens) && tokens[ti].Start <= i {
			color = theme.syntaxColor(tokens[ti].Kind)
		}
		if color != lastColor {
			b.WriteString(color)
			lastColor = color
			colored = color != noColor
		}

		if r == '\t' {
			b.WriteString(strings.Repeat(" ", w))
		} else {
			b.WriteString(string(r))
		}
		x += w
	}

	if isCurrent && cursor.Col >= len(text) && x < avail {
		// 光标停在行尾之外一格：文档以换行结尾时末行是空行，
		// 光标就落在补出来的空白上。
		cursorX = x
	}

	if colored {
		b.WriteString(reset)
	}

	// 行尾用空格补满，避免终端残留上一帧的残影。
	if x < avail {
		b.WriteString(strings.Repeat(" ", avail-x))
	}
	return b.String(), cursorX
}

// expandedRune 返回一个字符在帧里的实际写法：制表符展开成空格，其余原样。
func expandedRune(r rune, width int) string {
	if r == '\t' {
		return strings.Repeat(" ", width)
	}
	return string(r)
}

// runeCells 返回一个字符占用的列数。制表符按当前列对齐到下一个制表位。
func runeCells(r rune, tabWidth, col int) int {
	if r == '\t' {
		return tabWidth - col%tabWidth
	}
	w := runewidth.RuneWidth(r)
	if w <= 0 {
		// 零宽字符（组合符号等）不占列，但也不能让它把循环卡死。
		return 0
	}
	return w
}

// textWidth 返回一段文字占用的终端列数。
func textWidth(s string) int {
	total := 0
	for _, r := range s {
		total += runewidth.RuneWidth(r)
	}
	return total
}

// TextWidth 返回一段文字占用的终端列数。
//
// 界面里任何「这段文字有多宽」的判断都必须走它：字符个数与终端列数不是一回事，
// 一个汉字占两列，按字符个数算会让文本超出终端宽度、终端随即折行。
func TextWidth(s string) int { return textWidth(s) }

// VisibleWidth 返回一段**可能含 ANSI 转义序列**的渲染结果的终端列数。
//
// 它和 TextWidth 的区别正是这个函数存在的理由：转义序列本身不占列，
// 但其中的 "[38;5;236m" 全是可打印字符、宽度非零。
// 拿 TextWidth 去量一帧渲染结果，会凭空多出十几列，
// 于是「按它补齐到终端宽度」的逻辑全部算错，帧宽就不再精确。
//
// 判断一帧是否对齐，必须用这个函数。
func VisibleWidth(s string) int {
	return textWidth(stripANSI(s))
}

// stripANSI 去掉 ANSI 转义序列，只留下会被终端真正画出来的字符。
func stripANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if s[i] != 0x1b {
			b.WriteByte(s[i])
			i++
			continue
		}
		i = skipEscape(s, i)
	}
	return b.String()
}

// skipEscape 返回 s[i] 处转义序列结束后的下标。
// 支持 CSI（以 [ 开头、字母结尾）与 OSC（以 ] 开头、BEL 或 ST 结尾）。
func skipEscape(s string, i int) int {
	j := i + 1
	if j >= len(s) {
		return len(s)
	}
	switch s[j] {
	case '[':
		j++
		for j < len(s) && !(s[j] >= 0x40 && s[j] <= 0x7e) {
			j++
		}
		if j < len(s) {
			j++ // 跳过终止字节
		}
	case ']':
		// OSC：到 BEL 或 ST(ESC \) 为止。
		j++
		for j < len(s) {
			if s[j] == 0x07 {
				return j + 1
			}
			if s[j] == 0x1b && j+1 < len(s) && s[j+1] == '\\' {
				return j + 2
			}
			j++
		}
	default:
		// 裸 ESC（后面既不是 [ 也不是 ]）本身就是一个完整序列。
		// j 在进 switch 之前已经指向 ESC 之后的位置，
		// 这里绝不能再 j++——那会把后面那个正常字符一起吞掉，
		// 让这一行的可见宽度凭空少一列，补齐逻辑随之全错。
		return j
	}
	return j
}

// Fit 把文字裁到指定列数之内，放不下时末尾用省略号收尾。
//
// 必须按「显示宽度」而不是字符个数来裁：一个汉字占两列，
// 按字符个数裁会让中文文本超出终端宽度，终端随即折行，
// 整帧布局就崩了。这个函数是界面里唯一做宽度裁剪的地方。
//
// width 小到放不下省略号时直接硬裁，保证结果一定不超过 width。
func Fit(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if textWidth(text) <= width {
		return text
	}
	const ellipsis = "..."
	limit := width - textWidth(ellipsis)
	if limit <= 0 {
		// 连省略号都放不下：逐字符硬裁到刚好放得下为止。
		return clip(text, width)
	}
	return clip(text, limit) + ellipsis
}

// clip 按显示宽度截断文字，不会截出半个宽字符。
func clip(text string, width int) string {
	if width <= 0 {
		return ""
	}
	var b strings.Builder
	used := 0
	for _, r := range text {
		w := runewidth.RuneWidth(r)
		if used+w > width {
			break
		}
		b.WriteRune(r)
		used += w
	}
	return b.String()
}

// numberWidth 是最大行号所占的字符数。
// 行号从 1 开始，因此最大的是 lineCount 而不是 lineCount-1。
func numberWidth(lineCount int) int {
	return len(strconv.Itoa(max(lineCount, 1)))
}

// lineNumberText 生成行号文本，右对齐到行号槽宽度。
func lineNumberText(line int, isCurrent bool, lineCount int, absolute bool) string {
	text := strconv.Itoa(line + 1)
	if isCurrent && !absolute {
		text = "0"
	}
	return strings.Repeat(" ", max(numberWidth(lineCount)-len(text), 0)) + text
}
