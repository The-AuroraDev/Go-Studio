// keymap_test.go — 键位表构造与查找的单元测试。
// SPDX-License-Identifier: MIT

package keymap

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/29anan29/Go-Studio/internal/screen"
)

func TestEmacsTableBuilds(t *testing.T) {
	table, err := Emacs()
	if err != nil {
		t.Fatalf("Emacs() 返回错误: %v", err)
	}
	if table.Name != EmacsName {
		t.Errorf("方案名 = %q, want %q", table.Name, EmacsName)
	}
	if len(table.Bindings()) == 0 {
		t.Fatal("绑定表为空")
	}
}

// TestEmacsTableMatchesSpec 逐条核对 docs/spec.md 写明的键位。
// 这是本包最重要的一条测试：spec 是需求文档，键位表是它的实现，
// 两者一旦漂移，用户按 spec 里的键却没有反应，且很难察觉。
func TestEmacsTableMatchesSpec(t *testing.T) {
	table, err := Emacs()
	if err != nil {
		t.Fatalf("Emacs() 返回错误: %v", err)
	}

	// spec 一.1 文本编辑
	// spec 一.2 文件与项目界面
	// spec 一.3 编辑器交互
	// spec 二.3 构建、运行、测试
	cases := []struct {
		keys string
		cmd  Command
		arg  Arg
	}{
		// spec 一.1：复制、粘贴、撤销、重做
		{"ctrl+shift+c", CmdCopy, ""},
		{"ctrl+shift+v", CmdPaste, ""},
		{"ctrl+a r", CmdUndo, ""},
		{"ctrl+a y", CmdRedo, ""},
		// spec 一.1：多选与列选择
		{"ctrl+d", CmdSelectNext, ""},
		{"alt+shift+<up>", CmdColumnSelectUp, ""},
		{"alt+shift+<down>", CmdColumnSelectDown, ""},
		{"alt+shift+<left>", CmdColumnSelectLeft, ""},
		{"alt+shift+<right>", CmdColumnSelectRight, ""},
		// spec 一.2：打开文件、文件树、全局搜索、分屏
		{"ctrl+a f", CmdOpenFile, ""},
		{"ctrl+space", CmdFileTree, ""},
		{"ctrl+a s", CmdGlobalSearch, ""},
		{"ctrl+a 0", CmdSplitVertical, ""},
		{"ctrl+a 1", CmdSplitHorizontal, ""},
		// spec 一.2：多标签页，M-数字切换，M-9 跳到最后一个
		{"alt+1", CmdSelectTab, "1"},
		{"alt+5", CmdSelectTab, "5"},
		{"alt+8", CmdSelectTab, "8"},
		{"alt+9", CmdLastTab, ""},
		// spec 一.3：内置终端
		{"ctrl+a t", CmdTerminal, ""},
		// spec 二.3：Go 工具链
		{"ctrl+g r", CmdGoRun, ""},
		{"ctrl+g b", CmdGoBuild, ""},
		{"ctrl+g t", CmdGoTest, ""},
		{"ctrl+g v", CmdGoVet, ""},
		{"ctrl+g e", CmdGoEnv, ""},
		{"ctrl+g l", CmdGoList, ""},
		{"ctrl+g V", CmdGoVersion, ""},
		// spec 写的 C-g-m t / C-g-m d，理解为 C-g 之后按 M-t / M-d。
		// meta 与 alt 都绑：传统终端把两者一并以 ESC 前缀发送。
		{"ctrl+g meta+t", CmdGoModTidy, ""},
		{"ctrl+g alt+t", CmdGoModTidy, ""},
		{"ctrl+g meta+d", CmdGoModDownload, ""},
		{"ctrl+g alt+d", CmdGoModDownload, ""},
	}

	for _, tc := range cases {
		binding, ok := table.Lookup(tc.keys)
		if !ok {
			t.Errorf("按键序列 %q 在键位表里不存在", tc.keys)
			continue
		}
		if binding.Cmd != tc.cmd {
			t.Errorf("按键序列 %q -> 命令 %q, want %q", tc.keys, binding.Cmd, tc.cmd)
		}
		if binding.Arg != tc.arg {
			t.Errorf("按键序列 %q -> 参数 %q, want %q", tc.keys, binding.Arg, tc.arg)
		}
		if binding.Help == "" {
			t.Errorf("按键序列 %q 缺少说明文字", tc.keys)
		}
	}
}

