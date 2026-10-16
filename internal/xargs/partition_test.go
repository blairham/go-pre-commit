// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package xargs

import (
	"errors"
	"os"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func repeat(s string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = s
	}
	return out
}

// The cases below are upstream's tests/xargs_test.py partition tests.

func TestPartition(t *testing.T) {
	dots := strings.Repeat
	tests := []struct {
		name        string
		cmd         []string
		varargs     []string
		want        [][]string
		concurrency int
		maxLength   int
	}{
		{"trivial", []string{"cmd"}, nil, [][]string{{}}, 1, 4096},
		{"simple", []string{"cmd"}, []string{"foo"}, [][]string{{"foo"}}, 1, 4096},
		{
			"limits",
			[]string{"ninechars"},
			[]string{dots(".", 5), dots(".", 4), dots(".", 10), dots(".", 5), dots(".", 6)},
			[][]string{{dots(".", 5), dots(".", 4)}, {dots(".", 10)}, {dots(".", 5)}, {dots(".", 6)}},
			1, 21,
		},
		{
			"target concurrency",
			[]string{"foo"},
			repeat("A", 22),
			[][]string{repeat("A", 6), repeat("A", 6), repeat("A", 6), repeat("A", 4)},
			4, 50,
		},
		{
			"target concurrency won't make tiny partitions",
			[]string{"foo"},
			repeat("A", 10),
			[][]string{repeat("A", 4), repeat("A", 4), repeat("A", 2)},
			4, 50,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Partition(tc.cmd, tc.varargs, tc.concurrency, tc.maxLength)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Partition = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPartitionUnicodeLength(t *testing.T) {
	cmd := []string{"ninechars"}
	// U+1F611 is 4 UTF-8 bytes but 2 UTF-16 code units.
	varargs := []string{strings.Repeat("\U0001F611", 5)}
	limit := 31 // upstream's test_partition_limit_linux
	if runtime.GOOS == "windows" {
		limit = 21 // test_partition_limit_win32
	}
	got, err := Partition(cmd, varargs, 1, limit)
	if err != nil || !reflect.DeepEqual(got, [][]string{varargs}) {
		t.Errorf("Partition = %q, %v; want one partition", got, err)
	}
	if runtime.GOOS != "windows" {
		_, err := Partition(cmd, []string{strings.Repeat("\U0001F611", 10)}, 1, 20)
		var tooLong *ArgumentTooLongError
		if !errors.As(err, &tooLong) {
			t.Errorf("err = %v, want ArgumentTooLongError", err)
		}
	}
}

func TestPartitionArgumentTooLong(t *testing.T) {
	_, err := Partition([]string{"aaaaa"}, []string{"aaaaa"}, 1, 10)
	var tooLong *ArgumentTooLongError
	if !errors.As(err, &tooLong) || tooLong.Arg != "aaaaa" {
		t.Errorf("err = %v, want ArgumentTooLongError(aaaaa)", err)
	}
}

func TestEnvironSize(t *testing.T) {
	// upstream's test_environ_size
	tests := []struct {
		env  []string
		want int
	}{
		{nil, 0},
		{[]string{"x=1"}, 12},
		{[]string{"x=12"}, 13},
		{[]string{"x=1", "y=2"}, 24},
	}
	for _, tc := range tests {
		if got := environSize(tc.env); got != tc.want {
			t.Errorf("environSize(%q) = %d, want %d", tc.env, got, tc.want)
		}
	}
}

func TestPlatformMaxLength(t *testing.T) {
	got := PlatformMaxLength()
	if runtime.GOOS == "windows" {
		if got != 1<<15-2048 {
			t.Errorf("PlatformMaxLength = %d, want %d", got, 1<<15-2048)
		}
		return
	}
	if got < 1<<12 || got > 1<<17 {
		t.Errorf("PlatformMaxLength = %d, want within [4096, 131072]", got)
	}
	// A huge environment pushes the limit down to the floor.
	t.Setenv("GPC_HUGE", strings.Repeat("x", argMax()))
	if got := PlatformMaxLength(); got != 1<<12 {
		t.Errorf("PlatformMaxLength with a huge environment = %d, want 4096", got)
	}
}

func TestTargetConcurrency(t *testing.T) {
	unset := func(keys ...string) {
		for _, k := range keys {
			t.Setenv(k, "")
			os.Unsetenv(k)
		}
	}
	unset("PRE_COMMIT_NO_CONCURRENCY", "TRAVIS")
	if got := TargetConcurrency(); got != CPUCount() {
		t.Errorf("TargetConcurrency = %d, want CPU count %d", got, CPUCount())
	}
	t.Setenv("TRAVIS", "")
	if got := TargetConcurrency(); got != 2 {
		t.Errorf("TargetConcurrency under TRAVIS = %d, want 2", got)
	}
	t.Setenv("PRE_COMMIT_NO_CONCURRENCY", "")
	if got := TargetConcurrency(); got != 1 {
		t.Errorf("TargetConcurrency with NO_CONCURRENCY = %d, want 1", got)
	}
}
