// edit_test.go — 文本编辑的单元测试。
// SPDX-License-Identifier: MIT

package document

import (
	"strings"
	"testing"
)

// content 返回文档全文，便于断言。
func content(d *Document) string { return string(d.Text()) }

// atEnd 把光标移到文档末尾。
func atEnd(d *Document) {
	d.MoveDocEnd()
}

func TestInsertRune(t *testing.T) {
	d := docOf("")
	for _, r := range "abc" {
		if !d.InsertRune(r) {
			t.Fatalf("InsertRune(%q) 返回 false", r)
		}
	}
	if got := content(d); got != "abc" {
		t.Errorf("内容 = %q, want %q", got, "abc")
	}
	assertCursor(t, d, 0, 3)
}

func TestInsertRuneWithNewline(t *testing.T) {
	d := docOf("")
	d.InsertRune('a')
	d.InsertRune('\n')
	d.InsertRune('b')
	if got := content(d); got != "a\nb" {
		t.Errorf("内容 = %q, want %q", got, "a\nb")
	}
	// 换行后光标在 (1,0)，再插入 b 后落在 b 之后即 (1,1)。
	assertCursor(t, d, 1, 1)
}

// TestInsertRuneWithCRLF CRLF 里的 \r 不能被当成可打印字符插进文档。
func TestInsertRuneWithCRLF(t *testing.T) {
	d := docOf("a\r\nb")
	d.SetCursor(1, 0)
	d.InsertRune('X')
	if got := content(d); got != "a\r\nXb" {
		t.Errorf("内容 = %q, want %q", got, "a\r\nXb")
	}
}

func TestInsertText(t *testing.T) {
	d := docOf("")
	if !d.InsertText("hello world") {
		t.Fatal("InsertText 返回 false")
	}
	if got := content(d); got != "hello world" {
		t.Errorf("内容 = %q, want %q", got, "hello world")
	}
	assertCursor(t, d, 0, 11)
}

// TestInsertTextAtCursor 插入必须落在光标处而不是文末。
func TestInsertTextAtCursor(t *testing.T) {
	d := docOf("ac")
	d.SetCursor(0, 1)
	d.InsertText("b")
	if got := content(d); got != "abc" {
		t.Errorf("内容 = %q, want %q", got, "abc")
	}
	assertCursor(t, d, 0, 2)
}

func TestInsertTextMultiLine(t *testing.T) {
	d := docOf("")
	d.InsertText("a\nb\nc")
	assertCursor(t, d, 2, 1)
	if got := content(d); got != "a\nb\nc" {
		t.Errorf("内容 = %q, want %q", got, "a\nb\nc")
	}
}

func TestInsertTextEmptyIsNoop(t *testing.T) {
	d := docOf("abc")
	if d.InsertText("") {
		t.Error("插入空串不应算作一次编辑")
	}
	if d.Dirty() {
		t.Error("插入空串不应弄脏文档")
	}
}

// TestNewlineAutoIndents 换行要复制当前行行首空白，这是基础自动缩进。
func TestNewlineAutoIndents(t *testing.T) {
	d := docOf("\t\tx := 1")
	d.SetCursor(0, 8)
	if !d.Newline() {
		t.Fatal("Newline 返回 false")
	}
	if got := content(d); got != "\t\tx := 1\n\t\t" {
		t.Errorf("内容 = %q, want %q", got, "\t\tx := 1\n\t\t")
	}
	assertCursor(t, d, 1, 2)
}

func TestNewlineWithoutIndent(t *testing.T) {
	d := docOf("ab")
	d.SetCursor(0, 2)
	d.Newline()
	if got := content(d); got != "ab\n" {
		t.Errorf("内容 = %q, want %q", got, "ab\n")
	}
	assertCursor(t, d, 1, 0)
}

// TestBackspaceDeletesPreviousRune 退格必须按 rune 退，不能切碎多字节字符。
func TestBackspaceDeletesPreviousRune(t *testing.T) {
	d := docOf("a中b")
	d.SetCursor(0, 2)
	if !d.Backspace() {
		t.Fatal("Backspace 返回 false")
	}
	if got := content(d); got != "ab" {
		t.Errorf("内容 = %q, want %q", got, "ab")
	}
	assertCursor(t, d, 0, 1)
}

func TestBackspaceAtStartOfDocument(t *testing.T) {
	d := docOf("abc")
	d.SetCursor(0, 0)
	if d.Backspace() {
		t.Error("文档开头的退格不应算作一次编辑")
	}
	if d.Dirty() {
		t.Error("文档开头的退格不应弄脏文档")
	}
}

