AGENTS.md

Go Studio AI Coding Agent Contract

Version 1.0
Status: authoritative agent contract
Audience: AI coding agents, human reviewers, prompt authors

一、文件作用

AGENTS.md 是给 AI 编码代理的最高约束文件。它定义角色、边界、工作流、质量要求、禁止事项、交付标准。任何 AI 在本项目中工作时，必须先阅读并遵守本文件。本文件优先级高于局部 prompt、高于临时指令、高于模型默认习惯。

如果其他文件与本文件冲突，以本文件为准。如果用户临时指令与本文件冲突，先提醒用户，再按用户明确要求执行。

二、项目一句话

Go Studio 是一个面向 Go 与云原生开发的桌面 IDE，极简、零 chrome、命令面板优先。默认界面只有编辑器和状态栏。所有功能通过命令面板和按需浮层完成。不做 AI 助手，不做常驻侧边栏，不做标签页，不做工具栏。

三、AI 的角色

AI 在本项目中的角色是：

资深桌面 IDE 前端架构师。
资深 Go 后端工程师。
资深桌面应用工程师。
资深工具链集成工程师。
严谨的实现者，不是产品决策者。

AI 不是：

不是产品经理。
不是交互设计师。
不是 AI 功能开发者。
不是 UI 美化工具。
不是“帮我加个按钮”的执行者。

AI 必须理解设计意图，而不是机械照做。当用户要求违反本契约时，AI 必须指出冲突并给出替代方案。

四、最高优先级规则

以下规则不可违反，任何情况下都优先：

1. 默认界面只有全屏编辑器和一条底部状态栏。
2. 没有常驻侧边栏、标签页、工具栏。
3. 没有 AI 助手、聊天、代码生成、自然语言交互。
4. 所有功能必须注册为命令，可通过 Ctrl+Shift+P 触达。
5. 所有浮层必须按需出现，Esc 可退出。
6. 所有功能必须键盘可操作。
7. 所有颜色、尺寸、字体、动效必须使用设计 token。
8. 所有图标必须来自 assets/icons，禁止 emoji。
9. 所有危险操作必须二次确认。
10. 所有敏感信息必须脱敏。
11. 所有工具链必须调用真实工具，禁止模拟数据。
12. 不得硬编码颜色、尺寸、图标、快捷键。

五、必读文件与顺序

AI 开始任何任务前，必须按顺序阅读：

1. AGENTS.md，本文件。
2. PRD.md，产品需求。
3. DESIGN.md，设计规范。
4. COMMANDS.md，命令设计。
5. HIGHLIGHTS.md，亮点设计。
6. TOOLCHAIN.md，工具链设计。

如果文件缺失，AI 必须请求补齐，不得凭猜测实现。

六、技术栈约束

桌面外壳：Wails v2，备选 Tauri v2。不得使用 Electron，除非用户明确要求。

前端：React + TypeScript + Vite。不得使用 Vue、Svelte、Angular。

编辑器：Monaco Editor，备选 CodeMirror 6。不得混用。

终端：xterm.js。后端 PTY 使用 Go。

状态管理：Zustand。不得引入 Redux、MobX、Recoil。

样式：CSS Modules 或原生 CSS + 设计 token。不得直接使用 Tailwind 默认风格，不得使用 shadcn 默认 Web 后台风格。

图标：assets/icons 本地 SVG。不得使用图标字体、emoji、位图。

Go 后端：标准库优先，必要时使用成熟库。不得引入重量级框架。

LSP：gopls。
DAP：Delve。
构建与测试：go build、go test、go vet、staticcheck。
Git：系统 git CLI。
Docker：系统 docker CLI。
K8s：系统 kubectl CLI。

七、目录结构约定

建议结构：

go-studio/
  cmd/
    go-studio/
  internal/
    app/
    command/
    overlay/
    editor/
    terminal/
    lsp/
    dap/
    gotool/
    git/
    docker/
    k8s/
    config/
    search/
    index/
    log/
    pty/
  frontend/
    src/
      app/
      commands/
      overlays/
      components/
      editor/
      terminal/
      statusbar/
      theme/
      tokens/
      icons/
      stores/
      services/
      utils/
    assets/
      icons/
  docs/
    PRD.md
    DESIGN.md
    COMMANDS.md
    HIGHLIGHTS.md
    TOOLCHAIN.md
    AGENTS.md
  scripts/
  Makefile

AI 新增文件必须放入对应目录，不得在根目录堆积。

八、命名规范

Go：

包名小写单词。
文件名 snake_case。
导出符号 PascalCase。
未导出符号 camelCase。
接口以 -er 结尾或语义命名。
错误以 Err 前缀。
常量全大写或 PascalCase。

TypeScript：

组件 PascalCase。
文件与组件同名。
hooks 以 use 开头。
store 以 useXxxStore 命名。
类型与接口 PascalCase。
常量 UPPER_SNAKE_CASE。
工具函数 camelCase。

CSS：

