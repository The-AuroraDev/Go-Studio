// screen_test.go — Screen 契约、按键归一化与 Bubble Tea 事件翻译的单元测试。
// SPDX-License-Identifier: MIT

package screen

import (
	"testing"

	tea "charm.land/bubbletea/v2"
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
		{tea.Key{Code: tea.KeySpace}, "<space>"},
		{tea.Key{Code: tea.KeyBackspace}, "<backspace>"},
		{tea.Key{Code: tea.KeyUp}, "<up>"},
		{tea.Key{Code: tea.KeyF5}, "<f5>"},
		{tea.Key{Code: 'a', Text: "a"}, "a"},
		{tea.Key{Code: 'a', Text: "a", Mod: tea.ModCtrl}, "ctrl+a"},
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
