# 测试指南

本项目的测试分三层，各自有明确的分工。`make check` 是提交前的完整门禁。

## 一、快速上手

```bash
make test          # 日常跑这个：单元 + 属性测试，跳过深度档位（约 35 秒）
make e2e           # 完整使用测试：把编辑器当真人用一遍（约 25 秒）
make test-deep     # 深度属性测试（8.5 分钟，不建议随手跑）
make check         # 完整门禁：fmt / vet / test / cover / bench / race
make cover         # 只看覆盖率
```

只想快速验证某个包（约 11 秒，跳过 pty 端到端测试）：

```bash
go test -short ./internal/...
```

`make run` 会启动编辑器，需要一个真实终端。

## 二、完整使用测试（make e2e）

`internal/*` 的测试验证「某个函数对不对」，`scripts/e2e` 验证
**「一个真人坐下来能不能用」**。两者互补，不能互相替代。

```bash
make e2e              # 全部 13 个场景
make e2e-list         # 只列场景名
scripts/e2e.sh -v     # 打印每个步骤
scripts/e2e.sh -only 查找   # 只跑名字含「查找」的场景
scripts/e2e.sh -keep  # 失败时保留工作目录，便于手工复现
```

它做的事：

1. 编译真实二进制，放进伪终端（100x30）里跑。
2. 按脚本敲键——和真人一样只用终端发字节。
3. 把编辑器输出**重建成一张虚拟屏幕**，在屏幕上断言。
4. 每个场景结束时确认终端 termios 的四个控制字都恢复了。

覆盖的场景：基本编辑与保存、导航、撤销重做、打开文件、目录浏览、
跳转到行、查找、多标签页、中文与宽字符、只读文件、Esc 取消、
新建文件、改变窗口尺寸。

### 为什么要在虚拟屏幕上断言

Bubble Tea 做的是差分渲染，只重写变化的单元格。光标从 1:2 走到 1:6 时，
字节流里看到的是 `…e3 s4 i5 z6` 这样的碎片，`1:6` 作为连续字符串永远不会出现。
断言字节流等于在断言渲染器的内部实现细节；断言重建后的屏幕才是「用户看到了什么」。

`scripts/e2e/screen.go` 是一台极简虚拟终端，实现了渲染器实际用到的那些序列：

| 序列 | 含义 | 不实现的后果 |
|---|---|---|
| `CSI n d` | 纵向绝对定位 | 状态栏画在正文行上，正文整片空白 |
| `CSI n b` | REP 重复上一个字符 | 状态栏出现 `NOR6 L` 这种被削一半的字 |
| `CSI n P` | 删除字符 | 状态栏出现 `1::1` 这种残字 |
| `CSI n @` | 插入字符 | 同上 |
| `CSI n L` / `M` | 插入/删除行 | 布局错位 |
| `CSI t;br` + `CSI n S/T` | 滚动区域 + 上下滚 | 关掉浮层后旧内容不消失 |
| `CSI n H` / `G` / `A`-`D` | 各种光标定位 | 定位错位 |
| `CSI J` / `K` | 清屏/清行 | 残影 |
| `CSI n X` | 擦除字符 | 残影 |

另外要注意 **raw 模式下终端不做自动折行**（DECAWM 关），
虚拟终端也不折行：写满 100 列的行之后渲染器会显式发 CRLF 或重新定位。
若在这里折行，写满的行会折一次、CRLF 又换一次，整屏一行行往上错位。

### 按键序列不能猜

不同终端对同一个功能编码不同，`End` 可能是 `CSI F`、`CSI 4~` 或 `CSI 8~`；
`Ctrl+End` 必须是 `CSI 1;5 F`，光发 `CSI F` 的话编辑器收到的是普通 `<end>`，
Ctrl 那一下根本没带上。所以键表是用探测出来的，不是猜的：

```bash
go test -v -run TestProbeSequences ./internal/screen/
```

它会把每个序列归一化之后的按键名打印出来。`TestProbeSequencesStable`
把最容易回退的几条钉死（空格、Ctrl+空格、退格、Ctrl+End/Right）。

