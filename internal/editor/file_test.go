// file_test.go — 文件操作、标签页、查找、浮层的单元测试。
// SPDX-License-Identifier: MIT

package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/29anan29/Go-Studio/internal/config"
	"github.com/29anan29/Go-Studio/internal/document"
	"github.com/29anan29/Go-Studio/internal/editor/browser"
	"github.com/29anan29/Go-Studio/internal/keymap"
	"github.com/29anan29/Go-Studio/internal/screen"
	"github.com/29anan29/Go-Studio/internal/view"
)

// clearPrompt 把提示浮层里的内容清空。
// 打开文件的浮层会预填当前目录，测试要输入自己的路径就得先清掉。
func clearPrompt(h *harness) {
	h.t.Helper()
	p := h.app.overlay.prompt
	for range p.text() {
		h.app.handleKey("<backspace>")
	}
}

// writeFile 造一个测试文件。
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写文件失败 %s: %v", path, err)
	}
}

// ---- 打开文件 ----

// TestOpenFileThroughPrompt 走完整的浮层交互：打开浮层、敲路径、回车。
func TestOpenFileThroughPrompt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "target.txt")
	writeFile(t, path, "hello file")

	h := newHarness(t, "")
	h.app.draw()
	h.pressAll("ctrl+a", "f")

	if h.app.overlay == nil || h.app.overlay.kind != overlayPrompt {
		t.Fatal("C-a f 之后没有打开输入浮层")
	}

	// 浮层预填了当前目录，先清空再输入完整路径。
	clearPrompt(h)
	for _, r := range path {
		h.app.handleKey(string(r))
	}
	if got := h.app.overlay.prompt.text(); got != path {
		t.Fatalf("浮层输入 = %q, want %q", got, path)
	}
	h.app.handleKey("<enter>")

	if h.app.overlay != nil {
		t.Error("回车之后浮层没有关闭")
	}
	if h.doc().Path() != path {
		t.Errorf("当前文档路径 = %q, want %q", h.doc().Path(), path)
	}
	if got := string(h.doc().Text()); got != "hello file" {
		t.Errorf("文档内容 = %q, want %q", got, "hello file")
	}
}

// TestOpenMissingFileCreatesNamedDocument 文件不存在时要造具名空文档，
// 否则保存时无处可写——「打开新文件再保存」这条路会断掉。
func TestOpenMissingFileCreatesNamedDocument(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "brand-new.txt")

	h := newHarness(t, "")
	h.app.draw()
	h.app.openPath(path)

	if h.doc().Path() != path {
		t.Errorf("新文档路径 = %q, want %q（必须保留路径才能保存）", h.doc().Path(), path)
	}
	if got := string(h.doc().Text()); got != "" {
		t.Errorf("新文档内容 = %q, want 空", got)
	}
	if h.doc().Dirty() {
		t.Error("刚创建的空文档不该算作有未保存改动")
	}

	// 敲入内容后必须能真的保存到那个路径。
	h.app.handleKey("x")
	if err := h.doc().Save(); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读回文件失败: %v", err)
	}
	if string(got) != "x" {
		t.Errorf("磁盘内容 = %q, want %q", got, "x")
	}
}

// TestOpenDirectoryLaunchesBrowser 传目录要转成浏览浮层，对应 spec 一.2。
func TestOpenDirectoryLaunchesBrowser(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"), "a")
	writeFile(t, filepath.Join(dir, "b.txt"), "b")

	h := newHarness(t, "")
	h.app.draw()
	h.app.openPath(dir)

	if h.app.overlay == nil || h.app.overlay.kind != overlayBrowser {
		t.Fatal("打开目录后没有进入浏览浮层")
	}
	if h.app.overlay.browser.Dir != dir {
		t.Errorf("浏览目录 = %q, want %q", h.app.overlay.browser.Dir, dir)
	}
}

