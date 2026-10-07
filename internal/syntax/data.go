// data.go — JSON、TOML、YAML、INI 的高亮规则。
// SPDX-License-Identifier: MIT

package syntax

func init() {
	Register(jsonLexer{})
	Register(tomlLexer{})
	Register(yamlLexer{})
	Register(iniLexer{})
}

// ---- JSON ----

// jsonLexer 是 JSON 的规则。
//
// 只有字符串、数字、布尔与 null 四类，规则很少，
// 但键与值要区分开——这是读配置文件时最需要的信息。
type jsonLexer struct{}

func (jsonLexer) Name() string         { return "JSON" }
func (jsonLexer) Extensions() []string { return []string{".json", ".jsonc"} }
func (jsonLexer) Filenames() []string {
	return []string{"package.json", "tsconfig.json", ".babelrc", ".eslintrc"}
}
func (jsonLexer) LineComment() string { return "//" }
func (jsonLexer) BlockComment() (string, string, bool) {
	return "/*", "*/", true
}

func (jsonLexer) LexLine(line []rune, state State) ([]Token, State) {
	var out []Token

	if state.Has(stateInBlockComment) {
		end := indexRunes(line, "*/", 0)
		if end < 0 {
			return emit(out, KindComment, 0, len(line)), state
		}
		out = emit(out, KindComment, 0, end+2)
		return jsonLexFrom(line, end+2, state.Without(stateInBlockComment), out)
	}
	return jsonLexFrom(line, 0, state, out)
}

func jsonLexFrom(line []rune, i int, state State, out []Token) ([]Token, State) {
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
				return emit(out, KindComment, i, len(line)), state.With(stateInBlockComment)
			}
			out = emit(out, KindComment, i, end+2)
			i = end + 2
		case r == '"':
			// 冒号前的字符串是键，其余是值。
			base := len(out)
			j := scanQuoted(line, i, '"', &out)
			if restIsColon(line, j) {
				for k := base; k < len(out); k++ {
					if out[k].Kind == KindString {
						out[k].Kind = KindField
					}
				}
			}
			i = j
		case isDigit(r) || (r == '-' && i+1 < len(line) && isDigit(line[i+1])):
			j := scanNumber(line, i)
			out = emit(out, KindNumber, i, j)
			i = j
		case isIdentRune(r):
			j := scanIdent(line, i)
			out = emit(out, jsonLiteralKind(string(line[i:j])), i, j)
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

// restIsColon 报告从 i 起（跳过空白）是否紧跟冒号。
func restIsColon(line []rune, i int) bool {
	for j := i; j < len(line); j++ {
		if line[j] == ' ' || line[j] == '\t' {
			continue
		}
		return line[j] == ':'
	}
	return false
}

func jsonLiteralKind(word string) Kind {
	switch word {
	case "true", "false", "null":
		return KindConstant
	}
	return KindText
}

// ---- TOML ----

// tomlLexer 是 TOML 的规则。
//
// TOML 的注释、字符串、数字与 JSON 很像，
// 差别在行首的 [table] 与 key = value。
type tomlLexer struct{}

func (tomlLexer) Name() string         { return "TOML" }
func (tomlLexer) Extensions() []string { return []string{".toml"} }
func (tomlLexer) Filenames() []string  { return []string{"Cargo.lock", "go.mod", "Gopkg.lock"} }
func (tomlLexer) LineComment() string  { return "#" }
func (tomlLexer) BlockComment() (string, string, bool) {
	return "", "", false
}

