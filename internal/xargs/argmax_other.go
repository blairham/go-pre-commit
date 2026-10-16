// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

//go:build !linux && !darwin && !dragonfly && !freebsd && !netbsd && !openbsd

package xargs

// argMax is the POSIX minimum ARG_MAX where there is no portable way to ask.
// PlatformMaxLength does not consult it on Windows.
func argMax() int {
	return 1 << 12
}
