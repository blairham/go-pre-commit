package matching

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/blairham/go-pre-commit/pkg/config"
)

func TestNewMatcher(t *testing.T) {
	matcher := NewMatcher()
	assert.NotNil(t, matcher)
	assert.NotNil(t, matcher.typeMatchers)
	assert.Greater(t, len(matcher.typeMatchers), 0)
}

func TestMatcher_GetFilesForHook(t *testing.T) {
	matcher := NewMatcher()

	hook := config.Hook{
		ID:    "test-hook",
		Files: `\.py$`,
	}

	contextFiles := []string{
		"main.py",
		"test.py",
		"README.md",
		"config.go",
	}

	// Test normal mode
	result := matcher.GetFilesForHook(hook, contextFiles, false)
	assert.Len(t, result, 2)
	assert.Contains(t, result, "main.py")
	assert.Contains(t, result, "test.py")

	// Test all files mode
	result = matcher.GetFilesForHook(hook, contextFiles, true)
	assert.Len(t, result, 2) // Still filtered by hook criteria
	assert.Contains(t, result, "main.py")
	assert.Contains(t, result, "test.py")
}

func TestMatcher_FileMatchesHook(t *testing.T) {
	matcher := NewMatcher()

	tests := []struct {
		name     string
		file     string
		hook     config.Hook
		expected bool
	}{
		{
			name: "matches file pattern",
			file: "main.py",
			hook: config.Hook{
				Files: `\.py$`,
			},
			expected: true,
		},
		{
			name: "does not match file pattern",
			file: "main.go",
			hook: config.Hook{
				Files: `\.py$`,
			},
			expected: false,
		},
		{
			name: "matches exclude pattern",
			file: "test_file.py",
			hook: config.Hook{
				Files:        `\.py$`,
				ExcludeRegex: `^test_`,
			},
			expected: false,
		},
		{
			name: "does not match exclude pattern",
			file: "main.py",
			hook: config.Hook{
				Files:        `\.py$`,
				ExcludeRegex: `^test_`,
			},
			expected: true,
		},
		{
			name: "matches type filter",
			file: "main.py",
			hook: config.Hook{
				Types: []string{"python"},
			},
			expected: true,
		},
		{
			name: "does not match type filter",
			file: "main.go",
			hook: config.Hook{
				Types: []string{"python"},
			},
			expected: false,
		},
		{
			name: "matches types_or filter",
			file: "main.py",
			hook: config.Hook{
				TypesOr: []string{"python", "javascript"},
			},
			expected: true,
		},
		{
			name: "excluded by exclude_types",
			file: "main.py",
			hook: config.Hook{
				ExcludeTypes: []string{"python"},
			},
			expected: false,
		},
		{
			name: "not excluded by exclude_types",
			file: "main.go",
			hook: config.Hook{
				ExcludeTypes: []string{"python"},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.FileMatchesHook(tt.file, tt.hook)
			assert.Equal(t, tt.expected, result, "File: %s, Hook: %+v", tt.file, tt.hook)
		})
	}
}

