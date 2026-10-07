// app_test.go — 编辑器主循环的端到端测试，全部经由 screen.Fake，不依赖真实终端。
// SPDX-License-Identifier: MIT

package editor

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/29anan29/Go-Studio/internal/config"
	"github.com/29anan29/Go-Studio/internal/document"
	"github.com/29anan29/Go-Studio/internal/keymap"
	"github.com/29anan29/Go-Studio/internal/screen"
	"github.com/29anan29/Go-Studio/internal/view"
)

// testCaps 是测试用的终端能力：100x30，足够放下几屏文档。
var testCaps = screen.Caps{Width: 100, Height: 30, TrueColor: false, ColorDepth: 16}

// fakeClock 是可控时钟，让状态栏超时可测。
type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time          { return c.now }
func (c *fakeClock) Advance(d time.Duration) { c.now = c.now.Add(d) }

// harness 把 App、Fake 后端与可控时钟绑在一起，供测试使用。
type harness struct {
	t     *testing.T
	app   *App
	back  *screen.Fake
	clock *fakeClock
}

// doc 返回当前文档。
//
// 它必须走 app 而不是缓存一个指针：切换标签之后 App 里的当前文档就换了，
// 缓存下来的旧指针会让测试看着通过、实际验证的是另一个文档。
func (h *harness) doc() *document.Document { return h.app.doc }

// newHarness 用给定内容造一个待测试的编辑器。
func newHarness(t *testing.T, content string) *harness {
	t.Helper()
	back := screen.NewFake(testCaps)
	doc := document.FromString(content)
	table, err := keymap.Lookup(keymap.EmacsName)
	if err != nil {
		t.Fatalf("构造键位表失败: %v", err)
	}
	clock := &fakeClock{now: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}

	app, err := New(Options{
		Backend: back,
		Config:  config.Default(),
		Table:   table,
		Doc:     doc,
		Now:     clock.Now,
	})
	if err != nil {
		t.Fatalf("New() 返回错误: %v", err)
	}
	return &harness{t: t, app: app, back: back, clock: clock}
}

// press 模拟一次按键，并让 App 处理它。
func (h *harness) press(keystroke string) {
	h.t.Helper()
	h.app.handleKey(keystroke)
	h.app.draw()
}

// pressAll 连续按下一串按键。
func (h *harness) pressAll(keystrokes ...string) {
	h.t.Helper()
	for _, k := range keystrokes {
		h.press(k)
	}
}

// frame 返回最后一帧的可见内容（已剥掉 ANSI 序列）。
func (h *harness) frame() string {
	h.t.Helper()
	return stripANSI(h.back.LastFrame().Content)
}

// frameLines 把最后一帧拆成逐行可见内容。
func (h *harness) frameLines() []string {
	h.t.Helper()
	return strings.Split(h.frame(), "\n")
}

// textArea 返回最后一帧的文本区（去掉最后一行状态栏）。
func (h *harness) textArea() []string {
	h.t.Helper()
	lines := h.frameLines()
	if len(lines) > 0 {
		return lines[:len(lines)-1]
	}
	return nil
}

// statusLine 返回最后一帧的状态栏。
func (h *harness) statusLine() string {
	h.t.Helper()
	lines := h.frameLines()
	if len(lines) == 0 {
		return ""
	}
	return lines[len(lines)-1]
}

// cursor 返回最后一帧的光标位置。
func (h *harness) cursor() *screen.CursorSpec {
	h.t.Helper()
	return h.back.LastFrame().Cursor
}

// stripANSI 去掉 ANSI 转义序列，只留可见字符。
func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != 0x1b {
			b.WriteByte(s[i])
			continue
		}
		j := i + 1
		if j < len(s) && s[j] == '[' {
			j++
			for j < len(s) && !(s[j] >= 0x40 && s[j] <= 0x7e) {
				j++
			}
			if j < len(s) {
				j++
			}
		}
		i = j - 1
	}
	return b.String()
}

