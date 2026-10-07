// motion_test.go — 光标移动的单元测试。
// SPDX-License-Identifier: MIT

package document

import (
	"fmt"
	"strings"
	"testing"
)

// docOf 造一个关闭合并的文档，起点光标在 (0,0)。
func docOf(content string) *Document { return newTestDoc(content) }

// assertCursor 断言光标位置，失败信息里带上文档内容便于定位。
func assertCursor(t *testing.T, d *Document, line, col int) {
	t.Helper()
	got := d.Cursor()
	if got.Line != line || got.Col != col {
		t.Fatalf("光标 = %+v, want {%d %d}；内容=%q",
			got, line, col, strings.ReplaceAll(string(d.Text()), "\n", "\\n"))
	}
}

func TestMoveLeftRightWithinLine(t *testing.T) {
	d := docOf("abc")
	d.MoveRight()
	assertCursor(t, d, 0, 1)
	d.MoveRight()
	assertCursor(t, d, 0, 2)
	d.MoveLeft()
	assertCursor(t, d, 0, 1)
	d.MoveLeft()
	d.MoveLeft()
	assertCursor(t, d, 0, 0)
	// 行首再左移不动。
	d.MoveLeft()
	assertCursor(t, d, 0, 0)
}

func TestMoveRightStopsAtLineEnd(t *testing.T) {
	d := docOf("ab")
	d.MoveRight()
	d.MoveRight()
	assertCursor(t, d, 0, 2)
	d.MoveRight()
	assertCursor(t, d, 0, 2)
}

// TestMoveAcrossLineBoundary 跨行移动必须在两个方向都成立。
func TestMoveAcrossLineBoundary(t *testing.T) {
	d := docOf("ab\ncd")

	d.MoveLineEnd()
	d.MoveRight()
	assertCursor(t, d, 1, 0)
	d.MoveLeft()
	assertCursor(t, d, 0, 2)
}

// TestMoveLeftFromLineStartJoinsPrevLine 行首左移要落到上一行行尾。
func TestMoveLeftFromLineStartJoinsPrevLine(t *testing.T) {
	d := docOf("abc\nde")
	d.SetCursor(1, 0)
	d.MoveLeft()
	assertCursor(t, d, 0, 3)
}

// TestDesiredColumnSurvivesShortLine 是移动手感最容易出错的地方：
// 光标在长行第 8 列，下移穿过短行时被压到行尾，
// 再移回长行时必须弹回第 8 列，而不是停在短行留下的位置。
func TestDesiredColumnSurvivesShortLine(t *testing.T) {
	d := docOf("1234567890\nab\n1234567890")

	d.SetCursor(0, 8)
	d.MoveDown()
	assertCursor(t, d, 1, 2) // 被压到短行行尾

	d.MoveDown()
	assertCursor(t, d, 2, 8) // 必须弹回期望列

	d.MoveUp()
	assertCursor(t, d, 1, 2)
	d.MoveUp()
	assertCursor(t, d, 0, 8)
}

// TestHorizontalMoveUpdatesDesiredColumn 水平移动要重置期望列，
// 否则用户先左右挪动再上下移动会跳到不期望的列。
func TestHorizontalMoveUpdatesDesiredColumn(t *testing.T) {
	d := docOf("1234567890\n1234567890")
	d.SetCursor(0, 8)
	d.MoveLeft()
	d.MoveLeft()
	assertCursor(t, d, 0, 6)

	d.MoveDown()
	assertCursor(t, d, 1, 6) // 期望列已更新为 6，不是 8
}

// TestEditResetsDesiredColumn 编辑之后期望列要跟着新光标走，
// 否则插入字符再上下移动会跳到旧列。
func TestEditResetsDesiredColumn(t *testing.T) {
	d := docOf("1234567890\n1234567890")
	d.SetCursor(0, 5)
	d.InsertRune('x')
	assertCursor(t, d, 0, 6)

	d.MoveDown()
	assertCursor(t, d, 1, 6)
}

