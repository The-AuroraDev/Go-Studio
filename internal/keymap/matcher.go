// matcher.go — 按键序列的状态机：把逐次按键累积成命令。
// SPDX-License-Identifier: MIT

package keymap

import "strings"

// Result 是一次按键解析的结果。
type Result int

const (
	// ResultMatch 表示已确定命令，应当执行。
	ResultMatch Result = iota
	// ResultPending 表示已按下某个前缀键，正在等待后续按键。
	ResultPending
	// ResultUnknown 表示当前按键序列不匹配任何绑定，应当丢弃并清空前缀。
	ResultUnknown
	// ResultIgnored 表示该按键不参与匹配（例如只有修饰键），状态不变。
	ResultIgnored
)

// String 返回结果名，便于日志与测试失败信息。
func (r Result) String() string {
	switch r {
	case ResultMatch:
		return "match"
	case ResultPending:
		return "pending"
	case ResultUnknown:
		return "unknown"
	case ResultIgnored:
		return "ignored"
	default:
		return "invalid"
	}
}

// AbortKey 是「放弃当前前缀」的中止键，取 Emacs 的 C-g。
//
// 它的行为取决于是否已有前缀在等待：没有待定前缀时，C-g 正常进入 Go 命令子树；
// 已经有待定前缀时（例如刚按了 C-a），C-g 不进入子树而是清空前缀，
// 让用户能从任何半途而废的按键序列里脱身。
const AbortKey = "ctrl+g"

// Matcher 逐次接收按键，累积成一条完整的按键序列并解析成命令。
// 它保存待定前缀，因此每个文档只能有一个 Matcher。
type Matcher struct {
	table   *Table
	pending []string
	node    *node
}

// NewMatcher 为一套键位方案构造匹配器。
func NewMatcher(table *Table) *Matcher {
	return &Matcher{table: table, node: table.root}
}

// Reset 清空待定前缀。切换文档或重新载入配置后调用。
func (m *Matcher) Reset() {
	m.pending = m.pending[:0]
	m.node = m.table.root
}

// Pending 返回当前已按下但尚未完成的按键序列。
// 状态栏用它显示「C-a」这样的提示，让用户知道还在等下一键。
func (m *Matcher) Pending() []string {
	out := make([]string, len(m.pending))
	copy(out, m.pending)
	return out
}

// PendingSeq 返回待定按键序列的可读形式，空序列返回空串。
func (m *Matcher) PendingSeq() string { return strings.Join(m.pending, " ") }

// Push 送入一次按键（已归一化），返回解析结果。
// 命中时 Binding 有效；ResultPending 时应把 Binding 忽略并把 PendingSeq 显示给用户。
func (m *Matcher) Push(key string) (Result, Binding) {
	if key == "" {
		return ResultIgnored, Binding{}
	}

	// 中止键只在已有前缀待定时生效，否则它就是 Go 命令子树的首键。
	if key == AbortKey && len(m.pending) > 0 {
		m.Reset()
		return ResultMatch, Binding{Keys: AbortKey, Cmd: CmdCancelPrefix, Help: "取消当前按键前缀"}
	}

	next, ok := m.node.children[key]
	if !ok {
		// 未命中就丢弃整条序列：C-a 之后按下的无效键不该让 C-a 继续生效，
		// 否则一次误触会让后面几键都落到错误的上下文里。
		m.Reset()
		return ResultUnknown, Binding{}
	}
	m.node = next
	m.pending = append(m.pending, key)

	// 还有更长的序列可选时必须继续等：立刻执行会让更长的绑定永远无法命中。
	if next.isPrefix() {
		return ResultPending, Binding{}
	}
	binding := *next.direct
	m.Reset()
	return ResultMatch, binding
}
