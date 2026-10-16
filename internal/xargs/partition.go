// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package xargs

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf16"
)

// ArgumentTooLongError is upstream's xargs.ArgumentTooLongError: a single
// argument that does not fit in a command line on its own.
type ArgumentTooLongError struct {
	Arg string
}

func (e *ArgumentTooLongError) Error() string {
	return "ArgumentTooLongError: " + e.Arg
}

// CPUCount is upstream's xargs.cpu_count. Go's runtime.NumCPU already honors
// the process's CPU affinity mask on Linux, as os.sched_getaffinity does, and
// falls back to the logical CPU count elsewhere, as multiprocessing.cpu_count
// does.
func CPUCount() int {
	if n := runtime.NumCPU(); n > 0 {
		return n
	}
	return 1
}

// TargetConcurrency is upstream's lang_base.target_concurrency: 1 when
// PRE_COMMIT_NO_CONCURRENCY is set, 2 under TRAVIS, and otherwise the CPU
// count. Both variables count when set at all, even to an empty string.
func TargetConcurrency() int {
	if _, ok := os.LookupEnv("PRE_COMMIT_NO_CONCURRENCY"); ok {
		return 1
	}
	if _, ok := os.LookupEnv("TRAVIS"); ok {
		return 2
	}
	return CPUCount()
}

// environSize is upstream's xargs._environ_size: a pointer per entry of envp
// plus each "k=v\0" string.
func environSize(env []string) int {
	size := 8 * len(env)
	for _, kv := range env {
		size += len(kv) + 1
	}
	return size
}

// PlatformMaxLength is upstream's xargs._get_platform_max_length: on POSIX,
// ARG_MAX less 2048 bytes of headroom and the size of the environment,
// clamped to [2**12, 2**17]; on Windows, the UNICODE_STRING maximum less
// headroom.
func PlatformMaxLength() int {
	switch runtime.GOOS {
	case "windows":
		return 1<<15 - 2048
	case "plan9", "js", "wasip1":
		return 1 << 12
	}
	maximum := argMax() - 2048 - environSize(os.Environ())
	return max(min(maximum, 1<<17), 1<<12)
}

// MaxLength is the command-line limit upstream's xargs partitions cmd
// against. On Windows a .bat or .cmd runs through cmd.exe, which has its own
// 8192-character limit; upstream subtracts the cmd.exe invocation and 1024
// characters of headroom for expansion inside the batch file.
func MaxLength(cmd []string) int {
	if runtime.GOOS == "windows" && len(cmd) > 0 {
		lower := strings.ToLower(cmd[0])
		if strings.HasSuffix(lower, ".bat") || strings.HasSuffix(lower, ".cmd") {
			cmdExe, err := exec.LookPath("cmd.exe")
			if err != nil {
				cmdExe = filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe")
			}
			return 8192 - len(cmdExe) - len(" /c ") - 1024
		}
	}
	return PlatformMaxLength()
}

// commandLength is upstream's xargs._command_length: the space-joined command
// measured in UTF-16 code units on Windows and in bytes elsewhere.
func commandLength(args ...string) int {
	full := strings.Join(args, " ")
	if runtime.GOOS == "windows" {
		return len(utf16.Encode([]rune(full)))
	}
	return len(full)
}

// Partition is upstream's xargs.partition. It splits varargs, in order, into
// runs of at most max(4, ceil(len(varargs)/targetConcurrency)) arguments,
// each also short enough that cmd plus the run fits within maxLength. It
// returns the argument runs without cmd; with no varargs there is a single
// empty run, so the command still runs once.
func Partition(cmd, varargs []string, targetConcurrency, maxLength int) ([][]string, error) {
	if targetConcurrency < 1 {
		targetConcurrency = 1
	}
	maxArgs := max(4, (len(varargs)+targetConcurrency-1)/targetConcurrency)

	cmdLength := commandLength(cmd...) + 1
	var ret [][]string
	cur := []string{}
	total := cmdLength
	for i := 0; i < len(varargs); {
		arg := varargs[i]
		argLength := commandLength(arg) + 1
		switch {
		case total+argLength <= maxLength && len(cur) < maxArgs:
			cur = append(cur, arg)
			total += argLength
			i++
		case len(cur) == 0:
			return nil, &ArgumentTooLongError{Arg: arg}
		default:
			ret = append(ret, cur)
			cur = []string{}
			total = cmdLength
		}
	}
	return append(ret, cur), nil
}
