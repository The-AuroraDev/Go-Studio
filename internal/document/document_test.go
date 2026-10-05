// document_test.go — 文档的构造、文件读写与脏标记。
// SPDX-License-Identifier: MIT

package document

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewIsClean(t *testing.T) {
	d := New()
	if d.Dirty() {
		t.Error("新建文档不应有未保存改动")
	}
	if d.Path() != "" {
		t.Errorf("Path() = %q, want 空", d.Path())
	}
	if d.Readonly() {
		t.Error("新建文档不应只读")
	}
	if d.LineCount() != 1 {
		t.Errorf("LineCount() = %d, want 1（空文档也算一行）", d.LineCount())
	}
	if got := d.Cursor(); got.Line != 0 || got.Col != 0 {
		t.Errorf("Cursor() = %+v, want {0 0}", got)
	}
	if d.CanUndo() {
		t.Error("新建文档不应能撤销")
	}
	if d.CanRedo() {
		t.Error("新建文档不应能重做")
	}
}

func TestOpenReadsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.go")
	// 文件以换行结尾，按片段表算法算出的行数是换行数+1=4。
	// 这正是缓冲层的行为，不是 bug。
	if err := os.WriteFile(path, []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := Open(path)
	if err != nil {
		t.Fatalf("Open() 返回错误: %v", err)
	}
	if d.Path() != path {
		t.Errorf("Path() = %q, want %q", d.Path(), path)
	}
	if d.Dirty() {
		t.Error("打开后不应有未保存改动")
	}
	if d.LineCount() != 4 {
		t.Errorf("LineCount() = %d, want 4（文件以换行结尾）", d.LineCount())
	}
	if got := string(d.LineText(0)); got != "package main" {
		t.Errorf("LineText(0) = %q, want %q", got, "package main")
	}
}

func TestOpenRejectsMissingFile(t *testing.T) {
	if _, err := Open(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("Open() 对不存在的文件未报错")
	}
}

func TestOpenRejectsDirectory(t *testing.T) {
	if _, err := Open(t.TempDir()); err == nil {
		t.Fatal("Open() 对目录未报错")
	}
}

func TestOpenRejectsBinary(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bin")
	if err := os.WriteFile(path, []byte{0x7f, 'E', 'L', 'F', 0, 1, 2, 3}, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path); err == nil {
		t.Fatal("Open() 对二进制文件未报错")
	}
}

func TestIsBinary(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want bool
	}{
		{"空文件", nil, false},
		{"纯文本", []byte("hello\nworld\n"), false},
		{"中文", []byte("你好，世界\n"), false},
		{"含NUL", []byte("a\x00b"), true},
		{"NUL在8KB之后", append(bytesOf('x', 9000), 0), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isBinary(tc.data); got != tc.want {
				t.Errorf("isBinary() = %t, want %t", got, tc.want)
			}
		})
	}
}

// bytesOf 造 n 个相同字节的切片。
func bytesOf(b byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = b
	}
	return out
}

func TestSaveAsIsAtomicAndKeepsMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	// 把光标移到文末，删掉旧内容，再插入新内容。
	d.SetCursor(0, d.LineRuneLen(0))
	for d.Cursor().Col > 0 {
		d.Backspace()
	}
	if !d.InsertText("new") {
		t.Fatal("InsertText 失败")
	}
	if err := d.Save(); err != nil {
		t.Fatalf("Save() 返回错误: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Errorf("磁盘内容 = %q, want %q", got, "new")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("权限 = %o, want 600（保存不能改权限）", info.Mode().Perm())
	}
	if d.Dirty() {
		t.Error("保存后不应再有未保存改动")
	}

	// 原子保存：目录下除了目标文件不该留下临时文件。
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() != "a.txt" {
			t.Errorf("目录下残留临时文件 %q", entry.Name())
		}
	}
}

func TestSaveWithoutPathFails(t *testing.T) {
	if err := New().Save(); err == nil {
		t.Fatal("Save() 对未命名文档未报错")
	}
}