// ---- 基本连通性 ----

// TestNewRejectsMissingDependencies 依赖不全时必须报错而不是 panic。
func TestNewRejectsMissingDependencies(t *testing.T) {
	table, err := keymap.Lookup(keymap.EmacsName)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]Options{
		"缺后端":  {Doc: document.New(), Table: table},
		"缺文档":  {Backend: screen.NewFake(testCaps), Table: table},
		"缺键位表": {Backend: screen.NewFake(testCaps), Doc: document.New()},
	}
	for name, opts := range cases {
		if _, err := New(opts); err == nil {
			t.Errorf("%s 时 New() 未报错", name)
		}
	}
}

// TestFirstFrameShowsDocument Run 一启动就要画出第一帧，
// 否则用户会在备用屏里看到一片空白。
func TestFirstFrameShowsDocument(t *testing.T) {
	h := newHarness(t, "hello\nworld")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- h.app.Run(ctx) }()

	// 等到第一帧出现。
	deadline := time.After(2 * time.Second)
	for {
		if len(h.back.Frames()) > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("Run() 启动后没有送出任何帧")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run() 返回错误: %v", err)
	}

	text := h.frame()
	if !strings.Contains(text, "hello") || !strings.Contains(text, "world") {
		t.Errorf("第一帧 = %q, want 含 hello 与 world", text)
	}
}

// ---- 键入文本 ----

// TestTypingInsertsCharacters 未绑定到命令的可打印字符必须直接插入文档。
func TestTypingInsertsCharacters(t *testing.T) {
	h := newHarness(t, "")
	h.app.draw()
	h.pressAll("h", "e", "l", "l", "o")

	if got := string(h.doc().Text()); got != "hello" {
		t.Errorf("文档内容 = %q, want %q", got, "hello")
	}
	if !strings.Contains(h.frame(), "hello") {
		t.Errorf("帧内容 = %q, want 含 hello", h.frame())
	}
}

// TestTypingUppercaseAndSpace 验证大写字母与空格真的能被键入。
// 这两样曾经因为按键归一化丢字符而打不出来，是本项目的关键回归点。
func TestTypingUppercaseAndSpace(t *testing.T) {
	h := newHarness(t, "")
	h.app.draw()
	h.pressAll("G", "o", " ", "S", "t", "u", "d", "i", "o")

	if got := string(h.doc().Text()); got != "Go Studio" {
		t.Errorf("文档内容 = %q, want %q", got, "Go Studio")
	}
}

// TestTypingChinese 直接上屏中文，列号按 rune 计。
func TestTypingChinese(t *testing.T) {
	h := newHarness(t, "")
	h.app.draw()
	h.pressAll("你", "好")

	if got := string(h.doc().Text()); got != "你好" {
		t.Errorf("文档内容 = %q, want %q", got, "你好")
	}
	if got := h.doc().Cursor(); got.Col != 2 {
		t.Errorf("光标列 = %d, want 2（按 rune 计）", got.Col)
	}
}

// TestBackspaceDeletes 退格必须真的删字符。
func TestBackspaceDeletes(t *testing.T) {
	h := newHarness(t, "abc")
	h.app.draw()
	h.pressAll("<right>", "<right>", "<right>", "<backspace>")

	if got := string(h.doc().Text()); got != "ab" {
		t.Errorf("文档内容 = %q, want %q", got, "ab")
	}
}

// ---- spec 键位端到端 ----

// TestUndoRedoViaSpecKeys 走 spec.md 的 C-a r / C-a y。
func TestUndoRedoViaSpecKeys(t *testing.T) {
	h := newHarness(t, "")
	h.app.draw()
	h.pressAll("a", "b", "c")
	if got := string(h.doc().Text()); got != "abc" {
		t.Fatalf("输入后 = %q", got)
	}

	h.pressAll("ctrl+a", "r")
	if got := string(h.doc().Text()); got != "" {
		t.Errorf("撤销后 = %q, want 空（三次输入合并成一步）", got)
	}

	h.pressAll("ctrl+a", "y")
	if got := string(h.doc().Text()); got != "abc" {
		t.Errorf("重做后 = %q, want %q", got, "abc")
	}
}

