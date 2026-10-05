// bubble.go — Screen 的 Bubble Tea v2 实现。
// SPDX-License-Identifier: MIT

package screen

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
)

// 默认帧率。渲染器用变更检测跳过相同帧，因此这个值只是写入频率的上限。
const defaultFPS = 60

// gracefulShutdownTimeout 是等待底层事件循环收尾的上限。
// 超过它就放弃等待并让调用方继续退出：卡住的 TUI 比未完全恢复的终端更糟。
const gracefulShutdownTimeout = 2 * time.Second

// BubbleOptions 是 Bubble Tea 后端的启动选项。
type BubbleOptions struct {
	// Context 取消后程序退出并恢复终端。
	Context context.Context
	// FPS 限制每秒最多写入终端的帧数。
	FPS int
	// Env 覆盖环境变量来源，测试时用于注入。
	Env []string
	// Output 是帧的输出目标，默认为 os.Stdout。
	Output *os.File
}

// redrawMsg 用于在 Render 之后请求一次重绘。
// Bubble Tea 只在处理完一条消息后才渲染，因此在两次消息之间调用 Render
// 不会自动生效，必须再注入一条消息驱动一次事件循环。
type redrawMsg struct{}

// bubbleScreen 是 Screen 的 Bubble Tea 实现，同时充当 Bubble Tea 的 Model。
type bubbleScreen struct {
	program *tea.Program
	events  chan Event

	// tty 是启动前的终端设置快照。恢复终端不能只依赖 Bubble Tea 的收尾：
	// 那个收尾在超时后会被放弃，而终端残留脏状态是最糟的失败模式。
	tty *ttyState

	mu           sync.Mutex
	caps         Caps
	frame        string
	cursor       *CursorSpec
	redrawQueued bool
	closed       bool
	ttyErr       error
}

// ttyState 保存终端的 termios 设置。
// termios 与读写方式由 build-tagged 的 tty_unix.go / tty_windows.go 提供。
type ttyState struct {
	fd  int
	sys termios
}

// captureTTYState 读取 fd 所指终端的当前设置。fd 不是终端时返回 nil。
func captureTTYState(f *os.File) *ttyState {
	if f == nil {
		return nil
	}
	state, err := getTermios(int(f.Fd()))
	if err != nil {
		return nil
	}
	return &ttyState{fd: int(f.Fd()), sys: state}
}

// restore 把设置写回终端。
//
// 用 TCSETS 而非 TCSANOW：后者会等输出排空，一旦输出被阻塞就会挂住，
// 而卡住的退出比少写几个转义序列更糟。重复调用是安全的。
func (t *ttyState) restore() error {
	if t == nil {
		return nil
	}
	return setTermios(t.fd, t.sys)
}

// restoreTTY 幂等地恢复终端设置。
func (s *bubbleScreen) restoreTTY() {
	if err := s.tty.restore(); err != nil {
		s.mu.Lock()
		s.ttyErr = err
		s.mu.Unlock()
	}
}

// NewBubble 启动 Bubble Tea 后端并返回 Screen。
// 程序在后台 goroutine 中运行，Close 会等待它退出并恢复终端状态。
func NewBubble(opts BubbleOptions) (Screen, error) {
	output := opts.Output
	if output == nil {
		output = os.Stdout
	}
	env := opts.Env
	if env == nil {
		env = os.Environ()
	}

	profile := colorprofile.Detect(output, env)
	s := &bubbleScreen{
		events: make(chan Event, 64),
		caps: Caps{
			TrueColor:  profile == colorprofile.TrueColor,
			ColorDepth: colorDepthOf(profile),
		},
	}

	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}
	fps := opts.FPS
	if fps <= 0 {
		fps = defaultFPS
	}

	// 在 Bubble Tea 动终端之前先留一份快照。
	tty := captureTTYState(output)
	if tty == nil && isTerminal(output) {
		return nil, fmt.Errorf("screen: 无法读取终端设置，拒绝在无法恢复的前提下启动")
	}
	s.tty = tty

	s.program = tea.NewProgram(s,
		tea.WithContext(ctx),
		tea.WithOutput(output),
		tea.WithFPS(fps),
		tea.WithColorProfile(profile),
	)

	go func() {
		// Run 返回后终端已被 Bubble Tea 恢复，此时关闭事件通道让调用方收尾。
		// 这里再恢复一次是兜底：万一事件循环 panic，Close 未必会被执行。
		defer s.restoreTTY()
		_, _ = s.program.Run()
		close(s.events)
	}()
	return s, nil
}

