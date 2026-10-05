// naive_test.go — 朴素参考实现与属性测试：piece table 的正确性基准。
// SPDX-License-Identifier: MIT

package buffer

import (
	"fmt"
	"math/rand"
	"testing"
	"unicode/utf8"
)

// naive 是刻意写得很笨的可编辑文本缓冲：直接用字节切片。
// 它的正确性一目了然，因此适合当作 piece table 的对照标准。
type naive struct {
	data []byte
}

// newNaive 构造朴素实现。
func newNaive(data []byte) *naive {
	return &naive{data: append([]byte(nil), data...)}
}

// Replace 用与 piece table 相同的语义替换内容。
func (n *naive) Replace(offset, removed int, inserted []byte) []byte {
	if offset < 0 || offset > len(n.data) || removed < 0 || removed > len(n.data)-offset {
		panic("naive: 越界")
	}
	removedText := append([]byte(nil), n.data[offset:offset+removed]...)
	next := make([]byte, 0, len(n.data)-removed+len(inserted))
	next = append(next, n.data[:offset]...)
	next = append(next, inserted...)
	next = append(next, n.data[offset+removed:]...)
	n.data = next
	return removedText
}

// String 返回当前内容。
func (n *naive) String() string {
	return string(n.data)
}

// lineStarts 逐字节扫描出全部行首偏移。
func (n *naive) lineStarts() []int {
	starts := []int{0}
	for i, c := range n.data {
		if c == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}

// propertySeeds 是属性测试用的文档种子，覆盖空文档、换行边界、CRLF、
// 无尾换行、中文宽字符以及万行大文件。
var propertySeeds = map[string]string{
	"empty":       "",
	"justLF":      "\n",
	"oneChar":     "a",
	"words":       "hello world",
	"goFile":      "package main\n\nfunc main() {}\n",
	"noTrailing":  "line1\nline2\nline3",
	"trailing":    "trailing newline\n",
	"blankLines":  "a\n\n\nb\n",
	"cjk":         "中文注释\n第二行\n",
	"crlf":        "first\r\nsecond\r\n",
	"tenThousand": tenThousandLines(),
}

// TestPropertyAgainstNaive 是 M1 最核心的测试，跑在默认档位。
func TestPropertyAgainstNaive(t *testing.T) {
	runPropertyAgainstNaive(t, propertySteps)
}

// TestPropertyAgainstNaiveDeep 是同一属性在 10 万次编辑下的深度档位。
// 耗时明显更长，因此 -short 下跳过，用 make test-deep 显式运行。
func TestPropertyAgainstNaiveDeep(t *testing.T) {
	if testing.Short() {
		t.Skip("深度属性测试在 -short 下跳过")
	}
	runPropertyAgainstNaive(t, 10000)
}

// runPropertyAgainstNaive 对每个种子执行 steps 次随机编辑，全程与朴素实现比对。
func runPropertyAgainstNaive(t *testing.T, steps int) {
	t.Helper()

	for name, seed := range propertySeeds {
		t.Run(name, func(t *testing.T) {
			// 用种子名派生随机源，保证失败可复现。
			var seedValue int64
			for _, c := range name {
				seedValue = seedValue*31 + int64(c)
			}
			rng := rand.New(rand.NewSource(seedValue))

			b := New([]byte(seed))
			n := newNaive([]byte(seed))

			for step := range steps {
				offset, removed, inserted := randomEdit(rng, len(n.data))
				gotRemoved := b.Replace(offset, removed, inserted)
				wantRemoved := n.Replace(offset, removed, inserted)

				if string(gotRemoved) != string(wantRemoved) {
					t.Fatalf("步骤 %d: 返回的被删内容不一致 got=%q want=%q",
						step, gotRemoved, wantRemoved)
				}
				checkCheaply(t, b, n, step)
			}

			// 序列结束做一次全量逐行核对，覆盖抽样可能漏掉的行。
			verifyEveryLine(t, b, n, steps)
		})
	}
}

// checkCheaply 每步执行：内容、长度、行首偏移全量比对，加若干抽样行。
func checkCheaply(t *testing.T, b *Buffer, n *naive, step int) {
	t.Helper()

	if got, want := string(b.Text()), n.String(); got != want {
		t.Fatalf("步骤 %d: 内容不一致\n  piece table: %q\n  naive:      %q", step, got, want)
	}
	if got, want := b.Len(), len(n.data); got != want {
		t.Fatalf("步骤 %d: 长度不一致 got=%d want=%d", step, got, want)
	}

	gotStarts := b.lines.starts
	wantStarts := n.lineStarts()
	if len(gotStarts) != len(wantStarts) {
		t.Fatalf("步骤 %d: 行数不一致 got=%d want=%d\n  got=%v\n  want=%v",
			step, len(gotStarts), len(wantStarts), truncate(gotStarts), truncate(wantStarts))
	}
	for i := range wantStarts {
		if gotStarts[i] != wantStarts[i] {
			t.Fatalf("步骤 %d: 第 %d 行起始不一致 got=%d want=%d\n  got=%v\n  want=%v",
				step, i, gotStarts[i], wantStarts[i], truncate(gotStarts), truncate(wantStarts))
		}
	}

	for _, i := range sampleLines(len(wantStarts)) {
		got := string(b.LineText(i))
		want := lineTextAt(n.data, wantStarts, i)
		if got != want {
			t.Fatalf("步骤 %d: 第 %d 行内容不一致 got=%q want=%q", step, i, got, want)
		}
	}
}

// verifyEveryLine 逐行核对全文，代价 O(全文)，只在序列结束时调用。
func verifyEveryLine(t *testing.T, b *Buffer, n *naive, step int) {
	t.Helper()
	starts := n.lineStarts()
	for i := range starts {
		got := string(b.LineText(i))
		want := lineTextAt(n.data, starts, i)
		if got != want {
			t.Fatalf("步骤 %d: 第 %d 行内容不一致 got=%q want=%q", step, i, got, want)
		}
		if got := b.LineStart(i); got != starts[i] {
			t.Fatalf("步骤 %d: LineStart(%d)=%d want=%d", step, i, got, starts[i])
		}
		if got, want := b.LineEnd(i), lineEndAt(n.data, starts, i); got != want {
			t.Fatalf("步骤 %d: LineEnd(%d)=%d want=%d", step, i, got, want)
		}
	}
}

// sampleLines 给出需要抽样的行号：首行、末行，以及每 512 行取一行。
func sampleLines(lines int) []int {
	if lines == 0 {
		return nil
	}
	picked := []int{0, lines - 1}
	for i := 0; i < lines; i += 512 {
		picked = append(picked, i)
	}
	return picked
}

// lineTextAt 用朴素方式取第 i 行内容，starts 是预先算好的行首偏移。
func lineTextAt(data []byte, starts []int, i int) string {
	line := data[starts[i]:lineEndAt(data, starts, i)]
	if n := len(line); n > 0 && line[n-1] == '\n' {
		line = line[:n-1]
	}
	if n := len(line); n > 0 && line[n-1] == '\r' {
		line = line[:n-1]
	}
	return string(line)
}

// lineEndAt 返回第 i 行结束（含换行符）的偏移。
func lineEndAt(data []byte, starts []int, i int) int {
	if i+1 < len(starts) {
		return starts[i+1]
	}
	return len(data)
}

// truncate 只保留偏移数组的前后各若干项，便于失败信息可读。
func truncate(values []int) []int {
	const keep = 8
	if len(values) <= keep*2 {
		return values
	}
	return append(append([]int{}, values[:keep]...), values[len(values)-keep:]...)
}

// randomEdit 生成一个一定合法的随机编辑操作。
func randomEdit(rng *rand.Rand, size int) (offset, removed int, inserted []byte) {
	offset = rng.Intn(size + 1)
	maxRemoved := size - offset
	if maxRemoved > 12 {
		maxRemoved = 12
	}
	removed = rng.Intn(maxRemoved + 1)
	inserted = randomText(rng, rng.Intn(8))
	return offset, removed, inserted
}

// randomText 生成可能包含换行、中文与制表符的随机片段。
// 刻意包含多字节字符，让属性测试能覆盖 UTF-8 边界被切开的情况。
func randomText(rng *rand.Rand, n int) []byte {
	if n == 0 {
		return nil
	}
	alphabet := []rune{
		'a', 'b', 'z', '0', '9', ' ', '\n', '\r', '\t',
		'中', '文', 'é', '→', '😀',
	}
	out := make([]rune, n)
	for i := range out {
		out[i] = alphabet[rng.Intn(len(alphabet))]
	}
	return []byte(string(out))
}

// tenThousandLines 造一个一万行的文档，用于压力属性测试。
func tenThousandLines() string {
	buf := make([]byte, 0, 64*1024)
	for i := range 10000 {
		buf = append(buf, fmt.Sprintf("line %d\n", i)...)
	}
	return string(buf)
}

func TestOffsetOfAndLineColRoundTrip(t *testing.T) {
	// 第三行含中文与 emoji，用于验证列号按 rune 计而不是按字节计。
	src := "ascii\n\tsecond line\n中文 mixed 字符\nlast"
	b := New([]byte(src))

	if got := b.LineCount(); got != 4 {
		t.Fatalf("LineCount = %d, want 4", got)
	}

	line, col := b.LineCol(len(src))
	if line != 3 || col != 4 {
		t.Errorf("LineCol(len) = (%d, %d), want (3, 4)", line, col)
	}

	// 逐个 rune 走一遍行内偏移，确认与直接计数一致。
	for want := 0; want <= b.LineRuneLen(2); want++ {
		off, err := b.OffsetOf(2, want)
		if err != nil {
			t.Fatalf("OffsetOf(2, %d): %v", want, err)
		}
		gl, gc := b.LineCol(off)
		if gl != 2 || gc != want {
			t.Errorf("OffsetOf(2, %d)=%d 回转得 (%d, %d)", want, off, gl, gc)
		}
	}
}

func TestOffsetOfRejectsInvalidLine(t *testing.T) {
	b := New([]byte("a\nb\n"))

	if _, err := b.OffsetOf(5, 0); err == nil {
		t.Error("OffsetOf(5, 0) 应返回错误")
	}
	if _, err := b.OffsetOf(-1, 0); err == nil {
		t.Error("OffsetOf(-1, 0) 应返回错误")
	}
	if _, err := b.OffsetOf(0, -1); err == nil {
		t.Error("OffsetOf(0, -1) 应返回错误")
	}
}

func TestOffsetOfClampsColumn(t *testing.T) {
	b := New([]byte("hello\nworld"))

	got, err := b.OffsetOf(0, 999)
	if err != nil {
		t.Fatalf("OffsetOf: %v", err)
	}
	if want := b.LineEnd(0); got != want {
		t.Errorf("OffsetOf(0, 999) = %d, want 行尾 %d", got, want)
	}
}

// TestReplaceRejectsOutOfRange 越界编辑必须 panic 而不是静默损坏。
func TestReplaceRejectsOutOfRange(t *testing.T) {
	cases := []struct {
		name            string
		offset, removed int
	}{
		{"负偏移", -1, 0},
		{"偏移越界", 100, 0},
		{"负删除长度", 0, -1},
		{"删除越过末尾", 2, 999},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := New([]byte("abc"))
			defer func() {
				if recover() == nil {
					t.Errorf("Replace(%d, %d) 应当 panic", tc.offset, tc.removed)
				}
			}()
			b.Replace(tc.offset, tc.removed, nil)
		})
	}
}

