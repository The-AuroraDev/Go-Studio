// emacs.go — Emacs 风格键位表，逐条对应 docs/spec.md。
// SPDX-License-Identifier: MIT

package keymap

import (
	"fmt"
	"sort"
)

// CtrlPlus 把 C- 前缀写成字面量，让下面的绑定读起来与 spec.md 一致。
const ctrlPlus = "ctrl+"

// Emacs 返回 Emacs 风格键位表。
//
// 每条绑定都注明了 docs/spec.md 的出处。表里只有三棵子树：
// C-a（编辑器通用）、C-g（Go 工具链）、C-S（系统剪贴板），
// 其余都是单键或单键加参数。
func Emacs() (*Table, error) {
	return New(EmacsName, emacsBindings())
}

// emacsBindings 是 Emacs 风格键位表的全部内容。
func emacsBindings() []Binding {
	return []Binding{
		// ---- 文本编辑：撤销重做（spec 一.1）----
		{Keys: ctrlPlus + "a r", Cmd: CmdUndo, Help: "撤销"},
		{Keys: ctrlPlus + "a y", Cmd: CmdRedo, Help: "重做"},

		// ---- 文本编辑：复制粘贴（spec 一.1）----
		//
		// 注意：传统 xterm 不区分 Ctrl 与 Ctrl+Shift，Ctrl+Shift+C 会以
		// ctrl+c 的形式到达，因此这两条绑定需要终端支持 kitty 键盘协议。
		// 表里刻意不绑 ctrl+c，避免在不支持的终端上误触发。
		//
		// 写法必须是 ctrl+shift+c：写成 "ctrl+S c" 会被解析成「ctrl 加字母 S」，
		// 那是另一个键，永远不会被终端上报成这个序列。
		{Keys: ctrlPlus + "shift+c", Cmd: CmdCopy, Help: "复制"},
		{Keys: ctrlPlus + "shift+v", Cmd: CmdPaste, Help: "粘贴"},

		// ---- 文本编辑：多选与列选择（spec 一.1）----
		//
		// spec 原文「双击按 C-d 多选」：C-d 在下一个相同词处追加选区，
		// 连续按可一路选下去。
		{Keys: ctrlPlus + "d", Cmd: CmdSelectNext, Help: "在下一个相同词处追加选区"},
		// spec 原文「S-M 上下左右键」，即 Shift+Alt+方向键。
		{Keys: "alt+shift+<up>", Cmd: CmdColumnSelectUp, Help: "列选择向上"},
		{Keys: "alt+shift+<down>", Cmd: CmdColumnSelectDown, Help: "列选择向下"},
		{Keys: "alt+shift+<left>", Cmd: CmdColumnSelectLeft, Help: "列选择向左"},
		{Keys: "alt+shift+<right>", Cmd: CmdColumnSelectRight, Help: "列选择向右"},

		// ---- 文件与项目界面（spec 一.2）----
		{Keys: ctrlPlus + "a f", Cmd: CmdOpenFile, Help: "打开文件"},
		// spec 原文「文件树(C- <Spacse>)」，Spacse 是 Space 的笔误。
		{Keys: ctrlPlus + "space", Cmd: CmdFileTree, Help: "文件树"},
		{Keys: ctrlPlus + "a s", Cmd: CmdGlobalSearch, Help: "全局搜索"},
		{Keys: ctrlPlus + "a 0", Cmd: CmdSplitVertical, Help: "上下分屏"},
		{Keys: ctrlPlus + "a 1", Cmd: CmdSplitHorizontal, Help: "左右分屏"},

		// 多标签页（spec 一.2）：M-1 到 M-8 切到第 1 到第 8 个标签，
		// M-9 按 spec 的约定跳到最后一个。
		{Keys: "alt+1", Cmd: CmdSelectTab, Arg: Arg("1"), Help: "切到第 1 个标签"},
		{Keys: "alt+2", Cmd: CmdSelectTab, Arg: Arg("2"), Help: "切到第 2 个标签"},
		{Keys: "alt+3", Cmd: CmdSelectTab, Arg: Arg("3"), Help: "切到第 3 个标签"},
		{Keys: "alt+4", Cmd: CmdSelectTab, Arg: Arg("4"), Help: "切到第 4 个标签"},
		{Keys: "alt+5", Cmd: CmdSelectTab, Arg: Arg("5"), Help: "切到第 5 个标签"},
		{Keys: "alt+6", Cmd: CmdSelectTab, Arg: Arg("6"), Help: "切到第 6 个标签"},
		{Keys: "alt+7", Cmd: CmdSelectTab, Arg: Arg("7"), Help: "切到第 7 个标签"},
		{Keys: "alt+8", Cmd: CmdSelectTab, Arg: Arg("8"), Help: "切到第 8 个标签"},
		{Keys: "alt+9", Cmd: CmdLastTab, Help: "切到最后一个标签"},
		// 标签之间的相邻移动没有占用 spec 的键位，用两个不与前缀冲突的组合。
		{Keys: ctrlPlus + "a ]", Cmd: CmdNextTab, Help: "下一个标签"},
		{Keys: ctrlPlus + "a [", Cmd: CmdPrevTab, Help: "上一个标签"},

		// ---- 编辑器交互（spec 一.3）----
		{Keys: ctrlPlus + "a t", Cmd: CmdTerminal, Help: "内置终端"},
		{Keys: ctrlPlus + "a p", Cmd: CmdCommandPalette, Help: "命令面板"},
		{Keys: ctrlPlus + "a w", Cmd: CmdSave, Help: "保存"},
		// Shift+字母在大写时，键位表里要写字母本身而不是 shift+字母：
		// 终端把 Shift 吸收进了字符，toKeystroke 返回的是 "S"，
		// 永远不会是 "shift+s"。写成后者这条绑定就永远按不出来。
		{Keys: ctrlPlus + "a S", Cmd: CmdSaveAs, Help: "另存为"},
		// 退出。有未保存改动时 C-a q 不会真退出，会在状态栏提示；
		// 确实要走就用 C-a x 强制退出，不给一条「无条件丢弃」的路径。
		{Keys: ctrlPlus + "a q", Cmd: CmdQuit, Help: "退出（有改动时会拒绝）"},
		{Keys: ctrlPlus + "a x", Cmd: CmdForceQuit, Help: "强制退出，丢弃未保存改动"},
		{Keys: ctrlPlus + "a k", Cmd: CmdCloseTab, Help: "关闭标签"},
		// 查找：第一次按 C-a / 先问要查什么，之后在匹配间移动。
		//
		// 「下一处/上一处」必须绑到组合键上，不能绑裸字母。
		// 本编辑器没有 Vim 那样的模式切换，字母默认就是往文档里插字符；
		// 把 n 绑给「下一处匹配」会让人打不出单词里的 n，
		// 而且症状很隐蔽：只有恰好输入到那个字母时才发现。
		// C-s / C-r 是 Emacs 的 isearch-forward / isearch-backward。
		{Keys: ctrlPlus + "a /", Cmd: CmdFindInBuffer, Help: "在文件中查找"},
		{Keys: ctrlPlus + "s", Cmd: CmdFindNext, Help: "下一处匹配"},
		{Keys: ctrlPlus + "r", Cmd: CmdFindPrev, Help: "上一处匹配"},
		{Keys: ctrlPlus + "a %", Cmd: CmdReplace, Help: "查找并替换"},
		{Keys: ctrlPlus + "a g", Cmd: CmdGotoLine, Help: "跳转到行"},
		{Keys: ctrlPlus + "a z", Cmd: CmdToggleFold, Help: "折叠/展开"},
		{Keys: ctrlPlus + "a o", Cmd: CmdOutputPanel, Help: "输出面板"},
		{Keys: ctrlPlus + "a e", Cmd: CmdProblemsPanel, Help: "问题面板"},

		// ---- Go 工具链：spec 二.3 ----
		//
		// spec 给出的键位是 C-g 打头的一棵子树。C-g 同时是 Matcher 的中止键，
		// 但中止只在「已有前缀待定」时生效，所以单独按 C-g 仍能进入这棵子树。
		{Keys: ctrlPlus + "g r", Cmd: CmdGoRun, Help: "go run"},
		{Keys: ctrlPlus + "g b", Cmd: CmdGoBuild, Help: "go build"},
		{Keys: ctrlPlus + "g t", Cmd: CmdGoTest, Help: "go test"},
		{Keys: ctrlPlus + "g v", Cmd: CmdGoVet, Help: "go vet"},
		{Keys: ctrlPlus + "g e", Cmd: CmdGoEnv, Help: "go env"},
		{Keys: ctrlPlus + "g l", Cmd: CmdGoList, Help: "go list"},
		// spec 里 go vet 与 go version 都写成 C-g v，二者相撞。
		// 这里让一次性的 go version 走 Shift+V，go vet 保留小写 v。
		// 同另存为：Shift+V 上报的按键名就是 "V"。
		{Keys: ctrlPlus + "g V", Cmd: CmdGoVersion, Help: "go version"},
		// spec 的「C-g-m t」理解为 C-g 之后按 M-t。同理 M-d。
		// 传统终端把 Meta 与 Alt 一并以 ESC 前缀发送，kitty 键盘协议才区分得开，
		// 因此 meta 与 alt 两个写法都绑定到同一条命令。
		{Keys: ctrlPlus + "g meta+t", Cmd: CmdGoModTidy, Help: "go mod tidy"},
		{Keys: ctrlPlus + "g alt+t", Cmd: CmdGoModTidy, Help: "go mod tidy"},
		{Keys: ctrlPlus + "g meta+d", Cmd: CmdGoModDownload, Help: "go mod download"},
		{Keys: ctrlPlus + "g alt+d", Cmd: CmdGoModDownload, Help: "go mod download"},
		{Keys: ctrlPlus + "g f", Cmd: CmdGoFormatFile, Help: "gofmt 格式化"},
		{Keys: ctrlPlus + "g R", Cmd: CmdRestartLSP, Help: "重启 gopls"},

		// ---- 导航与基础编辑：这些键在 spec 里没写死，用终端自身的按键 ----
		//
		// spec 只给了命令式键位，日常移动交给方向键与 Home/End 更符合直觉，
		// 也不占用前缀键。
		{Keys: "<left>", Cmd: CmdMoveLeft, Help: "左移"},
		{Keys: "<right>", Cmd: CmdMoveRight, Help: "右移"},
		{Keys: "<up>", Cmd: CmdMoveUp, Help: "上移"},
		{Keys: "<down>", Cmd: CmdMoveDown, Help: "下移"},
		{Keys: "<home>", Cmd: CmdLineStart, Help: "行首"},
		{Keys: "<end>", Cmd: CmdLineEnd, Help: "行尾"},
		{Keys: "<pgup>", Cmd: CmdPageUp, Help: "上翻一页"},
		{Keys: "<pgdown>", Cmd: CmdPageDown, Help: "下翻一页"},
		{Keys: ctrlPlus + "<left>", Cmd: CmdMoveWordLeft, Help: "按词左移"},
		{Keys: ctrlPlus + "<right>", Cmd: CmdMoveWordRight, Help: "按词右移"},
		{Keys: ctrlPlus + "<home>", Cmd: CmdDocStart, Help: "跳到文件开头"},
		{Keys: ctrlPlus + "<end>", Cmd: CmdDocEnd, Help: "跳到文件末尾"},
		{Keys: ctrlPlus + "<enter>", Cmd: CmdInsertNewline, Help: "换行"},
		{Keys: "<enter>", Cmd: CmdInsertNewline, Help: "换行"},
		{Keys: "<tab>", Cmd: CmdInsertTab, Help: "缩进"},
		{Keys: "<backspace>", Cmd: CmdBackspace, Help: "删除左侧字符"},
		{Keys: "<delete>", Cmd: CmdDeleteRight, Help: "删除右侧字符"},
	}
}

// registry 是可用的键位方案。后续加 Vim 风格时在这里登记即可，
// 调用方只认方案名，不需要知道表是怎么建的。
var registry = map[string]func() (*Table, error){
	EmacsName: Emacs,
}

// Register 登记一套键位方案。同名重复登记会 panic：
// 这属于开发期的接线错误，必须立刻暴露。
func Register(name string, build func() (*Table, error)) {
	if _, exists := registry[name]; exists {
		panic(fmt.Sprintf("keymap: 方案名 %q 重复登记", name))
	}
	registry[name] = build
}

// Lookup 按名字构造键位方案。名字未知时返回错误，
// 而不是回退到默认表：静默回退会让用户按了键却毫无反应。
func Lookup(name string) (*Table, error) {
	build, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("keymap: 未知键位方案 %q", name)
	}
	table, err := build()
	if err != nil {
		return nil, err
	}
	if table.Name != name {
		return nil, fmt.Errorf("keymap: 方案 %q 构造出的表自称 %q", name, table.Name)
	}
	return table, nil
}

// Names 返回全部可用方案名，字典序。
func Names() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