// isTerminal 报告 f 是否指向终端。
func isTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	_, err := getTermios(int(f.Fd()))
	return err == nil
}

// colorDepthOf 把色彩档位映射成可用色数。
func colorDepthOf(profile colorprofile.Profile) int {
	switch profile {
	case colorprofile.TrueColor:
		return 256
	case colorprofile.ANSI256:
		return 256
	case colorprofile.ANSI:
		return 16
	default:
		return 8
	}
}

// Init 发起能力探测。
// 尺寸由 Bubble Tea 主动推送一个 WindowSizeMsg，其余能力靠终端回包。
func (s *bubbleScreen) Init() tea.Cmd {
	// 这些请求函数返回的是 Msg 而非 Cmd，需要包一层交给事件循环投递。
	return tea.Batch(
		func() tea.Msg { return tea.RequestBackgroundColor() },
		func() tea.Msg { return tea.RequestTerminalVersion() },
		func() tea.Msg { return tea.RequestCursorColor() },
	)
}

// Update 把 Bubble Tea 消息翻译成 Screen 事件。
// 本层不做任何业务判断，只做格式转换。
func (s *bubbleScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case redrawMsg:
		s.mu.Lock()
		s.redrawQueued = false
		s.mu.Unlock()

	case tea.WindowSizeMsg:
		s.mu.Lock()
		s.caps.Width, s.caps.Height = msg.Width, msg.Height
		s.mu.Unlock()
		s.emit(Event{Kind: EventResize, Width: msg.Width, Height: msg.Height})

	case tea.KeyPressMsg:
		if key := toKeystroke(msg.Key()); key != "" {
			s.emit(Event{Kind: EventKeyPress, Keystroke: key})
		}

	case tea.KeyReleaseMsg:
		if key := toKeystroke(msg.Key()); key != "" {
			s.emit(Event{Kind: EventKeyRelease, Keystroke: key})
		}

	case tea.MouseWheelMsg:
		mouse := msg.Mouse()
		deltaY := 0
		switch mouse.Button {
		case tea.MouseWheelUp:
			deltaY = -1
		case tea.MouseWheelDown:
			deltaY = 1
		}
		deltaX := 0
		switch mouse.Button {
		case tea.MouseWheelLeft:
			deltaX = -1
		case tea.MouseWheelRight:
			deltaX = 1
		}
		if deltaX != 0 || deltaY != 0 {
			s.emit(Event{Kind: EventMouseScroll, DeltaX: deltaX, DeltaY: deltaY})
		}

	case tea.BlurMsg:
		s.emit(Event{Kind: EventFocusLost})

	case tea.BackgroundColorMsg:
		s.mu.Lock()
		s.caps.BackgroundDark = msg.IsDark()
		s.mu.Unlock()

	case tea.TerminalVersionMsg:
		s.mu.Lock()
		s.caps.Terminal = msg.Name
		s.mu.Unlock()

	case tea.KeyboardEnhancementsMsg:
		s.mu.Lock()
		s.caps.KittyKeyboard = msg.SupportsKeyDisambiguation()
		s.caps.KeyDisambiguation = msg.SupportsKeyDisambiguation()
		s.mu.Unlock()

	case tea.ModeReportMsg:
		s.applyModeReport(msg)
	}
	return s, nil
}

