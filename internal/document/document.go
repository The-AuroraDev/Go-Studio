// document.go — 文档：把文本缓冲、光标、撤销栈和脏标记组装成编辑器的一个可编辑单元。
// SPDX-License-Identifier: MIT

// Package document 是编辑器的语义层：文本怎么改、光标怎么动、何时算「有未保存改动」。
//
// 它不碰终端、不碰渲染，只面对 buffer.Buffer 与一个抽象的光标位置，
// 因此所有编辑行为都能脱离界面直接单测。
//
// 光标位置按「行、列」计，列以 rune 为单位——字节偏移对多字节字符没有意义，
// 而列号正是渲染与用户感知的单位。
package document

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
	"unicode/utf8"

	"github.com/29anan29/Go-Studio/internal/buffer"
)

// coalesceWindow 是连续输入合并成一次撤销的时长。
// 与多数编辑器的直觉一致：停手超过这个时间再敲，就算新的一次编辑。
const coalesceWindow = 500 * time.Millisecond

// Cursor 是文档内的位置。Line 从 0 开始，Col 以 rune 计、从 0 开始。
type Cursor struct {
	Line int
	Col  int
}

// Document 是一个打开的文件。
type Document struct {
	buf  *buffer.Buffer
	undo *buffer.UndoStack

	cursor Cursor
	// desiredCol 是垂直移动想要到达的列。
	// 光标穿过短行时会被压到行尾，穿回长行时要能弹回原列，
	// 否则在代码里上下移动会一路往左缩，这是最影响手感的细节之一。
	desiredCol int

	// rev 是内容版本号，每次内容变化（编辑、撤销、重做）都会变。
	// savedRev 是最后一次保存时的版本号，两者不等即「有未保存改动」。
	//
	// 版本号不能在编辑时简单自增：撤销回到保存点时必须重新变干净。
	// 因此撤销栈与重做栈各有一份平行的版本区间记录，见 revPair。
	rev      uint64
	savedRev uint64

	// undoRevs 与 undo 栈一一对应，redoRevs 与 redo 栈一一对应。
	// 每条记录该次变更前后的内容版本，使撤销与重做能精确还原版本号。
	undoRevs []revPair
	redoRevs []revPair
	// revCounter 只增不减，保证每个内容状态有唯一版本号。
	revCounter uint64

	path     string
	readonly bool

	// dirtyFrom 是最近一次编辑影响到的最前行号。
	// 语法高亮缓存靠它作废：该行之后的所有 token 都不能再信，
	// 因为一次编辑可能改变后续所有行的上下文（比如开了一个块注释）。
	dirtyFrom int
}

// revPair 是一次变更前后的内容版本。
type revPair struct {
	before uint64
	after  uint64
}

// New 造一个没有对应磁盘文件的空文档。
func New() *Document {
	return newDocument("", nil, false, coalesceWindow)
}

// FromString 用给定内容造一个内存文档，路径为空。
//
// 输入不是文件路径而是一段现成的文本，主要用于测试与「从剪贴板新建」这类场景。
// 它保留生产环境的输入合并行为，因此用它的测试看到的撤销粒度与真实编辑器一致。
// 需要逐步撤销的单测请用包内的 newDocument 显式关闭合并。
func FromString(content string) *Document {
	return newDocument("", []byte(content), false, coalesceWindow)
}

// NewNamed 造一个绑定到路径、但内容为空的文档，用于「新建文件」尚未保存的阶段。
func NewNamed(path string, readonly bool) *Document {
	return newDocument(path, nil, readonly, coalesceWindow)
}

