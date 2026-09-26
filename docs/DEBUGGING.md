# 调试指南

Go Studio 的调试入口、日志位置与常见故障处理。面向贡献者与排障人员。

## 一分钟上手

```sh
make deps     # 安装前端依赖并生成 Wails 绑定
make dev      # 启动开发模式（Vite 热更新 + Go 自动重编译）
make check    # 提交前完整门禁
```

## 调试前端

### 打开 DevTools

`make dev` 启动后，在应用窗口内按 `Ctrl+Shift+I`（macOS 为 `Cmd+Option+I`）打开 WebKit 开发者工具。控制台里的报错与 `fetch` 失败都在这里看。

`Ctrl+Shift+P` 之类快捷键在阶段 1 接入后才会生效；阶段 0 只能靠 DevTools 与 `make check`。

### 只调前端，不启动桌面外壳

```sh
cd frontend
pnpm run dev
```

浏览器打开 Vite 输出的地址即可。此时 **Wails 绑定不可用**，`src/services/bridge.ts` 里的调用会失败，这是预期行为，不要为它加 mock。`ThemeProvider` 与 `StatusBar` 已经对绑定失败做了降级，界面仍可渲染。

### 前端测试

```sh
make test-web                       # 全部
cd frontend && pnpm run test:watch  # 监听模式
```

- 纯逻辑测试跑在 node 环境（`src/**/*.test.ts`）
- 组件渲染测试用文件顶部的 `// @vitest-environment jsdom` 声明（`src/**/*.test.tsx`）

`src/tokens/tokens.test.ts` 会逐项比对 `tokens.ts` 与 `tokens.css`。**改了任何一个都必须让两边一致**，否则测试失败。

## 调试 Go 后端

### 日志

日志同时写文件与标准错误（`make dev` 会自动打开标准错误输出）：

| 位置 | 内容 |
|---|---|
| `~/.config/go-studio/logs/go-studio-YYYY-MM-DD.log` | JSON 结构化日志，按天切分 |
| 标准错误 | 仅当 `GO_STUDIO_CONSOLE_LOG=1` |

调整日志级别：改 `~/.config/go-studio/config.json` 的 `logLevel` 为 `debug`，重启应用生效。级别取值 `debug` / `info` / `warn` / `error`。

日志会轮转：单文件超过 8 MB 时改名为 `go-studio-<日期>-<时间>.rotated.log`，保留最近 5 份。

### 日志脱敏

字段名命中 `password`、`secret`、`token`、`apikey`、`authorization`、`credential`、`privatekey`、`kubeconfig`、`cookie`（忽略大小写与分隔符）时，值会被替换为 `***`。脱敏在 `slog.Handler` 层完成，因此经由 `internal/log.Logger` 落盘的记录都被覆盖。

`author` **不在**敏感词表内，`wails.json` 的 `author` 字段能正常记录。

### 用 Delve 调 Go 代码

`wails dev` 本身不带调试器。Wails v2 要求 `main` 包在项目根目录，所以直接调根包，并带上 webkit tag：

```sh
dlv debug . -- -tags webkit2_41
```

连上之后下断点再继续：

```
(dlv) break github.com/29anan29/Go-Studio/internal/app.(*App).Ping
(dlv) continue
```

要调 gopls / Delve 集成（阶段 4、5 之后），把 `dlv` 附加到已经跑起来的进程：

```sh
dlv attach $(pgrep -f 'build/bin/go-studio-dev')
```

## 常见故障

### `wails dev` 报 `listen tcp 127.0.0.1:34115: bind: address already in use`，界面全白

**原因**：Wails 的 dev server 默认绑定 `localhost:34115`（见 wails 源码 `internal/project/project.go`）。如果 `frontend/vite.config.ts` 里也把 Vite 固定到 34115，两者抢同一个端口，dev 会话会直接退出，窗口加载不到任何资源。

**处理**：

1. 确认 `frontend/vite.config.ts` 的 `server` 里**没有** `port: 34115`。用 Vite 自己的默认端口，靠 `wails.json` 的 `"frontend:dev:serverUrl": "auto"` 从 Vite 启动输出里自动发现地址。
2. 端口被别的进程占用时先清掉：

```sh
ss -ltnp | grep 34115
pkill -f 'bin/vite'
```

### `wails doctor` 报 `Fatal: Required dependencies missing: libwebkit`

**这是误报，不要照它去装包。** `doctor` 硬编码检查 `webkit2gtk-4.0`，而 Ubuntu 24.04 以后的发行版只提供 4.1。Wails v2 用 `webkit2_41` build tag 支持 4.1：

- `wails.json` 里 `"build:tags": "webkit2_41"` 已配置
- 裸 `go` 命令需要显式加 tag，Makefile 里是 `GO_TAGS := webkit2_41`

判断能否构建的唯一标准是 `make build` 成功。

