# Makefile — Go Studio 构建、测试与检查入口。
# 本地与 CI 统一走 make check，避免两套命令产生差异。

SHELL := /bin/sh

GO ?= go
BINARY := go-studio
PKG := ./...

.DEFAULT_GOAL := help

.PHONY: help
help: ## 列出可用目标
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: ## 编译二进制
	$(GO) build -o $(BINARY) ./cmd/go-studio

.PHONY: run
run: ## 运行（需要真实终端）
	$(GO) run ./cmd/go-studio

.PHONY: test
test: ## 运行全部单元测试
	$(GO) test $(PKG)

.PHONY: test-race
test-race: ## 带竞态检测运行测试
	$(GO) test -race $(PKG)

.PHONY: cover
cover: ## 生成覆盖率报告
	$(GO) test -coverprofile=coverage.out -covermode=atomic $(PKG)
	$(GO) tool cover -func=coverage.out | tail -1

.PHONY: bench
bench: ## 运行基准测试
	$(GO) test -run '^$$' -bench . -benchmem $(PKG)

.PHONY: vet
vet: ## go vet
	$(GO) vet $(PKG)

.PHONY: lint
lint: fmt-check vet ## 静态检查

.PHONY: fmt
fmt: ## 按 gofmt 整理代码
	$(GO) fmt $(PKG)

.PHONY: fmt-check
fmt-check: ## 校验 gofmt 已应用
	@out="$$(gofmt -l .)"; \
	if [ -n "$$out" ]; then printf 'gofmt 未覆盖:\n%s\n' "$$out" >&2; exit 1; fi

.PHONY: tidy
tidy: ## 整理依赖
	$(GO) mod tidy

.PHONY: check
check: fmt-check vet test cover bench ## 提交前的完整门禁

.PHONY: clean
clean: ## 清理构建产物
	rm -f $(BINARY) coverage.out
	$(GO) clean -testcache
