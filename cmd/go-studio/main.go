// main.go — Go Studio 入口：装配配置与日志、启动终端后端、驱动事件循环。
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/29anan29/Go-Studio/internal/config"
	"github.com/29anan29/Go-Studio/internal/document"
	"github.com/29anan29/Go-Studio/internal/editor"
	"github.com/29anan29/Go-Studio/internal/keymap"
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

	return runEditor(ctx, screenBackend, logger, cfg, opts)
}

// runEditor 装配编辑器并进入主循环。
//
// 初始文档的来源：命令行给了文件就打开它，没给就开一个空的未命名文档。
// 打开失败不是致命错误——一个打不开的文件不该让编辑器整个起不来，
// 退回到空文档并把原因显示在状态栏上，用户仍然可以编辑别的文件。
func runEditor(
	ctx context.Context,
	backend screen.Screen,
	logger *log.Logger,
	cfg config.Config,
	opts options,
) error {
	doc, status := initialDocument(opts.files)
	table, err := keymap.Lookup(cfg.General.Keymap)
	if err != nil {
		// 键位方案不可用必须直接失败：静默退回默认键位会让用户
		// 按文档里的键却毫无反应，而且极难察觉是配置问题。
		return fmt.Errorf("键位方案不可用: %w", err)
	}

	app, err := editor.New(editor.Options{
		Backend: backend,
		Logger:  logger,
		Config:  cfg,
		Table:   table,
		Doc:     doc,
		Status:  status,
	})
	if err != nil {
		return err
	}
	return app.Run(ctx)
}

// initialDocument 按命令行参数决定初始打开哪个文档。
// 第二个返回值是要显示在状态栏上的说明，出错时也会用到。
func initialDocument(files []string) (*document.Document, string) {
	if len(files) == 0 {
		return document.New(), "C-a f 打开文件，M-1 到 M-8 切换标签"
	}

	// 多文件参数先只打开第一个：多标签页属于后续阶段，
	// 与其在这里悄悄丢掉其余文件，不如先明确支持一个。
	path := files[0]
	note := ""
	if len(files) > 1 {
		note = fmt.Sprintf("已忽略 %d 个额外参数（多标签页尚未支持）", len(files)-1)
	}

	doc, err := document.Open(path)
	switch {
	case err == nil:
		return doc, withNote(displayPath(path), note)

	case errors.Is(err, fs.ErrNotExist):
		// 文件还不存在是一个正常场景：用户可能想新建文件。
		// 必须保留路径，否则保存时无处可写——
		// 退回无名空文档会让「打开新文件后保存」这条路直接断掉。
		return document.NewNamed(path, false), withNote(displayPath(path)+"（新建）", note)

	default:
		// 真的打不开（权限、目录、是二进制文件……）才退回未命名文档。
		return document.New(), "打开失败：" + err.Error()
	}
}

// withNote 在状态说明后面追加备注，没有备注时原样返回。
func withNote(text, note string) string {
	if note == "" {
		return text
	}
	return text + "　" + note
}

// displayPath 返回状态栏上显示用的文件名。
func displayPath(path string) string {
	base := path
	if idx := strings.LastIndexAny(base, "/\\"); idx >= 0 {
		base = base[idx+1:]
	}
	return base
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
