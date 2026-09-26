// main.go — Wails 桌面外壳入口：装配配置与日志、挂载前端资源、启动事件循环。
// SPDX-License-Identifier: MIT

package main

import (
	"embed"
	"log/slog"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"github.com/29anan29/Go-Studio/internal/app"
	"github.com/29anan29/Go-Studio/internal/config"
	"github.com/29anan29/Go-Studio/internal/log"
)

// consoleLogEnv 置为 1 时把日志同时输出到标准错误，便于开发期观察。
const consoleLogEnv = "GO_STUDIO_CONSOLE_LOG"

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	cfg, logger := bootstrap()
	application := app.New(cfg, logger)
	lifecycle := app.NewLifecycle(application)

	err := wails.Run(&options.App{
		Title:            "Go Studio",
		Width:            1280,
		Height:           800,
		MinWidth:         640,
		MinHeight:        400,
		BackgroundColour: &options.RGBA{R: 14, G: 15, B: 17, A: 1},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  lifecycle.Startup,
		OnShutdown: lifecycle.Shutdown,
		Bind: []any{
			application,
		},
	})
	if err != nil {
		logger.Error("wails run failed", "err", err)
		slog.Error("Go Studio exited with error", "err", err)
		os.Exit(1)
	}
}

// bootstrap 读取配置并建立日志器。配置损坏时回退到默认值并继续启动，
// 日志目录不可用时降级为仅标准错误输出，两种降级都会留下可检索的痕迹。
func bootstrap() (config.Config, *log.Logger) {
	cfg, err := config.Load("")
	if err != nil {
		slog.Warn("fallback to builtin config", "err", err)
		cfg = config.Default()
	}

	logsDir, dirErr := config.LogsDir()
	if dirErr != nil {
		slog.Warn("resolve log dir failed", "err", dirErr)
	}
	if dirErr == nil {
		logger, logErr := log.New(log.Options{
			Dir:     logsDir,
			Level:   cfg.LogLevel,
			Console: os.Getenv(consoleLogEnv) == "1",
		})
		if logErr == nil {
			logger.Info("logger ready", "dir", logsDir, "level", cfg.LogLevel)
			return cfg, logger
		}
		slog.Warn("file logger unavailable", "err", logErr)
	}

	logger, err := log.NewConsole(cfg.LogLevel)
	if err != nil {
		// 级别字面量非法意味着配置本身不可信，退回 info 至少保证能启动。
		slog.Warn("console logger level invalid, using info", "err", err)
		logger, _ = log.NewConsole(config.Default().LogLevel)
	}
	return cfg, logger
}
