// command.go — 命令分派：把 keymap 解析出的命令执行在 App 上。
// SPDX-License-Identifier: MIT

package editor

import (
	"fmt"
	"strings"

	"github.com/29anan29/Go-Studio/internal/keymap"
	"github.com/29anan29/Go-Studio/internal/view"
)

// handler 是一个命令的实现。receiver 已经由方法表达式绑定，
// 因此这里只需要再接收这条绑定的参数。
type handler func(a *App, b keymap.Binding)

// commands 是命令到实现的分派表。
//
// 加一个新功能只需两处改动：keymap 里加一条绑定，这里加一个处理函数。
// 键位表与实现因此不会散落成两半。
var commands = map[keymap.Command]handler{
	// 导航
	keymap.CmdMoveLeft:      (*App).moveLeft,
	keymap.CmdMoveRight:     (*App).moveRight,
	keymap.CmdMoveUp:        (*App).moveUp,
	keymap.CmdMoveDown:      (*App).moveDown,
	keymap.CmdMoveWordLeft:  (*App).moveWordLeft,
	keymap.CmdMoveWordRight: (*App).moveWordRight,
	keymap.CmdLineStart:     (*App).lineStart,
	keymap.CmdLineEnd:       (*App).lineEnd,
	keymap.CmdDocStart:      (*App).docStart,
	keymap.CmdDocEnd:        (*App).docEnd,
	keymap.CmdPageUp:        (*App).pageUp,
	keymap.CmdPageDown:      (*App).pageDown,

	// 编辑
	keymap.CmdInsertNewline: (*App).insertNewline,
	keymap.CmdInsertTab:     (*App).insertTab,
	keymap.CmdBackspace:     (*App).backspace,
	keymap.CmdDeleteRight:   (*App).deleteRight,
	keymap.CmdDeleteLine:    (*App).deleteLine,
	keymap.CmdJoinLine:      (*App).joinLine,
	keymap.CmdIndent:        (*App).indent,
	keymap.CmdUnindent:      (*App).unindent,
	keymap.CmdUndo:          (*App).undo,
	keymap.CmdRedo:          (*App).redo,
	keymap.CmdCopy:          (*App).copy,
	keymap.CmdPaste:         (*App).paste,

	// 文件与标签
	keymap.CmdSave:      (*App).save,
	keymap.CmdOpenFile:  (*App).openFile,
	keymap.CmdSaveAs:    (*App).saveAs,
	keymap.CmdCloseTab:  (*App).closeTab,
	keymap.CmdSelectTab: (*App).selectTab,
	keymap.CmdLastTab:   (*App).lastTab,
	keymap.CmdNextTab:   (*App).nextTab,
	keymap.CmdPrevTab:   (*App).prevTab,

	// 界面
	keymap.CmdFileTree:        (*App).notImplemented,
	keymap.CmdGlobalSearch:    (*App).notImplemented,
	keymap.CmdFindInBuffer:    (*App).findNextCmd,
	keymap.CmdFindNext:        (*App).findNextCmd,
	keymap.CmdFindPrev:        (*App).findPrevCmd,
	keymap.CmdReplace:         (*App).notImplemented,
	keymap.CmdGotoLine:        (*App).gotoLine,
	keymap.CmdToggleFold:      (*App).notImplemented,
	keymap.CmdSplitVertical:   (*App).notImplemented,
	keymap.CmdSplitHorizontal: (*App).notImplemented,
	keymap.CmdCommandPalette:  (*App).notImplemented,
	keymap.CmdTerminal:        (*App).notImplemented,
	keymap.CmdOutputPanel:     (*App).notImplemented,
	keymap.CmdProblemsPanel:   (*App).notImplemented,

	// 选择（spec 一.1，后续阶段实现）
	keymap.CmdSelectNext:        (*App).notImplemented,
	keymap.CmdColumnSelectUp:    (*App).notImplemented,
	keymap.CmdColumnSelectDown:  (*App).notImplemented,
	keymap.CmdColumnSelectLeft:  (*App).notImplemented,
	keymap.CmdColumnSelectRight: (*App).notImplemented,

	// Go 工具链（spec 二，后续阶段实现）
	keymap.CmdGoRun:         (*App).notImplemented,
	keymap.CmdGoBuild:       (*App).notImplemented,
	keymap.CmdGoTest:        (*App).notImplemented,
	keymap.CmdGoVet:         (*App).notImplemented,
	keymap.CmdGoEnv:         (*App).notImplemented,
	keymap.CmdGoList:        (*App).notImplemented,
	keymap.CmdGoVersion:     (*App).notImplemented,
	keymap.CmdGoModTidy:     (*App).notImplemented,
	keymap.CmdGoModDownload: (*App).notImplemented,
	keymap.CmdGoFormatFile:  (*App).notImplemented,
	keymap.CmdRestartLSP:    (*App).notImplemented,

	// 会话
	keymap.CmdQuit:         (*App).quit,
	keymap.CmdForceQuit:    (*App).forceQuit,
	keymap.CmdCancelPrefix: (*App).cancelPrefix,
}

