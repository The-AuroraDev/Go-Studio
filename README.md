# Go Studio

面向 Go 的终端 IDE。

```bash
make build-release            # 编译出 ./go-studio（带版本号）
./go-studio                   # 打开空白编辑器
./go-studio main.go           # 打开文件
./go-studio -workspace .      # 指定工作区
./go-studio -h                # 全部命令行选项
```

版本号取自 `v0.0.x` 形式的 git tag；没有 tag 时是 `0.0.1-dev`，
工作树不干净时带 `-dirty` 后缀。

### 最常用的几个键

| 按键 | 作用 |
|---|---|
| 直接打字 | 插入（**任何字母都没被绑成快捷键**，随便打） |
| `方向键` / `Home` / `End` | 移动光标 |
| `C-a w` | 保存 |
| `C-a f` | 打开文件；路径是目录就进浏览浮层，右箭头进入 |
| `C-a q` | 退出（有未保存改动会拒绝，并提示 `C-a x` 强退） |
| `C-a r` / `C-a y` | 撤销 / 重做 |
| `C-a /` 然后 `C-s` / `C-r` | 查找、下一处、上一处 |
| `C-a g` | 跳转到行 |
| `C-g` | 中止当前按键前缀 |

按下 `C-a` / `C-g` 之后，编辑器会等下一键组成组合键；
**此时按键不会插进文档**。要退出这个状态按 `C-g`。

## 现在能做什么

- **文本编辑**：输入、删除、撤销重做、缩进、连接行、行号、当前行高亮、
  tab 宽度设置、自动缩进、中文与宽字符正确对齐
- **文件**：`C-a f` 打开文件或目录（目录会进入浏览浮层）、`C-a w` 保存、
  `C-a S` 另存为、`C-a k` 关闭标签
- **导航**：方向键、Home/End、Ctrl+Home/End、按词移动、翻页、`C-a g` 跳转到行
- **查找**：`C-a /` 搜索、`C-s` 下一处、`C-r` 上一处
- **多标签**：`M-1` 到 `M-8` 切换、`M-9` 最后一个、`C-a [` `C-a ]` 相邻切换
- **语法高亮**：按扩展名或文件名自动识别语言，Go 支持得最细
  （导出名、字段、结构体标签都单独着色）；认不出的文件就是纯文本，不报错
- **安全**：只读文件不误改、退出时终端状态完全恢复、二进制文件拒绝以文本打开

完整键位见 [docs/keybindings.md](docs/keybindings.md)（由 `make keys` 从键位表生成）。

### 语法高亮

编辑器是围绕 Go 优化的，但**不是只能写 Go**。目前支持：

| 类别 | 语言 |
| --- | --- |
| Go | Go |
| C 家族 | C、C++、Objective-C、Java、Kotlin、Scala、Swift、Rust、Dart、PHP、Groovy |
| 脚本 | Python、Shell、Lua、Perl |
| 网页 | JavaScript、TypeScript、HTML、XML、CSS |
| 数据 | JSON、TOML、YAML、INI |
| 标记 | Markdown |
| 其他 | SQL |

状态栏会显示识别到的语言名——「为什么这份文件没高亮」基本总是「没认出来」，
把结果摆出来比让人猜省事。

配置（`~/.config/go-studio/config.toml`）：

```toml
[editor]
syntax = true            # 关闭则完全不高亮
syntax_lang = ""         # 强制指定语言名，如 "Go"；空串表示按扩展名判断
```

无扩展名的文件（`Makefile`、`Dockerfile`）按扩展名判断必然落空，
这时用 `syntax_lang` 兜底最省事。

尚未实现的功能按下会在状态栏明确提示「尚未实现」，不会静默无反应。

## 测试

```bash
make test        # 单元 + 属性测试，约 35 秒
make e2e         # 完整使用测试：伪终端里把编辑器当真人用一遍，约 30 秒
make check       # 提交前完整门禁
make cover       # 覆盖率（当前 90.3%）
make help        # 全部目标
```

测试怎么写、有哪些坑，见 [docs/testing.md](docs/testing.md)。

## 构建

需要 Go 1.27 或更高。

```bash
make build            # ./go-studio（开发版）
make build-release    # 带版本号，版本取自 v0.0.x 的 git tag
make keys             # 从键位表重新生成 docs/keybindings.md
```

交叉编译已验证：`GOOS=linux/darwin/freebsd/netbsd/openbsd/windows go build ./...`

> 依赖需要从 `https://goproxy.cn` 拉取。若 `proxy.golang.org` 不通，
> 先执行 `go env -w GOPROXY=https://goproxy.cn,direct`。

## 架构

```
cmd/go-studio      进程入口、装配、信号处理
scripts/e2e        完整使用测试（pty 驱动 + 虚拟终端重建）
scripts/keys       从键位表生成速查表
internal/screen    终端边界：帧 + 事件（Screen 接口 / Bubble Tea / Fake）
internal/keymap    按键 → 命令（纯函数，前缀树）
internal/buffer    文本存储（piece table + 行索引 + 撤销栈）
internal/document  编辑语义：光标、移动、编辑、脏标记、文件读写
internal/view      状态 → ANSI 帧（纯函数）
internal/config    分层配置（TOML）
internal/log       结构化日志（按天切分 + 脱敏 + 轮转）
```

依赖单向向下。`keymap` / `document` / `view` / `buffer` 都不碰终端，
因此可以脱离真实终端完整测试。

许可证见 [License.md](License.md)。