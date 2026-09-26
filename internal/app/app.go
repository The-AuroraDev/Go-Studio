// app.go — Wails 绑定宿主：持有配置与日志，向前端暴露受控的应用级接口。
// SPDX-License-Identifier: MIT

package app

import (
	"fmt"
	"sync"

	"github.com/29anan29/Go-Studio/internal/config"
	"github.com/29anan29/Go-Studio/internal/log"
)

// App 是绑定给前端的结构体。它的每一个导出方法都会成为前端可调用接口，
// 因此新增导出方法等同于新增对外契约，必须同步补充文档与测试；
// 仅供内部使用的逻辑一律走未导出方法。
type App struct {
	mu     sync.RWMutex
	cfg    config.Config
	logger *log.Logger
}

// New 创建绑定宿主。logger 为 nil 表示日志尚未初始化，
// 记录方法会退化为空操作而不是崩溃，保证启动早期仍可继续。
func New(cfg config.Config, logger *log.Logger) *App {
	return &App{cfg: cfg, logger: logger}
}

// Config 返回当前生效的完整配置，前端启动时用它初始化本地状态。
func (a *App) Config() config.Config {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.cfg
}

// SetTheme 校验并持久化主题取值。非法取值返回错误而不改变当前状态，
// 避免前端因为拼写错误把界面切成不可用配色。
func (a *App) SetTheme(theme string) error {
	switch theme {
	case config.ThemeDark, config.ThemeLight, config.ThemeSystem:
	default:
		return fmt.Errorf("unsupported theme %q", theme)
	}

	a.mu.Lock()
	a.cfg.Theme = theme
	snapshot := a.cfg
	a.mu.Unlock()

	if err := config.SaveUser(snapshot); err != nil {
		return fmt.Errorf("persist theme: %w", err)
	}
	a.info("theme changed", "theme", theme)
	return nil
}

// LogFilePath 返回当前日志文件路径，供设置界面展示与问题反馈。
func (a *App) LogFilePath() string {
	if a.logger == nil {
		return ""
	}
	return a.logger.LogFilePath()
}

// info 记录关键状态变化，日志未初始化时静默跳过。
func (a *App) info(msg string, args ...any) {
	if a.logger != nil {
		a.logger.Info(msg, args...)
	}
}

// errorf 记录需要人工介入的失败，日志未初始化时静默跳过。
func (a *App) errorf(msg string, args ...any) {
	if a.logger != nil {
		a.logger.Error(msg, args...)
	}
}
