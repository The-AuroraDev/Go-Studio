// app.go — 编辑器主循环：事件进来、命令分派、状态重绘。
// SPDX-License-Identifier: MIT

// Package editor 把 screen、keymap、document、view 装配成一个可运行的编辑器。
//
// 它是唯一同时持有「终端」「键位表」「文档」「视口」的地方，也是唯一做决策的层：
// 其余三个包都只管自己那一片，彼此之间没有横向依赖。
//
// 主循环刻意保持极简：读事件 → 更新状态 → 送一帧。没有脏标记、没有帧率节流、
// 没有 diff 合并——screen 层已经会丢弃与上一帧完全相同的内容，
// 而一次按键只可能产生一帧，多加一层缓存只会引入不一致的窗口。
package editor

import (
	"context"
	"fmt"
	"time"

	"github.com/29anan29/Go-Studio/internal/config"
	"github.com/29anan29/Go-Studio/internal/document"
	"github.com/29anan29/Go-Studio/internal/editor/browser"
	"github.com/29anan29/Go-Studio/internal/keymap"
	"github.com/29anan29/Go-Studio/internal/log"
	"github.com/29anan29/Go-Studio/internal/screen"
	"github.com/29anan29/Go-Studio/internal/syntax"
	"github.com/29anan29/Go-Studio/internal/view"
)

// heartbeatInterval 是心跳日志的间隔，用来确认主循环还活着。
const heartbeatInterval = 30 * time.Second

// statusTimeout 是状态栏提示自动消失的时长。
const statusTimeout = 3 * time.Second

// statusBarHeight 是状态栏占用的行数。
const statusBarHeight = 1

// App 是编辑器实例。
type App struct {
	backend screen.Screen
	logger  *log.Logger
	cfg     config.Config
	table   *keymap.Table
	matcher *keymap.Matcher

	tabs *tabs
	// doc 是当前文档的快捷方式，等价于 tabs.current()。
	// 缓存它是为了让命令处理函数写起来短；任何切换标签的操作
	// 都必须调用 syncActive 刷新它。
	doc *document.Document

	// hl 是当前文档的语法高亮缓存，为 nil 表示不高亮。
	// 它按文档维护：切换标签时整体重建，而不是每个标签各留一份，
	// 那样内存会随打开的文件数一直涨。
	hl *syntax.Highlighter
	// hlRev 是 hl 已同步到的文档版本号，用来发现自上次绘制以来的改动。
	hlRev uint64
	// hlLang 是 hl 当前使用的语言名，用于在状态栏显示识别结果。
	hlLang string

	top  int
	mode string

	// find 是当前的查找会话，nil 表示还没开始查找。
	find *search

	// status 是状态栏提示，statusUntil 是它自动消失的时刻。
	status      string
	statusUntil time.Time

	// overlay 是当前打开的浮层，nil 表示没有。
	// 三种浮层互斥：同时开两个会让按键归属变得无法判断。
	overlay *overlay

	// now 可注入，用于测试里控制状态栏超时。
	now func() time.Time

	width, height int

	shouldQuit bool
	// lastFrame 记住上一帧的完整内容，用来跳过无变化的帧。
	lastFrame string
}

// overlayKind 区分浮层类型。
type overlayKind uint8

const (
	overlayNone overlayKind = iota
	overlayPrompt
	overlayBrowser
)

// overlay 是当前浮层。三种浮层的按键处理完全不同，
// 但对 App 来说只需要知道「有没有浮层」以及「把按键交给谁」。
type overlay struct {
	kind    overlayKind
	prompt  *prompt
	browser *browser.Browser
}

// Options 是构造 App 所需的全部输入。
type Options struct {
	// Backend 是终端后端，不可为 nil。
	Backend screen.Screen
	// Logger 用于记录日志，可为 nil（此时不记日志）。
	Logger *log.Logger
	// Config 是生效的配置。
	Config config.Config
	// Table 是键位方案。
	Table *keymap.Table
	// Doc 是初始文档，不可为 nil。
	Doc *document.Document
	// Status 是初始状态栏文字。
	Status string
	// Now 可注入时钟，用于测试。
	Now func() time.Time
}

