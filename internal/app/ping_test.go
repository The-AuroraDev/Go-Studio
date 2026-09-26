// ping_test.go — 前后端自检接口与配置读写的单元测试。
// SPDX-License-Identifier: MIT

package app

import (
	"strings"
	"testing"

	"github.com/29anan29/Go-Studio/internal/config"
)

func TestPingEchoesMessage(t *testing.T) {
	application := New(config.Default(), nil)

	got, err := application.Ping("hello")
	if err != nil {
		t.Fatalf("ping: %v", err)
	}
	if !strings.HasSuffix(got, "hello") {
		t.Errorf("ping reply = %q, want it to end with %q", got, "hello")
	}
	if !strings.Contains(got, "pong") {
		t.Errorf("ping reply = %q, want it to mention pong", got)
	}
}

func TestPingAcceptsEmptyMessage(t *testing.T) {
	application := New(config.Default(), nil)

	got, err := application.Ping("")
	if err != nil {
		t.Fatalf("ping: %v", err)
	}
	if got != "pong v1" {
		t.Errorf("ping reply = %q, want %q", got, "pong v1")
	}
}

func TestPingVersionIsExposed(t *testing.T) {
	application := New(config.Default(), nil)
	if got := application.PingVersion(); got != PingVersion {
		t.Errorf("ping version = %d, want %d", got, PingVersion)
	}
}

func TestSetThemePersistsAndUpdatesMemory(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	application := New(config.Default(), nil)

	if err := application.SetTheme(config.ThemeDark); err != nil {
		t.Fatalf("set theme: %v", err)
	}
	if got := application.Config().Theme; got != config.ThemeDark {
		t.Errorf("in-memory theme = %q, want %q", got, config.ThemeDark)
	}

	// 重新从磁盘加载，确认主题真的落盘而不是只改了内存。
	reloaded, err := config.Load("")
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if reloaded.Theme != config.ThemeDark {
		t.Errorf("persisted theme = %q, want %q", reloaded.Theme, config.ThemeDark)
	}
}

func TestSetThemeRejectsUnknownValue(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	application := New(config.Default(), nil)

	if err := application.SetTheme("solarized"); err == nil {
		t.Fatal("expected error for unknown theme, got nil")
	}
	if got := application.Config().Theme; got != config.ThemeSystem {
		t.Errorf("theme after rejection = %q, want unchanged %q", got, config.ThemeSystem)
	}
}

func TestNewWithoutLoggerDoesNotPanic(t *testing.T) {
	// logger 为 nil 是启动早期的合法状态，记录方法必须静默降级。
	application := New(config.Default(), nil)
	application.info("no logger attached")
	application.errorf("still no logger")

	if got := application.LogFilePath(); got != "" {
		t.Errorf("log file path = %q, want empty string", got)
	}
}