## 三、三层测试的分工

| 层 | 位置 | 验证什么 | 怎么跑 |
|---|---|---|---|
| 单元测试 | 各 `internal/*` 包内 | 单个函数的行为、边界、错误路径 | `go test -short ./internal/...` |
| 属性/差分测试 | `internal/buffer`、`document` | 随机编辑序列下不变量仍然成立 | 同上（默认档位） |
| 端到端（Go 测试） | `cmd/go-studio/*_test.go` | 真实伪终端里程序能跑、终端能恢复 | `go test -short ./cmd/...` |
| 端到端（使用脚本） | `scripts/e2e` | 一个真人能不能把功能用起来 | `make e2e` |

`cmd/go-studio` 的 pty 测试和 `scripts/e2e` 都在伪终端里跑，区别在于：
前者验证单个行为（输入恢复、能否启动），后者跑完整流程并断言用户看到的画面。

端到端测试用 `//go:build !windows` 约束（需要 `creack/pty` 与 termios），
在缺少 `/dev/ptmx` 的环境里会 `t.Skip` 而不是失败。

## 三、按档位的强度

深度档位由两个 build-tagged 常量控制，这是本项目的一个刻意的取舍：

- `norace_steps_test.go`（`//go:build !race`）：`propertySteps = 2000`
- `race_steps_test.go`（`//go:build race`）：`propertySteps = 400`

竞态检测会把执行速度拖慢一个数量级，所以带 `-race` 时步数降到 1/5。
`TestPropertyAgainstNaiveDeep` 固定跑 10000 步，用 `-run Deep` 单独触发。

| 命令 | 实测耗时 | 说明 |
|---|---|---|
| `go test -short ./internal/...` | ~11s | 日常反馈 |
| `go test -short ./...` | ~35s | 含 pty 端到端 |
| `make test`（= 上面的 `-short`） | ~35s | |
| `go test -race -short ./...` | ~40s | 竞态检测 |
| `make test-deep` | **8.5 分钟** | 只在怀疑性能/不变量时跑 |

> `make test-deep` 很慢是因为它对 11 个文档种子各跑 10000 次随机编辑，
> 每次编辑后都做全量不变量校验。日常开发不必跑。

## 四、常用命令

### 跑单个测试或子测试

```bash
go test -run TestName ./internal/keymap/              # 单个测试
go test -run 'TestNavigationKeys/按词右移' ./internal/editor/   # 单个子测试
go test -v -run TestEmacsTableMatchesSpec ./internal/keymap/     # 看断言细节
go test -list '.*' ./internal/buffer/                 # 列出全部测试名
go test -list 'Deep' ./...                            # 只看深度测试有哪些
```

### 覆盖率

```bash
make cover                                           # 总覆盖率 + 每包
go test -short -cover ./internal/screen/             # 单包

go test -short -coverprofile=/tmp/c.out -covermode=atomic ./internal/view/
go tool cover -func=/tmp/c.out                       # 按函数看
go tool cover -func=/tmp/c.out | awk '$3+0 < 60'      # 找出覆盖率低的函数
go tool cover -html=/tmp/c.out -o /tmp/c.html         # 浏览器里看
```

`cmd/go-studio` 会显示 0%：它的测试把编译出的二进制放进伪终端里驱动，
子进程不带插桩。这是有意的——那层验证的是「真实终端里到底能不能用」。

### 基准

```bash
make bench        # 冒烟（benchtime 200ms），门禁用
make bench-full   # 完整基准，要真实性能数字时跑
go test -run '^$' -bench BenchmarkOpenLargeFile -benchmem ./internal/buffer/
go test -run '^$' -bench BenchmarkInsert -benchmem -count=5 ./internal/buffer/  # 多跑几次看波动
```

### 调试

```bash
go test -v -run TestX ./pkg/           # 断言失败时看清上下文
go test -race -run TestX ./pkg/        # 怀疑有竞态时
go test -count=1 -run TestX ./pkg/     # 跳过结果缓存，验证偶发失败
```

交互式调试器（需先装一次）：

