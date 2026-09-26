提示词模板

Prompt Templates

Go Studio 给 AI 下任务时直接复制使用的提示词

Version 1.0
Status: operational prompt library
Audience: prompt authors, tech leads, AI coding agents

一、文件作用

本文件提供一组可直接复制给 AI 的提示词模板。每个模板对应一个明确的阶段或任务。使用时，先复制通用头部，再复制对应任务模板，最后附上相关文档。

原则：一次只下一个任务。任务必须明确、可验收、可回退。不得让 AI 同时做多个阶段。

二、通用头部（每次必带）

你现在为 Go Studio 项目工作。Go Studio 是一个面向 Go 与云原生开发的桌面 IDE，极简、零 chrome、命令面板优先。默认界面只有全屏编辑器和一条底部状态栏。所有功能通过 Ctrl+Shift+P 命令面板触发。不做 AI 助手，不做常驻侧边栏，不做标签页，不做工具栏。

技术栈：Wails v2，React + TypeScript + Vite，Monaco Editor，xterm.js，Zustand，Go 后端，gopls，Delve，系统 git、docker、kubectl。

开始前，请阅读并按顺序遵守：
1. AGENTS.md
2. PRD.md
3. DESIGN.md
4. COMMANDS.md
5. HIGHLIGHTS.md
6. TOOLCHAIN.md
7. IMPLEMENTATION.md
8. WELCOME.md

最高优先级规则，不可违反：
1. 默认界面只有编辑器和状态栏。
2. 无常驻侧边栏、标签页、工具栏。
3. 无 AI 助手、聊天、代码生成。
4. 所有功能注册为命令，可通过 Ctrl+Shift+P 触达。
5. 所有浮层按需出现，Esc 可退出。
6. 所有功能键盘可操作。
7. 所有颜色、尺寸、字体、动效使用设计 token。
8. 所有图标来自 assets/icons，禁止 emoji。
9. 危险操作二次确认。
10. 敏感信息脱敏。
11. 工具链调用真实工具，禁止模拟数据。
12. 不得硬编码颜色、尺寸、图标、快捷键。

工作流：
1. 复述任务与约束。
2. 指出与契约的冲突，如有。
3. 给出实现计划，分步骤。
4. 列出将修改的文件。
5. 列出验收标准。
6. 等待我确认，若任务较大。
7. 实现。
8. 自测。
9. 汇报结果与遗留问题。

现在执行下面的任务。

三、阶段 0 任务模板

任务：项目骨架与基础设施

目标：建立可运行的 Wails v2 空壳，前后端通信打通，设计 token 与图标系统就位。

交付：
1. Wails v2 项目初始化。
2. Go 后端目录结构，按 AGENTS.md 约定。
3. React + TypeScript + Vite 前端。
4. Wails 绑定示例，前后端 ping。
5. 设计 token 文件，颜色、字体、间距、圆角、阴影、动效。
6. 主题切换基础，暗色与亮色。
7. assets/icons 目录，下载 git、docker、kubernetes 等图标。
8. 复制 /home/anan/programming-languages-logos/go-old/go-old.svg 为 assets/icons/go.svg。
9. 图标加载组件。
10. 基础布局，全屏编辑器容器加底部状态栏占位。
11. 日志系统，写入 ~/.config/go-studio/logs/。
12. 配置系统，用户级 ~/.config/go-studio/settings.json 与工作区级 .go-studio/settings.json。
13. Makefile 或脚本，构建、运行、测试、lint。

验收：
应用可启动，显示空编辑器与状态栏。前后端通信正常。主题可切换。图标可加载。配置文件可读写。日志可写入。类型检查与 lint 通过。

约束：不得加入 AI 助手、常驻侧边栏、标签页、工具栏。不得使用假数据。不得硬编码颜色尺寸。

四、阶段 1 任务模板

任务：命令系统与命令面板

目标：实现命令注册中心与命令面板，作为后续所有功能的基础。

交付：
1. 命令数据模型，id、title、category、icon、keybinding、when、enablement、args、run、source、tags。
2. 命令注册中心。
3. 命令执行器，支持同步、异步、取消、错误。
4. when 表达式解析器。
5. 参数系统，string、number、boolean、enum、file、folder、pick。
6. 模糊搜索引擎，标题、id、category、tags 加权。
7. 最近使用与常用排序，持久化到 command-history.json。
8. 命令面板浮层，Ctrl+Shift+P。
9. 前缀模式，> 命令、@ 符号、# 全局搜索、: 行号。
10. 分类分组与图标。
11. 快捷键展示。
12. 空、加载、错误、无结果状态。
13. 键盘操作，上下、Ctrl+N/P、Enter、Esc、Backspace 返回。
14. 内置基础命令，view.commandPalette、file.save、file.open、workspace.open、help.welcome、settings.open。