// TestOpenSameFileTwiceSwitchesInstead 当前文件已打开时不重复开标签，
// 两份拷贝互相覆盖的问题极难排查。
func TestOpenSameFileTwiceSwitchesInstead(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	writeFile(t, a, "AAA")
	writeFile(t, b, "BBB")

	h := newHarness(t, "")
	h.app.draw()
	h.app.openPath(a)
	h.app.openPath(b)
	if got := h.app.tabs.count(); got != 2 {
		t.Fatalf("标签数 = %d, want 2", got)
	}

	h.app.openPath(a)
	if got := h.app.tabs.count(); got != 2 {
		t.Errorf("重复打开同一文件后标签数 = %d, want 2（不该重复开）", got)
	}
	if h.doc().Path() != a {
		t.Errorf("当前文档 = %q, want %q（应切回已打开的那个）", h.doc().Path(), a)
	}
}

func TestOpenPathErrorsAreReported(t *testing.T) {
	dir := t.TempDir()
	// 父目录不存在，写不进去——这才是真的打不开。
	bad := filepath.Join(dir, "no-such-dir", "a.txt")

	h := newHarness(t, "")
	h.app.draw()
	h.app.openPath(bad)

	if h.app.currentStatus() == "" {
		t.Error("失败时没有给出任何提示")
	}
	if h.doc().Path() != "" {
		t.Errorf("失败后当前文档变成了 %q", h.doc().Path())
	}

	// 空路径同样要给出提示。
	h.app.openPath("")
	if h.app.currentStatus() == "" {
		t.Error("空路径没有给出提示")
	}
}

// TestOpenBinaryFileIsRefused 二进制文件不能按文本打开。
func TestOpenBinaryFileIsRefused(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bin.dat")
	if err := os.WriteFile(path, []byte{0x7f, 'E', 'L', 'F', 0, 1}, 0o644); err != nil {
		t.Fatal(err)
	}

	h := newHarness(t, "")
	h.app.draw()
	h.app.openPath(path)

	if !strings.Contains(h.app.currentStatus(), "打开失败") {
		t.Errorf("状态栏 = %q, want 提到打开失败", h.app.currentStatus())
	}
}

// ---- 另存为 ----

// TestSaveAsChangesPathAndDisk 另存为要同时更新内存里的路径与磁盘内容。
func TestSaveAsChangesPathAndDisk(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "saved.txt")

	h := newHarness(t, "")
	h.app.draw()
	h.app.handleKey("h")
	h.app.handleKey("i")
	h.app.saveAsPath(target)

	if h.doc().Path() != target {
		t.Errorf("路径 = %q, want %q", h.doc().Path(), target)
	}
	if h.doc().Dirty() {
		t.Error("另存为之后不该再有未保存改动")
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("读回失败: %v", err)
	}
	if string(got) != "hi" {
		t.Errorf("磁盘内容 = %q, want %q", got, "hi")
	}
	// 标签名要跟着变，否则状态栏会一直显示旧名字。
	if !strings.Contains(h.app.statusLine(), "saved.txt") {
		t.Errorf("状态栏 = %q, want 含新文件名", h.app.statusLine())
	}
}

func TestSaveAsRejectsEmptyPath(t *testing.T) {
	h := newHarness(t, "")
	h.app.saveAsPath("")

	if !strings.Contains(h.app.currentStatus(), "路径为空") {
		t.Errorf("状态栏 = %q, want 提到路径为空", h.app.currentStatus())
	}
}

// TestSaveAsFailureIsReported 写不进去时要报错，不能吞掉。
func TestSaveAsFailureIsReported(t *testing.T) {
	h := newHarness(t, "")
	h.app.handleKey("x")
	h.app.saveAsPath("/definitely/not/here/a.txt")

	if !strings.Contains(h.app.currentStatus(), "失败") {
		t.Errorf("状态栏 = %q, want 提到失败", h.app.currentStatus())
	}
}

// TestSaveAsOnReadonlyRefused 只读文档不能另存。
func TestSaveAsOnReadonlyRefused(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ro.txt")
	writeFile(t, path, "abc")
	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatal(err)
	}

	doc, err := document.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	h := newHarness(t, "")
	h.app.doc = doc
	h.app.tabs.docs[0] = doc

	h.app.saveAs(keymap.Binding{})
	if !strings.Contains(h.app.currentStatus(), "只读") {
		t.Errorf("状态栏 = %q, want 提到只读", h.app.currentStatus())
	}
}

