// keymap.go — 键位引擎：把归一化后的按键序列解析成命令。
// SPDX-License-Identifier: MIT

// Package keymap 把「按了什么键」翻译成「要执行哪个命令」。
//
// 键位表是一棵前缀树。节点可以既是完整绑定、又是更长序列的前缀，
// 这正是 Emacs 风格键位的形状：C-a r 是「撤销」，而 C-a 本身也可以有含义。
//
// 本包是纯函数式的，不碰终端、不碰文件系统、不持有可变全局状态。
// 按键名统一经 screen.NormalizeKeystroke 归一化，因此修饰键的书写顺序
// 不影响匹配：alt+ctrl+a 与 ctrl+alt+a 是同一个键。
package keymap

import (
	"fmt"
	"sort"
	"strings"

	"github.com/29anan29/Go-Studio/internal/screen"
)

// Arg 是绑定携带的可选参数，通常是命令的编号或模式名。
type Arg string

// Binding 是一条完整的「按键序列 → 命令」绑定。
type Binding struct {
	// Keys 是按键序列，元素之间用空格分隔，例如 "ctrl+a r"。
	// 构造 Table 时会逐个元素归一化，因此书写顺序任意。
	Keys string
	// Cmd 是按下该序列后要执行的命令。
	Cmd Command
	// Arg 是传给命令的参数，可为空。
	Arg Arg
	// Help 是状态栏与命令面板显示的一行说明。
	Help string
}

// node 是前缀树的一个节点。
type node struct {
	// direct 是该节点自身作为完整按键序列时的绑定，nil 表示按下后必须继续按键。
	direct *Binding
	// children 是更长的按键序列，键为归一化后的单个按键。
	children map[string]*node
}

// isPrefix 报告该节点是否可以继续按键，即它是否还有更长的序列。
func (n *node) isPrefix() bool { return len(n.children) > 0 }

// Table 是一套完整的键位方案。
type Table struct {
	// Name 是方案名，对应配置里的 general.keymap。
	Name string
	// root 是按键序列的根节点。
	root *node
	// bindings 按 Keys 字典序保存全部绑定。
	bindings []Binding
}

// EmacsName 是 Emacs 风格键位方案的名称，也是配置的默认值。
const EmacsName = "emacs"

// New 构造键位方案。
//
// 按键序列会被归一化，因此调用方不必自己保证修饰键顺序。
// 表里出现同一序列重复绑定、或某个序列既是终点又是别处的中间段时，
// 直接返回错误：这类问题若留到运行时，表现为「按了没反应」，排查成本极高。
func New(name string, bindings []Binding) (*Table, error) {
	if name == "" {
		return nil, fmt.Errorf("keymap: 方案名为空")
	}
	root := &node{children: make(map[string]*node)}
	normalized := make([]Binding, 0, len(bindings))
	for _, binding := range bindings {
		seq, err := normalizeSeq(binding.Keys)
		if err != nil {
			return nil, fmt.Errorf("keymap %s: %w", name, err)
		}
		if binding.Cmd == "" {
			return nil, fmt.Errorf("keymap %s: 按键序列 %q 没有对应命令", name, seq)
		}
		binding.Keys = seq
		if err := root.insert(binding); err != nil {
			return nil, fmt.Errorf("keymap %s: %w", name, err)
		}
		normalized = append(normalized, binding)
	}
	if err := root.validate(""); err != nil {
		return nil, fmt.Errorf("keymap %s: %w", name, err)
	}
	sort.Slice(normalized, func(i, j int) bool { return normalized[i].Keys < normalized[j].Keys })
	return &Table{Name: name, root: root, bindings: normalized}, nil
}

// normalizeSeq 归一化整条按键序列。空序列、空元素、只有修饰键的元素都算错误。
func normalizeSeq(seq string) (string, error) {
	fields := strings.Fields(seq)
	if len(fields) == 0 {
		return "", fmt.Errorf("按键序列为空")
	}
	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		key := screen.NormalizeKeystroke(field)
		if key == "" {
			// NormalizeKeystroke 对「只有修饰键」的输入返回空串，
			// 这类按键无法出现在绑定表里。
			return "", fmt.Errorf("按键序列 %q 中的 %q 不是合法按键", seq, field)
		}
		parts = append(parts, key)
	}
	return strings.Join(parts, " "), nil
}

// insert 把一条绑定插入树中，遇到冲突时返回错误。
func (root *node) insert(binding Binding) error {
	seq := strings.Fields(binding.Keys)
	current := root
	for _, key := range seq[:len(seq)-1] {
		child, ok := current.children[key]
		if !ok {
			child = &node{children: make(map[string]*node)}
			current.children[key] = child
		}
		current = child
	}

	last := seq[len(seq)-1]
	leaf, ok := current.children[last]
	if !ok {
		leaf = &node{children: make(map[string]*node)}
		current.children[last] = leaf
	}
	if leaf.direct != nil {
		return fmt.Errorf("按键序列 %q 重复绑定（已有命令 %q）", binding.Keys, leaf.direct.Cmd)
	}
	stored := binding
	leaf.direct = &stored
	return nil
}

// validate 遍历整棵树，拒绝「既是终点又是前缀」的节点。
//
// 这种节点的行为依赖 Matcher 的消歧策略：按下它必须先等下一键才知道该不该执行，
// 而一旦消歧失败，这次按键就白按了。留着这种绑定最糟的失败模式是「按了没反应」，
// 所以在构造期就拒绝。检查放在插入之后统一做，因此与绑定书写顺序无关。
func (n *node) validate(prefix string) error {
	if n.direct != nil && n.isPrefix() {
		return fmt.Errorf("按键序列 %q 既是终点又是更长序列的前缀，行为有歧义", prefix)
	}
	for key, child := range n.children {
		seq := key
		if prefix != "" {
			seq = prefix + " " + key
		}
		if err := child.validate(seq); err != nil {
			return err
		}
	}
	return nil
}

// Bindings 返回全部绑定，按按键序列字典序排列。
// 命令面板与「键位帮助」界面需要这份清单。
func (t *Table) Bindings() []Binding {
	out := make([]Binding, len(t.bindings))
	copy(out, t.bindings)
	return out
}

// Lookup 返回按键序列对应的绑定，供「键位帮助」与反向查询使用。
// seq 可以是单个按键，也可以是空格分隔的多键序列。
func (t *Table) Lookup(seq string) (Binding, bool) {
	want, err := normalizeSeq(seq)
	if err != nil {
		return Binding{}, false
	}
	binding := t.root.lookup(strings.Fields(want))
	if binding == nil {
		return Binding{}, false
	}
	return *binding, true
}

// lookup 沿按键序列下行，返回终点的 direct 绑定。
func (root *node) lookup(seq []string) *Binding {
	current := root
	for _, key := range seq {
		child, ok := current.children[key]
		if !ok {
			return nil
		}
		current = child
	}
	return current.direct
}
