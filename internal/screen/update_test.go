// update_test.go — Bubble Tea 消息翻译层的单元测试。
// SPDX-License-Identifier: MIT

// 本文件只依赖 Bubble Tea 与 x/ansi，不涉及 termios，
// 因此不需要平台约束，Windows 上也能编译运行。

package screen

import (
	"image/color"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
)

// newTestBubble 造一个脱离真实终端的 bubbleScreen。
//
// 直接构造而不是走 NewBubble：本层只做「消息 → 事件」的形式转换，
// 不需要真的启动 Bubble Tea 事件循环，也就不该在测试里等一个真终端。
func newTestBubble() *bubbleScreen {
	return &bubbleScreen{
		events: make(chan Event, 64),
		caps:   Caps{Width: 80, Height: 24},
	}
}

// nextEvent 取出一条事件，没有则返回 false。
func nextEvent(s *bubbleScreen) (Event, bool) {
	select {
	case event := <-s.events:
		return event, true
	default:
		return Event{}, false
	}
}

// assertNoEvent 确认没有多余事件。
func assertNoEvent(t *testing.T, s *bubbleScreen, what string) {
	t.Helper()
	if event, ok := nextEvent(s); ok {
		t.Errorf("%s 之后仍有未消费的事件: %+v", what, event)
	}
}

// update 送一条消息进 Update。
func update(t *testing.T, s *bubbleScreen, msg tea.Msg) {
	t.Helper()
	if _, cmd := s.Update(msg); cmd != nil {
		t.Errorf("Update 不应返回命令，实际返回了 %v", cmd)
	}
}

// ---- WindowSizeMsg ----

// TestUpdateWindowSizeReportsSize 尺寸消息既要记进能力，也要作为事件上报。
func TestUpdateWindowSizeReportsSize(t *testing.T) {
	s := newTestBubble()
	update(t, s, tea.WindowSizeMsg{Width: 120, Height: 40})

	if got := s.Caps(); got.Width != 120 || got.Height != 40 {
		t.Errorf("Caps = %dx%d, want 120x40", got.Width, got.Height)
	}
	event, ok := nextEvent(s)
	if !ok {
		t.Fatal("没有收到 resize 事件")
	}
	if event.Kind != EventResize {
		t.Errorf("事件类型 = %v, want %v", event.Kind, EventResize)
	}
	if event.Width != 120 || event.Height != 40 {
		t.Errorf("事件尺寸 = %dx%d, want 120x40", event.Width, event.Height)
	}
}

// ---- 按键 ----

func TestUpdateKeyPress(t *testing.T) {
	cases := []struct {
		name string
		key  tea.Key
		want string
	}{
		{"字母", tea.Key{Code: 'a', Text: "a"}, "a"},
		{"控制键", tea.Key{Code: 'a', Text: "a", Mod: tea.ModCtrl}, "ctrl+a"},
		{"退出键", tea.Key{Code: tea.KeyEscape}, "<esc>"},
		{"方向键", tea.Key{Code: tea.KeyUp}, "<up>"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestBubble()
			update(t, s, tea.KeyPressMsg(tc.key))

			event, ok := nextEvent(s)
			if !ok {
				t.Fatal("没有收到按键事件")
			}
			if event.Kind != EventKeyPress {
				t.Errorf("事件类型 = %v, want %v", event.Kind, EventKeyPress)
			}
			if event.Keystroke != tc.want {
				t.Errorf("按键 = %q, want %q", event.Keystroke, tc.want)
			}
		})
	}
}

// TestUpdateKeyRelease 键位引擎要区分按下与松开，因此两种事件都必须上报。
func TestUpdateKeyRelease(t *testing.T) {
	s := newTestBubble()
	update(t, s, tea.KeyReleaseMsg{Code: 'a', Text: "a"})

	event, ok := nextEvent(s)
	if !ok {
		t.Fatal("没有收到松开事件")
	}
	if event.Kind != EventKeyRelease {
		t.Errorf("事件类型 = %v, want %v", event.Kind, EventKeyRelease)
	}
	if event.Keystroke != "a" {
		t.Errorf("按键 = %q, want %q", event.Keystroke, "a")
	}
}

