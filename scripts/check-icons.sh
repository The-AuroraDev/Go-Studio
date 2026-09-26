#!/bin/sh
#
# check-icons.sh — 图标规范校验：命名、颜色、格式、必备清单。
# 依据 AIdocs/DESIGN.md 第 9 节。任何一条不满足即以非零码退出。

set -eu

ICON_DIR="${1:-assets/icons}"
FAILURES=0

fail() {
	printf 'icons: FAIL %s\n' "$1" >&2
	FAILURES=$((FAILURES + 1))
}

# 命令面板分类与设计文档要求的必备图标，缺一个都不允许。
REQUIRED="git docker kubernetes terminal settings search debug test problems output branch
	container image volume network pod service deployment configmap secret namespace node
	event port-forward logs apply delete scale rollout go"

if [ ! -d "$ICON_DIR" ]; then
	printf 'icons: FAIL 目录不存在 %s\n' "$ICON_DIR" >&2
	exit 1
fi

for name in $REQUIRED; do
	if [ ! -f "$ICON_DIR/$name.svg" ]; then
		fail "缺少必备图标 $name.svg"
	fi
done

# 欢迎页专用的品牌色 Go 图标与全局单色版必须同时存在。
if [ ! -f "$ICON_DIR/go-brand.svg" ]; then
	fail "缺少欢迎页品牌图标 go-brand.svg"
fi

for path in "$ICON_DIR"/*.svg; do
	base=$(basename "$path")

	# 命名必须是 kebab-case，只允许小写字母、数字与连字符。
	if ! printf '%s' "${base%.svg}" | grep -Eq '^[a-z0-9]+(-[a-z0-9]+)*$'; then
		fail "$base 文件名不是 kebab-case"
	fi

	# 必须是矢量 SVG，且不得内嵌位图、渐变或滤镜。
	case "$base" in
	*.svg) ;;
	*) fail "$base 不是 .svg" ;;
	esac
	if grep -Eq '<image|linearGradient|radialGradient|<filter' "$path"; then
		fail "$base 含位图、渐变或滤镜"
	fi

	# go-old.svg 是 Go 原始素材，go-brand.svg 是欢迎页专用彩色版，
	# 其余图标一律单色 currentColor。
	case "$base" in
	go-old.svg | go-brand.svg) ;;
	*)
		if grep -Eq 'fill="#|fill="rgb' "$path"; then
			fail "$base 含字面量颜色，应为 currentColor"
		fi
		;;
	esac
done

# 品牌图标只允许出现在欢迎页，因此必须真的是彩色版本。
if [ -f "$ICON_DIR/go-brand.svg" ] && ! grep -q 'fill="#' "$ICON_DIR/go-brand.svg"; then
	fail "go-brand.svg 应保留 Go Studio 品牌色"
fi

# 品牌衍生物必须齐备：改了主文件却忘了重新生成是最容易漏的一步。
BRANDING_MASTER="build/branding/gostudio-mark.svg"
if [ ! -f "$BRANDING_MASTER" ]; then
	fail "缺少品牌主文件 $BRANDING_MASTER"
else
	for derived in build/linux/go-studio.svg build/appicon.png build/windows/icon.ico; do
		if [ ! -f "$derived" ]; then
			fail "缺少品牌衍生物 $derived，请运行 python3 scripts/render-icons.py"
		fi
	done
	brand="$ICON_DIR/go-brand.svg"
	if [ -f "$brand" ] && ! grep -q '<title>Go Studio</title>' "$brand"; then
		fail "go-brand.svg 的 title 应为 Go Studio，说明它来自品牌主文件"
	fi
	# 位图边长必须与文件名一致，否则桌面环境会缩放错位。
	python3 - <<'PY' || fail "品牌位图尺寸与文件名不一致"
import re
import struct
import sys
from pathlib import Path

bad = []
for path in sorted(Path("build/linux/icons").glob("*x*.png")):
    size = int(re.match(r"(\d+)x", path.name).group(1))
    width, height = struct.unpack(">II", path.read_bytes()[16:24])
    if (width, height) != (size, size):
        bad.append(f"{path} 实际 {width}x{height}")

app = Path("build/appicon.png")
if app.is_file():
    width, height = struct.unpack(">II", app.read_bytes()[16:24])
    if (width, height) != (1024, 1024):
        bad.append(f"{app} 实际 {width}x{height}")

for line in bad:
    print(f"icons: {line}", file=sys.stderr)
sys.exit(1 if bad else 0)
PY
fi

if [ "$FAILURES" -ne 0 ]; then
	printf 'icons: %d 项不合规\n' "$FAILURES" >&2
	exit 1
fi

printf 'icons: OK（%s 个图标）\n' "$(find "$ICON_DIR" -maxdepth 1 -name '*.svg' | wc -l | tr -d ' ')"
