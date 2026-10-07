// scripting.go — Python、Shell、Lua、Perl 的高亮规则。
// SPDX-License-Identifier: MIT

package syntax

func init() {
	Register(pythonLexer{})
	Register(shellLexer{})
	Register(luaLexer{})
	Register(perlLexer{})
}

// ---- Python ----

// pythonLexer 是 Python 的规则。
//
// 关键差异是三引号字符串能跨行，且内容可以是任意文字，
// 所以它必须靠状态跨行延续，而不是按行独立判断。
type pythonLexer struct{}

func (pythonLexer) Name() string         { return "Python" }
func (pythonLexer) Extensions() []string { return []string{".py", ".pyi", ".pyw"} }
func (pythonLexer) Filenames() []string  { return nil }
func (pythonLexer) LineComment() string  { return "#" }
func (pythonLexer) BlockComment() (string, string, bool) {
	return `"""`, `"""`, true
}

// pythonKeywords 是 Python 3 的关键字。
var pythonKeywords = newSet(
	"and", "as", "assert", "async", "await", "break", "class", "continue",
	"def", "del", "elif", "else", "except", "finally", "for", "from",
	"global", "if", "import", "in", "is", "lambda", "nonlocal", "not", "or",
	"pass", "raise", "return", "try", "while", "with", "yield", "match",
	"case",
)

// pythonBuiltins 是内建函数与常量。
var pythonBuiltins = newSet(
	"abs", "all", "any", "bool", "bytes", "callable", "dict", "dir",
	"enumerate", "filter", "float", "format", "frozenset", "getattr",
	"hasattr", "hash", "id", "input", "int", "isinstance", "issubclass",
	"iter", "len", "list", "map", "max", "min", "next", "object", "open",
	"ord", "pow", "print", "range", "repr", "reversed", "round", "set",
	"setattr", "sorted", "str", "sum", "super", "tuple", "type", "zip",
	"True", "False", "None", "Exception", "ValueError", "TypeError",
	"KeyError", "IndexError", "RuntimeError", "NotImplementedError",
)

func (pythonLexer) LexLine(line []rune, state State) ([]Token, State) {
	var out []Token
	i := 0

	if state.Has(stateInRawString) {
		end := findTripleClose(line, 0)
		if end < 0 {
			return emit(out, KindString, 0, len(line)), state
		}
		out = emit(out, KindString, 0, end+3)
		i = end + 3
		state = state.Without(stateInRawString)
	}

	for i < len(line) {
		r := line[i]
		switch {
		case isBlank(r):
			i++

		case r == '#':
			return emit(out, KindComment, i, len(line)), state
		case startsTriple(line, i):
			// 找同一行的收尾；找不到就说明这个三引号字符串跨行了。
			if end := findTripleClose(line, i+3); end >= 0 {
				out = emit(out, KindString, i, end+3)
				i = end + 3
			} else {
				out = emit(out, KindString, i, len(line))
				return out, state.With(stateInRawString)
			}
		case r == '"' || r == '\'':
			out = emit(out, KindString, i, i+1)
			j := i + 1
			for j < len(line) {
				if line[j] == '\\' {
					out = emit(out, KindEscape, j, minInt(j+2, len(line)))
					j += 2
					continue
				}
				if line[j] == r {
					out = emit(out, KindString, j, j+1)
					break
				}
				j++
			}
			if j >= len(line) {
				out = emit(out, KindString, j, len(line))
			}
			i = j + 1
		case r == '@' && i+1 < len(line) && (isLetter(line[i+1]) || line[i+1] == '_'):
			// 装饰器 @property：整体一种颜色，不按标识符再分类。
			j := i + 1
			for j < len(line) && (isIdentRune(line[j]) || line[j] == '.') {
				j++
			}
			out = emit(out, KindBuiltin, i, j)
			i = j
		case isDigit(r):
			j := scanNumber(line, i)
			out = emit(out, KindNumber, i, j)
			i = j
		case isIdentRune(r):
			j := scanIdent(line, i)
			word := string(line[i:j])
			kind := KindText
			switch {
			case pythonKeywords[word]:
				kind = KindKeyword
			case pythonBuiltins[word]:
				kind = KindBuiltin
			case isUpper(rune(word[0])):
				kind = KindType
			case j < len(line) && line[j] == '(':
				kind = KindFunction
			}
			out = emit(out, kind, i, j)
			i = j
		default:
			if isOperatorRune(r) || r == ':' {
				j := scanOperator(line, i)
				out = emit(out, KindOperator, i, j)
				i = j
				continue
			}
			out = emit(out, KindPunct, i, i+1)
			i++
		}
	}
	return out, state
}

