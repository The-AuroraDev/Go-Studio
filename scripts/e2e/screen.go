// screen.go — 极简虚拟终端：把编辑器的输出重建成一张可见的屏幕。
// SPDX-License-Identifier: MIT

//go:build !windows

package main

import (
	"strconv"
	"strings"
)

// virtualScreen 把带 ANSI 的输出流重建成「终端上真正显示的样子」。
//
// 为什么必须有它：Bubble Tea 做的是差分渲染，只重写发生变化的单元格。
// 光标从 1:2 走到 1:6 时，它只把那几个数字单元格改掉，
// 字节流里看到的是 "…e3 s4 i5 z6" 这种碎片，
// "1:6" 作为连续字符串永远不会出现。
// 断言字节流等于在断言渲染器的内部实现细节；
// 断言重建后的屏幕才是「用户看到了什么」。
type virtualScreen struct {
	width, height int
	cells         [][]rune
	row, col      int
	// last 是最近一个被写进屏幕的字符，供 REP 序列复用。
	last rune
	// scrollTop、scrollBottom 是滚动区域（DECSTBM），闭区间，0 基。
	// 整屏滚动时它们分别是 0 与 height-1。
	scrollTop, scrollBottom int
}

// newVirtualScreen 造一张全空屏幕。
func newVirtualScreen(width, height int) *virtualScreen {
	s := &virtualScreen{width: width, height: height}
	s.clear()
	return s
}

// resize 改变屏幕尺寸并清屏。
func (s *virtualScreen) resize(width, height int) {
	s.width, s.height = width, height
	s.clear()
}

// clear 清空整屏并把光标移到左上角。
func (s *virtualScreen) clear() {
	s.cells = make([][]rune, s.height)
	for r := range s.cells {
		s.cells[r] = blankRow(s.width)
	}
	s.row, s.col = 0, 0
	s.last = 0
	s.scrollTop, s.scrollBottom = 0, maxInt(s.height-1, 0)
}

// blankRow 造一行全空格。
func blankRow(width int) []rune {
	row := make([]rune, width)
	for i := range row {
		row[i] = ' '
	}
	return row
}

// apply 把一段输出喂进屏幕。
func (s *virtualScreen) apply(data string) {
	runes := []rune(data)
	for i := 0; i < len(runes); {
		switch r := runes[i]; r {
		case 0x1b:
			i = s.applyEscape(runes, i)
		case '\n':
			s.lineFeed()
			i++
		case '\r':
			s.col = 0
			i++
		case '\b':
			if s.col > 0 {
				s.col--
			}
			i++
		case '\t':
			// 制表符推进到下一个 8 列停靠点。
			next := (s.col/8 + 1) * 8
			if next > s.width {
				next = s.width
			}
			s.col = next
			i++
		default:
			if r < 0x20 {
				i++ // 其余控制字符不可见，忽略
				continue
			}
			s.put(r)
			s.last = r
			i++
		}
	}
}

// applyEscape 处理 ESC 开头的一个转义序列，返回其后一个位置的下标。
func (s *virtualScreen) applyEscape(runes []rune, i int) int {
	j := i + 1
	if j >= len(runes) {
		return j
	}

	switch runes[j] {
	case '[':
		// CSI：ESC [ 参数 中间字节 最终字节
		j++
		start := j
		for j < len(runes) && !(runes[j] >= 0x40 && runes[j] <= 0x7e) {
			j++
		}
		if j >= len(runes) {
			return j
		}
		params := string(runes[start:j])
		final := runes[j]
		j++
		s.applyCSI(params, final)
		return j

	case ']':
		// OSC：到 BEL 或 ST 为止，整段丢弃。
		for j < len(runes) {
			if runes[j] == 0x07 {
				return j + 1
			}
			if runes[j] == 0x1b && j+1 < len(runes) && runes[j+1] == '\\' {
				return j + 2
			}
			j++
		}
		return j

	default:
		// ESC 单字符序列（如 ESC ( B）：两字节，跳过。
		return j + 1
	}
}