func TestMatcher_matchesFilePattern(t *testing.T) {
	matcher := NewMatcher()

	tests := []struct {
		name     string
		file     string
		pattern  string
		expected bool
	}{
		{
			name:     "empty pattern matches all",
			file:     "any-file.txt",
			pattern:  "",
			expected: true,
		},
		{
			name:     "matches full path",
			file:     "src/main.py",
			pattern:  `src/.*\.py$`,
			expected: true,
		},
		{
			name:     "matches basename",
			file:     "src/main.py",
			pattern:  `main\.py$`,
			expected: true,
		},
		{
			name:     "does not match",
			file:     "src/main.go",
			pattern:  `\.py$`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.matchesFilePattern(tt.file, tt.pattern)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatcher_matchesExcludePattern(t *testing.T) {
	matcher := NewMatcher()

	tests := []struct {
		name         string
		file         string
		excludeRegex string
		expected     bool
	}{
		{
			name:         "empty pattern excludes nothing",
			file:         "any-file.txt",
			excludeRegex: "",
			expected:     false,
		},
		{
			name:         "matches exclude pattern",
			file:         "test_file.py",
			excludeRegex: `^test_`,
			expected:     true,
		},
		{
			name:         "does not match exclude pattern",
			file:         "main.py",
			excludeRegex: `^test_`,
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.matchesExcludePattern(tt.file, tt.excludeRegex)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatcher_matchesTypeFilters(t *testing.T) {
	matcher := NewMatcher()

	tests := []struct {
		name     string
		file     string
		hook     config.Hook
		expected bool
	}{
		{
			name: "matches all types (AND logic)",
			file: "main.py",
			hook: config.Hook{
				Types: []string{"python", "text"},
			},
			expected: true,
		},
		{
			name: "does not match all types",
			file: "main.py",
			hook: config.Hook{
				Types: []string{"python", "javascript"},
			},
			expected: false,
		},
		{
			name: "matches any types_or (OR logic)",
			file: "main.py",
			hook: config.Hook{
				TypesOr: []string{"python", "javascript"},
			},
			expected: true,
		},
		{
			name: "excluded by exclude_types",
			file: "main.py",
			hook: config.Hook{
				ExcludeTypes: []string{"python"},
			},
			expected: false,
		},
		{
			name: "not excluded by exclude_types",
			file: "main.go",
			hook: config.Hook{
				ExcludeTypes: []string{"python"},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.matchesTypeFilters(tt.file, tt.hook)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatcher_matchesTypes(t *testing.T) {
	matcher := NewMatcher()

	tests := []struct {
		name     string
		file     string
		types    []string
		expected bool
	}{
		{
			name:     "matches python type",
			file:     "main.py",
			types:    []string{"python"},
			expected: true,
		},
		{
			name:     "matches javascript type",
			file:     "main.js",
			types:    []string{"javascript"},
			expected: true,
		},
		{
			name:     "matches one of multiple types",
			file:     "main.py",
			types:    []string{"python", "javascript"},
			expected: true,
		},
		{
			name:     "does not match any type",
			file:     "main.go",
			types:    []string{"python", "javascript"},
			expected: false,
		},
		{
			name:     "unknown type",
			file:     "main.unknown",
			types:    []string{"unknown-type"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.matchesTypes(tt.file, tt.types)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatcher_matchesAllTypes(t *testing.T) {
	matcher := NewMatcher()

	tests := []struct {
		name     string
		file     string
		types    []string
		expected bool
	}{
		{
			name:     "matches single type",
			file:     "main.py",
			types:    []string{"python"},
			expected: true,
		},
		{
			name:     "matches all types when possible",
			file:     "main.py",
			types:    []string{"python", "text"},
			expected: true,
		},
		{
			name:     "does not match all types",
			file:     "main.py",
			types:    []string{"python", "javascript"},
			expected: false,
		},
		{
			name:     "unknown type fails",
			file:     "main.py",
			types:    []string{"python", "unknown-type"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.matchesAllTypes(tt.file, tt.types)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatcher_matchesFilePattern_ErrorHandling(t *testing.T) {
	matcher := NewMatcher()

	tests := []struct {
		name     string
		file     string
		pattern  string
		expected bool
	}{
		{
			name:     "invalid regex pattern",
			file:     "main.py",
			pattern:  "[invalid(regex",
			expected: false,
		},
		{
			name:     "complex valid regex",
			file:     "test_file.py",
			pattern:  `^test_.*\.py$`,
			expected: true,
		},
		{
			name:     "regex matches basename but not full path",
			file:     "src/main.py",
			pattern:  `^main\.py$`,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.matchesFilePattern(tt.file, tt.pattern)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatcher_matchesTypeFilters_EdgeCases(t *testing.T) {
	matcher := NewMatcher()

	tests := []struct {
		name     string
		file     string
		hook     config.Hook
		expected bool
	}{
		{
			name: "empty types arrays - should match",
			file: "main.py",
			hook: config.Hook{
				Types:        []string{},
				TypesOr:      []string{},
				ExcludeTypes: []string{},
			},
			expected: true,
		},
		{
			name: "types and types_or both specified",
			file: "main.py",
			hook: config.Hook{
				Types:   []string{"python"},
				TypesOr: []string{"javascript", "python"},
			},
			expected: true,
		},
		{
			name: "types matches but excluded by exclude_types",
			file: "main.py",
			hook: config.Hook{
				Types:        []string{"python"},
				ExcludeTypes: []string{"python"},
			},
			expected: false,
		},
		{
			name: "types_or matches but excluded by exclude_types",
			file: "main.py",
			hook: config.Hook{
				TypesOr:      []string{"python", "javascript"},
				ExcludeTypes: []string{"python"},
			},
			expected: false,
		},
		{
			name: "types fails but types_or succeeds",
			file: "main.py",
			hook: config.Hook{
				Types:   []string{"javascript", "python"}, // needs both
				TypesOr: []string{"python"},               // needs any
			},
			expected: false, // types requires ALL to match, python+javascript won't work
		},
		{
			name: "only types_or specified and matches",
			file: "main.py",
			hook: config.Hook{
				TypesOr: []string{"python", "javascript"},
			},
			expected: true,
		},
		{
			name: "only types_or specified and does not match",
			file: "main.go",
			hook: config.Hook{
				TypesOr: []string{"python", "javascript"},
			},
			expected: false,
		},
		{
			name: "only exclude_types specified - file not excluded",
			file: "main.go",
			hook: config.Hook{
				ExcludeTypes: []string{"python", "javascript"},
			},
			expected: true,
		},
		{
			name: "only exclude_types specified - file excluded",
			file: "main.py",
			hook: config.Hook{
				ExcludeTypes: []string{"python", "javascript"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.matchesTypeFilters(tt.file, tt.hook)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatcher_FileMatchesHook_AllFiltersEmpty(t *testing.T) {
	matcher := NewMatcher()

	// Test hook with no filters - should match all files
	hook := config.Hook{
		ID: "test-hook",
		// No Files, ExcludeRegex, Types, TypesOr, or ExcludeTypes
	}

	tests := []string{
		"main.py",
		"test.go",
		"README.md",
		"Dockerfile",
		"some/path/file.txt",
	}

	for _, file := range tests {
		t.Run("no filters matches "+file, func(t *testing.T) {
			result := matcher.FileMatchesHook(file, hook)
			assert.True(t, result, "File with no filters should match all files")
		})
	}
}

func TestMatcher_GetFilesForHook_EmptyInput(t *testing.T) {
	matcher := NewMatcher()

	hook := config.Hook{
		ID:    "test-hook",
		Files: `\.py$`,
	}

	// Test with empty contextFiles
	result := matcher.GetFilesForHook(hook, []string{}, false)
	assert.Empty(t, result)

	result = matcher.GetFilesForHook(hook, []string{}, true)
	assert.Empty(t, result)
}