// applyModeReport 记录终端对私有模式的应答。
// ModeReset 表示终端不认识该模式，因此不置位对应能力。
func (s *bubbleScreen) applyModeReport(msg tea.ModeReportMsg) {
	if msg.Value == ansi.ModeReset {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	switch msg.Mode {
	case ansi.ModeSynchronizedOutput:
		s.caps.SynchronizedUpdates = true
	case ansi.ModeUnicodeCore:
		s.caps.UnicodeCore = true
	}
}

// View 把当前帧交给 Bubble Tea 渲染。
//
// 每帧都必须设置 Cursor：在不支持同步更新的终端上，渲染器靠「写入期间隐藏
// 光标」来抑制闪烁，光标为 nil 时这条降级路径会失效。
func (s *bubbleScreen) View() tea.View {
	s.mu.Lock()
	frame, cursor := s.frame, s.cursor
	s.mu.Unlock()

	view := tea.NewView(frame)
	view.AltScreen = true
	view.Cursor = toTeaCursor(cursor)
	// 模态编辑需要区分按下与松开，M3 键位引擎依赖这个事件。
	view.KeyboardEnhancements.ReportEventTypes = true
	return view
}

// toTeaCursor 把本包的 CursorSpec 转成 Bubble Tea 的光标描述。
func toTeaCursor(spec *CursorSpec) *tea.Cursor {
	if spec == nil {
		return nil
	}
	cursor := &tea.Cursor{
		Position: tea.Position{X: spec.X, Y: spec.Y},
		Blink:    spec.Blink,
	}
	switch spec.Shape {
	case CursorUnderline:
		cursor.Shape = tea.CursorUnderline
	case CursorBar:
		cursor.Shape = tea.CursorBar
	default:
		cursor.Shape = tea.CursorBlock
	}
	return cursor
}

// Caps 返回当前探测到的终端能力。
// 能力是随消息陆续补齐的：尺寸最先到达，模式回包可能晚若干毫秒。
func (s *bubbleScreen) Caps() Caps {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.caps
}

// Render 记录一帧并请求重绘。多次连续调用会被合并成一次重绘。
func (s *bubbleScreen) Render(frame string, cursor *CursorSpec) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.frame = frame
	s.cursor = cursor

	needRedraw := !s.redrawQueued
	if needRedraw {
		s.redrawQueued = true
	}
	s.mu.Unlock()

	if needRedraw {
		s.program.Send(redrawMsg{})
	}
}

// Events 返回输入事件通道。
func (s *bubbleScreen) Events() <-chan Event {
	return s.events
}

// Close 停止程序并恢复终端。
//
// 等待事件循环收尾是有界的：Bubble Tea 在极端情况下（例如渲染器或输入读循环
// 没能正常收尾）可能不返回，此时程序绝不能跟着卡死，否则用户的 shell 提示符
// 永远回不来。
//
// 但终端设置由我们自己无条件恢复，不挂在超时分支上——超时只影响"等多久"，
// 绝不能影响"是否恢复"。Bubble Tea 正常收尾时它也会恢复，这里再恢复一次，
// 因为写回同一份快照是幂等的。
func (s *bubbleScreen) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()

	s.program.Quit()

	stopped := make(chan struct{})
	go func() {
		// events 由 Run 所在 goroutine 在返回后关闭，这里等它即等 Run 结束。
		for range s.events {
		}
		close(stopped)
	}()

	var waitErr error
	select {
	case <-stopped:
	case <-time.After(gracefulShutdownTimeout):
		waitErr = fmt.Errorf("screen: 终端未在 %v 内恢复，已强制返回", gracefulShutdownTimeout)
	}

	s.restoreTTY()

	s.mu.Lock()
	restoreErr := s.ttyErr
	s.mu.Unlock()
	if restoreErr != nil {
		return fmt.Errorf("screen: 恢复终端设置失败: %w", restoreErr)
	}
	return waitErr
}

// emit 把事件送入通道。通道满时丢弃该事件而不是阻塞：
// 事件循环不能因为下游处理慢而被卡住，丢失的按键由下一次按键纠正。
func (s *bubbleScreen) emit(event Event) {
	select {
	case s.events <- event:
	default:
	}
}