// TestMoveUpAtFirstLineStays 顶行再上移必须不动。
func TestMoveUpAtFirstLineStays(t *testing.T) {
	d := docOf("a\nb\nc")
	d.MoveUp()
	assertCursor(t, d, 0, 0)
}

// TestMoveDownAtLastLineStays 末行再下移必须不动。
func TestMoveDownAtLastLineStays(t *testing.T) {
	d := docOf("a\nb\nc")
	d.MoveDocEnd()
	d.MoveDown()
	assertCursor(t, d, 2, 1)
}

func TestMoveLineStartEnd(t *testing.T) {
	d := docOf("abc\ndefgh")
	d.MoveDocEnd()
	assertCursor(t, d, 1, 5)

	d.MoveLineStart()
	assertCursor(t, d, 1, 0)
	d.MoveLineEnd()
	assertCursor(t, d, 1, 5)
}

func TestMoveDocStartEnd(t *testing.T) {
	d := docOf("abc\ndef")
	d.MoveDocStart()
	assertCursor(t, d, 0, 0)
	d.MoveDocEnd()
	assertCursor(t, d, 1, 3)
}

func TestGotoLine(t *testing.T) {
	d := docOf("a\nb\nc\nd")
	d.GotoLine(2)
	assertCursor(t, d, 2, 0)
	// 越界要截断而不是 panic。
	d.GotoLine(99)
	assertCursor(t, d, 3, 0)
	d.GotoLine(-1)
	assertCursor(t, d, 0, 0)
}

func TestPageUpDown(t *testing.T) {
	d := docOf(strings.TrimSuffix(strings.Repeat("line\n", 100), "\n"))
	d.GotoLine(50)

	d.PageDown(10)
	assertCursor(t, d, 60, 0)
	d.PageDown(1000) // 到底
	assertCursor(t, d, 99, 0)

	d.PageUp(10)
	assertCursor(t, d, 89, 0)
	d.PageUp(1000) // 到顶
	assertCursor(t, d, 0, 0)

	// 高度非法时按 1 处理，不能原地不动。
	d.GotoLine(5)
	d.PageDown(0)
	assertCursor(t, d, 6, 0)
}

// TestPageKeepsDesiredColumn 翻页也是垂直移动，必须保持期望列。
func TestPageKeepsDesiredColumn(t *testing.T) {
	d := docOf("123456\nab\n123456\n123456")
	d.SetCursor(0, 5)
	d.PageDown(1)
	assertCursor(t, d, 1, 2)
	d.PageDown(1)
	assertCursor(t, d, 2, 5)
}

// ---- 按词移动 ----

func TestMoveWordRight(t *testing.T) {
	// 语义是「下一个词的词首」（Vim 的 w），不是 Emacs M-f 的「当前词末尾」。
	// "hello world foo"：0 → world(6) → foo(12) → 行尾(15) → 不动
	d := docOf("hello world foo")
	cases := []int{6, 12, 15, 15}
	for i, want := range cases {
		d.MoveWordRight()
		if got := d.Cursor().Col; got != want {
			t.Fatalf("第 %d 次 MoveWordRight 后列 = %d, want %d", i+1, got, want)
		}
	}
}

func TestMoveWordRightAcrossLines(t *testing.T) {
	d := docOf("abc\ndef")
	// 本行没有下一个词时落到下一行行首。
	d.MoveWordRight()
	assertCursor(t, d, 1, 0)
	d.MoveWordRight()
	assertCursor(t, d, 1, 3)
	d.MoveWordRight() // 已是最后，不能再动
	assertCursor(t, d, 1, 3)
}

func TestMoveWordLeft(t *testing.T) {
	// "hello world foo"：行尾(15) → foo(12) → world(6) → 开头(0) → 不动
	d := docOf("hello world foo")
	d.MoveDocEnd()
	for _, want := range []int{12, 6, 0, 0} {
		d.MoveWordLeft()
		if got := d.Cursor().Col; got != want {
			t.Fatalf("MoveWordLeft 后列 = %d, want %d", got, want)
		}
	}
}