// runCommand 执行一条命令。
func (a *App) runCommand(binding keymap.Binding) {
	run, ok := commands[binding.Cmd]
	if !ok {
		// 键位表里有、但没实现的命令：说明接线漏了，必须显眼地报出来，
		// 静默忽略会让「按了没反应」极难排查。
		a.setStatus("命令未实现：" + string(binding.Cmd))
		a.error("editor: command not implemented", "command", string(binding.Cmd))
		return
	}
	run(a, binding)
}

// ---- 导航 ----

func (a *App) moveLeft(_ keymap.Binding)      { a.doc.MoveLeft() }
func (a *App) moveRight(_ keymap.Binding)     { a.doc.MoveRight() }
func (a *App) moveUp(_ keymap.Binding)        { a.doc.MoveUp() }
func (a *App) moveDown(_ keymap.Binding)      { a.doc.MoveDown() }
func (a *App) moveWordLeft(_ keymap.Binding)  { a.doc.MoveWordLeft() }
func (a *App) moveWordRight(_ keymap.Binding) { a.doc.MoveWordRight() }
func (a *App) lineStart(_ keymap.Binding)     { a.doc.MoveLineStart() }
func (a *App) lineEnd(_ keymap.Binding)       { a.doc.MoveLineEnd() }
func (a *App) docStart(_ keymap.Binding)      { a.doc.MoveDocStart() }
func (a *App) docEnd(_ keymap.Binding)        { a.doc.MoveDocEnd() }

func (a *App) pageUp(_ keymap.Binding)   { a.doc.PageUp(a.textHeight()) }
func (a *App) pageDown(_ keymap.Binding) { a.doc.PageDown(a.textHeight()) }

// ---- 编辑 ----

// editable 先挡下只读文档，让所有编辑命令共用这一条提示。
func (a *App) editable() bool {
	if a.doc.Readonly() {
		a.setStatus("文件是只读的")
		return false
	}
	return true
}

func (a *App) insertNewline(_ keymap.Binding) {
	if a.editable() {
		a.doc.Newline()
	}
}

func (a *App) insertTab(_ keymap.Binding) {
	if a.editable() {
		a.doc.InsertTab(a.cfg.Editor.TabWidth)
	}
}

func (a *App) backspace(_ keymap.Binding) {
	if a.editable() {
		a.doc.Backspace()
	}
}

func (a *App) deleteRight(_ keymap.Binding) {
	if a.editable() {
		a.doc.DeleteRight()
	}
}

func (a *App) deleteLine(_ keymap.Binding) {
	if a.editable() {
		a.doc.DeleteLine()
	}
}

func (a *App) joinLine(_ keymap.Binding) {
	if a.editable() {
		a.doc.JoinLine()
	}
}

func (a *App) indent(_ keymap.Binding) {
	if a.editable() {
		a.doc.Indent(a.cfg.Editor.TabWidth)
	}
}

func (a *App) unindent(_ keymap.Binding) {
	if a.editable() {
		a.doc.Unindent(a.cfg.Editor.TabWidth)
	}
}

func (a *App) undo(_ keymap.Binding) {
	if a.doc.Undo() {
		a.setStatus("已撤销")
	}
}

func (a *App) redo(_ keymap.Binding) {
	if a.doc.Redo() {
		a.setStatus("已重做")
	}
}

// clipboard 是进程内的剪贴板。
//
// 这里不做系统剪贴板：接入 X11/Wayland/macOS 各有一套协议，
// 而 spec 里的 C-S c/v 只要求「复制粘贴能工作」。进程内剪贴板让功能先跑起来，
// 接系统剪贴板是后续独立的一步，不与命令分派搅在一起。
var clipboard []byte

func (a *App) copy(_ keymap.Binding) {
	if a.doc.Readonly() {
		a.setStatus("文件是只读的")
		return
	}
	clipboard = a.doc.Buffer().Text()
	a.setStatus(fmt.Sprintf("已复制 %d 字节", len(clipboard)))
}

func (a *App) paste(_ keymap.Binding) {
	// 只读要先挡下来：否则「剪贴板是空的」会覆盖掉真正的失败原因，
	// 用户会以为问题出在剪贴板上。
	if !a.editable() {
		return
	}
	if len(clipboard) == 0 {
		a.setStatus("剪贴板是空的")
		return
	}
	a.doc.InsertText(string(clipboard))
	a.setStatus(fmt.Sprintf("已粘贴 %d 字节", len(clipboard)))
}

// ---- 文件与标签 ----

func (a *App) save(_ keymap.Binding) {
	if a.doc.Readonly() {
		a.setStatus("文件是只读的")
		return
	}
	if err := a.doc.Save(); err != nil {
		a.setStatus("保存失败：" + err.Error())
		a.error("editor: save failed", "path", a.doc.Path(), "error", err.Error())
		return
	}
	a.setStatus("已保存 " + displayName(a.doc.Path()))
}