// applyCSI 处理 CSI 序列里我们真正关心的那几条。
//
// 只实现这些是因为渲染器只会用到它们：定位光标、清行、清屏。
// 其余（SGR 颜色、模式切换、光标显隐）都不影响可见字符，忽略即可——
// 屏幕上「显示成什么样」由字符本身决定，颜色不参与宽度计算。
func (s *virtualScreen) applyCSI(params string, final rune) {
	nums := parseParams(params)

	switch final {
	case 'H', 'f': // 光标绝对定位（行列均从 1 开始）
		row, col := 1, 1
		if len(nums) > 0 && nums[0] > 0 {
			row = nums[0]
		}
		if len(nums) > 1 && nums[1] > 0 {
			col = nums[1]
		}
		s.moveTo(row-1, col-1)

	case 'A': // 光标上移
		s.row -= intOr(nums, 0, 1)
		s.clampCursor()
	case 'B': // 光标下移
		s.row += intOr(nums, 0, 1)
		s.clampCursor()
	case 'C': // 光标右移
		s.col += intOr(nums, 0, 1)
		s.clampCursor()
	case 'D': // 光标左移
		s.col -= intOr(nums, 0, 1)
		s.clampCursor()
	case 'G': // 光标横向绝对定位
		s.col = intOr(nums, 0, 1) - 1
		s.clampCursor()

	case '`': // HPA，与 G 同义
		s.col = intOr(nums, 0, 1) - 1
		s.clampCursor()

	case 'd': // VPA：纵向绝对定位，只改行不改列
		// 渲染器靠它跳到状态栏那一行。不实现的话状态栏会画在当前行上，
		// 而本该显示正文的那一行被覆盖掉——症状是「正文整片空白」。
		s.row = intOr(nums, 0, 1) - 1
		s.clampCursor()

	case 'a': // HPR：横向相对右移
		s.col += intOr(nums, 0, 1)
		s.clampCursor()

	case 'e': // VPR：纵向相对下移
		s.row += intOr(nums, 0, 1)
		s.clampCursor()

	case 'J': // 擦除显示
		mode := intOr(nums, 0, 0)
		switch mode {
		case 0: // 从光标到屏幕末尾
			s.eraseFromCursorToEnd()
		case 1: // 从屏幕开头到光标
			s.eraseFromStartToCursor()
		default: // 2 或 3：整屏
			s.clear()
		}

	case 'K': // 擦除行
		mode := intOr(nums, 0, 0)
		switch mode {
		case 0:
			s.eraseLineToEnd()
		case 1:
			s.eraseLineToStart()
		default:
			s.eraseWholeLine()
		}

	case 'X': // ECH：擦除光标起的 n 个字符（不移位）
		s.eraseChars(intOr(nums, 0, 1))

	case 'P': // DCH：删除光标起的 n 个字符，后面的左移补空格
		// 渲染器用它来「删除」一个单元格，而不是拿空格盖住。
		// 不实现就会在屏幕上留下残字——例如状态栏里出现 "NOR6 L"
		// 这种被削掉一半的文字。断言会因此全部失真。
		s.deleteChars(intOr(nums, 0, 1))

	case '@': // ICH：在光标处插入 n 个空格，后面的右移
		s.insertChars(intOr(nums, 0, 1))

	case 'L': // IL：在光标行插入 n 个空行
		s.insertLines(intOr(nums, 0, 1))

	case 'M': // DL：删除光标起的 n 个行，下面的上移补空行
		s.deleteLines(intOr(nums, 0, 1))

	case 'r': // DECSTBM：设置滚动区域（上下边界，1 基）
		// 渲染器靠「设定区域 + 滚动」来整体移动内容，
		// 比逐行重画省字节。不实现的话关掉浮层时旧内容不会被推走。
		top, bottom := 1, s.height
		if len(nums) > 0 && nums[0] > 0 {
			top = nums[0]
		}
		if len(nums) > 1 && nums[1] > 0 {
			bottom = nums[1]
		}
		if bottom > s.height {
			bottom = s.height
		}
		if top < 1 {
			top = 1
		}
		if bottom < top {
			top, bottom = 1, s.height
		}
		s.scrollTop, s.scrollBottom = top-1, bottom-1
		// DECSTBM 会把光标移回区域左上角。
		s.moveTo(s.scrollTop, 0)

	case 'S': // SU：区域内上滚 n 行，底部补空行
		s.scrollUp(intOr(nums, 0, 1))

	case 'T': // SD：区域内下滚 n 行，顶部补空行
		s.scrollDown(intOr(nums, 0, 1))

	case 'b': // REP：把上一个字符再写 n 次
		// 渲染器会用它压缩重复内容：写一串空格与其写 n 次空格，
		// 前者只要 4 个字节。不实现就会在屏幕上留下旧字符——
		// 症状是状态栏里出现 "NOR6 L" 这种被削掉一半的文字。
		s.repeatLast(intOr(nums, 0, 1))
	}
}

