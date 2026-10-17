// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

//go:build !unix

package cli

// stdoutColumns falls back to shutil.get_terminal_size's default width off
// unix.
func stdoutColumns() int { return 0 }
