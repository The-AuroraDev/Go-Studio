// matcher_test.go — 按键序列状态机的单元测试。
// SPDX-License-Identifier: MIT

package keymap

import (
	"strings"
	"testing"
)

// newTestTable 造一张小表，覆盖「前缀 + 终点」「单键」「多级前缀」三种形状。
func newTestTable(t *testing.T) *Table {
	t.Helper()
	table, err := New("test", []Binding{
		{Keys: "g", Cmd: CmdGoRun, Help: "单键"},
		{Keys: "ctrl+a r", Cmd: CmdUndo, Help: "两级前缀的终点"},
		{Keys: "ctrl+a y", Cmd: CmdRedo, Help: "两级前缀的终点"},
		{Keys: "ctrl+g r", Cmd: CmdGoRun, Help: "另一棵子树"},
		{Keys: "ctrl+x", Cmd: CmdCopy, Help: "单键加修饰"},
	})
	if err != nil {
		t.Fatalf("New() 返回错误: %v", err)
	}
	return table
}

func TestMatcherMatchesSingleKey(t *testing.T) {
	m := NewMatcher(newTestTable(t))
	result, binding := m.Push("g")
	if result != ResultMatch {
		t.Fatalf("Push(g) = %v, want %v", result, ResultMatch)
	}
	if binding.Cmd != CmdGoRun {
		t.Errorf("命令 = %q, want %q", binding.Cmd, CmdGoRun)
	}
	if len(m.Pending()) != 0 {
		t.Errorf("命中后仍有待定前缀: %v", m.Pending())
	}
}

// TestMatcherWaitsForPrefix 是状态机最核心的一条：
// C-a 之后不能立刻执行任何命令，必须等第二键落地。
func TestMatcherWaitsForPrefix(t *testing.T) {
	m := NewMatcher(newTestTable(t))
	result, binding := m.Push("ctrl+a")
	if result != ResultPending {
		t.Fatalf("Push(ctrl+a) = %v, want %v", result, ResultPending)
	}
	if binding.Cmd != "" {
		t.Errorf("待定时不应带命令，得到 %q", binding.Cmd)
	}
	if got := m.PendingSeq(); got != "ctrl+a" {
		t.Errorf("PendingSeq() = %q, want %q", got, "ctrl+a")
	}

	result, binding = m.Push("r")
	if result != ResultMatch {
		t.Fatalf("Push(r) = %v, want %v", result, ResultMatch)
	}
	if binding.Cmd != CmdUndo {
		t.Errorf("命令 = %q, want %q", binding.Cmd, CmdUndo)
	}
	if len(m.Pending()) != 0 {
		t.Errorf("命中后仍有待定前缀: %v", m.Pending())
	}
}

// TestMatcherRejectsUnknownContinuation 未命中的后续键必须丢弃整条序列。
// 若只清掉最后一段，C-a 的待定状态会残留，后面几键都会落到错误的上下文。
func TestMatcherRejectsUnknownContinuation(t *testing.T) {
	m := NewMatcher(newTestTable(t))
	if result, _ := m.Push("ctrl+a"); result != ResultPending {
		t.Fatalf("Push(ctrl+a) = %v, want %v", result, ResultPending)
	}
	result, binding := m.Push("q")
	if result != ResultUnknown {
		t.Fatalf("Push(q) = %v, want %v", result, ResultUnknown)
	}
	if binding.Cmd != "" {
		t.Errorf("未命中不应带命令，得到 %q", binding.Cmd)
	}
	if len(m.Pending()) != 0 {
		t.Errorf("未命中后待定前缀未清空: %v", m.Pending())
	}

	// 关键：清空必须彻底，r 不能因为前面按过 ctrl+a 就触发撤销。
	if result, _ := m.Push("r"); result != ResultUnknown {
		t.Errorf("Push(r) = %v, want %v（前缀应已清空）", result, ResultUnknown)
	}
}

func TestMatcherRejectsUnknownFirstKey(t *testing.T) {
	m := NewMatcher(newTestTable(t))
	if result, _ := m.Push("q"); result != ResultUnknown {
		t.Errorf("Push(q) = %v, want %v", result, ResultUnknown)
	}
}