// startsTriple 报告 i 处是否是三引号的开头。
func startsTriple(line []rune, i int) bool {
	if i+3 > len(line) {
		return false
	}
	return line[i] == line[i+1] && line[i+1] == line[i+2] &&
		(line[i] == '"' || line[i] == '\'')
}

// findTripleClose 从 i 起找三引号的收尾位置，找不到返回 -1。
func findTripleClose(line []rune, i int) int {
	for j := i; j+3 <= len(line); j++ {
		if startsTriple(line, j) {
			return j
		}
	}
	return -1
}

// ---- Shell ----

// shellLexer 是 POSIX shell / bash 的规则。
//
// 最需要注意的是引号可以嵌套在另一种引号里：
// echo "it's fine" 里的单引号不是字符串定界符。
// 所以扫描时要跟踪当前处在哪种引号里。
type shellLexer struct{}

func (shellLexer) Name() string         { return "Shell" }
func (shellLexer) Extensions() []string { return []string{".sh", ".bash", ".zsh", ".ksh"} }
func (shellLexer) Filenames() []string {
	return []string{
		".bashrc", ".bash_profile", ".zshrc", ".profile",
		".bash_history", ".zprofile", ".zshenv",
	}
}
func (shellLexer) LineComment() string { return "#" }
func (shellLexer) BlockComment() (string, string, bool) {
	return "", "", false
}

var shellKeywords = newSet(
	"if", "then", "elif", "else", "fi", "for", "while", "until", "do",
	"done", "case", "esac", "in", "function", "select", "time", "return",
	"break", "continue", "local", "export", "readonly", "declare",
	"typeset", "unset", "shift", "trap", "eval", "exec", "source", "alias",
	"set", "unset", "source",
)

var shellBuiltins = newSet(
	"echo", "printf", "read", "cd", "pwd", "test", "true", "false", "exit",
	"kill", "wait", "jobs", "fg", "bg", "help", "type", "command",
	"getopts", "hash", "umask", "ulimit", "sudo", "apt", "apt-get",
	"brew", "curl", "wget", "git", "go", "make", "ls", "cat", "grep",
	"sed", "awk", "find", "chmod", "chown", "mkdir", "rm", "cp", "mv",
	"docker", "systemctl", "service",
)

