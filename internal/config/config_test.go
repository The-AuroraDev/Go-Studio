// config_test.go — 配置合并、默认值与原子读写的单元测试。
// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFile 在测试目录中写入文件，失败时立即终止测试。
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestDefaultMatchesBuiltinJSON(t *testing.T) {
	cfg := Default()
	if cfg.Theme != ThemeSystem {
		t.Errorf("default theme = %q, want %q", cfg.Theme, ThemeSystem)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("default log level = %q, want %q", cfg.LogLevel, "info")
	}
}

func TestLoadWithoutAnyFileReturnsDefaults(t *testing.T) {
	// 空工作区且用户级文件不存在时，必须得到内置默认值而不是错误。
	if _, err := loadWithUserDir(t, ""); err != nil {
		t.Fatalf("load: %v", err)
	}
}

func TestWorkspaceOverridesUser(t *testing.T) {
	userDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userDir)

	writeFile(t, filepath.Join(userDir, appDirName, configFileName),
		`{"theme":"dark","logLevel":"warn"}`)

	workspace := t.TempDir()
	writeFile(t, filepath.Join(workspace, workspaceDirName, configFileName),
		`{"theme":"light"}`)

	cfg, err := Load(workspace)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Theme != ThemeLight {
		t.Errorf("theme = %q, want %q from workspace layer", cfg.Theme, ThemeLight)
	}
	if cfg.LogLevel != "warn" {
		t.Errorf("log level = %q, want %q inherited from user layer", cfg.LogLevel, "warn")
	}
}

func TestUserValueUsedWhenWorkspaceOmitsField(t *testing.T) {
	userDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userDir)

	writeFile(t, filepath.Join(userDir, appDirName, configFileName), `{"theme":"dark"}`)

	workspace := t.TempDir()
	writeFile(t, filepath.Join(workspace, workspaceDirName, configFileName),
		`{"logLevel":"error"}`)

	cfg, err := Load(workspace)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Theme != ThemeDark {
		t.Errorf("theme = %q, want %q", cfg.Theme, ThemeDark)
	}
	if cfg.LogLevel != "error" {
		t.Errorf("log level = %q, want %q", cfg.LogLevel, "error")
	}
}

func TestLoadRejectsMalformedJSON(t *testing.T) {
	userDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userDir)

	writeFile(t, filepath.Join(userDir, appDirName, configFileName), `{"theme":`)

	if _, err := Load(""); err == nil {
		t.Fatal("expected error for malformed config, got nil")
	}
}

func TestSaveThenLoadRoundTrip(t *testing.T) {
	userDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userDir)

	want := Config{Theme: ThemeLight, LogLevel: "debug"}
	if err := SaveUser(want); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := Load("")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got != want {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}
}

func TestSaveUsesRestrictivePermissions(t *testing.T) {
	userDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userDir)

	if err := SaveUser(Config{Theme: ThemeDark, LogLevel: "info"}); err != nil {
		t.Fatalf("save: %v", err)
	}

	path, err := UserFile()
	if err != nil {
		t.Fatalf("resolve user file: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	if perm := info.Mode().Perm(); perm != configFileMode {
		t.Errorf("config perm = %o, want %o", perm, configFileMode)
	}
}

func TestSaveRejectsEmptyPath(t *testing.T) {
	if err := Save("", Default()); err == nil {
		t.Fatal("expected error for empty path, got nil")
	}
}

func TestWorkspaceFileEmptyWhenNoWorkspace(t *testing.T) {
	if got := WorkspaceFile(""); got != "" {
		t.Errorf("workspace file = %q, want empty string", got)
	}
}

// loadWithUserDir 在隔离的用户配置目录中执行一次 Load，
// 用于验证「无任何配置文件」这条路径。
func loadWithUserDir(t *testing.T, workspaceRoot string) (Config, error) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	return Load(workspaceRoot)
}
