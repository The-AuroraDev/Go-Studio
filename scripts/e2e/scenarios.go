// scenarios.go — 完整使用测试的场景定义，逐条对应 spec.md 与真实使用流程。
// SPDX-License-Identifier: MIT

//go:build !windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// allScenarios 返回全部场景。
//
// 顺序按「上手难度」排：先是最基本的输入与保存，再逐步到导航、查找、
// 多标签、目录浏览，最后是边界情况。出问题时从前往后看更容易定位。
func allScenarios() []scenario {
	return []scenario{
		basicEditing(),
		navigation(),
		undoRedo(),
		openFile(),
		openDirectory(),
		gotoLine(),
		findInBuffer(),
		tabs(),
		wideCharacters(),
		readonlyFile(),
		cancelPrompt(),
		newFile(),
		resize(),
	}
}

// file 在工作目录里造一个文件，返回路径。
func file(dir, name, content string) string {
	path := filepath.Join(dir, name)
	must(os.WriteFile(path, []byte(content), 0o644))
	return path
}

// ---- 场景 1：基本编辑与保存 ----

func basicEditing() scenario {
	return scenario{
		name: "基本编辑：键入、保存、终端恢复",
		steps: []step{
			{name: "启动并显示文件名", fn: func(e *env) error {
				if err := e.start("demo.txt", ""); err != nil {
					return err
				}
				return e.expect("demo.txt", 15*time.Second)
			}},
			{name: "键入 hello 时光标列号推进", fn: func(e *env) error {
				e.sendSlow("hello", 60*time.Millisecond)
				return e.expect("1:6", 10*time.Second)
			}},
			{name: "编辑后状态栏出现脏标记", fn: func(e *env) error {
				return e.expect("*", 5*time.Second)
			}},
			{name: "C-a w 保存", fn: func(e *env) error {
				e.ctrlA()
				e.send("w")
				return e.expect("已保存", 10*time.Second)
			}},
			{name: "磁盘内容与输入一致", fn: func(e *env) error {
				return e.text(filepath.Join(e.dir, "demo.txt"), "hello")
			}},
			{name: "退出时终端被完全恢复", fn: func(e *env) error {
				return e.stop()
			}},
		},
	}
}

// ---- 场景 2：导航 ----

func navigation() scenario {
	steps := []step{{name: "打开 20 行的文件", fn: func(e *env) error {
		return e.start("nav.txt", numberedContent(20))
	}}}
	steps = append(steps,
		step{name: "Ctrl+End 跳到文末", fn: func(e *env) error {
			e.key("ctrl+end")
			// numberedContent 以换行结尾，缓冲按「换行数 + 1」算行，
			// 所以文末是第 21 行（空行）的第 1 列。
			return e.expect("21:1", 10*time.Second)
		}},
		step{name: "上一行到第 20 行（期望列沿用 1）", fn: func(e *env) error {
			e.key("up")
			return e.expect("20:1", 10*time.Second)
		}},
		step{name: "end 到第 20 行行尾（line20 有 6 个字符，列号从 1 起）", fn: func(e *env) error {
			e.key("end")
			return e.expect("20:7", 10*time.Second)
		}},
		step{name: "home 回到行首（20:1）", fn: func(e *env) error {
			e.key("home")
			return e.expect("20:1", 10*time.Second)
		}},
		step{name: "Ctrl+Home 跳到文首（1:1）", fn: func(e *env) error {
			e.key("ctrl+home")
			return e.expect("1:1", 10*time.Second)
		}},
		step{name: "下移 5 行（6:1）", fn: func(e *env) error {
			for range 5 {
				e.key("down")
				time.Sleep(40 * time.Millisecond)
			}
			return e.expect("6:1", 10*time.Second)
		}},
		step{name: "end 后上移会保留期望列", fn: func(e *env) error {
			e.key("end")
			// line6 有 5 个字符，光标落在行尾即第 6 列。
			if err := e.expect("6:6", 10*time.Second); err != nil {
				return err
			}
			e.key("up")
			// 期望列被带到了上一行，穿过短行也不会缩到行首。
			return e.expect("5:6", 10*time.Second)
		}},
		step{name: "下移 3 行仍然保持那一列", fn: func(e *env) error {
			for range 3 {
				e.key("down")
				time.Sleep(40 * time.Millisecond)
			}
			return e.expect("8:6", 10*time.Second)
		}},
		step{name: "按词移动 Ctrl+Right", fn: func(e *env) error {
			e.key("ctrl+right")
			// line8 整行是一个词，走到行尾。
			return e.expect("8:6", 10*time.Second)
		}},
		step{name: "翻页后仍在文档范围内", fn: func(e *env) error {
			e.key("pgdown")
			time.Sleep(150 * time.Millisecond)
			return e.expect(":", 5*time.Second)
		}},
		step{name: "退出并恢复终端", fn: func(e *env) error { return e.stop() }},
	)
	return scenario{name: "导航：方向键、行首行尾、按词、翻页", steps: steps}
}

