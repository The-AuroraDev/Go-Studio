// view_test.go — 渲染层的单元测试，重点是 buffer 到帧这条接缝。
// SPDX-License-Identifier: MIT

package view

import (
	"strings"
	"testing"

	"github.com/29anan29/Go-Studio/internal/document"
)

// frames 把帧拆成逐行的可见内容。
func frameLines(f Frame) []string {
	return strings.Split(stripANSI(f.Content), "\n")
}

// docOf 造一个关闭合并的文档。
func docOf(content string) *document.Document {
	return document.FromString(content)
}

// basicOpts 返回一组常用渲染选项：无行号、纯文本、40x10 视口。
func basicOpts(doc *document.Document) Options {
	return Options{
		Doc:             doc,
		Width:           40,
		Height:          10,
		TabWidth:        4,
		ShowLineNumbers: false,
		Theme:           PlainTheme(),
	}
}

// ---- 基本接缝：文档内容必须真的出现在帧里 ----

// TestRenderShowsDocumentContent 是最基本的一条：
// buffer 里的每一行都要出现在对应的那一屏行上。
func TestRenderShowsDocumentContent(t *testing.T) {
	doc := docOf("first\nsecond\nthird")
	frame := Render(basicOpts(doc))

	lines := frameLines(frame)
	if len(lines) != 10 {
		t.Fatalf("帧的行数 = %d, want 10（高度必须稳定）", len(lines))
	}
	want := []string{"first", "second", "third"}
	for i, text := range want {
		got := strings.TrimRight(lines[i], " ")
		if got != text {
			t.Errorf("第 %d 屏行 = %q, want %q", i, got, text)
		}
	}
}

// TestRenderPadsShortDocument 文档比视口短时，剩下的行补空白而不是留上一帧残影。
func TestRenderPadsShortDocument(t *testing.T) {
	doc := docOf("only")
	lines := frameLines(Render(basicOpts(doc)))

	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "" {
			t.Errorf("第 %d 屏行 = %q, want 全空白", i, lines[i])
		}
	}
}

// TestRenderLineCountIsStable 帧高度永远等于请求的高度。
// 高度不稳会让终端每帧滚动，这是最容易忽略也最影响体验的缺陷。
func TestRenderLineCountIsStable(t *testing.T) {
	for _, lines := range []int{0, 1, 5, 100} {
		doc := docOf(strings.Repeat("x\n", lines))
		for _, height := range []int{1, 3, 10} {
			opts := basicOpts(doc)
			opts.Height = height
			got := len(frameLines(Render(opts)))
			if got != height {
				t.Errorf("文档 %d 行、视口高度 %d 时帧高 = %d", lines, height, got)
			}
		}
	}
}

// TestRenderRespectsTop 视口起点必须生效，否则滚动功能无从谈起。
func TestRenderRespectsTop(t *testing.T) {
	doc := docOf("a\nb\nc\nd\ne")
	opts := basicOpts(doc)
	opts.Top = 2
	opts.Height = 3

	lines := frameLines(Render(opts))
	if got := strings.TrimRight(lines[0], " "); got != "c" {
		t.Errorf("首屏行 = %q, want %q", got, "c")
	}
	if got := strings.TrimRight(lines[2], " "); got != "e" {
		t.Errorf("末屏行 = %q, want %q", got, "e")
	}
}

// ---- 行号槽 ----

// TestRenderWithLineNumbers 行号槽形如 "1  "，数字后是两个空格。
// 分隔刻意不用制表框字符：它们的终端宽度属于「东亚歧义宽度」，
// 判断错一列就会让整帧比终端宽，终端把每行折成两行。
func TestRenderWithLineNumbers(t *testing.T) {
	doc := docOf("alpha\nbeta\ngamma")
	opts := basicOpts(doc)
	opts.ShowLineNumbers = true
	opts.LineNumbersAbsolute = true
	opts.Height = 4

	lines := frameLines(Render(opts))
	for i, want := range []string{"1  alpha", "2  beta", "3  gamma", ""} {
		if got := strings.TrimRight(lines[i], " "); got != want {
			t.Errorf("第 %d 屏行 = %q, want %q", i, got, want)
		}
	}
}

// TestNumberWidthGrowsWithLineCount 行号位数变化时槽宽必须跟着变，
// 否则超过 9 行后行号会挤进文本区。
func TestNumberWidthGrowsWithLineCount(t *testing.T) {
	cases := []struct {
		lines int
		want  int
	}{
		{1, 1},
		{9, 1},
		{10, 2},
		{99, 2},
		{100, 3},
		{1000, 4},
	}
	for _, tc := range cases {
		if got := numberWidth(tc.lines); got != tc.want {
			t.Errorf("numberWidth(%d) = %d, want %d", tc.lines, got, tc.want)
		}
	}
}

