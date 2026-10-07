// command_test.go — 命令分派的单元测试：接线完整性与逐键行为。
// SPDX-License-Identifier: MIT

package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/29anan29/Go-Studio/internal/config"
	"github.com/29anan29/Go-Studio/internal/document"
	"github.com/29anan29/Go-Studio/internal/keymap"
	"github.com/29anan29/Go-Studio/internal/screen"
	"github.com/29anan29/Go-Studio/internal/view"
)

// TestEveryKeymapCommandHasHandler 是接线完整性的守门测试：
// 键位表里有、而分派表里没有的命令，按下去只会得到一句「命令未实现」。
// 这类遗漏必须在测试期暴露，不能留给用户在终端里发现。
func TestEveryKeymapCommandHasHandler(t *testing.T) {
	table, err := keymap.Lookup(keymap.EmacsName)
	if err != nil {
		t.Fatal(err)
	}
	for _, binding := range table.Bindings() {
		if _, ok := commands[binding.Cmd]; !ok {
			t.Errorf("按键 %q 绑定的命令 %q 没有注册处理函数", binding.Keys, binding.Cmd)
		}
	}
}

// TestRunCommandReportsUnknownBinding 分派表里没有的命令必须明确报出来，
// 静默返回会被当成按键坏了。
func TestRunCommandReportsUnknownBinding(t *testing.T) {
	h := newHarness(t, "")
	h.app.runCommand(keymap.Binding{Keys: "ctrl+q", Cmd: keymap.Command("不存在的命令")})

	if !strings.Contains(h.app.currentStatus(), "命令未实现") {
		t.Errorf("状态栏 = %q, want 含「命令未实现」", h.app.currentStatus())
	}
}

// ---- 导航键 ----

// TestNavigationKeys 逐个验证绑定在方向键与 Home/End/PgUp/PgDn 上的命令。
func TestNavigationKeys(t *testing.T) {
	// 内容故意长短不一，才能看出期望列是否被正确保留。
	content := "0123456789\nab\n0123456789\nabc\n0123456789"

	cases := []struct {
		name string
		keys []string
		line int
		col  int
	}{
		{"右移", []string{"<right>", "<right>"}, 0, 2},
		{"左移", []string{"<left>"}, 0, 0},
		{"下移", []string{"<down>"}, 1, 0},
		{"上移", []string{"<up>"}, 0, 0},
		{"跳到行尾", []string{"<end>"}, 0, 10},
		{"跳到行首", []string{"<home>"}, 0, 0},
		{"跳到文末", []string{"ctrl+<end>"}, 4, 10},
		{"跳到文首", []string{"ctrl+<home>"}, 0, 0},
		// 「0123456789」整行都是一个词，从第 2 列按词右移会走出这一行，
		// 落到下一行行首——语义是「下一个词的词首」，不是「当前词末尾」。
		{"按词右移", []string{"<right>", "<right>", "ctrl+<right>"}, 1, 0},
		{"按词左移", []string{"ctrl+<end>", "ctrl+<left>"}, 4, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, content)
			h.app.draw()
			h.pressAll(tc.keys...)

			got := h.doc().Cursor()
			if got.Line != tc.line || got.Col != tc.col {
				t.Errorf("光标 = %+v, want {%d %d}", got, tc.line, tc.col)
			}
		})
	}
}

// ---- 编辑键 ----

