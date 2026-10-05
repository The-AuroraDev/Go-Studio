// tty_termios_bsd.go — Darwin 与 BSD 系的 termios 读写。
// SPDX-License-Identifier: MIT

// BSD 系不用 TCGETS/TCSETS，而是 TIOCGETA/TIOCSETA，且请求码是 uint。

//go:build darwin || dragonfly || freebsd || netbsd || openbsd || hurd

package screen

import "golang.org/x/sys/unix"

// termios 是本平台的终端设置类型。
type termios = unix.Termios

// getTermios 读取 fd 的终端设置。fd 不是终端时返回错误。
func getTermios(fd int) (termios, error) {
	state, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		return termios{}, err
	}
	return *state, nil
}

// setTermios 把设置写回 fd。
//
// 用 TIOCSETA 而非 TIOCSETAW：后者会等输出排空，一旦输出被阻塞就会挂住，
// 而卡住的退出比少写几个转义序列更糟。重复调用是安全的。
func setTermios(fd int, state termios) error {
	return unix.IoctlSetTermios(fd, unix.TIOCSETA, &state)
}