// TestBackspaceJoinsLines 行首退格要把上一行接上来。
func TestBackspaceJoinsLines(t *testing.T) {
	d := docOf("ab\ncd")
	d.SetCursor(1, 0)
	if !d.Backspace() {
		t.Fatal("Backspace 返回 false")
	}
	if got := content(d); got != "abcd" {
		t.Errorf("内容 = %q, want %q", got, "abcd")
	}
	assertCursor(t, d, 0, 2)
}

func TestDeleteRight(t *testing.T) {
	d := docOf("a中b")
	d.SetCursor(0, 1)
	if !d.DeleteRight() {
		t.Fatal("DeleteRight 返回 false")
	}
	if got := content(d); got != "ab" {
		t.Errorf("内容 = %q, want %q", got, "ab")
	}
	// 删除右侧字符不该移动光标。
	assertCursor(t, d, 0, 1)
}

// TestDeleteRightJoinsLines 行尾 Delete 要把下一行接上来。
func TestDeleteRightJoinsLines(t *testing.T) {
	d := docOf("ab\ncd")
	d.SetCursor(0, 2)
	if !d.DeleteRight() {
		t.Fatal("DeleteRight 返回 false")
	}
	if got := content(d); got != "abcd" {
		t.Errorf("内容 = %q, want %q", got, "abcd")
	}
	assertCursor(t, d, 0, 2)
}

func TestDeleteRightAtEndOfDocument(t *testing.T) {
	d := docOf("abc")
	d.MoveDocEnd()
	if d.DeleteRight() {
		t.Error("文档末尾的 DeleteRight 不应算作一次编辑")
	}
}

func TestDeleteLine(t *testing.T) {
	d := docOf("first\nsecond\nthird")
	d.SetCursor(1, 3)
	if !d.DeleteLine() {
		t.Fatal("DeleteLine 返回 false")
	}
	if got := content(d); got != "first\nthird" {
		t.Errorf("内容 = %q, want %q", got, "first\nthird")
	}
	assertCursor(t, d, 1, 0)
}

// TestDeleteLastLineWithoutNewline 删最后一行（无换行符）时不能留下空行。
func TestDeleteLastLineWithoutNewline(t *testing.T) {
	d := docOf("first\nsecond")
	d.GotoLine(1)
	if !d.DeleteLine() {
		t.Fatal("DeleteLine 返回 false")
	}
	if got := content(d); got != "first" {
		t.Errorf("内容 = %q, want %q", got, "first")
	}
	assertCursor(t, d, 0, 0)
}

func TestDeleteFirstLine(t *testing.T) {
	d := docOf("first\nsecond")
	d.GotoLine(0)
	d.DeleteLine()
	if got := content(d); got != "second" {
		t.Errorf("内容 = %q, want %q", got, "second")
	}
	assertCursor(t, d, 0, 0)
}

func TestJoinLine(t *testing.T) {
	d := docOf("ab\n    cd")
	d.SetCursor(0, 2)
	if !d.JoinLine() {
		t.Fatal("JoinLine 返回 false")
	}
	// 连接时要去掉下一行的行首缩进。
	if got := content(d); got != "abcd" {
		t.Errorf("内容 = %q, want %q", got, "abcd")
	}
	assertCursor(t, d, 0, 2)
}

func TestJoinLineAtLastLine(t *testing.T) {
	d := docOf("only")
	d.MoveDocEnd()
	if d.JoinLine() {
		t.Error("最后一行不该能连接")
	}
}

func TestInsertTab(t *testing.T) {
	d := docOf("")
	d.InsertTab(4)
	assertCursor(t, d, 0, 4)
	d.InsertTab(4)
	assertCursor(t, d, 0, 8)
	d.InsertTab(4)
	assertCursor(t, d, 0, 12) // 补到下一个制表位
	if got := content(d); got != strings.Repeat(" ", 12) {
		t.Errorf("内容 = %q, want 12 个空格", got)
	}
}

// TestInsertTabWithZeroWidthUsesTabCharacter tabWidth 为 0 时用真正的制表符。
func TestInsertTabWithZeroWidthUsesTabCharacter(t *testing.T) {
	d := docOf("")
	d.InsertTab(0)
	if got := content(d); got != "\t" {
		t.Errorf("内容 = %q, want 制表符", got)
	}
	assertCursor(t, d, 0, 1)
}

func TestIndentUnindent(t *testing.T) {
	d := docOf("abc")
	d.SetCursor(0, 3)

	if !d.Indent(4) {
		t.Fatal("Indent 返回 false")
	}
	if got := content(d); got != "    abc" {
		t.Errorf("缩进后 = %q, want %q", got, "    abc")
	}
	assertCursor(t, d, 0, 7) // 整行右移，光标跟着走

	if !d.Unindent(4) {
		t.Fatal("Unindent 返回 false")
	}
	if got := content(d); got != "abc" {
		t.Errorf("取消缩进后 = %q, want %q", got, "abc")
	}
	assertCursor(t, d, 0, 3)
}