// ---- 标签页 ----

func TestTabNavigation(t *testing.T) {
	dir := t.TempDir()
	paths := []string{
		filepath.Join(dir, "one.txt"),
		filepath.Join(dir, "two.txt"),
		filepath.Join(dir, "three.txt"),
	}
	for _, p := range paths {
		writeFile(t, p, "x")
	}

	h := newHarness(t, "")
	h.app.draw()
	for _, p := range paths {
		h.app.openPath(p)
	}
	if got := h.app.tabs.count(); got != 3 {
		t.Fatalf("标签数 = %d, want 3", got)
	}
	if got := h.app.tabs.active; got != 2 {
		t.Fatalf("当前标签 = %d, want 2（刚打开的）", got)
	}

	// M-2 切到第 2 个。
	h.pressAll("alt+2")
	if got := h.doc().Path(); got != paths[1] {
		t.Errorf("M-2 之后当前 = %q, want %q", got, paths[1])
	}

	// M-9 跳到最后一个。
	h.pressAll("alt+9")
	if got := h.doc().Path(); got != paths[2] {
		t.Errorf("M-9 之后当前 = %q, want %q", got, paths[2])
	}

	// C-a ] 下一个、C-a [ 上一个。
	h.pressAll("ctrl+a", "[")
	if got := h.doc().Path(); got != paths[1] {
		t.Errorf("上一个标签后当前 = %q, want %q", got, paths[1])
	}
	h.pressAll("ctrl+a", "]")
	if got := h.doc().Path(); got != paths[2] {
		t.Errorf("下一个标签后当前 = %q, want %q", got, paths[2])
	}
}

func TestSelectTabOutOfRangeIsReported(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	writeFile(t, a, "a")
	writeFile(t, b, "b")

	h := newHarness(t, "")
	h.app.draw()
	h.app.openPath(a)
	h.app.openPath(b)
	if got := h.app.tabs.count(); got != 2 {
		t.Fatalf("标签数 = %d, want 2", got)
	}

	h.app.selectTab(keymap.Binding{Arg: keymap.Arg("9")})

	if !strings.Contains(h.app.currentStatus(), "不存在") {
		t.Errorf("状态栏 = %q, want 提到标签不存在", h.app.currentStatus())
	}
	// 请求越界不该改变当前标签。
	if got := h.doc().Path(); got != b {
		t.Errorf("越界请求后当前 = %q, want %q（不该切换）", got, b)
	}
}

// TestSelectTabWithSingleTab 只有一个标签时切标签要明说，而不是假装切了。
func TestSelectTabWithSingleTab(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	writeFile(t, a, "a")

	h := newHarness(t, "")
	h.app.draw()
	h.app.openPath(a)
	h.app.selectTab(keymap.Binding{Arg: keymap.Arg("5")})

	if !strings.Contains(h.app.currentStatus(), "1 个标签页") {
		t.Errorf("状态栏 = %q, want 提到只有 1 个标签页", h.app.currentStatus())
	}
}

// TestSwitchingTabResetsViewport 每个标签都要能看到自己的内容。
func TestSwitchingTabResetsViewport(t *testing.T) {
	dir := t.TempDir()
	long := filepath.Join(dir, "long.txt")
	short := filepath.Join(dir, "short.txt")
	writeFile(t, long, strings.Repeat("x\n", 500))
	writeFile(t, short, "one line")

	h := newHarness(t, "")
	h.app.draw()
	h.app.openPath(long)
	h.pressAll("ctrl+<end>")
	h.app.draw()
	if h.app.top == 0 {
		t.Fatal("长文件跳到末尾后视口应滚动")
	}

	h.app.openPath(short)
	h.app.draw()
	if h.app.top != 0 {
		t.Errorf("切到短文件后 top = %d, want 0", h.app.top)
	}
	if got := string(h.doc().Text()); got != "one line" {
		t.Errorf("当前内容 = %q, want %q", got, "one line")
	}
}

