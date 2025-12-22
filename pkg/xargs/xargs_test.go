package xargs

import (
	"os"
	"strings"
	"testing"
)

func TestCPUCount(t *testing.T) {
	count := CPUCount()
	if count < 1 {
		t.Errorf("CPUCount() = %d, want >= 1", count)
	}
}

func TestTargetConcurrency(t *testing.T) {
	// Test default returns cpu count (when no special env vars are set)
	// We can't easily test env vars without modifying global state
	got := TargetConcurrency()
	if got < 1 {
		t.Errorf("TargetConcurrency() = %d, want >= 1", got)
	}
}

func TestTargetConcurrencyNoConc(t *testing.T) {
	old := os.Getenv("PRE_COMMIT_NO_CONCURRENCY")
	os.Setenv("PRE_COMMIT_NO_CONCURRENCY", "1")
	defer os.Setenv("PRE_COMMIT_NO_CONCURRENCY", old)

	got := TargetConcurrency()
	if got != 1 {
		t.Errorf("TargetConcurrency() with PRE_COMMIT_NO_CONCURRENCY = %d, want 1", got)
	}
}

func TestCommandLength(t *testing.T) {
	tests := []struct {
		name string
		cmd  []string
		want int
	}{
		{
			name: "single word",
			cmd:  []string{"echo"},
			want: 4, // "echo"
		},
		{
			name: "two words",
			cmd:  []string{"echo", "hello"},
			want: 10, // "echo hello"
		},
		{
			name: "empty",
			cmd:  []string{},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CommandLength(tt.cmd...)
			if got != tt.want {
				t.Errorf("CommandLength() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestPartitionTrivial(t *testing.T) {
	got := Partition([]string{"cmd"}, []string{}, 1, 100)
	if len(got) != 1 {
		t.Errorf("Partition trivial = %d partitions, want 1", len(got))
	}
}

func TestPartitionSimple(t *testing.T) {
	got := Partition([]string{"cmd"}, []string{"a", "b"}, 1, 100)
	if len(got) != 1 {
		t.Errorf("Partition simple = %d partitions, want 1", len(got))
	}
	if len(got[0]) != 3 { // cmd + a + b
		t.Errorf("Partition simple[0] length = %d, want 3", len(got[0]))
	}
}

func TestPartitionLengthLimit(t *testing.T) {
	// Very short max length forces splitting
	got := Partition([]string{"echo"}, []string{"aaaa", "bbbb", "cccc"}, 1, 15)
	if len(got) < 2 {
		t.Errorf("Partition with short length = %d partitions, want >= 2", len(got))
	}
}

func TestShuffledDeterministic(t *testing.T) {
	input := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}

	result1 := Shuffled(input)
	result2 := Shuffled(input)

	if len(result1) != len(result2) {
		t.Errorf("Shuffled results have different lengths: %d vs %d", len(result1), len(result2))
	}

	for i := range result1 {
		if result1[i] != result2[i] {
			t.Errorf("Shuffled results differ at index %d: %q vs %q", i, result1[i], result2[i])
		}
	}
}

func TestShuffledEmpty(t *testing.T) {
	input := []string{}
	result := Shuffled(input)
	if len(result) != 0 {
		t.Errorf("Shuffled(empty) = %v, want empty", result)
	}
}

func TestRun(t *testing.T) {
	result := Run([]string{"echo", "hello"}, []string{}, nil)
	if result.ExitCode != 0 {
		t.Errorf("Run(echo hello) exit code = %d, want 0", result.ExitCode)
	}
	if !strings.Contains(string(result.Output), "hello") {
		t.Errorf("Run(echo hello) output = %q, want to contain 'hello'", result.Output)
	}
}

func TestRunWithFiles(t *testing.T) {
	files := []string{"file1.txt", "file2.txt"}
	result := Run([]string{"echo"}, files, nil)
	if result.ExitCode != 0 {
		t.Errorf("Run(echo) exit code = %d, want 0", result.ExitCode)
	}
	output := string(result.Output)
	for _, f := range files {
		if !strings.Contains(output, f) {
			t.Errorf("Run output = %q, want to contain %q", output, f)
		}
	}
}

func TestRunSerial(t *testing.T) {
	files := []string{"a", "b", "c"}
	cfg := &Config{RequireSerial: true}
	result := Run([]string{"echo"}, files, cfg)
	if result.ExitCode != 0 {
		t.Errorf("Run serial exit code = %d, want 0", result.ExitCode)
	}
}

func TestAbs(t *testing.T) {
	tests := []struct {
		input int
		want  int
	}{
		{0, 0},
		{5, 5},
		{-5, 5},
		{-1, 1},
		{1, 1},
	}

	for _, tt := range tests {
		got := abs(tt.input)
		if got != tt.want {
			t.Errorf("abs(%d) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestGetPlatformMaxLength(t *testing.T) {
	maxLength := GetPlatformMaxLength()
	if maxLength < MinMaxLength {
		t.Errorf("GetPlatformMaxLength() = %d, want >= %d", maxLength, MinMaxLength)
	}
}
