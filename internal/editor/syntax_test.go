// syntax_test.go — 编辑器与语法高亮的接线测试。
// SPDX-License-Identifier: MIT

package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/29anan29/Go-Studio/internal/config"
	"github.com/29anan29/Go-Studio/internal/document"
	"github.com/29anan29/Go-Studio/internal/keymap"
	"github.com/29anan29/Go-Studio/internal/screen"
	"github.com/29anan29/Go-Studio/internal/syntax"
	"github.com/29anan29/Go-Studio/internal/view"
)

// commentColor 取自 view 的真实配色表，而不是在测试里硬编码。
// 硬编码的坏处是：以后调整配色，测试会误报，而那时该改的是断言。
func commentColor() string { return view.SyntaxTheme().Syntax[syntax.KindComment] }

// keywordColor 同理，用来确认代码确实上了色。
func keywordColor() string { return view.SyntaxTheme().Syntax[syntax.KindKeyword] }

// namedHarness 造一个带真实路径的编辑器，用来测「按扩展名认语言」。
// 路径必须来自真实文件：Document 的 path 只能通过打开文件得到。
func namedHarness(t *testing.T, name, content string, cfg config.Config) *harness {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写入临时文件失败: %v", err)
	}
	doc, err := document.Open(path)
	if err != nil {
		t.Fatalf("打开 %s 失败: %v", name, err)
	}

	back := screen.NewFake(testCaps)
	table, err := keymap.Lookup(keymap.EmacsName)
	if err != nil {
		t.Fatalf("构造键位表失败: %v", err)
	}
	clock := &fakeClock{now: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}

	app, err := New(Options{
		Backend: back, Config: cfg, Table: table, Doc: doc, Now: clock.Now,
	})
	if err != nil {
		t.Fatalf("New() 返回错误: %v", err)
	}
	app.draw()
	return &harness{t: t, app: app, back: back, clock: clock}
}

// rawFrame 取最后一帧的原始内容（含 ANSI 序列）。
func (h *harness) rawFrame() string {
	h.t.Helper()
	return h.back.LastFrame().Content
}

func TestHighlighterBuiltForKnownExtension(t *testing.T) {
	cfg := config.Default()
	cfg.Editor.Syntax = true

	cases := map[string]string{
		"main.go":   "Go",
		"a.py":      "Python",
		"a.rs":      "Rust",
		"a.ts":      "TypeScript",
		"a.md":      "Markdown",
		"a.json":    "JSON",
		"deploy.sh": "Shell",
		"style.css": "CSS",
		"a.unknown": "",
		"notes":     "",
		"data.txt":  "",
		"build.log": "",
	}
	for path, want := range cases {
		h := namedHarness(t, path, "x := 1\n", cfg)
		if got := h.app.hlLang; got != want {
			t.Errorf("%s: 语言 = %q，期望 %q", path, got, want)
		}
	}
}

func TestNoHighlighterForPlainTextFiles(t *testing.T) {
	// 认不出语言就是纯文本：不报错也不提示。
	// 用户打开日志、临时文件时不该被弹窗骚扰。
	h := namedHarness(t, "server.log", "GET / 200\n", config.Default())

	if h.app.hl != nil {
		t.Error("未认出的文件不该建高亮器")
	}
	if h.app.hlLang != "" {
		t.Errorf("语言名应为空，得到 %q", h.app.hlLang)
	}
	// 不能笼统地查 "\x1b[38;5;"：行号槽本来就用前景色，
	// 那不是语法上色。只查具体的语法色。
	for _, c := range []string{commentColor(), keywordColor()} {
		if c != "" && strings.Contains(h.rawFrame(), c) {
			t.Errorf("纯文本不该有语法色 %q：%q", c, h.rawFrame())
		}
	}
}

func TestSyntaxCanBeDisabled(t *testing.T) {
	cfg := config.Default()
	cfg.Editor.Syntax = false
	h := namedHarness(t, "main.go", "package main\n", cfg)

	if h.app.hl != nil {
		t.Error("syntax = false 时不该建高亮器")
	}
	if c := commentColor(); c != "" && strings.Contains(h.rawFrame(), c) {
		t.Errorf("关掉高亮后不该还有注释色 %q", c)
	}
}

