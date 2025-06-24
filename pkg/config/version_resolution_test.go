package config

import (
	"testing"
)

func TestResolveEffectiveLanguageVersion(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		hook     Hook
		config   Config
	}{
		{
			name: "Hook with specific language_version takes precedence",
			hook: Hook{
				Language:        "python",
				LanguageVersion: "3.9",
			},
			config: Config{
				DefaultLanguageVersion: map[string]string{
					"python": "3.8",
				},
			},
			expected: "3.9",
		},
		{
			name: "Uses default_language_version when hook has no version",
			hook: Hook{
				Language: "python",
			},
			config: Config{
				DefaultLanguageVersion: map[string]string{
					"python": "3.11",
				},
			},
			expected: "3.11",
		},
		{
			name: "Returns empty when no version specified anywhere",
			hook: Hook{
				Language: "python",
			},
			config: Config{
				DefaultLanguageVersion: map[string]string{
					"node": "18",
				},
			},
			expected: "",
		},
		{
			name: "Empty hook language_version uses default",
			hook: Hook{
				Language:        "python",
				LanguageVersion: "",
			},
			config: Config{
				DefaultLanguageVersion: map[string]string{
					"python": "3.10",
				},
			},
			expected: "3.10",
		},
		{
			name: "Works with nil DefaultLanguageVersion",
			hook: Hook{
				Language: "python",
			},
			config: Config{
				DefaultLanguageVersion: nil,
			},
			expected: "",
		},
		{
			name: "Multiple languages in default_language_version",
			hook: Hook{
				Language: "node",
			},
			config: Config{
				DefaultLanguageVersion: map[string]string{
					"python": "3.9",
					"node":   "18.19.0",
					"ruby":   "3.1",
				},
			},
			expected: "18.19.0",
		},
		{
			name: "Hook version overrides even when default exists",
			hook: Hook{
				Language:        "ruby",
				LanguageVersion: "2.7",
			},
			config: Config{
				DefaultLanguageVersion: map[string]string{
					"ruby": "3.0",
				},
			},
			expected: "2.7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveEffectiveLanguageVersion(tt.hook, tt.config)
			if result != tt.expected {
				t.Errorf("ResolveEffectiveLanguageVersion() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestResolveEffectiveLanguageVersion_EdgeCases(t *testing.T) {
	t.Run("Empty language", func(t *testing.T) {
		hook := Hook{
			Language: "",
		}
		config := Config{
			DefaultLanguageVersion: map[string]string{
				"python": "3.9",
			},
		}
		result := ResolveEffectiveLanguageVersion(hook, config)
		if result != "" {
			t.Errorf("Expected empty result for empty language, got %q", result)
		}
	})

	t.Run("Case sensitivity", func(t *testing.T) {
		hook := Hook{
			Language: "Python",
		}
		config := Config{
			DefaultLanguageVersion: map[string]string{
				"python": "3.9",
			},
		}
		result := ResolveEffectiveLanguageVersion(hook, config)
		// Should not match due to case sensitivity
		if result != "" {
			t.Errorf("Expected empty result for case mismatch, got %q", result)
		}
	})
}
