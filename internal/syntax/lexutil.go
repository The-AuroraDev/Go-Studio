// lexutil.go — 各语言规则共用的词法辅助函数。
// SPDX-License-Identifier: MIT

package syntax

// 这些函数刻意写成「无状态、纯函数」：高亮结果要能被缓存，
// 同样的输入必须得到同样的输出。

// newSet 从字符串列表建集合。
func newSet(words ...string) map[string]bool {
	set := make(map[string]bool, len(words))
	for _, w := range words {
		set[w] = true
	}
	return set
}

// indexRunes 返回 sub 首次出现的下标，找不到返回 -1。
func indexRunes(line []rune, sub string, from int) int {
	if sub == "" || from < 0 || from > len(line) {
		return -1
	}
	want := []rune(sub)
	limit := len(line) - len(want)
	for i := from; i <= limit; i++ {
		match := true
		for j := range want {
			if line[i+j] != want[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// indexRuneFrom 返回 r 从 from 起首次出现的下标，找不到返回 -1。
func indexRuneFrom(line []rune, r rune, from int) int {
	for i := from; i < len(line); i++ {
		if line[i] == r {
			return i
		}
	}
	return -1
}

// startsWithAt 报告 line 在 i 处是否以 sub 开头。
func startsWithAt(line []rune, i int, sub string) bool {
	if i < 0 || i+len(sub) > len(line) {
		return false
	}
	for j, r := range sub {
		if line[i+j] != r {
			return false
		}
	}
	return true
}

// scanIdent 从 i 起吃掉一个标识符，返回结束后的下标。
func scanIdent(line []rune, i int) int {
	j := i
	for j < len(line) && isIdentRune(line[j]) {
		j++
	}
	return j
}

// scanNumber 从 i 起吃掉一个数字字面符，返回结束后的下标。
//
// 覆盖整数、浮点、十六进制与各类后缀：
// 0x1F、1_000、3.14e-9、0b1010、100u8、'A'。
func scanNumber(line []rune, i int) int {
	j := i

	// 先吃掉可选的正负号。JSON/TOML/YAML 把 '-' 当作数字的起点传进来，
	// 若这里不吞掉它，下面的数字循环一次都不前进，调用方又是 i = j，
	// 整个编辑器就会卡死在这一行。顺便保证任何输入都能前进至少一格。
	if j < len(line) && (line[j] == '-' || line[j] == '+') {
		j++
	}
	if j >= len(line) {
		return j
	}

	// 字符字面量：'a'、'\n'、'\u00e9'。
	if line[j] == '\'' {
		j++
		if j < len(line) && line[j] == '\\' {
			j += 2
		} else if j < len(line) {
			j++
		}
		if j < len(line) && line[j] == '\'' {
			j++
		}
		return j
	}

	if j < len(line) && line[j] == '0' && j+1 < len(line) {
		switch line[j+1] {
		case 'x', 'X', 'b', 'B', 'o', 'O':
			j += 2
			for j < len(line) && (isHexDigit(line[j]) || line[j] == '_') {
				j++
			}
			return scanNumberSuffix(line, j)
		}
	}

	for j < len(line) && (isDigit(line[j]) || line[j] == '_') {
		j++
	}
	// 小数部分。
	if j+1 < len(line) && line[j] == '.' && isDigit(line[j+1]) {
		j++
		for j < len(line) && (isDigit(line[j]) || line[j] == '_') {
			j++
		}
	}
	// 指数部分。
	if j < len(line) && (line[j] == 'e' || line[j] == 'E') {
		k := j + 1
		if k < len(line) && (line[k] == '+' || line[k] == '-') {
			k++
		}
		if k < len(line) && isDigit(line[k]) {
			j = k
			for j < len(line) && (isDigit(line[j]) || line[j] == '_') {
				j++
			}
		}
	}
	return scanNumberSuffix(line, j)
}

// scanNumberSuffix 吃掉数字类型后缀，如 100u8、1.5f32。
func scanNumberSuffix(line []rune, j int) int {
	for j < len(line) && isIdentRune(line[j]) {
		j++
	}
	return j
}

// isHexDigit 判断是否是十六进制数字。
func isHexDigit(r rune) bool {
	return isDigit(r) || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

// isOperatorRune 判断是否是运算符字符。
//
// 引号单独处理：字符串定界符也在 ASCII 标点范围内，
// 若把它算成运算符，字符串的着色会在引号处断掉。
func isOperatorRune(r rune) bool {
	switch r {
	case '"', '\'', '`':
		return false
	}
	switch r {
	case '+', '-', '*', '/', '%', '^', '&', '|', '~', '!', '<', '>', '=', '?',
		':', '.':
		// 冒号与点号单独处理：Go 的标签、Rust 的路径分隔符需要更细的判断。
		return r != ':' && r != '.'
	}
	return false
}

// operatorRunes 是运算符里那些「可以连成一个整体」的多字符符号。
var operatorRunes = []string{
	// 长的排前面，否则 "==" 会被切成两个 "="。
	"<<=", ">>=", "...", "->>", "<=>",
	// === 与 !== 必须排在 == 与 != 前面，否则会被切成两个字符。
	"===", "!==",
	"==", "!=", "<=", ">=", "&&", "||", "++", "--", "+=", "-=", "*=", "/=",
	"%=", "&=", "|=", "^=", "<<", ">>", "->", "=>", "::", "??", "?.",
}

// scanOperator 从 i 起吃掉一个运算符，返回结束后的下标。
//
// 先试双字符与三字符运算符，避免把 "==" 拆成两个 "="。
func scanOperator(line []rune, i int) int {
	for _, op := range operatorRunes {
		if startsWithAt(line, i, op) {
			return i + len([]rune(op))
		}
	}
	return i + 1
}

// scanQuoted 扫描一个定界字符串（含转义），返回结束后的下标。
//
// 转义里的定界符不能提前结束字符串，所以要逐字符跟踪反斜杠。
// 收尾定界符缺失时，本行剩余部分仍按字符串着色——
// 编辑器要能显示「这里少了个引号」，而不是默默不着色。
func scanQuoted(line []rune, i int, quote rune, out *[]Token) int {
	*out = emit(*out, KindString, i, i+1)
	j := i + 1
	for j < len(line) {
		switch line[j] {
		case '\\':
			*out = emit(*out, KindEscape, j, minInt(j+2, len(line)))
			j += 2
		case quote:
			*out = emit(*out, KindString, j, j+1)
			return j + 1
		default:
			j++
		}
	}
	*out = emit(*out, KindString, j, len(line))
	return len(line)
}

// minInt 返回两者中较小的。
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// lexIdentAt 按规则给标识符分类。
//
// 这是各语言共用的「关键字 / 类型 / 内建 / 普通」的判定骨架，
// 差别只在传进来的三张表。
func lexIdentAt(line []rune, start, end int, tables wordTables) Kind {
	word := string(line[start:end])
	switch {
	case tables.keywords[word]:
		return KindKeyword
	case tables.types[word]:
		return KindKeywordType
	case tables.builtins[word]:
		return KindBuiltin
	case tables.literals[word]:
		return KindConstant
	}
	// 紧跟左括号的是函数调用或定义。
	if end < len(line) && line[end] == '(' {
		return KindFunction
	}
	// 首字母大写在多数语言里是类型或常量。
	if isUpper(rune(word[0])) {
		return KindType
	}
	return KindText
}

// wordTables 是一门语言的四张标识符表。
type wordTables struct {
	keywords map[string]bool
	types    map[string]bool
	builtins map[string]bool
	literals map[string]bool
}

// PunctRune 判断字符是否是不含语义的标点。
func PunctRune(r rune) bool {
	switch r {
	case '(', ')', '[', ']', '{', '}', ',', ';', '.', '\\', '@', '#':
		return true
	}
	return false
}

// isPreprocLine 报告从 i 起是否是一个预处理指令行（允许前导空白）。
func isPreprocLine(line []rune, i int) bool {
	j := i
	for j < len(line) && (line[j] == ' ' || line[j] == '\t') {
		j++
	}
	return j < len(line) && line[j] == '#'
}

// scanCharLiteral 扫描字符字面量，返回结束下标与是否成功。
//
// Rust 的生命周期 'a、C++ 的模板 '...' 分隔符都不是字符字面量。
// 用「五个字符内必须出现收尾引号」这个硬上限把两者区分开：
// 真的字面量最多 '\\x41'、'é' 这么长，生命周期后面不会紧跟引号。
func scanCharLiteral(line []rune, i int, out *[]Token) (int, bool) {
	limit := minInt(i+5, len(line))
	for j := i + 1; j < limit; j++ {
		if line[j] == '\\' {
			j++ // 跳过被转义的那个字符
			continue
		}
		if line[j] == '\'' {
			*out = emit(*out, KindString, i, j+1)
			return j + 1, true
		}
	}
	return i, false
}

// scanRawString 扫描 Rust 的原样字符串 r"..."、r#"..."#、r##"..."##。
// 返回结束下标与是否匹配到这种形式。
func scanRawString(line []rune, i int, marker rune) (int, bool) {
	if line[i] != marker || i+1 >= len(line) {
		return i, false
	}
	j := i + 1
	hashes := 0
	for j < len(line) && line[j] == '#' {
		hashes++
		j++
	}
	if j >= len(line) || line[j] != '"' {
		return i, false
	}
	j++

	closer := make([]rune, 0, hashes+1)
	closer = append(closer, '"')
	for k := 0; k < hashes; k++ {
		closer = append(closer, '#')
	}
	for ; j < len(line); j++ {
		if startsWithAt(line, j, string(closer)) {
			return j + len(closer), true
		}
	}
	// 没找到收尾：原样字符串确实可以跨行，但本行先着色到这里。
	return len(line), true
}

// isBlank 报告是否是空白字符。
//
// 空白不产生 token：渲染层本来就按原样输出它们，
// 给空白套一个 token 只是白白多分配内存——十万行代码里
// 空白能占到三成。
func isBlank(r rune) bool {
	return r == ' ' || r == '\t'
}