func TestCloseTab(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	writeFile(t, a, "a")
	writeFile(t, b, "b")

	h := newHarness(t, "")
	h.app.draw()
	h.app.openPath(a)
	h.app.openPath(b)

	h.pressAll("ctrl+a", "k") // C-a k 关闭标签
	if got := h.app.tabs.count(); got != 1 {
		t.Errorf("关闭后标签数 = %d, want 1", got)
	}
	if got := h.doc().Path(); got != a {
		t.Errorf("关闭后当前 = %q, want %q", got, a)
	}
}

// TestCloseLastTabKeepsAnEditableDocument 关掉最后一个标签后必须补一个空文档，
// 否则编辑器会卡在没有文档的状态。
func TestCloseLastTabKeepsAnEditableDocument(t *testing.T) {
	h := newHarness(t, "")
	h.app.draw()
	h.pressAll("ctrl+a", "k")

	if got := h.app.tabs.count(); got != 1 {
		t.Fatalf("标签数 = %d, want 1", got)
	}
	if h.app.doc == nil {
		t.Fatal("没有可编辑的文档了")
	}
	// 还能编辑。
	h.app.handleKey("z")
	if got := string(h.app.doc.Text()); got != "z" {
		t.Errorf("内容 = %q, want %q", got, "z")
	}
}

// TestTabBarOnlyWhenMultiple 只有一个标签时不画标签栏，那一行没有信息量。
func TestTabBarOnlyWhenMultiple(t *testing.T) {
	h := newHarness(t, "abc")
	h.app.draw()
	if got := h.app.tabs.renderTabs(h.app.theme(), 80); got != "" {
		t.Errorf("单标签时标签栏 = %q, want 空", got)
	}

	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	writeFile(t, a, "a")
	h.app.openPath(a)

	bar := h.app.tabs.renderTabs(h.app.theme(), 80)
	if bar == "" {
		t.Error("多标签时应该画标签栏")
	}
	if view := h.app.tabs.renderTabs(h.app.theme(), 80); len(view) == 0 {
		t.Error("标签栏为空")
	}
}

// TestTabBarFitsWidth 标签栏必须精确占满宽度，否则帧宽不对、终端会折行。
func TestTabBarFitsWidth(t *testing.T) {
	dir := t.TempDir()
	paths := make([]string, 12)
	for i := range paths {
		paths[i] = filepath.Join(dir, strings.Repeat("longname", 3)+string(rune('a'+i))+".txt")
		writeFile(t, paths[i], "x")
	}

	h := newHarness(t, "")
	h.app.draw()
	for _, p := range paths {
		h.app.openPath(p)
	}
	for _, width := range []int{10, 20, 40, 80, 200} {
		bar := h.app.tabs.renderTabs(h.app.theme(), width)
		if got := view.VisibleWidth(bar); got != width {
			t.Errorf("宽度 %d 时标签栏宽 = %d, want %d", width, got, width)
		}
	}
}

// TestTabBarNarrowTerminal 极窄终端下也不能让帧宽超出。
func TestTabBarNarrowTerminal(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a-very-long-file-name-here.txt")
	b := filepath.Join(dir, "b.txt")
	writeFile(t, a, "a")
	writeFile(t, b, "b")

	h := newHarness(t, "")
	h.app.draw()
	h.app.openPath(a)
	h.app.openPath(b)

	for width := 1; width <= 30; width++ {
		bar := h.app.tabs.renderTabs(h.app.theme(), width)
		if got := view.VisibleWidth(bar); got != width {
			t.Errorf("宽度 %d 时标签栏宽 = %d, want %d", width, got, width)
		}
	}
}

// ---- 查找 ----

// TestFindMovesThroughMatches 连按 n 必须走遍所有匹配。
func TestFindMovesThroughMatches(t *testing.T) {
	h := newHarness(t, "foo one\nfoo two\nfoo three")
	h.app.draw()
	h.app.startFind("foo")

	// 第一次匹配在第 0 行。
	if got := h.doc().Cursor().Line; got != 0 {
		t.Fatalf("第一次查找后行 = %d, want 0", got)
	}
	h.press("ctrl+s")
	if got := h.doc().Cursor().Line; got != 1 {
		t.Errorf("第二次后行 = %d, want 1", got)
	}
	h.press("ctrl+s")
	if got := h.doc().Cursor().Line; got != 2 {
		t.Errorf("第三次后行 = %d, want 2", got)
	}
}

