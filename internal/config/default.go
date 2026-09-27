// default.go — 内置默认配置的 TOML 原文。
// SPDX-License-Identifier: MIT

package config

// defaultTOML 是配置默认值。字段一��保持与 Config 结构体一一对应，
// 改结构体时必须同步改这里，否则测试会失败。
const defaultTOML = `
[general]
keymap = "vim"
recent_files = []
recent_limit = 20

[editor]
tab_width = 4
soft_wrap = false
line_numbers = true
relative_line_numbers = false

[ui]
theme = "go-studio-dark"
true_color = true
border_style = "rounded"

[log]
level = "info"
max_size_mb = 8
max_backups = 5
`
