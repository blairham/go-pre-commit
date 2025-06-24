package commands

// Git hook type constants
const (
	hookTypePreCommit      = "pre-commit"
	hookTypePreMergeCommit = "pre-merge-commit"
	hookTypePrePush        = "pre-push"
	hookTypePrepareCommit  = "prepare-commit-msg"
	hookTypeCommitMsg      = "commit-msg"
	hookTypePostCheckout   = "post-checkout"
	hookTypePostCommit     = "post-commit"
	hookTypePostMerge      = "post-merge"
	hookTypePostRewrite    = "post-rewrite"
	hookTypePreRebase      = "pre-rebase"
	hookTypePreAutoGC      = "pre-auto-gc"
)

// Common constants used across command implementations
const (
	// Command usage patterns
	OptionsUsage = "[OPTIONS]"

	// Repository types
	LocalRepo = "local"
	MetaRepo  = "meta"

	// Configuration file names
	ConfigFileName = ".pre-commit-config.yaml"

	// Test configuration templates
	ValidRepoConfigWithFiles = `repos:
- repo: local
  hooks:
  - id: test-hook
    name: Test Hook
    entry: echo "test"
    language: system
    files: \.py$
`
)
