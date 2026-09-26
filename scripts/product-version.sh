#!/bin/sh
#
# product-version.sh — 打印 wails.json 里的 info.productVersion。
# 版本号的唯一真源是 wails.json，打包脚本与发布工作流都从这里取值。

set -eu

sed -n 's/.*"productVersion"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' wails.json | head -1