func TestSyntaxLangOverridesExtension(t *testing.T) {
	// 无扩展名的文件按扩展名判断必然落空，syntax_lang 就是给这种情况兜底的。
	cfg := config.Default()
	cfg.Editor.SyntaxLang = "Go"

	h := namedHarness(t, "Makefile", "package main\n", cfg)
	if h.app.hlLang != "Go" {
		t.Errorf("syntax_lang 应覆盖扩展名判断，得到 %q", h.app.hlLang)
	}
}

func TestUnknownSyntaxLangFallsBackToExtension(t *testing.T) {
	cfg := config.Default()
	cfg.Editor.SyntaxLang = "不存在的语言"

	h := namedHarness(t, "main.go", "package main\n", cfg)
	if h.app.hlLang != "Go" {
		t.Errorf("未知语言应回落到扩展名判断，得到 %q", h.app.hlLang)
	}
	if !strings.Contains(h.app.status, "不存在的语言") {
		t.Errorf("应提示未知语言，实际状态栏 = %q", h.app.status)
	}
}

func TestHighlighterInvalidatedOnEdit(t *testing.T) {
	// 在第 1 行开一个块注释，后面所有行都得变成注释色。
	// 缓存若没跟着作废，第 2、3 行会继续显示旧颜色。
	h := namedHarness(t, "main.go", "package main\nx := 1\ny := 2\n", config.Default())

	if strings.Contains(h.rawFrame(), commentColor()) {
		t.Error("编辑前不该有注释色")
	}

	h.pressAll("/", "*")

	if !strings.Contains(h.rawFrame(), commentColor()) {
		t.Errorf("开启块注释后下面应变成注释色，实际帧：%q", h.rawFrame())
	}
}

func TestHighlighterFollowsUndo(t *testing.T) {
	// 撤销也是编辑，缓存必须同步作废，
	// 否则撤销回没有注释的状态、颜色却还留着。
	h := namedHarness(t, "main.go", "package main\nx := 1\n", config.Default())

	h.pressAll("/", "*")
	if !strings.Contains(h.rawFrame(), commentColor()) {
		t.Fatal("先确认注释色出现了，撤销测试才有意义")
	}

	h.app.undo(keymap.Binding{})
	h.app.draw()

	if strings.Contains(h.rawFrame(), commentColor()) {
		t.Errorf("撤销后不该还留着注释色：%q", h.rawFrame())
	}
}

func TestStatusLineShowsLanguage(t *testing.T) {
	// 用户最常问「为什么这份文件没高亮」。
	// 把识别结果直接显示出来，比让人猜省事。
	h := namedHarness(t, "main.go", "package main\n", config.Default())
	if got := h.app.statusLine(); !strings.Contains(got, "Go") {
		t.Errorf("状态栏应显示语言名，得到 %q", got)
	}

	plain := namedHarness(t, "notes.txt", "hello\n", config.Default())
	if got := plain.app.statusLine(); strings.Contains(got, "Go") {
		t.Errorf("纯文本不该显示语言名，得到 %q", got)
	}
}

func TestThemeFollowsHighlighter(t *testing.T) {
	withHL := namedHarness(t, "main.go", "x := 1\n", config.Default())
	withoutHL := namedHarness(t, "notes.txt", "x := 1\n", config.Default())

	if got := withHL.app.theme().Syntax[syntax.KindKeyword]; got == "" {
		t.Error("有高亮器时应使用语法配色")
	}
	if got := withoutHL.app.theme().Syntax[syntax.KindKeyword]; got != "" {
		t.Errorf("无高亮器时不该用语法配色，得到 %q", got)
	}
}

func TestHighlighterRebuiltOnSaveAs(t *testing.T) {
	// 另存为可能换扩展名（main.go -> main.py），语言规则要跟着重认。
	h := namedHarness(t, "main.go", "x = 1\n", config.Default())
	if h.app.hlLang != "Go" {
		t.Fatalf("起点应是 Go，得到 %q", h.app.hlLang)
	}

	h.app.saveAsPath(filepath.Join(t.TempDir(), "main.py"))

	if got := h.app.hlLang; got != "Python" {
		t.Errorf("另存为 .py 后语言应变成 Python，得到 %q", got)
	}
}
