// keys.go — 从键位表生成键位速查表。
// SPDX-License-Identifier: MIT

// 这个程序不手抄键位，而是从 internal/keymap 的表里生成 docs/keybindings.md。
// 手抄的文档一定会和实现漂移，而键位漂移的后果是「按了没反应」，
// 用户既查不到、也猜不出原因。
//
// 用法：go run ./scripts/keys
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/29anan29/Go-Studio/internal/keymap"
)

// modifierOrder 是修饰键在速查表里的书写顺序，沿用 Emacs 惯例：
// C-、M-、S-。顺序固定，读起来才不会因人而异。
var modifierOrder = []struct{ internal, shown string }{
	{"ctrl+", "C-"},
	{"alt+", "M-"},
	{"meta+", "M-"},
	{"shift+", "S-"},
}

// pretty 把内部按键名换成 Emacs 写法的按键序列。
//
// 输入像 "ctrl+shift+c"、"alt+1"、"ctrl+<home>"、"ctrl+g meta+t"，
// 输出像 "C-S c"、"M-1"、"C-<home>"、"C-g M-t"。
func pretty(keys string) string {
	groups := strings.Fields(keys)
	out := make([]string, 0, len(groups))

	for _, group := range groups {
		rest := group
		var prefix strings.Builder
		// 修饰键要按固定顺序重排：内部的 "shift+alt+" 要写成 "M-S "。
		for {
			matched := false
			for _, mod := range modifierOrder {
				if strings.HasPrefix(rest, mod.internal) {
					prefix.WriteString(mod.shown)
					rest = rest[len(mod.internal):]
					matched = true
					break
				}
			}
			if !matched {
				break
			}
		}
		// 空格键写 C-Space，避免读者以为是「C 加空格键名」。
		if rest == "space" && prefix.Len() > 0 {
			rest = "Space"
		}
		out = append(out, prefix.String()+rest)
	}
	return strings.Join(out, " ")
}

func main() {
	table, err := keymap.Lookup(keymap.EmacsName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "keys: %v\n", err)
		os.Exit(1)
	}
	bindings := table.Bindings()

	// 按「是不是和弦」分组，和弦在前。
	var chords, singles []keymap.Binding
	for _, b := range bindings {
		if strings.Contains(b.Keys, " ") {
			chords = append(chords, b)
		} else {
			singles = append(singles, b)
		}
	}
	sort.Slice(chords, func(i, j int) bool { return chords[i].Keys < chords[j].Keys })
	sort.Slice(singles, func(i, j int) bool { return singles[i].Keys < singles[j].Keys })

	var b strings.Builder
	b.WriteString("<!-- 本文件由 `go run ./scripts/keys` 生成，请勿手改。 -->\n\n")
	b.WriteString("# 键位速查表\n\n")
	b.WriteString(fmt.Sprintf("键位方案：`%s`，共 %d 条绑定。\n\n",
		table.Name, len(bindings)))

	b.WriteString("## 组合键（和弦）\n\n| 按键 | 作用 |\n|---|---|\n")
	for _, x := range chords {
		fmt.Fprintf(&b, "| `%s` | %s |\n", pretty(x.Keys), x.Help)
	}

	b.WriteString("\n## 单键\n\n| 按键 | 作用 |\n|---|---|\n")
	for _, x := range singles {
		fmt.Fprintf(&b, "| `%s` | %s |\n", pretty(x.Keys), x.Help)
	}

	// 收尾说明用普通字符串拼：Markdown 里的反引号与 Go 的原始字符串字面量冲突。
	tail := []string{
		"",
		"## 约定",
		"",
		"- `C-` 表示 Ctrl，`M-` 表示 Alt(Meta)，`S-` 表示 Shift。",
		"- 字母直接输入即可；**没有裸字母被绑到命令上**，所以任何字母都能正常打出来。",
		"- 标着「尚未实现」的键位按下会在状态栏明确提示，不会静默无反应。",
		"",
		"## 已知限制",
		"",
		"- `C-S c`（复制）与 `C-S v`（粘贴）需要终端支持 kitty 键盘协议才能区分。",
		"  传统 xterm 会把 `C-S c` 发成 `C-c`、`C-S v` 发成 `C-v`，",
		"  所以在那些终端上这两条快捷键不生效。这是终端的限制，不是绑定写错了。",
		"  kitty、ghostty、wezterm、foot、rio 等现代终端上可以正常工作。",
		"- `C-S c` 当前复制的是整个文件，粘贴会追加到光标处。",
		"",
	}
	b.WriteString(strings.Join(tail, "\n"))

	path := "docs/keybindings.md"
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "keys: 写 %s 失败: %v\n", path, err)
		os.Exit(1)
	}
	fmt.Printf("已生成 %s（%d 条绑定）\n", path, len(bindings))
}
