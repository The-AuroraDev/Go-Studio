// clike.go — C 家族语言的高亮规则（参数化，一份实现服务多门语言）。
// SPDX-License-Identifier: MIT

package syntax

// clikeLexer 覆盖 C 家族：C、C++、Java、Kotlin、Swift、Rust、JavaScript、
// TypeScript、Go 之外的大部分主流语言。
//
// 它们共享同一套结构——行注释、块注释、引号字符串、数字、
// 关键字表——差异只在关键词表与少量语法糖。
// 拆成一份参数化实现而不是一门语言一个文件，是因为规则几乎相同，
// 分开写会产生几十个只差几个词的文件，改一处漏九处。
type clikeLexer struct {
	name        string
	extensions  []string
	tables      wordTables
	lineComment string
	// blockPair 是块注释定界符；ok 为 false 时该语言没有块注释。
	blockOpen, blockClose string
	blockOK               bool
	// rawQuotes 是「原样字符串」的定界符，如 Rust 的 r#"..."#。
	// 为 0 表示不支持。
	rawQuote rune
	// preproc 为真时支持 # 开头的预处理指令（C/C++）。
	preproc bool
	// extra 是该语言特有的识别钩子，返回 false 表示「我不管这一段」。
	extra func(l *clikeLexer, line []rune, i int, out *[]Token) (next int, handled bool)
}

// clike 返回一门 C 家族语言的规则。
func clike(l clikeLexer) Lexer { return l }

// cTables 是 C 与 C++ 的表。
var cTables = wordTables{
	keywords: newSet(
		"auto", "break", "case", "const", "continue", "default", "do", "else",
		"extern", "for", "goto", "if", "inline", "register", "restrict",
		"return", "sizeof", "static", "struct", "switch", "typedef",
		"union", "volatile", "while",
	),
	types: newSet(
		"bool", "char", "double", "float", "int", "long", "short",
		"signed", "size_t", "ssize_t", "unsigned", "void", "wchar_t",
		"int8_t", "int16_t", "int32_t", "int64_t", "uint8_t", "uint16_t",
		"uint32_t", "uint64_t", "FILE",
	),
	builtins: newSet(
		"abort", "assert", "calloc", "exit", "fclose", "fopen", "fprintf",
		"free", "malloc", "memcpy", "memset", "printf", "realloc", "scanf",
		"snprintf", "strcmp", "strcpy", "strlen", "strncmp",
	),
	literals: newSet("NULL", "true", "false"),
}

// cppTables 是 C++ 的表。
var cppTables = wordTables{
	keywords: newSet(
		"alignas", "alignof", "and", "asm", "auto", "bool", "break", "case",
		"catch", "char", "class", "concept", "const", "consteval", "constexpr",
		"constinit", "continue", "co_await", "co_return", "co_yield", "decltype",
		"default", "delete", "do", "double", "dynamic_cast", "else", "enum",
		"explicit", "export", "extern", "false", "float", "for", "friend", "goto",
		"if", "inline", "int", "long", "mutable", "namespace", "new",
		"noexcept", "operator", "private", "protected", "public", "register",
		"reinterpret_cast", "requires", "return", "short", "signed", "sizeof",
		"static", "static_assert", "static_cast", "struct", "switch", "template",
		"this", "thread_local", "throw", "true", "try", "typedef", "typeid",
		"typename", "union", "unsigned", "using", "virtual", "void", "volatile",
		"while",
	),
	types:    newSet("char8_t", "char16_t", "char32_t", "wchar_t", "size_t"),
	builtins: newSet("std"),
	literals: newSet("nullptr"),
}