// ---- 场景 3：撤销重做 ----

func undoRedo() scenario {
	path := "undo.txt"
	return scenario{
		name: "撤销重做：C-a r / C-a y",
		steps: []step{
			{name: "打开空文件", fn: func(e *env) error { return e.start(path, "") }},
			{name: "键入 abcde", fn: func(e *env) error {
				e.sendSlow("abcde", 60*time.Millisecond)
				return e.expect("1:6", 10*time.Second)
			}},
			{name: "一次撤销退掉整串（合并生效）", fn: func(e *env) error {
				e.ctrlA()
				e.send("r")
				return e.expect("已撤销", 10*time.Second)
			}},
			{name: "撤销后光标回到列 1", fn: func(e *env) error {
				return e.expect("1:1", 5*time.Second)
			}},
			{name: "重做恢复整串", fn: func(e *env) error {
				e.ctrlA()
				e.send("y")
				return e.expect("已重做", 10*time.Second)
			}},
			{name: "保存后磁盘上是 abcde", fn: func(e *env) error {
				e.ctrlA()
				e.send("w")
				return e.textEventually(filepath.Join(e.dir, path), "abcde", 10*time.Second)
			}},
			{name: "退出并恢复终端", fn: func(e *env) error { return e.stop() }},
		},
	}
}

// ---- 场景 4：打开文件 ----

func openFile() scenario {
	return scenario{
		name: "打开文件：C-a f 输入路径",
		steps: []step{
			{name: "预先放好目标文件", fn: func(e *env) error {
				file(e.dir, "target.txt", "目标文件内容")
				return nil
			}},
			{name: "C-a f 打开输入浮层", fn: func(e *env) error {
				e.ctrlA()
				e.send("f")
				return e.expect("打开文件", 10*time.Second)
			}},
			{name: "清空预填的当前目录并输入目标路径", fn: func(e *env) error {
				clearPrompt(e)
				target := filepath.Join(e.dir, "target.txt")
				e.sendSlow(target, 12*time.Millisecond)
				return e.expect("target.txt", 10*time.Second)
			}},
			{name: "回车后确认打开成功", fn: func(e *env) error {
				e.key("enter")
				return e.expect("已打开", 10*time.Second)
			}},
			{name: "文件内容出现在屏幕上", fn: func(e *env) error {
				return e.expect("目标文件内容", 10*time.Second)
			}},
			{name: "浮层已关闭", fn: func(e *env) error {
				return e.expectGone("打开文件:")
			}},
			{name: "退出并恢复终端", fn: func(e *env) error { return e.stop() }},
		},
	}
}

// clearPrompt 把输入浮层里预填的内容清空。
//
// 打开文件的浮层会预填当前工作目录，用户要输别的路径就得先清掉。
// 分批写入并留间隔：一次灌太多会挤在同一个读循环里。
func clearPrompt(e *env) {
	for range 12 {
		e.send("\x7f\x7f\x7f\x7f\x7f\x7f\x7f\x7f\x7f\x7f")
		time.Sleep(15 * time.Millisecond)
	}
	e.quiet(200 * time.Millisecond)
}

// ---- 场景 5：目录浏览 ----

func openDirectory() scenario {
	return scenario{
		name: "目录浏览：右箭头打开/进入",
		steps: []step{
			{name: "预先造一个目录和文件", fn: func(e *env) error {
				must(os.Mkdir(filepath.Join(e.dir, "pkg"), 0o755))
				file(e.dir, "readme.md", "说明")
				file(e.dir, "pkg/inner.go", "package pkg")
				return nil
			}},
			{name: "C-a f 后输入目录路径", fn: func(e *env) error {
				e.ctrlA()
				e.send("f")
				if err := e.expect("打开文件", 10*time.Second); err != nil {
					return err
				}
				clearPrompt(e)
				e.sendSlow(e.dir, 12*time.Millisecond)
				e.quiet(200 * time.Millisecond)
				e.key("enter")
				return e.expect("浏览", 10*time.Second)
			}},
			{name: "目录里的文件列出来了", fn: func(e *env) error {
				return e.expect("readme.md", 10*time.Second)
			}},
			{name: "目录也列出来了（带结尾斜线）", fn: func(e *env) error {
				return e.expect("pkg/", 10*time.Second)
			}},
			{name: "下移一项选中 pkg/ 后按右箭头进入", fn: func(e *env) error {
				// 列表顺序是：.. 、pkg/ 、readme.md。
				// 初始停在 ..，下移一次才是 pkg/。
				e.key("down")
				time.Sleep(200 * time.Millisecond)
				e.key("right")
				return e.expect("inner.go", 10*time.Second)
			}},
			{name: "左箭头返回上级", fn: func(e *env) error {
				e.key("left")
				return e.expect("readme.md", 10*time.Second)
			}},
			{name: "Esc 关闭浏览浮层", fn: func(e *env) error {
				e.key("esc")
				e.quiet(300 * time.Millisecond)
				return e.expectGone("浏览:")
			}},
			{name: "退出并恢复终端", fn: func(e *env) error { return e.stop() }},
		},
	}
}