// TestRelativeLineNumbers 相对行号模式下当前行显示 0。
func TestRelativeLineNumbers(t *testing.T) {
	doc := docOf("a\nb\nc\nd")
	doc.SetCursor(2, 0)

	opts := basicOpts(doc)
	opts.ShowLineNumbers = true
	opts.Height = 4

	lines := frameLines(Render(opts))
	if !strings.HasPrefix(lines[2], "0  ") {
		t.Errorf("当前行 = %q, want 以 %q 开头", lines[2], "0  ")
	}
	if !strings.HasPrefix(lines[0], "1  ") {
		t.Errorf("上方一行 = %q, want 以 %q 开头", lines[0], "1  ")
	}

	opts.LineNumbersAbsolute = true
	lines = frameLines(Render(opts))
	if !strings.HasPrefix(lines[2], "3  ") {
		t.Errorf("绝对行号模式下当前行 = %q, want 以 %q 开头", lines[2], "3  ")
	}
}

// ---- 光标 ----

// TestCursorPosition 光标的行列必须与文档光标一致，
// 这是 buffer 到帧这条接缝上最容易出错的地方。
func TestCursorPosition(t *testing.T) {
	doc := docOf("abc\ndefg\nhi")
	doc.SetCursor(1, 2)

	frame := Render(basicOpts(doc))
	if frame.Cursor == nil {
		t.Fatal("光标为 nil")
	}
	if frame.Cursor.Y != 1 {
		t.Errorf("光标 Y = %d, want 1", frame.Cursor.Y)
	}
	if frame.Cursor.X != 2 {
		t.Errorf("光标 X = %d, want 2（无行号槽时列号即 x）", frame.Cursor.X)
	}
}

// TestCursorShiftsWithGutter 开了行号槽，光标必须整体右移。
func TestCursorShiftsWithGutter(t *testing.T) {
	doc := docOf("abc")
	doc.SetCursor(0, 1)

	opts := basicOpts(doc)
	opts.ShowLineNumbers = true
	frame := Render(opts)

	if frame.Cursor == nil {
		t.Fatal("光标为 nil")
	}
	// 行号槽 = 1 位行号 + 2 空格 = 3，光标在第 1 列即 x=3+1。
	if frame.Cursor.X != 4 {
		t.Errorf("光标 X = %d, want 4", frame.Cursor.X)
	}
}

// TestCursorAtEveryPosition 文档里每个位置都必须能算出合法光标坐标。
func TestCursorAtEveryPosition(t *testing.T) {
	doc := docOf("ab\n中文字符\nxyz")
	for line := 0; line < doc.LineCount(); line++ {
		for col := 0; col <= doc.LineRuneLen(line); col++ {
			doc.SetCursor(line, col)
			opts := basicOpts(doc)
			opts.Top = line
			opts.Height = 1
			opts.ShowLineNumbers = true

			frame := Render(opts)
			if frame.Cursor == nil {
				t.Fatalf("位置 {%d %d} 没有光标", line, col)
			}
			if frame.Cursor.Y != 0 {
				t.Errorf("位置 {%d %d} 光标 Y = %d, want 0", line, col, frame.Cursor.Y)
			}
			if frame.Cursor.X < 0 {
				t.Errorf("位置 {%d %d} 光标 X = %d, want >= 0", line, col, frame.Cursor.X)
			}
		}
	}
}

// TestCursorHiddenWhenOffscreen 光标行在视口之外时不画光标。
func TestCursorHiddenWhenOffscreen(t *testing.T) {
	doc := docOf("a\nb\nc\nd")
	doc.SetCursor(3, 0)

	opts := basicOpts(doc)
	opts.Top = 0
	opts.Height = 2

	if frame := Render(opts); frame.Cursor != nil {
		t.Errorf("光标行不在视口内时不应画光标，得到 %+v", frame.Cursor)
	}
}

// TestCursorShapeAndBlink 形状与闪烁必须透传。
func TestCursorShapeAndBlink(t *testing.T) {
	doc := docOf("abc")
	opts := basicOpts(doc)
	opts.CursorShape = 2
	opts.CursorBlink = true

	frame := Render(opts)
	if frame.Cursor == nil {
		t.Fatal("光标为 nil")
	}
	if frame.Cursor.Shape != 2 {
		t.Errorf("光标形状 = %v, want 2", frame.Cursor.Shape)
	}
	if !frame.Cursor.Blink {
		t.Error("光标闪烁未透传")
	}
}

