// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package fsutil

import "golang.org/x/sys/unix"

// Executable is Python's os.access(path, os.X_OK): whether this process may
// execute path, by access(2).
func Executable(path string) bool {
	return unix.Access(path, unix.X_OK) == nil
}

// Writable is Python's os.access(path, os.W_OK): whether this process may
// write to path, by access(2).
func Writable(path string) bool {
	return unix.Access(path, unix.W_OK) == nil
}