// TestEmacsResolvesVimKeyConflict 固化 C-g v 的冲突裁决。
// spec 把 go vet 与 go version 都写成 C-g v，两者必然相撞；
// 这里确保 go vet 保留小写 v、go version 用大写 V，不会被后续改动悄悄改回去。
func TestEmacsResolvesVimKeyConflict(t *testing.T) {
	table, err := Emacs()
	if err != nil {
		t.Fatalf("Emacs() 返回错误: %v", err)
	}
	vet, _ := table.Lookup("ctrl+g v")
	version, _ := table.Lookup("ctrl+g V")
	if vet.Cmd != CmdGoVet {
		t.Errorf("C-g v = %q, want %q", vet.Cmd, CmdGoVet)
	}
	if version.Cmd != CmdGoVersion {
		t.Errorf("C-g V = %q, want %q", version.Cmd, CmdGoVersion)
	}
}

// TestEveryBindingIsReachable 保证表里每条绑定都真的能被解析出来。
// 构造期的校验只查冲突，查不出「绑了但永远到不了」的情况。
func TestEveryBindingIsReachable(t *testing.T) {
	table, err := Emacs()
	if err != nil {
		t.Fatalf("Emacs() 返回错误: %v", err)
	}
	for _, binding := range table.Bindings() {
		got, ok := table.Lookup(binding.Keys)
		if !ok {
			t.Errorf("绑定的按键序列 %q 无法反查", binding.Keys)
			continue
		}
		if got.Cmd != binding.Cmd || got.Arg != binding.Arg {
			t.Errorf("按键序列 %q 反查得到 %q/%q, want %q/%q",
				binding.Keys, got.Cmd, got.Arg, binding.Cmd, binding.Arg)
		}
	}
}

// TestBindingsAreSorted 保证命令面板与键位帮助的列举顺序稳定。
func TestBindingsAreSorted(t *testing.T) {
	table, err := Emacs()
	if err != nil {
		t.Fatalf("Emacs() 返回错误: %v", err)
	}
	bindings := table.Bindings()
	for i := 1; i < len(bindings); i++ {
		if bindings[i-1].Keys > bindings[i].Keys {
			t.Fatalf("绑定未按字典序排列: %q 出现在 %q 之前",
				bindings[i-1].Keys, bindings[i].Keys)
		}
	}
}

// TestBindingsReturnCopy 保证调用方改返回值不会污染键位表。
func TestBindingsReturnCopy(t *testing.T) {
	table, err := Emacs()
	if err != nil {
		t.Fatalf("Emacs() 返回错误: %v", err)
	}
	bindings := table.Bindings()
	if len(bindings) == 0 {
		t.Fatal("绑定表为空")
	}
	bindings[0].Cmd = "tampered"
	if table.Bindings()[0].Cmd == "tampered" {
		t.Error("修改 Bindings() 的返回值污染了键位表")
	}
}

func TestNewRejectsBadTables(t *testing.T) {
	cases := []struct {
		name     string
		table    string
		bindings []Binding
		wantErr  string
	}{
		{
			name:     "方案名为空",
			table:    "",
			bindings: []Binding{{Keys: "a", Cmd: CmdMoveLeft}},
			wantErr:  "方案名为空",
		},
		{
			name:     "按键序列为空",
			table:    "t",
			bindings: []Binding{{Keys: "   ", Cmd: CmdMoveLeft}},
			wantErr:  "按键序列为空",
		},
		{
			name:     "没有对应命令",
			table:    "t",
			bindings: []Binding{{Keys: "a"}},
			wantErr:  "没有对应命令",
		},
		{
			name:     "同一序列重复绑定",
			table:    "t",
			bindings: []Binding{{Keys: "ctrl+a", Cmd: CmdUndo}, {Keys: "ctrl+a", Cmd: CmdRedo}},
			wantErr:  "重复绑定",
		},
		{
			name:     "归一化后重复绑定",
			table:    "t",
			bindings: []Binding{{Keys: "alt+ctrl+a", Cmd: CmdUndo}, {Keys: "ctrl+alt+a", Cmd: CmdRedo}},
			wantErr:  "重复绑定",
		},
		{
			name:     "既是终点又是前缀",
			table:    "t",
			bindings: []Binding{{Keys: "ctrl+a", Cmd: CmdUndo}, {Keys: "ctrl+a r", Cmd: CmdRedo}},
			wantErr:  "有歧义",
		},
		{
			name:     "前缀冲突反向书写",
			table:    "t",
			bindings: []Binding{{Keys: "ctrl+a r", Cmd: CmdUndo}, {Keys: "ctrl+a", Cmd: CmdRedo}},
			wantErr:  "有歧义",
		},
		{
			name:     "只有修饰键",
			table:    "t",
			bindings: []Binding{{Keys: "ctrl+", Cmd: CmdUndo}},
			wantErr:  "不是合法按键",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := New(tc.table, tc.bindings)
			if err == nil {
				t.Fatalf("New() 未报错，want %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("New() 报错 = %q, want 包含 %q", err, tc.wantErr)
			}
		})
	}
}

