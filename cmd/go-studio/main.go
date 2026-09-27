// main.go — Go Studio 入口：装配配置与日志、启动终端后端、驱动事件循环。
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/29anan29/Go-Studio/internal/config"
	"github.com/29anan29/Go-Studio/internal/log"
	"github.com/29anan29/Go-Studio/internal/screen"
)

// usage 是 -h 输出的帮助文本。
const usage = `Go Studio — 面向 Go 的终端 IDE

用法:
  go-studio [选项] [文件...]

选项:
  -version        打印版本号后退出
  -log-level      覆盖日志级别：debug、info、warn、error
  -workspace      指定工作区根目录，默认为当前目录
  -help           显示本帮助
`

// version 由构建时通过 -ldflags 注入。
var version = "0.1.0-dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "go-studio: %v\n", err)
		os.Exit(1)
	}
}

// options 是解析后的命令行参数。
type options struct {
	showVersion bool
	showHelp    bool
	logLevel    string
	workspace   string
	files       []string
}

// parseArgs 解析命令行参数。只接受已知选项，遇到未知项直接报错。
func parseArgs(args []string) (options, error) {
	var opts options
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-h", "--help", "-help":
			opts.showHelp = true
		case "-version", "--version":
			opts.showVersion = true
		case "-log-level":
			if i+1 >= len(args) {
				return opts, errors.New("-log-level 缺少取值")
			}
			i++
			opts.logLevel = args[i]
		case "-workspace":
			if i+1 >= len(args) {
				return opts, errors.New("-workspace 缺少取值")
			}
			i++
			opts.workspace = args[i]
		default:
			if len(arg) > 1 && arg[0] == '-' {
				return opts, fmt.Errorf("未知选项 %q", arg)
			}
			opts.files = append(opts.files, arg)
		}
	}
	return opts, nil
}

func run(args []string) error {
	opts, err := parseArgs(args)
	if err != nil {
		return err
	}
	if opts.showHelp {
		fmt.Print(usage)
		return nil
	}
	if opts.showVersion {
		fmt.Printf("go-studio %s\n", version)
		return nil
	}

	// TUI 占用 stdout，日志与错误只能走文件或 stderr。
	logger, cfg, err := bootstrap(opts)
	if err != nil {
		return err
	}
	defer func() {
		if err := logger.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "go-studio: 关闭日志失败: %v\n", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	screenBackend, err := screen.NewBubble(screen.BubbleOptions{Context: ctx})
	if err != nil {
		return fmt.Errorf("启动终端后端失败: %w", err)
	}
	defer func() {
		if err := screenBackend.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "go-studio: 恢复终端失败: %v\n", err)
		}
	}()

	// 尺寸是最先到达的消息，等到它才有意义地渲染第一帧。
	if !waitForSize(ctx, screenBackend) {
		return nil
	}
	logger.Info("terminal ready", screenBackend.Caps().LogKeyValue()...)

	return appLoop(ctx, screenBackend, logger, cfg, opts.files)
}

// bootstrap 读取配置并建立日志器。配置损坏时回退到默认值并继续启动。
func bootstrap(opts options) (*log.Logger, config.Config, error) {
	cfg, err := config.Load(opts.workspace)
	if err != nil {
		fmt.Fprintf(os.Stderr, "go-studio: 配置不可用，回退到默认值: %v\n", err)
		cfg = config.Default()
	}
	if opts.logLevel != "" {
		cfg.Log.Level = opts.logLevel
	}

	logsDir, err := config.LogsDir()
	if err != nil {
		return nil, cfg, fmt.Errorf("无法定位日志目录: %w", err)
	}
	logger, err := log.New(log.Options{
		Dir:        logsDir,
		Level:      cfg.Log.Level,
		MaxSizeMB:  cfg.Log.MaxSizeMB,
		MaxBackups: cfg.Log.MaxBackups,
	})
	if err != nil {
		return nil, cfg, fmt.Errorf("无法建立日志: %w", err)
	}
	logger.Info("go-studio starting", "version", version, "logLevel", cfg.Log.Level)
	return logger, cfg, nil
}

// waitForSize 等待首个有效窗口尺寸事件。
// 宽度或高度为 0 的尺寸必须忽略：某些终端与复用器在窗口尚未布局时会上报 0x0，
// 拿它当就绪信号会让程序对着空屏渲染。
func waitForSize(ctx context.Context, backend screen.Screen) bool {
	events := backend.Events()
	for {
		select {
		case <-ctx.Done():
			return false
		case event, ok := <-events:
			if !ok {
				return false
			}
			if event.Kind == screen.EventResize && event.Width > 0 && event.Height > 0 {
				return true
			}
		}
	}
}

// appLoop 是 M0 的占位事件循环：把事件原样记进日志后丢弃。
// M2 起由编辑器与界面接管。
func appLoop(
	ctx context.Context,
	backend screen.Screen,
	logger *log.Logger,
	cfg config.Config,
	files []string,
) error {
	logger.Debug("event loop started", "files", files, "keymap", cfg.General.Keymap)

	// 定期输出心跳，确认事件循环存活且日志在写。
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	events := backend.Events()
	for {
		select {
		case <-ctx.Done():
			logger.Info("shutting down")
			return nil

		case <-ticker.C:
			logger.Debug("event loop alive")

		case event, ok := <-events:
			if !ok {
				logger.Info("terminal closed")
				return nil
			}
			logger.Debug("event", "kind", event.Kind.String(), "keystroke", event.Keystroke)
		}
	}
}