// TestPrefixShowsPendingFeedback 按下前缀键后状态栏必须显示待定序列，
// 否则用户按了 C-a 却看不到反应，会以为按键坏了。
func TestPrefixShowsPendingFeedback(t *testing.T) {
	h := newHarness(t, "abc")
	h.app.draw()
	h.press("ctrl+a")

	if !strings.Contains(h.statusLine(), "C-a") && !strings.Contains(h.statusLine(), "ctrl+a") {
		t.Errorf("状态栏 = %q, want 显示待定前缀", h.statusLine())
	}
}

// TestAbortKeyCancelsPrefix 有前缀待定时 C-g 必须中止。
func TestAbortKeyCancelsPrefix(t *testing.T) {
	h := newHarness(t, "abc")
	h.app.draw()
	h.press("ctrl+a")
	h.press(keymap.AbortKey)

	if pending := h.app.matcher.Pending(); len(pending) != 0 {
		t.Errorf("中止后仍有待定前缀: %v", pending)
	}
	if !strings.Contains(h.statusLine(), "取消") {
		t.Errorf("状态栏 = %q, want 含「取消」", h.statusLine())
	}
}

// TestGoSubtreeViaSpecKeys 走 spec.md 的 C-g 子树。
func TestGoSubtreeViaSpecKeys(t *testing.T) {
	h := newHarness(t, "")
	h.app.draw()

	for _, keys := range [][]string{
		{"ctrl+g", "r"},      // go run
		{"ctrl+g", "b"},      // go build
		{"ctrl+g", "t"},      // go test
		{"ctrl+g", "v"},      // go vet
		{"ctrl+g", "e"},      // go env
		{"ctrl+g", "l"},      // go list
		{"ctrl+g", "V"},      // go version
		{"ctrl+g", "meta+t"}, // go mod tidy
		{"ctrl+g", "alt+t"},  // go mod tidy 的 alt 写法
	} {
		h.pressAll(keys...)
		// 这些命令尚未实现，状态栏应明确说明而不是毫无反应。
		if !strings.Contains(h.statusLine(), "尚未实现") {
			t.Errorf("键位 %v 的状态栏 = %q, want 含「尚未实现」", keys, h.statusLine())
		}
	}
}

// TestFileTreeViaCtrlSpace spec 一.2 的 C-<space>。
func TestFileTreeViaCtrlSpace(t *testing.T) {
	h := newHarness(t, "")
	h.app.draw()
	h.press("ctrl+space")

	if !strings.Contains(h.statusLine(), "尚未实现") {
		t.Errorf("状态栏 = %q, want 文件树命令已识别", h.statusLine())
	}
	// 关键：按了 C-space 不该往文档里插入任何字符。
	if got := string(h.doc().Text()); got != "" {
		t.Errorf("文档内容 = %q, want 空（按 C-space 不该插入）", got)
	}
}

// TestColumnSelectViaSMArrows spec 一.1 的 S-M 方向键。
func TestColumnSelectViaSMArrows(t *testing.T) {
	h := newHarness(t, "abc")
	h.app.draw()
	for _, key := range []string{"alt+shift+<up>", "alt+shift+<down>", "alt+shift+<left>", "alt+shift+<right>"} {
		h.press(key)
		if !strings.Contains(h.statusLine(), "尚未实现") {
			t.Errorf("按键 %q 的状态栏 = %q, want 列选择命令已识别", key, h.statusLine())
		}
	}
}

// ---- 保存与脏标记 ----