func TestNewNormalizesModifierOrder(t *testing.T) {
	table, err := New("t", []Binding{{Keys: "shift+alt+<up>", Cmd: CmdMoveUp}})
	if err != nil {
		t.Fatalf("New() 返回错误: %v", err)
	}
	// 存的必须是归一化后的写法：alt 排在 shift 之前。
	// 否则 Bindings() 的列举结果会随书写顺序变化，键位帮助界面对不上。
	bindings := table.Bindings()
	if len(bindings) != 1 {
		t.Fatalf("绑定数 = %d, want 1", len(bindings))
	}
	if want := "alt+shift+<up>"; bindings[0].Keys != want {
		t.Errorf("存下的按键序列 = %q, want %q", bindings[0].Keys, want)
	}
	// 反查会先归一化输入，两种写法因此都能命中同一个键。
	for _, seq := range []string{"alt+shift+<up>", "shift+alt+<up>"} {
		if _, ok := table.Lookup(seq); !ok {
			t.Errorf("Lookup(%q) 未命中", seq)
		}
	}
}

func TestLookupRejectsInvalidSequence(t *testing.T) {
	table, err := Emacs()
	if err != nil {
		t.Fatalf("Emacs() 返回错误: %v", err)
	}
	for _, seq := range []string{"", "  ", "ctrl+"} {
		if _, ok := table.Lookup(seq); ok {
			t.Errorf("Lookup(%q) 竟然命中了", seq)
		}
	}
}

func TestLookupRejectsUnknownKey(t *testing.T) {
	table, err := Emacs()
	if err != nil {
		t.Fatalf("Emacs() 返回错误: %v", err)
	}
	for _, seq := range []string{"ctrl+q ctrl+q", "F13", "ctrl+a ctrl+a r", "<f13>"} {
		if _, ok := table.Lookup(seq); ok {
			t.Errorf("Lookup(%q) 竟然命中了", seq)
		}
	}
}

func TestLookupByName(t *testing.T) {
	table, err := Lookup(EmacsName)
	if err != nil {
		t.Fatalf("Lookup(%q) 返回错误: %v", EmacsName, err)
	}
	if table.Name != EmacsName {
		t.Errorf("方案名 = %q, want %q", table.Name, EmacsName)
	}
}

func TestLookupRejectsUnknownName(t *testing.T) {
	// 不回退到默认表：静默回退会让用户按了键却毫无反应，且无从排查。
	_, err := Lookup("emacs-v2")
	if err == nil {
		t.Fatal("Lookup() 对未知方案名未报错")
	}
	if !strings.Contains(err.Error(), "未知键位方案") {
		t.Errorf("报错 = %q, want 包含 %q", err, "未知键位方案")
	}
}

func TestRegisterRejectsDuplicateName(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("重复登记同名方案时未 panic")
		}
	}()
	Register(EmacsName, Emacs)
}

func TestNamesIncludesEmacs(t *testing.T) {
	names := Names()
	for i := 1; i < len(names); i++ {
		if names[i-1] > names[i] {
			t.Fatalf("方案名未排序: %q 在 %q 之前", names[i-1], names[i])
		}
	}
	found := false
	for _, name := range names {
		if name == EmacsName {
			found = true
		}
	}
	if !found {
		t.Errorf("方案名列表 %v 里没有 %q", names, EmacsName)
	}
}

func TestCommandString(t *testing.T) {
	if got := CmdUndo.String(); got != "undo" {
		t.Errorf("CmdUndo.String() = %q, want %q", got, "undo")
	}
}