// javaTables 是 Java 的表。
var javaTables = wordTables{
	keywords: newSet(
		"abstract", "assert", "break", "case", "catch", "class", "const",
		"continue", "default", "do", "else", "enum", "extends", "final",
		"finally", "for", "goto", "if", "implements", "import", "instanceof",
		"interface", "native", "new", "package", "private", "protected",
		"public", "record", "return", "sealed", "static", "strictfp",
		"super", "switch", "synchronized", "this", "throw", "throws",
		"transient", "try", "var", "volatile", "while", "yield",
	),
	types: newSet(
		"boolean", "byte", "char", "double", "float", "int", "long", "short",
		"void", "String", "Object", "Integer", "List", "Map",
	),
	builtins: newSet(
		"System", "Math", "Arrays", "Collections", "Objects", "Optional",
	),
	literals: newSet("true", "false", "null"),
}

// rustTables 是 Rust 的表。
var rustTables = wordTables{
	keywords: newSet(
		"as", "async", "await", "break", "const", "continue", "crate", "dyn",
		"else", "enum", "extern", "fn", "for", "if", "impl", "in", "let",
		"loop", "match", "mod", "move", "mut", "pub", "ref", "return", "self",
		"Self", "static", "struct", "super", "trait", "type", "unsafe",
		"use", "where", "while",
	),
	types: newSet(
		"bool", "char", "f32", "f64", "i8", "i16", "i32", "i64", "i128",
		"isize", "str", "u8", "u16", "u32", "u64", "u128", "usize", "String",
		"Vec", "Option", "Result", "Box",
	),
	builtins: newSet(
		"println", "print", "eprintln", "eprint", "format", "vec", "panic",
		"assert", "assert_eq", "Some", "None", "Ok", "Err",
	),
	literals: newSet("true", "false"),
}

// jsTables 是 JavaScript 与 TypeScript 的表。
var jsTables = wordTables{
	keywords: newSet(
		"async", "await", "break", "case", "catch", "class", "const",
		"continue", "debugger", "default", "delete", "do", "else", "export",
		"extends", "finally", "for", "function", "get", "if", "import", "in",
		"instanceof", "let", "new", "of", "return", "set", "static", "super",
		"switch", "this", "throw", "try", "typeof", "var", "void", "while",
		"with", "yield",
	),
	types: newSet(
		"any", "bigint", "boolean", "never", "number", "object", "string",
		"symbol", "unknown", "void",
	),
	builtins: newSet(
		"console", "Array", "Object", "String", "Number", "Boolean", "Math",
		"JSON", "Promise", "Map", "Set", "Symbol", "RegExp", "Error",
	),
	literals: newSet("true", "false", "null", "undefined", "NaN"),
}

// tsTables 额外加上 TypeScript 独有的类型关键字。
var tsTypes = func() map[string]bool {
	m := newSet(
		"any", "bigint", "boolean", "never", "number", "object", "string",
		"symbol", "unknown", "void",
	)
	for _, w := range []string{
		"bigint", "declare", "enum", "implements", "infer", "interface",
		"is", "keyof", "namespace", "private", "protected", "public", "readonly",
		"type", "unique", "satisfies",
	} {
		m[w] = true
	}
	return m
}()

// swiftTables 是 Swift 的表。
var swiftTables = wordTables{
	keywords: newSet(
		"associatedtype", "class", "deinit", "enum", "extension", "fileprivate",
		"func", "import", "init", "inout", "internal", "let", "open", "operator",
		"private", "protocol", "public", "rethrows", "static", "struct",
		"subscript", "typealias", "var", "where", "while", "guard", "defer",
		"do", "catch", "fallthrough", "for", "if", "in", "repeat", "return",
		"switch", "throw", "throws", "case", "default", "async", "await",
	),
	types: newSet(
		"Any", "Bool", "Character", "Double", "Float", "Int", "String", "UInt",
		"Void", "Array", "Dictionary", "Set", "Optional", "Result",
	),
	builtins: newSet("print", "assert", "precondition"),
	literals: newSet("true", "false", "nil"),
}