func (shellLexer) LexLine(line []rune, state State) ([]Token, State) {
	var out []Token
	i := 0

	// #! 开头是指向解释器的指示，单列一种颜色便于一眼看清文件怎么跑。
	if startsWithAt(line, 0, "#!") {
		return emit(out, KindPreproc, 0, len(line)), state
	}

	// 顶格开头的 # 是注释；引号内的 # 是内容。
	for i < len(line) {
		r := line[i]
		switch {
		case isBlank(r):
			i++

		case r == '#':
			return emit(out, KindComment, i, len(line)), state
		case r == '\'':
			// 单引号内一切都是字面量，连转义都不认。
			j := i + 1
			for j < len(line) && line[j] != '\'' {
				j++
			}
			out = emit(out, KindString, i, minInt(j+1, len(line)))
			i = j + 1
		case r == '"':
			i = scanDoubleQuotedShell(line, i, &out)
		case r == '$':
			j := scanShellVar(line, i)
			out = emit(out, KindConstant, i, j)
			i = j
		case isDigit(r):
			j := scanNumber(line, i)
			out = emit(out, KindNumber, i, j)
			i = j
		case r == '-' || r == '~':
			// my-var 里的连字符属于变量名，只有独立成词的 -v、--verbose、
			// ~/x 才是选项或路径。判据是前面是不是空白或行首。
			//
			// 这里不能用 break 跳出再往下走：break 只跳出 switch，
			// i 没有前进，整个循环就卡死了。必须每条路径都推进 i。
			if i > 0 && !isBlank(line[i-1]) {
				out = emit(out, KindText, i, i+1)
				i++
				continue
			}
			j := i + 1
			for j < len(line) && (line[j] == '-' || line[j] == '~' || isIdentRune(line[j])) {
				j++
			}
			out = emit(out, KindOperator, i, j)
			i = j
		case isIdentRune(r):
			j := scanIdent(line, i)
			word := string(line[i:j])
			kind := KindText
			switch {
			case shellKeywords[word]:
				kind = KindKeyword
			case shellBuiltins[word]:
				kind = KindBuiltin
			case j < len(line) && line[j] == '=':
				kind = KindField
			}
			out = emit(out, kind, i, j)
			i = j
		default:
			if isOperatorRune(r) {
				j := scanOperator(line, i)
				out = emit(out, KindOperator, i, j)
				i = j
				continue
			}
			out = emit(out, KindPunct, i, i+1)
			i++
		}
	}
	return out, state
}

// scanDoubleQuotedShell 扫描双引号串，内部单引号不结束串。
func scanDoubleQuotedShell(line []rune, i int, out *[]Token) int {
	*out = emit(*out, KindString, i, i+1)
	j := i + 1
	for j < len(line) {
		switch line[j] {
		case '\\':
			*out = emit(*out, KindEscape, j, minInt(j+2, len(line)))
			j += 2
		case '"':
			*out = emit(*out, KindString, j, j+1)
			return j + 1
		case '$':
			k := scanShellVar(line, j)
			*out = emit(*out, KindConstant, j, k)
			j = k
		default:
			j++
		}
	}
	*out = emit(*out, KindString, j, len(line))
	return len(line)
}

// scanShellVar 扫描 $VAR、${VAR}、$1、$(cmd)、$? 等变量与展开。
func scanShellVar(line []rune, i int) int {
	if i+1 >= len(line) {
		return i + 1
	}
	switch next := line[i+1]; {
	case next == '{':
		j := indexRuneFrom(line, '}', i+2)
		if j < 0 {
			return len(line)
		}
		return j + 1
	case next == '(':
		// $(...) 里可以再嵌套 $，配括号即可。
		depth := 0
		for j := i + 1; j < len(line); j++ {
			switch line[j] {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					return j + 1
				}
			}
		}
		return len(line)
	case next == '_' || isLetter(next):
		return scanIdent(line, i+1)
	case isDigit(next):
		j := i + 1
		for j < len(line) && (isDigit(line[j]) || isOperatorRune(line[j])) {
			j++
		}
		return j
	}
	// $* $@ $# $? $! $$ $0 等单字符变量。
	return i + 2
}

// ---- Lua ----

// luaLexer 是 Lua 的规则。
type luaLexer struct{}

func (luaLexer) Name() string         { return "Lua" }
func (luaLexer) Extensions() []string { return []string{".lua"} }
func (luaLexer) Filenames() []string  { return nil }
func (luaLexer) LineComment() string  { return "--" }
func (luaLexer) BlockComment() (string, string, bool) {
	return "--[[", "]]", true
}

var luaTables = wordTables{
	keywords: newSet(
		"and", "break", "do", "else", "elseif", "end", "for", "function",
		"goto", "if", "in", "local", "not", "or", "repeat", "return",
		"then", "until", "while",
	),
	types:    newSet(),
	builtins: newSet("print", "pairs", "ipairs", "type", "tostring", "tonumber", "require", "pcall", "error", "assert", "select", "setmetatable", "getmetatable", "rawget", "rawset"),
	literals: newSet("true", "false", "nil", "self"),
}