func TestResultString(t *testing.T) {
	cases := map[Result]string{
		ResultMatch:   "match",
		ResultPending: "pending",
		ResultUnknown: "unknown",
		ResultIgnored: "ignored",
		Result(99):    "invalid",
	}
	for result, want := range cases {
		if got := result.String(); got != want {
			t.Errorf("Result(%d).String() = %q, want %q", int(result), got, want)
		}
	}
}

// TestNoPlainLetterIsBound 是本项目最重要的一条键位不变量。
//
// 本编辑器没有 Vim 那样的模式切换：没有「普通模式」，字母按下去就是往文档里插字符。
// 因此任何一个裸字母都不能被绑到命令上——
// 否则用户打不出包含那个字母的单词，而且症状极其隐蔽：
// 只有恰好输入到那个字母时才出问题，很容易被当成偶发故障。
//
// 这条测试就是防止有人再加回 n / N 这类绑定。
func TestNoPlainLetterIsBound(t *testing.T) {
	table, err := Emacs()
	if err != nil {
		t.Fatalf("Emacs() 返回错误: %v", err)
	}
	for _, binding := range table.Bindings() {
		keys := strings.Fields(binding.Keys)
		if len(keys) != 1 {
			continue // 多键序列不是裸字母
		}
		key := keys[0]
		// 裸字母就是「没有修饰符、没有尖括号」的可打印字符。
		if screen.IsNamedKey(key) || strings.Contains(key, "+") {
			continue
		}
		t.Errorf("按键 %q（命令 %q）是裸字母，会挡住该字符的输入",
			binding.Keys, binding.Cmd)
	}
}

// TestCommonWordsRemainTypeable 拿真实单词过一遍，确认每个字母都没被占用。
func TestCommonWordsRemainTypeable(t *testing.T) {
	words := []string{
		"n", "N", "function", "return", "if", "else", "for", "range",
		"import", "package", "struct", "string", "int", "bool", "nil",
		"func", "var", "const", "type", "go", "defer", "chan", "map",
		"true", "false", "error", "make", "new", "copy", "len", "cap",
	}
	occupied := make(map[string]Command)
	table, err := Emacs()
	if err != nil {
		t.Fatal(err)
	}
	for _, binding := range table.Bindings() {
		fields := strings.Fields(binding.Keys)
		if len(fields) != 1 {
			continue
		}
		key := fields[0]
		if screen.IsNamedKey(key) || strings.Contains(key, "+") {
			continue
		}
		occupied[key] = binding.Cmd
	}
	for _, word := range words {
		for _, r := range word {
			ch := string(r)
			if cmd, taken := occupied[ch]; taken {
				t.Errorf("字母 %q（出现在 %q 里）被命令 %q 占用，打不出来",
					ch, word, cmd)
			}
		}
	}
}

// TestNoUnreachableShiftLetterBinding 堵住一类「按了永远没反应」的绑定。
//
// 终端对「Shift + 可打印字母」不上报 shift 修饰，而是把 Shift 吸收进字符本身：
// 按 Shift+S 的按键名是 "S"，不是 "shift+s"。
// 因此任何形如 shift+<单个可打印字符>（且没有 ctrl/alt/meta 同时存在）的
// 序列都不可能被终端产生，这类绑定等于凭空写上去的。
//
// 带 ctrl 的不受影响：Ctrl+Shift+S 的按键名确实是 "ctrl+shift+s"。
func TestNoUnreachableShiftLetterBinding(t *testing.T) {
	table, err := Emacs()
	if err != nil {
		t.Fatal(err)
	}
	for _, binding := range table.Bindings() {
		for _, token := range strings.Fields(binding.Keys) {
			if !strings.HasPrefix(token, "shift+") {
				continue
			}
			base := strings.TrimPrefix(token, "shift+")
			// 带其它修饰键时终端确实会保留 shift，这条例外。
			if strings.Contains(base, "+") {
				continue
			}
			if screen.IsNamedKey(token) {
				continue // shift+<up> 这类命名键是真实的
			}
			r, size := utf8.DecodeRuneInString(base)
			if size != len(base) || !unicode.IsPrint(r) {
				continue
			}
			t.Errorf("按键序列 %q 里的 %q 永远不会被终端上报：Shift+字母的按键名就是字母本身",
				binding.Keys, token)
		}
	}
}