// kotlinTables 是 Kotlin 的表。
var kotlinTables = wordTables{
	keywords: newSet(
		"as", "break", "by", "catch", "class", "companion", "const",
		"constructor", "continue", "do", "else", "enum", "external", "false",
		"final", "finally", "for", "fun", "if", "import", "in", "infix",
		"init", "inline", "interface", "internal", "is", "object", "open",
		"operator", "package", "private", "protected", "public", "reified",
		"return", "sealed", "super", "suspend", "this", "throw", "try",
		"typealias", "val", "var", "vararg", "when", "where", "while",
	),
	types:    newSet("Any", "Boolean", "Byte", "Char", "Double", "Float", "Int", "Long", "String", "Unit"),
	builtins: newSet("println", "print", "listOf", "mapOf", "mutableListOf"),
	literals: newSet("true", "false", "null"),
}

// 各语言的构造。这些函数在包初始化时登记，避免用户测到一半才发现少一门语言。

func init() {
	Register(clike(clikeLexer{
		name: "C", extensions: []string{".c", ".h"},
		tables: cTables, lineComment: "//",
		blockOpen: "/*", blockClose: "*/", blockOK: true,
		rawQuote: 0, preproc: true,
	}))
	Register(clike(clikeLexer{
		name: "C++", extensions: []string{".cc", ".cpp", ".cxx", ".hpp", ".hh", ".hxx"},
		tables: cppTables, lineComment: "//",
		blockOpen: "/*", blockClose: "*/", blockOK: true,
		rawQuote: 0, preproc: true,
	}))
	Register(clike(clikeLexer{
		name: "Java", extensions: []string{".java"},
		tables: javaTables, lineComment: "//",
		blockOpen: "/*", blockClose: "*/", blockOK: true,
	}))
	Register(clike(clikeLexer{
		name: "Kotlin", extensions: []string{".kt", ".kts"},
		tables: kotlinTables, lineComment: "//",
		blockOpen: "/*", blockClose: "*/", blockOK: true,
	}))
	Register(clike(clikeLexer{
		name: "Rust", extensions: []string{".rs"},
		tables: rustTables, lineComment: "//",
		blockOpen: "/*", blockClose: "*/", blockOK: true,
		rawQuote: 'r',
	}))
	Register(clike(clikeLexer{
		name: "Swift", extensions: []string{".swift"},
		tables: swiftTables, lineComment: "//",
		blockOpen: "/*", blockClose: "*/", blockOK: true,
	}))
	Register(clike(clikeLexer{
		name: "JavaScript", extensions: []string{".js", ".jsx", ".mjs", ".cjs"},
		tables: jsTables, lineComment: "//",
		blockOpen: "/*", blockClose: "*/", blockOK: true,
	}))
	Register(clike(clikeLexer{
		name: "TypeScript", extensions: []string{".ts", ".tsx"},
		tables:      wordTables{jsTables.keywords, tsTypes, jsTables.builtins, jsTables.literals},
		lineComment: "//",
		blockOpen:   "/*", blockClose: "*/", blockOK: true,
	}))
	Register(clike(clikeLexer{
		name: "Scala", extensions: []string{".scala", ".sc"},
		tables: wordTables{
			keywords: newSet(
				"abstract", "case", "catch", "class", "def", "do", "else",
				"extends", "final", "finally", "for", "forSome", "if",
				"implicit", "import", "lazy", "match", "new", "object",
				"override", "package", "private", "protected", "return",
				"sealed", "super", "this", "throw", "trait", "try", "type",
				"val", "var", "while", "with", "yield",
			),
			types:    newSet("Any", "AnyRef", "Boolean", "Byte", "Char", "Double", "Float", "Int", "Long", "Short", "String", "Unit"),
			builtins: newSet("println", "print", "List", "Map", "Option", "Some", "None"),
			literals: newSet("true", "false", "null"),
		},
		lineComment: "//",
		blockOpen:   "/*", blockClose: "*/", blockOK: true,
	}))
	Register(clike(clikeLexer{
		name: "Dart", extensions: []string{".dart"},
		tables: wordTables{
			keywords: newSet(
				"abstract", "as", "assert", "async", "await", "break", "case",
				"catch", "class", "const", "continue", "covariant", "default",
				"deferred", "do", "else", "enum", "export", "extends",
				"extension", "external", "factory", "false", "final",
				"finally", "for", "get", "if", "implements", "import", "in",
				"is", "late", "library", "mixin", "new", "null", "on",
				"operator", "part", "required", "rethrow", "return", "set",
				"show", "static", "super", "switch", "sync", "this", "throw",
				"try", "typedef", "var", "void", "while", "with", "yield",
			),
			types:    newSet("bool", "double", "dynamic", "int", "num", "String", "List", "Map", "Set"),
			builtins: newSet("print"),
			literals: newSet("true", "false", "null"),
		},
		lineComment: "//",
		blockOpen:   "/*", blockClose: "*/", blockOK: true,
	}))
	Register(clike(clikeLexer{
		name: "Objective-C", extensions: []string{".m", ".mm"},
		tables: cTables, lineComment: "//",
		blockOpen: "/*", blockClose: "*/", blockOK: true,
		preproc: true,
	}))
	Register(clike(clikeLexer{
		name: "PHP", extensions: []string{".php"},
		tables: wordTables{
			keywords: newSet(
				"abstract", "and", "array", "as", "break", "callable", "case",
				"catch", "class", "clone", "const", "continue", "declare",
				"default", "do", "echo", "else", "elseif", "empty", "enddeclare",
				"endfor", "endforeach", "endif", "endswitch", "endwhile",
				"extends", "final", "finally", "fn", "for", "foreach",
				"function", "global", "goto", "if", "implements", "include",
				"instanceof", "insteadof", "interface", "isset", "list",
				"namespace", "new", "or", "print", "private", "protected",
				"public", "require", "return", "static", "switch", "throw",
				"trait", "try", "unset", "use", "var", "while", "xor", "yield",
			),
			types:    newSet("bool", "float", "int", "string", "void", "mixed", "iterable", "object"),
			builtins: newSet("count", "isset", "empty", "in_array", "array_map", "var_dump", "sprintf"),
			literals: newSet("true", "false", "null", "TRUE", "FALSE", "NULL"),
		},
		lineComment: "//",
		blockOpen:   "/*", blockClose: "*/", blockOK: true,
		preproc: true,
	}))
	Register(clike(clikeLexer{
		name: "Groovy", extensions: []string{".groovy", ".gradle"},
		tables: wordTables{
			keywords: newSet(
				"as", "assert", "break", "case", "catch", "class", "const",
				"continue", "def", "default", "do", "else", "enum", "extends",
				"final", "finally", "for", "goto", "if", "implements", "import",
				"in", "instanceof", "interface", "new", "package", "return",
				"static", "switch", "this", "throw", "throws", "trait", "try",
				"var", "while",
			),
			types:    newSet("boolean", "byte", "char", "double", "float", "int", "long", "short", "void"),
			builtins: newSet("println", "printf", "System"),
			literals: newSet("true", "false", "null"),
		},
		lineComment: "//",
		blockOpen:   "/*", blockClose: "*/", blockOK: true,
	}))
	Register(clike(clikeLexer{
		name: "CSS", extensions: []string{".css"},
		tables: wordTables{
			keywords: newSet(
				"@charset", "@font-face", "@import", "@keyframes", "@media",
				"@supports", "and", "from", "not", "only", "or", "to",
			),
			types:    newSet(),
			builtins: newSet(),
			literals: newSet("inherit", "initial", "unset", "none", "auto"),
		},
		lineComment: "",
		blockOK:     false,
	}))
	Register(clike(clikeLexer{
		name: "SQL", extensions: []string{".sql"},
		tables: wordTables{
			keywords: newSet(
				"ADD", "ALL", "ALTER", "AND", "AS", "ASC", "BETWEEN", "BY",
				"CREATE", "CROSS", "DELETE", "DESC", "DISTINCT", "DROP",
				"EXISTS", "FROM", "FULL", "GROUP", "HAVING", "IN", "INNER",
				"INSERT", "INTO", "IS", "JOIN", "LEFT", "LIKE", "LIMIT",
				"NOT", "NULL", "ON", "OR", "ORDER", "OUTER", "RIGHT",
				"SELECT", "SET", "TABLE", "THEN", "UNION", "UPDATE", "VALUES",
				"WHERE", "WITH",
			),
			types:    newSet("INT", "INTEGER", "BIGINT", "SMALLINT", "VARCHAR", "TEXT", "BOOLEAN", "TIMESTAMP", "DATE", "NUMERIC", "REAL", "BLOB"),
			builtins: newSet("COUNT", "SUM", "AVG", "MIN", "MAX", "COALESCE", "NOW"),
			literals: newSet("TRUE", "FALSE"),
		},
		lineComment: "--",
		blockOpen:   "/*", blockClose: "*/", blockOK: true,
	}))
}

