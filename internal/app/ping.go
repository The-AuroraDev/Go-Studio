// ping.go — 前后端连通性自检：状态栏用它确认 Wails 绑定链路真实可用。
// SPDX-License-Identifier: MIT

package app

import "fmt"

// PingVersion 是自检协议版本，前端据此判断后端能力是否匹配。
const PingVersion = 1

// Ping 回显传入的消息并附带协议版本，用于验证前端到 Go 的完整往返链路。
// 消息为空时是合法调用，自检不应因为调用方没传参而失败。
func (a *App) Ping(message string) (string, error) {
	if message == "" {
		return fmt.Sprintf("pong v%d", PingVersion), nil
	}
	return fmt.Sprintf("pong v%d: %s", PingVersion, message), nil
}

// PingVersion 返回自检协议版本，供前端做能力协商。
func (a *App) PingVersion() int {
	return PingVersion
}
