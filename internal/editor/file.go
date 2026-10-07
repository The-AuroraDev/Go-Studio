// file.go — 文件与浮层相关的命令：打开、另存为、关闭标签、查找、目录浏览。
// SPDX-License-Identifier: MIT

package editor

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/29anan29/Go-Studio/internal/document"
	"github.com/29anan29/Go-Studio/internal/editor/browser"
	"github.com/29anan29/Go-Studio/internal/keymap"
)

// ---- 打开文件 ----

// openFile 打开文件浮层。目录会转成浏览浮层，
// 对应 spec 一.2「如果对应的是一个文件夹那么就把文件夹内的文件列出来」。
func (a *App) openFile(_ keymap.Binding) {
	prefill := ""
	if dir := a.workingDir(); dir != "" {
		// 预填当前文件所在目录：用户接着输文件名即可，不用重打整条路径。
		prefill = dir
	}
	a.openPrompt(promptOpenFile, prefill)
}

// workingDir 返回当前文档所在目录，没有文档路径时返回工作目录。
func (a *App) workingDir() string {
	if path := a.doc.Path(); path != "" {
		return filepath.Dir(path)
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return ""
}

// openPrompt 打开一个输入浮层。
func (a *App) openPrompt(kind promptKind, prefill string) {
	a.matcher.Reset()
	a.overlay = &overlay{kind: overlayPrompt, prompt: newPrompt(kind, prefill)}
}

// handlePromptKey 处理提示浮层里的按键。
func (a *App) handlePromptKey(key string) {
	p := a.overlay.prompt
	if !p.handleKey(key) {
		return
	}
	switch key {
	case "<esc>":
		a.closeOverlay()
		a.setStatus("已取消")
		return
	case "<enter>":
		input := p.text()
		kind := p.confirmKind()
		a.closeOverlay()
		a.submitPrompt(kind, input)
	}
}

// submitPrompt 处理浮层确认后的输入。
func (a *App) submitPrompt(kind promptKind, input string) {
	switch kind {
	case promptOpenFile:
		a.openPath(resolvePath(input))
	case promptSaveAs:
		a.saveAsPath(resolvePath(input))
	case promptGotoLine:
		a.gotoLineInput(input)
	case promptFind:
		a.startFind(input)
	}
}

// openPath 打开一个路径。文件直接打开，目录转成浏览浮层。
func (a *App) openPath(path string) {
	if path == "" {
		a.setStatus("路径为空")
		return
	}
	info, err := os.Stat(path)
	switch {
	case err == nil && info.IsDir():
		a.openBrowserAt(path)
		return

	case err != nil && a.canCreateNew(path, err):
		// 文件还不存在、但父目录在：这是「新建文件」的场景，
		// 造一个具名空文档，用户敲完内容直接保存即可。
		doc := document.NewNamed(path, false)
		a.tabs.open(doc)
		a.syncActive()
		a.setStatus("新建 " + displayName(path))
		return

	case err != nil:
		// 权限不足、父目录不存在、坏符号链接：真的打不开。
		a.setStatus("打不开：" + err.Error())
		return
	}

	// 已经打开过就直接切过去，不重复开标签：
	// 同一个文件开两份拷贝，保存时会互相覆盖。
	if existing := a.tabs.indexOf(path); existing >= 0 {
		a.tabs.selectN(existing + 1)
		a.syncActive()
		a.setStatus("已切到 " + displayName(path))
		return
	}

	doc, err := a.loadDocument(path)
	if err != nil {
		a.setStatus("打开失败：" + err.Error())
		return
	}
	a.tabs.open(doc)
	a.syncActive()
	a.setStatus("已打开 " + displayName(path))
}

// canCreateNew 判断这个不存在的路径是否可以当成「新建文件」。
//
// 判据是父目录存不存在：父目录在，说明用户只是还没建这个文件；
// 父目录都不在，之后保存必然失败，那属于真的打不开，
// 不该给用户一个看起来打开了、实际存不下去的文档。
func (a *App) canCreateNew(path string, statErr error) bool {
	if !errors.Is(statErr, fs.ErrNotExist) {
		return false
	}
	parent := filepath.Dir(path)
	info, err := os.Stat(parent)
	return err == nil && info.IsDir()
}

// loadDocument 读取路径，文件不存在时造一个具名的空文档。
//
// 新建文件必须保留路径：退回无名文档会让「打开新文件再保存」这条路断掉，
// 因为保存时无处可写。
func (a *App) loadDocument(path string) (*document.Document, error) {
	doc, err := document.Open(path)
	if err == nil {
		return doc, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return document.NewNamed(path, false), nil
	}
	return nil, err
}

// openBrowserAt 在指定目录打开浏览浮层。
func (a *App) openBrowserAt(dir string) {
	a.matcher.Reset()
	b := browser.New(dir)
	a.overlay = &overlay{kind: overlayBrowser, browser: b}
}

// handleBrowserKey 处理浏览浮层里的按键。
func (a *App) handleBrowserKey(key string) {
	switch b := a.overlay.browser.HandleKey(key); b {
	case browser.Close:
		a.closeOverlay()
		a.setStatus("已取消")

	case browser.Open:
		path := a.overlay.browser.CurrentPath()
		a.closeOverlay()
		a.openPath(path)

	case browser.Enter:
		a.overlay.browser.Enter()

	case browser.Up:
		a.overlay.browser.GoUp()
	}
}

// ---- 保存 ----

// saveAs 打开另存为浮层。
func (a *App) saveAs(_ keymap.Binding) {
	if a.doc.Readonly() {
		a.setStatus("文件是只读的")
		return
	}
	prefill := a.doc.Path()
	if prefill == "" {
		prefill = a.workingDir()
	}
	a.openPrompt(promptSaveAs, prefill)
}

// saveAsPath 另存到指定路径。
func (a *App) saveAsPath(path string) {
	if path == "" {
		a.setStatus("路径为空")
		return
	}
	if err := a.doc.SaveAs(path); err != nil {
		a.setStatus("另存为失败：" + err.Error())
		a.error("editor: save-as failed", "path", path, "error", err.Error())
		return
	}
	// SaveAs 原地改了文档的 Path()，标签数组本身不用动：
	// 标签名与去重索引都是现算的，下一次渲染就会显示新名字。
	// 但扩展名可能跟着变（main.go 另存为 main.py），语言规则要重认。
	a.rebuildHighlighter()
	a.setStatus("已另存为 " + displayName(path))
}

// ---- 标签 ----

// selectTab 切到指定序号的标签。
func (a *App) selectTab(b keymap.Binding) {
	n, err := strconv.Atoi(string(b.Arg))
	if err != nil || n < 1 {
		a.setStatus("标签序号无效")
		return
	}
	if a.tabs.count() <= 1 {
		a.setStatus("当前只有 1 个标签页（请求第 " + string(b.Arg) + " 个）")
		return
	}
	if !a.tabs.selectN(n) {
		a.setStatus("第 " + string(b.Arg) + " 个标签不存在，当前共 " +
			strconv.Itoa(a.tabs.count()) + " 个")
		return
	}
	a.syncActive()
	a.setStatus("切到 " + displayName(a.doc.Path()))
}

// lastTab 切到最后一个标签，对应 spec 的 M-9。
func (a *App) lastTab(_ keymap.Binding) {
	if a.tabs.count() <= 1 {
		a.setStatus("当前只有 1 个标签页")
		return
	}
	a.tabs.selectLast()
	a.syncActive()
	a.setStatus("切到 " + displayName(a.doc.Path()))
}

// closeTab 关闭当前标签。
func (a *App) closeTab(_ keymap.Binding) {
	closed := a.doc
	doc, ok := a.tabs.close()
	if !ok {
		return
	}
	a.syncActive()
	name := displayName(closed.Path())
	if name == "" {
		name = "[未命名]"
	}
	a.setStatus("已关闭 " + name + "，当前 " + displayName(doc.Path()))
}

// nextTab 切到下一个标签。
func (a *App) nextTab(_ keymap.Binding) {
	if a.tabs.count() <= 1 {
		a.setStatus("当前只有 1 个标签页")
		return
	}
	a.tabs.selectN(a.tabs.active + 2)
	a.syncActive()
}

// prevTab 切到上一个标签。
func (a *App) prevTab(_ keymap.Binding) {
	if a.tabs.count() <= 1 {
		a.setStatus("当前只有 1 个标签页")
		return
	}
	a.tabs.selectN(maxInt(a.tabs.active, 1))
	a.syncActive()
}

// ---- 跳转到行 ----

// gotoLine 打开跳转到行的输入浮层。
func (a *App) gotoLine(_ keymap.Binding) {
	prefill := strconv.Itoa(a.doc.Cursor().Line + 1)
	a.openPrompt(promptGotoLine, prefill)
}

// gotoLineInput 处理跳转行号的输入。
func (a *App) gotoLineInput(input string) {
	text := strings.TrimSpace(input)
	if text == "" {
		a.setStatus("没有输入行号")
		return
	}
	line, err := strconv.Atoi(text)
	if err != nil {
		a.setStatus("行号无效：" + text)
		return
	}
	// 状态栏与行号显示都从 1 开始，内部行号从 0 开始。
	a.doc.GotoLine(line - 1)
	a.setStatus("跳转到第 " + text + " 行")
}

// ---- 查找 ----

// startFind 记下查找串，并立刻跳到第一处匹配。
func (a *App) startFind(query string) {
	text := strings.TrimSpace(query)
	if text == "" {
		a.setStatus("查找内容为空")
		return
	}
	a.find = a.newSearch(text)
	if !a.findNext() {
		a.setStatus("找不到 " + text)
		return
	}
	a.setStatus("找到 " + text)
}
