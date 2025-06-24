// Package integration provides modular integration tests for the go-pre-commit tool.
// This package replaces the monolithic integration_test.go with a well-organized,
// maintainable structure that supports testing language compatibility across
// 19+ programming languages.
package integration

import (
	"time"
)

// GetAllLanguageTests returns all language compatibility tests
//
//nolint:funlen // Large function with language definitions - acceptable for configuration
func (s *Suite) GetAllLanguageTests() []LanguageCompatibilityTest {
	return []LanguageCompatibilityTest{
		// Core Programming Languages
		{
			Name:                     "Python Environment",
			Language:                 "python",
			TestRepository:           "https://github.com/pre-commit/pre-commit-hooks",
			TestCommit:               "v4.4.0",
			HookID:                   "check-yaml",
			TestVersions:             []string{"default", "3.9", "3.10", "3.11"},
			NeedsRuntimeInstalled:    true,
			CacheTestEnabled:         true,
			BiDirectionalTestEnabled: true,
			TestTimeout:              10 * time.Minute,
		},
		{
			Name:                     "Node.js Environment",
			Language:                 "node",
			TestRepository:           "https://github.com/pre-commit/pre-commit-hooks",
			TestCommit:               "v4.4.0",
			HookID:                   "check-json",
			TestVersions:             []string{"default", "16", "18", "20"},
			NeedsRuntimeInstalled:    true,
			CacheTestEnabled:         true,
			BiDirectionalTestEnabled: true,
			TestTimeout:              10 * time.Minute,
		},
		{
			Name:                     "Go Environment",
			Language:                 "golang",
			TestRepository:           "local", // Use local repository to create a Go hook
			TestCommit:               "",
			HookID:                   "go-test-simple",
			TestVersions:             []string{"default", "1.22.0", "1.23.0", "1.24.0"},
			NeedsRuntimeInstalled:    true,
			CacheTestEnabled:         true,
			BiDirectionalTestEnabled: true,
			TestTimeout:              10 * time.Minute,
		},
		{
			Name:                     "Rust Environment",
			Language:                 "rust",
			TestRepository:           "https://github.com/doublify/pre-commit-rust",
			TestCommit:               "v1.0",
			HookID:                   "fmt",
			TestVersions:             []string{"default", "1.70", "1.71", "1.72"},
			NeedsRuntimeInstalled:    true,
			CacheTestEnabled:         true,
			BiDirectionalTestEnabled: true,
			TestTimeout:              15 * time.Minute,
		},
		{
			Name:                     "Ruby Environment",
			Language:                 "ruby",
			TestRepository:           "https://github.com/mattlqx/pre-commit-ruby",
			TestCommit:               "v1.3.5",
			HookID:                   "rubocop",
			TestVersions:             []string{"default", "2.7", "3.0", "3.1"},
			NeedsRuntimeInstalled:    true,
			CacheTestEnabled:         true,
			BiDirectionalTestEnabled: true,
			TestTimeout:              10 * time.Minute,
		},
		// Mobile & Modern Languages
		{
			Name:                     "Dart Environment",
			Language:                 "dart",
			TestRepository:           "https://github.com/nakamura-to/pre-commit-dart",
			TestCommit:               "v1.0.0",
			HookID:                   "dart-format",
			TestVersions:             []string{"default", "2.19", "3.0"},
			NeedsRuntimeInstalled:    true,
			CacheTestEnabled:         true,
			BiDirectionalTestEnabled: true,
			TestTimeout:              10 * time.Minute,
		},
		{
			Name:                     "Swift Environment",
			Language:                 "swift",
			TestRepository:           "https://github.com/nicklockwood/SwiftFormat",
			TestCommit:               "0.51.12",
			HookID:                   "swift-format",
			TestVersions:             []string{"default", "5.7", "5.8"},
			NeedsRuntimeInstalled:    true,
			CacheTestEnabled:         true,
			BiDirectionalTestEnabled: true,
			TestTimeout:              10 * time.Minute,
		},
		// Scripting Languages
		{
			Name:                     "Lua Environment",
			Language:                 "lua",
			TestRepository:           "https://github.com/Koihik/LuaFormatter",
			TestCommit:               "1.3.6",
			HookID:                   "lua-format",
			TestVersions:             []string{"default", "5.3", "5.4"},
			NeedsRuntimeInstalled:    true,
			CacheTestEnabled:         true,
			BiDirectionalTestEnabled: true,
			TestTimeout:              10 * time.Minute,
		},
		{
			Name:                     "Perl Environment",
			Language:                 "perl",
			TestRepository:           "https://github.com/pre-commit/mirrors-perl-critic",
			TestCommit:               "v1.140",
			HookID:                   "perl-critic",
			TestVersions:             []string{"default", "5.32", "5.34"},
			NeedsRuntimeInstalled:    true,
			CacheTestEnabled:         true,
			BiDirectionalTestEnabled: true,
			TestTimeout:              10 * time.Minute,
		},
		{
			Name:                     "R Environment",
			Language:                 "r",
			TestRepository:           "https://github.com/lorenzwalthert/precommit",
			TestCommit:               "v0.3.2",
			HookID:                   "style-files",
			TestVersions:             []string{"default"}, // Python pre-commit only supports 'default' version
			NeedsRuntimeInstalled:    true,
			CacheTestEnabled:         true,
			BiDirectionalTestEnabled: true,
			TestTimeout:              15 * time.Minute,
		},
		// Functional & Academic Languages
		{
			Name:           "Haskell Environment",
			Language:       "haskell",
			TestRepository: "https://github.com/mihaimaruseac/hindent",
			TestCommit:     "v5.3.4",
			HookID:         "hindent",
			TestVersions: []string{
				"default",
				"system",
			}, // Haskell only supports default/system
			NeedsRuntimeInstalled:    true,
			CacheTestEnabled:         true,
			BiDirectionalTestEnabled: true,
			TestTimeout:              20 * time.Minute,
		},
		{
			Name:                     "Julia Environment",
			Language:                 "julia",
			TestRepository:           "local", // Use local repository to create Julia hook
			TestCommit:               "",
			HookID:                   "julia-formatter",
			TestVersions:             []string{"default", "1.8", "1.9", "1.10"},
			NeedsRuntimeInstalled:    true,
			CacheTestEnabled:         true,
			BiDirectionalTestEnabled: true,
			TestTimeout:              15 * time.Minute,
		},
		// Enterprise & JVM Languages
		{
			Name:                     ".NET Environment",
			Language:                 "dotnet",
			TestRepository:           "https://github.com/dotnet/format",
			TestCommit:               "v5.1.250801",
			HookID:                   "dotnet-format",
			TestVersions:             []string{"default", "6.0", "7.0", "8.0"},
			NeedsRuntimeInstalled:    true,
			CacheTestEnabled:         true,
			BiDirectionalTestEnabled: true,
			TestTimeout:              10 * time.Minute,
		},
		{
			Name:                     "Coursier Environment",
			Language:                 "coursier",
			TestRepository:           "https://github.com/coursier/coursier",
			TestCommit:               "v2.1.6",
			HookID:                   "scalafmt",
			TestVersions:             []string{"default"}, // Python pre-commit only supports 'default' version
			NeedsRuntimeInstalled:    true,
			CacheTestEnabled:         true,
			BiDirectionalTestEnabled: true,
			TestTimeout:              15 * time.Minute,
		},
		// Container & Environment Languages
		{
			Name:                     "Docker Environment",
			Language:                 "docker",
			TestRepository:           "https://github.com/hadolint/hadolint",
			TestCommit:               "v2.12.0",
			HookID:                   "hadolint",
			TestVersions:             []string{"default"},
			NeedsRuntimeInstalled:    true,
			CacheTestEnabled:         true,
			BiDirectionalTestEnabled: true,
			TestTimeout:              10 * time.Minute,
		},
		{
			Name:                     "Docker Image Environment",
			Language:                 "docker_image",
			TestRepository:           "https://github.com/hadolint/hadolint",
			TestCommit:               "v2.12.0",
			HookID:                   "hadolint",
			TestVersions:             []string{"default"},
			NeedsRuntimeInstalled:    true,
			CacheTestEnabled:         true,
			BiDirectionalTestEnabled: true,
			TestTimeout:              10 * time.Minute,
		},
		{
			Name:                     "Conda Environment",
			Language:                 "conda",
			TestRepository:           "local", // Use local repository to create conda hook
			TestCommit:               "",
			HookID:                   "conda-black",
			TestVersions:             []string{"default", "3.8", "3.9", "3.10", "3.11"},
			NeedsRuntimeInstalled:    true,
			CacheTestEnabled:         true,
			BiDirectionalTestEnabled: true,
			TestTimeout:              15 * time.Minute,
		},
		// System & Utility Languages
		{
			Name:                     "System Environment",
			Language:                 "system",
			TestRepository:           "local", // Use local repository to create system hook
			TestCommit:               "",
			HookID:                   "simple-system-command",
			TestVersions:             []string{"default"},
			NeedsRuntimeInstalled:    false,
			CacheTestEnabled:         true,
			BiDirectionalTestEnabled: true,
			TestTimeout:              5 * time.Minute,
		},
		{
			Name:                     "Script Environment",
			Language:                 ScriptLanguage,
			TestRepository:           "local", // Use local repository to create a proper script hook
			TestCommit:               "",
			HookID:                   "simple-shell-script",
			TestVersions:             []string{"default"},
			NeedsRuntimeInstalled:    false,
			CacheTestEnabled:         true,
			BiDirectionalTestEnabled: true,
			TestTimeout:              5 * time.Minute,
		},
		{
			Name:                     "Fail Environment",
			Language:                 "fail",
			TestRepository:           "local", // Use local repository to test fail hooks
			TestCommit:               "",
			HookID:                   "no-commit-to-branch",
			TestVersions:             []string{"default"},
			NeedsRuntimeInstalled:    false,
			CacheTestEnabled:         true,
			BiDirectionalTestEnabled: true,
			TestTimeout:              5 * time.Minute,
		},
		{
			Name:                     "PyGrep Environment",
			Language:                 "pygrep",
			TestRepository:           "local", // Use local repository to test pygrep hooks
			TestCommit:               "",
			HookID:                   "python-check-blanket-noqa",
			TestVersions:             []string{"default"},
			NeedsRuntimeInstalled:    false,
			CacheTestEnabled:         true,
			BiDirectionalTestEnabled: true,
			TestTimeout:              5 * time.Minute,
		},
	}
}