// ---- 场景 6：跳转到行 ----

func gotoLine() scenario {
	return scenario{
		name: "跳转到行：C-a g",
		steps: []step{
			{name: "打开 30 行文件", fn: func(e *env) error { return e.start("goto.txt", numberedContent(30)) }},
			{name: "C-a g 打开浮层并预填当前行号", fn: func(e *env) error {
				e.ctrlA()
				e.send("g")
				return e.expect("跳转到行", 10*time.Second)
			}},
			{name: "清空后输入 15 并回车", fn: func(e *env) error {
				clearPrompt(e)
				e.sendSlow("15", 40*time.Millisecond)
				e.quiet(150 * time.Millisecond)
				e.key("enter")
				return e.expect("跳转到第 15 行", 10*time.Second)
			}},
			{name: "光标落到第 15 行第 1 列", fn: func(e *env) error {
				return e.expect("15:1", 10*time.Second)
			}},
			{name: "屏幕上有第 15 行的内容", fn: func(e *env) error {
				return e.expect("line15", 10*time.Second)
			}},
			{name: "退出并恢复终端", fn: func(e *env) error { return e.stop() }},
		},
	}
}

// numberedContent 生成 n 行「第 i 行」。
func numberedContent(n int) string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("line%d", i+1)
	}
	return strings.Join(lines, "\n") + "\n"
}

// ---- 场景 7：查找 ----

func findInBuffer() scenario {
	return scenario{
		name: "在文件中查找：C-a / 再按 n",
		steps: []step{
			{name: "打开含 3 处匹配的文件", fn: func(e *env) error { return e.start("find.txt", "alpha one\nbeta two\nalpha three\nalpha four\n") }},
			{name: "C-a / 打开查找浮层", fn: func(e *env) error {
				e.ctrlA()
				e.send("/")
				return e.expect("查找", 10*time.Second)
			}},
			{name: "输入 alpha 并回车", fn: func(e *env) error {
				e.sendSlow("alpha", 40*time.Millisecond)
				e.quiet(150 * time.Millisecond)
				e.key("enter")
				return e.expect("找到 alpha", 10*time.Second)
			}},
			{name: "停在第 1 处匹配（第 1 行）", fn: func(e *env) error {
				return e.expect("1:1", 10*time.Second)
			}},
			{name: "C-s 到第 2 处（第 3 行）", fn: func(e *env) error {
				e.key("ctrl+s")
				return e.expect("3:1", 10*time.Second)
			}},
			{name: "再按 C-s 到第 3 处（第 4 行）", fn: func(e *env) error {
				e.key("ctrl+s")
				return e.expect("4:1", 10*time.Second)
			}},
			{name: "按 C-r 回到上一处（第 3 行）", fn: func(e *env) error {
				e.key("ctrl+r")
				return e.expect("3:1", 10*time.Second)
			}},
			{name: "字母 n 仍然能打进文档（不被查找快捷键占用）", fn: func(e *env) error {
				e.sendSlow("n", 60*time.Millisecond)
				// 打完 n 之后不该冒出查找浮层，
				// 而这个 n 必须真的进了文档：光标从 3:1 走到 3:2。
				if err := e.expectGone("查找:"); err != nil {
					return err
				}
				return e.expect("3:2", 10*time.Second)
			}},
			{name: "退出并恢复终端", fn: func(e *env) error { return e.stop() }},
		},
	}
}

// ---- 场景 8：多标签页 ----

