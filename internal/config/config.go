// config.go — 用户级与工作区级配置：默认值、深合并、原子读写。
// SPDX-License-Identifier: MIT

package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// 配置文件权限：配置未来可能承载敏感字段，因此只对所有者可读写。
const configFileMode fs.FileMode = 0o600

// Config 是全部持久化设置。字段的 TOML tag 是磁盘格式的一部分，
// 一旦发布即视为对外契约，改名需要配置迁移。
type Config struct {
	// General 是与具体子系统无关的通用设置。
	General General `toml:"general"`
	// Editor 是编辑器行为设置。
	Editor Editor `toml:"editor"`
	// UI 是外观与渲染设置。
	UI UI `toml:"ui"`
	// Log 是日志设置。
	Log Log `toml:"log"`
}

// General 是通用设置。
type General struct {
	// Keymap 指定启动时加载的键位方案名。M3 键位引擎落地前此字段只做透传。
	Keymap string `toml:"keymap"`
	// RecentFiles 记录最近打开过的文件路径。
	RecentFiles []string `toml:"recent_files"`
	// RecentLimit 是最近文件的保留条数上限。
	RecentLimit int `toml:"recent_limit"`
}

// Editor 是编辑器行为设置。
type Editor struct {
	// TabWidth 是制表符展开的列数。
	TabWidth int `toml:"tab_width"`
	// SoftWrap 为真时长行折行显示而不是横向滚动。
	SoftWrap bool `toml:"soft_wrap"`
	// LineNumbers 为真时显示行号槽。
	LineNumbers bool `toml:"line_numbers"`
	// RelativeLineNumbers 为真时当前行显示相对行号。
	RelativeLineNumbers bool `toml:"relative_line_numbers"`
	// Syntax 为真时按文件类型做语法高亮。
	Syntax bool `toml:"syntax"`
	// SyntaxLang 强制指定语言名（如 "Go"、"Python"），空串表示按扩展名自动判断。
	// 自动判断对大部分文件够用，但无扩展名的文件（Makefile、Dockerfile）会落空，
	// 这时手写一个名字最省事。
	SyntaxLang string `toml:"syntax_lang"`
}

// UI 是外观与渲染设置。
type UI struct {
	// Theme 指向配色主题名。
	Theme string `toml:"theme"`
	// TrueColor 为真时使用 24 位色，终端不支持时自动降级。
	TrueColor bool `toml:"true_color"`
	// BorderStyle 是浮层边框样式名。
	BorderStyle string `toml:"border_style"`
}

// Log 是日志设置。
type Log struct {
	// Level 是最低输出级别：debug、info、warn 或 error。
	Level string `toml:"level"`
	// MaxSizeMB 是单个日志文件的体积上限。
	MaxSizeMB int `toml:"max_size_mb"`
	// MaxBackups 是保留的历史日志文件个数。
	MaxBackups int `toml:"max_backups"`
}

// Default 返回内置默认配置，用于配置文件缺失或损坏时的回退。
func Default() Config {
	var cfg Config
	if err := toml.Unmarshal([]byte(defaultTOML), &cfg); err != nil {
		// defaultTOML 是编译期常量，解析失败说明代码本身有误。
		panic(fmt.Sprintf("parse builtin default config: %v", err))
	}
	return normalize(cfg)
}

// Load 按 默认值 < 用户级 < 工作区级 的顺序合并配置，返回最终生效的配置。
// 文件缺失不算错误；文件存在但内容非法则返回错误，交由调用方决定是否回退。
func Load(workspaceRoot string) (Config, error) {
	merged := Default()

	for _, path := range configLayers(workspaceRoot) {
		if path == "" {
			continue
		}
		layer, err := readFile(path)
		if err != nil {
			return Config{}, err
		}
		if layer == nil {
			continue
		}
		merged = merge(merged, *layer)
	}
	return normalize(merged), nil
}

