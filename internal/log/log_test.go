// log_test.go — 日志写入、级别解析、轮转与脱敏的单元测试。
// SPDX-License-Identifier: MIT

package log

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// readRecords 读取日志文件并按行解析为 JSON 记录。
func readRecords(t *testing.T, path string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log %s: %v", path, err)
	}

	var records []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("parse log line %q: %v", line, err)
		}
		records = append(records, record)
	}
	return records
}

func TestParseLevelAcceptsKnownNames(t *testing.T) {
	cases := map[string]slog.Level{
		"debug":   slog.LevelDebug,
		"INFO":    slog.LevelInfo,
		" warn ":  slog.LevelWarn,
		"warning": slog.LevelWarn,
		"error":   slog.LevelError,
	}
	for input, want := range cases {
		got, err := ParseLevel(input)
		if err != nil {
			t.Errorf("ParseLevel(%q) returned error: %v", input, err)
			continue
		}
		if got != want {
			t.Errorf("ParseLevel(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestParseLevelRejectsUnknownName(t *testing.T) {
	if _, err := ParseLevel("verbose"); err == nil {
		t.Fatal("expected error for unknown level, got nil")
	}
}

func TestLoggerWritesRecordsToFile(t *testing.T) {
	dir := t.TempDir()
	logger, err := New(Options{Dir: dir, Level: "info"})
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	defer func() {
		if err := logger.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	}()

	logger.Info("app started", "version", "0.1.0")
	logger.Debug("this is filtered out")

	path := logger.LogFilePath()
	if path == "" {
		t.Fatal("LogFilePath returned empty string")
	}

	records := readRecords(t, path)
	if len(records) != 1 {
		t.Fatalf("record count = %d, want 1 (debug must be filtered)", len(records))
	}
	if records[0]["msg"] != "app started" {
		t.Errorf("msg = %v, want %q", records[0]["msg"], "app started")
	}
	if records[0]["version"] != "0.1.0" {
		t.Errorf("version = %v, want %q", records[0]["version"], "0.1.0")
	}
}

func TestLoggerRedactsSensitiveAttributes(t *testing.T) {
	dir := t.TempDir()
	logger, err := New(Options{Dir: dir, Level: "debug"})
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	defer logger.Close()

	logger.Info("auth attempt",
		"apiKey", "super-secret-value",
		"userPassword", "hunter2",
		"author", "29anan29",
	)

	raw, err := os.ReadFile(logger.LogFilePath())
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	for _, secret := range []string{"super-secret-value", "hunter2"} {
		if strings.Contains(string(raw), secret) {
			t.Errorf("log leaked secret %q", secret)
		}
	}

	record := readRecords(t, logger.LogFilePath())[0]
	if record["apiKey"] != mask {
		t.Errorf("apiKey = %v, want %q", record["apiKey"], mask)
	}
	// author 不在敏感词表内，必须保持原值以证明脱敏没有过度拦截。
	if record["author"] != "29anan29" {
		t.Errorf("author = %v, want %q", record["author"], "29anan29")
	}
}

func TestNewConsoleRejectsInvalidLevel(t *testing.T) {
	if _, err := NewConsole("nope"); err == nil {
		t.Fatal("expected error for invalid level, got nil")
	}
}

func TestRotateIfNeededKeepsSmallFile(t *testing.T) {
	dir := t.TempDir()
	current := logFilePath(dir, fixedTime())
	writeFile(t, current, strings.Repeat("x", 2*1024))

	// 上限 1 MB 而文件只有 2 KB，不应触发轮转。
	if err := RotateIfNeeded(dir, 1, 3); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if _, err := os.Stat(current); err != nil {
		t.Fatalf("small file must not be rotated: %v", err)
	}
}

func TestRotateIfNeededRenamesOversizedFile(t *testing.T) {
	dir := t.TempDir()
	current := logFilePath(dir, fixedTime())
	writeFile(t, current, strings.Repeat("x", 1024*1024+1))

	// 上限 1 MB 而文件写到 1 MB + 1 字节，应被改名并保留 .rotated 标记。
	if err := RotateIfNeeded(dir, 1, 3); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if _, err := os.Stat(current); !os.IsNotExist(err) {
		t.Fatalf("oversized file should be renamed, stat err = %v", err)
	}

	backups, err := filepath.Glob(filepath.Join(dir, "*"+rotatedMarker+"*"))
	if err != nil {
		t.Fatalf("glob backups: %v", err)
	}
	if len(backups) != 1 {
		t.Fatalf("backup count = %d, want 1", len(backups))
	}
}

func TestPruneBackupsKeepsNewestWithinLimit(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"go-studio-2026-01-01-010101.rotated.log",
		"go-studio-2026-01-02-010101.rotated.log",
		"go-studio-2026-01-03-010101.rotated.log",
	} {
		writeFile(t, filepath.Join(dir, name), "payload")
	}

	if err := pruneBackups(dir, 2); err != nil {
		t.Fatalf("prune: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("backup count = %d, want 2", len(entries))
	}
	if entries[0].Name() != "go-studio-2026-01-02-010101.rotated.log" {
		t.Errorf("oldest kept = %q, want the second newest", entries[0].Name())
	}
}

// writeFile 在测试目录写入文件，失败时立即终止测试。
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// fixedTime 返回一个固定时刻，保证日志文件名在测试中可预测。
func fixedTime() time.Time {
	return time.Date(2026, time.September, 26, 12, 0, 0, 0, time.UTC)
}