// TestMatcherAbortKeyCancelsPending 固化 C-g 的双重身份：
// 有待定前缀时它是中止键，没有前缀时它是普通首键。
func TestMatcherAbortKeyCancelsPending(t *testing.T) {
	m := NewMatcher(newTestTable(t))
	if result, _ := m.Push("ctrl+a"); result != ResultPending {
		t.Fatalf("Push(ctrl+a) = %v, want %v", result, ResultPending)
	}
	result, binding := m.Push(AbortKey)
	if result != ResultMatch {
		t.Fatalf("Push(ctrl+g) = %v, want %v", result, ResultMatch)
	}
	if binding.Cmd != CmdCancelPrefix {
		t.Errorf("命令 = %q, want %q", binding.Cmd, CmdCancelPrefix)
	}
	if len(m.Pending()) != 0 {
		t.Errorf("中止后待定前缀未清空: %v", m.Pending())
	}
}

func TestMatcherAbortKeyIsNormalKeyWhenNotPending(t *testing.T) {
	m := NewMatcher(newTestTable(t))
	result, _ := m.Push(AbortKey)
	if result != ResultPending {
		t.Fatalf("Push(ctrl+g) = %v, want %v（应进入 C-g 子树）", result, ResultPending)
	}
	result, binding := m.Push("r")
	if result != ResultMatch || binding.Cmd != CmdGoRun {
		t.Errorf("Push(ctrl+g r) = %v/%q, want %v/%q", result, binding.Cmd, ResultMatch, CmdGoRun)
	}
}

// TestMatcherAbortAlsoCancelsGoSubtree 固化一个容易踩的边界：
// 进入 C-g 子树后 Ctrl+G 不再是「进入子树」而是中止。
func TestMatcherAbortAlsoCancelsGoSubtree(t *testing.T) {
	m := NewMatcher(newTestTable(t))
	if result, _ := m.Push("ctrl+g"); result != ResultPending {
		t.Fatalf("Push(ctrl+g) = %v, want %v", result, ResultPending)
	}
	result, binding := m.Push("ctrl+g")
	if result != ResultMatch || binding.Cmd != CmdCancelPrefix {
		t.Fatalf("Push(ctrl+g ctrl+g) = %v/%q, want %v/%q",
			result, binding.Cmd, ResultMatch, CmdCancelPrefix)
	}
}

func TestMatcherIgnoresEmptyKey(t *testing.T) {
	m := NewMatcher(newTestTable(t))
	if result, _ := m.Push(""); result != ResultIgnored {
		t.Errorf("Push(\"\") = %v, want %v", result, ResultIgnored)
	}
	if result, _ := m.Push("ctrl+a"); result != ResultPending {
		t.Fatalf("Push(ctrl+a) = %v, want %v", result, ResultPending)
	}
	// 空键不能打断待定前缀，否则按键松开事件会毁掉正在输入的组合键。
	if result, _ := m.Push(""); result != ResultIgnored {
		t.Errorf("Push(\"\") = %v, want %v", result, ResultIgnored)
	}
	if got := m.PendingSeq(); got != "ctrl+a" {
		t.Errorf("待定前缀被空键打断: %q", got)
	}
}

func TestMatcherReset(t *testing.T) {
	m := NewMatcher(newTestTable(t))
	if result, _ := m.Push("ctrl+a"); result != ResultPending {
		t.Fatalf("Push(ctrl+a) = %v, want %v", result, ResultPending)
	}
	m.Reset()
	if len(m.Pending()) != 0 {
		t.Errorf("Reset() 后仍有待定前缀: %v", m.Pending())
	}
	if got := m.PendingSeq(); got != "" {
		t.Errorf("PendingSeq() = %q, want 空", got)
	}
	if result, _ := m.Push("ctrl+a"); result != ResultPending {
		t.Errorf("Reset() 后无法重新进入前缀，得到 %v", result)
	}
}

// TestMatcherPendingIsACopy 保证调用方改返回值不会污染匹配器状态。
func TestMatcherPendingIsACopy(t *testing.T) {
	m := NewMatcher(newTestTable(t))
	if result, _ := m.Push("ctrl+a"); result != ResultPending {
		t.Fatalf("Push(ctrl+a) = %v, want %v", result, ResultPending)
	}
	pending := m.Pending()
	pending[0] = "tampered"
	if got := m.PendingSeq(); got != "ctrl+a" {
		t.Errorf("修改 Pending() 的返回值污染了状态: %q", got)
	}
}

