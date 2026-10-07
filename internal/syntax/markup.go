// markup.go — Markdown、HTML、XML 的高亮规则。
// SPDX-License-Identifier: MIT

package syntax

func init() {
	Register(markdownLexer{})
	Register(htmlLexer{})
}

// ---- Markdown ----

// markdownLexer 是 Markdown 的规则。
//
// Markdown 有跨行结构：围栏代码块可以包含任意行，链接目标在下一行。
// 两者都要靠状态跨行延续。行内元素（粗体、代码、行内链接）
// 只在同一行里，用状态机即可。
type markdownLexer struct{}

func (markdownLexer) Name() string         { return "Markdown" }
func (markdownLexer) Extensions() []string { return []string{".md", ".markdown", ".mdown"} }
func (markdownLexer) Filenames() []string {
	return []string{"README", "README.md", "CHANGELOG", "CONTRIBUTING", "LICENSE"}
}
func (markdownLexer) LineComment() string { return "" }
func (markdownLexer) BlockComment() (string, string, bool) {
	return "", "", false
}

func (markdownLexer) LexLine(line []rune, state State) ([]Token, State) {
	var out []Token

	// 围栏代码块内：整行按代码着色，什么都不解析。
	if state.Has(stateInBlockComment) {
		if isFenceClose(line) {
			out = emit(out, KindPreproc, 0, len(line))
			return out, state.Without(stateInBlockComment)
		}
		return emit(out, KindString, 0, len(line)), state
	}

	trimmed := trimLeftSpace(line)
	indent := indexOfNonSpace(line, 0)

	// 空行没有 token，避免渲染层为空行白算一趟。
	if len(trimmed) == 0 {
		return nil, state
	}

	// 代码围栏的开启与关闭。
	if isFence(line) {
		out = emit(out, KindPreproc, indent, len(line))
		return out, state.With(stateInBlockComment)
	}
	// ATX 标题：# 开头。
	if trimmed[0] == '#' {
		j := indent
		for j < len(line) && line[j] == '#' {
			j++
		}
		out = emit(out, KindPreproc, indent, j)
		out = emit(out, KindKeyword, j, len(line))
		return out, state
	}
	// 引用与列表标记：> - + * 或数字加点
	if trimmed[0] == '>' {
		out = emit(out, KindKeyword, indent, indent+1)
		return markdownInline(line, indent+1, out, state)
	}
	if trimmed[0] == '-' || trimmed[0] == '*' || trimmed[0] == '+' {
		if len(trimmed) == 1 || trimmed[1] == ' ' {
			out = emit(out, KindOperator, indent, indent+1)
			return markdownInline(line, indent+1, out, state)
		}
	}
	if len(trimmed) > 1 && isDigit(rune(trimmed[0])) {
		if dot := indexRuneFrom([]rune(trimmed), '.', 0); dot > 0 {
			out = emit(out, KindOperator, indent, indent+dot+1)
			return markdownInline(line, indent+dot+1, out, state)
		}
	}
	// 分隔线 --- ***
	if trimmed == "---" || trimmed == "***" || trimmed == "___" {
		return emit(out, KindPreproc, indent, len(line)), state
	}
	return markdownInline(line, indent, out, state)
}

// markdownInline 扫描行内元素：行内代码、链接、加粗、强调。
func markdownInline(line []rune, i int, out []Token, state State) ([]Token, State) {
	for i < len(line) {
		switch {
		case line[i] == '`':
			// 行内代码：找到同数量的收尾反引号。
			j := i + 1
			for j < len(line) && line[j] != '`' {
				j++
			}
			if j < len(line) {
				out = emit(out, KindString, i, j+1)
				i = j + 1
				continue
			}
			out = emit(out, KindString, i, len(line))
			i = len(line)

		case line[i] == '!' || line[i] == '[':
			// 链接或图片：! 与 [ 用关键字色，] 之后的 (target) 用另一种。
			open := line[i] == '!'
			bracket := i
			if open {
				if i+1 < len(line) && line[i+1] == '[' {
					out = emit(out, KindKeyword, i, i+2)
					i += 2
					continue
				}
				bracket = i + 1
			}
			depth := 0
			j := bracket
			for ; j < len(line); j++ {
				if line[j] == '[' {
					depth++
				} else if line[j] == ']' {
					depth--
					if depth == 0 {
						break
					}
				}
			}
			if j >= len(line) {
				out = emit(out, KindText, i, len(line))
				return out, state
			}
			out = emit(out, KindKeyword, i, j+1)
			i = j + 1
			if i < len(line) && line[i] == '(' {
				k := indexRuneFrom(line, ')', i)
				if k < 0 {
					out = emit(out, KindField, i, len(line))
					return out, state
				}
				out = emit(out, KindField, i, k+1)
				i = k + 1
			}

		case startsAt(line, i, "**") || startsAt(line, i, "__"):
			// 加粗：成对出现。
			mark := line[i : i+2]
			if k := indexRunes(line, string(mark), i+2); k >= 0 {
				out = emit(out, KindBuiltin, i, k+2)
				i = k + 2
				continue
			}
			out = emit(out, KindBuiltin, i, i+2)
			i += 2

		case line[i] == '*' || line[i] == '_':
			if k := indexRuneFrom(line, line[i], i+1); k > 0 {
				out = emit(out, KindBuiltin, i, k+1)
				i = k + 1
				continue
			}
			out = emit(out, KindBuiltin, i, i+1)
			i++

		case line[i] == '<':
			// 自动链接 <https://...> 与行内 HTML。
			if k := indexRuneFrom(line, '>', i); k > 0 {
				out = emit(out, KindField, i, k+1)
				i = k + 1
				continue
			}
			out = emit(out, KindText, i, i+1)
			i++

		case line[i] == '&':
			if k := indexRuneFrom(line, ';', i); k > 0 && k-i <= 10 {
				out = emit(out, KindConstant, i, k+1)
				i = k + 1
				continue
			}
			out = emit(out, KindText, i, i+1)
			i++

		default:
			i++
		}
	}
	return out, state
}