// ---- 宽度处理 ----

// TestTabExpansion 制表符必须展开到下一个制表位。
func TestTabExpansion(t *testing.T) {
	doc := docOf("\tx")
	doc.SetCursor(0, 1) // 光标放到 x 上，让制表符走普通渲染路径
	opts := basicOpts(doc)
	opts.Height = 1
	opts.TabWidth = 4

	// 制表符从第 0 列展开到第 4 列，共 4 格，再加 x。
	got := strings.TrimRight(stripANSI(Render(opts).Content), " ")
	if got != "    x" {
		t.Errorf("制表符展开后 = %q, want %q", got, "    x")
	}
}

// TestTabUnderCursorIsExpanded 光标压在制表符上时也必须展开成空格。
// 直接把制表符字节写进帧里会让整行的列对齐崩掉。
func TestTabUnderCursorIsExpanded(t *testing.T) {
	doc := docOf("\tx")
	opts := basicOpts(doc)
	opts.Height = 1
	opts.TabWidth = 4

	got := strings.TrimRight(stripANSI(Render(opts).Content), " ")
	if got != "    x" {
		t.Errorf("光标下的制表符 = %q, want %q", got, "    x")
	}
	if strings.Contains(Render(opts).Content, "\t") {
		t.Error("帧里残留了制表符字节")
	}
}

// TestTabExpansionAlignment 制表位对齐到当前列，不是固定 4 格。
func TestTabExpansionAlignment(t *testing.T) {
	doc := docOf("ab\tx")
	opts := basicOpts(doc)
	opts.Height = 1
	opts.TabWidth = 4

	// "ab" 占 2 列，制表符补到第 4 列，共 2 格，再加 x。
	got := strings.TrimRight(stripANSI(Render(opts).Content), " ")
	if got != "ab  x" {
		t.Errorf("制表符展开 = %q, want %q", got, "ab  x")
	}
}

// TestWideCharactersTakeTwoColumns 中日韩字符占两列，
// 光标列与显示列因此不同。
func TestWideCharactersTakeTwoColumns(t *testing.T) {
	doc := docOf("中文")
	doc.SetCursor(0, 1) // 第二个中文字符

	opts := basicOpts(doc)
	opts.Height = 1

	frame := Render(opts)
	if frame.Cursor == nil {
		t.Fatal("光标为 nil")
	}
	// 第一个中文字符占 2 列，光标应落在 x=2。
	if frame.Cursor.X != 2 {
		t.Errorf("光标 X = %d, want 2（宽字符占两列）", frame.Cursor.X)
	}
}

// TestLongLineIsTruncated 超过视口宽度的行必须截断，不能换行破坏帧结构。
func TestLongLineIsTruncated(t *testing.T) {
	doc := docOf(strings.Repeat("x", 100))
	opts := basicOpts(doc)
	opts.Width = 10
	opts.Height = 1

	frame := Render(opts)
	lines := frameLines(frame)
	if len(lines) != 1 {
		t.Fatalf("帧行数 = %d, want 1（不能因为长行多出一行）", len(lines))
	}
	if got := len([]rune(lines[0])); got != 10 {
		t.Errorf("行宽 = %d, want 10", got)
	}
}

// TestFrameWidthIsExact 每一行的可见宽度都必须精确等于请求的宽度。
// 少一列会露出上一帧的残影，多一列会让终端折行。
func TestFrameWidthIsExact(t *testing.T) {
	docs := []string{
		"短",
		strings.Repeat("x", 100),
		"中文字符测试内容",
		"a\tb\tc",
		"emoji 😀 测试",
		"",
	}
	for _, content := range docs {
		for _, width := range []int{1, 5, 20, 40} {
			doc := docOf(content)
			opts := basicOpts(doc)
			opts.Width = width
			opts.Height = 3
			opts.ShowLineNumbers = true

			frame := Render(opts)
			for i, line := range frameLines(frame) {
				if got := displayWidth(line); got != width {
					t.Errorf("内容 %q、宽度 %d 时第 %d 行宽 = %d",
						content, width, i, got)
				}
			}
		}
	}
}