验收：
Ctrl+Shift+P 打开命令面板。可搜索并执行基础命令。参数命令可二级输入。最近使用与常用排序正确。快捷键展示正确。键盘完全可操作。单元测试覆盖注册、解析、搜索、执行、错误。

约束：所有后续功能必须通过命令注册，不得硬编码按钮。命令必须有 when 条件。命令面板必须先于其他浮层实现。

五、阶段 2 任务模板

任务：编辑器与状态栏

目标：编辑器可用，状态栏作为指挥中心。

交付：
1. Monaco Editor 集成。
2. 全屏编辑器布局。
3. 多光标、查找替换、撤销重做、折叠、行号、当前行高亮。
4. 分屏编辑，最多四组，命令触发。
5. 文件打开、保存、另存为、全部保存。
6. 快速打开，Ctrl+P。
7. 全局搜索，Ctrl+Shift+F。
8. 符号搜索，Ctrl+Shift+O。
9. 行号跳转，Ctrl+G。
10. 历史前进后退，Alt+左/右。
11. 最近文件，Ctrl+Tab。
12. 状态栏字段：文件、Git 分支占位、Go 版本、gopls 状态占位、诊断数占位、行列、编码、换行符、调试状态占位、构建状态占位。
13. 状态栏点击映射命令。
14. 状态栏窄窗口折叠。
15. 自动保存可配置。
16. 大文件模式。

验收：
打开真实 Go 文件可编辑。Ctrl+P 快速打开文件。Ctrl+Shift+F 全局搜索可用。状态栏字段可点击并触发命令。分屏可用。快捷键可用。暗色与亮色主题正常。

约束：不得加入常驻标签页。不得使用后台管理风格组件。Monaco 按需加载。

六、阶段 3 任务模板

任务：浮层状态机与通用浮层

目标：统一浮层原语，后续所有面板基于此。

交付：
1. 浮层状态机，栈式管理。
2. 浮层原语，模态、抽屉、面板、通知。
3. 焦点陷阱与焦点恢复。
4. Esc 逐层退出。
5. 点击外部关闭可配置。
6. 位置、大小、上次状态持久化。
7. 通用浮层组件：命令面板、快速打开、全局搜索、符号搜索、问题、输出、通知、确认对话框。
8. 问题浮层，Ctrl+Shift+M。
9. 输出浮层，通道选择器。
10. 确认对话框，危险操作使用。
11. 通知浮层，成功、警告、错误、信息。

验收：
浮层互斥、可叠加、可退出。焦点正确。状态持久化。问题与输出浮层可用。确认对话框可用。通知可用。

约束：同一时间只允许一个主浮层。抽屉不遮状态栏。浮层必须键盘可操作。

七、阶段 4 任务模板

任务：Go 工具链基础

目标：gopls、构建、运行、测试可用。

交付：
1. GoplsService，启动、停止、重启、日志、状态。
2. LSP 客户端，补全、签名、悬停、定义、类型定义、实现、引用、重命名、诊断、格式化、导入整理、代码操作、文档符号、工作区符号、语义高亮、内联提示。
3. GoToolService，调用 go 命令。
4. 构建命令，go.build.package、go.build.file、go.build.cross、go.build.tags.toggle。
5. 运行命令，go.run.file、go.run.package。
6. 模块命令，go.mod.tidy、go.mod.add、go.mod.remove、go.mod.list、go.mod.graph、go.env.show。
7. 测试命令，go.test.package、go.test.file、go.test.cursor、go.test.workspace、go.bench.package、go.bench.cursor、go.test.coverage.toggle、go.test.rerun、go.test.stop。
8. 测试结果浮层，树状展示、失败跳转、覆盖率。
9. 输出通道 Build、Test、gopls。
10. 状态栏 gopls 状态与诊断数实时更新。

验收：
打开真实多模块 Go 项目。gopls 补全、跳转、诊断可用。go build、go run、go test 可用。测试结果树可用，失败可跳转。覆盖率可切换。状态栏实时更新。

