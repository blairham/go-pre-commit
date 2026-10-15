// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package pyrandom

import (
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// seed is upstream pre-commit's lang_base.FIXED_RANDOM_SEED.
const seed = 1542676187

// Every expectation below was captured from CPython 3.15 with
//
//	python3 -I -c 'import random; r=random.Random(); r.seed(1542676187, version=1); s=list(range(N)); r.shuffle(s); print(s)'
//
// (and the equivalent for the other calls), not computed by this package.

func TestGetRandBitsMatchesCPython(t *testing.T) {
	r := New(seed)
	got := []uint64{r.GetRandBits(32), r.GetRandBits(32), r.GetRandBits(32), r.GetRandBits(64), r.GetRandBits(5)}
	want := []uint64{2472326011, 2061721674, 2625518588, 5771775725549008018, 10}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestSeedMatchesCPython(t *testing.T) {
	// random.seed(int) uses abs(int), and splits wide ints into 32-bit words.
	for _, tc := range []struct {
		seed int64
		want uint32
	}{
		{-seed, 2472326011},
		{0, 3626764237},
		{1<<40 + 5, 2166296868},
	} {
		if got := New(tc.seed).Uint32(); got != tc.want {
			t.Errorf("seed %d: first output %d, want %d", tc.seed, got, tc.want)
		}
	}
}

func shuffledRange(count int) []int {
	s := make([]int, count)
	for i := range s {
		s[i] = i
	}
	Shuffle(New(seed), s)
	return s
}

func TestShuffleMatchesCPython(t *testing.T) {
	for _, tc := range []struct {
		want []int
		n    int
	}{
		{[]int{}, 0},
		{[]int{0}, 1},
		{[]int{0, 1}, 2},
		{[]int{0, 1, 2}, 3},
		{[]int{4, 0, 5, 1, 8, 6, 2, 3, 7, 9}, 10},
		{[]int{22, 8, 9, 5, 17, 0, 2, 14, 16, 12, 3, 21, 24, 1, 11, 13, 4, 7, 23, 20, 10, 6, 19, 15, 18}, 25},
	} {
		if got := shuffledRange(tc.n); !slices.Equal(got, tc.want) {
			t.Errorf("n=%d: got %v, want %v", tc.n, got, tc.want)
		}
	}
}

func TestShuffle1000MatchesCPython(t *testing.T) {
	raw, err := os.ReadFile("testdata/shuffle1000.txt")
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Split(strings.Trim(strings.TrimSpace(string(raw)), "[]"), ", ")
	want := make([]int, len(fields))
	for i, f := range fields {
		if want[i], err = strconv.Atoi(f); err != nil {
			t.Fatalf("testdata field %d: %v", i, err)
		}
	}
	if len(want) != 1000 {
		t.Fatalf("testdata has %d entries, want 1000", len(want))
	}
	if got := shuffledRange(1000); !slices.Equal(got, want) {
		t.Fatalf("1000-element permutation differs from CPython's\ngot  %v\nwant %v", got[:20], want[:20])
	}
}

func TestShuffleStringsMatchesCPython(t *testing.T) {
	got := []string{"README.md", "main.go", "go.mod", "internal/a.go", "internal/b.go", "x y.txt", "z.py", ".gitignore"}
	Shuffle(New(seed), got)
	want := []string{"README.md", "x y.txt", "internal/a.go", "z.py", "go.mod", "main.go", "internal/b.go", ".gitignore"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}