// Open 读取文件并构造文档。
func Open(path string) (*Document, error) {
	if path == "" {
		return nil, errors.New("document: 路径为空")
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("document: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("document: %s 是目录，不是文件", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("document: 读取 %s 失败: %w", path, err)
	}
	if isBinary(data) {
		return nil, fmt.Errorf("document: %s 看起来是二进制文件，拒绝以文本方式打开", path)
	}
	return newDocument(path, data, !isWritable(info.Mode()), coalesceWindow), nil
}

// newDocument 是各构造函数共用的装配逻辑。
// coalesce 是连续输入合并成一次撤销的时长，小于等于 0 表示关闭合并：
// 单测需要每步编辑都是独立的撤销项，否则撤销行为会被合并规则干扰。
func newDocument(path string, data []byte, readonly bool, coalesce time.Duration) *Document {
	return &Document{
		buf:      buffer.New(data),
		undo:     buffer.NewUndoStack(coalesce),
		path:     path,
		readonly: readonly,
	}
}

// isWritable 判断文件权限是否允许写入。
func isWritable(mode fs.FileMode) bool { return mode.Perm()&0o222 != 0 }

// isBinary 用「前 8KB 内是否出现 NUL」判断二进制。
// 这是 git 与 grep 都在用的经典启发式：文本文件极少含 NUL。
func isBinary(data []byte) bool {
	const sniff = 8 * 1024
	if len(data) > sniff {
		data = data[:sniff]
	}
	return bytes.IndexByte(data, 0) >= 0
}

// Buffer 暴露底层缓冲，供渲染层读取文本。
// 调用方不得直接修改它，所有编辑必须走本包的方法，否则脏标记与撤销会失配。
func (d *Document) Buffer() *buffer.Buffer { return d.buf }

// Path 返回文件路径，未命名文档返回空串。
func (d *Document) Path() string { return d.path }

// Readonly 报告文档是否只读。
func (d *Document) Readonly() bool { return d.readonly }

// Dirty 报告是否有未保存改动。
//
// 它靠版本号比较而不是内容比较：撤销回到保存点时必须重新变干净，
// 这用「编辑次数」是判断不出来的。
func (d *Document) Dirty() bool { return d.rev != d.savedRev }

// Revision 返回当前内容版本号，主要供测试断言。
func (d *Document) Revision() uint64 { return d.rev }

// DirtyFromLine 返回最近一次编辑影响到的最前行号。
//
// 没有编辑过时为 0。撤销与重做同样经过 applyEdit，因此也会更新它。
func (d *Document) DirtyFromLine() int { return d.dirtyFrom }

// LineCount 返回总行数，空文档也算一行。
func (d *Document) LineCount() int { return d.buf.LineCount() }

// Cursor 返回当前光标位置。
func (d *Document) Cursor() Cursor { return d.cursor }

// MustOffset 返回光标处的字节偏移。光标始终被约束在合法范围内，因此不会失败。
func (d *Document) MustOffset() int {
	offset, err := d.buf.OffsetOf(d.cursor.Line, d.cursor.Col)
	if err != nil {
		// 光标是自己的状态，越界说明本包有 bug，必须立刻暴露而不是静默兜底。
		panic(fmt.Sprintf("document: 光标越界 %+v: %v", d.cursor, err))
	}
	return offset
}

// LineRuneLen 返回第 i 行去掉换行后的 rune 数。
func (d *Document) LineRuneLen(line int) int { return d.buf.LineRuneLen(line) }

// LineText 返回第 i 行去掉换行后的内容。
func (d *Document) LineText(line int) []byte { return d.buf.LineText(line) }

// Text 返回全文副本，仅用于测试与保存。
func (d *Document) Text() []byte { return d.buf.Text() }

// SetCursor 把光标移到指定位置，越界会被截断到合法范围。
func (d *Document) SetCursor(line, col int) {
	d.cursor = d.clamp(line, col)
	d.desiredCol = d.cursor.Col
}

// clamp 把行列截断到文档内的合法位置。
func (d *Document) clamp(line, col int) Cursor {
	if line < 0 {
		line = 0
	}
	if last := d.buf.LineCount() - 1; line > last {
		line = last
	}
	if col < 0 {
		col = 0
	}
	if max := d.buf.LineRuneLen(line); col > max {
		col = max
	}
	return Cursor{Line: line, Col: col}
}

// setCursorKeepDesired 用于垂直移动：列被压到行尾时保留期望列。
func (d *Document) setCursorKeepDesired(line, col int) {
	d.cursor = d.clamp(line, col)
}

// ---- 文件读写 ----

// Save 把文档写回原路径。未命名文档会报错，应改用 SaveAs。
func (d *Document) Save() error {
	if d.path == "" {
		return errors.New("document: 文档没有路径，无法保存")
	}
	return d.SaveAs(d.path)
}

// SaveAs 原子地把文档写到 path：先写同目录临时文件再改名。
// 直接截断原文件再写，一旦中途失败就会留下半截内容，这是不可接受的。
func (d *Document) SaveAs(path string) error {
	if path == "" {
		return errors.New("document: 路径为空")
	}
	if d.readonly {
		return fmt.Errorf("document: %s 是只读的", path)
	}

	dir := filepath.Dir(path)
	file, err := os.CreateTemp(dir, filepath.Base(path)+".*")
	if err != nil {
		return fmt.Errorf("document: 创建临时文件失败: %w", err)
	}
	tmpName := file.Name()
	defer os.Remove(tmpName)

	// 沿用目标文件原有的权限；目标不存在时用 0o644。
	mode := fs.FileMode(0o644)
	if info, statErr := os.Stat(path); statErr == nil {
		mode = info.Mode().Perm()
	}

	if _, err := file.Write(d.buf.Text()); err != nil {
		file.Close()
		return fmt.Errorf("document: 写入临时文件失败: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("document: 关闭临时文件失败: %w", err)
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		return fmt.Errorf("document: 设置权限失败: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("document: 替换 %s 失败: %w", path, err)
	}

	// 保存之后这个内容状态就是基准，脏标记随之清零。
	d.savedRev = d.rev
	d.path = path
	d.readonly = false
	return nil
}

// ---- 版本号维护 ----

// nextRev 产生一个新的内容版本号。
func (d *Document) nextRev() uint64 {
	d.revCounter++
	d.rev = d.revCounter
	return d.rev
}

// recordEdit 在 Push 之后同步撤销栈的版本区间。
// merged 为真表示这次编辑并入了栈顶，不能新增一条记录。
func (d *Document) recordEdit(before uint64, merged bool) {
	after := d.nextRev()
	if merged && len(d.undoRevs) > 0 {
		d.undoRevs[len(d.undoRevs)-1].after = after
		return
	}
	d.undoRevs = append(d.undoRevs, revPair{before: before, after: after})
}

// utf8Len 返回字符串的 rune 数，供缩进计算使用。
func utf8Len(s string) int { return utf8.RuneCountInString(s) }