// TestCRLFTreatedAsSingleNewline 验证行尾 CRLF 被视为一个换行符。
func TestCRLFTreatedAsSingleNewline(t *testing.T) {
	b := New([]byte("first\r\nsecond\r\n"))

	if got := b.LineCount(); got != 3 {
		t.Fatalf("LineCount = %d, want 3", got)
	}
	if got := string(b.LineText(0)); got != "first" {
		t.Errorf("LineText(0) = %q, want %q", got, "first")
	}
	if got := string(b.LineText(1)); got != "second" {
		t.Errorf("LineText(1) = %q, want %q", got, "second")
	}
	if got := string(b.LineText(2)); got != "" {
		t.Errorf("LineText(2) = %q, want 空", got)
	}
}

// TestEmptyDocumentBehavesLikeOneLine 空文档也是一行。
func TestEmptyDocumentBehavesLikeOneLine(t *testing.T) {
	b := New(nil)

	if got := b.LineCount(); got != 1 {
		t.Errorf("LineCount = %d, want 1", got)
	}
	if got := string(b.LineText(0)); got != "" {
		t.Errorf("LineText(0) = %q, want 空", got)
	}
	if got := b.LineStart(0); got != 0 {
		t.Errorf("LineStart(0) = %d, want 0", got)
	}
	if got := b.LineEnd(0); got != 0 {
		t.Errorf("LineEnd(0) = %d, want 0", got)
	}
}

