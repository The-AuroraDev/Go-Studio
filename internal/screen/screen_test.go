// screen_test.go — Screen 契约、按键归一化与 Bubble Tea 事件翻译的单元测试。
// SPDX-License-Identifier: MIT

// 本文件用到 unix.TCGETS 与 creack/pty，只能在类 Unix 平台编译。
//go:build !windows

package screen

import (
	"os"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

func TestNormalizeKeystrokeSortsModifiers(t *testing.T) {
	cases := map[string]string{
		"alt+ctrl+a":   "ctrl+alt+a",
		"shift+ctrl+a": "ctrl+shift+a",
		"a":            "a",
		"ctrl+c":       "ctrl+c",
		"":             "",
	}
	for input, want := range cases {
		if got := NormalizeKeystroke(input); got != want {
			t.Errorf("NormalizeKeystroke(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeKeystrokeKeepsNamedKeys(t *testing.T) {
	if got := NormalizeKeystroke("<esc>"); got != "<esc>" {
		t.Errorf("NormalizeKeystroke(<esc>) = %q, want unchanged", got)
	}
}

func TestIsNamedKey(t *testing.T) {
	if !IsNamedKey("<esc>") {
		t.Error("IsNamedKey(<esc>) = false, want true")
	}
	if IsNamedKey("<nope>") {
		t.Error("IsNamedKey(<nope>) = true, want false")
	}
	if IsNamedKey("a") {
		t.Error("IsNamedKey(a) = true, want false")
	}
}

func TestToKeystrokeMapsSpecialKeys(t *testing.T) {
	cases := []struct {
		key  tea.Key
		want string
	}{
		{tea.Key{Code: tea.KeyEscape}, "<esc>"},
		{tea.Key{Code: tea.KeyEnter}, "<enter>"},
		{tea.Key{Code: tea.KeyTab}, "<tab>"},
		// 空格是可打印字符，不该被当成命名键：否则输入一个空格就会触发
		// 绑在它上面的命令，编辑器连空格都打不出来。
		{tea.Key{Code: tea.KeySpace, Text: " "}, "space"},
		{tea.Key{Code: tea.KeySpace}, "space"},
		{tea.Key{Code: tea.KeyBackspace}, "<backspace>"},
		{tea.Key{Code: tea.KeyUp}, "<up>"},
		{tea.Key{Code: tea.KeyF5}, "<f5>"},
		{tea.Key{Code: 'a', Text: "a"}, "a"},
		{tea.Key{Code: 'a', Text: "a", Mod: tea.ModCtrl}, "ctrl+a"},
		// Ctrl+空格必须与普通空格区分开，否则文件树快捷键一按就变成输入空格。
		{tea.Key{Code: tea.KeySpace, Mod: tea.ModCtrl}, "ctrl+space"},
	}
	for _, tc := range cases {
		if got := toKeystroke(tc.key); got != tc.want {
			t.Errorf("toKeystroke(%v) = %q, want %q", tc.key, got, tc.want)
		}
	}
}

// TestToKeystrokePreservesTypedCharacter 是文本编辑器的命门：
// 终端不会把大写字母当成独立的上档键上报，A 是以 Code='a' + shift + Text="A"
// 的形式到达的。只取 Keystroke() 会把每个大写字母都变成 shift+a，字符本身丢失。
func TestToKeystrokePreservesTypedCharacter(t *testing.T) {
	cases := []struct {
		name string
		key  tea.Key
		want string
	}{
		{"小写字母", tea.Key{Code: 'a', Text: "a"}, "a"},
		{"大写字母", tea.Key{Code: 'a', Text: "A", ShiftedCode: 'A', Mod: tea.ModShift}, "A"},
		{"大写字母不带上档码", tea.Key{Code: 'A', Text: "A", Mod: tea.ModShift}, "A"},
		{"数字", tea.Key{Code: '1', Text: "1"}, "1"},
		{"符号", tea.Key{Code: ';', Text: ";"}, ";"},
		{"中文", tea.Key{Code: '中', Text: "中"}, "中"},
		{"emoji", tea.Key{Code: '😀', Text: "😀"}, "😀"},
		{"Shift 加符号", tea.Key{Code: '1', Text: "!", Mod: tea.ModShift}, "!"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := toKeystroke(tc.key); got != tc.want {
				t.Errorf("toKeystroke(%v) = %q, want %q", tc.key, got, tc.want)
			}
		})
	}
}

// TestToKeystrokeKeepsModifiedKeys 带 ctrl/alt/meta 的按键是快捷键，
// 必须保留修饰键信息，不能当成字符输入。
func TestToKeystrokeKeepsModifiedKeys(t *testing.T) {
	cases := []struct {
		key  tea.Key
		want string
	}{
		{tea.Key{Code: 'a', Text: "a", Mod: tea.ModCtrl}, "ctrl+a"},
		{tea.Key{Code: 'a', Text: "a", Mod: tea.ModAlt}, "alt+a"},
		{tea.Key{Code: 'a', Text: "a", Mod: tea.ModAlt | tea.ModShift}, "alt+shift+a"},
		{tea.Key{Code: 'a', Text: "a", Mod: tea.ModMeta}, "meta+a"},
		{tea.Key{Code: '<', Text: "<", Mod: tea.ModCtrl}, "ctrl+<"},
	}
	for _, tc := range cases {
		if got := toKeystroke(tc.key); got != tc.want {
			t.Errorf("toKeystroke(%v) = %q, want %q", tc.key, got, tc.want)
		}
	}
}

func TestToKeystrokeKeepsModifiersOnSpecialKeys(t *testing.T) {
	key := tea.Key{Code: tea.KeyEnter, Mod: tea.ModShift}
	if got := toKeystroke(key); got != "shift+<enter>" {
		t.Errorf("toKeystroke = %q, want %q", got, "shift+<enter>")
	}
}

func TestPrintableRune(t *testing.T) {
	cases := []struct {
		keystroke string
		want      rune
		ok        bool
	}{
		{"a", 'a', true},
		{"A", 'A', true},
		{"1", '1', true},
		{"中", '中', true},
		{"😀", '😀', true},
		{SpaceKey, ' ', true},
		// 命名键不是字符输入。
		{"<esc>", 0, false},
		{"<enter>", 0, false},
		{"<up>", 0, false},
		{"<f5>", 0, false},
		// 带修饰键的是快捷键。
		{"ctrl+a", 0, false},
		{"alt+1", 0, false},
		{"ctrl+space", 0, false},
		{"meta+t", 0, false},
		// 空与纯修饰键。
		{"", 0, false},
		{"ctrl+", 0, false},
		{"ctrl+alt", 0, false},
		// 多字符的文本不是单个字符。
		{"ab", 0, false},
		// 不可打印字符。
		{"\n", 0, false},
		{"\x01", 0, false},
	}
	for _, tc := range cases {
		got, ok := PrintableRune(tc.keystroke)
		if ok != tc.ok {
			t.Errorf("PrintableRune(%q) ok = %t, want %t", tc.keystroke, ok, tc.ok)
			continue
		}
		if ok && got != tc.want {
			t.Errorf("PrintableRune(%q) = %q, want %q", tc.keystroke, got, tc.want)
		}
	}
}

// TestPrintableRuneSpaceIsTypable 固化「空格必须可输入」这条最基本的可用性要求。
func TestPrintableRuneSpaceIsTypable(t *testing.T) {
	r, ok := PrintableRune(toKeystroke(tea.Key{Code: tea.KeySpace, Text: " "}))
	if !ok || r != ' ' {
		t.Errorf("按空格键得到 (%q, %t), want (' ', true)", r, ok)
	}
}

func TestCursorShapeString(t *testing.T) {
	cases := map[CursorShape]string{
		CursorBlock:     "block",
		CursorUnderline: "underline",
		CursorBar:       "bar",
	}
	for shape, want := range cases {
		if got := shape.String(); got != want {
			t.Errorf("CursorShape(%d).String() = %q, want %q", int(shape), got, want)
		}
	}
}

func TestEventKindString(t *testing.T) {
	cases := map[EventKind]string{
		EventKeyPress:    "key-press",
		EventKeyRelease:  "key-release",
		EventResize:      "resize",
		EventMouseScroll: "mouse-scroll",
		EventFocusLost:   "focus-lost",
	}
	for kind, want := range cases {
		if got := kind.String(); got != want {
			t.Errorf("EventKind(%d).String() = %q, want %q", int(kind), got, want)
		}
	}
}

func TestCapsLogKeyValueIsComplete(t *testing.T) {
	caps := Caps{Width: 120, Height: 40, TrueColor: true, Terminal: "kitty"}
	pairs := caps.LogKeyValue()
	if len(pairs)%2 != 0 {
		t.Fatalf("LogKeyValue returned odd length %d", len(pairs))
	}
	seen := make(map[any]bool, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		key, ok := pairs[i].(string)
		if !ok {
			t.Fatalf("pair %d key is %T, want string", i, pairs[i])
		}
		if seen[key] {
			t.Errorf("duplicate key %q in LogKeyValue", key)
		}
		seen[key] = true
	}
	for _, want := range []string{"width", "height", "trueColor", "terminal"} {
		if !seen[want] {
			t.Errorf("LogKeyValue missing key %q", want)
		}
	}
}

func TestFakeRecordsFramesAndEmitsEvents(t *testing.T) {
	fake := NewFake(Caps{Width: 80, Height: 24})

	fake.Render("hello", &CursorSpec{X: 3, Y: 4})
	last := fake.LastFrame()
	if last.Content != "hello" {
		t.Errorf("content = %q, want %q", last.Content, "hello")
	}
	if last.Cursor == nil || last.Cursor.X != 3 || last.Cursor.Y != 4 {
		t.Errorf("cursor = %+v, want {3 4}", last.Cursor)
	}

	if !fake.EmitKey("q") {
		t.Fatal("EmitKey returned false")
	}
	event := <-fake.Events()
	if event.Kind != EventKeyPress || event.Keystroke != "q" {
		t.Errorf("event = %+v, want key-press q", event)
	}

	if err := fake.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	// 关闭后 Render 与 Emit 都必须被忽略，而不是 panic。
	fake.Render("after close", nil)
	if fake.EmitKey("x") {
		t.Error("EmitKey after close returned true")
	}
	if got := len(fake.Frames()); got != 1 {
		t.Errorf("frame count after close = %d, want 1", got)
	}
}

func TestFakeCopiesCursorToProtectRecordedFrames(t *testing.T) {
	fake := NewFake(Caps{})
	cursor := &CursorSpec{X: 1, Y: 1}
	fake.Render("frame", cursor)

	cursor.X = 99
	if got := fake.LastFrame().Cursor.X; got != 1 {
		t.Errorf("recorded cursor X = %d, want 1 (frame must not alias caller's cursor)", got)
	}
}

func TestToTeaCursorMapsShapes(t *testing.T) {
	if got := toTeaCursor(nil); got != nil {
		t.Error("toTeaCursor(nil) must be nil so the cursor is hidden")
	}

	cases := map[CursorShape]tea.CursorShape{
		CursorBlock:     tea.CursorBlock,
		CursorUnderline: tea.CursorUnderline,
		CursorBar:       tea.CursorBar,
	}
	for shape, want := range cases {
		got := toTeaCursor(&CursorSpec{Shape: shape})
		if got.Shape != want {
			t.Errorf("toTeaCursor(%v).Shape = %v, want %v", shape, got.Shape, want)
		}
	}

	positioned := toTeaCursor(&CursorSpec{X: 5, Y: 6, Blink: true})
	if positioned.X != 5 || positioned.Y != 6 || !positioned.Blink {
		t.Errorf("toTeaCursor position = (%d,%d) blink=%t, want (5,6) blink=true",
			positioned.X, positioned.Y, positioned.Blink)
	}
}

// TestCaptureTTYStateOnNonTerminal 验证非终端文件不会让启动失败，
// 也不会产生可用的恢复快照。
func TestCaptureTTYStateOnNonTerminal(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "notatty")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if got := captureTTYState(f); got != nil {
		t.Errorf("普通文件的 captureTTYState = %v, want nil", got)
	}
	if isTerminal(f) {
		t.Error("普通文件不应被判定为终端")
	}
	if captureTTYState(nil) != nil {
		t.Error("nil 文件的 captureTTYState 应为 nil")
	}
}

// TestRestoreTTYStateOnNilIsNoop 验证 nil 快照恢复时不报错，
// 这是输出被重定向到管道时的正常路径。
func TestRestoreTTYStateOnNilIsNoop(t *testing.T) {
	if err := (*ttyState)(nil).restore(); err != nil {
		t.Errorf("nil 快照恢复应无错，得到 %v", err)
	}
}

// TestRestoreTTYStateIsIdempotent 验证重复恢复不会出错，
// 因为 Bubble Tea 收尾与我们自己的兜底都会写回同一份快照。
func TestRestoreTTYStateIsIdempotent(t *testing.T) {
	ptmx, ptty, err := pty.Open()
	if err != nil {
		t.Skipf("无法分配伪终端: %v", err)
	}
	defer ptmx.Close()
	defer ptty.Close()

	state := captureTTYState(ptmx)
	if state == nil {
		t.Skip("无法读取伪终端设置")
	}

	// 先改成 raw，模拟程序运行期间的状态。
	if err := unix.IoctlSetTermios(int(ptmx.Fd()), unix.TCSETS, &unix.Termios{
		Iflag: unix.IGNBRK, Lflag: 0,
	}); err != nil {
		t.Fatal(err)
	}

	for i := range 3 {
		if err := state.restore(); err != nil {
			t.Fatalf("第 %d 次恢复失败: %v", i, err)
		}
	}

	got, err := unix.IoctlGetTermios(int(ptmx.Fd()), unix.TCGETS)
	if err != nil {
		t.Fatal(err)
	}
	if got.Lflag != state.sys.Lflag || got.Iflag != state.sys.Iflag {
		t.Errorf("恢复后与快照不符: got Lflag=%#x Iflag=%#x, want Lflag=%#x Iflag=%#x",
			got.Lflag, got.Iflag, state.sys.Lflag, state.sys.Iflag)
	}
}

// TestRestoreRecoversTerminalLeftInRawMode 直接验证核心承诺：
// 无论收尾流程如何，restore 都能把留在 raw 模式的终端救回来。
func TestRestoreRecoversTerminalLeftInRawMode(t *testing.T) {
	ptmx, ptty, err := pty.Open()
	if err != nil {
		t.Skipf("无法分配伪终端: %v", err)
	}
	defer ptmx.Close()
	defer ptty.Close()

	state := captureTTYState(ptmx)
	if state == nil {
		t.Skip("无法读取伪终端设置")
	}
	original := state.sys.Lflag
	if original&unix.ECHO == 0 {
		t.Skip("伪终端默认未开 ECHO，跳过")
	}

	// 模拟 raw 模式：关掉回显、规范化、信号与扩展处理。
	if err := unix.IoctlSetTermios(int(ptmx.Fd()), unix.TCSETS, &unix.Termios{
		Iflag: unix.IGNBRK, Lflag: 0,
	}); err != nil {
		t.Fatal(err)
	}

	if err := state.restore(); err != nil {
		t.Fatalf("恢复失败: %v", err)
	}

	got, err := unix.IoctlGetTermios(int(ptmx.Fd()), unix.TCGETS)
	if err != nil {
		t.Fatal(err)
	}
	if got.Lflag != original {
		t.Errorf("ECHO 未恢复: Lflag=%#x, want %#x", got.Lflag, original)
	}
}