// TestFindWrapsAround 找完一遍之后要绕回开头，并说明绕回了。
func TestFindWrapsAround(t *testing.T) {
	h := newHarness(t, "foo\nfoo\nfoo")
	h.app.draw()
	h.app.startFind("foo")

	h.press("ctrl+s")
	h.press("ctrl+s")
	if got := h.app.currentStatus(); strings.Contains(got, "绕到") {
		t.Errorf("还没绕完就提示绕回: %q", got)
	}

	h.press("ctrl+s") // 第三处之后应当绕回第 0 处
	if got := h.doc().Cursor().Line; got != 0 {
		t.Errorf("绕回后行 = %d, want 0", got)
	}
	if got := h.app.currentStatus(); !strings.Contains(got, "绕到") {
		t.Errorf("状态栏 = %q, want 说明绕回了", got)
	}
}

// TestFindPrev 反向查找要能回到前一处。
func TestFindPrev(t *testing.T) {
	h := newHarness(t, "foo\nfoo\nfoo")
	h.app.draw()
	h.app.startFind("foo")
	h.pressAll("ctrl+s", "ctrl+s") // 到第 2 处

	h.press("ctrl+r")
	if got := h.doc().Cursor().Line; got != 1 {
		t.Errorf("反向查找后行 = %d, want 1", got)
	}
	h.press("ctrl+r")
	if got := h.doc().Cursor().Line; got != 0 {
		t.Errorf("再反向查找后行 = %d, want 0", got)
	}
}

// TestFindBeforeCursorIsFound 从光标之前的内容里也能找到。
func TestFindBeforeCursor(t *testing.T) {
	h := newHarness(t, "foo\nbar\nfoo")
	h.app.draw()
	h.app.gotoLineInput("3") // 光标在第三行
	h.app.startFind("foo")

	// 光标前面（第三行自己的 foo）应该先命中。
	if got := h.doc().Cursor().Line; got != 2 {
		t.Errorf("查找后行 = %d, want 2（光标所在处优先）", got)
	}
	h.press("ctrl+s")
	if got := h.doc().Cursor().Line; got != 0 {
		t.Errorf("再找后行 = %d, want 0（绕回开头）", got)
	}
}

func TestFindNotFoundIsReported(t *testing.T) {
	h := newHarness(t, "hello")
	h.app.draw()
	h.app.startFind("nonexistent")

	if !strings.Contains(h.app.currentStatus(), "找不到") {
		t.Errorf("状态栏 = %q, want 提到找不到", h.app.currentStatus())
	}
}

func TestFindEmptyQueryIsRejected(t *testing.T) {
	h := newHarness(t, "hello")
	h.app.draw()
	h.app.startFind("   ")

	if h.app.find != nil {
		t.Error("空查询不该建立查找会话")
	}
	if !strings.Contains(h.app.currentStatus(), "为空") {
		t.Errorf("状态栏 = %q, want 提到为空", h.app.currentStatus())
	}
}

// TestFindNextWithoutQueryOpensPrompt 没有查找串时按 n 要先问用户查什么。
func TestFindNextWithoutQueryOpensPrompt(t *testing.T) {
	h := newHarness(t, "hello foo")
	h.app.draw()
	h.press("ctrl+s")

	if h.app.overlay == nil || h.app.overlay.kind != overlayPrompt {
		t.Fatal("没有查找串时按 n 应该打开输入浮层")
	}
}

