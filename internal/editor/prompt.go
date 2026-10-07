// prompt.go — 单行输入浮层：打开文件、另存为、跳转到行、查找都要先收集用户输入。
// SPDX-License-Identifier: MIT

package editor

import (
	"strings"

	"github.com/29anan29/Go-Studio/internal/screen"
	"github.com/29anan29/Go-Studio/internal/view"
)

// promptKind 区分浮层收集到的输入要拿来做什么。
type promptKind uint8

const (
	// promptOpenFile 收集要打开的文件或目录路径。
	promptOpenFile promptKind = iota
	// promptSaveAs 收集另存为的目标路径。
	promptSaveAs
	// promptGotoLine 收集行号。
	promptGotoLine
	// promptFind 收集要查找的字符串。
	promptFind
)

// label 返回浮层提示语。提示语必须说清这里要什么，
// 否则用户面对一个空输入框无从判断该打什么。
func (k promptKind) label() string {
	switch k {
	case promptOpenFile:
		return "打开文件"
	case promptSaveAs:
		return "另存为"
	case promptGotoLine:
		return "跳转到行"
	case promptFind:
		return "查找"
	default:
		return "输入"
	}
}

// prompt 是单行输入浮层。
//
// 浮层打开时按键不再走键位表，全部当作输入处理；
// 只有 <enter> 与 <esc> 保留原本的含义。这让「输入路径」这类交互
// 不会与编辑命令抢按键：用户打到的每个字符都进输入框，而不是触发命令。
type prompt struct {
	kind  promptKind
	input []rune
	// initial 是打开浮层时的预填内容。目录浏览会反复用同一个前缀，
	// 预填能让用户直接追加，而不必每次重打。
	initial string
}

// newPrompt 造一个浮层，prefill 非空时预填。
func newPrompt(kind promptKind, prefill string) *prompt {
	return &prompt{kind: kind, initial: prefill, input: []rune(prefill)}
}

// text 返回输入框当前的内容。
func (p *prompt) text() string { return string(p.input) }

// handleKey 处理一次按键，返回是否已经消费了该键。
//
// 恒返回 true：浮层打开期间编辑器不该再处理任何按键，
// 否则方向键会去移动文档光标、<enter> 会插入换行，
// 用户只想打路径却把文档改掉了。
func (p *prompt) handleKey(key string) bool {
	switch key {
	case "<esc>":
		// 清空并由调用方关闭；按键在这里被吃掉。
		p.input = p.input[:0]
	case "<enter>":
		// 由调用方读取 text() 后关闭。
	case "<backspace>":
		if n := len(p.input); n > 0 {
			p.input = p.input[:n-1]
		}
	case "<delete>", "<left>", "<right>", "<home>", "<end>":
		// 输入框内的光标移动还没实现（needs 左移、删除光标后内容）。
		// 先吞掉这些键，让它们不会穿透到文档上。
	case screen.SpaceKey:
		p.input = append(p.input, ' ')
	default:
		if r, ok := screen.PrintableRune(key); ok {
			p.input = append(p.input, r)
		}
	}
	return true
}

// render 渲染浮层那一行，返回行内容与光标位置。
//
// 浮层画在状态栏的位置上，不额外占行：终端尺寸因此不会因为开浮层而变化，
// 也就不会出现「打开浮层后整个视口重排」这种让人晕的现象。
//
// 返回的行宽必须精确等于 width，否则终端会折行、整帧布局跟着崩。
func (p *prompt) render(theme view.Theme, width int) (string, *screen.CursorSpec) {
	head := p.kind.label() + ": " + string(p.input)
	cursorX := view.TextWidth(head)

	// 光标落在输入末尾。留一格给光标块，否则光标会被挤出可视范围。
	line := theme.CurrentLine + padToWidth(view.Fit(head, maxInt(width-1, 0)), width) + "\x1b[0m"

	if cursorX >= width {
		// 输入比视口还长时光标会被挤出可视范围，此时不报坐标：
		// 报一个越界的坐标会让终端把光标写到别处去。
		return line, nil
	}
	return line, &screen.CursorSpec{X: cursorX, Y: 0, Shape: screen.CursorBlock, Blink: true}
}

// confirmKind 返回这次确认的输入要拿来做什么。
func (p *prompt) confirmKind() promptKind { return p.kind }

// resolvePath 把用户输入的路径规整一下：去掉首尾空白与成对引号。
//
// 用户常常从终端里复制路径过来，路径两端会带上引号或空格，
// 这些字符必须去掉才能真正打开文件。
func resolvePath(raw string) string {
	path := strings.TrimSpace(raw)
	if len(path) >= 2 {
		first, last := path[0], path[len(path)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			path = path[1 : len(path)-1]
		}
	}
	return strings.TrimSpace(path)
}
