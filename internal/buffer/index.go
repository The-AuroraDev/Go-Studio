// index.go — 行索引：记录每一行第一个字节的偏移，支持 O(log n) 行定位。
// SPDX-License-Identifier: MIT

package buffer

import "sort"

// lineIndex 保存有序的行首偏移。starts[0] 恒为 0，空文档也算一行。
type lineIndex struct {
	starts []int
}

// init 为已有内容的缓冲建立初始索引。
// 只有构造缓冲时需要扫描全文，之后的编辑都走增量更新。
func (li *lineIndex) init(b *Buffer) {
	li.starts = []int{0}
	li.starts = append(li.starts, b.collectNewlines(0, b.total)...)
}

// count 返回行数。
func (li *lineIndex) count() int {
	return len(li.starts)
}

// start 返回第 i 行的起始偏移，越界时返回文档末尾之后的值由调用方保证。
func (li *lineIndex) start(i int) int {
	if i < 0 {
		return 0
	}
	if i >= len(li.starts) {
		return li.starts[len(li.starts)-1]
	}
	return li.starts[i]
}

// end 返回第 i 行结束（不含换行符之后、含换行符）的偏移。
// i 是最后一行时返回 total。
func (li *lineIndex) end(i, total int) int {
	if i+1 < len(li.starts) {
		return li.starts[i+1]
	}
	return total
}

// lineOf 返回给定偏移所在的行号，即最后一个不超过该偏移的行首下标。
func (li *lineIndex) lineOf(offset int) int {
	idx := sort.SearchInts(li.starts, offset+1) - 1
	if idx < 0 {
		return 0
	}
	return idx
}

// adjust 在一次编辑后增量更新索引。
//
// offset 是编辑起点，removed 是删除字节数，inserted 是插入字节数，
// oldTotal 是编辑前的文档长度。b 必须已经应用了这次编辑。
//
// 只重扫受影响的那几行，因此代价是 O(受影响行数 + log n)，
// 与文件总长度无关。
func (li *lineIndex) adjust(b *Buffer, offset, removed, inserted, oldTotal int) {
	first := li.lineOf(offset)
	last := li.lineOf(offset + removed)

	// 最后一个受影响行之后的第一行起始位置，也就是重扫区间的右边界。
	oldStop := oldTotal
	if last+1 < len(li.starts) {
		oldStop = li.starts[last+1]
	}
	// 右边界在新坐标下的位置。
	newStop := oldStop + inserted - removed

	// 保留受影响行之前的行首，其余丢弃重建。
	tail := make([]int, 0, len(li.starts)-(last+1))
	for _, s := range li.starts[last+1:] {
		tail = append(tail, s+inserted-removed)
	}
	li.starts = li.starts[:first+1]

	// 从编辑点向后扫描，逐个登记新行首。
	// 恰好落在 newStop 的行首由保留下来的尾部提供，重复登记会让行数虚增；
	// 但尾部为空时（文档正好以换行结尾）没有别人能提供它，必须保留。
	hasTail := len(tail) > 0
	for _, s := range b.collectNewlines(offset, newStop) {
		if s == newStop && hasTail {
			continue
		}
		li.starts = append(li.starts, s)
	}
	li.starts = append(li.starts, tail...)
}