func TestMoveWordLeftCrossesLines(t *testing.T) {
	d := docOf("abc\ndef")
	d.MoveDocEnd() // (1, 3)
	d.MoveWordLeft()
	assertCursor(t, d, 1, 0)
	d.MoveWordLeft()
	assertCursor(t, d, 0, 0)
}

// TestWordMotionWithPunctuation 分隔符与词要交替处理。
func TestWordMotionWithPunctuation(t *testing.T) {
	d := docOf("a, b; c")
	d.MoveWordRight()
	assertCursor(t, d, 0, 3) // b
	d.MoveWordRight()
	assertCursor(t, d, 0, 6) // c
	d.MoveWordLeft()
	assertCursor(t, d, 0, 3)
}

// TestWordMotionWithCJK 中文字符整体算一个词。
func TestWordMotionWithCJK(t *testing.T) {
	// "你好 世界"：你好(0-1) 空格(2) 世界(3-4)
	d := docOf("你好 世界")
	d.MoveWordRight()
	assertCursor(t, d, 0, 3) // 「世界」的词首
	d.MoveWordRight()
	assertCursor(t, d, 0, 5) // 行尾
	d.MoveWordLeft()
	assertCursor(t, d, 0, 3)
	d.MoveWordLeft()
	assertCursor(t, d, 0, 0)
}

// TestWordMotionOnEmptyLine 空行上移动不越界。
func TestWordMotionOnEmptyLine(t *testing.T) {
	d := docOf("\nabc")
	d.GotoLine(0)
	d.MoveWordLeft()
	assertCursor(t, d, 0, 0)
	d.MoveWordRight()
	assertCursor(t, d, 1, 0)
}

// ---- 移动不变量 ----

// TestRandomMotionKeepsCursorValid 用随机移动做一次不变量检查：
// 光标必须始终落在合法范围内，且 Text 不被移动改动。
func TestRandomMotionKeepsCursorValid(t *testing.T) {
	content := "alpha beta\n\n中文字符测试\n  indented\nlast"
	d := docOf(content)

	moves := []func(){
		d.MoveLeft, d.MoveRight, d.MoveUp, d.MoveDown,
		d.MoveWordLeft, d.MoveWordRight,
		d.MoveLineStart, d.MoveLineEnd, d.MoveDocStart, d.MoveDocEnd,
		func() { d.PageUp(2) }, func() { d.PageDown(2) },
	}

	for i := 0; i < 3000; i++ {
		moves[i%len(moves)]()

		line, col := d.Cursor().Line, d.Cursor().Col
		if line < 0 || line >= d.LineCount() {
			t.Fatalf("第 %d 步后行号 %d 越界（共 %d 行）", i, line, d.LineCount())
		}
		if col < 0 || col > d.LineRuneLen(line) {
			t.Fatalf("第 %d 步后列号 %d 越界（第 %d 行有 %d 字符）",
				i, col, line, d.LineRuneLen(line))
		}
		if got := string(d.Text()); got != content {
			t.Fatalf("移动改变了内容: %q", got)
		}
	}
}

// TestEveryCellReachable 每行每一列都能被移动到达，否则渲染光标会跑到非法位置。
func TestEveryCellReachable(t *testing.T) {
	content := "ab\n中文\n\nxyz\n"
	d := docOf(content)

	for line := 0; line < d.LineCount(); line++ {
		for col := 0; col <= d.LineRuneLen(line); col++ {
			// 先回到文档开头，再一路向右走到目标位置。
			d.MoveDocStart()
			for d.Cursor().Line < line {
				d.MoveDown()
			}
			for d.Cursor().Col < col {
				d.MoveRight()
			}
			if got := d.Cursor(); got.Line != line || got.Col != col {
				t.Fatalf("无法到达 {%d %d}，实际 %+v", line, col, got)
			}
		}
	}
	_ = fmt.Sprint(d.Cursor())
}