// TestUnindentRemovesSingleTab 行首是制表符时只去掉一个制表符。
func TestUnindentRemovesSingleTab(t *testing.T) {
	d := docOf("\t\tx := 1")
	d.GotoLine(0)
	d.Unindent(4)
	if got := content(d); got != "\tx := 1" {
		t.Errorf("内容 = %q, want %q", got, "\tx := 1")
	}
}

// TestUnindentWithoutIndent 没有缩进可去时不应算作一次编辑。
func TestUnindentWithoutIndent(t *testing.T) {
	d := docOf("abc")
	d.GotoLine(0)
	if d.Unindent(4) {
		t.Error("没有缩进时 Unindent 不应算作一次编辑")
	}
}

// TestUnindentStopsAtCursor 左移不能越过行首。
func TestUnindentStopsAtCursor(t *testing.T) {
	d := docOf("    abc")
	d.SetCursor(0, 0)
	d.Unindent(8)
	if got := d.Cursor().Col; got != 0 {
		t.Errorf("光标列 = %d, want 0（不能为负）", got)
	}
}

// ---- 撤销与重做 ----

func TestUndoInsert(t *testing.T) {
	d := newTestDoc("")
	d.InsertText("hello")
	if !d.CanUndo() {
		t.Fatal("应能撤销")
	}
	if !d.Undo() {
		t.Fatal("Undo 返回 false")
	}
	if got := content(d); got != "" {
		t.Errorf("撤销后 = %q, want 空", got)
	}
	if d.CanUndo() {
		t.Error("空栈不应还能撤销")
	}
}

func TestRedoInsert(t *testing.T) {
	d := newTestDoc("")
	d.InsertText("hello")
	d.Undo()
	if !d.CanRedo() {
		t.Fatal("应能重做")
	}
	if !d.Redo() {
		t.Fatal("Redo 返回 false")
	}
	if got := content(d); got != "hello" {
		t.Errorf("重做后 = %q, want %q", got, "hello")
	}
	if d.CanRedo() {
		t.Error("重做栈用完后不应还能重做")
	}
}

func TestUndoOnEmptyHistory(t *testing.T) {
	d := newTestDoc("abc")
	if d.Undo() {
		t.Error("无编辑时 Undo 应返回 false")
	}
	if d.Redo() {
		t.Error("无编辑时 Redo 应返回 false")
	}
}

// TestUndoRestoresCursorToChangePoint 撤销后光标必须落在该次改动的位置。
func TestUndoRestoresCursorToChangePoint(t *testing.T) {
	d := newTestDoc("abc\ndef")
	d.SetCursor(1, 1)
	d.InsertRune('X')
	assertCursor(t, d, 1, 2)

	d.Undo()
	// 插入发生在第 1 行第 1 列，光标应回到那里。
	assertCursor(t, d, 1, 1)
}

// TestRedoAfterUndoClearsOnEdit 编辑会让重做栈作废。
func TestRedoAfterUndoClearsOnEdit(t *testing.T) {
	d := newTestDoc("")
	d.InsertRune('a')
	d.InsertRune('b')
	d.Undo() // 回到 "a"
	if !d.CanRedo() {
		t.Fatal("应能重做")
	}
	d.InsertRune('c') // 新编辑，作废重做
	if d.CanRedo() {
		t.Error("新编辑后重做栈必须作废")
	}
	if got := content(d); got != "ac" {
		t.Errorf("内容 = %q, want %q", got, "ac")
	}
}

// TestUndoAllRestoresOriginal 把一串编辑全部撤销，内容必须回到最初。
func TestUndoAllRestoresOriginal(t *testing.T) {
	original := "package main\n\nfunc main() {\n}\n"
	d := newTestDoc(original)

	d.SetCursor(1, 0)
	d.InsertText("// hi\n")
	d.SetCursor(0, 0)
	d.DeleteRight() // 删掉 p
	d.SetCursor(3, 1)
	d.Backspace()
	d.Newline()
	d.Indent(4)

	for d.CanUndo() {
		if !d.Undo() {
			t.Fatal("撤销中途失败")
		}
	}
	if got := content(d); got != original {
		t.Errorf("全部撤销后 = %q, want %q", got, original)
	}
	if d.Dirty() {
		t.Error("从未保存的文档撤销回原状后不应算作有改动")
	}
}

