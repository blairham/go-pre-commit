// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

//go:build !(linux || darwin || freebsd || netbsd || openbsd || dragonfly)

package languages

import (
	"errors"
	"os/exec"
)

// Upstream has no pty on Windows (cmd_output_p is cmd_output_b there), and
// this tool has none on platforms it does not build pty support for: hooks
// always run with pipes.
const ptySupported = false

func runInPty(*exec.Cmd) ([]byte, error) {
	return nil, errors.New("no pty on this platform")
}
