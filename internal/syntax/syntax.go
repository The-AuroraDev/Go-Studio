// syntax.go — 语法高亮：语言无关的 token 模型与语言注册表。
// SPDX-License-Identifier: MIT

// Package syntax 提供按行切分的语法高亮。
//
// 设计取舍：
//
// 按行切分，不做整篇解析。编辑器每帧只画可见的几十行，若为了高亮而
// 解析整个文件，十万行的大文件每帧要重扫一遍，根本没法用。
// 代价是必须显式携带「跨行状态」——块注释、多行字符串要能跨行延续。
// State 就是干这个的：上一行结束时的状态传给下一行。
//
// 语言之间只共用 token 种类与 State 位标志，具体规则各自实现。
// 这样加一门语言不影响已有语言，也不用引入任何代码生成。
package syntax

import (
	"path/filepath"
	"strings"
	"sync"
	"unicode"
)

// Kind 是 token 的种类，决定用哪种颜色。
//
// 刻意保持语言中立：不同语言里同一个概念（类型、函数、字符串）
// 归到同一类，配色才能统一，也不会因为换语言而整屏变色。
type Kind uint8

const (
	// KindText 是没有被归类的普通文本，也是零值。
	KindText Kind = iota
	// KindComment 是注释。
	KindComment
	// KindString 是字符串字面量（含其定界符）。
	KindString
	// KindEscape 是字符串内部的转义序列。
	KindEscape
	// KindNumber 是数字字面量。
	KindNumber
	// KindKeyword 是语言的关键字。
	KindKeyword
	// KindKeywordType 是内建类型名，如 Go 的 int、C++ 的 unsigned。
	KindKeywordType
	// KindBuiltin 是内建函数或常量，如 Go 的 len/cap、C 的 printf。
	KindBuiltin
	// KindFunction 是函数名。
	KindFunction
	// KindType 是类型名。
	KindType
	// KindField 是结构体字段、成员变量。
	KindField
	// KindConstant 是常量。
	KindConstant
	// KindOperator 是运算符。
	KindOperator
	// KindPunct 是括号、逗号、句号一类不含语义的标点。
	KindPunct
	// KindPreproc 是预处理指令，如 C 的 #include。
	KindPreproc
	// KindLabel 是标签，如 Go 的 `back:`、C 的 goto 目标。
	KindLabel
	// KindInvalid 是明显不合法的写法，如未闭合的 Go 反引号串、未闭合的 HTML 标签。
	// 单独一类是为了让用户一眼看到哪里写错了。
	KindInvalid
)

// KindCount 是 token 种类总数，供配色表按数组下标索引。
const KindCount = int(KindInvalid) + 1

// kindNames 是用于日志与测试的名字，与 Kind 的声明顺序严格一致。
var kindNames = [...]string{
	"text", "comment", "string", "escape", "number", "keyword",
	"keyword-type", "builtin", "function", "type", "field", "constant",
	"operator", "punct", "preproc", "label", "invalid",
}

// String 返回 token 种类的名字。
func (k Kind) String() string {
	if int(k) < len(kindNames) {
		return kindNames[k]
	}
	return "unknown"
}

// Token 是行内的一段同类字符。
// Start、End 是 rune 下标，区间为 [Start, End)。
type Token struct {
	Kind  Kind
	Start int
	End   int
}

// Len 返回 token 覆盖的 rune 数。
func (t Token) Len() int { return t.End - t.Start }

// State 是跨行状态，用位标志表示可以同时成立的若干「上下文」。
//
// 只放真正会跨行的少数状态。放进 State 的东西越多，
// 状态组合的合法性就越难推理。
type State uint32

// State 各位的含义。
const (
	// stateInBlockComment 表示处于 /* */ 或等价结构内部。
	stateInBlockComment State = 1 << iota
	// stateInRawString 表示处于会吞掉换行的字符串里（Go 的反引号、Python 的三引号）。
	stateInRawString
	// stateInTemplate 表示处于模板字符串内部（C++/Rust 的 r#"..."#）。
	stateInTemplate
)

// Has 报告某个位是否置起。
func (s State) Has(bit State) bool { return s&bit != 0 }

// With 返回置起某些位之后的状态。
func (s State) With(bits State) State { return s | bits }

// Without 返回清掉某些位之后的状态。
func (s State) Without(bits State) State { return s &^ bits }

// String 便于调试与失败信息。
func (s State) String() string {
	if s == 0 {
		return "none"
	}
	var names []string
	for _, item := range []struct {
		bit  State
		name string
	}{
		{stateInBlockComment, "block-comment"},
		{stateInRawString, "raw-string"},
		{stateInTemplate, "template"},
		{stateInLongBracket, "long-bracket"},
	} {
		if s.Has(item.bit) {
			names = append(names, item.name)
		}
	}
	return strings.Join(names, "+")
}