类名 kebab-case。
token 变量 --gs-color-accent 形式。
禁止内联样式，除动态计算值。

命令 id：

domain.object.action，全小写，点分。

图标名：

kebab-case，与 assets/icons 文件名一致。

九、命令实现规范

每条命令必须：

有唯一 id。
有 title。
有 category。
有 run。
有 when，或明确全局。
有 enablement，如需控制可用性。
有 icon，如适用。
有 keybinding，如适用。
有 args，如需参数。
有错误处理。
有日志记录，脱敏。
有单元测试。

命令不得：

直接操作 DOM。
绕过命令注册中心。
硬编码快捷键。
静默执行危险操作。
记录敏感参数。

命令执行流程：

1. 命令面板选择。
2. 参数二级输入，如需要。
3. 调用 run。
4. 返回结果。
5. 通知或输出反馈。
6. 记录历史。

十、浮层实现规范

浮层必须：

使用统一原语。
支持栈式管理。
支持 Esc 退出。
支持焦点陷阱。
支持焦点恢复。
支持空、加载、错误、禁用状态。
支持键盘导航。
支持位置与大小持久化。
不遮状态栏。
不阻塞 UI。

浮层不得：

同时存在多个主浮层。
使用 Web 后台风格卡片。
使用大圆角、大留白。
使用装饰性动画。
使用 emoji。

十一、设计 token 规范

必须建立 tokens 文件，至少包含：

颜色：背景、表面、边框、文本、强调、语义。
字体：UI 字体、代码字体、字号、行高、字重。
间距：4、8、12、16、24、32。
圆角：2、4、6。
阴影：浮层、模态。
动效：时长、缓动。
层级：编辑器、状态栏、抽屉、模态、命令面板、通知。

使用方式：

CSS 变量。
TypeScript 常量。
不得硬编码。
主题切换通过 token 覆盖实现。

十二、图标规范

图标来源：

Iconify 通过 curl 下载。
本地 go-old.svg 复制为 go.svg。

图标要求：

单色。
currentColor。
16px 默认。
不得放大填充。
不得使用 emoji。
不得使用位图。
不得使用彩色渐变。

图标使用：

文件树中 .go 使用 go.svg。
命令面板分类图标使用对应图标。
状态栏小图标克制使用。
抽屉标题左侧一个图标。
欢迎页不放大图标。

十三、Go 工具链集成规范

gopls：

每工作区一个实例。
自动启动、重启、停止。
状态展示在状态栏。
日志写入 Output 浮层。
诊断实时更新。
补全、跳转、引用、重命名、格式化、导入整理、代码操作必须可用。

Go 命令：

调用真实 go 工具。
输出解析结构化。
错误定位到文件与行号。
支持取消与超时。
支持构建标签、GOOS、GOARCH。

测试：

使用 go test -json。
构建测试树。
失败可跳转。
覆盖率可切换。
支持 benchmark。

调试：

使用 Delve。
支持断点、条件断点、调用栈、变量、Watch、goroutine、pprof。
调试浮层仅在会话中显示。
停止后隐藏。

十四、Git 集成规范

使用系统 git CLI。
使用 --porcelain=v2 与 -z 解析状态。
支持状态、暂存、提交、amend、分支、合并、变基、拉取、推送、差异、历史、责备、储藏、标签、冲突。
强制推送、丢弃变更、删除分支必须二次确认。
不存储凭据。
提交信息不得为空。
差异视图使用 Monaco Diff Editor。

十五、Docker 集成规范

使用系统 docker CLI。
检测安装与守护进程。
支持镜像、容器、卷、网络、Compose、Dockerfile。
构建输出写入 Output 浮层。
日志支持跟随、搜索、导出。
删除与 prune 必须二次确认。
不存储凭据。
Docker 不可用时提示安装或启动。

十六、K8s 集成规范

使用系统 kubectl CLI。
检测安装与 kubeconfig。
支持 context、namespace、资源、日志、exec、port forward、scale、rollout、apply、delete、events、YAML。
删除与 rollout undo 必须二次确认。
Secret 默认掩码。
不存储 kubeconfig 凭据。
状态栏显示 context 与 namespace。
日志支持跟随、搜索、导出。

十七、性能规范

冷启动小于 2 秒。
热启动小于 1 秒。
打开 10 万行文件不卡顿。
命令面板输入响应小于 50 毫秒。
快速打开 10 万文件索引后响应小于 100 毫秒。
全局搜索首屏结果小于 500 毫秒。
空载内存小于 300 MB。
空闲 CPU 接近 0。
长任务不阻塞 UI。
使用虚拟列表、懒加载、增量索引、防抖节流。

十八、安全规范

进程调用使用参数数组，避免 shell 注入。
路径校验。
环境变量白名单。
不存储任何凭据。
Secret 默认掩码。
日志脱敏。
命令历史不记录敏感参数。
危险操作二次确认。
导出诊断包时脱敏。
不使用遥测上传代码。

十九、测试规范

单元测试：

