// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package xargs

import "golang.org/x/sys/unix"

// argMax is glibc's sysconf(_SC_ARG_MAX): a quarter of the stack limit, but
// never less than the historical ARG_MAX of 131072.
func argMax() int {
	const legacy = 131072
	var rl unix.Rlimit
	if err := unix.Getrlimit(unix.RLIMIT_STACK, &rl); err != nil {
		return legacy
	}
	// PlatformMaxLength clamps to 2**17 anyway, so anything past 2**30
	// (an unlimited stack included) only has to stay positive.
	if quarter := rl.Cur / 4; quarter < 1<<30 {
		return max(legacy, int(quarter))
	}
	return 1 << 30
}