// startsAt 报告 line 在 i 处是否以 sub 开头。
func startsAt(line []rune, i int, sub string) bool {
	return startsWithAt(line, i, sub)
}

// isFence 报告本行是否是围栏代码块的开启行。
func isFence(line []rune) bool {
	trimmed := trimLeftSpace(line)
	if len(trimmed) < 3 {
		return false
	}
	c := trimmed[0]
	if c != '`' && c != '~' {
		return false
	}
	n := 0
	for n < len(trimmed) && trimmed[n] == c {
		n++
	}
	if n < 3 {
		return false
	}
	// 反引号围栏后面跟信息串时，信息串里不能再有反引号——
	// CommonMark 就是靠这条区分「开围栏」与「闭围栏」。
	if c == '`' {
		for _, r := range trimmed[n:] {
			if r == '`' {
				return false
			}
		}
	}
	return true
}

// isFenceClose 报告本行是否能关闭围栏代码块。
func isFenceClose(line []rune) bool {
	trimmed := trimRight(trimLeftSpace(line))
	if len(trimmed) < 3 {
		return false
	}
	c := trimmed[0]
	if c != '`' && c != '~' {
		return false
	}
	for _, r := range trimmed {
		if r != rune(c) {
			return false
		}
	}
	return true
}

// trimRight 去掉行尾空白。
func trimRight(s string) string {
	end := len(s)
	for end > 0 && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\r') {
		end--
	}
	return s[:end]
}

// ---- HTML / XML ----

// htmlLexer 是 HTML 与 XML 的规则。
//
// HTML 的行内标签可以跨行（<a\nhref="x">），但按行切分时不必追求
// 完整还原：标签本身通常不跨行，跨行的只当作普通文本继续即可，
// 这样规则保持简单，出错时也只是少几处上色而不会错乱。
type htmlLexer struct{}

func (htmlLexer) Name() string { return "HTML" }
func (htmlLexer) Extensions() []string {
	return []string{".html", ".htm", ".xhtml", ".xml", ".svg", ".vue", ".svelte"}
}
func (htmlLexer) Filenames() []string { return nil }
func (htmlLexer) LineComment() string { return "" }
func (htmlLexer) BlockComment() (string, string, bool) {
	return "<!--", "-->", true
}

func (htmlLexer) LexLine(line []rune, state State) ([]Token, State) {
	var out []Token
	i := 0

	if state.Has(stateInBlockComment) {
		end := indexRunes(line, "-->", 0)
		if end < 0 {
			return emit(out, KindComment, 0, len(line)), state
		}
		out = emit(out, KindComment, 0, end+3)
		i = end + 3
		state = state.Without(stateInBlockComment)
	}

	for i < len(line) {
		if startsWithAt(line, i, "<!--") {
			end := indexRunes(line, "-->", i+4)
			if end < 0 {
				return emit(out, KindComment, i, len(line)), state.With(stateInBlockComment)
			}
			stop := end + 3
			out = emit(out, KindComment, i, stop)
			i = stop
			continue
		}
		if line[i] == '<' {
			end := indexRuneFrom(line, '>', i)
			if end < 0 {
				// 标签未闭合：本行剩余按标签名着色。
				out = emit(out, KindInvalid, i, len(line))
				return out, state
			}
			out = append(out, emitOffset(htmlTagTokens(line[i:end+1]), i)...)
			i = end + 1
			continue
		}
		i++
	}
	return out, state
}

// htmlTagTokens 把一个标签拆成 token（列下标相对标签起点）。
func htmlTagTokens(tag []rune) []Token {
	var out []Token
	if len(tag) == 0 {
		return out
	}
	i := 0
	if tag[0] == '<' {
		i = 1
	}
	if i < len(tag) && tag[i] == '/' {
		i++
	}
	nameStart := i
	for i < len(tag) && (isIdentRune(tag[i]) || tag[i] == '-' || tag[i] == ':') {
		i++
	}
	if i > nameStart {
		out = emit(out, KindType, nameStart, i)
	}

	for i < len(tag) {
		switch {
		case tag[i] == '"' || tag[i] == '\'':
			quote := tag[i]
			j := i + 1
			for j < len(tag) && tag[j] != quote {
				j++
			}
			if j < len(tag) {
				j++
			}
			out = emit(out, KindString, i, j)
			i = j
		case isIdentRune(tag[i]):
			j := scanIdent(tag, i)
			out = emit(out, KindField, i, j)
			i = j
		case tag[i] == '=':
			out = emit(out, KindOperator, i, i+1)
			i++
		default:
			out = emit(out, KindPunct, i, i+1)
			i++
		}
	}
	return out
}

// emitOffset 把一组 token 平移到 base 列上。
func emitOffset(tokens []Token, base int) []Token {
	out := make([]Token, 0, len(tokens))
	for _, t := range tokens {
		t.Start += base
		t.End += base
		out = append(out, t)
	}
	return out
}
