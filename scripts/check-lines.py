#!/usr/bin/env python3
"""
check-lines.py — 按字符数校验行宽。

AIdocs/DESIGN.md 与 coding-standards 约定行宽上限 100。技能自带的
check-style.sh 使用 mawk 计算长度，按字节计数，中文注释会被误判为超长，
因此行宽由本脚本按字符数精确校验，其余风格项仍交给 check-style.sh。
"""

from __future__ import annotations

import sys
from pathlib import Path

# 只检查源码与配置，跳过构建产物与锁文件。
SKIP_DIRS = {"node_modules", "dist", "bin", ".git", "wailsjs", "AIdocs", ".opencode"}
# .md 跳过：Markdown 表格按格式必须保持单行，.editorconfig 里已关闭其行宽限制。
SKIP_SUFFIXES = {".svg", ".png", ".ico", ".sum", ".woff2", ".md"}


def iter_source_files(paths: list[str]):
    """展开命令行路径，产出需要检查的文本文件。"""
    for raw in paths:
        path = Path(raw)
        if path.is_file():
            yield path
            continue
        for candidate in sorted(path.rglob("*")):
            if not candidate.is_file():
                continue
            if SKIP_DIRS & set(candidate.parts):
                continue
            if candidate.suffix in SKIP_SUFFIXES:
                continue
            yield candidate


def check_file(path: Path, width: int, failures: list[str]) -> None:
    """检查单个文件，把超长行记入 failures。"""
    for number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
        # 制表符按 8 列展开，与 Go 的显示宽度保持一致。
        width_used = len(line.expandtabs(8))
        if width_used > width:
            failures.append(f"long-line: {path}:{number} (width {width_used} > {width})")


def main(argv: list[str]) -> int:
    if len(argv) < 3:
        print("usage: check-lines.py <width> <path> [<path> ...]", file=sys.stderr)
        return 2

    width = int(argv[1])
    failures: list[str] = []
    for path in iter_source_files(argv[2:]):
        check_file(path, width, failures)

    for failure in failures:
        print(failure, file=sys.stderr)
    if failures:
        print(f"lines: {len(failures)} 行超出 {width} 列", file=sys.stderr)
        return 1

    print(f"lines: OK（{width} 列）")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
