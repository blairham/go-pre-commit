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
			TestVersions:             []string{"default"},
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
			TestVersions:             []string{"default"},
			NeedsRuntimeInstalled:    true,
			CacheTestEnabled:         true,
			BiDirectionalTestEnabled: true,
			TestTimeout:              10 * time.Minute,
		},
		{
			Name:                     "Go Environment",
			Language:                 "golang",
			TestRepository:           "https://github.com/pre-commit/pre-commit-hooks",
			TestCommit:               "v4.4.0",
			HookID:                   "check-yaml",
			TestVersions:             []string{"default"},
			NeedsRuntimeInstalled:    true,
			CacheTestEnabled:         true,
			BiDirectionalTestEnabled: true,
			TestTimeout:              10 * time.Minute,
		},
		{
			Name:                     "Rust Environment",
			Language:                 "rust",
			TestRepository:           "local",
			TestCommit:               "",
			HookID:                   "rust-check",
			TestVersions:             []string{"default"},
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
			TestVersions:             []string{"default"},
			NeedsRuntimeInstalled:    true,
			CacheTestEnabled:         true,
			BiDirectionalTestEnabled: true,
			TestTimeout:              10 * time.Minute,
		},
		// Mobile & Modern Languages
		{
			Name:                     "Dart Environment",
			Language:                 "dart",
			TestRepository:           "https://github.com/Cretezy/dart-format-pre-commit",
			TestCommit:               "master",
			HookID:                   "dart-format",
			TestVersions:             []string{"default"},
			NeedsRuntimeInstalled:    true,
			CacheTestEnabled:         true,
			BiDirectionalTestEnabled: true,
			TestTimeout:              10 * time.Minute,
		},
		{
			Name:                     "Swift Environment",
			Language:                 "swift",
			TestRepository:           "local",
			TestCommit:               "",
			HookID:                   "swiftformat",
			TestVersions:             []string{"default"},
			NeedsRuntimeInstalled:    true,
			CacheTestEnabled:         true,
			BiDirectionalTestEnabled: true,
			TestTimeout:              2 * time.Minute,
		},
		// Scripting Languages
		{
			Name:                     "Lua Environment",
			Language:                 "lua", // Keep original language for test framework
			TestRepository:           "local",
			TestCommit:               "",
			HookID:                   "lua-syntax-check",
			TestVersions:             []string{"default"},
			NeedsRuntimeInstalled:    true,
			CacheTestEnabled:         true,
			BiDirectionalTestEnabled: true,
			TestTimeout:              10 * time.Minute,
		},
		{
			Name:                     "Perl Environment",
			Language:                 "perl", // Keep original language for test framework
			TestRepository:           "local",
			TestCommit:               "",
			HookID:                   "perl-syntax-check",
			TestVersions:             []string{"default"},
			NeedsRuntimeInstalled:    true,
			CacheTestEnabled:         true,
			BiDirectionalTestEnabled: true,
			TestTimeout:              10 * time.Minute,
		},
		{
			Name:                     "R Environment",
			Language:                 "r", // Keep original language for test framework
			TestRepository:           "local",
			TestCommit:               "",
			HookID:                   "r-syntax-check",
			TestVersions:             []string{"default"}, // Python pre-commit only supports 'default' version
			NeedsRuntimeInstalled:    true,
			CacheTestEnabled:         true,
			BiDirectionalTestEnabled: true,
			TestTimeout:              10 * time.Minute,
		},
		// Functional & Academic Languages
		{
			Name:                     "Haskell Environment",
			Language:                 "haskell",
			TestRepository:           "local", // Use local repository to create Haskell hook
			TestCommit:               "",
			HookID:                   "hindent",
			TestVersions:             []string{"default"}, // Haskell only supports default/system
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
			TestVersions:             []string{"default"},
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
			TestCommit:               "v8.0.453106",
			HookID:                   "dotnet-format",
			TestVersions:             []string{"default"},
			NeedsRuntimeInstalled:    true,
			CacheTestEnabled:         true,
			BiDirectionalTestEnabled: true,
			TestTimeout:              10 * time.Minute,
		},
		{
			Name:                     "Coursier Environment",
			Language:                 "coursier",
			TestRepository:           "https://github.com/coyainsurance/pre-commit-scalafmt",
			TestCommit:               "master",
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
			TestRepository:           "https://github.com/psf/black",
			TestCommit:               "23.12.1",
			HookID:                   "black",
			TestVersions:             []string{"default"},
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
