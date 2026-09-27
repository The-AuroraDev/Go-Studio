// paths.go — 配置与日志目录解析：用户级走系统配置目录，工作区级走项目内 .go-studio/。
// SPDX-License-Identifier: MIT

package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// appDirName 是 Go Studio 在用户配置目录下的子目录名，全平台统一。
	appDirName = "go-studio"
	// workspaceDirName 是工作区级配置在项目根目录下的子目录名。
	workspaceDirName = ".go-studio"
	// configFileName 是用户级与工作区级共用的配置文件名。
	configFileName = "config.toml"
	// logsDirName 是日志输出目录名，位于用户配置目录之下。
	logsDirName = "logs"
)

// UserDir 返回用户级配置目录。目录不存在时只返回路径，不创建。
func UserDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(base, appDirName), nil
}

// UserFile 返回用户级配置文件路径。
func UserFile() (string, error) {
	dir, err := UserDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configFileName), nil
}

// LogsDir 返回日志目录。目录不存在时只返回路径，不创建。
func LogsDir() (string, error) {
	dir, err := UserDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, logsDirName), nil
}

// WorkspaceFile 返回工作区级配置文件路径。workspaceRoot 为空时返回空字符串，
// 表示当前没有打开工作区。
func WorkspaceFile(workspaceRoot string) string {
	if workspaceRoot == "" {
		return ""
	}
	return filepath.Join(workspaceRoot, workspaceDirName, configFileName)
}

// ensureDir 创建目录及其父目录，已存在时不做任何事。
func ensureDir(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return fmt.Errorf("create dir %s: %w", path, err)
	}
	return nil
}