func (l clikeLexer) Name() string { return l.name }

func (l clikeLexer) Extensions() []string { return l.extensions }

func (l clikeLexer) Filenames() []string { return nil }

func (l clikeLexer) LineComment() string { return l.lineComment }

func (l clikeLexer) BlockComment() (string, string, bool) {
	if !l.blockOK {
		return "", "", false
	}
	return l.blockOpen, l.blockClose, true
}

// LexLine 按行切分。
func (l clikeLexer) LexLine(line []rune, state State) ([]Token, State) {
	var out []Token

	if state.Has(stateInBlockComment) {
		end := indexRunes(line, l.blockClose, 0)
		if end < 0 {
			return emit(out, KindComment, 0, len(line)), state
		}
		out = emit(out, KindComment, 0, end+len([]rune(l.blockClose)))
		state = state.Without(stateInBlockComment)
		return l.lexFrom(line, end+len([]rune(l.blockClose)), state, out)
	}

	return l.lexFrom(line, 0, state, out)
}

// lexFrom 从下标 start 开始扫描一行。
func (l clikeLexer) lexFrom(line []rune, start int, state State, out []Token) ([]Token, State) {
	i := start

	// 预处理指令整行一种颜色：#include 后面的宏名没必要再细分，
	// 而一旦细分就会把 <stdio.h> 里的尖括号刷成运算符，很难看。
	if l.preproc && isPreprocLine(line, i) {
		return emit(out, KindPreproc, i, len(line)), state
	}

	for i < len(line) {
		r := line[i]

		switch {
		case isBlank(r):
			i++

		case l.lineComment != "" && startsWithAt(line, i, l.lineComment):
			return emit(out, KindComment, i, len(line)), state

		case l.blockOK && startsWithAt(line, i, l.blockOpen):
			tail := indexRunes(line, l.blockClose, i+len([]rune(l.blockOpen)))
			if tail < 0 {
				return emit(out, KindComment, i, len(line)), state.With(stateInBlockComment)
			}
			end := tail + len([]rune(l.blockClose))
			out = emit(out, KindComment, i, end)
			i = end

		case l.rawQuote != 0 && r == l.rawQuote:
			if end, ok := scanRawString(line, i, l.rawQuote); ok {
				out = emit(out, KindString, i, end)
				i = end
				continue
			}
			out = emit(out, KindPunct, i, i+1)
			i++

		case r == '\'':
			if end, ok := scanCharLiteral(line, i, &out); ok {
				i = end
				continue
			}
			// 不是字符字面量：Rust 的生命周期 'a、Python 的撇号都走这里。
			out = emit(out, KindPunct, i, i+1)
			i++

		case r == '"':
			i = scanQuoted(line, i, '"', &out)

		case isDigit(r):
			j := scanNumber(line, i)
			out = emit(out, KindNumber, i, j)
			i = j

		case isIdentRune(r):
			j := scanIdent(line, i)
			out = emit(out, lexIdentAt(line, i, j, l.tables), i, j)
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
