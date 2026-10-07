// browser_test.go — 目录浏览浮层的单元测试。
// SPDX-License-Identifier: MIT

package browser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/29anan29/Go-Studio/internal/view"
)

// theme 返回中性主题，避免测试依赖具体配色。
func theme() view.Theme { return view.PlainTheme() }

// setupTree 造一棵目录树，返回根目录。
func setupTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"beta", "alpha", "zsub/deep"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range []string{"b.txt", "a.txt", ".hidden", "note.md"} {
		if err := os.WriteFile(filepath.Join(root, file), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// TestReloadSortsDirsBeforeFiles 目录排在文件之前，各自按名字排序，
// 「..」永远在最前。
func TestReloadSortsDirsBeforeFiles(t *testing.T) {
	root := setupTree(t)
	b := New(root)

	var names []string
	for _, entry := range b.Entries {
		names = append(names, entry.Name)
	}
	want := []string{"..", "alpha", "beta", "zsub", "a.txt", "b.txt", "note.md"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("列表顺序 = %v, want %v", names, want)
	}
}

// TestReloadSkipsHiddenEntries 隐藏文件默认不显示，否则 .git 会淹没真正要开的文件。
func TestReloadSkipsHiddenEntries(t *testing.T) {
	root := setupTree(t)
	b := New(root)
	for _, entry := range b.Entries {
		if strings.HasPrefix(entry.Name, ".") && entry.Name != ".." {
			t.Errorf("列表里出现了隐藏项 %q", entry.Name)
		}
	}
}

// TestReloadOnMissingDirectory 列不出来要有说明，列表为空，不能静默一片空白。
func TestReloadOnMissingDirectory(t *testing.T) {
	b := New(filepath.Join(t.TempDir(), "nope"))
	if b.Err == "" {
		t.Error("列目录失败时没有给出说明")
	}
	if len(b.Entries) != 0 {
		t.Errorf("失败时列出了 %d 项，want 0", len(b.Entries))
	}
}

func TestMoveClampsToRange(t *testing.T) {
	b := New(setupTree(t))
	last := len(b.Entries) - 1

	b.Move(-100)
	if b.Cursor != 0 {
		t.Errorf("往上越界后 cursor = %d, want 0", b.Cursor)
	}
	b.Move(1000)
	if b.Cursor != last {
		t.Errorf("往下越界后 cursor = %d, want %d", b.Cursor, last)
	}
}

// TestMoveOnEmptyList 空列表上移动不能越界 panic。
func TestMoveOnEmptyList(t *testing.T) {
	b := New(filepath.Join(t.TempDir(), "nope"))
	b.Move(5)
	b.Move(-5)
	b.MoveTo(3)
	if b.Cursor != 0 {
		t.Errorf("空列表上 cursor = %d, want 0", b.Cursor)
	}
}

// TestEnsureVisibleScrollsLongList 列表比可视区高时，选中项必须滚动进视野。
func TestEnsureVisibleScrollsLongList(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 50; i++ {
		name := string(rune('a'+i%26)) + string(rune('a'+i/26)) + ".txt"
		if err := os.WriteFile(filepath.Join(root, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	b := New(root)
	b.height = 5
	b.EnsureVisible()

	// 跳到最后一项，视口必须跟着滚。
	b.MoveTo(len(b.Entries) - 1)
	if b.Cursor < b.Offset || b.Cursor >= b.Offset+b.VisibleHeight() {
		t.Errorf("选中项 %d 不在可视范围 [%d, %d)",
			b.Cursor, b.Offset, b.Offset+b.VisibleHeight())
	}
	// 回到第一项。
	b.MoveTo(0)
	if b.Cursor < b.Offset {
		t.Errorf("选中项 %d 在视口 %d 之前", b.Cursor, b.Offset)
	}
}

func TestEnterAndGoUp(t *testing.T) {
	root := setupTree(t)
	b := New(root)

	// alpha 是第 1 项（".." 之后）。
	b.MoveTo(1)
	if entry, _ := b.Selected(); entry.Name != "alpha" {
		t.Fatalf("选中 = %q, want alpha", entry.Name)
	}
	b.Enter()
	if b.Dir != filepath.Join(root, "alpha") {
		t.Errorf("进入后 = %q, want %q", b.Dir, filepath.Join(root, "alpha"))
	}

	b.GoUp()
	if b.Dir != root {
		t.Errorf("返回后 = %q, want %q", b.Dir, root)
	}
}

// TestGoUpStopsAtRoot 已在根目录再往上不动，只给提示。
func TestGoUpStopsAtRoot(t *testing.T) {
	b := New("/")
	b.GoUp()
	if b.Dir != "/" {
		t.Errorf("根目录之上 = %q, want %q", b.Dir, "/")
	}
	if b.Err == "" {
		t.Error("在根目录继续返回应该给出提示")
	}
}

// TestEnterOnFileDoesNothing 对文件按「进入」没有意义，不该改变目录。
func TestEnterOnFileDoesNothing(t *testing.T) {
	root := setupTree(t)
	b := New(root)
	// a.txt 是文件。
	b.moveToName("a.txt")
	before := b.Dir
	b.Enter()
	if b.Dir != before {
		t.Errorf("对文件执行进入后目录变了: %q -> %q", before, b.Dir)
	}
}

// TestHandleKeyActions 每个键对应的动作必须正确，这是交互的核心契约。
func TestHandleKeyActions(t *testing.T) {
	root := setupTree(t)

	cases := []struct {
		name  string
		setup func(b *Browser)
		key   string
		want  Action
	}{
		{"Esc 关闭", nil, "<esc>", Close},
		{"左箭头返回上级", nil, "<left>", Up},
		{"在目录上右箭头进入", func(b *Browser) { b.MoveTo(1) }, "<right>", Enter},
		{"在目录上回车进入", func(b *Browser) { b.MoveTo(1) }, "<enter>", Enter},
		{"在文件上右箭头打开", func(b *Browser) { b.moveToName("a.txt") }, "<right>", Open},
		{"在文件上回车打开", func(b *Browser) { b.moveToName("a.txt") }, "<enter>", Open},
		{"上移", nil, "<up>", None},
		{"下移", nil, "<down>", None},
		{"翻页", nil, "<pgdown>", None},
		{"Home", nil, "<home>", None},
		{"End", nil, "<end>", None},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := New(root)
			if tc.setup != nil {
				tc.setup(b)
			}
			if got := b.HandleKey(tc.key); got != tc.want {
				t.Errorf("HandleKey(%q) = %v, want %v", tc.key, got, tc.want)
			}
		})
	}
}

// moveToName 把选择移到指定名字的项。
func (b *Browser) moveToName(name string) {
	for i, entry := range b.Entries {
		if entry.Name == name {
			b.MoveTo(i)
			return
		}
	}
}

// TestHandleKeyNavigationMoves 导航键要真的移动选择。
func TestHandleKeyNavigationMoves(t *testing.T) {
	b := New(setupTree(t))
	b.HandleKey("<home>")
	if b.Cursor != 0 {
		t.Fatalf("Home 之后 cursor = %d, want 0", b.Cursor)
	}
	b.HandleKey("<down>")
	if b.Cursor != 1 {
		t.Errorf("Down 之后 cursor = %d, want 1", b.Cursor)
	}
	b.HandleKey("<up>")
	if b.Cursor != 0 {
		t.Errorf("Up 之后 cursor = %d, want 0", b.Cursor)
	}
	b.HandleKey("<end>")
	if b.Cursor != len(b.Entries)-1 {
		t.Errorf("End 之后 cursor = %d, want %d", b.Cursor, len(b.Entries)-1)
	}
}

// TestCurrentPath 返回选中项的完整路径。
func TestCurrentPath(t *testing.T) {
	root := setupTree(t)
	b := New(root)
	b.MoveTo(0) // ".."
	if got, want := b.CurrentPath(), filepath.Dir(root); got != want {
		t.Errorf("CurrentPath = %q, want %q", got, want)
	}
	b.moveToName("a.txt")
	if got, want := b.CurrentPath(), filepath.Join(root, "a.txt"); got != want {
		t.Errorf("CurrentPath = %q, want %q", got, want)
	}
}

// TestRenderKeepsExactHeightAndWidth 渲染结果的行数与每行宽度都必须精确：
// 少一列露残影，多一列终端折行，整个布局就崩了。
func TestRenderKeepsExactHeightAndWidth(t *testing.T) {
	root := setupTree(t)

	for _, height := range []int{1, 2, 3, 5, 10, 24} {
		for _, width := range []int{1, 5, 20, 80, 200} {
			b := New(root)
			content, _ := b.Render(theme(), width, height)
			lines := strings.Split(content, "\n")
			if len(lines) != height {
				t.Fatalf("高度 %d 时渲染 %d 行, want %d", height, len(lines), height)
			}
			for i, line := range lines {
				if got := view.VisibleWidth(line); got != width {
					t.Errorf("高度 %d、宽度 %d 时第 %d 行宽 = %d, want %d",
						height, width, i, got, width)
				}
			}
		}
	}
}

// TestRenderCursorStaysInBounds 光标不能落在浮层之外。
func TestRenderCursorStaysInBounds(t *testing.T) {
	root := setupTree(t)
	for _, size := range []struct{ w, h int }{{80, 24}, {20, 6}, {5, 3}, {4, 2}} {
		b := New(root)
		_, cursor := b.Render(theme(), size.w, size.h)
		if cursor == nil {
			t.Errorf("尺寸 %dx%d 下没有光标", size.w, size.h)
			continue
		}
		if cursor.Y < 0 || cursor.Y >= size.h {
			t.Errorf("尺寸 %dx%d 下光标 Y = %d 越界", size.w, size.h, cursor.Y)
		}
		if cursor.X < 0 || cursor.X >= size.w {
			t.Errorf("尺寸 %dx%d 下光标 X = %d 越界", size.w, size.h, cursor.X)
		}
	}
}

// TestRenderWithErrorShowsMessage 列目录失败时要把错误显示出来。
func TestRenderWithErrorShowsMessage(t *testing.T) {
	b := New(filepath.Join(t.TempDir(), "nope"))
	content, _ := b.Render(theme(), 80, 10)
	if !strings.Contains(content, "读取目录失败") {
		t.Errorf("渲染内容里没有错误说明: %q", content)
	}
	// 出错时不该画光标：列表是空的，没有可选项。
	if _, cursor := b.Render(theme(), 80, 10); cursor != nil {
		t.Error("列表为空时不该画光标")
	}
}

// TestRenderMarksDirectories 目录名带结尾斜线，文件不带。
func TestRenderMarksDirectories(t *testing.T) {
	root := setupTree(t)
	b := New(root)
	content, _ := b.Render(theme(), 80, 12)

	if !strings.Contains(content, "alpha/") {
		t.Errorf("目录名没有加结尾斜线: %q", content)
	}
	if !strings.Contains(content, "a.txt") {
		t.Errorf("文件列表里没有 a.txt: %q", content)
	}
	// 隐藏文件不该出现。
	if strings.Contains(content, ".hidden") {
		t.Errorf("渲染里出现了隐藏文件: %q", content)
	}
}

// TestRenderUsesOnlyUnambiguousWidths 界面装饰必须用宽度确定的字符。
// 「东亚歧义宽度」字符在不同终端上占 1 列或 2 列，判断错一列就会撑破帧宽。
func TestRenderUsesOnlyUnambiguousWidths(t *testing.T) {
	root := setupTree(t)
	b := New(root)
	content, _ := b.Render(theme(), 80, 12)

	for _, forbidden := range []string{"│", "▸", "↑", "↓", "►"} {
		if strings.Contains(content, forbidden) {
			t.Errorf("渲染里出现了歧义宽度字符 %q", forbidden)
		}
	}
}

func TestHintMentionsEveryKey(t *testing.T) {
	hint := New("/").Hint()
	for _, key := range []string{"up", "down", "right", "left", "esc"} {
		if !strings.Contains(hint, key) {
			t.Errorf("提示里没有提到 %q: %q", key, hint)
		}
	}
}
