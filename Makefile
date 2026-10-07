# Makefile — Go Studio 构建、测试与检查入口。
# 本地与 CI 统一走 make check，避免两套命令产生差异。

SHELL := /bin/sh

GO ?= go
BINARY := go-studio
PKG := ./...

# 覆盖率只统计产品代码。scripts/e2e 是测试驱动本身（起伪终端、造场景、
# 重建虚拟屏幕），把它算进分母只会让数字失真——它是 0% 覆盖的，
# 却有好几百条语句。
COVER_PKG := ./cmd/... ./internal/...

.DEFAULT_GOAL := help

.PHONY: help
help: ## 列出可用目标
	@grep -hE '^[a-zA-Z0-9_-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: ## 编译二进制
	$(GO) build -o $(BINARY) ./cmd/go-studio

.PHONY: run
run: ## 运行（需要真实终端）
	$(GO) run ./cmd/go-studio

.PHONY: test
test: ## 运行单元测试（-short 跳过深度档位）
	$(GO) test -short $(PKG)

.PHONY: e2e
e2e: ## 完整使用测试：在伪终端里把编辑器当真人用一遍（约 25 秒）
	./scripts/e2e.sh

.PHONY: e2e-list
e2e-list: ## 列出完整使用测试的所有场景
	./scripts/e2e.sh -list

.PHONY: test-deep
test-deep: ## 运行深度属性测试（较慢，如缓冲 10 万次随机编辑）
	$(GO) test -count=1 -timeout 30m -run Deep $(PKG)

.PHONY: test-race
test-race: ## 带竞态检测运行测试
	$(GO) test -race -short $(PKG)

.PHONY: cover
cover: ## 生成覆盖率报告
	$(GO) test -short -coverprofile=coverage.out -covermode=atomic $(COVER_PKG)
	$(GO) tool cover -func=coverage.out | tail -1

.PHONY: bench
bench: ## 冒烟跑一遍全部基准（短 benchtime，用于门禁）
	$(GO) test -run '^$$' -bench . -benchmem -benchtime 200ms $(PKG)

.PHONY: bench-full
bench-full: ## 完整基准（默认 benchtime，用于真实性能数字）
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
check: fmt-check vet test cover bench test-race ## 提交前的完整门禁

.PHONY: clean
clean: ## 清理构建产物
	rm -f $(BINARY) coverage.out
	$(GO) clean -testcache