// TestUpdateReportsModifierKeysAsNamedKeys 裸的修饰键要报成命名键而不是空串。
// 空串会让键位引擎无从判断，而 `<left-ctrl>` 会经 PrintableRune 判为非字符输入，
// 于是「只按到 Ctrl 就松手」这种情况不会往文档里插入任何东西。
func TestUpdateReportsModifierKeysAsNamedKeys(t *testing.T) {
	cases := []struct {
		name string
		key  tea.Key
		want string
	}{
		{"左 Ctrl", tea.Key{Code: tea.KeyLeftCtrl, Mod: tea.ModCtrl}, "<left-ctrl>"},
		{"左 Alt", tea.Key{Code: tea.KeyLeftAlt, Mod: tea.ModAlt}, "<left-alt>"},
		{"左 Shift", tea.Key{Code: tea.KeyLeftShift, Mod: tea.ModShift}, "<left-shift>"},
		{"左 Super", tea.Key{Code: tea.KeyLeftSuper, Mod: tea.ModSuper}, "<left-super>"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestBubble()
			update(t, s, tea.KeyPressMsg(tc.key))

			event, ok := nextEvent(s)
			if !ok {
				t.Fatal("没有收到按键事件")
			}
			if event.Keystroke != tc.want {
				t.Errorf("按键 = %q, want %q", event.Keystroke, tc.want)
			}
			if _, printable := PrintableRune(event.Keystroke); printable {
				t.Errorf("修饰键 %q 被当成了可输入字符", event.Keystroke)
			}
		})
	}
}

// ---- 滚轮 ----

// TestUpdateMouseWheelDirections 四个方向的滚动量符号必须正确。
func TestUpdateMouseWheelDirections(t *testing.T) {
	cases := []struct {
		name   string
		button tea.MouseButton
		dx, dy int
	}{
		{"上", tea.MouseWheelUp, 0, -1},
		{"下", tea.MouseWheelDown, 0, 1},
		{"左", tea.MouseWheelLeft, -1, 0},
		{"右", tea.MouseWheelRight, 1, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestBubble()
			update(t, s, tea.MouseWheelMsg{Button: tc.button})

			event, ok := nextEvent(s)
			if !ok {
				t.Fatal("没有收到滚动事件")
			}
			if event.Kind != EventMouseScroll {
				t.Errorf("事件类型 = %v, want %v", event.Kind, EventMouseScroll)
			}
			if event.DeltaX != tc.dx || event.DeltaY != tc.dy {
				t.Errorf("滚动量 = (%d,%d), want (%d,%d)", event.DeltaX, event.DeltaY, tc.dx, tc.dy)
			}
		})
	}
}

// TestUpdateMouseWheelIgnoresNoMotion 没有位移就不该发事件，否则会空转一帧。
// 四个滚轮按钮之外的值（例如普通按键）都不产生位移。
func TestUpdateMouseWheelIgnoresNoMotion(t *testing.T) {
	s := newTestBubble()
	for _, button := range []tea.MouseButton{tea.MouseLeft, tea.MouseMiddle, tea.MouseRight, tea.MouseNone} {
		update(t, s, tea.MouseWheelMsg{Button: button})
	}
	assertNoEvent(t, s, "无位移滚动")
}

// ---- 焦点 ----

func TestUpdateBlurReportsFocusLost(t *testing.T) {
	s := newTestBubble()
	update(t, s, tea.BlurMsg{})

	event, ok := nextEvent(s)
	if !ok {
		t.Fatal("没有收到失焦事件")
	}
	if event.Kind != EventFocusLost {
		t.Errorf("事件类型 = %v, want %v", event.Kind, EventFocusLost)
	}
}

// ---- 能力探测 ----

