// lifecycle.go — Wails 生命周期钩子：启动与关闭阶段释放资源。
// SPDX-License-Identifier: MIT

package app

import "context"

// Lifecycle 承载 Wails 的 OnStartup 与 OnShutdown 钩子。
// 它刻意不放进 options.App 的 Bind 列表，因此不会出现在前端绑定接口中，
// 前端无法直接触发启动与关闭流程。
type Lifecycle struct {
	app *App
}

// NewLifecycle 创建一个绑定到 app 的生命周期钩子集合。
func NewLifecycle(app *App) *Lifecycle {
	return &Lifecycle{app: app}
}

// Startup 在 Wails 启动完成、上下文可用后调用，记录可用的日志文件位置。
func (l *Lifecycle) Startup(context.Context) {
	l.app.info("app started", "logFile", l.app.LogFilePath())
}

// Shutdown 在应用退出前调用，负责关闭日志文件。
func (l *Lifecycle) Shutdown(context.Context) {
	l.app.info("app shutting down")
	if l.app.logger == nil {
		return
	}
	if err := l.app.logger.Close(); err != nil {
		l.app.errorf("close logger failed", "err", err)
	}
}
