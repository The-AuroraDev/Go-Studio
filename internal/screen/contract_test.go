// contract_test.go — 对外契约的边界测试：默认值、非法输入、归一化。
// SPDX-License-Identifier: MIT

// 本文件不涉及 termios，不需要平台约束。

package screen

import (
	"strings"
	"testing"
)

// TestCursorShapeUnknown 未知形状必须给出可诊断的名字，而不是空串或数字。
func TestCursorShapeUnknown(t *testing.T) {
	got := CursorShape(42).String()
	if got != "unknown(42)" {
		t.Errorf("CursorShape(42).String() = %q, want %q", got, "unknown(42)")
	}
	for _, valid := range []CursorShape{CursorBlock, CursorUnderline, CursorBar} {
		if got := valid.String(); got == "" || strings.HasPrefix(got, "unknown") {
			t.Errorf("合法形状 %d 的名字 = %q", int(valid), got)
		}
	}
}

// TestEventKindUnknown 同上。
func TestEventKindUnknown(t *testing.T) {
	got := EventKind(99).String()
	if got != "unknown(99)" {
		t.Errorf("EventKind(99).String() = %q, want %q", got, "unknown(99)")
	}
	for _, valid := range []EventKind{
		EventKeyPress, EventKeyRelease, EventResize, EventMouseScroll, EventFocusLost,
	} {
		if got := valid.String(); got == "" || strings.HasPrefix(got, "unknown") {
			t.Errorf("合法事件 %d 的名字 = %q", int(valid), got)
		}
	}
}

// TestNormalizeKeystrokeRejectsModifierOnly 只有修饰键、没有基键的输入
// 归一化后必须是空串：它无法参与键位匹配。
// 这条契约由键位表依赖——把这类输入当成合法按键会让空绑定静默命中。
func TestNormalizeKeystrokeRejectsModifierOnly(t *testing.T) {
	cases := []string{
		"ctrl+",
		"ctrl+alt+",
		"alt+shift+meta+",
		"+", // 畸形输入里的空段
	}
	for _, in := range cases {
		if got := NormalizeKeystroke(in); got != "" {
			t.Errorf("NormalizeKeystroke(%q) = %q, want 空串", in, got)
		}
	}
}

// TestNormalizeKeystrokeEmpty 空输入必须原样返回空串。
func TestNormalizeKeystrokeEmpty(t *testing.T) {
	if got := NormalizeKeystroke(""); got != "" {
		t.Errorf("NormalizeKeystroke(\"\") = %q, want 空串", got)
	}
}

// TestNormalizeKeystrokeIsIdempotent 归一化之后再归一化必须不变，
// 否则每次按键都会被改写一次，键位表永远匹配不上。
func TestNormalizeKeystrokeIsIdempotent(t *testing.T) {
	cases := []string{
		"a", "A", "ctrl+a", "alt+ctrl+shift+a", "<up>", "alt+shift+<up>",
		"ctrl+<space>", "space", "中", "f5", "ctrl+shift+c",
	}
	for _, in := range cases {
		once := NormalizeKeystroke(in)
		if twice := NormalizeKeystroke(once); twice != once {
			t.Errorf("NormalizeKeystroke 不幂等: %q -> %q -> %q", in, once, twice)
		}
	}
}

// TestNormalizeKeystrokeFixesOrder 修饰键顺序不影响结果，这是键位表的前提。
func TestNormalizeKeystrokeFixesOrder(t *testing.T) {
	variants := []string{
		"ctrl+alt+shift+a",
		"shift+ctrl+alt+a",
		"alt+shift+ctrl+a",
		"shift+alt+ctrl+a",
	}
	want := NormalizeKeystroke(variants[0])
	for _, v := range variants[1:] {
		if got := NormalizeKeystroke(v); got != want {
			t.Errorf("NormalizeKeystroke(%q) = %q, want %q", v, got, want)
		}
	}
}

// TestIsNamedKeyAcceptsAllNamedKeys namedKeys 里登记的都必须被认出来，
// 否则这些按键无法写进配置。
func TestIsNamedKeyAcceptsAllNamedKeys(t *testing.T) {
	for name := range namedKeys {
		if !IsNamedKey(name) {
			t.Errorf("IsNamedKey(%q) = false，但它在 namedKeys 里", name)
		}
	}
}

// TestIsNamedKeyRejectsNonNamedKeys 不在表里的写法必须被拒绝，
// 让配置里的笔误在启动时就暴露。
func TestIsNamedKeyRejectsNonNamedKeys(t *testing.T) {
	cases := []string{
		"", "a", "ctrl+a", "<up", "up>", "<>", "<not-a-key>", " <up> ",
	}
	for _, in := range cases {
		if IsNamedKey(in) {
			t.Errorf("IsNamedKey(%q) = true, want false", in)
		}
	}
}

// TestNamedKeysAreLowercased IsNamedKey 大小写不敏感，
// 写 <UP> 也应当被接受。
func TestNamedKeysAreLowercased(t *testing.T) {
	if !IsNamedKey("<UP>") {
		t.Error("IsNamedKey(\"<UP>\") = false, want true（应大小写不敏感）")
	}
	if !IsNamedKey("<ESC>") {
		t.Error("IsNamedKey(\"<ESC>\") = false, want true（应大小写不敏感）")
	}
}

// TestCapsLogKeyValueIsSorted 键值对必须按键名排序，日志才好比对。
func TestCapsLogKeyValueIsSorted(t *testing.T) {
	pairs := Caps{Width: 1, Height: 2, TrueColor: true, Terminal: "x"}.LogKeyValue()
	if len(pairs)%2 != 0 {
		t.Fatalf("键值对数量 = %d，应为偶数", len(pairs))
	}
	for i := 0; i+1 < len(pairs); i += 2 {
		key, ok := pairs[i].(string)
		if !ok {
			t.Fatalf("第 %d 项 %v 不是字符串键", i, pairs[i])
		}
		if i+2 < len(pairs) {
			next, _ := pairs[i+2].(string)
			if key >= next {
				t.Errorf("键 %q 未排在 %q 之前", key, next)
			}
		}
	}
}