// luaLongBracket 匹配 Lua 的长括号字符串 [[...]]、[=[...]=]、[==[...]==]。
//
// 返回值：end 是收尾符之后的下标，收尾符不在本行时为 -1；
// lvl 是 '=' 的个数，决定用几个来配对；ok 表示这里确实是长括号串。
func luaLongBracket(line []rune, i int) (end int, lvl int, ok bool) {
	if i >= len(line) || line[i] != '[' {
		return 0, 0, false
	}
	j := i + 1
	for j < len(line) && line[j] == '=' {
		j++
	}
	if j >= len(line) || line[j] != '[' {
		return 0, 0, false
	}
	level := j - i - 1
	closer := "]" + stringsRepeat("=", level) + "]"
	for k := j + 1; k < len(line); k++ {
		if startsWithAt(line, k, closer) {
			return k + len([]rune(closer)), level, true
		}
	}
	// 收尾不在本行：串要跨到下一行，靠状态带出去。
	return -1, level, true
}

// stringsRepeat 复制字符串。
func stringsRepeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}

func (luaLexer) LexLine(line []rune, state State) ([]Token, State) {
	var out []Token
	i := 0

	if state.Has(stateInLongBracket) {
		lvl := int(state >> stateLevelShift)
		closer := "]" + stringsRepeat("=", lvl) + "]"
		k := indexRunes(line, closer, 0)
		if k < 0 {
			return emit(out, KindString, 0, len(line)), state
		}
		i = k + len([]rune(closer))
		out = emit(out, KindString, 0, i)
		// 收尾后要把标志位和层级一起清掉，否则后面的每一行都会被
		// 当成还在长字符串里。
		state = state.Without(stateInLongBracket).Without(State(lvl << stateLevelShift))
	}

	for i < len(line) {
		r := line[i]
		switch {
		case isBlank(r):
			i++
		case startsWithAt(line, i, "--[["):
			tail := indexRunes(line, "]]", i+4)
			if tail < 0 {
				return emit(out, KindComment, i, len(line)), state
			}
			end := tail + 2
			out = emit(out, KindComment, i, end)
			i = end
		case startsWithAt(line, i, "--"):
			return emit(out, KindComment, i, len(line)), state
		case r == '[':
			end, lvl, isBracket := luaLongBracket(line, i)
			if !isBracket {
				out = emit(out, KindPunct, i, i+1)
				i++
				continue
			}
			if end < 0 {
				out = emit(out, KindString, i, len(line))
				return out, state.With(stateInLongBracket).With(State(lvl << stateLevelShift))
			}
			out = emit(out, KindString, i, end)
			i = end
		case r == '"' || r == '\'':
			i = scanQuoted(line, i, r, &out)
		case isDigit(r):
			j := scanNumber(line, i)
			out = emit(out, KindNumber, i, j)
			i = j
		case isIdentRune(r):
			j := scanIdent(line, i)
			out = emit(out, lexIdentAt(line, i, j, luaTables), i, j)
			i = j
		default:
			if isOperatorRune(r) {
				j := scanOperator(line, i)
				out = emit(out, KindOperator, i, j)
				i = j
				continue
			}
			out = emit(out, KindPunct, i, i+1)
			i++
		}
	}
	return out, state
}

// stateInLongBracket 表示处于 Lua 长括号字符串 [[ ]] / [=[ ]=] 内部。
const stateInLongBracket State = 1 << 15

// stateLevelShift 把 Lua 长括号的层级（= 的个数）编码进 State 的高位。
// 低位留给布尔标志，层级用移位存，两者互不干扰。
const stateLevelShift = 16

// ---- Perl ----

// perlLexer 是 Perl 的规则。
type perlLexer struct{}

func (perlLexer) Name() string         { return "Perl" }
func (perlLexer) Extensions() []string { return []string{".pl", ".pm", ".t"} }
func (perlLexer) Filenames() []string  { return nil }
func (perlLexer) LineComment() string  { return "#" }
func (perlLexer) BlockComment() (string, string, bool) {
	return "", "", false
}

