// probe_test.go — 探测：各终端序列经本包归一化之后叫什么按键名。
// SPDX-License-Identifier: MIT

//go:build !windows

package screen

import (
	"testing"
	"time"

	"github.com/creack/pty"
)

// TestProbeSequences 把常见终端控制序列送进 Bubble Tea，打印归一化之后的按键名。
//
// 这个测试不写断言，它的作用是把「终端发什么字节」与「键位表看到什么名字」
// 两者对应关系打印出来。写 e2e 脚本时需要知道某个键到底该发哪几个字节，
// 猜是猜不准的——每种终端对 Ctrl+End 之类的编码都不一样。
//
// 运行：go test -v -run TestProbeSequences ./internal/screen/
func TestProbeSequences(t *testing.T) {
	cases := []struct{ name, seq string }{
		{"Enter", "\r"},
		{"Esc", "\x1b"},
		{"Backspace(BS)", "\x08"},
		{"Backspace(DEL)", "\x7f"},
		{"Tab", "\t"},
		{"Space", " "},
		{"Up", "\x1b[A"},
		{"Down", "\x1b[B"},
		{"Right", "\x1b[C"},
		{"Left", "\x1b[D"},
		{"Home CSI H", "\x1b[H"},
		{"Home CSI 1~", "\x1b[1~"},
		{"Home CSI 7~", "\x1b[7~"},
		{"End CSI F", "\x1b[F"},
		{"End CSI 4~", "\x1b[4~"},
		{"End CSI 8~", "\x1b[8~"},
		{"Ctrl+Home 1;5H", "\x1b[1;5H"},
		{"Ctrl+End 1;5F", "\x1b[1;5F"},
		{"Shift+End 1;2F", "\x1b[1;2F"},
		{"PgUp", "\x1b[5~"},
		{"PgDown", "\x1b[6~"},
		{"Ctrl+Right 1;5C", "\x1b[1;5C"},
		{"Ctrl+Left 1;5D", "\x1b[1;5D"},
		{"Delete", "\x1b[3~"},
		{"Ctrl+A", "\x01"},
		{"Ctrl+Space(NUL)", "\x00"},
		{"Alt+a(ESC a)", "\x1ba"},
	}

	for _, tc := range cases {
		keys := probeKeys(t, tc.seq)
		if len(keys) == 0 {
			t.Logf("%-20s %-12q -> (无事件)", tc.name, tc.seq)
			continue
		}
		t.Logf("%-20s %-12q -> %v", tc.name, tc.seq, keys)
	}
}

// TestProbeSequencesStable 把最容易出错、也最该被盯住的几条钉死。
//
// 探测表是给人看的，这里是给机器看的：
// 空格曾经被归一化成命名键 <space>（于是输入空格会触发文件树），
// Ctrl+Space 必须与普通空格区分开（它绑定着文件树），
// Ctrl+End 必须带上 ctrl 修饰（键位表里是 ctrl+<end> 这一个单键）。
// 任何一条回退，都会在 e2e 脚本里表现为「某个功能莫名其妙没反应」。
func TestProbeSequencesStable(t *testing.T) {
	cases := []struct {
		name, seq, want string
	}{
		{"空格必须是字符空格", " ", "space"},
		{"Ctrl+空格必须是独立按键", "\x00", "ctrl+space"},
		{"退格", "\x7f", "<backspace>"},
		{"Ctrl+End 要带修饰", "\x1b[1;5F", "ctrl+<end>"},
		{"Ctrl+Home 要带修饰", "\x1b[1;5H", "ctrl+<home>"},
		{"普通 End 不带修饰", "\x1b[F", "<end>"},
		{"Ctrl+Right 要带修饰", "\x1b[1;5C", "ctrl+<right>"},
		{"Ctrl+A", "\x01", "ctrl+a"},
		{"Alt 用 ESC 前缀", "\x1b1", "alt+1"},
		{"回车", "\r", "<enter>"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			keys := probeKeys(t, tc.seq)
			if len(keys) != 1 {
				t.Fatalf("序列 %q 解析出 %v, want 恰好 1 个按键", tc.seq, keys)
			}
			if keys[0] != tc.want {
				t.Errorf("序列 %q -> %q, want %q", tc.seq, keys[0], tc.want)
			}
		})
	}
}

// probeKeys 起一个真的后端，写入 seq，收集它解析出的按键名。
func probeKeys(t *testing.T, seq string) []string {
	t.Helper()

	master, tty, err := pty.Open()
	if err != nil {
		t.Skipf("无法分配伪终端: %v", err)
	}
	defer master.Close()
	defer tty.Close()
	if err := pty.Setsize(master, &pty.Winsize{Rows: 24, Cols: 80}); err != nil {
		t.Skipf("无法设置尺寸: %v", err)
	}
	go func() {
		buf := make([]byte, 4096)
		for {
			if _, err := master.Read(buf); err != nil {
				return
			}
		}
	}()

	s, err := NewBubble(BubbleOptions{Output: tty, Input: tty, FPS: 120})
	if err != nil {
		t.Skipf("启动后端失败: %v", err)
	}
	defer s.Close()
	b := s.(*bubbleScreen)

	// 等就绪，否则事件循环还没起来。
	deadline := time.After(5 * time.Second)
	for b.Caps().Width == 0 {
		select {
		case <-deadline:
			t.Skip("后端未就绪")
		default:
		}
	}

	if _, err := master.WriteString(seq); err != nil {
		return nil
	}

	var keys []string
	wait := time.After(time.Second)
	for {
		select {
		case event := <-b.Events():
			switch event.Kind {
			case EventKeyPress:
				keys = append(keys, event.Keystroke)
			case EventResize:
			default:
			}
			if len(keys) > 0 {
				return keys
			}
		case <-wait:
			return keys
		}
	}
}
