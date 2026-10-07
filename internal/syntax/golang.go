// golang.go — Go 语言高亮规则。
// SPDX-License-Identifier: MIT

package syntax

// goLexer 是 Go 的高亮规则。
//
// Go 特意做得比通用规则细，因为编辑器是围绕 Go 优化的：
//   - 反引号原始字符串不能跨行（未闭合就是语法错误），状态处理与
//     普通字符串不同；
//   - 首字母大写的标识符是导出的（类型、函数、变量），可据此上色；
//   - 点号后面是字段或方法，颜色与普通标识符区分开。
type goLexer struct{}

func init() { Register(Go()) }

// Go 返回 Go 的高亮规则。
func Go() Lexer { return goLexer{} }

func (goLexer) Name() string { return "Go" }

func (goLexer) Extensions() []string { return []string{".go"} }

func (goLexer) Filenames() []string { return nil }

func (goLexer) LineComment() string { return "//" }

func (goLexer) BlockComment() (string, string, bool) { return "/*", "*/", true }

// goKeywords 是 Go 的全部关键字。
var goKeywords = newSet(
	"break", "case", "chan", "const", "continue", "default", "defer", "else",
	"fallthrough", "for", "func", "goto", "if", "import", "interface",
	"map", "package", "range", "return", "select", "struct", "switch",
	"type", "var",
)

// goTypes 是内建类型名。
var goTypes = newSet(
	"bool", "byte", "complex64", "complex128", "error", "float32", "float64",
	"int", "int8", "int16", "int32", "int64", "rune", "string",
	"uint", "uint8", "uint16", "uint32", "uint64", "uintptr", "any",
)

// goBuiltins 是内建函数与常量。
var goBuiltins = newSet(
	"append", "cap", "clear", "close", "complex", "copy", "delete", "imag",
	"len", "make", "max", "min", "new", "panic", "print", "println",
	"real", "recover",
)

// goLiterals 是语言层面的字面量。
var goLiterals = newSet("true", "false", "iota", "nil")

func (goLexer) LexLine(line []rune, state State) ([]Token, State) {
	var out []Token

	// 上一行结束时还停在块注释里：先吃掉本行开头的注释部分。
	if state.Has(stateInBlockComment) {
		end := indexRunes(line, "*/", 0)
		if end < 0 {
			return emit(out, KindComment, 0, len(line)), state
		}
		out = emit(out, KindComment, 0, end+2)
		return goLexFrom(line, end+2, state.Without(stateInBlockComment), out)
	}

	return goLexFrom(line, 0, state, out)
}

// goLexFrom 从下标 start 开始扫描一行。
func goLexFrom(line []rune, start int, state State, out []Token) ([]Token, State) {
	i := start
	for i < len(line) {
		r := line[i]

		switch {
		case isBlank(r):
			i++

		case r == '/' && i+1 < len(line) && line[i+1] == '/':
			return emit(out, KindComment, i, len(line)), state

		case r == '/' && i+1 < len(line) && line[i+1] == '*':
			end := indexRunes(line, "*/", i+2)
			if end < 0 {
				// 块注释跨到下一行：把状态带出去。
				return emit(out, KindComment, i, len(line)), state.With(stateInBlockComment)
			}
			out = emit(out, KindComment, i, end+2)
			i = end + 2

		case r == '"':
			i = scanQuoted(line, i, '"', &out)

		case r == '\'':
			// Go 里单引号只可能是字符字面量，没有别的用法。
			if end, ok := scanCharLiteral(line, i, &out); ok {
				i = end
				continue
			}
			out = emit(out, KindText, i, i+1)
			i++

		case r == '`':
			// 反引号原始字符串不能跨行：Go 里未闭合是编译错误。
			// 若当成「继续到下一行」，整个文件后面都会被刷成字符串颜色。
			end := indexRuneFrom(line, '`', i+1)
			if end < 0 {
				out = emit(out, KindInvalid, i, len(line))
				return out, state.Without(stateInRawString)
			}
			out = emit(out, KindString, i, end+1)
			i = end + 1

		case isDigit(r):
			j := scanNumber(line, i)
			out = emit(out, KindNumber, i, j)
			i = j

		case isIdentRune(r):
			j := scanIdent(line, i)
			out = emit(out, goIdentKind(line, i, j, string(line[i:j])), i, j)
			i = j

		case isOperatorRune(r):
			j := scanOperator(line, i)
			out = emit(out, KindOperator, i, j)
			i = j

		default:
			out = emit(out, KindPunct, i, i+1)
			i++
		}
	}
	return out, state
}

// goIdentKind 判定标识符的种类。
func goIdentKind(line []rune, start, end int, word string) Kind {
	// 点号后面是字段或方法，与普通标识符区分开。
	if start > 0 && line[start-1] == '.' {
		return KindField
	}
	switch {
	case goKeywords[word]:
		return KindKeyword
	case goTypes[word]:
		return KindKeywordType
	case goBuiltins[word]:
		return KindBuiltin
	case goLiterals[word]:
		return KindConstant
	}
	// 首字母大写是导出的，在 Go 里基本都是类型或常量。
	if isUpper(rune(word[0])) {
		return KindType
	}
	// 紧跟左括号的标识符是函数调用或函数定义。
	if end < len(line) && line[end] == '(' {
		return KindFunction
	}
	return KindText
}