func TestEditingKeys(t *testing.T) {
	cases := []struct {
		name    string
		content string
		keys    []string
		want    string
	}{
		{"回车换行", "", []string{"<enter>"}, "\n"},
		{"制表符展开", "", []string{"<tab>"}, "    "},
		{"退格", "abc", []string{"ctrl+<end>", "<backspace>"}, "ab"},
		{"右删", "abc", []string{"<delete>"}, "bc"},
		{"删整行", "one\ntwo\n", []string{"<down>", "ctrl+<end>", "<backspace>"}, "one\ntwo"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, tc.content)
			h.app.draw()
			h.pressAll(tc.keys...)

			if got := string(h.doc().Text()); got != tc.want {
				t.Errorf("内容 = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestPageKeys 翻页要在足够长的文档上验证，否则翻不动。
func TestPageKeys(t *testing.T) {
	lines := make([]string, 200)
	for i := range lines {
		lines[i] = "0123456789"
	}
	content := strings.Join(lines, "\n")

	h := newHarness(t, content)
	h.app.draw()

	page := h.app.textHeight()
	h.press("<pgdown>")
	if got := h.doc().Cursor().Line; got != page {
		t.Errorf("下翻页后行 = %d, want %d（一屏的高度）", got, page)
	}
	h.press("<pgup>")
	if got := h.doc().Cursor().Line; got != 0 {
		t.Errorf("上翻页后行 = %d, want 0", got)
	}

	// 到底之后继续翻页不能越界。
	h.pressAll("ctrl+<end>")
	h.press("<pgdown>")
	if got := h.doc().Cursor().Line; got != 199 {
		t.Errorf("末行再翻页后行 = %d, want 199（停在末行）", got)
	}
	h.pressAll("ctrl+<home>")
	h.press("<pgup>")
	if got := h.doc().Cursor().Line; got != 0 {
		t.Errorf("首行再上翻后行 = %d, want 0", got)
	}
}

// TestNewlineKeepsIndent 回车要保留行首缩进。
func TestNewlineKeepsIndent(t *testing.T) {
	h := newHarness(t, "\tabc")
	h.app.draw()
	h.press("ctrl+<end>")
	h.press("<enter>")

	if got := string(h.doc().Text()); got != "\tabc\n\t" {
		t.Errorf("内容 = %q, want %q", got, "\tabc\n\t")
	}
}

// TestTabWidthFollowsConfig 制表符宽度必须来自配置。
func TestTabWidthFollowsConfig(t *testing.T) {
	back := screen.NewFake(testCaps)
	doc := document.FromString("")
	table, _ := keymap.Lookup(keymap.EmacsName)
	cfg := config.Default()
	cfg.Editor.TabWidth = 8

	app, err := New(Options{Backend: back, Config: cfg, Table: table, Doc: doc})
	if err != nil {
		t.Fatal(err)
	}
	app.draw()
	app.handleKey("<tab>")

	if got := string(doc.Text()); got != strings.Repeat(" ", 8) {
		t.Errorf("制表符展开 = %q, want 8 个空格", got)
	}
}

// ---- 撤销重做 ----

func TestUndoRedoKeys(t *testing.T) {
	h := newHarness(t, "")
	h.app.draw()
	h.pressAll("a", "b", "c")
	h.pressAll("ctrl+a", "r")
	if got := string(h.doc().Text()); got != "" {
		t.Errorf("撤销后 = %q, want 空", got)
	}
	h.pressAll("ctrl+a", "y")
	if got := string(h.doc().Text()); got != "abc" {
		t.Errorf("重做后 = %q, want %q", got, "abc")
	}
}

// ---- 剪贴板 ----

func TestCopyPasteKeys(t *testing.T) {
	clipboard = nil // 全局剪贴板，测试之间不能互相影响

	h := newHarness(t, "hello")
	h.app.draw()
	h.pressAll("ctrl+shift+c")
	if len(clipboard) != 5 {
		t.Fatalf("剪贴板 = %q, want 5 字节", clipboard)
	}
	h.pressAll("ctrl+<end>", "ctrl+shift+v")
	if got := string(h.doc().Text()); got != "hellohello" {
		t.Errorf("粘贴后 = %q, want %q", got, "hellohello")
	}
}

// TestPasteWithEmptyClipboard 空剪贴板粘贴不应报错，也不该插入任何东西。
func TestPasteWithEmptyClipboard(t *testing.T) {
	clipboard = nil

	h := newHarness(t, "abc")
	h.app.draw()
	h.press("ctrl+shift+v")

	if got := string(h.doc().Text()); got != "abc" {
		t.Errorf("内容 = %q, want 不变", got)
	}
	if !strings.Contains(h.app.currentStatus(), "空") {
		t.Errorf("状态栏 = %q, want 提到剪贴板为空", h.app.currentStatus())
	}
}

// ---- 未实现命令 ----

// TestNotImplementedCommandsAreAllReported 尚未实现的命令都必须明说，
// 不能静默返回——静默返回会被用户当成按键坏了。
func TestNotImplementedCommandsAreAllReported(t *testing.T) {
	h := newHarness(t, "")
	h.app.draw()

	cases := map[string]keymap.Command{
		"文件树":     keymap.CmdFileTree,
		"全局搜索":    keymap.CmdGlobalSearch,
		"分屏":      keymap.CmdSplitVertical,
		"命令面板":    keymap.CmdCommandPalette,
		"内置终端":    keymap.CmdTerminal,
		"输出面板":    keymap.CmdOutputPanel,
		"问题面板":    keymap.CmdProblemsPanel,
		"替换":      keymap.CmdReplace,
		"折叠":      keymap.CmdToggleFold,
		"多选":      keymap.CmdSelectNext,
		"列选择":     keymap.CmdColumnSelectUp,
		"go run":  keymap.CmdGoRun,
		"go test": keymap.CmdGoTest,
		"gopls":   keymap.CmdRestartLSP,
	}
	for name, cmd := range cases {
		t.Run(name, func(t *testing.T) {
			h.app.runCommand(keymap.Binding{Keys: "test", Cmd: cmd})
			status := h.app.currentStatus()
			if !strings.Contains(status, "尚未实现") {
				t.Errorf("命令 %q 的状态栏 = %q, want 含「尚未实现」", cmd, status)
			}
			if !strings.Contains(status, string(cmd)) {
				t.Errorf("状态栏 = %q, want 含命令名 %q", status, cmd)
			}
		})
	}
}

// ---- 只读 ----

// TestReadonlyBlocksEveryEditingCommand 只读文件下所有编辑命令都要被挡住。
func TestReadonlyBlocksEveryEditingCommand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ro.txt")
	if err := os.WriteFile(path, []byte("abc"), 0o444); err != nil {
		t.Fatal(err)
	}
	doc, err := document.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	back := screen.NewFake(testCaps)
	table, _ := keymap.Lookup(keymap.EmacsName)
	app, err := New(Options{Backend: back, Config: config.Default(), Table: table, Doc: doc})
	if err != nil {
		t.Fatal(err)
	}
	app.draw()

	edits := []keymap.Command{
		keymap.CmdInsertNewline, keymap.CmdInsertTab, keymap.CmdBackspace,
		keymap.CmdDeleteRight, keymap.CmdDeleteLine, keymap.CmdJoinLine,
		keymap.CmdIndent, keymap.CmdUnindent, keymap.CmdPaste, keymap.CmdSave,
	}
	for _, cmd := range edits {
		app.runCommand(keymap.Binding{Keys: "test", Cmd: cmd})
		if got := string(doc.Text()); got != "abc" {
			t.Fatalf("命令 %q 之后内容被改: %q", cmd, got)
		}
		if !strings.Contains(app.currentStatus(), "只读") {
			t.Errorf("命令 %q 之后状态栏 = %q, want 提到只读", cmd, app.currentStatus())
		}
	}
}

// TestCopyOnReadonlyAlsoRefused 只读文件不能复制到剪贴板：
// 用户复制不动会以为是快捷键坏了。
func TestCopyOnReadonlyAlsoRefused(t *testing.T) {
	clipboard = nil

	dir := t.TempDir()
	path := filepath.Join(dir, "ro.txt")
	if err := os.WriteFile(path, []byte("abc"), 0o444); err != nil {
		t.Fatal(err)
	}
	doc, err := document.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	back := screen.NewFake(testCaps)
	table, _ := keymap.Lookup(keymap.EmacsName)
	app, err := New(Options{Backend: back, Config: config.Default(), Table: table, Doc: doc})
	if err != nil {
		t.Fatal(err)
	}
	app.runCommand(keymap.Binding{Cmd: keymap.CmdCopy})

	if len(clipboard) != 0 {
		t.Errorf("只读文件的复制不该写入剪贴板，得到 %q", clipboard)
	}
}

// ---- 滚动 ----

func TestMouseScrollMovesViewport(t *testing.T) {
	h := newHarness(t, strings.Repeat("line\n", 200))
	h.app.draw()
	// 先把光标放到文档中段。视口必须跟着光标走，
	// 所以光标停在首行或末行时无论怎么滚都会被拉回去。
	// 跳转到行要走浮层，这里直接用内部实现把光标放到第 100 行：
	// 本测试关心的是滚动视口的行为，不是跳转行的交互。
	h.app.gotoLineInput("100")
	h.app.draw() // 视口跟随光标是在重画时收敛的
	if h.doc().Cursor().Line != 99 {
		t.Fatalf("光标行 = %d, want 99（跳转行没生效）", h.doc().Cursor().Line)
	}
	if h.app.top == 0 {
		t.Fatal("光标在第 100 行时视口本该已经滚动")
	}

	// 视口跟随光标意味着滚动有一个方向有富余：
	// 跳到某行时光标落在视口底边，往上滚有余量，往下滚会被拉回。
	before := h.app.top
	h.app.handleEvent(screen.Event{Kind: screen.EventMouseScroll, DeltaY: -3})
	if h.app.top <= before {
		t.Errorf("向上滚动后 top = %d, want > %d", h.app.top, before)
	}

	after := h.app.top
	h.app.handleEvent(screen.Event{Kind: screen.EventMouseScroll, DeltaY: 3})
	if h.app.top >= after {
		t.Errorf("向下滚动后 top = %d, want < %d", h.app.top, after)
	}

	// 无位移的滚动事件不该改变视口。
	stable := h.app.top
	h.app.handleEvent(screen.Event{Kind: screen.EventMouseScroll})
	if h.app.top != stable {
		t.Errorf("无位移滚动后 top = %d, want %d", h.app.top, stable)
	}
}

// ---- 状态栏 ----

func TestStatusHintExpires(t *testing.T) {
	h := newHarness(t, "abc")
	h.app.draw()

	h.app.setStatus("临时提示")
	if got := h.app.currentStatus(); got != "临时提示" {
		t.Errorf("刚设置的提示 = %q", got)
	}

	h.clock.Advance(statusTimeout + time.Second)
	if got := h.app.currentStatus(); got != "" {
		t.Errorf("超时后的提示 = %q, want 空", got)
	}
	// 超时后状态栏回落到模式名。
	if got := h.statusLine(); !strings.Contains(got, "NORMAL") {
		t.Errorf("超时后状态栏 = %q, want 含 NORMAL", got)
	}
}

func TestStatusLineMarksReadonly(t *testing.T) {
	d := document.NewNamed("ro.txt", true)
	d.InsertText("abc")
	back := screen.NewFake(testCaps)
	table, _ := keymap.Lookup(keymap.EmacsName)
	app, err := New(Options{Backend: back, Config: config.Default(), Table: table, Doc: d})
	if err != nil {
		t.Fatal(err)
	}
	if got := app.statusLine(); !strings.Contains(got, "只读") {
		t.Errorf("状态栏 = %q, want 含「只读」", got)
	}
}

func TestStatusLineTruncatesToWidth(t *testing.T) {
	h := newHarness(t, "abc")
	h.app.setStatus(strings.Repeat("很长的提示", 40))
	line := h.app.statusLine()

	// 中文按显示宽度算是两列，必须用显示宽度而不是字符个数来断言。
	if width := view.TextWidth(line); width > h.app.width {
		t.Errorf("状态栏宽度 = %d, want <= %d；截断必须生效否则终端会折行", width, h.app.width)
	}
}

func TestStatusLineWithVeryNarrowWidth(t *testing.T) {
	h := newHarness(t, "abc")
	for _, width := range []int{0, 1, 2, 3} {
		h.app.width = width
		if got := h.app.statusLine(); len([]rune(got)) > width && width > 0 {
			t.Errorf("宽度 %d 时状态栏 = %q, want 不超过 %d 个字符", width, got, width)
		}
	}
}

// TestDisplayName 状态栏只显示文件名，不显示整条路径。
func TestDisplayName(t *testing.T) {
	cases := map[string]string{
		"":                   "",
		"a.go":               "a.go",
		"/home/u/p/a.go":     "a.go",
		"C:\\Users\\u\\a.go": "a.go",
		"relative/dir/b.txt": "b.txt",
	}
	for path, want := range cases {
		if got := displayName(path); got != want {
			t.Errorf("displayName(%q) = %q, want %q", path, got, want)
		}
	}
}

// ---- 跳转行 ----

// TestGotoLineViaPrompt 跳转到行走浮层：命令本身不移动光标，
// 也不该依赖 binding.Arg —— 键位表里那条绑定本来就没有参数。
func TestGotoLineViaPrompt(t *testing.T) {
	h := newHarness(t, "a\nb\nc\nd")
	h.app.draw()
	h.pressAll("ctrl+a", "g")

	if h.app.overlay == nil || h.app.overlay.kind != overlayPrompt {
		t.Fatal("C-a g 之后没有打开输入浮层")
	}
	// 浮层预填当前行号，用户改一下就能跳。
	if got := h.app.overlay.prompt.text(); got != "1" {
		t.Errorf("浮层预填 = %q, want 当前行号 %q", got, "1")
	}
	// 命令本身不该移动光标。
	if got := h.doc().Cursor(); got.Line != 0 {
		t.Errorf("命令执行后光标行 = %d, want 0（要等用户确认）", got.Line)
	}
}

// TestGotoLineInputMovesCursor 输入合法行号后光标必须落到那一行。
func TestGotoLineInputMovesCursor(t *testing.T) {
	cases := []struct {
		name  string
		input string
		line  int
	}{
		{"第 3 行", "3", 2},
		{"第 1 行", "1", 0},
		{"越界截断", "99", 3},
		{"带空格", "  2  ", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, "a\nb\nc\nd")
			h.app.draw()
			h.app.gotoLineInput(tc.input)

			if got := h.doc().Cursor(); got.Line != tc.line {
				t.Errorf("输入 %q 后光标行 = %d, want %d", tc.input, got.Line, tc.line)
			}
		})
	}
}

// TestGotoLineRejectsBadInput 输入不合法时必须给出提示，且不移动光标。
func TestGotoLineRejectsBadInput(t *testing.T) {
	for _, input := range []string{"", "   ", "abc", "-", "1.5", "1 2"} {
		h := newHarness(t, "a\nb")
		h.app.draw()
		h.app.gotoLineInput(input)

		status := h.app.currentStatus()
		if status == "" {
			t.Errorf("输入 %q 之后没有任何提示", input)
		}
		if got := h.doc().Cursor(); got.Line != 0 {
			t.Errorf("输入 %q 之后光标行 = %d, want 0（不该移动）", input, got.Line)
		}
	}
}

// ---- 保存失败 ----

func TestSaveFailureIsReported(t *testing.T) {
	// 路径指向一个不存在的目录，写入必然失败。
	d := document.NewNamed("/definitely/not/here/x.txt", false)
	back := screen.NewFake(testCaps)
	table, _ := keymap.Lookup(keymap.EmacsName)
	app, err := New(Options{Backend: back, Config: config.Default(), Table: table, Doc: d})
	if err != nil {
		t.Fatal(err)
	}
	app.runCommand(keymap.Binding{Cmd: keymap.CmdSave})

	if !strings.Contains(app.currentStatus(), "保存失败") {
		t.Errorf("状态栏 = %q, want 含「保存失败」", app.currentStatus())
	}
}

func TestSaveWithoutPathIsReported(t *testing.T) {
	h := newHarness(t, "")
	h.app.runCommand(keymap.Binding{Cmd: keymap.CmdSave})

	if !strings.Contains(h.app.currentStatus(), "保存失败") {
		t.Errorf("状态栏 = %q, want 含「保存失败」", h.app.currentStatus())
	}
}

// ---- 标签 ----

// TestSelectTabWithoutMultipleDocs 只有一个标签页时切标签必须明说，
// 而不是假装切了。
func TestSelectTabWithoutMultipleDocs(t *testing.T) {
	h := newHarness(t, "abc")
	h.app.draw()
	h.pressAll("alt+1")

	if !strings.Contains(h.app.currentStatus(), "1 个标签页") {
		t.Errorf("状态栏 = %q, want 提到只有 1 个标签页", h.app.currentStatus())
	}
	// 按了 M-9 不该往文档里插入数字。
	if got := string(h.doc().Text()); got != "abc" {
		t.Errorf("内容 = %q, want 不变", got)
	}
}

// ---- 取消前缀 ----

func TestCancelPrefixCommand(t *testing.T) {
	h := newHarness(t, "abc")
	h.app.draw()
	h.press("ctrl+a")
	h.app.runCommand(keymap.Binding{Cmd: keymap.CmdCancelPrefix})

	if pending := h.app.matcher.Pending(); len(pending) != 0 {
		t.Errorf("取消后仍有待定前缀: %v", pending)
	}
}

// ---- 前缀键之后禁止编辑 ----

// TestPrefixBlocksEditing 是本项目一条关键交互约定：
// 按下前缀键之后，随后的按键是在「想完成组合键」，不是在「想输入字符」。
// 若把它插进文档，C-a 之后误按一个键就会污染文件、让文档变脏，
// 而且光标位置一变，后续操作继续错位。
func TestPrefixBlocksEditing(t *testing.T) {
	cases := []struct {
		name     string
		prefixes []string
		next     string
	}{
		{"C-a 之后按字母", []string{"ctrl+a"}, "j"},
		{"C-g 之后按字母", []string{"ctrl+g"}, "q"},
		{"C-a 之后按空格", []string{"ctrl+a"}, "space"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, "abc")
			h.app.draw()

			for _, key := range tc.prefixes {
				h.press(key)
			}
			before := string(h.doc().Text())
			cursorBefore := h.doc().Cursor()

			h.press(tc.next)

			if got := string(h.doc().Text()); got != before {
				t.Errorf("前缀之后按 %q 把文档改成了 %q（应该是 %q）",
					tc.next, got, before)
			}
			if h.doc().Cursor() != cursorBefore {
				t.Errorf("前缀之后按 %q 移动了光标: %+v -> %+v",
					tc.next, cursorBefore, h.doc().Cursor())
			}
			if h.doc().Dirty() {
				t.Errorf("前缀之后按 %q 让文档变脏了", tc.next)
			}
		})
	}
}

