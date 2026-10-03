// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package languages

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"syscall"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

// ptySupported reports whether hooks can run in a pseudo-terminal here.
// Upstream has no pty on Windows either, and falls back to pipes there.
const ptySupported = true

// runInPty is upstream's cmd_output_p: the hook's stdout and stderr are the
// slave side of a fresh pty, so isatty() is true for both and tools that color
// only for a terminal color their output; stdin is /dev/null. Output
// post-processing is switched off on the slave, as upstream does, so "\n" is
// not rewritten to "\r\n" -- the bytes are what the hook wrote.
func runInPty(cmd *exec.Cmd) ([]byte, error) {
	ptmx, tty, err := pty.Open()
	if err != nil {
		return nil, err
	}
	defer ptmx.Close()

	if t, err := unix.IoctlGetTermios(int(tty.Fd()), ioctlGetTermios); err == nil {
		t.Oflag &^= unix.ONLCR | unix.OPOST
		_ = unix.IoctlSetTermios(int(tty.Fd()), ioctlSetTermios, t)
	}

	devnull, err := os.Open(os.DevNull)
	if err != nil {
		tty.Close()
		return nil, err
	}
	defer devnull.Close()

	cmd.Stdin = devnull
	cmd.Stdout = tty
	cmd.Stderr = tty
	if err := cmd.Start(); err != nil {
		tty.Close()
		return nil, err
	}
	// The child holds its own copy; ours must go, or the read below never
	// sees the end of the output.
	tty.Close()

	var buf bytes.Buffer
	_, rerr := io.Copy(&buf, ptmx)
	// Linux reports the closed slave as EIO rather than EOF; both mean done.
	if rerr != nil && !errors.Is(rerr, syscall.EIO) {
		_ = cmd.Wait()
		return buf.Bytes(), rerr
	}
	return buf.Bytes(), cmd.Wait()
}
