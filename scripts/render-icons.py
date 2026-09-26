#!/usr/bin/env python3
"""
render-icons.py — 从品牌主文件生成应用图标与欢迎页图标。

唯一真源是 build/branding/gostudio-mark.svg。本脚本产出：

  build/linux/go-studio.svg   可缩放应用图标（hicolor/scalable）
  build/appicon.png           1024x1024 位图，Wails 用于生成 macOS icns
  build/windows/icon.ico      多尺寸图标，Windows 打包用
  build/linux/icons/*.png     常见尺寸位图，兼容不认可缩放图标的桌面
  assets/icons/go-brand.svg   欢迎页品牌图标

依赖：
  SVG 衍生物只需要标准库。
  位图需要 cairosvg（内含 Pillow），安装方式：python3 -m venv .venv && .venv/bin/pip install cairosvg
  缺少 cairosvg 时脚本只跳过位图部分并返回非零码，避免误以为图标已更新。
"""

from __future__ import annotations

import re
import struct
import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent
MASTER = REPO_ROOT / "build/branding/gostudio-mark.svg"

# 主文件的 viewBox 与内容外接框推出的正方形裁剪区。
# 内容外接框约 x[17.5, 275.5] y[23.5, 272.5]，取边长 264 并居中即得下式。
# 主文件一旦改动，此断言会失败，防止静默生成错误的裁剪结果。
EXPECTED_MASTER_VIEWBOX = "0 0 280 290"
SQUARE_VIEWBOX = "14.5 16 264 264"

# 位图产物：路径 -> 边长
RASTER_TARGETS = {
    "build/appicon.png": 1024,
    "build/windows/icon.ico": 256,
    "build/linux/icons/16x16.png": 16,
    "build/linux/icons/32x32.png": 32,
    "build/linux/icons/48x48.png": 48,
    "build/linux/icons/64x64.png": 64,
    "build/linux/icons/128x128.png": 128,
    "build/linux/icons/256x256.png": 256,
    "build/linux/icons/512x512.png": 512,
}

# Windows 图标内嵌的尺寸。
ICO_SIZES = [16, 24, 32, 48, 64, 128, 256]

# 衍生物顶部的自动生成标记，避免有人直接改生成出来的文件。
GENERATED_NOTICE = (
    "<!--\n"
    "  由 scripts/render-icons.py 从 build/branding/gostudio-mark.svg 生成，请勿直接修改。\n"
    "  改设计请改主文件后重新运行：python3 scripts/render-icons.py\n"
    "-->\n"
)


def read_master() -> str:
    """读取主文件并校验 viewBox，未变更时不会静默产出错误结果。"""
    if not MASTER.is_file():
        sys.exit(f"render-icons: 缺少主文件 {MASTER}")
    source = MASTER.read_text(encoding="utf-8")
    found = re.search(r'viewBox="([^"]+)"', source)
    if found is None or found.group(1) != EXPECTED_MASTER_VIEWBOX:
        actual = found.group(1) if found else "无"
        sys.exit(
            f"render-icons: 主文件 viewBox 变为 {actual}，"
            f"需要重新核算 {SQUARE_VIEWBOX} 这个正方形裁剪区"
        )
    return source


def square_svg(source: str) -> str:
    """把主文件转成正方形 viewBox 的可缩放图标，并换掉主文件的注释抬头。"""
    squared = re.sub(r'viewBox="[^"]+"', f'viewBox="{SQUARE_VIEWBOX}"', source, count=1)
    squared = squared.replace("<title>Gopher with a Pen</title>", "<title>Go Studio</title>", 1)
    reheadered = re.sub(r"\A<!--.*?-->", GENERATED_NOTICE, squared, count=1, flags=re.S)
    return reheadered.rstrip("\n") + "\n"


def write_text(relative: str, content: str) -> None:
    """写入文本产物，必要时创建父目录。"""
    path = REPO_ROOT / relative
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding="utf-8")
    print(f"render-icons: 写入 {relative}")


def render_rasters(square_source: str) -> int:
    """渲染全部位图产物，返回失败计数。"""
    try:
        import cairosvg
        from PIL import Image
    except ImportError:
        print(
            "render-icons: 缺少 cairosvg，位图未更新。"
            "请执行 python3 -m venv .venv && .venv/bin/pip install cairosvg",
            file=sys.stderr,
        )
        return 1

    for relative, size in RASTER_TARGETS.items():
        if relative.endswith(".ico"):
            continue
        path = REPO_ROOT / relative
        path.parent.mkdir(parents=True, exist_ok=True)
        cairosvg.svg2png(
            bytestring=square_source.encode("utf-8"),
            write_to=str(path),
            output_width=size,
            output_height=size,
        )
        print(f"render-icons: 写入 {relative}（{size}x{size}）")

    # Windows 图标：Vista 之后的 .ico 允许直接内嵌 PNG。
    ico_source = REPO_ROOT / "build/linux/icons/256x256.png"
    if ico_source.is_file():
        with Image.open(ico_source) as image:
            ico_path = REPO_ROOT / "build/windows/icon.ico"
            ico_path.parent.mkdir(parents=True, exist_ok=True)
            image.save(ico_path, format="ICO", sizes=[(s, s) for s in ICO_SIZES])
            print(f"render-icons: 写入 build/windows/icon.ico（{ICO_SIZES}）")
    return 0


def main() -> int:
    """生成全部衍生物并返回退出码。"""
    source = read_master()
    square = square_svg(source)

    write_text("build/linux/go-studio.svg", square)
    # 欢迎页按 64x64 渲染，这里补上绝对尺寸属性方便按需缩放。
    write_text(
        "assets/icons/go-brand.svg",
        square.replace("<svg ", '<svg width="256" height="256" ', 1),
    )

    return render_rasters(square)


if __name__ == "__main__":
    sys.exit(main())