// TestUpdateBackgroundColor 背景色明暗要记进能力，且两种明暗都要能区分。
func TestUpdateBackgroundColor(t *testing.T) {
	dark := color.RGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xff}
	light := color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}

	cases := []struct {
		name  string
		color color.Color
		want  bool
	}{
		{"深色", dark, true},
		{"浅色", light, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestBubble()
			update(t, s, tea.BackgroundColorMsg{Color: tc.color})

			if got := s.Caps().BackgroundDark; got != tc.want {
				t.Errorf("BackgroundDark = %t, want %t", got, tc.want)
			}
			// 能力探测不产生输入事件。
			assertNoEvent(t, s, "背景色探测")
		})
	}
}

func TestUpdateTerminalVersion(t *testing.T) {
	s := newTestBubble()
	update(t, s, tea.TerminalVersionMsg{Name: "ghostty"})

	if got := s.Caps().Terminal; got != "ghostty" {
		t.Errorf("Terminal = %q, want %q", got, "ghostty")
	}
	assertNoEvent(t, s, "终端版本探测")
}

// TestUpdateKeyboardEnhancements 键盘协议支持情况要记进能力。
func TestUpdateKeyboardEnhancements(t *testing.T) {
	cases := []struct {
		name  string
		flags int
		want  bool
	}{
		{"不支持", 0, false},
		{"支持", ansi.KittyReportEventTypes, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestBubble()
			update(t, s, tea.KeyboardEnhancementsMsg{Flags: tc.flags})

			caps := s.Caps()
			if caps.KittyKeyboard != tc.want || caps.KeyDisambiguation != tc.want {
				t.Errorf("KittyKeyboard=%t KeyDisambiguation=%t, want 都是 %t",
					caps.KittyKeyboard, caps.KeyDisambiguation, tc.want)
			}
		})
	}
}

// ---- 私有模式回包 ----

// TestApplyModeReportRecordsSupportedModes 终端认得的模式才置位对应能力。
func TestApplyModeReportRecordsSupportedModes(t *testing.T) {
	cases := []struct {
		name string
		mode ansi.Mode
		want func(Caps) bool
	}{
		{"同步更新", ansi.ModeSynchronizedOutput, func(c Caps) bool { return c.SynchronizedUpdates }},
		{"UnicodeCore", ansi.ModeUnicodeCore, func(c Caps) bool { return c.UnicodeCore }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestBubble()
			update(t, s, tea.ModeReportMsg{Mode: tc.mode, Value: ansi.ModeSet})

			if !tc.want(s.Caps()) {
				t.Errorf("模式 %v 被确认支持后对应能力仍未置位", tc.mode)
			}
		})
	}
}

// TestApplyModeReportIgnoresReset 是这条分支存在的全部理由：
// 终端回 ModeReset 表示它不认识这个模式，此时绝不能置位对应能力，
// 否则会以为终端支持同步更新，从而发出它根本不认识的转义序列。
func TestApplyModeReportIgnoresReset(t *testing.T) {
	cases := []ansi.Mode{ansi.ModeSynchronizedOutput, ansi.ModeUnicodeCore}
	for _, mode := range cases {
		s := newTestBubble()
		update(t, s, tea.ModeReportMsg{Mode: mode, Value: ansi.ModeReset})

		caps := s.Caps()
		if caps.SynchronizedUpdates || caps.UnicodeCore {
			t.Errorf("模式 %v 被拒绝后仍置位了能力: %+v", mode, caps)
		}
	}
}

// ---- redrawMsg ----

// TestUpdateRedrawClearsQueue redrawMsg 的作用是把重绘标记清掉，
// 之后下一次 Render 才允许再次请求重绘。
func TestUpdateRedrawClearsQueue(t *testing.T) {
	s := newTestBubble()
	s.redrawQueued = true

	update(t, s, redrawMsg{})

	s.mu.Lock()
	queued := s.redrawQueued
	s.mu.Unlock()
	if queued {
		t.Error("收到 redrawMsg 后重绘标记仍未清掉")
	}
}

// ---- View ----

