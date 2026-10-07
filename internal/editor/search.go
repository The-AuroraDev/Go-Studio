// search.go — 在当前文档里查找：跳到下一处/上一处匹配。
// SPDX-License-Identifier: MIT

package editor

import (
	"strings"

	"github.com/29anan29/Go-Studio/internal/document"
	"github.com/29anan29/Go-Studio/internal/keymap"
)

// search 是当前的一次查找会话。
//
// 它只记查询串与「上一次匹配的位置」。下一处/上一处都从这个位置出发，
// 因此连按同一个键就能一路走遍所有匹配，这是查找的基本预期。
type search struct {
	query string
	// last 是上一次匹配命中的字节偏移，-1 表示还没找到过。
	// 初值必须是 -1 而不是 0：0 是一个合法的偏移，用它当「没找到过」
	// 会让第一次查找跳过正好在文档开头的匹配。
	last int
	// wrapped 记录上一次查找是否绕回了文档开头。
	// 绕回时必须提示，否则用户会以为「还有匹配」而一直按。
	wrapped bool
}

// newSearch 造一个查找会话。
func (a *App) newSearch(query string) *search {
	return &search{query: query, last: -1}
}

// findNext 跳到下一处匹配，返回是否找到。
func (a *App) findNext() bool { return a.searchStep(1) }

// findPrev 跳到上一处匹配，返回是否找到。
func (a *App) findPrev() bool { return a.searchStep(-1) }

// searchStep 在文档里按 direction 方向找一处匹配并移动光标。
//
// 查找的起点很关键：必须从「当前光标所在匹配的后面」开始，而不是从光标处开始。
// 从光标处开始的话，光标已经停在某一处匹配上，再按一次 n 只会原地找到同一个位置，
// 连按几次都走不动——这是查找功能最容易犯的错误。
//
// 光标若被用户挪到别处（编辑、移动），则从光标处重新开始，
// 这样查找始终以用户当前关注的位置为准。
func (a *App) searchStep(direction int) bool {
	s := a.find
	if s == nil {
		return false
	}
	if a.doc == nil || !strings.Contains(string(a.doc.Text()), s.query) {
		return false
	}

	cursorOffset := a.doc.MustOffset()
	total := a.doc.Buffer().Len()
	onLastMatch := s.last >= 0 && s.last == cursorOffset

	if direction >= 0 {
		from := cursorOffset
		if onLastMatch {
			from = s.last + len(s.query)
		}
		if idx, ok := findFrom(a.doc, s.query, from, direction); ok {
			s.last = idx
			s.wrapped = false
			a.moveToOffset(idx)
			return true
		}
		// 从头再找一遍，且必须避开刚看过的那一处，否则会原地不动。
		if idx, ok := findFrom(a.doc, s.query, 0, direction); ok && idx != s.last {
			s.last = idx
			s.wrapped = true
			a.moveToOffset(idx)
			return true
		}
		return false
	}

	// 反向：从当前匹配的起点之前往回找。
	from := cursorOffset
	if onLastMatch {
		from = s.last
	}
	if idx, ok := findBackFrom(a.doc, s.query, from); ok {
		s.last = idx
		s.wrapped = false
		a.moveToOffset(idx)
		return true
	}
	if idx, ok := findBackFrom(a.doc, s.query, total); ok && idx != s.last {
		s.last = idx
		s.wrapped = true
		a.moveToOffset(idx)
		return true
	}
	return false
}

// moveToOffset 把光标移到指定的字节偏移。
func (a *App) moveToOffset(offset int) {
	line, col := a.doc.Buffer().LineCol(offset)
	a.doc.SetCursor(line, col)
}

// findFrom 从 from 起按 direction（只支持 +1）查找 query 第一次出现的位置。
// 匹配成功后返回的偏移是 query 的**起始**字节偏移。
func findFrom(doc *document.Document, query string, from, direction int) (int, bool) {
	if direction < 0 {
		return 0, false
	}
	text := string(doc.Text())
	idx := strings.Index(text[from:], query)
	if idx < 0 {
		return 0, false
	}
	return from + idx, true
}

// findBackFrom 从 from 往回查找 query 最后一次出现的位置。
// 返回的是 query 的**起始**字节偏移。
func findBackFrom(doc *document.Document, query string, from int) (int, bool) {
	if from <= 0 {
		return 0, false
	}
	text := string(doc.Text())
	if from > len(text) {
		from = len(text)
	}
	idx := strings.LastIndex(text[:from], query)
	if idx < 0 {
		return 0, false
	}
	return idx, true
}

// findNextCmd 是「查找下一个」的键位处理：没有查找串时先要用户输入。
func (a *App) findNextCmd(_ keymap.Binding) {
	if a.find == nil {
		a.openPrompt(promptFind, "")
		return
	}
	if !a.findNext() {
		a.setStatus("找不到 " + a.find.query)
		return
	}
	a.announceFind("下一处")
}

// findPrevCmd 是「查找上一个」的键位处理。
func (a *App) findPrevCmd(_ keymap.Binding) {
	if a.find == nil {
		a.openPrompt(promptFind, "")
		return
	}
	if !a.findPrev() {
		a.setStatus("找不到 " + a.find.query)
		return
	}
	a.announceFind("上一处")
}

// announceFind 报告查找结果，绕回文档另一端时必须说清楚。
func (a *App) announceFind(where string) {
	if !a.find.wrapped {
		a.setStatus(where)
		return
	}
	// 绕回的方向由匹配位置与光标的相对关系决定：
	// 停在光标之前就是绕到了文档开头，之后则是末尾。
	edge := "末尾"
	if a.find.last <= a.doc.MustOffset() {
		edge = "开头"
	}
	a.setStatus(where + "（已绕到文档" + edge + "）")
}
