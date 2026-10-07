#!/usr/bin/env bash
# e2e.sh — 完整使用测试的入口脚本。
#
# 它做的事很薄：确保在正确的模块根目录下运行 scripts/e2e。
# 真正的逻辑都在 Go 代码里，这样跨平台、且能被 go vet 检查。
#
# 用法:
#   scripts/e2e.sh                    # 全部场景
#   scripts/e2e.sh -only 编辑          # 只跑名字含「编辑」的场景
#   scripts/e2e.sh -v                 # 打印每个步骤
#   scripts/e2e.sh -keep              # 失败时保留工作目录
#   scripts/e2e.sh -list              # 只列出场景名

set -euo pipefail

# 无论从哪个目录调用，都要切到模块根，go 才能找到 go.mod。
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

exec go run ./scripts/e2e "$@"