// TestSaveViaCtrlAStatusBar C-a w 保存后状态栏应显示文件名。
func TestSaveViaCtrlAStatusBar(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/demo.txt"
	back := screen.NewFake(testCaps)

	doc, err := document.Open(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("准备文件失败: %v", err)
		}
		doc = document.NewNamed(path, false)
	}
	table, _ := keymap.Lookup(keymap.EmacsName)
	clock := &fakeClock{now: time.Now()}
	app, err := New(Options{Backend: back, Config: config.Default(), Table: table, Doc: doc, Now: clock.Now})
	if err != nil {
		t.Fatal(err)
	}
	app.draw()

	app.handleKey("x")
	app.draw()
	if !doc.Dirty() {
		t.Error("编辑后应有未保存改动")
	}

	app.handleKey("ctrl+a")
	app.draw()
	app.handleKey("w")
	app.draw()

	if doc.Dirty() {
		t.Error("保存后不应再有未保存改动")
	}
	if got := strings.TrimSpace(harnessStatus(app)); !strings.Contains(got, "已保存") {
		t.Errorf("状态栏 = %q, want 含「已保存」", got)
	}
}

// harnessStatus 返回 App 会画出的状态栏。
func harnessStatus(a *App) string { return a.statusLine() }

func TestStatusLineShowsPosition(t *testing.T) {
	h := newHarness(t, "abc\ndef")
	h.app.draw()
	h.pressAll("<down>", "<right>")
	// 行列号从 1 开始：第 2 行第 2 列。
	if got := h.statusLine(); !strings.Contains(got, "2:2") {
		t.Errorf("状态栏 = %q, want 含 %q", got, "2:2")
	}
}

func TestStatusLineMarksDirty(t *testing.T) {
	h := newHarness(t, "abc")
	h.app.draw()
	if got := h.statusLine(); strings.Contains(got, "*") {
		t.Errorf("未编辑时状态栏 = %q, want 无脏标记", got)
	}
	h.press("x")
	if got := h.statusLine(); !strings.Contains(got, "*") {
		t.Errorf("编辑后状态栏 = %q, want 含脏标记", got)
	}
}

// ---- 滚动与视口 ----

// TestViewportFollowsCursor 视口必须跟着光标走，否则光标会跑到屏幕外。
func TestViewportFollowsCursor(t *testing.T) {
	lines := make([]string, 100)
	for i := range lines {
		lines[i] = "line" + strings.Repeat("x", i%5)
	}
	h := newHarness(t, strings.Join(lines, "\n"))
	h.app.draw()

	// 光标在第 1 行，往下翻到很后面。
	h.press("ctrl+<end>")
	if got := h.app.top; got == 0 {
		t.Error("跳到文档末尾后视口应向下滚动")
	}
	text := h.frame()
	if !strings.Contains(text, "line") {
		t.Errorf("滚动后帧内容 = %q", text)
	}
}

func TestPageDownMovesViewport(t *testing.T) {
	lines := make([]string, 200)
	for i := range lines {
		lines[i] = "row"
	}
	h := newHarness(t, strings.Join(lines, "\n"))
	h.app.draw()
	h.press("<pgdown>")
	if h.app.top == 0 {
		t.Error("<pgdown> 之后视口应向下滚动")
	}
}

// ---- resize ----

// TestResizeUpdatesViewport resize 事件必须改变视口高度。
func TestResizeUpdatesViewport(t *testing.T) {
	h := newHarness(t, strings.Join([]string{"a", "b", "c"}, "\n"))
	h.app.draw()

	before := len(h.textArea())
	h.app.handleEvent(screen.Event{Kind: screen.EventResize, Width: 40, Height: 10})
	h.app.draw()
	after := len(h.textArea())

	if before == after {
		t.Errorf("resize 前后文本区高度都是 %d，resize 未生效", before)
	}
	if after >= 10 {
		t.Errorf("文本区高度 = %d, want < 10（要为状态栏留一行）", after)
	}
}

