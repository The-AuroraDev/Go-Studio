#!/bin/sh
#
# package-linux.sh — 把 build/bin/go-studio 打成 deb、rpm 与 AppImage。
#
# 版本号默认取自 wails.json 的 info.productVersion，可用 VERSION 覆盖；
# 架构默认 amd64，可用 ARCH 覆盖。产物写入 build/dist 并生成 SHA256SUMS。

set -eu

NFPM="${NFPM:-nfpm}"
CONFIG="build/linux/nfpm.yaml"
BINARY="build/bin/go-studio"
DIST="build/dist"

if ! command -v "$NFPM" >/dev/null 2>&1; then
	printf 'package: 未找到 %s\n' "$NFPM" >&2
	printf '请先执行 go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest\n' >&2
	exit 1
fi

# 版本号默认取自 wails.json，可用 VERSION 覆盖。
VERSION="${VERSION:-$(sh scripts/product-version.sh)}"
if [ -z "$VERSION" ]; then
	printf 'package: 无法从 wails.json 解析 productVersion，请用 VERSION=... 指定\n' >&2
	exit 1
fi

ARCH="${ARCH:-amd64}"

if [ ! -x "$BINARY" ]; then
	printf 'package: 找不到可执行文件 %s，请先运行 make build\n' "$BINARY" >&2
	exit 1
fi

rm -rf "$DIST"
mkdir -p "$DIST"

for packager in deb rpm; do
	printf 'package: 构建 %s（%s %s）\n' "$packager" "$VERSION" "$ARCH"
	VERSION="$VERSION" ARCH="$ARCH" "$NFPM" package \
		--config "$CONFIG" \
		--packager "$packager" \
		--target "$DIST/go-studio_${VERSION}_${ARCH}.${packager}"
done

cd "$DIST"
sha256sum ./*.deb ./*.rpm >SHA256SUMS
cd - >/dev/null

printf 'package: 产物位于 %s\n' "$DIST"
ls -1 "$DIST"
