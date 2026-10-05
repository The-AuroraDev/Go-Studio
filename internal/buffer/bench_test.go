// bench_test.go — 缓冲性能基准。
// SPDX-License-Identifier: MIT

package buffer

import (
	"fmt"
	"strings"
	"testing"
)

// benchSizes 是规模档位。makeDoc 每个单元产出 3 行，因此 docLines 才是真实行数。
var benchSizes = []struct {
	name     string
	docLines int
}{
	{"1k", 1000},
	{"10k", 10000},
	{"100k", 100000},
	{"500k", 500000},
}

// makeDoc 造出不少于 lines 行的 Go 风格文档。
func makeDoc(lines int) []byte {
	var sb strings.Builder
	sb.Grow(lines * 40)
	for written := 0; written < lines; {
		fmt.Fprintf(&sb, "func fn%d(a int, b string) error {\n\treturn nil\n}\n", written)
		written += 3
	}
	return []byte(sb.String())
}

// BenchmarkOpenLargeFile 是 M1 的关键指标：打开大文件必须足够快。
// 验收门槛是 10 万行 50 毫秒。
func BenchmarkOpenLargeFile(b *testing.B) {
	for _, size := range benchSizes {
		doc := makeDoc(size.docLines)
		b.Run(size.name, func(b *testing.B) {
			b.SetBytes(int64(len(doc)))
			b.ReportAllocs()
			for b.Loop() {
				New(doc)
			}
		})
	}
}

// BenchmarkLineLookup 测量按行号定位的成本。
func BenchmarkLineLookup(b *testing.B) {
	doc := makeDoc(100000)
	buf := New(doc)
	total := buf.LineCount()

	b.Run("100k", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			for i := range total {
				_ = buf.LineStart(i)
			}
		}
	})
}

// BenchmarkOffsetToLineCol 测量字节偏移到行列的转换成本。
func BenchmarkOffsetToLineCol(b *testing.B) {
	doc := makeDoc(100000)
	buf := New(doc)

	b.Run("100k", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			for offset := 0; offset < len(doc); offset += 997 {
				_, _ = buf.LineCol(offset)
			}
		}
	})
}

// BenchmarkInsert 测量单次插入的编辑成本。
//
// 每轮「插入后立即撤销」而不是每轮重建缓冲：重建 10 万行文档要几毫秒，
// 而 Go 是按计时部分决定 b.N 的，把昂贵的构造放在 b.StopTimer 里会让
// 迭代次数暴涨、总耗时反被构造占据。插入再撤销让片段数回到原状，
// 既保持了 O(1) 的准备开销，也仍然覆盖同样的片段路径。
func BenchmarkInsert(b *testing.B) {
	cases := []struct {
		name  string
		lines int
		at    func(total int) int
	}{
		{"100k-top", 100000, func(int) int { return 0 }},
		{"100k-middle", 100000, func(total int) int { return total / 2 }},
		{"100k-end", 100000, func(total int) int { return total }},
	}

	for _, tc := range cases {
		doc := makeDoc(tc.lines)
		inserted := []byte("// 插入一行\n")
		b.Run(tc.name, func(b *testing.B) {
			buf := New(doc)
			offset := tc.at(len(doc))
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				buf.Replace(offset, 0, inserted)
				buf.Replace(offset, len(inserted), nil)
			}
		})
	}
}

// BenchmarkTypingSession 模拟连续输入，考察片段数增长后的编辑成本。
// 规模翻倍若耗时增长超过四倍，说明存在平方级退化。
func BenchmarkTypingSession(b *testing.B) {
	for _, chars := range []int{500, 1000, 2000, 4000} {
		doc := makeDoc(10000)
		b.Run(fmt.Sprintf("chars=%d", chars), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				buf := New(doc)
				offset := 0
				for range chars {
					buf.Replace(offset, 0, []byte("a"))
					offset++
				}
			}
		})
	}
}

// BenchmarkReadWholeFile 测量全量读取，用于评估大文件上 Text() 的代价。
func BenchmarkReadWholeFile(b *testing.B) {
	doc := makeDoc(100000)
	buf := New(doc)
	b.SetBytes(int64(len(doc)))
	b.Run("100k", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_ = buf.Text()
		}
	})
}