// parseParams 解析 CSI 的数字参数，空段按 0 处理。
func parseParams(params string) []int {
	if params == "" {
		return nil
	}
	parts := strings.Split(params, ";")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			n = 0
		}
		out = append(out, n)
	}
	return out
}

// intOr 取 params[i]，越界或为 0 时用 fallback。
func intOr(params []int, i, fallback int) int {
	if i < len(params) && params[i] != 0 {
		return params[i]
	}
	return fallback
}

// moveTo 把光标移到绝对位置。
func (s *virtualScreen) moveTo(row, col int) {
	if row < 0 {
		row = 0
	}
	if col < 0 {
		col = 0
	}
	s.row, s.col = row, col
	s.clampCursor()
}

// clampCursor 把光标夹在屏幕内。
func (s *virtualScreen) clampCursor() {
	if s.row < 0 {
		s.row = 0
	}
	if s.row >= s.height {
		s.row = s.height - 1
	}
	if s.col < 0 {
		s.col = 0
	}
	if s.col >= s.width {
		s.col = s.width - 1
	}
}

// lineFeed 换行。到屏幕底部就停住，不滚动——
// 编辑器的帧是整屏重画，这里不需要滚动模型。
func (s *virtualScreen) lineFeed() {
	s.row++
	if s.row >= s.height {
		s.row = s.height - 1
	}
}

// put 在光标处写一个字符，右移光标。
//
// 这里不做自动折行。raw 模式下终端不启用 DECAWM（自动换行），
// 渲染器写满一行后会显式发 CRLF 或重新定位光标。
// 若在这里折行，写满 100 列的行会先折一次、随后 CRLF 又换一次，
// 整屏就会一行一行往上错位。
func (s *virtualScreen) put(r rune) {
	if s.row < 0 || s.row >= s.height || s.col < 0 || s.col >= s.width {
		return
	}
	s.cells[s.row][s.col] = r
	s.col++
	if s.col > s.width {
		s.col = s.width
	}
}

// eraseFromCursorToEnd 清掉光标之后的内容（含光标所在格到屏幕末尾）。
func (s *virtualScreen) eraseFromCursorToEnd() {
	s.eraseLineToEnd()
	for r := s.row + 1; r < s.height; r++ {
		s.cells[r] = blankRow(s.width)
	}
}

// eraseFromStartToCursor 清掉屏幕开头到光标的内容。
func (s *virtualScreen) eraseFromStartToCursor() {
	for r := 0; r < s.row; r++ {
		s.cells[r] = blankRow(s.width)
	}
	s.eraseLineToStart()
}

// eraseLineToEnd 清掉本行光标之后的内容。
func (s *virtualScreen) eraseLineToEnd() {
	if s.row < 0 || s.row >= s.height {
		return
	}
	for c := s.col; c < s.width; c++ {
		s.cells[s.row][c] = ' '
	}
}

// eraseLineToStart 清掉本行开头到光标的内容。
func (s *virtualScreen) eraseLineToStart() {
	if s.row < 0 || s.row >= s.height {
		return
	}
	for c := 0; c <= s.col && c < s.width; c++ {
		s.cells[s.row][c] = ' '
	}
}

// eraseWholeLine 清空本行。
func (s *virtualScreen) eraseWholeLine() {
	if s.row < 0 || s.row >= s.height {
		return
	}
	s.cells[s.row] = blankRow(s.width)
}

// line 返回第 row 行的可见内容，去掉行尾空白。
func (s *virtualScreen) line(row int) string {
	if row < 0 || row >= s.height {
		return ""
	}
	return strings.TrimRight(string(s.cells[row]), " ")
}

// String 返回整屏可见内容，每行去尾空白，行间用换行连接。
func (s *virtualScreen) String() string {
	rows := make([]string, 0, s.height)
	for r := 0; r < s.height; r++ {
		rows = append(rows, s.line(r))
	}
	return strings.Join(rows, "\n")
}

// contains 报告整屏可见内容里是否包含 want。
func (s *virtualScreen) contains(want string) bool {
	return strings.Contains(s.String(), want)
}