// specialKeyNames 把 Bubble Tea 的按键名映射到本包的 <name> 形式。
// 可打印字符本身就能作为键名，只有控制键需要显式命名。
var specialKeyNames = map[string]string{
	"enter":      "<enter>",
	"return":     "<enter>",
	"tab":        "<tab>",
	"backtab":    "<s-tab>",
	"backspace":  "<backspace>",
	"esc":        "<esc>",
	"escape":     "<esc>",
	"up":         "<up>",
	"down":       "<down>",
	"left":       "<left>",
	"right":      "<right>",
	"home":       "<home>",
	"end":        "<end>",
	"pgup":       "<pgup>",
	"pgdown":     "<pgdown>",
	"pageup":     "<pgup>",
	"pagedown":   "<pgdown>",
	"delete":     "<delete>",
	"del":        "<delete>",
	"insert":     "<insert>",
	"leftshift":  "<left-shift>",
	"rightshift": "<right-shift>",
	"leftctrl":   "<left-ctrl>",
	"rightctrl":  "<right-ctrl>",
	"leftalt":    "<left-alt>",
	"rightalt":   "<right-alt>",
	"leftsuper":  "<left-super>",
	"rightsuper": "<right-super>",
	"leftmeta":   "<left-meta>",
	"rightmeta":  "<right-meta>",
}

// functionKeyNames 处理 F1 到 F63，Bubble Tea 返回的是 "f12" 这类名字。
var functionKeyNames = func() map[string]string {
	names := make(map[string]string, 63)
	for i := 1; i <= 63; i++ {
		names["f"+strconv.Itoa(i)] = "<f" + strconv.Itoa(i) + ">"
	}
	return names
}()

// toKeystroke 把 Bubble Tea 的按键翻译成键位引擎使用的名字。
// 修饰键保持 Bubble Tea 的固定顺序，可打印字符原样保留。
func toKeystroke(key tea.Key) string {
	// 可打印字符直接用字符本身当键名。编辑器靠这一点区分「输入字母 a」
	// 与「按下 ctrl+a」——两者在 Bubble Tea 里只差一个 Mod 位。
	//
	// shift 必须在这一步消化掉：终端不会把 A 当成一个独立的上档字母上报，
	// 大写字母是以 Code='a' + Mod=shift + Text="A" 的形式到达的，
	// 真正的字符只存在于 Text 里。不取 Text 的话，每次按 Shift 都会丢字符。
	if text, ok := printableText(key); ok {
		return text
	}

	keystroke := key.Keystroke()
	if keystroke == "" {
		return ""
	}

	base, mods := splitKeystroke(keystroke)
	if name, ok := specialKeyNames[base]; ok {
		return mods + name
	}
	if name, ok := functionKeyNames[base]; ok {
		return mods + name
	}
	if base == "" {
		return ""
	}
	return mods + base
}

// modMask 是「会让按键变成快捷键」的修饰键集合。
// shift 不在其中：它不改变字符含义，只影响字符本身的大小写或符号。
const modMask = tea.ModCtrl | tea.ModAlt | tea.ModMeta | tea.ModSuper | tea.ModHyper

// printableText 判断按键是否代表单个可打印字符，是则返回该字符。
//
// 带 ctrl/alt/meta 的按键是快捷键而非输入，一律不算。
// 空格也不算：它有专门的名字 space，见 specialKeyNames。
func printableText(key tea.Key) (string, bool) {
	if key.Text == "" || key.Mod&modMask != 0 {
		return "", false
	}
	// 多字符的文本是输入法组合出的字素簇，交给上层按文本处理。
	if utf8.RuneCountInString(key.Text) != 1 {
		return "", false
	}
	r, _ := utf8.DecodeRuneInString(key.Text)
	if r == ' ' || !unicode.IsPrint(r) {
		return "", false
	}
	return key.Text, true
}

// splitKeystroke 把 "ctrl+alt+a" 拆成修饰键前缀 "ctrl+alt+" 与基键 "a"。
func splitKeystroke(keystroke string) (base, mods string) {
	parts := strings.Split(keystroke, "+")
	switch len(parts) {
	case 0:
		return "", ""
	case 1:
		return parts[0], ""
	}

	modParts := make([]string, 0, len(parts)-1)
	for _, part := range parts[:len(parts)-1] {
		modParts = append(modParts, part)
	}
	return parts[len(parts)-1], strings.Join(modParts, "+") + "+"
}

// describeCaps 便于调试时打印能力。
func describeCaps(c Caps) string {
	pairs := c.LogKeyValue()
	parts := make([]string, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		parts = append(parts, fmt.Sprintf("%v=%v", pairs[i], pairs[i+1]))
	}
	return strings.Join(parts, " ")
}