func tabs() scenario {
	return scenario{
		name: "多标签页：M-数字切换、顶部标签栏",
		steps: []step{
			{name: "造两个文件", fn: func(e *env) error {
				file(e.dir, "one.txt", "第一个文件")
				file(e.dir, "two.txt", "第二个文件")
				return nil
			}},
			{name: "先打开 one.txt", fn: func(e *env) error {
				e.ctrlA()
				e.send("f")
				if err := e.expect("打开文件", 10*time.Second); err != nil {
					return err
				}
				clearPrompt(e)
				e.sendSlow(filepath.Join(e.dir, "one.txt"), 12*time.Millisecond)
				e.quiet(200 * time.Millisecond)
				e.key("enter")
				return e.expect("已打开", 10*time.Second)
			}},
			{name: "再打开 two.txt，出现第二个标签", fn: func(e *env) error {
				e.ctrlA()
				e.send("f")
				if err := e.expect("打开文件", 10*time.Second); err != nil {
					return err
				}
				clearPrompt(e)
				e.sendSlow(filepath.Join(e.dir, "two.txt"), 12*time.Millisecond)
				e.quiet(200 * time.Millisecond)
				e.key("enter")
				return e.expect("已打开", 10*time.Second)
			}},
			{name: "M-1 切回第一个标签", fn: func(e *env) error {
				e.key("alt+1")
				return e.expect("第一个文件", 10*time.Second)
			}},
			{name: "M-2 切到第二个标签", fn: func(e *env) error {
				e.key("alt+2")
				return e.expect("第二个文件", 10*time.Second)
			}},
			{name: "M-9 跳到最后一个标签（当前就是最后一个）", fn: func(e *env) error {
				e.key("alt+9")
				return e.expect("第二个文件", 10*time.Second)
			}},
			{name: "C-a k 关闭标签", fn: func(e *env) error {
				e.ctrlA()
				e.send("k")
				return e.expect("已关闭", 10*time.Second)
			}},
			{name: "关闭后回到剩下的那个文件", fn: func(e *env) error {
				return e.expect("第一个文件", 10*time.Second)
			}},
			{name: "退出并恢复终端", fn: func(e *env) error { return e.stop() }},
		},
	}
}

// ---- 场景 9：宽字符 ----

func wideCharacters() scenario {
	return scenario{
		name: "中文与宽字符：列号按 rune 算",
		steps: []step{
			{name: "打开一个空文件", fn: func(e *env) error {
				if err := e.start("demo.txt", ""); err != nil {
					return err
				}
				return e.expect("demo.txt", 15*time.Second)
			}},
			{name: "键入中文", fn: func(e *env) error {
				e.sendSlow("你好世界", 70*time.Millisecond)
				// 4 个汉字占 4 列，光标应停在第 5 列。
				return e.expect("1:5", 10*time.Second)
			}},
			{name: "保存后磁盘内容是中文", fn: func(e *env) error {
				e.ctrlA()
				e.send("w")
				if err := e.expect("已保存", 10*time.Second); err != nil {
					return err
				}
				return e.text(filepath.Join(e.dir, "demo.txt"), "你好世界")
			}},
			{name: "退格删掉一个汉字而不是半个字节", fn: func(e *env) error {
				e.key("backspace")
				// 退格后光标应从第 5 列回到第 4 列：
				// 它删的是整个汉字，列号也只退一格。
				if err := e.expect("1:4", 10*time.Second); err != nil {
					return err
				}
				e.ctrlA()
				e.send("w")
				// 轮询磁盘而不是等「已保存」：这句状态文字上一次保存时
				// 已经出现过，等它会立刻返回，于是读到的是旧内容。
				return e.textEventually(filepath.Join(e.dir, "demo.txt"), "你好世", 10*time.Second)
			}},
			{name: "退出并恢复终端", fn: func(e *env) error { return e.stop() }},
		},
	}
}

// ---- 场景 10：只读文件 ----

func readonlyFile() scenario {
	return scenario{
		name: "只读文件：不能编辑并给出提示",
		steps: []step{
			{name: "造一个只读文件并打开", fn: func(e *env) error {
				if err := e.start("ro.txt", "只读内容"); err != nil {
					return err
				}
				if err := os.Chmod(filepath.Join(e.dir, "ro.txt"), 0o444); err != nil {
					return err
				}
				// 重新以只读状态打开。
				if err := e.restart(filepath.Join(e.dir, "ro.txt")); err != nil {
					return err
				}
				return e.expect("只读", 15*time.Second)
			}},
			{name: "键入被拒绝", fn: func(e *env) error {
				e.sendSlow("XYZ", 70*time.Millisecond)
				return e.expect("只读", 10*time.Second)
			}},
			{name: "文件内容没被改动", fn: func(e *env) error {
				return e.text(filepath.Join(e.dir, "ro.txt"), "只读内容")
			}},
			{name: "保存被拒绝", fn: func(e *env) error {
				e.ctrlA()
				e.send("w")
				return e.expect("只读", 10*time.Second)
			}},
			{name: "退出并恢复终端", fn: func(e *env) error { return e.stop() }},
		},
	}
}