```bash
go install github.com/go-delve/delve/cmd/dlv@latest
dlv test ./internal/editor/                     # 断点调试
dlv test -- -test.run TestX -test.v ./pkg/      # 带参数调试
```

## 五、测试约定

这些约定是本项目已经踩过坑之后定下来的，写新测试时请遵守。

### 1. 断言渲染结果前先剥 ANSI

```go
visible := view.VisibleWidth(line)   // 不要用 view.TextWidth
plain := stripANSI(frame.Content)     // view 包里已有生产实现
```

两个原因：
- 转义序列里的 `[38;5;236m` 全是可打印字符，`TextWidth` 会把它们算成可见列，
  凭空多出十几列。
- 光标反显会把一个字符劈成两半（`"package"` 变成 `"p"` + 转义 + `"ackage"`），
  直接搜原文必然找不到。

### 2. 帧宽必须精确等于终端宽度

少一列会露出上一帧残影，多一列会让终端折行。断言时用 `view.VisibleWidth`。

### 3. 属性测试要同时校验内容与不变量

`internal/buffer` 的做法：用 `naive`（一个一眼就对的全量 `[]byte` 实现）当参照，
每次编辑后比对内容、行数、行首偏移；光标类的不变量单独校验光标始终在合法范围。

### 4. 撤销粒度要想清楚

`document.FromString` 保留生产环境的输入合并行为，所以连续输入是一步撤销。
要逐步验证撤销的单测请用包内的 `newDocument(..., 0)` 关闭合并。

### 5. 平台约束

用到 `unix.TCGETS`、`creack/pty` 的测试文件必须加 `//go:build !windows`。

### 6. 全局状态要隔离

`internal/editor` 的剪贴板是包级变量，测试之间要用 `clipboard = nil` 复位。

### 7. 不要把字母绑成快捷键

本编辑器没有模式切换：字母按下去就是往文档里插字符。
任何一个裸字母都不能绑到命令上，否则用户打不出包含那个字母的单词。
`internal/keymap` 的 `TestNoPlainLetterIsBound` 会守住这条。

### 8. 保存类断言要轮询文件，不要等状态文字

状态文字是瞬时的：文字已经出现不等于写入已经落盘；而上一次保存留下的
同一个文字还在屏幕上时，等它会立刻返回，于是读到的是旧内容。
用 `e.textEventually(path, want, timeout)` 轮询磁盘本身。

### 9. 端到端测试要逐键等待并剥 ANSI

Bubble Tea 做的是差分渲染，只重写变化的单元格，累积输出里整串字符永远不会连续出现。
`cmd/go-studio/editor_pty_test.go` 里的 `typeOne` 逐键等待，`waitForOutput` 先剥 ANSI。

## 六、覆盖率现状

`make check` 全绿时总覆盖率 88.4%：

| 包 | 覆盖率 | 说明 |
|---|---|---|
| `screen` | 96.3% | 消息翻译层、后端生命周期、termios 恢复都已覆盖 |
| `keymap` | 97.6% | 键位表与匹配器状态机 |
| `buffer` | 97.0% | 差分属性测试 |
| `browser` | 93.5% | 目录浏览浮层 |
| `document` | 93.3% | 光标、编辑、脏标记、文件读写 |
| `view` | 98.2% | 渲染、宽度裁剪、剥转义 |
| `editor` | 87.1% | 主循环、命令分派、浮层 |
| `log` | 84.0% | |
| `config` | 81.7% | |
| `cmd/go-studio` | 0% | 二进制子进程，无插桩（见上） |

## 七、发现 bug 的标准流程

1. 先写一个失败的测试复现它（哪怕很小）。
2. 修代码，让它变绿。
3. 检查这个 bug 属于哪一类，看看同类代码有没有同样的问题。
   本项目的多数 bug 都是同一类问题的不同表现，例如：
   - 「按字符个数而不是显示宽度」→ 状态栏、标签栏、浮层都中招。
   - 「用带 ANSI 的串去量宽度」→ 补齐逻辑全部算错。
4. 在 `AIdocs/progress.md` 里记一笔，说明为什么这么改。