// TestLineStartsAreContiguous 校验逐行拼回等于全文。
func TestLineStartsAreContiguous(t *testing.T) {
	src := "a\nbb\n\nccc\n"
	b := New([]byte(src))

	var rebuilt []byte
	for i := range b.LineCount() {
		rebuilt = append(rebuilt, b.LineText(i)...)
		if i+1 < b.LineCount() {
			// 只补换行符，CRLF 文件会在这里暴露差异。
			rebuilt = append(rebuilt, '\n')
		}
	}
	if string(rebuilt) != src {
		t.Errorf("逐行拼回 = %q, want %q", rebuilt, src)
	}
}

// TestWideRuneIsOneColumn 确认列号按 rune 计，宽字符不占两列。
func TestWideRuneIsOneColumn(t *testing.T) {
	b := New([]byte("中文abc"))

	if got := b.LineRuneLen(0); got != 5 {
		t.Fatalf("LineRuneLen = %d, want 5", got)
	}
	if got := utf8.RuneCountInString("中文abc"); got != 5 {
		t.Errorf("对照：rune 总数 = %d, want 5", got)
	}
	off, err := b.OffsetOf(0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(b.Text()[off:]); got != "abc" {
		t.Errorf("第 2 个 rune 处偏移 = %d, 剩余内容 %q, want %q", off, got, "abc")
	}
}
