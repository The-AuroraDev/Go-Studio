// default.go — 内置默认配置的 TOML 原文。
// SPDX-License-Identifier: MIT

package config

// defaultTOML 是配置默认值。字段一一保持与 Config 结构体一一对应，
// 改结构体时必须同步改这里，否则测试会失败。
const defaultTOML = `
[general]
# 键位方案名，对应 internal/keymap 里登记的名字。
# 目前只实现了 emacs 一套（spec.md 的 C-a / C-S / C-g 前缀风格）。
keymap = "emacs"
recent_files = []
recent_limit = 20

[editor]
tab_width = 4
soft_wrap = false
line_numbers = true
relative_line_numbers = false
# 语法高亮。按扩展名自动判断语言，无扩展名的文件可用 syntax_lang 手写。
syntax = true
syntax_lang = ""

[ui]
theme = "go-studio-dark"
true_color = true
border_style = "rounded"

[log]
level = "info"
max_size_mb = 8
max_backups = 5
`