// TestViewReportsFrameAndCursor View 必须把当前帧与光标交给渲染器。
func TestViewReportsFrameAndCursor(t *testing.T) {
	s := newTestBubble()
	s.frame = "hello"
	s.cursor = &CursorSpec{X: 3, Y: 1, Shape: CursorUnderline, Blink: true}

	view := s.View()
	if view.Content != "hello" {
		t.Errorf("帧内容 = %q, want %q", view.Content, "hello")
	}
	if view.Cursor == nil {
		t.Fatal("光标为 nil")
	}
	if view.Cursor.Position.X != 3 || view.Cursor.Position.Y != 1 {
		t.Errorf("光标位置 = (%d,%d), want (3,1)", view.Cursor.Position.X, view.Cursor.Position.Y)
	}
	if !view.Cursor.Blink {
		t.Error("光标闪烁未透传")
	}
}

// TestViewUsesAltScreen 编辑器必须占用备用屏，否则退出后用户的滚动内容会丢。
func TestViewUsesAltScreen(t *testing.T) {
	s := newTestBubble()
	if !s.View().AltScreen {
		t.Error("View 未启用备用屏")
	}
}

// TestViewReportsKeyEventTypes 模态编辑要区分按下与松开，
// M3 键位引擎依赖这个开关，必须一直开着。
func TestViewReportsKeyEventTypes(t *testing.T) {
	s := newTestBubble()
	if !s.View().KeyboardEnhancements.ReportEventTypes {
		t.Error("View 未上报按键事件类型，松开事件将无法送达键位引擎")
	}
}

func TestViewWithNilCursor(t *testing.T) {
	s := newTestBubble()
	s.frame = "x"
	s.cursor = nil
	if got := s.View().Cursor; got != nil {
		t.Errorf("光标 = %v, want nil（本帧隐藏光标）", got)
	}
}

// ---- emit 溢出 ----

// TestEmitDropsWhenChannelFull 事件通道满时必须丢弃而不是阻塞：
// 下游处理慢不该把整个输入循环卡住。
func TestEmitDropsWhenChannelFull(t *testing.T) {
	s := &bubbleScreen{events: make(chan Event)} // 无缓冲，容量为 0

	done := make(chan struct{})
	go func() {
		defer close(done)
		s.emit(Event{Kind: EventKeyPress, Keystroke: "a"})
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("emit 在通道满时阻塞了")
	}
}

// ---- 纯函数辅助 ----

func TestColorDepthOf(t *testing.T) {
	cases := []struct {
		name    string
		profile colorprofile.Profile
		want    int
	}{
		{"真彩", colorprofile.TrueColor, 256},
		{"256 色", colorprofile.ANSI256, 256},
		{"16 色", colorprofile.ANSI, 16},
		{"8 色", colorprofile.Ascii, 8},
		{"无色", colorprofile.NoTTY, 8},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := colorDepthOf(tc.profile); got != tc.want {
				t.Errorf("colorDepthOf(%v) = %d, want %d", tc.name, got, tc.want)
			}
		})
	}
}

func TestSplitKeystroke(t *testing.T) {
	cases := []struct {
		in       string
		wantBase string
		wantMods string
	}{
		{"a", "a", ""},
		{"ctrl+a", "a", "ctrl+"},
		{"ctrl+alt+a", "a", "ctrl+alt+"},
		{"alt+shift+<up>", "<up>", "alt+shift+"},
	}
	for _, tc := range cases {
		base, mods := splitKeystroke(tc.in)
		if base != tc.wantBase || mods != tc.wantMods {
			t.Errorf("splitKeystroke(%q) = (%q,%q), want (%q,%q)",
				tc.in, base, mods, tc.wantBase, tc.wantMods)
		}
	}
}

func TestDescribeCapsIsReadable(t *testing.T) {
	got := describeCaps(Caps{Width: 80, Height: 24, Terminal: "xterm"})
	if !strings.Contains(got, "width=80") {
		t.Errorf("describeCaps = %q, want 含 width=80", got)
	}
	if !strings.Contains(got, "height=24") {
		t.Errorf("describeCaps = %q, want 含 height=24", got)
	}
}

// TestScreenImplementations 接口一致性：两个实现都必须能被当 Screen 用。
func TestScreenImplementations(t *testing.T) {
	var _ Screen = (*bubbleScreen)(nil)
	var _ Screen = (*Fake)(nil)
}