// TestFindSeesNewEdits 查找以当前光标为准，编辑后仍然可用。
func TestFindSeesNewEdits(t *testing.T) {
	h := newHarness(t, "aaa")
	h.app.draw()
	h.app.startFind("bbb")
	if !strings.Contains(h.app.currentStatus(), "找不到") {
		t.Fatalf("起初不该找到: %q", h.app.currentStatus())
	}

	// 在末尾补上 bbb。
	h.pressAll("ctrl+<end>")
	for _, r := range "bbb" {
		h.app.handleKey(string(r))
	}
	h.app.startFind("bbb")
	if got := h.doc().Cursor().Line; got != 0 {
		t.Errorf("查找后行 = %d, want 0（新增内容应被找到）", got)
	}
}

// ---- 浮层 ----

// TestPromptSwallowsEditingKeys 浮层打开时编辑键不能穿透到文档。
func TestPromptSwallowsEditingKeys(t *testing.T) {
	h := newHarness(t, "abc")
	h.app.draw()
	h.app.openPrompt(promptOpenFile, "")

	// 这些键在浮层里都不该改动文档。
	for _, key := range []string{"<down>", "<up>", "<left>", "<right>", "<delete>", "<tab>", "<enter>"} {
		before := string(h.doc().Text())
		h.app.handleKey(key)
		if got := string(h.doc().Text()); got != before {
			t.Errorf("浮层里按 %q 把文档改成了 %q", key, got)
		}
	}
}

// TestPromptTypesText 浮层里打到的字符都进输入框。
func TestPromptTypesText(t *testing.T) {
	h := newHarness(t, "")
	h.app.draw()
	h.app.openPrompt(promptOpenFile, "")

	for _, r := range "ab c中" {
		h.app.handleKey(string(r))
	}
	if got := h.app.overlay.prompt.text(); got != "ab c中" {
		t.Errorf("输入框内容 = %q, want %q", got, "ab c中")
	}
	// 文档不受影响。
	if got := string(h.doc().Text()); got != "" {
		t.Errorf("文档内容 = %q, want 空", got)
	}
}

// TestPromptEscapeCancels Esc 取消浮层且不产生任何副作用。
func TestPromptEscapeCancels(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.txt")
	writeFile(t, path, "content")

	h := newHarness(t, "")
	h.app.draw()
	h.app.openPrompt(promptOpenFile, "")
	for _, r := range path {
		h.app.handleKey(string(r))
	}
	h.app.handleKey("<esc>")

	if h.app.overlay != nil {
		t.Error("Esc 之后浮层没有关闭")
	}
	if h.doc().Path() != "" {
		t.Errorf("取消之后却打开了 %q", h.doc().Path())
	}
}

// TestPromptEscapeClearsMatcherPrefix 取消浮层要清掉半截的按键前缀，
// 否则关闭后第一键会落到错误的上下文里。
func TestPromptEscapeClearsMatcherPrefix(t *testing.T) {
	h := newHarness(t, "abc")
	h.app.draw()
	h.app.handleKey("ctrl+a") // 前缀待定
	if len(h.app.matcher.Pending()) == 0 {
		t.Fatal("前缀没有进入待定状态")
	}
	h.app.openPrompt(promptFind, "")
	h.app.handleKey("<esc>")

	if len(h.app.matcher.Pending()) != 0 {
		t.Errorf("取消浮层后仍有待定前缀: %v", h.app.matcher.Pending())
	}
}

// TestPromptPrefillDirectory 打开文件的浮层预填当前目录。
func TestPromptPrefillDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cur.txt")
	writeFile(t, path, "x")

	h := newHarness(t, "")
	h.app.openPath(path)
	h.app.openFile(keymap.Binding{})

	if h.app.overlay == nil || h.app.overlay.kind != overlayPrompt {
		t.Fatal("没有打开浮层")
	}
	if got := h.app.overlay.prompt.text(); got != dir {
		t.Errorf("预填 = %q, want 当前目录 %q", got, dir)
	}
}

// ---- 目录浏览浮层 ----