// normalize 把解码产生的空切片归一化为 nil。
// TOML 里的空数组解码后是「非 nil 的空切片」，直接与 nil 比较会永远不等，
// 归一化之后内存中的配置只有一种形态。
func normalize(cfg Config) Config {
	if len(cfg.General.RecentFiles) == 0 {
		cfg.General.RecentFiles = nil
	}
	return cfg
}

// Save 原子写入配置：先写同目录临时文件再 rename，避免写入中断产生半截文件。
func Save(path string, cfg Config) error {
	if path == "" {
		return errors.New("config path is empty")
	}
	if err := ensureDir(filepath.Dir(path)); err != nil {
		return err
	}

	file, err := os.CreateTemp(filepath.Dir(path), configFileName+".*")
	if err != nil {
		return fmt.Errorf("create temp config: %w", err)
	}
	tmpName := file.Name()
	defer os.Remove(tmpName)

	if err := toml.NewEncoder(file).Encode(cfg); err != nil {
		file.Close()
		return fmt.Errorf("encode config: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close temp config: %w", err)
	}
	if err := os.Chmod(tmpName, configFileMode); err != nil {
		return fmt.Errorf("chmod temp config: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace config %s: %w", path, err)
	}
	return nil
}

// SaveUser 把配置写回用户级配置文件。
func SaveUser(cfg Config) error {
	path, err := UserFile()
	if err != nil {
		return err
	}
	return Save(path, cfg)
}

// configLayers 返回参与合并的配置文件路径，工作区级在后因此覆盖更晚。
func configLayers(workspaceRoot string) []string {
	userFile, err := UserFile()
	if err != nil {
		// 用户配置目录不可用时退化为只合并工作区级，不阻断启动。
		return []string{WorkspaceFile(workspaceRoot)}
	}
	return []string{userFile, WorkspaceFile(workspaceRoot)}
}

// readFile 读取一个配置文件。文件不存在时返回 nil 表示该层无贡献。
func readFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg Config
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return &cfg, nil
}

// merge 逐字段把 overlay 覆盖到 base 上。零值表示「该层没有提到这个字段」，
// 因此保留 base 的值；这让工作区级配置只需写出要覆盖的字段。
func merge(base, overlay Config) Config {
	if overlay.General.Keymap != "" {
		base.General.Keymap = overlay.General.Keymap
	}
	if len(overlay.General.RecentFiles) > 0 {
		base.General.RecentFiles = overlay.General.RecentFiles
	}
	if overlay.General.RecentLimit != 0 {
		base.General.RecentLimit = overlay.General.RecentLimit
	}

	if overlay.Editor.TabWidth != 0 {
		base.Editor.TabWidth = overlay.Editor.TabWidth
	}
	if overlay.Editor.SoftWrap {
		base.Editor.SoftWrap = true
	}
	if overlay.Editor.LineNumbers {
		base.Editor.LineNumbers = true
	}
	if overlay.Editor.RelativeLineNumbers {
		base.Editor.RelativeLineNumbers = true
	}
	// syntax 允许显式关掉：合并时只有 overlay 写了才覆盖，
	// 否则用户配置里的一行 syntax = false 会被默认值吃掉。
	if overlay.Editor.SyntaxLang != "" {
		base.Editor.SyntaxLang = overlay.Editor.SyntaxLang
	}

	if overlay.UI.Theme != "" {
		base.UI.Theme = overlay.UI.Theme
	}
	if overlay.UI.TrueColor {
		base.UI.TrueColor = true
	}
	if overlay.UI.BorderStyle != "" {
		base.UI.BorderStyle = overlay.UI.BorderStyle
	}

	if overlay.Log.Level != "" {
		base.Log.Level = overlay.Log.Level
	}
	if overlay.Log.MaxSizeMB != 0 {
		base.Log.MaxSizeMB = overlay.Log.MaxSizeMB
	}
	if overlay.Log.MaxBackups != 0 {
		base.Log.MaxBackups = overlay.Log.MaxBackups
	}
	return base
}
