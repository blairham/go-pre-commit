package metahooks

import (
	"strings"
	"testing"
)

func TestIdentity(t *testing.T) {
	executor := NewMetaHookExecutor(".pre-commit-config.yaml", false)

	tests := []struct {
		name     string
		files    []string
		wantCode int
	}{
		{
			name:     "single file",
			files:    []string{"file1.txt"},
			wantCode: 0,
		},
		{
			name:     "multiple files",
			files:    []string{"file1.txt", "file2.txt", "subdir/file3.go"},
			wantCode: 0,
		},
		{
			name:     "empty files",
			files:    []string{},
			wantCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, output := executor.Identity(tt.files)

			if code != tt.wantCode {
				t.Errorf("Identity() code = %d, want %d", code, tt.wantCode)
			}

			// Verify output contains all files
			for _, file := range tt.files {
				if !strings.Contains(output, file) {
					t.Errorf("Identity() output should contain %q", file)
				}
			}
		})
	}
}

func TestIsMetaHook(t *testing.T) {
	tests := []struct {
		hookID string
		want   bool
	}{
		{"identity", true},
		{"check-hooks-apply", true},
		{"check-useless-excludes", true},
		{"trailing-whitespace", false},
		{"some-random-hook", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.hookID, func(t *testing.T) {
			got := IsMetaHook(tt.hookID)
			if got != tt.want {
				t.Errorf("IsMetaHook(%q) = %v, want %v", tt.hookID, got, tt.want)
			}
		})
	}
}

func TestPatternMatchesAnyFile(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		files   []string
		want    bool
	}{
		{
			name:    "simple match",
			pattern: `\.py$`,
			files:   []string{"test.py", "main.py"},
			want:    true,
		},
		{
			name:    "no match",
			pattern: `\.py$`,
			files:   []string{"test.go", "main.js"},
			want:    false,
		},
		{
			name:    "partial match",
			pattern: `test`,
			files:   []string{"test.py", "main.py"},
			want:    true,
		},
		{
			name:    "invalid pattern",
			pattern: `[invalid`,
			files:   []string{"test.py"},
			want:    false,
		},
		{
			name:    "empty files",
			pattern: `\.py$`,
			files:   []string{},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := patternMatchesAnyFile(tt.pattern, tt.files)
			if got != tt.want {
				t.Errorf("patternMatchesAnyFile(%q, %v) = %v, want %v",
					tt.pattern, tt.files, got, tt.want)
			}
		})
	}
}

func TestExecuteMetaHook(t *testing.T) {
	tests := []struct {
		name       string
		hookID     string
		files      []string
		wantCode   int
		wantOutput string
	}{
		{
			name:       "identity with files",
			hookID:     "identity",
			files:      []string{"file1.txt", "file2.txt"},
			wantCode:   0,
			wantOutput: "file1.txt",
		},
		{
			name:       "unknown hook",
			hookID:     "unknown-hook",
			files:      []string{},
			wantCode:   1,
			wantOutput: "Unknown meta hook",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, output := ExecuteMetaHook(tt.hookID, tt.files, ".pre-commit-config.yaml", false)

			if code != tt.wantCode {
				t.Errorf("ExecuteMetaHook() code = %d, want %d", code, tt.wantCode)
			}

			if !strings.Contains(output, tt.wantOutput) {
				t.Errorf("ExecuteMetaHook() output = %q, should contain %q", output, tt.wantOutput)
			}
		})
	}
}

func TestNewMetaHookExecutor(t *testing.T) {
	executor := NewMetaHookExecutor("/path/to/config.yaml", true)

	if executor == nil {
		t.Fatal("NewMetaHookExecutor() returned nil")
	}

	if executor.configPath != "/path/to/config.yaml" {
		t.Errorf("configPath = %q, want %q", executor.configPath, "/path/to/config.yaml")
	}

	if !executor.verbose {
		t.Error("verbose should be true")
	}
}
