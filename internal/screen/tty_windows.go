// tty_windows.go — Windows 上没有 termios，终端模式的保存与恢复交给 Bubble Tea。
// SPDX-License-Identifier: MIT

//go:build windows

package screen

import "errors"

// errNoTermios 表示本平台没有 termios 概念。
var errNoTermios = errors.New("screen: termios is not available on this platform")

// termios 在 Windows 上没有对应物，只是一个让 ttyState 能跨平台编译的空壳。
type termios struct{}

// getTermios 恒定失败，于是 captureTTYState 返回 nil、isTerminal 返回 false：
// 这正是「本平台不由我们管终端状态」的语义，Bubble Tea 会自行处理控制台模式。
func getTermios(fd int) (termios, error) {
	return termios{}, errNoTermios
}

// setTermios 是空操作。ttyState.restore 在没有快照时根本不会被调用。
func setTermios(fd int, state termios) error {
	return nil
}