// New 构造 App。
func New(opts Options) (*App, error) {
	if opts.Backend == nil {
		return nil, fmt.Errorf("editor: 缺少终端后端")
	}
	if opts.Doc == nil {
		return nil, fmt.Errorf("editor: 缺少初始文档")
	}
	if opts.Table == nil {
		return nil, fmt.Errorf("editor: 缺少键位方案")
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	caps := opts.Backend.Caps()
	a := &App{
		backend: opts.Backend,
		logger:  opts.Logger,
		cfg:     opts.Config,
		table:   opts.Table,
		matcher: keymap.NewMatcher(opts.Table),
		tabs:    newTabs(opts.Doc),
		doc:     opts.Doc,
		mode:    "NORMAL",
		status:  opts.Status,
		now:     now,
		width:   caps.Width,
		height:  caps.Height,
	}
	// 启动就要有高亮：这是打开命令行参数指定的文件那条路径，
	// 不经过 syncActive，不在这里建的话首个文件永远不高亮。
	a.rebuildHighlighter()
	return a, nil
}

// Run 是主循环，直到上下文取消或收到退出命令。
func (a *App) Run(ctx context.Context) error {
	a.syncSize(a.backend.Caps())
	a.draw()

	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	events := a.backend.Events()
	for {
		select {
		case <-ctx.Done():
			a.debug("editor: shutting down")
			return nil

		case <-ticker.C:
			a.debug("editor: loop alive")

		case event, ok := <-events:
			if !ok {
				a.info("editor: terminal closed")
				return nil
			}
			a.handleEvent(event)
			if a.shouldQuit {
				return nil
			}
			a.draw()
		}
	}
}

// handleEvent 把一个终端事件翻译成状态变化。
func (a *App) handleEvent(event screen.Event) {
	switch event.Kind {
	case screen.EventKeyPress:
		a.handleKey(event.Keystroke)
	case screen.EventKeyRelease:
		// 按键松开不参与命令解析：bubble.go 打开了 ReportEventTypes，
		// 前缀键的松开事件也走这里。若不忽略，松开 C-a 会清掉待定前缀，
		// 用户还没按第二键就失效了。
	case screen.EventResize:
		a.syncSize(screen.Caps{Width: event.Width, Height: event.Height})
	case screen.EventFocusLost:
		a.debug("editor: focus lost")
	case screen.EventMouseScroll:
		a.scroll(event.DeltaY)
	}
}

// syncSize 记录终端尺寸并把视口夹回合法范围。
func (a *App) syncSize(caps screen.Caps) {
	if caps.Width > 0 {
		a.width = caps.Width
	}
	if caps.Height > 0 {
		a.height = caps.Height
	}
	a.clampTop()
}

// handleKey 处理一次按键按下。
func (a *App) handleKey(keystroke string) {
	key := screen.NormalizeKeystroke(keystroke)
	if key == "" {
		return
	}
	// 浮层打开时全部按键归浮层，编辑命令一律不生效。
	if a.overlay != nil {
		a.handleOverlayKey(key)
		return
	}

	// 记下按下这个键之前的待定前缀。Push 在不匹配时会清空前缀，
	// 所以名字必须提前取好，事后已经取不到了。
	pendingBefore := a.matcher.PendingSeq()

	result, binding := a.matcher.Push(key)
	switch result {
	case keymap.ResultIgnored:
		return

	case keymap.ResultPending:
		// 前缀已按下，正在等下一键。把待定序列显示出来，
		// 否则用户按了 C-a 却看不到任何反馈，会以为按键坏了。
		a.setStatus("按键前缀：" + a.matcher.PendingSeq())
		return

	case keymap.ResultUnknown:
		// 有前缀待定时，这一下键是「想完成组合键」而不是「想输入字符」。
		// 插进文档会造成很糟的后果：C-a 之后按 x 想撤销，
		// 屏幕上却多出一个 x 并让文档变脏；更麻烦的是误插改变了光标位置，
		// 后续操作继续错位，问题越滚越大。
		// 所以这里一律丢弃，并把前缀名一起报出来，让用户知道发生了什么。
		if pendingBefore != "" {
			a.setStatus("「" + pendingBefore + " " + key + "」不是有效组合，已忽略")
			return
		}
		if r, ok := screen.PrintableRune(key); ok {
			a.insertRune(r)
			return
		}
		a.setStatus("未绑定的按键：" + key)
		return

	case keymap.ResultMatch:
		a.runCommand(binding)
	}
}

// handleOverlayKey 把按键交给当前浮层。
func (a *App) handleOverlayKey(key string) {
	switch a.overlay.kind {
	case overlayPrompt:
		a.handlePromptKey(key)
	case overlayBrowser:
		a.handleBrowserKey(key)
	}
}

// closeOverlay 关闭当前浮层。
func (a *App) closeOverlay() {
	if a.overlay == nil {
		return
	}
	// 关浮层要清掉键位前缀：它在浮层打开期间可能已经记下半截，
	// 留着会让关闭后的第一键落到错误的上下文里。
	a.matcher.Reset()
	a.overlay = nil
}

// insertRune 插入一个字符。
func (a *App) insertRune(r rune) {
	if a.doc.Readonly() {
		a.setStatus("文件是只读的")
		return
	}
	a.doc.InsertRune(r)
}

// scroll 按行滚动视口。
func (a *App) scroll(deltaY int) {
	if deltaY == 0 {
		return
	}
	a.top -= deltaY
	a.clampTop()
}

// clampTop 把视口起点夹到文档范围内。
//
// 夹住上下界还不够：用户把光标移到屏幕外时必须滚动视口才能看到它，
// 所以还要保证光标落在可视范围内。
func (a *App) clampTop() {
	viewHeight := a.textHeight()
	last := maxInt(a.doc.LineCount()-viewHeight, 0)
	a.top = clampInt(a.top, 0, last)

	cursorLine := a.doc.Cursor().Line
	if cursorLine < a.top {
		a.top = cursorLine
	} else if cursorLine >= a.top+viewHeight {
		a.top = cursorLine - viewHeight + 1
	}
	a.top = maxInt(a.top, 0)
}

// textHeight 是文本区可用行数：总高减去标签栏与状态栏。
func (a *App) textHeight() int {
	h := a.height - statusBarHeight - a.chromeRows()
	if h < 1 {
		return 1
	}
	return h
}

// chromeRows 是标签栏与浮层占用的额外行数。
func (a *App) chromeRows() int {
	rows := 0
	if a.tabs.count() > 1 {
		rows++
	}
	if a.overlay != nil {
		switch a.overlay.kind {
		case overlayPrompt:
			// 提示浮层画在状态栏那一行上，不额外占行。
		case overlayBrowser:
			rows += a.browserHeight()
		}
	}
	return rows
}

// browserHeight 是目录浏览浮层占用的高度。
func (a *App) browserHeight() int {
	// 浏览浮层最多占一半屏幕：占满的话文本区就完全没了。
	h := a.height / 2
	if h < 3 {
		h = 3
	}
	if h > a.height-1 {
		h = maxInt(a.height-1, 1)
	}
	return h
}

// draw 渲染并送出一帧。
func (a *App) draw() {
	if a.height <= 0 || a.width <= 0 {
		return
	}
	a.clampTop()
	a.syncHighlighter()
	theme := a.theme()

	body := view.Render(view.Options{
		Doc:                 a.doc,
		Width:               a.width,
		Height:              a.textHeight(),
		Top:                 a.top,
		TabWidth:            a.cfg.Editor.TabWidth,
		ShowLineNumbers:     a.cfg.Editor.LineNumbers,
		LineNumbersAbsolute: !a.cfg.Editor.RelativeLineNumbers,
		Theme:               theme,
		CursorShape:         screen.CursorBlock,
		CursorBlink:         true,
		Tokens:              a.hl,
	})

	rows := make([]string, 0, a.height)
	var cursor *screen.CursorSpec

	// 目录浏览浮层画在上方，先画它并让它接管光标。
	if a.overlay != nil && a.overlay.kind == overlayBrowser {
		blockHeight := a.browserHeight()
		content, browserCursor := a.overlay.browser.Render(theme, a.width, blockHeight)
		rows = append(rows, splitLines(content, blockHeight, a.width)...)
		cursor = shiftCursor(browserCursor, 0)
		// 浏览浮层之下是文本区，但正文已经被浮层遮挡，
		// 这里照常渲染以保持帧高稳定。
		rows = append(rows, splitLines(body.Content, a.textHeight(), a.width)...)
	} else {
		rows = append(rows, splitLines(body.Content, a.textHeight(), a.width)...)
		cursor = body.Cursor
	}

	// 标签栏排在文本区之前。正文渲染时已经扣掉了它的高度，
	// 因此这里直接插在正文前面不会造成行数错位。
	if bar := a.tabs.renderTabs(theme, a.width); bar != "" {
		rows = append([]string{bar}, rows...)
	}

	// 最后一行是状态栏；提示浮层画在它的位置。
	switch {
	case a.overlay != nil && a.overlay.kind == overlayPrompt:
		line, promptCursor := a.overlay.prompt.render(theme, a.width)
		rows = append(rows, padToWidth(line, a.width))
		cursor = promptCursor
	case a.overlay != nil && a.overlay.kind == overlayBrowser:
		rows = append(rows, padToWidth(view.Fit(" "+a.overlay.browser.Hint(), a.width), a.width))
	default:
		rows = append(rows, padToWidth(a.statusLine(), a.width))
	}

	frame := joinLines(rows)
	if frame == a.lastFrame {
		return
	}
	a.lastFrame = frame
	a.backend.Render(frame, cursor)
}

// splitLines 把一段多行内容拆成恰好 height 行，并把每行补齐到 width 列。
//
// 帧高必须稳定：忽高忽低会让终端不停滚动。
// 每行也必须补齐到 width：少一列会露出上一帧的残影，多一列会让终端折行。
// 补齐按「可见宽度」算，因为内容里可能带 ANSI 转义序列，它们不占列。
func splitLines(content string, height, width int) []string {
	height = maxInt(height, 0)
	if height == 0 {
		return nil
	}
	parts := splitAll(content)
	if len(parts) > height {
		parts = parts[:height]
	}
	for i := range parts {
		parts[i] = padToWidth(parts[i], width)
	}
	for len(parts) < height {
		parts = append(parts, padToWidth("", width))
	}
	return parts
}

// splitAll 按换行拆分，末尾的换行不产生额外的空行。
func splitAll(content string) []string {
	if content == "" {
		return []string{""}
	}
	parts := []string{}
	start := 0
	for i := 0; i < len(content); i++ {
		if content[i] == '\n' {
			parts = append(parts, content[start:i])
			start = i + 1
		}
	}
	return append(parts, content[start:])
}

// joinLines 把各行拼成一帧。
func joinLines(rows []string) string {
	frame := ""
	for i, row := range rows {
		if i > 0 {
			frame += "\n"
		}
		frame += row
	}
	return frame
}

// shiftCursor 把光标整体下移若干行。
func shiftCursor(cursor *screen.CursorSpec, dy int) *screen.CursorSpec {
	if cursor == nil {
		return nil
	}
	moved := *cursor
	moved.Y += dy
	return &moved
}

// syncActive 在切换标签后刷新缓存的当前文档与视口。
func (a *App) syncActive() {
	a.doc = a.tabs.current()
	a.rebuildHighlighter()
	// 每个标签各自记一个视口起点最省事，但那样内存随标签数增长；
	// 这里统一回到文件开头：切换时看到开头比看到上次滚动位置更可预期。
	a.top = 0
	a.clampTop()
}

// debug 记录调试日志，logger 为 nil 时静默跳过。
func (a *App) debug(msg string, args ...any) {
	if a.logger != nil {
		a.logger.Debug(msg, args...)
	}
}

// info 记录关键日志，logger 为 nil 时静默跳过。
func (a *App) info(msg string, args ...any) {
	if a.logger != nil {
		a.logger.Info(msg, args...)
	}
}

// error 记录失败日志，logger 为 nil 时静默跳过。
func (a *App) error(msg string, args ...any) {
	if a.logger != nil {
		a.logger.Error(msg, args...)
	}
}

// maxInt 返回两者中较大的一个。
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// minInt 返回两者中较小的那个。
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// clampInt 把 v 夹到 [lo, hi]。
func clampInt(v, lo, hi int) int {
	return minInt(maxInt(v, lo), maxInt(lo, hi))
}

// ---- 语法高亮 ----

// rebuildHighlighter 按当前文档路径与配置重建高亮缓存。
//
// 触发时机：打开文件、切换标签、另存为（路径变了，语言可能跟着变）、
// 运行时改配置。重建而不是复用，是因为语言规则变了之后
// 之前算出的 token 全部作废——同一行文本在 Go 与 Python 下完全不同。
func (a *App) rebuildHighlighter() {
	a.hl = nil
	a.hlLang = ""
	a.hlRev = 0
	if a.doc == nil || !a.cfg.Editor.Syntax {
		return
	}
	lex := a.lexerFor(a.doc.Path())
	if lex == nil {
		// 没有匹配到语言就是纯文本，不报错也不提示：
		// 用户打开 .log、.txt、临时文件时不该被弹窗骚扰。
		return
	}
	a.hl = syntax.NewHighlighter(a.doc, lex)
	a.hlLang = lex.Name()
	a.hlRev = a.doc.Revision()
}

// lexerFor 按配置与路径挑出语言规则。
func (a *App) lexerFor(path string) syntax.Lexer {
	if name := a.cfg.Editor.SyntaxLang; name != "" {
		if lex := syntax.ForLanguage(name); lex != nil {
			return lex
		}
		a.setStatus("未知语言：" + name + "，已按扩展名判断")
	}
	return syntax.ForFilename(path)
}

// syncHighlighter 让缓存跟上文档的改动。
//
// 挂在绘制路径上而不是各个编辑命令里，是为了不漏：插入、删除、
// 撤销、重做、粘贴，将来新增的编辑入口，只要动了内容就会改版本号。
// 靠版本号判断比靠命令白名单可靠。
func (a *App) syncHighlighter() {
	if a.hl == nil {
		return
	}
	if rev := a.doc.Revision(); rev != a.hlRev {
		a.hlRev = rev
		a.hl.Invalidate(a.doc.DirtyFromLine())
	}
}
