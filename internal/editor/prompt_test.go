// prompt_test.go — 单行输入浮层的单元测试。
// SPDX-License-Identifier: MIT

package editor

import (
	"strings"
	"testing"

	"github.com/29anan29/Go-Studio/internal/view"
)

// TestPromptCollectsInput 打到的字符都进输入框。
func TestPromptCollectsInput(t *testing.T) {
	p := newPrompt(promptOpenFile, "")
	for _, r := range "ab c中d" {
		p.handleKey(string(r))
	}
	if got := p.text(); got != "ab c中d" {
		t.Errorf("输入 = %q, want %q", got, "ab c中d")
	}
}

// TestPromptSpaceIsInput 空格必须能打进路径里：
// 路径含空格极常见，它曾经被当成命名键而打不进去。
func TestPromptSpaceIsInput(t *testing.T) {
	p := newPrompt(promptOpenFile, "")
	p.handleKey("space")
	if got := p.text(); got != " " {
		t.Errorf("输入 = %q, want 一个空格", got)
	}
}

// TestPromptSwallowsNavigationKeys 编辑键与导航键必须被吞掉，
// 否则用户只想打路径却把文档改了。
func TestPromptSwallowsNavigationKeys(t *testing.T) {
	swallowed := []string{
		"<down>", "<up>", "<left>", "<right>", "<home>", "<end>",
		"<delete>", "<enter>", "<esc>", "<tab>",
	}
	for _, key := range swallowed {
		p := newPrompt(promptFind, "")
		if !p.handleKey(key) {
			t.Errorf("handleKey(%q) = false, want true（按键必须被浮层吃掉）", key)
		}
		if got := p.text(); got != "" {
			t.Errorf("按 %q 之后输入框 = %q, want 不变", key, got)
		}
	}
}

// TestPromptBackspace 退格删最后一个字符，退到空也不能崩。
func TestPromptBackspace(t *testing.T) {
	p := newPrompt(promptOpenFile, "abc")
	p.handleKey("<backspace>")
	if got := p.text(); got != "ab" {
		t.Errorf("退格后 = %q, want %q", got, "ab")
	}
	for range 10 {
		p.handleKey("<backspace>")
	}
	if got := p.text(); got != "" {
		t.Errorf("反复退格后 = %q, want 空", got)
	}
}

// TestPromptEscapeClearsInput Esc 要清空输入。
func TestPromptEscapeClearsInput(t *testing.T) {
	p := newPrompt(promptOpenFile, "abc")
	p.handleKey("<esc>")
	if got := p.text(); got != "" {
		t.Errorf("Esc 后 = %q, want 空", got)
	}
}

// TestPromptPrefill 预填内容要成为初始输入。
func TestPromptPrefill(t *testing.T) {
	p := newPrompt(promptGotoLine, "42")
	if got := p.text(); got != "42" {
		t.Errorf("预填 = %q, want %q", got, "42")
	}
}

// TestPromptAppendsToPrefill 预填之后继续输入是追加，不是替换：
// 打开文件时预填当前目录，用户接着打文件名即可。
func TestPromptAppendsToPrefill(t *testing.T) {
	p := newPrompt(promptOpenFile, "/tmp/")
	p.handleKey("a")
	if got := p.text(); got != "/tmp/a" {
		t.Errorf("追加后 = %q, want %q", got, "/tmp/a")
	}
}

func TestPromptLabels(t *testing.T) {
	cases := map[promptKind]string{
		promptOpenFile: "打开文件",
		promptSaveAs:   "另存为",
		promptGotoLine: "跳转到行",
		promptFind:     "查找",
		promptKind(99): "输入",
	}
	for kind, want := range cases {
		if got := kind.label(); got != want {
			t.Errorf("label(%d) = %q, want %q", int(kind), got, want)
		}
	}
}

func TestPromptConfirmKind(t *testing.T) {
	for _, kind := range []promptKind{promptOpenFile, promptSaveAs, promptGotoLine, promptFind} {
		p := newPrompt(kind, "")
		if got := p.confirmKind(); got != kind {
			t.Errorf("confirmKind() = %v, want %v", got, kind)
		}
	}
}

// TestPromptRenderFitsWidth 浮层那一行必须精确占满宽度，
// 否则终端会折行、整帧布局跟着崩。
func TestPromptRenderFitsWidth(t *testing.T) {
	theme := view.PlainTheme()
	for _, width := range []int{1, 2, 5, 20, 80} {
		p := newPrompt(promptOpenFile, strings.Repeat("很长的路径内容", 30))
		line, _ := p.render(theme, width)
		if got := view.VisibleWidth(line); got != width {
			t.Errorf("宽度 %d 时浮层行宽 = %d, want %d", width, got, width)
		}
	}
}

// TestPromptRenderShowsLabel 提示语必须出现在渲染结果里，
// 否则用户不知道这里要输入什么。
func TestPromptRenderShowsLabel(t *testing.T) {
	p := newPrompt(promptGotoLine, "42")
	line, cursor := p.render(view.PlainTheme(), 40)

	if !strings.Contains(line, "跳转到行") {
		t.Errorf("渲染里没有提示语: %q", line)
	}
	if !strings.Contains(line, "42") {
		t.Errorf("渲染里没有预填内容: %q", line)
	}
	if cursor == nil {
		t.Fatal("没有光标")
	}
	// 光标应落在输入末尾。
	if got, want := cursor.X, view.TextWidth("跳转到行: 42"); got != want {
		t.Errorf("光标 X = %d, want %d（应落在输入末尾）", got, want)
	}
}

// TestPromptRenderHidesCursorWhenOverlong 输入比视口还长时不该报光标坐标：
// 越界的坐标会让终端把光标写到别处去。
func TestPromptRenderHidesCursorWhenOverlong(t *testing.T) {
	p := newPrompt(promptOpenFile, strings.Repeat("x", 200))
	line, cursor := p.render(view.PlainTheme(), 20)

	if got := view.VisibleWidth(line); got != 20 {
		t.Errorf("行宽 = %d, want 20", got)
	}
	if cursor != nil {
		t.Errorf("输入超长时不该报光标坐标，得到 %+v", cursor)
	}
}

func TestResolvePath(t *testing.T) {
	cases := map[string]string{
		"":                  "",
		"   ":               "",
		"  a.go  ":          "a.go",
		`"a.go"`:            "a.go",
		`'a.go'`:            "a.go",
		`  "/tmp/x/a.go"  `: "/tmp/x/a.go",
		"/tmp/x/a.go":       "/tmp/x/a.go",
		// 只有一侧引号时不该被剥掉，那本身就不是引号包裹的路径。
		`"a.go`: `"a.go`,
	}
	for in, want := range cases {
		if got := resolvePath(in); got != want {
			t.Errorf("resolvePath(%q) = %q, want %q", in, got, want)
		}
	}
}