// TestPrefixFailureIsExplained 丢弃按键必须给出说明，
// 否则用户会以为编辑器吞了输入。
func TestPrefixFailureIsExplained(t *testing.T) {
	h := newHarness(t, "abc")
	h.app.draw()
	h.press("ctrl+a")
	// j 是没被绑到任何命令上的字母，用它来验证「不认识的键」这条路径。
	h.press("j")

	status := h.app.currentStatus()
	if !strings.Contains(status, "不是有效组合") {
		t.Errorf("状态栏 = %q, want 说明不是有效组合", status)
	}
	// 提示里要带上前缀名，用户才知道自己刚才按了什么。
	if !strings.Contains(status, "ctrl+a") {
		t.Errorf("状态栏 = %q, want 提到前缀名", status)
	}
	// 丢弃之后前缀已经清空，可以正常继续输入。
	if pending := h.app.matcher.Pending(); len(pending) != 0 {
		t.Errorf("丢弃后仍有待定前缀: %v", pending)
	}
	// 光标还在行首，所以输入插在前面。
	h.press("k")
	if got := string(h.doc().Text()); got != "kabc" {
		t.Errorf("丢弃之后应能正常输入，得到 %q, want %q", got, "kabc")
	}
}

// TestPlainLetterStillTypes 没有前缀时，未绑定的字母必须照常插入。
// 这条与上一条互为反面：前缀之后禁止输入，但不能因此把所有字母都禁掉。
func TestPlainLetterStillTypes(t *testing.T) {
	h := newHarness(t, "")
	h.app.draw()
	h.pressAll("h", "i")

	if got := string(h.doc().Text()); got != "hi" {
		t.Errorf("内容 = %q, want %q", got, "hi")
	}
	if !h.doc().Dirty() {
		t.Error("输入后文档应变脏")
	}
}