### `make check` 里 staticcheck 报 `/usr/local/go/src/...` 错误

staticcheck 0.8.1 尚不能解析 go1.27 标准库，报错全在 Go 自身的源码里。`scripts/staticcheck.sh` 会识别这种情况并降级为告警，硬门禁是 `go vet`。若要真正启用 staticcheck，等它发布支持当前 Go 版本的版本即可，无需改代码。

### `make style` 报中文注释 `long-line`

技能自带的 `check-style.sh` 用 mawk 按**字节**计算长度，中文注释会被算成 3 倍长度而误报。行宽由 `scripts/check-lines.py` 按字符精确校验（`make style` 会先跑它），`check-style.sh` 在本项目里只负责行尾空格、混合缩进、编辑器 modeline 与文件头。

### `error TS2307: Cannot find module '@/...'`

`tsconfig.json` 里配了 `"paths": { "@/*": ["src/*"] }`。如果新加了顶层目录或改了 `baseUrl`，需要同步 `tsconfig.node.json`（它复制了一份同样的别名）。

### 前端 lint 报 “was not found by the project service”

`frontend/eslint.config.js` 用 `project: ['./tsconfig.json', './tsconfig.node.json']` 显式列出项目。新增测试文件时要保证它落在其中一个 tsconfig 的 `include` 范围内，且只落在一个里。

### 改了品牌图标但没生效

品牌图标的唯一真源是 `build/branding/gostudio-mark.svg`，其余全是生成产物：

```sh
python3 -m venv .venv
.venv/bin/pip install cairosvg
.venv/bin/python scripts/render-icons.py   # 或 make icons-render
```

漏了这一步时 `make check` 里的 `check-icons.sh` 会报「缺少品牌衍生物」或「位图尺寸与文件名不一致」。

**改完 SVG 一定要渲染成 PNG 实际看一眼**，不要只看代码。小部件（笔尖这类只有几个单位宽的形状）
会继承外层 `stroke-width` 而被黑色描边涂满；耳朵这类元素会因绘制顺序被身体盖住 —— 这两类问题读代码看不出来。

本机没有 `rsvg-convert` 与 ImageMagick，python3 受 PEP 668 保护，所以必须用 venv。这个 venv 不要提交，
它不是项目依赖；品牌位图本身是提交产物，生成是偶发的设计动作，不进 `make check` 也不进 CI。

### `go mod download` 超时

本机 `GOPROXY` 已设为 `https://goproxy.cn,direct`。注意 `~/.bashrc` 里把 `go` 定义成了注入 socks5 代理的 **shell 函数**，该函数不会传给 `make`、`sh -c` 和 Wails 子进程，所以非交互环境必须依赖 `GOPROXY` 而不是代理环境变量：

```sh
go env -w GOPROXY=https://goproxy.cn,direct
```

### 想从头再来

```sh
make clean
rm -rf frontend/node_modules
wails generate module   # 重新生成前端绑定
```

## 调试 CI

| 工作流 | 触发 | 作用 |
|---|---|---|
| `ci.yml` | push / PR | `make check` 门禁、Linux 构建、deb/rpm 打包冒烟 |
| `codeql.yml` | push / PR / 每周一 | Go 与 TypeScript 静态安全分析 |
| `release.yml` | 推送 `v*` tag | 校验门禁后构建 deb/rpm 并创建 GitHub Release |

### 在本地复现 CI

```sh
make check                 # 对应 verify job
make build                 # 对应 build-linux job
make package               # 对应 package-linux job
```

### 让 CodeQL 与 actionlint 在本地跑

```sh
# 工作流语法检查
actionlint

# CodeQL 规则本地预演（需要 gh CLI 与认证）
gh auth login
gh codeql database create db
gh codeql database analyze db --format=sarif-latest --output=results.sarif
```

### 发布流程

1. 改 `wails.json` 的 `info.productVersion`
2. `make check` 全绿
3. 打 tag：`git tag v1.2.3 && git push origin v1.2.3`
4. `release.yml` 会校验 tag 与 `productVersion` 一致，不一致直接失败
5. 校验通过后构建 deb/rpm，生成 `SHA256SUMS`，创建 GitHub Release

macOS dmg 与 Windows msi 属于阶段 11，届时在 `release.yml` 追加构建矩阵。

## 代码约定速查

| 事项 | 规则 |
|---|---|
| 颜色、尺寸、字体 | 只能用 `src/tokens/tokens.css` 里的 CSS 变量，eslint 会拦截字面量色值 |
| 主题 | `<html data-theme="dark|light">`，三态取值 `dark` / `light` / `system` |
| 配置 | 用户级 `~/.config/go-studio/config.json`，工作区级 `<项目>/.go-studio/config.json`，工作区覆盖用户级 |
| 命令 | 全部注册到命令注册中心，不允许硬编码按钮；危险操作必须 `confirm` |
| 提交前 | `make check` 必须全绿 |