// TestMatcherAcceptsNormalizedVariants 保证书写顺序不影响匹配。
func TestMatcherAcceptsNormalizedVariants(t *testing.T) {
	table, err := New("t", []Binding{{Keys: "alt+ctrl+shift+a", Cmd: CmdCopy}})
	if err != nil {
		t.Fatalf("New() 返回错误: %v", err)
	}
	// 归一化顺序是 ctrl、alt、shift，因此存下来的是 ctrl+alt+shift+a。
	for _, written := range []string{"ctrl+alt+shift+a", "shift+ctrl+alt+a", "alt+ctrl+shift+a"} {
		m := NewMatcher(table)
		key, ok := table.Lookup(written)
		if !ok {
			t.Fatalf("Lookup(%q) 未命中", written)
		}
		if result, _ := m.Push(key.Keys); result != ResultMatch {
			t.Errorf("以 %q 书写时 Push 未命中", written)
		}
	}
}

// TestMatcherStateIsIsolatedAcrossMatchers 保证两个匹配器互不影响。
func TestMatcherStateIsIsolatedAcrossMatchers(t *testing.T) {
	table := newTestTable(t)
	first := NewMatcher(table)
	second := NewMatcher(table)
	if result, _ := first.Push("ctrl+a"); result != ResultPending {
		t.Fatalf("first.Push(ctrl+a) = %v, want %v", result, ResultPending)
	}
	if len(second.Pending()) != 0 {
		t.Errorf("第二个匹配器被污染: %v", second.Pending())
	}
	if result, _ := second.Push("r"); result != ResultUnknown {
		t.Errorf("second.Push(r) = %v, want %v", result, ResultUnknown)
	}
}

// TestMatcherDeepChain 三级以上的序列也要能逐级等待。
func TestMatcherDeepChain(t *testing.T) {
	table, err := New("t", []Binding{{Keys: "ctrl+a ctrl+b ctrl+c", Cmd: CmdCopy}})
	if err != nil {
		t.Fatalf("New() 返回错误: %v", err)
	}
	m := NewMatcher(table)
	for i, key := range []string{"ctrl+a", "ctrl+b"} {
		if result, _ := m.Push(key); result != ResultPending {
			t.Fatalf("第 %d 键 %q = %v, want %v", i+1, key, result, ResultPending)
		}
	}
	result, binding := m.Push("ctrl+c")
	if result != ResultMatch || binding.Cmd != CmdCopy {
		t.Errorf("末键 = %v/%q, want %v/%q", result, binding.Cmd, ResultMatch, CmdCopy)
	}
}

func TestMatcherKeepsPendingOnUnknownThenRecovers(t *testing.T) {
	m := NewMatcher(newTestTable(t))
	if result, _ := m.Push("ctrl+a"); result != ResultPending {
		t.Fatalf("Push(ctrl+a) = %v, want %v", result, ResultPending)
	}
	if result, _ := m.Push("z"); result != ResultUnknown {
		t.Fatalf("Push(z) = %v, want %v", result, ResultUnknown)
	}
	// 出错之后必须还能正常使用。
	if result, _ := m.Push("ctrl+a"); result != ResultPending {
		t.Errorf("Push(ctrl+a) = %v, want %v", result, ResultPending)
	}
	if result, binding := m.Push("y"); result != ResultMatch || binding.Cmd != CmdRedo {
		t.Errorf("Push(ctrl+a y) = %v/%q, want %v/%q", result, binding.Cmd, ResultMatch, CmdRedo)
	}
}

func TestPendingSeqEmptyWhenIdle(t *testing.T) {
	m := NewMatcher(newTestTable(t))
	if got := m.PendingSeq(); got != "" {
		t.Errorf("PendingSeq() = %q, want 空", got)
	}
}

// TestAbortKeyIsBoundInGoSubtree 确认中止键本身没有把 Go 子树堵死。
func TestAbortKeyIsBoundInGoSubtree(t *testing.T) {
	table, err := Emacs()
	if err != nil {
		t.Fatalf("Emacs() 返回错误: %v", err)
	}
	m := NewMatcher(table)
	if result, _ := m.Push("ctrl+g"); result != ResultPending {
		t.Fatalf("Push(ctrl+g) = %v, want %v", result, ResultPending)
	}
	result, binding := m.Push("r")
	if result != ResultMatch {
		t.Fatalf("Push(ctrl+g r) = %v, want %v", result, ResultMatch)
	}
	if binding.Cmd != CmdGoRun {
		t.Errorf("命令 = %q, want %q", binding.Cmd, CmdGoRun)
	}
	if !strings.Contains(binding.Help, "go run") {
		t.Errorf("说明文字 = %q, want 包含 %q", binding.Help, "go run")
	}
}
