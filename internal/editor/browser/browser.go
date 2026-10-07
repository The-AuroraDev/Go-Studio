// browser.go — 目录浏览浮层：spec 一.2 要求「选中文件/文件夹 右箭头键打开/进入」。
// SPDX-License-Identifier: MIT

package browser

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/29anan29/Go-Studio/internal/screen"
	"github.com/29anan29/Go-Studio/internal/view"
)

// Entry 是列表里的一项。
type Entry struct {
	Name string
	Dir  bool
}

// Browser 是目录浏览浮层。
//
// 它只做「列目录 + 移动选择 + 进入/返回」三件事，不负责决定打开之后做什么，
// 因此打开文件与打开目录都能复用它。
type Browser struct {
	// Dir 是当前所在目录。
	Dir string
	// Entries 是列表内容，第一项固定是回到上级目录的 ".."。
	Entries []Entry
	// Cursor 是选中项下标。
	Cursor int
	// Offset 是列表首行下标，用于长目录滚动。
	Offset int
	// Err 是列目录或进入目录失败的说明，非空时列表为空。
	Err string

	height int
}

// New 打开一个目录用于浏览。读不出来时不会失败，
// 只把原因记进 Err 交给界面显示——用户打错路径不该让程序崩溃。
func New(dir string) *Browser {
	b := &Browser{Dir: dir}
	b.Reload()
	return b
}

// Reload 重新读取目录内容。
//
// 「..」固定排在最前用于返回上级；其余目录在文件之前、字典序在后，
// 符合文件浏览器的通用直觉。隐藏文件默认不显示：
// Go 项目里 .git 之类会淹没真正要打开的文件。
func (b *Browser) Reload() {
	b.Entries = nil
	b.Err = ""
	b.Cursor = 0
	b.Offset = 0

	entries, err := os.ReadDir(b.Dir)
	if err != nil {
		b.Err = "读取目录失败: " + err.Error()
		return
	}

	b.Entries = append(b.Entries, Entry{Name: "..", Dir: true})
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		isDir := entry.IsDir()
		if !isDir && entry.Type()&os.ModeSymlink != 0 {
			// 符号链接按它指向的类型判断，否则进入链接目录会失败。
			if info, err := os.Stat(filepath.Join(b.Dir, name)); err == nil {
				isDir = info.IsDir()
			}
		}
		b.Entries = append(b.Entries, Entry{Name: name, Dir: isDir})
	}

	sort.SliceStable(b.Entries, func(i, j int) bool {
		// 「..」永远排最前，不能参与字典序比较。
		if b.Entries[i].Name == ".." {
			return true
		}
		if b.Entries[j].Name == ".." {
			return false
		}
		if b.Entries[i].Dir != b.Entries[j].Dir {
			return b.Entries[i].Dir
		}
		return b.Entries[i].Name < b.Entries[j].Name
	})
}

// Selected 返回当前选中的项，没有则返回 false。
func (b *Browser) Selected() (Entry, bool) {
	if b.Cursor < 0 || b.Cursor >= len(b.Entries) {
		return Entry{}, false
	}
	return b.Entries[b.Cursor], true
}

// CurrentPath 返回选中项的完整路径。
func (b *Browser) CurrentPath() string {
	entry, ok := b.Selected()
	if !ok {
		return ""
	}
	return filepath.Join(b.Dir, entry.Name)
}

// Move 把选择移动 delta 行。
func (b *Browser) Move(delta int) {
	if len(b.Entries) == 0 {
		return
	}
	b.Cursor = clamp(b.Cursor+delta, 0, len(b.Entries)-1)
	b.EnsureVisible()
}

// MoveTo 把选择移到指定项。
func (b *Browser) MoveTo(index int) {
	if len(b.Entries) == 0 {
		return
	}
	b.Cursor = clamp(index, 0, len(b.Entries)-1)
	b.EnsureVisible()
}

// VisibleHeight 返回列表区可见行数。
func (b *Browser) VisibleHeight() int {
	if b.height < 1 {
		return 1
	}
	return b.height
}

// EnsureVisible 保证选中项落在可视范围内。
func (b *Browser) EnsureVisible() {
	height := b.VisibleHeight()
	if b.Cursor < b.Offset {
		b.Offset = b.Cursor
	}
	if b.Cursor >= b.Offset+height {
		b.Offset = b.Cursor - height + 1
	}
	b.Offset = clamp(b.Offset, 0, maxInt(len(b.Entries)-height, 0))
}

// Action 是按一次键的结果。
type Action uint8

const (
	// None 表示按键已被消费但什么也没发生。
	None Action = iota
	// Open 表示用户确认打开当前选中项。
	Open
	// Enter 表示用户要进入当前选中的目录。
	Enter
	// Up 表示用户要返回上级目录。
	Up
	// Close 表示用户关闭了浮层。
	Close
)