// ---- 退出 ----

func TestQuitKeyExitsWhenClean(t *testing.T) {
	h := newHarness(t, "abc")
	h.app.draw()

	h.pressAll("ctrl+a", "q")

	if !h.app.shouldQuit {
		t.Error("没有未保存改动时 C-a q 应该退出")
	}
}

// TestQuitRefusesWhenDirty 有未保存改动时不能退出，
// 而且必须把两个出口都说清楚。
func TestQuitRefusesWhenDirty(t *testing.T) {
	h := newHarness(t, "abc")
	h.app.draw()
	h.press("x")

	h.pressAll("ctrl+a", "q")

	if h.app.shouldQuit {
		t.Fatal("有未保存改动时不该退出")
	}
	status := h.app.currentStatus()
	for _, want := range []string{"C-a w", "C-a x"} {
		if !strings.Contains(status, want) {
			t.Errorf("状态栏 = %q, want 提到 %q（用户要知道下一步该按什么）", status, want)
		}
	}
}

// TestForceQuitExitsWithDirtyChanges 强制退出会真的退出。
func TestForceQuitExitsWithDirtyChanges(t *testing.T) {
	h := newHarness(t, "abc")
	h.app.draw()
	h.press("x")

	h.pressAll("ctrl+a", "x")

	if !h.app.shouldQuit {
		t.Error("C-a x 应该强制退出")
	}
}