// Lexer 是一门语言的高亮规则。
//
// LexLine 只处理一行：读入上一行留下的状态，返回本行的 token
// 与本行结束时的状态。它必须是纯函数——同样的输入必须得到同样的输出，
// 高亮结果才能被缓存、被测试、被复用。
type Lexer interface {
	// Name 是语言名，用于状态栏与命令面板显示。
	Name() string
	// Extensions 是这份规则认领的文件扩展名（含点，小写）。
	Extensions() []string
	// Filenames 是这份规则认领的完整文件名（如 Makefile、Dockerfile）。
	Filenames() []string
	// LineComment 是行注释的起始符。空字符串表示该语言没有行注释。
	LineComment() string
	// BlockComment 是块注释的成对定界符。ok 为 false 表示不支持块注释。
	BlockComment() (open, close string, ok bool)
	// LexLine 把一行拆成 token。state 是上一行结束时的状态。
	// 返回的 token 必须按 Start 递增且互不重叠。
	LexLine(line []rune, state State) ([]Token, State)
}

// registry 是语言注册表。
var registry = struct {
	mu    sync.RWMutex
	byExt map[string]Lexer
	byNam map[string]Lexer
}{byExt: map[string]Lexer{}, byNam: map[string]Lexer{}}

// Register 登记一门语言。同名或同扩展名重复登记会 panic：
// 那是开发期的接线错误，必须立刻暴露，否则会静默用错规则。
func Register(lex Lexer) {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	name := lex.Name()
	if _, dup := registry.byNam[name]; dup {
		panic("syntax: 语言名重复登记 " + name)
	}
	registry.byNam[name] = lex
	for _, ext := range lex.Extensions() {
		key := strings.ToLower(ext)
		if _, dup := registry.byExt[key]; dup {
			panic("syntax: 扩展名重复登记 " + key)
		}
		registry.byExt[key] = lex
	}
}

// ForFilename 返回适用于该路径的语言规则，不认领则返回 nil。
//
// 认领顺序：先看完整文件名（Makefile、.gitignore），
// 再看扩展名。都不中就返回 nil，由调用方决定不高亮。
func ForFilename(path string) Lexer {
	name := filepath.Base(path)
	ext := strings.ToLower(filepath.Ext(name))

	registry.mu.RLock()
	defer registry.mu.RUnlock()

	for _, lex := range registry.byNam {
		for _, candidate := range lex.Filenames() {
			if candidate == name {
				return lex
			}
		}
	}
	if ext == "" {
		return nil
	}
	return registry.byExt[ext]
}

// ForLanguage 按语言名返回规则。
func ForLanguage(name string) Lexer {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	return registry.byNam[name]
}

// Languages 返回全部已登记的语言名，字典序。
func Languages() []string {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	out := make([]string, 0, len(registry.byNam))
	for name := range registry.byNam {
		out = append(out, name)
	}
	sortStrings(out)
	return out
}

// sortStrings 是插入排序，避免为一个几十元素的切片引入 sort 依赖。
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// ---- 词法辅助 ----

// isIdentRune 报告字符是否可构成标识符。
//
// 覆盖 Unicode 字母与数字：中文等语言的标识符里出现汉字并不罕见，
// 只认 ASCII 会让整段中文被当成标点刷一遍。
func isIdentRune(r rune) bool {
	return r == '_' || isLetter(r) || isDigit(r)
}

// isLetter 报告是否是字母（ASCII 或 Unicode 字母）。
func isLetter(r rune) bool {
	if r < 0x80 {
		return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
	}
	return unicodeIsLetter(r)
}

// isDigit 报告是否是十进制数字。
func isDigit(r rune) bool {
	if r < 0x80 {
		return r >= '0' && r <= '9'
	}
	return unicodeIsDigit(r)
}

// isUpper 报告是否是大写字母（用于 Go/C 的类型名约定）。
func isUpper(r rune) bool {
	if r < 0x80 {
		return r >= 'A' && r <= 'Z'
	}
	return unicodeIsUpper(r)
}

// token 构造一个 token。
func token(kind Kind, start, end int) Token { return Token{Kind: kind, Start: start, End: end} }

// emit 把一个区间追加到结果里，忽略空区间与越界区间。
func emit(out []Token, kind Kind, start, end int) []Token {
	if end <= start || start < 0 {
		return out
	}
	return append(out, token(kind, start, end))
}

// unicodeIsLetter 等是包内的小包装，把 unicode 包的判定集中在一处，
// 便于将来按语言覆写。
func unicodeIsLetter(r rune) bool { return unicode.IsLetter(r) }
func unicodeIsDigit(r rune) bool  { return unicode.IsDigit(r) }
func unicodeIsUpper(r rune) bool  { return unicode.IsUpper(r) }