// ---- 场景 11：取消浮层 ----

func cancelPrompt() scenario {
	return scenario{
		name: "Esc 取消浮层且不影响文档",
		steps: []step{
			{name: "打开一个空文件", fn: func(e *env) error {
				if err := e.start("demo.txt", ""); err != nil {
					return err
				}
				return e.expect("demo.txt", 15*time.Second)
			}},
			{name: "键入一个字符", fn: func(e *env) error {
				e.send("a")
				return e.expect("1:2", 10*time.Second)
			}},
			{name: "C-a f 打开浮层后按 Esc", fn: func(e *env) error {
				e.ctrlA()
				e.send("f")
				if err := e.expect("打开文件", 10*time.Second); err != nil {
					return err
				}
				e.key("esc")
				return e.expect("已取消", 10*time.Second)
			}},
			{name: "浮层已关闭", fn: func(e *env) error {
				return e.expectGone("打开文件:")
			}},
			{name: "取消之后还能继续编辑", fn: func(e *env) error {
				e.send("b")
				return e.expect("1:3", 10*time.Second)
			}},
			{name: "保存后磁盘上是 ab", fn: func(e *env) error {
				e.ctrlA()
				e.send("w")
				return e.textEventually(filepath.Join(e.dir, "demo.txt"), "ab", 10*time.Second)
			}},
			{name: "退出并恢复终端", fn: func(e *env) error { return e.stop() }},
		},
	}
}

// ---- 场景 12：新建文件 ----

func newFile() scenario {
	return scenario{
		name: "新建文件：路径不存在也能打开并保存",
		steps: []step{
			{name: "先以空文档启动", fn: func(e *env) error {
				if err := e.restartEmpty(); err != nil {
					return err
				}
				return e.expect("未命名", 15*time.Second)
			}},
			{name: "C-a f 输入一个还不存在的路径", fn: func(e *env) error {
				e.ctrlA()
				e.send("f")
				if err := e.expect("打开文件", 10*time.Second); err != nil {
					return err
				}
				clearPrompt(e)
				e.sendSlow(filepath.Join(e.dir, "brand-new.txt"), 12*time.Millisecond)
				e.quiet(200 * time.Millisecond)
				e.key("enter")
				return e.expect("新建", 10*time.Second)
			}},
			{name: "键入内容", fn: func(e *env) error {
				e.sendSlow("brand new", 60*time.Millisecond)
				// 9 个字符（含空格）占 9 列，光标在第 10 列。
				return e.expect("1:10", 10*time.Second)
			}},
			{name: "保存后文件真的被创建出来", fn: func(e *env) error {
				e.ctrlA()
				e.send("w")
				if err := e.expect("已保存", 10*time.Second); err != nil {
					return err
				}
				return e.text(filepath.Join(e.dir, "brand-new.txt"), "brand new")
			}},
			{name: "退出并恢复终端", fn: func(e *env) error { return e.stop() }},
		},
	}
}

// ---- 场景 13：改变窗口尺寸 ----

func resize() scenario {
	return scenario{
		name: "改变窗口尺寸后继续可用",
		steps: []step{
			{name: "打开一个空文件", fn: func(e *env) error {
				if err := e.start("demo.txt", ""); err != nil {
					return err
				}
				return e.expect("demo.txt", 15*time.Second)
			}},
			{name: "键入一些内容", fn: func(e *env) error {
				e.sendSlow("resize test", 60*time.Millisecond)
				return e.expect("1:12", 10*time.Second)
			}},
			{name: "把窗口改成 40x12", fn: func(e *env) error {
				return e.resize(40, 12)
			}},
			{name: "缩小后仍能键入", fn: func(e *env) error {
				e.sendSlow("ok", 60*time.Millisecond)
				return e.expect("1:14", 10*time.Second)
			}},
			{name: "放大回 100x30", fn: func(e *env) error {
				return e.resize(100, 30)
			}},
			{name: "保存后内容完整", fn: func(e *env) error {
				e.ctrlA()
				e.send("w")
				if err := e.expect("已保存", 10*time.Second); err != nil {
					return err
				}
				return e.text(filepath.Join(e.dir, "demo.txt"), "resize testok")
			}},
			{name: "退出并恢复终端", fn: func(e *env) error { return e.stop() }},
		},
	}
}