约束：必须调用真实 gopls 与 go 工具链。不得模拟数据。必须处理进程生命周期、日志、错误、取消。

八、阶段 5 任务模板

任务：Delve 调试

目标：Go 调试可用。

交付：
1. DelveService，启动、附加、停止、重启。
2. DAP 客户端。
3. 调试命令，debug.start.package、debug.start.file、debug.start.test、debug.attach、debug.stop、debug.restart、debug.continue、debug.pause、debug.step.over、debug.step.into、debug.step.out。
4. 断点，行断点、条件断点、命中次数、日志断点、清除全部。
5. 调用栈、变量、Watch、Goroutines、控制台。
6. pprof 命令，go.pprof.show。
7. 调试浮层，右侧抽屉，仅在会话中显示。
8. 快捷键，F5、F10、F11、Shift+F11、F9。
9. 调试状态栏指示。

验收：
启动调试，命中断点。调用栈、变量、goroutine 可见。条件断点可用。pprof 可用。停止后浮层隐藏。

约束：调试浮层仅在会话中显示。必须与 gopls 并发安全。

九、阶段 6 任务模板

任务：终端与任务系统

目标：交互式终端与任务运行器。

交付：
1. TerminalService，Go PTY。
2. xterm.js 集成。
3. 终端浮层，Ctrl+`。
4. 多终端、分屏、重命名、关闭、清屏、清历史、复制、粘贴、搜索、链接识别。
5. Shell 集成，Windows PowerShell、cmd、WSL，macOS zsh，Linux bash。
6. 任务系统，任务定义、运行、停止、重启、输出、退出码。
7. 内置任务，go build、go run、go test、go vet、staticcheck、go generate、docker build、docker compose up、kubectl apply。
8. 任务输出写入 Output 浮层。
9. 任务命令，task.run、task.stop、task.restart、view.output。

验收：
终端可交互，能跑 go run。多终端可用。任务可运行、取消、重启。输出正确。

约束：终端是抽屉，不遮状态栏。任务不得阻塞 UI。

十、阶段 7 任务模板

任务：Git 集成

目标：Git 全流程可用。

交付：
1. GitService，调用系统 git CLI。
2. 状态解析，--porcelain=v2 与 -z。
3. 命令，git.status、git.stage.file、git.stage.all、git.unstage.file、git.unstage.all、git.discard、git.commit.create、git.commit.amend、git.push、git.push.force、git.pull、git.fetch、git.branch.checkout、git.branch.create、git.branch.delete、git.branch.merge、git.branch.rebase、git.diff.show、git.history.file、git.history.line、git.blame.file、git.stash.create、git.stash.apply、git.stash.pop、git.stash.drop、git.conflict.resolve、git.merge.tool、git.tag.create、git.tag.delete。
4. Git 浮层，Ctrl+Shift+G，左侧抽屉或中间模态。
5. 差异视图，Monaco Diff Editor。
6. 状态栏分支与脏标记。
7. 危险操作二次确认。

验收：
真实仓库可查看状态、暂存、提交、分支、差异。冲突可解决。强制推送、丢弃变更、删除分支有确认。不存储凭据。

约束：使用系统 git CLI。不存储密码。提交信息不得为空。

十一、阶段 8 任务模板

任务：Docker 集成

目标：Docker 与 Compose 可用。

交付：
1. DockerService，调用系统 docker CLI。
2. 检测安装与守护进程。
3. 命令，docker.image.build、docker.container.run、docker.container.stop、docker.container.start、docker.container.restart、docker.container.remove、docker.container.logs、docker.container.exec、docker.container.stats、docker.image.list、docker.image.pull、docker.image.push、docker.image.remove、docker.image.tag、docker.image.history、docker.volume.list、docker.volume.create、docker.volume.remove、docker.network.list、docker.network.create、docker.network.remove、docker.compose.up、docker.compose.down、docker.compose.restart、docker.compose.logs、docker.prune、docker.info。
4. Docker 浮层，右侧或底部抽屉，分区容器、镜像、卷、网络、Compose。
5. Dockerfile 语法高亮、构建参数、目标阶段选择。
6. 构建输出写入 Output 浮层，通道 Docker。
7. 日志跟随、搜索、导出。
8. 删除与 prune 二次确认。
9. 状态栏 Docker 可用指示。

验收：
真实 Docker 环境可用。构建镜像、运行容器、查看日志、进入 shell。Compose up/down/logs 可用。危险操作有确认。

约束：使用系统 docker CLI。不存储凭据。Docker 不可用时提示安装或启动。

十二、阶段 9 任务模板

任务：Kubernetes 集成

目标：K8s 资源与操作可用。

交付：
1. K8sService，调用系统 kubectl CLI。
2. 检测安装与 kubeconfig。
3. 命令，k8s.context.set、k8s.namespace.set、k8s.context.list、k8s.apply.file、k8s.apply.dir、k8s.delete、k8s.pod.list、k8s.deployment.list、k8s.service.list、k8s.configmap.list、k8s.secret.list、k8s.namespace.list、k8s.node.list、k8s.describe、k8s.pod.logs、k8s.pod.logs.follow、k8s.pod.exec、k8s.portforward.start、k8s.portforward.stop、k8s.deployment.scale、k8s.rollout.restart、k8s.rollout.status、k8s.rollout.undo、k8s.events.show、k8s.resource.yaml、k8s.resource.edit、k8s.metrics.show。
4. K8s 浮层，右侧或底部抽屉，分区 Contexts、Namespaces、Workloads、Services、Config、Storage、Nodes、Events。
5. 资源列表过滤、搜索、排序。
6. 资源详情 YAML、事件、日志、指标。
7. 日志跟随、搜索、导出。
8. Exec 使用 xterm.js。
9. Port Forward 列表与停止。
10. 状态栏 context、namespace、连通性。
11. 删除与 rollout undo 二次确认。
12. Secret 默认掩码。

验收：
真实或 kind/minikube 集群可用。切换 context 与 namespace。apply、get、describe、logs、exec、port-forward、scale、rollout 可用。危险操作有确认。Secret 掩码。

约束：使用系统 kubectl CLI。不存储 kubeconfig 凭据。Secret 默认掩码。

十三、阶段 10 任务模板

任务：设置、主题、键位、持久化

目标：可配置、可持久化、可恢复。

交付：
1. 设置浮层，Ctrl+,，搜索式配置。
2. 用户级与工作区级配置。
3. 配置项覆盖 gopls、go、git、docker、kubectl、Delve、终端、格式化、lint、构建标签、GOOS、GOARCH、测试超时、调试参数、主题、键位、浮层位置。
4. 主题系统，暗色、亮色、跟随系统。
5. 键位系统，默认、Vim、Emacs，冲突检测，覆盖。
6. 布局持久化，浮层位置与大小、分屏、编辑器状态、光标位置、Git 面板状态、Docker 与 K8s 选择。
7. 工作区恢复，崩溃恢复，自动保存。
8. 配置导入导出。
9. 设置重置需确认。

验收：
设置可搜索、可修改、即时生效。主题切换正常。键位可覆盖，冲突提示。重启后布局恢复。崩溃后可恢复。

约束：配置必须异步原子写。不存储敏感信息。配置校验错误提示。

十四、阶段 11 任务模板

任务：性能优化、跨平台、打包、自动更新

目标：达到性能指标，跨平台可用，可分发。

交付：
1. 性能优化，虚拟列表、懒加载、增量索引、防抖节流、进程池、缓存。
2. 启动优化，冷启动小于 2 秒，热启动小于 1 秒。
3. 内存优化，空载小于 300 MB。
4. 搜索优化，命令面板小于 50 毫秒，快速打开小于 100 毫秒，全局搜索首屏小于 500 毫秒。
5. 跨平台适配，macOS 菜单栏与 Cmd，Windows 标题栏与 Ctrl，Linux GNOME/KDE、Wayland/X11。
6. DPI 缩放、多显示器、字体渲染。
7. 单实例、多窗口、工作区恢复。
8. 崩溃恢复、自动保存、日志导出。
9. 打包，macOS dmg、Windows msi、Linux deb/rpm/AppImage。
10. 自动更新，可选，需签名与校验。
11. 安装包签名与公证，macOS 与 Windows。

验收：
性能指标达标。三平台可安装运行。快捷键与菜单符合平台习惯。自动更新可用。崩溃恢复可用。

约束：不得为了性能牺牲功能。不得引入重量级依赖。

十五、欢迎界面任务模板

任务：欢迎界面

目标：实现首次启动欢迎界面，一次性引导，之后不再打扰。

交付：
1. 居中模态，宽度 480 px，最大 520 px，圆角 6 px，阴影使用 modal shadow token。
2. 图标使用 assets/icons/go.svg，来源为 /home/anan/programming-languages-logos/go-old/go-old.svg，64 x 64 px，保留原色，这是唯一允许彩色品牌图标的位置。
3. 标题 Go Studio，20 px，600 字重。
4. 副标题 A minimal IDE for Go and cloud-native development，13 px。
5. 三行快捷键提示，Ctrl+Shift+P 命令面板，Ctrl+P 快速打开文件，Esc 关闭浮层。macOS 显示 Cmd。
6. 主操作 Get Started，Enter 触发，关闭并写入 welcomeShown。
7. 次操作 Show Keyboard Shortcuts，打开 help.keybindings，不关闭欢迎界面。
8. 焦点默认在 Get Started，焦点陷阱，Esc 关闭。
9. 不阻塞 Ctrl+Shift+P 与 Ctrl+P。
10. 支持 i18n，默认英文，中文可选。
11. 支持 reduce motion、高对比度、字体缩放、窄窗口自适应。
12. Help: Welcome 可再次打开，不修改 welcomeShown。
13. Settings 可重置 welcomeShown。

验收：
首次启动显示。图标为本地 go.svg，原色，64 x 64 px。三行快捷键正确，macOS 显示 Cmd。Enter 与 Esc 关闭并回到编辑器。Ctrl+Shift+P 在欢迎界面中仍可用。关闭后重启不再显示。Help: Welcome 可再次打开。暗色、亮色、高对比度、reduce motion、字体缩放正常。无网络请求，无遥测，无 AI 助手。

约束：不显示教程、视频、账号、插件、AI 助手、外部链接。不收集信息。不请求网络。只显示一次。

十六、Bug 修复任务模板

任务：修复 Bug

Bug 描述：请填写现象、复现步骤、期望行为、实际行为、环境。

要求：
1. 先复现问题。
2. 定位根因，不要只改表象。
3. 提出最小修复方案。
4. 列出将修改文件。
5. 补充或更新测试，覆盖该 Bug。
6. 修复后自测，确认无回归。
7. 汇报根因、修复、测试、遗留。

约束：不得为了修复引入新依赖。不得破坏设计契约。不得跳过测试。

十七、重构任务模板

任务：重构

重构范围：请填写模块或文件。
重构目标：请填写可维护性、性能、可读性等。

要求：
1. 先说明现状与问题。
2. 给出重构方案与边界。
3. 列出将修改文件。
4. 保持外部行为不变。
5. 保持命令 id 与 API 不变。
6. 补充或更新测试。
7. 分小步提交，每步可运行。
8. 汇报结果与风险。

约束：不得大范围重写。不得改变设计契约。不得删除测试。

十八、评审任务模板

任务：代码评审

评审对象：请填写 PR 或文件。

要求：
1. 检查是否符合 AGENTS.md 契约。
2. 检查是否使用设计 token。
3. 检查是否注册为命令。
4. 检查是否支持键盘操作。
5. 检查是否处理空、加载、错误、禁用状态。
6. 检查是否脱敏敏感信息。
7. 检查危险操作是否二次确认。
8. 检查是否引入 AI 助手、常驻侧边栏、标签页、工具栏。
9. 检查是否硬编码颜色、尺寸、图标、快捷键。
10. 检查测试覆盖。
11. 输出问题清单，按严重程度排序。
12. 给出改进建议。

约束：只评审，不修改。除非用户明确要求。

十九、每轮对话结束的固定追问

每轮任务结束时，AI 必须汇报：

1. 完成了什么。
2. 修改了哪些文件。
3. 新增了哪些命令。
4. 新增了哪些测试。
5. 验收结果。
6. 遗留问题。
7. 下一步建议。

不得只说“完成了”。必须可验证、可追踪。

二十、提示词使用提醒

一次只用一个模板。
不要在一轮里叠加多个阶段。
不要把 PRD、DESIGN、COMMANDS 全文粘进 prompt，改为引用文件名。
如果 AI 偏离契约，停止并纠正，不要继续。
如果 AI 加入 AI 助手、常驻侧边栏、标签页、工具栏，立即要求移除。
如果 AI 使用假数据，立即要求替换为真实工具链。
如果 AI 硬编码颜色尺寸，立即要求改为设计 token。
如果 AI 跳过测试，立即要求补测试。

最终提醒：Go Studio 的核心是极简、命令优先、键盘流、工具链完整。任何 prompt 都必须服务于这个核心。AI 不是产品经理，不是设计师，是实现者。你负责意图与验收，AI 负责按契约实现。