// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package cli

import (
	"os"

	"golang.org/x/sys/unix"
)

// stdoutColumns is os.get_terminal_size(sys.__stdout__.fileno()).columns,
// or 0 when stdout is not a terminal.
func stdoutColumns() int {
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return 0
	}
	return int(ws.Col)
}
