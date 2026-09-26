# Makefile — Go Studio 构建、测试与检查入口。
# 本地与 CI 统一走 make check，避免两套命令产生差异。

SHELL := /bin/sh

# 工具链入口，可用环境变量覆盖以指向特定版本的 go 与 wails。
GO ?= go
WAILS ?= wails
NFPM ?= nfpm

# Ubuntu 26.04 已移除 webkit2gtk-4.0，wails v2 通过 webkit2_41 tag 走 4.1。
# wails.json 的 build:tags 已包含此 tag，这里覆盖裸 go 命令。
GO_TAGS := webkit2_41

FRONTEND := frontend

.DEFAULT_GOAL := help

.PHONY: help
help: ## 列出可用目标
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

.PHONY: deps
deps: ## 安装前端依赖并生成 Wails 绑定
	cd $(FRONTEND) && pnpm install
	$(GO) generate ./...

.PHONY: bindings
bindings: ## 重新生成 Wails 前端绑定
	$(WAILS) generate module

.PHONY: dev
dev: ## 启动开发模式（Wails + Vite 热更新）
	GO_STUDIO_CONSOLE_LOG=1 $(WAILS) dev

.PHONY: build
build: ## 构建桌面应用二进制到 build/bin
	$(WAILS) build -clean

.PHONY: build-frontend
build-frontend: ## 只构建前端产物
	cd $(FRONTEND) && pnpm run build

.PHONY: package
package: build ## 构建 deb 与 rpm 安装包到 build/dist
	NFPM=$(NFPM) sh scripts/package-linux.sh

.PHONY: package-arm64
package-arm64: ## 构建 arm64 的 deb 与 rpm（需先交叉编译二进制）
	@test -f build/bin/go-studio || { \
		printf '请先用 wails build -platform linux/arm64 生成二进制\n' >&2; exit 1; }
	NFPM=$(NFPM) ARCH=arm64 sh scripts/package-linux.sh

.PHONY: test
test: test-go test-web ## 运行全部测试

.PHONY: test-go
test-go: ## 运行 Go 单元测试
	$(GO) test -tags $(GO_TAGS) ./...

.PHONY: test-web
test-web: ## 运行前端单元测试
	cd $(FRONTEND) && pnpm run test

.PHONY: lint
lint: lint-go lint-web ## 运行全部静态检查

.PHONY: lint-go
lint-go: ## go vet 硬门禁 + staticcheck 尽力而为
	$(GO) vet -tags $(GO_TAGS) ./...
	@sh scripts/staticcheck.sh

.PHONY: lint-web
lint-web: ## eslint + prettier 格式检查
	cd $(FRONTEND) && pnpm run lint
	cd $(FRONTEND) && pnpm run format:check

.PHONY: typecheck
typecheck: ## 前端类型检查
	cd $(FRONTEND) && pnpm run typecheck

.PHONY: fmt
fmt: ## 按各语言官方格式化器整理代码
	$(GO) fmt ./...
	cd $(FRONTEND) && pnpm run format

STYLE_SCRIPT := .opencode/skills/coding-standards/scripts/check-style.sh
STYLE_PATHS := internal main.go frontend/src scripts build/linux .github
LINE_WIDTH := 100

.PHONY: style
style: ## 编码规范风格检查（行宽、缩进、文件头、行尾空格）
	@python3 scripts/check-lines.py $(LINE_WIDTH) $(STYLE_PATHS)
	@if [ ! -f "$(STYLE_SCRIPT)" ]; then \
		printf 'style: 未找到 %s，跳过行尾空格与文件头检查（该脚本不纳入版本控制）\n' "$(STYLE_SCRIPT)"; \
		exit 0; \
	fi; \
	sh $(STYLE_SCRIPT) --width=1000 $(STYLE_PATHS)

.PHONY: icons
icons: ## 校验图标规范
	sh scripts/check-icons.sh

.PHONY: icons-render
icons-render: ## 从 build/branding/gostudio-mark.svg 重新生成全部品牌衍生物
	python3 scripts/render-icons.py

.PHONY: check
check: fmt-check typecheck lint test style icons ## 提交前的完整门禁

.PHONY: fmt-check
fmt-check: ## 校验格式化是否已应用
	@out="$$(gofmt -l . )"; \
	if [ -n "$$out" ]; then printf 'gofmt 未覆盖:\n%s\n' "$$out" >&2; exit 1; fi
	cd $(FRONTEND) && pnpm run format:check

.PHONY: clean
clean: ## 清理构建产物
	rm -rf build/bin build/dist $(FRONTEND)/dist
