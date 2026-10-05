// keymap_test.go — 键位表构造与查找的单元测试。
// SPDX-License-Identifier: MIT

package keymap

import (
	"strings"
	"testing"
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
			bindings: []Binding{{Keys: "ctrl+a", Cmd: CmdUndo}, {Keys: "alt+ctrl+a", Cmd: CmdRedo}},
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
	// alt 在 shift 之前，因此反查时必须用归一化后的写法。
	if _, ok := table.Lookup("alt+shift+<up>"); !ok {
		t.Error("归一化后的按键序列反查不到")
	}
	if _, ok := table.Lookup("shift+alt+<up>"); ok {
		t.Error("未归一化的按键序列竟然能反查到")
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
	for _, seq := range []string{"ctrl+q", "F13", "ctrl+a ctrl+a r", "<f13>"} {
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