// TestZeroSizeRender 尺寸非法时必须返回空帧而不是 panic。
func TestZeroSizeRender(t *testing.T) {
	doc := docOf("abc")
	for _, opts := range []Options{
		{Doc: doc, Width: 0, Height: 5},
		{Doc: doc, Width: 5, Height: 0},
		{Doc: doc, Width: -1, Height: -1},
		{Doc: nil, Width: 5, Height: 5},
	} {
		frame := Render(opts)
		if frame.Content != "" {
			t.Errorf("尺寸非法时帧内容 = %q, want 空", frame.Content)
		}
		if frame.Cursor != nil {
			t.Errorf("尺寸非法时不该有光标，得到 %+v", frame.Cursor)
		}
	}
}

// ---- 主题 ----

// TestPlainThemeHasNoColor 无配色主题只允许出现光标反显与复位，不允许任何颜色序列。
func TestPlainThemeHasNoColor(t *testing.T) {
	doc := docOf("hello")
	opts := basicOpts(doc)
	opts.ShowLineNumbers = true
	opts.Height = 2

	content := Render(opts).Content
	for _, seq := range []string{"\x1b[38;5;", "\x1b[48;5;", "\x1b[1m", "\x1b[4m"} {
		if strings.Contains(content, seq) {
			t.Errorf("无配色主题输出了样式序列 %q", seq)
		}
	}
}

// TestPlainThemeIsAlmostFree 无配色主题不该为每个字符都套一遍复位序列：
// 那会让帧体积翻好几倍，窄终端上尤其明显。
func TestPlainThemeIsAlmostFree(t *testing.T) {
	doc := docOf("hello world")
	opts := basicOpts(doc)
	opts.Height = 1

	content := Render(opts).Content
	// 只允许光标那一处反显与对应复位。
	if got := strings.Count(content, reset); got > 1 {
		t.Errorf("无配色主题输出 %d 次复位序列（want <= 1）", got)
	}
}

// TestColoredThemeEmitsStyle 彩色主题应输出样式序列。
func TestColoredThemeEmitsStyle(t *testing.T) {
	doc := docOf("hello")
	opts := basicOpts(doc)
	opts.Theme = DefaultTheme()
	opts.Height = 2

	if !strings.Contains(Render(opts).Content, "\x1b[") {
		t.Error("彩色主题没有输出任何样式序列")
	}
}

// TestZeroThemeFallsBackToPlain 零值主题必须可用，不能输出空样式导致的错乱。
func TestZeroThemeFallsBackToPlain(t *testing.T) {
	doc := docOf("hello")
	opts := Options{Doc: doc, Width: 20, Height: 2, ShowLineNumbers: true}
	frame := Render(opts)
	if strings.Contains(frame.Content, "\x1b[38;5;") {
		t.Error("零值主题不该输出 256 色序列")
	}
}

// ---- 编辑后的帧 ----

// TestRenderFollowsEdits 编辑后帧必须反映新内容，而不是上一帧的缓存。
func TestRenderFollowsEdits(t *testing.T) {
	doc := docOf("before")
	opts := basicOpts(doc)
	opts.Height = 1

	if got := strings.TrimRight(frameLines(Render(opts))[0], " "); got != "before" {
		t.Fatalf("编辑前 = %q", got)
	}

	doc.MoveDocEnd()
	doc.InsertText(" after")

	if got := strings.TrimRight(frameLines(Render(opts))[0], " "); got != "before after" {
		t.Errorf("编辑后 = %q, want %q", got, "before after")
	}
}

// TestRenderFollowsCursorMoves 光标移动后帧里的光标坐标必须跟着变。
func TestRenderFollowsCursorMoves(t *testing.T) {
	doc := docOf("abcdef\nghijkl")
	opts := basicOpts(doc)
	opts.Height = 2

	doc.SetCursor(0, 0)
	if got := Render(opts).Cursor.X; got != 0 {
		t.Fatalf("初始光标 X = %d, want 0", got)
	}

	doc.SetCursor(0, 3)
	if got := Render(opts).Cursor.X; got != 3 {
		t.Errorf("移动后光标 X = %d, want 3", got)
	}

	doc.SetCursor(1, 2)
	frame := Render(opts)
	if frame.Cursor.X != 2 || frame.Cursor.Y != 1 {
		t.Errorf("跨行后光标 = (%d,%d), want (2,1)", frame.Cursor.X, frame.Cursor.Y)
	}
}

// displayWidth 返回一行可见内容的终端列数。
func displayWidth(s string) int {
	total := 0
	for _, r := range s {
		w := runeCells(r, 4, total)
		total += w
	}
	return total
}