func TestBrowserOpensAndEntersDirectory(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "file.txt"), "f")
	writeFile(t, filepath.Join(sub, "inner.txt"), "i")

	h := newHarness(t, "")
	h.app.draw()
	h.app.openPath(root)

	b := h.app.overlay.browser
	if b == nil {
		t.Fatal("没有浏览浮层")
	}
	// 第一项是 ".."，第二项应是 sub 目录（目录排在文件前）。
	if b.Entries[0].Name != ".." {
		t.Errorf("首项 = %q, want %q", b.Entries[0].Name, "..")
	}
	if !b.Entries[1].Dir || b.Entries[1].Name != "sub" {
		t.Errorf("第二项 = %+v, want 目录 sub", b.Entries[1])
	}

	// 第一项是 ".."，要先选中 sub 才能用右箭头进入它。
	b.MoveTo(1)
	if got, _ := b.Selected(); got.Name != "sub" {
		t.Fatalf("选中 = %q, want %q", got.Name, "sub")
	}
	h.app.handleKey("<right>")
	if h.app.overlay == nil || h.app.overlay.kind != overlayBrowser {
		t.Fatal("进入目录后浏览浮层不见了")
	}
	if h.app.overlay.browser.Dir != sub {
		t.Errorf("进入后目录 = %q, want %q", h.app.overlay.browser.Dir, sub)
	}
}

// TestBrowserOpensSelectedFile 在文件上按右箭头要真的打开文件。
func TestBrowserOpensSelectedFile(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target.txt")
	writeFile(t, target, "content here")

	h := newHarness(t, "")
	h.app.draw()
	h.app.openPath(root)

	b := h.app.overlay.browser
	// 找到 target.txt 并选中它。
	index := -1
	for i, entry := range b.Entries {
		if entry.Name == "target.txt" {
			index = i
			break
		}
	}
	if index < 0 {
		t.Fatalf("列表里没有 target.txt：%+v", b.Entries)
	}
	b.MoveTo(index)

	h.app.handleKey("<right>")

	if h.app.overlay != nil {
		t.Error("打开文件之后浮层没有关闭")
	}
	if h.doc().Path() != target {
		t.Errorf("当前文档 = %q, want %q", h.doc().Path(), target)
	}
	if got := string(h.doc().Text()); got != "content here" {
		t.Errorf("内容 = %q, want %q", got, "content here")
	}
}

// TestBrowserGoUpLeftArrow 左箭头返回上级。
func TestBrowserGoUpLeftArrow(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	h := newHarness(t, "")
	h.app.draw()
	h.app.openPath(sub)
	if h.app.overlay.browser.Dir != sub {
		t.Fatalf("起始目录 = %q, want %q", h.app.overlay.browser.Dir, sub)
	}

	h.app.handleKey("<left>")
	if got := h.app.overlay.browser.Dir; got != root {
		t.Errorf("返回上级后 = %q, want %q", got, root)
	}
}

// TestBrowserEscapeCloses Esc 关闭浏览浮层且不打开任何文件。
func TestBrowserEscapeCloses(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.txt"), "a")

	h := newHarness(t, "")
	h.app.draw()
	h.app.openPath(root)
	h.app.handleKey("<esc>")

	if h.app.overlay != nil {
		t.Error("Esc 之后浮层没有关闭")
	}
	if h.doc().Path() != "" {
		t.Errorf("取消之后却打开了 %q", h.doc().Path())
	}
}

// TestBrowserMissingDirIsReported 列目录失败要有说明，不能空白一片。
func TestBrowserMissingDirIsReported(t *testing.T) {
	b := browser.New(filepath.Join(t.TempDir(), "no-such-dir"))
	if b.Err == "" {
		t.Error("读取不存在的目录时没有给出说明")
	}
	if len(b.Entries) != 0 {
		t.Errorf("失败时列出了 %d 项，want 0", len(b.Entries))
	}
}

// TestBrowserRenderKeepsHeight 渲染结果的行数必须与请求一致，
// 否则会撑破布局把状态栏顶掉。
func TestBrowserRenderKeepsHeight(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 30; i++ {
		writeFile(t, filepath.Join(root, string(rune('a'+i))+".txt"), "x")
	}
	b := browser.New(root)

	h := newHarness(t, "")
	theme := h.app.theme()
	for _, size := range []int{3, 5, 10, 24} {
		for _, width := range []int{10, 40, 80} {
			content, _ := b.Render(theme, width, size)
			lines := strings.Split(content, "\n")
			if len(lines) != size {
				t.Errorf("高度 %d、宽度 %d 时渲染 %d 行, want %d", size, width, len(lines), size)
			}
			for i, line := range lines {
				if got := view.VisibleWidth(line); got > width {
					t.Errorf("高度 %d、宽度 %d 时第 %d 行宽 = %d，超出上限", size, width, i, got)
				}
			}
		}
	}
}