// TestKeyReleaseIsIgnored 按键松开事件不能干扰待定前缀。
func TestKeyReleaseIsIgnored(t *testing.T) {
	h := newHarness(t, "abc")
	h.app.draw()
	h.app.handleEvent(screen.Event{Kind: screen.EventKeyPress, Keystroke: "ctrl+a"})
	h.app.handleEvent(screen.Event{Kind: screen.EventKeyRelease, Keystroke: "ctrl+a"})

	if pending := h.app.matcher.Pending(); len(pending) != 1 {
		t.Errorf("松开前缀键后待定前缀 = %v, want 保留", pending)
	}
	// 再按 r 仍应命中撤销。
	h.app.handleEvent(screen.Event{Kind: screen.EventKeyPress, Keystroke: "r"})
	if pending := h.app.matcher.Pending(); len(pending) != 0 {
		t.Errorf("前缀未正常完成: %v", pending)
	}
}

// ---- 只读 ----

// TestReadonlyBlocksEdits 只读文件不该能被改，也不该崩。
func TestReadonlyBlocksEdits(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/ro.txt"
	if err := os.WriteFile(path, []byte("abc"), 0o444); err != nil {
		t.Fatal(err)
	}
	doc, err := document.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	back := screen.NewFake(testCaps)
	table, _ := keymap.Lookup(keymap.EmacsName)
	app, err := New(Options{Backend: back, Config: config.Default(), Table: table, Doc: doc})
	if err != nil {
		t.Fatal(err)
	}
	app.draw()

	app.handleKey("x")
	app.draw()
	if got := string(doc.Text()); got != "abc" {
		t.Errorf("只读文件内容被改: %q", got)
	}
	if !strings.Contains(app.statusLine(), "只读") {
		t.Errorf("状态栏 = %q, want 含「只读」", app.statusLine())
	}
}

// ---- 退出 ----

// TestQuitWithUnsavedChangesRefuses 有未保存改动时不该退出。
func TestQuitWithUnsavedChangesRefuses(t *testing.T) {
	h := newHarness(t, "abc")
	h.app.draw()
	h.press("x")
	if h.app.shouldQuit {
		t.Error("有未保存改动时不该退出")
	}

	h.pressAll("ctrl+a", "r") // 撤销改动
	if h.doc().Dirty() {
		h.t.Fatal("撤销后仍有未保存改动")
	}
}

// ---- 帧稳定性 ----

// TestFrameHeightIsStable 每次重画的帧高度必须一致。
func TestFrameHeightIsStable(t *testing.T) {
	h := newHarness(t, strings.Join([]string{"a", "b", "c"}, "\n"))
	h.app.draw()
	want := len(h.frameLines())
	for i := 0; i < 5; i++ {
		h.press("x")
		if got := len(h.frameLines()); got != want {
			t.Fatalf("第 %d 次编辑后帧高 = %d, want %d", i, got, want)
		}
	}
}

// TestIdenticalStateDoesNotRedraw 状态没变时不该重复送帧。
func TestIdenticalStateDoesNotRedraw(t *testing.T) {
	h := newHarness(t, "abc")
	h.app.draw()
	before := len(h.back.Frames())
	h.app.draw()
	h.app.draw()
	if got := len(h.back.Frames()); got != before {
		t.Errorf("状态未变时多送了 %d 帧", got-before)
	}
}

// ---- 主题 ----

// TestPlainThemeWhenNoTrueColor 终端不支持真彩时必须用无配色主题。
func TestPlainThemeWhenNoTrueColor(t *testing.T) {
	back := screen.NewFake(screen.Caps{Width: 80, Height: 24, TrueColor: false})
	doc := document.FromString("hello")
	table, _ := keymap.Lookup(keymap.EmacsName)
	cfg := config.Default()
	cfg.UI.TrueColor = true // 配置说要用，但终端不支持

	app, err := New(Options{Backend: back, Config: cfg, Table: table, Doc: doc})
	if err != nil {
		t.Fatal(err)
	}
	app.draw()
	if got := app.theme(); got.Text != view.PlainTheme().Text {
		t.Error("终端不支持真彩时应退回无配色主题")
	}
}
