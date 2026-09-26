// log.go — 结构化日志：按级别写入用户配置目录下的 logs/ 并做体积轮转。
// SPDX-License-Identifier: MIT

package log

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// 日志文件命名与轮转参数。文件名带日期，便于按天定位问题。
const (
	filePrefix    = "go-studio-"
	fileSuffix    = ".log"
	rotatedMarker = ".rotated"
	defaultMaxMB  = 8
	defaultKeep   = 5
)

// Options 描述日志输出位置、级别与轮转策略。
type Options struct {
	// Dir 是日志目录，调用方负责确保它位于用户配置目录之下。
	Dir string
	// Level 是最低输出级别：debug、info、warn 或 error。
	Level string
	// MaxSizeMB 是单个日志文件的体积上限，超过后轮转。
	MaxSizeMB int
	// MaxBackups 是保留的历史日志文件个数。
	MaxBackups int
	// Console 为真时额外把日志写到标准错误，便于开发期观察。
	Console bool
}

// Logger 封装 slog.Logger，额外负责日志文件句柄的所有权与关闭。
type Logger struct {
	slog *slog.Logger
	file *os.File
	mu   sync.Mutex
}

// New 按给定选项创建日志器。目录与轮转失败都会返回错误而不是静默降级，
// 避免开发者误以为日志已经落盘。
func New(opts Options) (*Logger, error) {
	level, err := ParseLevel(opts.Level)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(opts.Dir, 0o755); err != nil {
		return nil, fmt.Errorf("create log dir %s: %w", opts.Dir, err)
	}
	if err := RotateIfNeeded(opts.Dir, opts.MaxSizeMB, opts.MaxBackups); err != nil {
		return nil, err
	}

	path := logFilePath(opts.Dir, time.Now())
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open log file %s: %w", path, err)
	}

	var writer io.Writer = file
	if opts.Console {
		writer = io.MultiWriter(file, os.Stderr)
	}
	handler := newRedactHandler(slog.NewJSONHandler(writer, &slog.HandlerOptions{Level: level}))

	return &Logger{slog: slog.New(handler), file: file}, nil
}

// NewConsole 创建只写标准错误的日志器，用于日志目录不可用时的降级，
// 保证配置或文件系统出错时诊断信息仍然可见。
func NewConsole(level string) (*Logger, error) {
	parsed, err := ParseLevel(level)
	if err != nil {
		return nil, err
	}
	options := &slog.HandlerOptions{Level: parsed}
	handler := newRedactHandler(slog.NewJSONHandler(os.Stderr, options))
	return &Logger{slog: slog.New(handler)}, nil
}

// ParseLevel 把配置里的级别字面量解析为 slog 级别。
func ParseLevel(name string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unknown log level %q", name)
	}
}

// Debug 记录调试细节，正常流程保持安静。
func (l *Logger) Debug(msg string, args ...any) {
	l.slog.Debug(msg, args...)
}

// Info 记录关键状态变化。
func (l *Logger) Info(msg string, args ...any) {
	l.slog.Info(msg, args...)
}

// Warn 记录可恢复的异常，例如回退到默认配置。
func (l *Logger) Warn(msg string, args ...any) {
	l.slog.Warn(msg, args...)
}

// Error 记录失败，需要人工介入。
func (l *Logger) Error(msg string, args ...any) {
	l.slog.Error(msg, args...)
}

// LogFilePath 返回当前生效的日志文件路径，供设置界面与诊断包使用。
func (l *Logger) LogFilePath() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return ""
	}
	return l.file.Name()
}

// Close 关闭日志文件，可重复调用。
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return nil
	}
	err := l.file.Close()
	l.file = nil
	if err != nil {
		return fmt.Errorf("close log file: %w", err)
	}
	return nil
}

// RotateIfNeeded 在当天日志文件超过体积上限时轮转，并裁剪历史备份。
func RotateIfNeeded(dir string, maxSizeMB, maxBackups int) error {
	if maxSizeMB <= 0 {
		maxSizeMB = defaultMaxMB
	}
	if maxBackups <= 0 {
		maxBackups = defaultKeep
	}

	current, err := currentLogFile(dir)
	if err != nil {
		return err
	}
	if current == "" {
		return nil
	}

	info, err := os.Stat(current)
	if err != nil {
		return fmt.Errorf("stat log file %s: %w", current, err)
	}
	if info.Size() < int64(maxSizeMB)*1024*1024 {
		return nil
	}

	// 轮转后的文件名形如 go-studio-2026-09-26-153045.rotated.log，
	// 保留原日期并追加时间，保证字典序仍然与时间序一致。
	stamped := strings.TrimSuffix(current, fileSuffix) +
		"-" + time.Now().Format("150405") + rotatedMarker + fileSuffix
	if err := os.Rename(current, stamped); err != nil {
		return fmt.Errorf("rotate log file %s: %w", current, err)
	}
	return pruneBackups(dir, maxBackups)
}

// currentLogFile 返回目录中最新的一天级日志文件，没有则返回空字符串。
// 文件名以日期结尾，字典序与时间序一致，因此取字典序最大者即可。
func currentLogFile(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("read log dir %s: %w", dir, err)
	}

	newest := ""
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !isDailyLog(name) {
			continue
		}
		if name > newest {
			newest = name
		}
	}
	if newest == "" {
		return "", nil
	}
	return filepath.Join(dir, newest), nil
}

// pruneBackups 删除超出保留数量的历史日志。
func pruneBackups(dir string, maxBackups int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read log dir %s: %w", dir, err)
	}

	var backups []string
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && strings.Contains(name, rotatedMarker) {
			backups = append(backups, name)
		}
	}
	if len(backups) <= maxBackups {
		return nil
	}

	for _, name := range backups[:len(backups)-maxBackups] {
		path := filepath.Join(dir, name)
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove old log %s: %w", path, err)
		}
	}
	return nil
}

// logFilePath 按日期生成当天日志文件的完整路径。
func logFilePath(dir string, now time.Time) string {
	return filepath.Join(dir, filePrefix+now.Format("2006-01-02")+fileSuffix)
}

// isDailyLog 判断文件名是否为按天切分的日志，而不是轮转备份。
func isDailyLog(name string) bool {
	return strings.HasPrefix(name, filePrefix) && strings.HasSuffix(name, fileSuffix)
}