// ---- 帧稳定性 ----

// TestFrameHeightStableAcrossStates 各种界面状态下帧高都必须一致，
// 否则终端会不停滚动。
func TestFrameHeightStableAcrossStates(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	writeFile(t, a, "a")
	writeFile(t, b, "b")

	h := newHarness(t, "")
	h.app.draw()
	want := len(h.frameLines())

	states := []func(){
		func() { h.app.handleKey("x") },
		func() { h.app.openPrompt(promptOpenFile, "") },
		func() { h.app.handleKey("<esc>") },
		func() { h.app.openPath(a) },
		func() { h.app.openPath(b) },
		func() { h.app.openPath(dir) },
		func() { h.app.handleKey("<esc>") },
	}
	for i, setup := range states {
		setup()
		h.app.draw()
		if got := len(h.frameLines()); got != want {
			t.Errorf("状态 %d 之后帧高 = %d, want %d", i, got, want)
		}
	}
}

// TestFrameWidthIsExact 每一帧的每一行宽度都必须精确等于终端宽度。
func TestFrameWidthIsExact(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"short.txt", "中文字符名称的文件.txt"} {
		writeFile(t, filepath.Join(dir, name), strings.Repeat("很长的中文内容", 40))
	}

	h := newHarness(t, "")
	h.app.draw()

	widths := []int{20, 40, 80}
	for _, width := range widths {
		h.app.width = width
		h.app.draw()
		for i, line := range h.frameLines() {
			if got := view.VisibleWidth(line); got != width {
				t.Errorf("宽度 %d 时第 %d 行宽 = %d, want %d", width, i, got, width)
			}
		}
	}

	// 打开浮层与浏览浮层时同样要精确。
	h.app.width = 40
	h.app.openPrompt(promptFind, strings.Repeat("很长的查询", 20))
	h.app.draw()
	for i, line := range h.frameLines() {
		if got := view.VisibleWidth(line); got != 40 {
			t.Errorf("浮层状态下第 %d 行宽 = %d, want 40", i, got)
		}
	}
	h.app.handleKey("<esc>")

	h.app.openPath(dir)
	h.app.draw()
	for i, line := range h.frameLines() {
		if got := view.VisibleWidth(line); got != 40 {
			t.Errorf("浏览浮层下第 %d 行宽 = %d, want 40", i, got)
		}
	}
}

// TestCursorStaysInBounds 无论界面处于什么状态，光标坐标都不能越出帧。
func TestCursorStaysInBounds(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"), strings.Repeat("x\n", 100))

	for _, size := range []screen.Caps{{Width: 80, Height: 24}, {Width: 20, Height: 6}, {Width: 5, Height: 3}} {
		back := screen.NewFake(size)
		doc := document.FromString(strings.Repeat("abc\n", 20))
		table, _ := keymap.Lookup(keymap.EmacsName)
		app, err := New(Options{
			Backend: back,
			Config:  config.Default(),
			Table:   table,
			Doc:     doc,
		})
		if err != nil {
			t.Fatal(err)
		}
		app.gotoLineInput("20")
		app.draw()

		frame := back.LastFrame()
		if frame.Cursor == nil {
			t.Errorf("尺寸 %dx%d 下没有光标", size.Width, size.Height)
			continue
		}
		if frame.Cursor.X < 0 || frame.Cursor.X >= size.Width {
			t.Errorf("尺寸 %dx%d 下光标 X = %d 越界", size.Width, size.Height, frame.Cursor.X)
		}
		if frame.Cursor.Y < 0 || frame.Cursor.Y >= size.Height {
			t.Errorf("尺寸 %dx%d 下光标 Y = %d 越界", size.Width, size.Height, frame.Cursor.Y)
		}
	}
}
