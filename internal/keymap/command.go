// command.go — 命令标识：键位解析的结果，也是编辑器分派执行的依据。
// SPDX-License-Identifier: MIT

package keymap

// Command 是一个可执行命令的标识。
//
// 它是字符串而不是整数或函数指针：命令需要跨包传递（日志、状态栏提示、
// 命令面板搜索都要用），而函数指针没法比较、没法存进配置、没法在测试里断言。
// 真正的执行逻辑留在 editor 包里，keymap 只负责「哪个键对应哪个命令」。
type Command string

// String 返回命令名，便于日志与状态栏显示。
func (c Command) String() string { return string(c) }

// 导航类命令：只移动光标，不改动文本。
const (
	CmdMoveLeft      Command = "move-left"
	CmdMoveRight     Command = "move-right"
	CmdMoveUp        Command = "move-up"
	CmdMoveDown      Command = "move-down"
	CmdMoveWordLeft  Command = "move-word-left"
	CmdMoveWordRight Command = "move-word-right"
	CmdLineStart     Command = "line-start"
	CmdLineEnd       Command = "line-end"
	CmdDocStart      Command = "doc-start"
	CmdDocEnd        Command = "doc-end"
	CmdPageUp        Command = "page-up"
	CmdPageDown      Command = "page-down"
	// CmdSelectNext 对应 spec 的「双击按 C-d 多选」，在下一个出现处追加一个选区。
	CmdSelectNext Command = "select-next"
	// CmdColumnSelectLeft 等四个对应 spec 的「S-M 上下左右键」列选择。
	CmdColumnSelectLeft  Command = "column-select-left"
	CmdColumnSelectRight Command = "column-select-right"
	CmdColumnSelectUp    Command = "column-select-up"
	CmdColumnSelectDown  Command = "column-select-down"
)

// 编辑类命令：改动文本内容。
const (
	CmdInsertNewline Command = "insert-newline"
	CmdInsertTab     Command = "insert-tab"
	CmdBackspace     Command = "backspace"
	CmdDeleteRight   Command = "delete-right"
	CmdDeleteLine    Command = "delete-line"
	CmdJoinLine      Command = "join-line"
	CmdIndent        Command = "indent"
	CmdUnindent      Command = "unindent"
	CmdCopy          Command = "copy"
	CmdPaste         Command = "paste"
	CmdUndo          Command = "undo"
	CmdRedo          Command = "redo"
)

// 文件与界面类命令。
const (
	CmdOpenFile         Command = "open-file"
	CmdSave             Command = "save"
	CmdSaveAs           Command = "save-as"
	CmdCloseTab         Command = "close-tab"
	CmdSelectTab        Command = "select-tab"
	CmdLastTab          Command = "last-tab"
	CmdFileTree         Command = "file-tree"
	CmdGlobalSearch     Command = "global-search"
	CmdFindInBuffer     Command = "find-in-buffer"
	CmdReplace          Command = "replace"
	CmdGotoLine         Command = "goto-line"
	CmdToggleFold       Command = "toggle-fold"
	CmdSplitVertical    Command = "split-vertical"
	CmdSplitHorizontal  Command = "split-horizontal"
	CmdCommandPalette   Command = "command-palette"
	CmdTerminal         Command = "terminal"
	CmdOutputPanel      Command = "output-panel"
	CmdProblemsPanel    Command = "problems-panel"
	CmdToggleIndentMode Command = "toggle-indent-mode"
)

// Go 工具链类命令：spec.md 第二节。
const (
	CmdGoRun         Command = "go-run"
	CmdGoBuild       Command = "go-build"
	CmdGoTest        Command = "go-test"
	CmdGoVet         Command = "go-vet"
	CmdGoModTidy     Command = "go-mod-tidy"
	CmdGoModDownload Command = "go-mod-download"
	CmdGoEnv         Command = "go-env"
	CmdGoList        Command = "go-list"
	CmdGoVersion     Command = "go-version"
	CmdGoFormatFile  Command = "go-format-file"
	CmdRestartLSP    Command = "lsp-restart"
)

// 会话控制命令。
const (
	CmdQuit         Command = "quit"
	CmdForceQuit    Command = "force-quit"
	CmdCancelPrefix Command = "cancel-prefix"
	CmdCancelAll    Command = "cancel-all"
	CmdRescan       Command = "rescan"
)