// ---- 会话 ----

func (a *App) quit(_ keymap.Binding) {
	if a.doc.Dirty() {
		// 拒绝退出时必须把「接下来怎么办」一起说出来。
		// 只说「未退出」的话，用户只能反复按 C-a q，
		// 或者干脆以为编辑器坏了。明确给出保存与强退两个出口。
		a.setStatus("有未保存改动：C-a w 保存，或 C-a x 强制退出")
		return
	}
	a.shouldQuit = true
}

// forceQuit 无条件退出，未保存的改动会被丢弃。
//
// 它是刻意存在的：有未保存改动时 C-a q 会拒绝，
// 若不给一条强退路径，用户就只能在「保存」和「被卡住」之间二选一。
// 危险操作要有，但必须与其他命令一样显式地摆在那里，而不是藏起来。
func (a *App) forceQuit(_ keymap.Binding) {
	if a.doc.Dirty() {
		a.error("editor: force quit with unsaved changes", "path", a.doc.Path())
	}
	a.shouldQuit = true
}

func (a *App) cancelPrefix(_ keymap.Binding) {
	a.matcher.Reset()
	a.setStatus("已取消按键前缀")
}

// ---- 未实现 ----

// notImplemented 是尚未落地的命令的统一处理。
//
// 明确告诉用户「这个键位存在但还没做」，比静默返回强得多：
// 静默返回会被当成按键坏了，或者被当成没绑键。
func (a *App) notImplemented(b keymap.Binding) {
	a.setStatus(fmt.Sprintf("「%s」尚未实现：%s", b.Keys, b.Cmd))
	a.debug("editor: command not implemented", "command", string(b.Cmd), "keys", b.Keys)
}

// ---- 状态栏 ----

// positionWidth 是状态栏位置字段的固定宽度。
//
// 位置必须占固定宽度，否则每移动一次光标状态栏的整体长度就变一次
// （"9:1" 变 "10:1" 再变回），渲染器的增量更新会因为整行错位而留下残字，
// 屏幕上会出现 "1::1" 这种东西。同时整个状态栏也会跟着抖。
const positionWidth = 8

// statusLine 生成状态栏内容。
//
// 从左到右：文件名与脏标记、光标位置、提示文字。提示优先显示，
// 因为它是刚刚发生的事，比位置更重要。
func (a *App) statusLine() string {
	name := displayName(a.doc.Path())
	if name == "" {
		name = "[未命名]"
	}
	if a.doc.Dirty() {
		name += " *"
	}
	if a.doc.Readonly() {
		name += " [只读]"
	}
	// 识别到的语言显示在文件名后面。用户最常问的问题是
	// 「为什么这份 .conf 没高亮」，答案基本总是「没认出来」，
	// 把结果直接摆出来比让用户去猜省事。
	if a.hlLang != "" {
		name += " " + a.hlLang
	}

	cursor := a.doc.Cursor()
	// 状态栏里的行列号从 1 开始，与行号显示保持一致；
	// 右对齐到固定宽度，避免光标移动时整行长度变化。
	position := fmt.Sprintf("%d:%d", cursor.Line+1, cursor.Col+1)
	padded := position
	if pad := positionWidth - len(position); pad > 0 {
		padded = strings.Repeat(" ", pad) + position
	}

	hint := a.currentStatus()
	if hint == "" {
		hint = a.mode
	}

	return fit(fmt.Sprintf(" %s  %s  %s", name, padded, hint), a.width)
}

// currentStatus 返回尚未过期的提示文字。
func (a *App) currentStatus() string {
	if a.status == "" {
		return ""
	}
	if a.now().After(a.statusUntil) {
		return ""
	}
	return a.status
}

// setStatus 设置一条带超时的状态栏提示。
func (a *App) setStatus(text string) {
	a.status = text
	a.statusUntil = a.now().Add(statusTimeout)
}

// theme 按配置选主题。
func (a *App) theme() view.Theme {
	if !a.cfg.UI.TrueColor && a.backend != nil && !a.backend.Caps().TrueColor {
		return view.PlainTheme()
	}
	if a.hl != nil {
		return view.SyntaxTheme()
	}
	return view.DefaultTheme()
}

// displayName 返回路径的末段，用于状态栏显示。
func displayName(path string) string {
	if path == "" {
		return ""
	}
	parts := strings.Split(strings.ReplaceAll(path, "\\", "/"), "/")
	return parts[len(parts)-1]
}

// fit 把文字截断到指定列数，保证状态栏永不折行。
//
// 裁剪必须按显示宽度而不是字符个数：状态栏里有中文，
// 一个汉字占两列，按字符个数裁会让整行超出终端宽度、终端随即折行。
// 宽度算法集中在 view 包，这里不重复实现。
func fit(text string, width int) string { return view.Fit(text, width) }