var perlTables = wordTables{
	keywords: newSet(
		"and", "cmp", "continue", "do", "else", "elsif", "eq", "exp",
		"for", "foreach", "ge", "gt", "if", "last", "le", "local", "lt",
		"my", "ne", "next", "not", "or", "our", "package", "redo", "ref",
		"require", "return", "sub", "unless", "until", "use", "wantarray",
		"while", "x", "given", "when", "default", "say",
	),
	types:    newSet("scalar", "keys", "values"),
	builtins: newSet("print", "printf", "sprintf", "push", "pop", "shift", "unshift", "splice", "join", "split", "map", "grep", "sort", "die", "warn", "open", "close", "bless", "defined", "exists", "delete"),
	literals: newSet("undef", "__PACKAGE__", "__END__", "__DATA__"),
}

func (perlLexer) LexLine(line []rune, state State) ([]Token, State) {
	var out []Token
	i := 0

	if state.Has(stateInRawString) {
		// POD 段落以 = 开头、以 =cut 结束，属于块注释语义。
		trimmed := trimLeftSpace(line)
		if trimmed == "=cut" {
			out = emit(out, KindComment, 0, indexOfNonSpace(line, 4))
			state = state.Without(stateInRawString)
			i = indexOfNonSpace(line, 4)
		} else {
			return emit(out, KindComment, 0, len(line)), state
		}
	}

	for i < len(line) {
		r := line[i]
		switch {
		case isBlank(r):
			i++
		case state.Has(stateInRawString):
			// POD 内部：整段按注释着色，直到 =cut。
			return emit(out, KindComment, i, len(line)), state
		case r == '#':
			return emit(out, KindComment, i, len(line)), state
		case r == '=' && i == indexOfNonSpace(line, 0) && startsPODBlock(line, i):
			out = emit(out, KindComment, i, len(line))
			return out, state.With(stateInRawString)
		case r == '$' || r == '@' || r == '%':
			// sigil 加变量名：$foo、@arr、%hash、$&、$1。
			j := i + 1
			if j < len(line) && line[j] == '{' {
				k := indexRuneFrom(line, '}', j)
				if k >= 0 {
					out = emit(out, KindConstant, i, k+1)
					i = k + 1
					continue
				}
			}
			if j < len(line) && isIdentRune(line[j]) {
				k := scanIdent(line, j)
				out = emit(out, KindConstant, i, k)
				i = k
				continue
			}
			out = emit(out, KindConstant, i, i+1)
			i++
		case r == '"' || r == '\'' || r == '`':
			i = scanQuoted(line, i, r, &out)
		case isDigit(r):
			j := scanNumber(line, i)
			out = emit(out, KindNumber, i, j)
			i = j
		case isIdentRune(r):
			j := scanIdent(line, i)
			out = emit(out, lexIdentAt(line, i, j, perlTables), i, j)
			i = j
		default:
			if isOperatorRune(r) {
				j := scanOperator(line, i)
				out = emit(out, KindOperator, i, j)
				i = j
				continue
			}
			out = emit(out, KindPunct, i, i+1)
			i++
		}
	}
	return out, state
}

// startsPODBlock 报告顶格的 =xxx 是否是 POD 块起始。
//
// POD 规则：行首（允许空白）的 = 后面必须是字母或标点，
// 且不能有空格。
func startsPODBlock(line []rune, i int) bool {
	if i+1 >= len(line) {
		return false
	}
	next := line[i+1]
	return isLetter(next) || next == '~' || next == '@'
}

// trimLeftSpace 去掉行首空白。
func trimLeftSpace(line []rune) string {
	j := indexOfNonSpace(line, 0)
	return string(line[j:])
}

// indexOfNonSpace 返回从 i 起第一个非空白字符的下标。
func indexOfNonSpace(line []rune, i int) int {
	for j := i; j < len(line); j++ {
		if line[j] != ' ' && line[j] != '\t' {
			return j
		}
	}
	return len(line)
}
