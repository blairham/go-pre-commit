// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package fsutil

import "os"

// Executable is Python's os.access(path, os.X_OK), which on Windows only
// asks whether path exists.
func Executable(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
