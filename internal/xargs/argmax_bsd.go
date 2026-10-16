// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package xargs

import "golang.org/x/sys/unix"

// argMax is sysconf(_SC_ARG_MAX), which the BSDs answer from kern.argmax.
func argMax() int {
	n, err := unix.SysctlUint32("kern.argmax")
	if err != nil || n == 0 {
		return 1 << 18
	}
	return int(n)
}