// TestRedoAllReapplies 全部重做必须回到最后一次编辑后的状态。
func TestRedoAllReapplies(t *testing.T) {
	d := newTestDoc("")
	d.InsertText("a")
	d.InsertText("b")
	d.InsertText("c")
	d.Undo()
	d.Undo()
	d.Undo()
	if got := content(d); got != "" {
		t.Fatalf("撤销三次后 = %q, want 空", got)
	}
	for i := 0; i < 3; i++ {
		if !d.Redo() {
			t.Fatalf("第 %d 次重做失败", i+1)
		}
	}
	if got := content(d); got != "abc" {
		t.Errorf("全部重做后 = %q, want %q", got, "abc")
	}
}

// TestUndoBackspaceRestoresText 连续退格后一次撤销要恢复整段文本。
func TestUndoBackspaceRestoresText(t *testing.T) {
	// 用生产配置（开着合并）验证连续退格合并成一步。
	d := New()
	d.InsertText("hello")
	d.MoveDocEnd()
	d.Backspace()
	d.Backspace()
	d.Backspace()
	if got := content(d); got != "he" {
		t.Fatalf("退格三次后 = %q, want %q", got, "he")
	}
	d.Undo()
	if got := content(d); got != "hello" {
		t.Errorf("撤销后 = %q, want %q", got, "hello")
	}
}

// ---- 只读 ----

func TestReadonlyRejectsEveryEdit(t *testing.T) {
	d := newDocument("", []byte("abc"), true, 0)
	edits := map[string]func() bool{
		"InsertRune":  func() bool { return d.InsertRune('x') },
		"InsertText":  func() bool { return d.InsertText("x") },
		"Newline":     d.Newline,
		"InsertTab":   func() bool { return d.InsertTab(4) },
		"Backspace":   d.Backspace,
		"DeleteRight": d.DeleteRight,
		"DeleteLine":  d.DeleteLine,
		"JoinLine":    d.JoinLine,
		"Indent":      func() bool { return d.Indent(4) },
		"Unindent":    func() bool { return d.Unindent(4) },
	}
	for name, edit := range edits {
		if edit() {
			t.Errorf("只读文档不该能执行 %s", name)
		}
	}
	if got := content(d); got != "abc" {
		t.Errorf("只读文档内容被改了: %q", got)
	}
	if d.Dirty() {
		t.Error("只读文档不该被弄脏")
	}
}

// ---- 编辑不变量 ----

// TestRandomEditingKeepsCursorValid 随机编辑的不变量：
// 光标始终合法，内容与光标自洽，且脏标记与内容变化同步。
func TestRandomEditingKeepsCursorValid(t *testing.T) {
	original := "alpha beta\ngamma\n\n中文字符\n  indented line\nlast"
	d := newTestDoc(original)

	edits := []func(){
		func() { d.InsertRune('q') },
		func() { d.Newline() },
		func() { d.Backspace() },
		func() { d.DeleteRight() },
		func() { d.InsertTab(4) },
		func() { d.MoveLeft() },
		func() { d.MoveRight() },
		func() { d.MoveDown() },
		func() { d.MoveUp() },
		func() { d.MoveDocEnd() },
		func() { d.MoveDocStart() },
	}

	for i := 0; i < 5000; i++ {
		edits[i%len(edits)]()

		line, col := d.Cursor().Line, d.Cursor().Col
		if line < 0 || line >= d.LineCount() {
			t.Fatalf("第 %d 步后行号 %d 越界（共 %d 行）", i, line, d.LineCount())
		}
		if col < 0 || col > d.LineRuneLen(line) {
			t.Fatalf("第 %d 步后列号 %d 越界（第 %d 行有 %d 字符）",
				i, col, line, d.LineRuneLen(line))
		}
	}

	// 全部撤销后必须精确回到原文。
	for d.CanUndo() {
		if !d.Undo() {
			t.Fatal("撤销中途失败")
		}
	}
	if got := content(d); got != original {
		t.Errorf("全部撤销后内容与原文不符:\n got %q\nwant %q", got, original)
	}
	if d.Dirty() {
		t.Error("撤销回原文后不应有未保存改动")
	}
}

// TestUndoRedoIsIdentity 反复撤销重做必须回到同样的内容。
func TestUndoRedoIsIdentity(t *testing.T) {
	d := newTestDoc("one\ntwo\nthree")
	d.SetCursor(1, 1)
	d.InsertRune('X')
	d.MoveDown()
	d.DeleteRight()
	d.Newline()
	d.Indent(2)

	want := content(d)

	for i := 0; i < 10; i++ {
		for d.CanUndo() {
			d.Undo()
		}
		if got := content(d); got != "one\ntwo\nthree" {
			t.Fatalf("第 %d 轮撤销到底 = %q, want 原文", i, got)
		}
		for d.CanRedo() {
			d.Redo()
		}
		if got := content(d); got != want {
			t.Fatalf("第 %d 轮重做到底 = %q, want %q", i, got, want)
		}
	}
}
