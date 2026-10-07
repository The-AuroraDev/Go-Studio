// tabs.go — 多标签页：顶部标签栏与标签切换。
// SPDX-License-Identifier: MIT

package editor

import (
	"strings"

	"github.com/29anan29/Go-Studio/internal/document"
	"github.com/29anan29/Go-Studio/internal/view"
)

// tabs 管理已打开的文档。
//
// 它只管「有哪些标签、当前是哪个」，不碰渲染也不碰命令：
// 渲染在 view/editor 层，命令在 command.go，两者都通过这里拿当前文档。
type tabs struct {
	docs   []*document.Document
	active int
}

// newTabs 用单个初始文档构造。
func newTabs(doc *document.Document) *tabs {
	return &tabs{docs: []*document.Document{doc}}
}

// current 返回当前文档，永远非空：至少会有一个空文档兜底。
func (t *tabs) current() *document.Document {
	if t.active < 0 || t.active >= len(t.docs) {
		return t.docs[0]
	}
	return t.docs[t.active]
}

// count 返回标签个数。
func (t *tabs) count() int { return len(t.docs) }

// open 在末尾打开一个文档并切过去。
//
// 已经打开过的路径不再重复打开：同一个文件开两个标签，
// 两份拷贝会各自被编辑，保存时互相覆盖，用户几乎无法排查这种冲突。
func (t *tabs) open(doc *document.Document) {
	if existing := t.indexOf(doc.Path()); existing >= 0 && doc.Path() != "" {
		t.active = existing
		return
	}
	// 开在一个「空的、未命名、未改动」的文档上时直接替换掉它：
	// 否则刚启动就打开文件会先看到两个标签，其中一个永远是空的。
	if t.canReplaceActive() {
		t.docs[t.active] = doc
		return
	}
	t.docs = append(t.docs, doc)
	t.active = len(t.docs) - 1
}

// canReplaceActive 报告当前标签是不是「还没用过」的空白文档。
func (t *tabs) canReplaceActive() bool {
	if len(t.docs) != 1 {
		return false
	}
	doc := t.docs[0]
	// Text() 对空文档返回的是「非 nil 的空切片」，必须按长度判断。
	return doc.Path() == "" && len(doc.Text()) == 0 && !doc.Dirty()
}

// indexOf 返回已打开文档中路径匹配的下标，没有则返回 -1。
func (t *tabs) indexOf(path string) int {
	if path == "" {
		return -1
	}
	for i, doc := range t.docs {
		if doc.Path() == path {
			return i
		}
	}
	return -1
}

// selectN 切到第 n 个标签，n 从 1 开始，与状态栏显示一致。
func (t *tabs) selectN(n int) bool {
	if n < 1 || n > len(t.docs) {
		return false
	}
	t.active = n - 1
	return true
}

// selectLast 切到最后一个标签，对应 spec 的 M-9。
func (t *tabs) selectLast() {
	t.active = maxInt(len(t.docs)-1, 0)
}

// close 关闭当前标签，返回关闭后的当前文档。
// 关闭最后一个标签时会补一个空文档，保证始终有文档可编辑。
func (t *tabs) close() (*document.Document, bool) {
	if len(t.docs) == 0 {
		return nil, false
	}
	t.docs = append(t.docs[:t.active], t.docs[t.active+1:]...)
	if len(t.docs) == 0 {
		t.docs = append(t.docs, document.New())
		t.active = 0
		return t.docs[0], true
	}
	// 关闭的是最后一个标签时，前一个接管；否则当前位置原地接管。
	if t.active >= len(t.docs) {
		t.active = len(t.docs) - 1
	}
	return t.docs[t.active], true
}

// dirtyCount 返回有未保存改动的标签数，用于状态栏提示。
func (t *tabs) dirtyCount() int {
	n := 0
	for _, doc := range t.docs {
		if doc.Dirty() {
			n++
		}
	}
	return n
}

// label 返回第 i 个标签的显示文字。
func (t *tabs) label(i int) string {
	if i < 0 || i >= len(t.docs) {
		return ""
	}
	name := displayName(t.docs[i].Path())
	if name == "" {
		name = "[未命名]"
	}
	if t.docs[i].Dirty() {
		name += "*"
	}
	return name
}

// renderTabs 渲染顶部标签栏，返回整行内容。
//
// 只有一个标签时不画标签栏：那一行没有信息量，
// 白占一行高度还不如给文本区。
//
// 宽度必须精确等于 width。这里先按「不含 ANSI 的纯文本」裁剪与补齐，
// 最后再给当前标签套上样式：按带转义序列的串去量宽度是不成立的——
// 转义序列里的 "[38;5;236m" 全是可见宽度的可打印字符，会把测量结果抬高十几列。
func (t *tabs) renderTabs(theme view.Theme, width int) string {
	if len(t.docs) <= 1 || width <= 0 {
		return ""
	}

	var plain strings.Builder
	// 先算出这一行在纯文本下的样子。
	for i := range t.docs {
		if view.TextWidth(plain.String()) >= width {
			break
		}
		if i > 0 {
			plain.WriteString(" ")
		}
		plain.WriteString(" " + t.label(i) + " ")
	}

	text := padToWidth(plain.String(), width)
	if text == "" {
		return ""
	}

	// 再把当前标签那段套上样式。反转义序列不改变可见宽度，
	// 因此这不会破坏刚算好的宽度。
	label := " " + t.label(t.active) + " "
	at := strings.Index(text, strings.TrimSpace(label))
	if at < 0 {
		return text
	}
	start := at + 1
	end := start + view.TextWidth(strings.TrimSpace(label))
	return text[:start] + theme.CurrentLine + text[start:end] + "\x1b[0m" + text[end:]
}

// padToWidth 把纯文本补齐或裁剪到指定显示宽度。
//
// 裁剪交给 view.Fit：它不会切出半个宽字符，而半个宽字符会让整行错位。
// 但 Fit 可能因为放不下一个宽字符而返回略短的串（宁可少一列也不切字），
// 所以这里在裁剪之后还要补齐，保证宽度精确——
// 少一列会让终端残留上一帧的残影，多一列会让终端折行。
func padToWidth(text string, width int) string {
	if width <= 0 {
		return ""
	}
	// 用 VisibleWidth 而不是 TextWidth：text 可能已经带了 ANSI 样式，
	// 转义序列里的可打印字符会被 TextWidth 算成可见列，
	// 那样补齐量会算错十几列，帧宽立刻不准。
	if view.VisibleWidth(text) == width {
		return text
	}
	if view.VisibleWidth(text) > width {
		text = view.Fit(stripStyles(text), width)
	}
	if pad := width - view.VisibleWidth(text); pad > 0 {
		return text + strings.Repeat(" ", pad)
	}
	return text
}

// stripStyles 去掉 ANSI 样式序列，只留可见字符。
func stripStyles(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != 0x1b {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}