// HandleKey 处理一次按键。
func (b *Browser) HandleKey(key string) Action {
	switch key {
	case "<esc>":
		return Close
	case "<enter>":
		// 回车对目录是进入，对文件是打开。
		if entry, ok := b.Selected(); ok && entry.Dir {
			return Enter
		}
		return Open
	case "<right>":
		// spec 一.2：右箭头打开/进入。
		if entry, ok := b.Selected(); ok && entry.Dir {
			return Enter
		}
		return Open
	case "<left>":
		return Up
	case "<up>":
		b.Move(-1)
	case "<down>":
		b.Move(1)
	case "<pgup>":
		b.Move(-b.VisibleHeight())
	case "<pgdown>":
		b.Move(b.VisibleHeight())
	case "<home>":
		b.MoveTo(0)
	case "<end>":
		b.MoveTo(len(b.Entries) - 1)
	}
	return None
}

// Enter 进入选中项对应的目录。
func (b *Browser) Enter() {
	entry, ok := b.Selected()
	if !ok || !entry.Dir {
		return
	}
	if entry.Name == ".." {
		b.GoUp()
		return
	}
	next := filepath.Join(b.Dir, entry.Name)
	// 进入之前先确认它确实是可进入的目录：
	// 悬空符号链接会让浏览停在一个打不开的路径上。
	info, err := os.Stat(next)
	if err != nil || !info.IsDir() {
		b.Err = "进不去: " + entry.Name
		return
	}
	b.Dir = next
	b.Reload()
}

// GoUp 返回上级目录。已在根目录时不动。
func (b *Browser) GoUp() {
	parent := filepath.Dir(b.Dir)
	if parent == b.Dir {
		b.Err = "已经在最上层"
		return
	}
	b.Dir = parent
	b.Reload()
}

// Hint 返回底部提示文字。提示里必须说清每个键的作用。
func (b *Browser) Hint() string {
	return "up/down 选择  right 打开或进入  left 返回上级  esc 取消"
}

// Render 渲染浮层，返回整块内容与光标位置。
//
// 每一行的可见宽度都必须精确等于 width，行数必须精确等于 height：
// 少一列会露出上一帧的残影，多一列会让终端把整行折成两行。
// 因此这里一律先算「纯文本」，补齐到 width 之后再套样式——
// 按带转义序列的串去量宽度是不成立的。
func (b *Browser) Render(theme view.Theme, width, height int) (string, *screen.CursorSpec) {
	width = maxInt(width, 0)
	limit := maxInt(height, 1)

	// 减去标题行。
	b.height = maxInt(limit-1, 1)
	b.EnsureVisible()

	out := make([]string, 0, limit)
	// 标题必须带模式名：只显示一个路径的话，
	// 用户看不出自己是在浏览浮层里、还是在看一个文件。
	out = append(out, fit("  浏览: "+b.Dir, width))

	switch {
	case b.Err != "":
		out = append(out, fit("  "+b.Err, width))
	case len(b.Entries) == 0:
		out = append(out, fit("  (空目录)", width))
	default:
		for i := b.Offset; i < len(b.Entries) && i < b.Offset+b.VisibleHeight(); i++ {
			out = append(out, b.renderEntry(theme, b.Entries[i], i == b.Cursor, width))
		}
	}

	// 截到可用高度，再补满：帧高必须稳定，否则终端会不停滚动。
	if len(out) > limit {
		out = out[:limit]
	}
	for len(out) < limit {
		out = append(out, fit("", width))
	}

	var cursor *screen.CursorSpec
	// 标题占第 0 行，列表从第 1 行开始。
	row := 1 + (b.Cursor - b.Offset)
	if len(b.Entries) > 0 && b.Err == "" && row >= 1 && row < len(out) {
		cursor = &screen.CursorSpec{X: minInt(2, maxInt(width-1, 0)), Y: row,
			Shape: screen.CursorBlock, Blink: true}
	}
	return strings.Join(out, "\n"), cursor
}

// renderEntry 渲染一行条目。
func (b *Browser) renderEntry(theme view.Theme, entry Entry, selected bool, width int) string {
	name := entry.Name
	if entry.Dir && name != ".." {
		name += "/"
	}
	line := fit("  "+name, width)
	if selected {
		return theme.CurrentLine + line + "\x1b[0m"
	}
	return line
}

// fit 把纯文本裁剪并补齐到指定宽度。
//
// view.Fit 可能因为放不下一个宽字符而返回略短的串（宁可少一列也不切字），
// 所以补齐要放在裁剪之后，才能保证宽度精确。
func fit(text string, width int) string {
	if width <= 0 {
		return ""
	}
	fitted := view.Fit(text, width)
	if pad := width - view.TextWidth(fitted); pad > 0 {
		return fitted + strings.Repeat(" ", pad)
	}
	return fitted
}

// clamp 把 v 夹到 [lo, hi]。
func clamp(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
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
