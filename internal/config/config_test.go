// config_test.go — 配置合并、默认值与原子读写的单元测试。
// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// writeFile 在测试目录中写入文件，失败时立即终止测试。
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("create dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), configFileMode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// isolateUserDir 把 XDG_CONFIG_HOME 指向临时目录，隔离真实用户配置。
func isolateUserDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	return dir
}

func TestDefaultIsUsable(t *testing.T) {
	cfg := Default()
	if cfg.Editor.TabWidth != 4 {
		t.Errorf("default tab width = %d, want 4", cfg.Editor.TabWidth)
	}
	if !cfg.Editor.LineNumbers {
		t.Error("default line numbers should be enabled")
	}
	if cfg.UI.Theme == "" || cfg.Log.Level == "" {
		t.Errorf("default theme and log level must be set, got %q / %q", cfg.UI.Theme, cfg.Log.Level)
	}
}

func TestLoadWithoutAnyFileReturnsDefaults(t *testing.T) {
	isolateUserDir(t)

	got, err := Load("")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !reflect.DeepEqual(got, Default()) {
		t.Errorf("load with no files = %+v, want defaults", got)
	}
}

func TestWorkspaceOverridesUser(t *testing.T) {
	userDir := isolateUserDir(t)
	writeFile(t, filepath.Join(userDir, appDirName, configFileName), "[ui]\ntheme = \"dark\"\n")

	workspace := t.TempDir()
	writeFile(t, filepath.Join(workspace, workspaceDirName, configFileName), "[ui]\ntheme = \"light\"\n")

	cfg, err := Load(workspace)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.UI.Theme != "light" {
		t.Errorf("theme = %q, want %q from workspace layer", cfg.UI.Theme, "light")
	}
	// 工作区级没提到的字段必须保留用户级的值。
	if cfg.Editor.TabWidth != 4 {
		t.Errorf("tab width = %d, want 4 inherited from defaults", cfg.Editor.TabWidth)
	}
}

func TestUserValueUsedWhenWorkspaceOmitsField(t *testing.T) {
	userDir := isolateUserDir(t)
	writeFile(t, filepath.Join(userDir, appDirName, configFileName),
		"[editor]\ntab_width = 8\nsoft_wrap = true\n")

	workspace := t.TempDir()
	writeFile(t, filepath.Join(workspace, workspaceDirName, configFileName), "[editor]\ntab_width = 2\n")

	cfg, err := Load(workspace)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Editor.TabWidth != 2 {
		t.Errorf("tab width = %d, want 2 from workspace", cfg.Editor.TabWidth)
	}
	if !cfg.Editor.SoftWrap {
		t.Error("soft_wrap = false, want true inherited from user layer")
	}
}

func TestLoadRejectsMalformedTOML(t *testing.T) {
	userDir := isolateUserDir(t)
	writeFile(t, filepath.Join(userDir, appDirName, configFileName), "[ui\ntheme = \n")

	if _, err := Load(""); err == nil {
		t.Fatal("expected error for malformed config, got nil")
	}
}

func TestSaveThenLoadRoundTrip(t *testing.T) {
	isolateUserDir(t)

	want := Config{
		General: General{Keymap: "emacs", RecentLimit: 5},
		Editor:  Editor{TabWidth: 2, SoftWrap: true, LineNumbers: true},
		UI:      UI{Theme: "custom", TrueColor: true, BorderStyle: "thick"},
		Log:     Log{Level: "debug", MaxSizeMB: 16, MaxBackups: 3},
	}
	if err := SaveUser(want); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := Load("")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}
}

func TestRecentFilesRoundTrip(t *testing.T) {
	isolateUserDir(t)

	want := Default()
	want.General.RecentFiles = []string{"/tmp/a.go", "/tmp/b.go"}
	if err := SaveUser(want); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := Load("")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !reflect.DeepEqual(got.General.RecentFiles, want.General.RecentFiles) {
		t.Errorf("recent files = %v, want %v", got.General.RecentFiles, want.General.RecentFiles)
	}
}

func TestSaveUsesRestrictivePermissions(t *testing.T) {
	isolateUserDir(t)

	if err := SaveUser(Default()); err != nil {
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

func TestSaveLeavesNoTempFileBehind(t *testing.T) {
	userDir := isolateUserDir(t)
	if err := SaveUser(Default()); err != nil {
		t.Fatalf("save: %v", err)
	}

	entries, err := os.ReadDir(filepath.Join(userDir, appDirName))
	if err != nil {
		t.Fatalf("read config dir: %v", err)
	}
	if len(entries) != 1 {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Errorf("config dir contains %v, want only %q", names, configFileName)
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