// TestDirtyIsExact 固化脏标记的精确语义：
// 撤销回到保存点必须重新变干净——这是用「编辑次数」判断不出来的。
func TestDirtyIsExact(t *testing.T) {
	d := New()
	if !d.InsertText("hello") {
		t.Fatal("InsertText 失败")
	}
	if !d.Dirty() {
		t.Fatal("编辑后应有未保存改动")
	}
	d.savedRev = d.rev // 相当于在当前状态保存

	if !d.InsertText(" world") {
		t.Fatal("InsertText 失败")
	}
	if !d.Dirty() {
		t.Fatal("再次编辑后应有未保存改动")
	}

	// 撤销一次，回到保存点：必须重新变干净。
	if !d.Undo() {
		t.Fatal("Undo 失败")
	}
	if d.Dirty() {
		t.Error("撤销回到保存点后不应再有未保存改动")
	}
	if string(d.Text()) != "hello" {
		t.Errorf("撤销后内容 = %q, want %q", d.Text(), "hello")
	}

	// 重做：再次变脏。
	if !d.Redo() {
		t.Fatal("Redo 失败")
	}
	if !d.Dirty() {
		t.Error("重做后应有未保存改动")
	}
}

// TestDirtyStaysDirtyAfterEditUndoEdit 曾是旧实现的 bug：
// 编辑后撤销、再编辑一次，版本号会撞上保存点的版本号，
// 于是内容明明不同却显示「已保存」。
func TestDirtyStaysDirtyAfterEditUndoEdit(t *testing.T) {
	d := New()
	d.InsertText("a")
	d.savedRev = d.rev

	d.InsertText("b")
	d.Undo()          // 回到 "a"
	d.InsertText("c") // 现在是 "ac"

	if !d.Dirty() {
		t.Error("内容已偏离保存点，不应显示干净")
	}
	if got := string(d.Text()); got != "ac" {
		t.Errorf("内容 = %q, want %q", got, "ac")
	}
}

func TestSaveAsChangesPath(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	d := New()
	d.InsertText("x")
	if err := d.SaveAs(a); err != nil {
		t.Fatalf("SaveAs(a) 返回错误: %v", err)
	}
	if d.Path() != a {
		t.Fatalf("Path() = %q, want %q", d.Path(), a)
	}
	if d.Dirty() {
		t.Fatal("保存后不应脏")
	}
	d.InsertText("y")
	if err := d.SaveAs(b); err != nil {
		t.Fatalf("SaveAs(b) 返回错误: %v", err)
	}
	if d.Path() != b {
		t.Errorf("Path() = %q, want %q", d.Path(), b)
	}
	got, err := os.ReadFile(b)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "xy" {
		t.Errorf("b 内容 = %q, want %q", got, "xy")
	}
}

func TestReadonly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ro.txt")
	if err := os.WriteFile(path, []byte("x"), 0o444); err != nil {
		t.Fatal(err)
	}
	d, err := Open(path)
	if err != nil {
		t.Fatalf("Open() 返回错误: %v", err)
	}
	if !d.Readonly() {
		t.Error("只读文件应标记为只读")
	}
	if d.InsertText("y") {
		t.Error("只读文档不应能编辑")
	}
	if err := d.Save(); err == nil {
		t.Error("只读文档不应能保存")
	}
}

func TestSetCursorClamps(t *testing.T) {
	d := New()
	d.InsertText("ab\ncd")
	d.SetCursor(99, 99)
	if got := d.Cursor(); got.Line != 1 || got.Col != 2 {
		t.Errorf("Cursor() = %+v, want {1 2}", got)
	}
	d.SetCursor(-1, -1)
	if got := d.Cursor(); got.Line != 0 || got.Col != 0 {
		t.Errorf("Cursor() = %+v, want {0 0}", got)
	}
}

func TestMustOffsetMatchesCursor(t *testing.T) {
	d := New()
	d.InsertText("ab\n中d")
	for _, line := range []int{0, 1} {
		for col := 0; col <= d.LineRuneLen(line); col++ {
			d.SetCursor(line, col)
			offset := d.MustOffset()
			gotLine, gotCol := d.Buffer().LineCol(offset)
			if gotLine != line || gotCol != col {
				t.Errorf("光标 {%d %d} 的偏移 %d 对应 {%d %d}",
					line, col, offset, gotLine, gotCol)
			}
		}
	}
}