func (tomlLexer) LexLine(line []rune, state State) ([]Token, State) {
	var out []Token
	i := 0

	// 行首的 [ 或 [[ 是表头，整行一种颜色。
	if isTableHeader(line) {
		return emit(out, KindType, 0, len(line)), state
	}

	for i < len(line) {
		r := line[i]
		switch {
		case isBlank(r):
			i++
		case r == '#':
			return emit(out, KindComment, i, len(line)), state
		case r == '"' || r == '\'':
			base := len(out)
			j := scanQuoted(line, i, r, &out)
			if restIsEquals(line, j) {
				for k := base; k < len(out); k++ {
					if out[k].Kind == KindString {
						out[k].Kind = KindField
					}
				}
			}
			i = j
		case isDigit(r) || (r == '-' && i+1 < len(line) && isDigit(line[i+1])):
			j := scanNumber(line, i)
			out = emit(out, KindNumber, i, j)
			i = j
		case isIdentRune(r):
			j := scanIdent(line, i)
			kind := tomlIdentKind(string(line[i:j]))
			if kind == KindText && restIsEquals(line, j) {
				kind = KindField
			}
			out = emit(out, kind, i, j)
			i = j
		default:
			if isOperatorRune(r) || r == '=' {
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

func tomlIdentKind(word string) Kind {
	switch word {
	case "true", "false":
		return KindConstant
	case "inf", "nan":
		return KindBuiltin
	}
	return KindText
}

// restIsEquals 报告从 i 起（跳过空白）是否紧跟等号。
func restIsEquals(line []rune, i int) bool {
	for j := i; j < len(line); j++ {
		if line[j] == ' ' || line[j] == '\t' {
			continue
		}
		return line[j] == '='
	}
	return false
}

// isTableHeader 报告本行是否是 TOML 表头（[table] 或 [[array]]）。
func isTableHeader(line []rune) bool {
	trimmed := []rune(trimLeftSpace(line))
	if len(trimmed) < 2 || trimmed[0] != '[' {
		return false
	}
	// 表头必须成对闭合。光有一个左括号不算——
	// 否则正在输入的 "[" 会被整行刷成表头颜色。
	return indexRuneFrom(trimmed, ']', 1) >= 0
}

// ---- YAML ----

// yamlLexer 是 YAML 的规则。
//
// YAML 的难点是没有明显语法标记，全靠缩进与键值形态。
// 这里用三个启发式：行首 - 是列表项、行首 key: 是映射、
// 行首 # 是注释。
type yamlLexer struct{}

func (yamlLexer) Name() string         { return "YAML" }
func (yamlLexer) Extensions() []string { return []string{".yaml", ".yml"} }
func (yamlLexer) Filenames() []string {
	return []string{".travis.yml", "docker-compose.yml", "docker-compose.yaml"}
}
func (yamlLexer) LineComment() string { return "#" }
func (yamlLexer) BlockComment() (string, string, bool) {
	return "", "", false
}

func (yamlLexer) LexLine(line []rune, state State) ([]Token, State) {
	var out []Token
	i := 0

	trimmed := trimLeftSpace(line)
	indent := indexOfNonSpace(line, 0)

	// 整行注释。
	if len(trimmed) == 0 {
		return nil, state
	}
	if trimmed[0] == '#' {
		return emit(out, KindComment, indent, len(line)), state
	}
	// 文档分隔符 --- 与 ...
	if trimmed == "---" || trimmed == "..." {
		return emit(out, KindConstant, indent, len(line)), state
	}
	// 锚点 &name 与别名 *name。
	// 先发符号再发名字：反过来写两个 token 的区间会重叠，
	// 渲染层靠递增下标推进，重叠就等于漏字。
	if trimmed[0] == '&' || trimmed[0] == '*' {
		// scanIdent 返回的是结束下标，从 1 起扫就得减 1 才是长度。
		// 单独一个 & 是合法的输入（正在输入锚点），不能因为没有名字就越界。
		nameLen := scanIdent([]rune(trimmed), 1) - 1
		out = emit(out, KindOperator, indent, indent+1)
		if nameLen > 0 {
			out = emit(out, KindConstant, indent+1, indent+1+nameLen)
		}
		return out, state
	}

	for i < len(line) {
		r := line[i]
		switch {
		case isBlank(r):
			i++
		case r == '#':
			return emit(out, KindComment, i, len(line)), state
		case r == '"' || r == '\'':
			i = scanQuoted(line, i, r, &out)
		case isDigit(r) || (r == '-' && i+1 < len(line) && isDigit(line[i+1])):
			j := scanNumber(line, i)
			out = emit(out, KindNumber, i, j)
			i = j
		case isIdentRune(r):
			j := scanIdent(line, i)
			word := string(line[i:j])
			kind := yamlWordKind(word)
			if restIsColon(line, j) {
				kind = KindField
			} else if j < len(line) && line[j] == ':' {
				kind = KindField
			}
			out = emit(out, kind, i, j)
			i = j
		default:
			if r == '-' && (i == indent || isBlankBeforeValue(line, i)) {
				out = emit(out, KindOperator, i, i+1)
				i++
				continue
			}
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

func yamlWordKind(word string) Kind {
	switch word {
	case "true", "false", "null", "yes", "no", "on", "off", "~":
		return KindConstant
	}
	return KindText
}

// isBlankBeforeValue 报告连字符后是否紧跟空白（`- value` 的列表项）。
func isBlankBeforeValue(line []rune, i int) bool {
	return i+1 >= len(line) || line[i+1] == ' ' || line[i+1] == '\t'
}

// ---- INI ----

// iniLexer 是 INI 与 .properties 的规则。
type iniLexer struct{}

func (iniLexer) Name() string { return "INI" }
func (iniLexer) Extensions() []string {
	return []string{".ini", ".cfg", ".conf", ".properties", ".desktop"}
}
func (iniLexer) Filenames() []string { return []string{".editorconfig", ".gitconfig"} }
func (iniLexer) LineComment() string { return ";" }
func (iniLexer) BlockComment() (string, string, bool) {
	return "", "", false
}

func (iniLexer) LexLine(line []rune, state State) ([]Token, State) {
	var out []Token
	trimmed := trimLeftSpace(line)

	// 段名 [section]
	if len(trimmed) > 0 && trimmed[0] == '[' {
		return emit(out, KindType, indexOfNonSpace(line, 0), len(line)), state
	}
	// 注释：; 或 # 开头。# 不在 LineComment 里是因为 shell 风格的
	// 配置常常用 #。
	if len(trimmed) > 0 && (trimmed[0] == ';' || trimmed[0] == '#') {
		return emit(out, KindComment, indexOfNonSpace(line, 0), len(line)), state
	}

	i := 0
	for i < len(line) {
		r := line[i]
		switch {
		case isBlank(r):
			i++
		case r == ';' || r == '#':
			return emit(out, KindComment, i, len(line)), state
		case r == '=' || r == ':':
			out = emit(out, KindOperator, i, i+1)
			i++
		case r == '"' || r == '\'':
			i = scanQuoted(line, i, r, &out)
		case isDigit(r):
			j := scanNumber(line, i)
			out = emit(out, KindNumber, i, j)
			i = j
		case isIdentRune(r):
			j := scanIdent(line, i)
			word := string(line[i:j])
			kind := KindText
			// 等号/冒号与键之间通常隔着空格，必须跳过空白再看。
			switch {
			case restIsEquals(line, j), restIsColon(line, j):
				kind = KindField
			case isUpper(rune(word[0])):
				kind = KindType
			}
			out = emit(out, kind, i, j)
			i = j
		default:
			out = emit(out, KindText, i, i+1)
			i++
		}
	}
	return out, state
}