// ---- 编辑类序列 ----
//
// 渲染器不是只靠「重画整行」来更新的，它会用 DCH/ICH 这类序列
// 局部增删单元格。真实终端全都实现了这些序列；
// 少实现一个，屏幕上就会留下残字，而断言残字毫无意义。

// repeatLast 把最近写入的字符再写 n 次。
// 此前一个字符都没写过时按空格处理。
func (s *virtualScreen) repeatLast(n int) {
	ch := s.last
	if ch == 0 {
		ch = ' '
	}
	for i := 0; i < n; i++ {
		s.put(ch)
		s.last = ch
	}
}

// rowCells 返回第 row 行的可写切片，越界返回 nil。
func (s *virtualScreen) rowCells(row int) []rune {
	if row < 0 || row >= s.height {
		return nil
	}
	return s.cells[row]
}

// eraseChars 从光标起擦除 n 个字符。
func (s *virtualScreen) eraseChars(n int) {
	row := s.rowCells(s.row)
	if row == nil {
		return
	}
	for i := 0; i < n && s.col+i < s.width; i++ {
		row[s.col+i] = ' '
	}
}

// deleteChars 从光标起删除 n 个字符，后面的左移、行尾补空格。
func (s *virtualScreen) deleteChars(n int) {
	row := s.rowCells(s.row)
	if row == nil {
		return
	}
	if n < 1 {
		n = 1
	}
	copy(row[s.col:], row[s.col+n:])
	for i := s.width - n; i < s.width; i++ {
		if i >= 0 {
			row[i] = ' '
		}
	}
	if s.col >= s.width {
		s.col = maxInt(s.width-1, 0)
	}
}

// insertChars 在光标处插入 n 个空格，后面的右移、超出部分丢弃。
func (s *virtualScreen) insertChars(n int) {
	row := s.rowCells(s.row)
	if row == nil {
		return
	}
	if n < 1 {
		n = 1
	}
	copy(row[s.col+n:], row[s.col:])
	for i := 0; i < n && s.col+i < s.width; i++ {
		row[s.col+i] = ' '
	}
}

// insertLines 在光标行处插入 n 个空行，下面的行下移、超出部分丢弃。
func (s *virtualScreen) insertLines(n int) {
	if n < 1 {
		n = 1
	}
	for i := 0; i < n; i++ {
		copy(s.cells[s.row+1:], s.cells[s.row:])
		s.cells[s.row] = blankRow(s.width)
	}
}

// deleteLines 删除光标起的 n 个行，下面的行上移、底部补空行。
func (s *virtualScreen) deleteLines(n int) {
	if n < 1 {
		n = 1
	}
	for i := 0; i < n; i++ {
		copy(s.cells[s.row:], s.cells[s.row+1:])
		s.cells[s.height-1] = blankRow(s.width)
	}
}

// scrollBounds 返回合法的滚动区域与行数。
func (s *virtualScreen) scrollBounds() (top, bottom, count int) {
	top, bottom = s.scrollTop, s.scrollBottom
	if top < 0 {
		top = 0
	}
	if bottom >= s.height {
		bottom = s.height - 1
	}
	if bottom < top {
		return 0, -1, 0
	}
	return top, bottom, bottom - top + 1
}

// scrollUp 在滚动区域内上滚 n 行，底部补空行。
func (s *virtualScreen) scrollUp(n int) {
	top, bottom, count := s.scrollBounds()
	if count == 0 {
		return
	}
	if n < 1 {
		n = 1
	}
	if n > count {
		n = count
	}
	// 目标区间是 [top, bottom-n]，来源区间是 [top+n, bottom]，
	// 两段长度都是 count-n。写清楚比手算下标可靠得多。
	copy(s.cells[top:bottom-n+1], s.cells[top+n:bottom+1])
	for r := bottom - n + 1; r <= bottom; r++ {
		s.cells[r] = blankRow(s.width)
	}
}

// scrollDown 在滚动区域内下滚 n 行，顶部补空行。
func (s *virtualScreen) scrollDown(n int) {
	top, bottom, count := s.scrollBounds()
	if count == 0 {
		return
	}
	if n < 1 {
		n = 1
	}
	if n > count {
		n = count
	}
	for r := bottom; r >= top+n; r-- {
		s.cells[r] = s.cells[r-n]
	}
	for r := top; r < top+n; r++ {
		s.cells[r] = blankRow(s.width)
	}
}

// maxInt 返回两者中较大的一个。
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
