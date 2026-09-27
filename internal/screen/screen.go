// screen.go — 终端抽象层：把「帧」与「事件」两个概念从具体 TUI 框架里剥离。
// SPDX-License-Identifier: MIT

// Package screen 定义 Go Studio 与终端之间的唯一边界。
//
// 上层负责把状态渲染成一帧 ANSI 字符串并交给 Screen 送出，同时从 Screen
// 订阅输入事件。渲染逻辑完全在 Screen 之外，因此可以脱离真实终端做单元测试。
//
// 这一层存在的另一个理由是隔离渲染器：Bubble Tea 的 Cursed Renderer 每帧都会
// 重新解析整帧字符串，200x50 终端约 300-800 微秒。当这个代价成为瓶颈时，
// 只需要替换本包，编辑器与界面代码不受影响。
package screen

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// CursorShape 是文本光标的形状。
type CursorShape int

const (
	// CursorBlock 是块状光标，Emacs 风格的默认值。
	CursorBlock CursorShape = iota
	// CursorUnderline 是下划线光标，Vim 风格的默认值。
	CursorUnderline
	// CursorBar 是竖线光标。
	CursorBar
)

// String 返回光标形状的名字。
func (s CursorShape) String() string {
	switch s {
	case CursorBlock:
		return "block"
	case CursorUnderline:
		return "underline"
	case CursorBar:
		return "bar"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// CursorSpec 描述文本光标的位置与外观。
// 光标的 X、Y 都是 0 基的帧内坐标，原点在左上角。
type CursorSpec struct {
	X, Y  int
	Shape CursorShape
	Blink bool
}

// Caps 是启动时探测到的终端能力。
// 这些值只读，用于决定配色深度、键位解码策略等，不用于判断能否运行。
type Caps struct {
	// Width、Height 是终端的字符尺寸。
	Width, Height int
	// TrueColor 为真时终端支持 24 位色。
	TrueColor bool
	// ColorDepth 是可用色数：256、16 或 8。
	ColorDepth int
	// SynchronizedUpdates 为真时终端支持 Mode 2026，可原子提交一帧。
	SynchronizedUpdates bool
	// UnicodeCore 为真时终端支持 Mode 2027，宽字符宽度由终端计算。
	UnicodeCore bool
	// KittyKeyboard 为真时终端支持 kitty 键盘协议，
	// 按键与转义序列不再歧义。
	KittyKeyboard bool
	// KeyDisambiguation 为真时终端能区分 Esc 与转义序列起始。
	KeyDisambiguation bool
	// BackgroundDark 是探测到的终端背景色明暗。
	BackgroundDark bool
	// Terminal 是终端自报的名字，可能为空。
	Terminal string
}

// LogKeyValue 把能力探测结果整理成适合写日志的有序键值对。
func (c Caps) LogKeyValue() []any {
	fields := map[string]any{
		"width":             c.Width,
		"height":            c.Height,
		"trueColor":         c.TrueColor,
		"colorDepth":        c.ColorDepth,
		"syncUpdates":       c.SynchronizedUpdates,
		"unicodeCore":       c.UnicodeCore,
		"kittyKeyboard":     c.KittyKeyboard,
		"keyDisambiguation": c.KeyDisambiguation,
		"backgroundDark":    c.BackgroundDark,
		"terminal":          c.Terminal,
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	pairs := make([]any, 0, len(keys))
	for _, key := range keys {
		pairs = append(pairs, key, fields[key])
	}
	return pairs
}

// Screen 是终端的读写接口。实现必须保证 Close 之后终端状态完全恢复。
type Screen interface {
	// Caps 返回启动时探测到的能力。调用方不应修改返回值。
	Caps() Caps
	// Render 送出一帧内容并定位光标。cursor 为 nil 表示本帧隐藏光标。
	// 实现允许丢弃与上一帧完全相同的内容。
	Render(frame string, cursor *CursorSpec)
	// Events 返回输入事件通道。Close 之后通道会被关闭。
	Events() <-chan Event
	// Close 释放终端并恢复进入前的状态。可重复调用。
	Close() error
}

// ErrClosed 表示对已关闭的 Screen 继续操作。
var ErrClosed = errors.New("screen: already closed")

// EventKind 区分输入事件的类别。
type EventKind int

const (
	// EventKeyPress 是按键按下。
	EventKeyPress EventKind = iota
	// EventKeyRelease 是按键松开。未启用按键释放上报时不会出现。
	EventKeyRelease
	// EventResize 是终端尺寸变化。
	EventResize
	// EventMouseScroll 是滚轮事件。
	EventMouseScroll
	// EventFocusLost 是终端失去焦点。
	EventFocusLost
)

// String 返回事件种类的名字。
func (k EventKind) String() string {
	switch k {
	case EventKeyPress:
		return "key-press"
	case EventKeyRelease:
		return "key-release"
	case EventResize:
		return "resize"
	case EventMouseScroll:
		return "mouse-scroll"
	case EventFocusLost:
		return "focus-lost"
	default:
		return fmt.Sprintf("unknown(%d)", int(k))
	}
}

// Event 是所有输入事件的共同表示。
//
// 按键用归一化的 Keystroke 字符串表示，例如 "a"、"ctrl+c"、"<esc>"、"<up>"。
// 键位引擎只面对 Keystroke，不需要理解终端如何编码按键。
type Event struct {
	// Kind 是事件类别。
	Kind EventKind
	// Keystroke 是归一化后的按键，仅对按键事件有意义。
	Keystroke string
	// Width、Height 是新尺寸，仅对 EventResize 有意义。
	Width, Height int
	// DeltaX、DeltaY 是滚动量，仅对 EventMouseScroll 有意义。
	// 向下滚动时 DeltaY 为正。
	DeltaX, DeltaY int
}

// NormalizeKeystroke 把终端上报的按键描述归一化成键位引擎使用的名字。
//
// 输入是 Bubble Tea 的 Key.Keystroke() 或 String() 风格的名字。归一化只做
// 两件事：把修饰键按 ctrl+alt+shift+meta 的固定顺序排列，把可打印字符原样
// 保留。没有可打印字符时返回空串，表示「不参与键位匹配」。
func NormalizeKeystroke(keystroke string) string {
	if keystroke == "" {
		return ""
	}
	if isNamedKey(keystroke) {
		return keystroke
	}
	if !strings.Contains(keystroke, "+") {
		return keystroke
	}

	// 修饰键固定排序，避免 "alt+ctrl+a" 与 "ctrl+alt+a" 被当成两个键。
	parts := strings.Split(keystroke, "+")
	mods := make([]string, 0, len(parts))
	var base string
	for _, part := range parts {
		switch part {
		case "ctrl", "alt", "shift", "meta", "super", "hyper":
			mods = append(mods, part)
		case "":
			// 忽略 "a++b" 这类畸形输入里的空段。
		default:
			base = part
		}
	}
	if base == "" {
		return ""
	}

	order := map[string]int{"ctrl": 0, "alt": 1, "shift": 2, "meta": 3, "hyper": 4, "super": 5}
	sort.Slice(mods, func(i, j int) bool { return order[mods[i]] < order[mods[j]] })
	return strings.Join(append(mods, base), "+")
}

// namedKeys 是无法用可打印字符表示、需要写成 <name> 的按键。
var namedKeys = map[string]bool{
	"<esc>": true, "<enter>": true, "<tab>": true, "<space>": true, "<backspace>": true,
	"<up>": true, "<down>": true, "<left>": true, "<right>": true,
	"<home>": true, "<end>": true, "<pgup>": true, "<pgdown>": true,
	"<delete>": true, "<insert>": true,
	"<f1>": true, "<f2>": true, "<f3>": true, "<f4>": true, "<f5>": true,
	"<f6>": true, "<f7>": true, "<f8>": true, "<f9>": true, "<f10>": true,
	"<f11>": true, "<f12>": true,
	"<left-shift>": true, "<right-shift>": true, "<left-ctrl>": true, "<right-ctrl>": true,
	"<left-alt>": true, "<right-alt>": true, "<left-super>": true, "<right-super>": true,
}

// isNamedKey 判断一个按键名是否已经是 <name> 形式。
func isNamedKey(keystroke string) bool {
	return strings.HasPrefix(keystroke, "<") && strings.HasSuffix(keystroke, ">")
}

// IsNamedKey 导出版本，供键位配置校验使用。
func IsNamedKey(keystroke string) bool {
	if !isNamedKey(keystroke) {
		return false
	}
	return namedKeys[strings.ToLower(keystroke)]
}