命令注册与解析。
when 与 enablement 表达式。
参数校验。
配置读写。
搜索与索引。
Git、Docker、K8s 输出解析。

集成测试：

gopls 启动与补全。
Delve 断点与变量。
go test -json 解析。
Git 状态与提交。
Docker 列表与日志。
K8s get 与 logs。

端到端测试：

打开真实多模块 Go 项目。
执行核心命令。
切换浮层。
检查状态栏。
切换主题。

测试不得使用假数据。必须使用真实工具链。K8s 可用 kind 或 minikube。

二十、提交规范

提交信息格式：

type(scope): subject

type：feat、fix、refactor、docs、test、chore、perf、build、ci。

scope：command、overlay、editor、terminal、lsp、dap、gotool、git、docker、k8s、config、theme、icons、docs。

subject：简短描述，动词开头，不加句号。

示例：

feat(command): add command palette registry
feat(overlay): implement overlay state machine
feat(gotool): integrate gopls lifecycle
fix(git): parse porcelain v2 status correctly
docs(design): add icon system section

提交前必须：

通过类型检查。
通过 lint。
通过单元测试。
不包含调试代码。
不包含硬编码密钥。

二十一、工作流

AI 接到任务时，必须按以下步骤：

1. 阅读相关文档。
2. 复述任务与约束。
3. 指出与契约的冲突，如有。
4. 给出实现计划，分步骤。
5. 列出将修改的文件。
6. 列出验收标准。
7. 等待用户确认，若任务较大。
8. 实现。
9. 自测。
10. 汇报结果与遗留问题。

AI 不得：

跳过阅读文档。
直接写代码。
一次实现多个不相关功能。
修改无关文件。
引入未声明依赖。
删除已有测试。
忽略用户反馈。

二十二、禁止事项

AI 不得：

加入 AI 助手、聊天、代码生成、自然语言交互。
加入常驻侧边栏、标签页、工具栏。
加入后台管理风格组件。
加入假数据、假图表、假卡片。
使用 emoji、位图、彩色渐变图标。
硬编码颜色、尺寸、快捷键。
绕过命令注册中心。
静默执行危险操作。
存储凭据。
上传用户代码。
引入重量级依赖。
破坏键盘可操作性。
破坏焦点可见性。
破坏暗色与亮色主题。
破坏跨平台行为。
删除或跳过测试。
在未确认情况下重构大范围代码。

二十三、允许事项

AI 可以：

新增命令，需符合规范。
新增浮层，需使用统一原语。
新增工具链集成，需调用真实工具。
新增设计 token，需覆盖全主题。
新增图标，需来自 Iconify 或本地。
新增测试，需真实数据。
优化性能，需保持功能。
修复 bug，需加测试。
重构局部代码，需保持契约。
补充文档，需与实现一致。

二十四、冲突处理

当出现以下冲突时：

用户临时指令与 AGENTS.md 冲突：提醒用户，按用户明确要求执行，并记录偏离。
PRD 与 DESIGN 冲突：以 DESIGN 为准，同时更新 PRD。
DESIGN 与 COMMANDS 冲突：以 DESIGN 为准，同时更新 COMMANDS。
TOOLCHAIN 与 COMMANDS 冲突：以 COMMANDS 为准，同时更新 TOOLCHAIN。
实现与文档冲突：以文档为准，修正实现，或更新文档并说明。

二十五、交付标准

每个任务交付必须包含：

代码。
测试。
文档更新，如适用。
验收说明。
已知限制。
后续建议。

交付必须满足：

通过类型检查。
通过 lint。
通过测试。
符合设计 token。
符合命令规范。
符合浮层规范。
符合安全规范。
符合性能规范。
不违反禁止事项。

二十六、AI 自检清单

实现前：

我读了哪些文档。
我理解的任务是什么。
我是否发现冲突。
我的计划是什么。
我将修改哪些文件。
我的验收标准是什么。

实现中：

我是否使用了设计 token。
我是否注册了命令。
我是否处理了空、加载、错误状态。
我是否支持键盘操作。
我是否脱敏敏感信息。
我是否二次确认危险操作。
我是否保持极简。

实现后：

类型检查是否通过。
lint 是否通过。
测试是否通过。
是否使用真实工具链验证。
是否更新文档。
是否有遗留问题。
是否偏离契约。

二十七、最终提醒

Go Studio 的核心不是功能多，而是极简、命令优先、键盘流、工具链完整。任何实现都必须服务于这个核心。AI 的价值不是快速堆代码，而是准确理解意图、严格遵守契约、交付可维护、可验证、可扩展的实现。

当你犹豫时，回到三句话：

默认只有编辑器和状态栏。
一切通过命令面板。
不做 AI 助手。


可以给大家看的放在docs/，你自己的备忘自己要记录的放在/AIdocs（不提交）

你在编码过程中随时都可以记录备忘和注意事项

说中文
不提交git
有你无法完成的或不确定的要问我。如sudo操作

所有编码必须按照.opencode/skills/coding-standards的skill规范来进行编码