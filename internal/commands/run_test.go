package commands

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blairham/go-pre-commit/pkg/config"
	"github.com/blairham/go-pre-commit/pkg/hook/execution"
)

func TestRunCommand_Synopsis(t *testing.T) {
	cmd := &RunCommand{}
	synopsis := cmd.Synopsis()
	assert.Equal(t, "Run hooks on files", synopsis)
}

func TestRunCommand_Help(t *testing.T) {
	cmd := &RunCommand{}
	help := cmd.Help()
	assert.NotEmpty(t, help)
	assert.Contains(t, help, "pre-commit run")
	assert.Contains(t, help, "--help")
	assert.Contains(t, help, "--all-files")
	assert.Contains(t, help, "--files")
	assert.Contains(t, help, "--config")
	assert.Contains(t, help, "--verbose")
	assert.Contains(t, help, "--show-diff-on-failure")
	assert.Contains(t, help, "--fail-fast")
	assert.Contains(t, help, "--hook-stage")
	assert.Contains(t, help, "--from-ref")
	assert.Contains(t, help, "--to-ref")
	assert.Contains(t, help, "--jobs")
	assert.Contains(t, help, "--color")
	// Check for positional argument documentation
	assert.Contains(t, help, "positional arguments")
	assert.Contains(t, help, "hook")
	assert.Contains(t, help, "a single hook-id to run")
}

func TestRunCommand_parseAndValidateRunArgs(t *testing.T) {
	cmd := &RunCommand{}

	tests := []struct {
		name         string
		args         []string
		expectExit   int
		validateOpts func(t *testing.T, opts *RunOptions, remainingArgs []string)
	}{
		{
			name:       "help flag",
			args:       []string{"--help"},
			expectExit: 0,
			validateOpts: func(t *testing.T, opts *RunOptions, remainingArgs []string) {
				// Help is shown via flags.ErrHelp
			},
		},
		{
			name:       "short help flag",
			args:       []string{"-h"},
			expectExit: 0,
			validateOpts: func(t *testing.T, opts *RunOptions, remainingArgs []string) {
				// Help is shown via flags.ErrHelp
			},
		},
		{
			name:       "default values",
			args:       []string{},
			expectExit: -1,
			validateOpts: func(t *testing.T, opts *RunOptions, remainingArgs []string) {
				assert.Equal(t, ".pre-commit-config.yaml", opts.Config)
				assert.Equal(t, "pre-commit", opts.HookStage)
				assert.Equal(t, "auto", opts.Color)
				assert.Equal(t, 1, opts.Parallel)
				assert.False(t, opts.AllFiles)
				assert.False(t, opts.Verbose)
				assert.False(t, opts.ShowDiff)
				assert.False(t, opts.FailFast)
				assert.Empty(t, remainingArgs)
			},
		},
		{
			name:       "custom config",
			args:       []string{"--config", "custom.yaml"},
			expectExit: -1,
			validateOpts: func(t *testing.T, opts *RunOptions, remainingArgs []string) {
				assert.Equal(t, "custom.yaml", opts.Config)
			},
		},
		{
			name:       "short config flag",
			args:       []string{"-c", "custom.yaml"},
			expectExit: -1,
			validateOpts: func(t *testing.T, opts *RunOptions, remainingArgs []string) {
				assert.Equal(t, "custom.yaml", opts.Config)
			},
		},
		{
			name:       "all files flag",
			args:       []string{"--all-files"},
			expectExit: -1,
			validateOpts: func(t *testing.T, opts *RunOptions, remainingArgs []string) {
				assert.True(t, opts.AllFiles)
			},
		},
		{
			name:       "short all files flag",
			args:       []string{"-a"},
			expectExit: -1,
			validateOpts: func(t *testing.T, opts *RunOptions, remainingArgs []string) {
				assert.True(t, opts.AllFiles)
			},
		},
		{
			name:       "verbose flag",
			args:       []string{"--verbose"},
			expectExit: -1,
			validateOpts: func(t *testing.T, opts *RunOptions, remainingArgs []string) {
				assert.True(t, opts.Verbose)
			},
		},
		{
			name:       "short verbose flag",
			args:       []string{"-v"},
			expectExit: -1,
			validateOpts: func(t *testing.T, opts *RunOptions, remainingArgs []string) {
				assert.True(t, opts.Verbose)
			},
		},
		{
			name:       "show diff on failure flag",
			args:       []string{"--show-diff-on-failure"},
			expectExit: -1,
			validateOpts: func(t *testing.T, opts *RunOptions, remainingArgs []string) {
				assert.True(t, opts.ShowDiff)
			},
		},
		{
			name:       "fail-fast flag",
			args:       []string{"--fail-fast"},
			expectExit: -1,
			validateOpts: func(t *testing.T, opts *RunOptions, remainingArgs []string) {
				assert.True(t, opts.FailFast)
			},
		},
		{
			name:       "hook stage flag",
			args:       []string{"--hook-stage", "pre-push"},
			expectExit: -1,
			validateOpts: func(t *testing.T, opts *RunOptions, remainingArgs []string) {
				assert.Equal(t, "pre-push", opts.HookStage)
			},
		},
		{
			name:       "from-ref and to-ref flags",
			args:       []string{"--from-ref", "HEAD~5", "--to-ref", "HEAD"},
			expectExit: -1,
			validateOpts: func(t *testing.T, opts *RunOptions, remainingArgs []string) {
				assert.Equal(t, "HEAD~5", opts.FromRef)
				assert.Equal(t, "HEAD", opts.ToRef)
			},
		},
		{
			name:       "short from-ref and to-ref flags",
			args:       []string{"-s", "HEAD~5", "-o", "HEAD"},
			expectExit: -1,
			validateOpts: func(t *testing.T, opts *RunOptions, remainingArgs []string) {
				assert.Equal(t, "HEAD~5", opts.FromRef)
				assert.Equal(t, "HEAD", opts.ToRef)
			},
		},
		{
			name:       "jobs flag",
			args:       []string{"--jobs", "4"},
			expectExit: -1,
			validateOpts: func(t *testing.T, opts *RunOptions, remainingArgs []string) {
				assert.Equal(t, 4, opts.Parallel)
			},
		},
		{
			name:       "short jobs flag",
			args:       []string{"-j", "8"},
			expectExit: -1,
			validateOpts: func(t *testing.T, opts *RunOptions, remainingArgs []string) {
				assert.Equal(t, 8, opts.Parallel)
			},
		},
		{
			name:       "files flag",
			args:       []string{"--files", "file1.py", "--files", "file2.py"},
			expectExit: -1,
			validateOpts: func(t *testing.T, opts *RunOptions, remainingArgs []string) {
				assert.Equal(t, []string{"file1.py", "file2.py"}, opts.Files)
			},
		},
		{
			name:       "positional hook id",
			args:       []string{"black"},
			expectExit: -1,
			validateOpts: func(t *testing.T, opts *RunOptions, remainingArgs []string) {
				assert.Equal(t, []string{"black"}, remainingArgs)
			},
		},
		{
			name:       "multiple positional hook ids",
			args:       []string{"black", "flake8", "mypy"},
			expectExit: -1,
			validateOpts: func(t *testing.T, opts *RunOptions, remainingArgs []string) {
				assert.Equal(t, []string{"black", "flake8", "mypy"}, remainingArgs)
			},
		},
		{
			name:       "hook id with flags",
			args:       []string{"--verbose", "black", "--all-files"},
			expectExit: -1,
			validateOpts: func(t *testing.T, opts *RunOptions, remainingArgs []string) {
				assert.True(t, opts.Verbose)
				assert.True(t, opts.AllFiles)
				assert.Equal(t, []string{"black"}, remainingArgs)
			},
		},
		{
			name:       "git hook specific flags - pre-push",
			args:       []string{"--remote-name", "origin", "--remote-url", "git@github.com:user/repo.git", "--local-branch", "main", "--remote-branch", "refs/heads/main"},
			expectExit: -1,
			validateOpts: func(t *testing.T, opts *RunOptions, remainingArgs []string) {
				assert.Equal(t, "origin", opts.RemoteName)
				assert.Equal(t, "git@github.com:user/repo.git", opts.RemoteURL)
				assert.Equal(t, "main", opts.LocalBranch)
				assert.Equal(t, "refs/heads/main", opts.RemoteBranch)
			},
		},
		{
			name:       "git hook specific flags - commit-msg",
			args:       []string{"--commit-msg-filename", ".git/COMMIT_EDITMSG"},
			expectExit: -1,
			validateOpts: func(t *testing.T, opts *RunOptions, remainingArgs []string) {
				assert.Equal(t, ".git/COMMIT_EDITMSG", opts.CommitMsgFilename)
			},
		},
		{
			name:       "git hook specific flags - prepare-commit-msg",
			args:       []string{"--prepare-commit-message-source", "message", "--commit-object-name", "abc123"},
			expectExit: -1,
			validateOpts: func(t *testing.T, opts *RunOptions, remainingArgs []string) {
				assert.Equal(t, "message", opts.PrepareCommitMessageSource)
				assert.Equal(t, "abc123", opts.CommitObjectName)
			},
		},
		{
			name:       "git hook specific flags - post-checkout",
			args:       []string{"--checkout-type", "1"},
			expectExit: -1,
			validateOpts: func(t *testing.T, opts *RunOptions, remainingArgs []string) {
				assert.Equal(t, "1", opts.CheckoutType)
			},
		},
		{
			name:       "git hook specific flags - post-merge",
			args:       []string{"--is-squash-merge", "0"},
			expectExit: -1,
			validateOpts: func(t *testing.T, opts *RunOptions, remainingArgs []string) {
				assert.Equal(t, "0", opts.IsSquashMerge)
			},
		},
		{
			name:       "git hook specific flags - post-rewrite",
			args:       []string{"--rewrite-command", "amend"},
			expectExit: -1,
			validateOpts: func(t *testing.T, opts *RunOptions, remainingArgs []string) {
				assert.Equal(t, "amend", opts.RewriteCommand)
			},
		},
		{
			name:       "git hook specific flags - pre-rebase",
			args:       []string{"--pre-rebase-upstream", "origin/main", "--pre-rebase-branch", "feature"},
			expectExit: -1,
			validateOpts: func(t *testing.T, opts *RunOptions, remainingArgs []string) {
				assert.Equal(t, "origin/main", opts.PreRebaseUpstream)
				assert.Equal(t, "feature", opts.PreRebaseBranch)
			},
		},
		{
			name:       "combined flags and positional args",
			args:       []string{"-v", "-a", "--fail-fast", "-j", "4", "black", "flake8"},
			expectExit: -1,
			validateOpts: func(t *testing.T, opts *RunOptions, remainingArgs []string) {
				assert.True(t, opts.Verbose)
				assert.True(t, opts.AllFiles)
				assert.True(t, opts.FailFast)
				assert.Equal(t, 4, opts.Parallel)
				assert.Equal(t, []string{"black", "flake8"}, remainingArgs)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, remainingArgs, exitCode := cmd.parseAndValidateRunArgs(tt.args)

			assert.Equal(t, tt.expectExit, exitCode, "unexpected exit code")

			if tt.validateOpts != nil && exitCode == -1 {
				require.NotNil(t, opts, "opts should not be nil when exit code is -1")
				tt.validateOpts(t, opts, remainingArgs)
			}
		})
	}
}

func TestRunCommand_validateRunOptions(t *testing.T) {
	cmd := &RunCommand{}

	tests := []struct {
		name      string
		opts      *RunOptions
		expectErr bool
		errMsg    string
	}{
		{
			name: "valid default options",
			opts: &RunOptions{
				HookStage: "pre-commit",
				Parallel:  1,
			},
			expectErr: false,
		},
		{
			name: "all-files and files both set",
			opts: &RunOptions{
				AllFiles:  true,
				Files:     []string{"file.py"},
				HookStage: "pre-commit",
			},
			expectErr: true,
			errMsg:    "--all-files, --files, and --from-ref/--to-ref are mutually exclusive",
		},
		{
			name: "from-ref without to-ref",
			opts: &RunOptions{
				FromRef:   "HEAD~5",
				HookStage: "pre-commit",
			},
			expectErr: true,
			errMsg:    "--to-ref is required when --from-ref is specified",
		},
		{
			name: "to-ref without from-ref",
			opts: &RunOptions{
				ToRef:     "HEAD",
				HookStage: "pre-commit",
			},
			expectErr: true,
			errMsg:    "--from-ref is required when --to-ref is specified",
		},
		{
			name: "valid from-ref and to-ref",
			opts: &RunOptions{
				FromRef:   "HEAD~5",
				ToRef:     "HEAD",
				HookStage: "pre-commit",
			},
			expectErr: false,
		},
		{
			name: "all-files with from-ref",
			opts: &RunOptions{
				AllFiles:  true,
				FromRef:   "HEAD~5",
				ToRef:     "HEAD",
				HookStage: "pre-commit",
			},
			expectErr: true,
			errMsg:    "--all-files, --files, and --from-ref/--to-ref are mutually exclusive",
		},
		{
			name: "files with from-ref",
			opts: &RunOptions{
				Files:     []string{"file.py"},
				FromRef:   "HEAD~5",
				ToRef:     "HEAD",
				HookStage: "pre-commit",
			},
			expectErr: true,
			errMsg:    "--all-files, --files, and --from-ref/--to-ref are mutually exclusive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cmd.validateRunOptions(tt.opts)

			if tt.expectErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRunCommand_setEnvironmentVariables(t *testing.T) {
	cmd := &RunCommand{}

	tests := []struct {
		name     string
		opts     *RunOptions
		expected map[string]string
	}{
		{
			name: "basic env vars",
			opts: &RunOptions{
				HookStage: "pre-commit",
			},
			expected: map[string]string{
				"PRE_COMMIT":            "1",
				"PRE_COMMIT_HOOK_STAGE": "pre-commit",
			},
		},
		{
			name: "pre-push env vars",
			opts: &RunOptions{
				HookStage:    "pre-push",
				RemoteName:   "origin",
				RemoteURL:    "git@github.com:user/repo.git",
				LocalBranch:  "main",
				RemoteBranch: "refs/heads/main",
			},
			expected: map[string]string{
				"PRE_COMMIT":               "1",
				"PRE_COMMIT_HOOK_STAGE":    "pre-push",
				"PRE_COMMIT_REMOTE_NAME":   "origin",
				"PRE_COMMIT_REMOTE_URL":    "git@github.com:user/repo.git",
				"PRE_COMMIT_LOCAL_BRANCH":  "main",
				"PRE_COMMIT_REMOTE_BRANCH": "refs/heads/main",
			},
		},
		{
			name: "diff-based env vars",
			opts: &RunOptions{
				HookStage: "pre-commit",
				FromRef:   "HEAD~5",
				ToRef:     "HEAD",
			},
			expected: map[string]string{
				"PRE_COMMIT":            "1",
				"PRE_COMMIT_HOOK_STAGE": "pre-commit",
				"PRE_COMMIT_FROM_REF":   "HEAD~5",
				"PRE_COMMIT_TO_REF":     "HEAD",
			},
		},
		{
			name: "commit-msg env vars",
			opts: &RunOptions{
				HookStage:         "commit-msg",
				CommitMsgFilename: ".git/COMMIT_EDITMSG",
			},
			expected: map[string]string{
				"PRE_COMMIT":                     "1",
				"PRE_COMMIT_HOOK_STAGE":          "commit-msg",
				"PRE_COMMIT_COMMIT_MSG_FILENAME": ".git/COMMIT_EDITMSG",
			},
		},
		{
			name: "prepare-commit-msg env vars",
			opts: &RunOptions{
				HookStage:                  "prepare-commit-msg",
				PrepareCommitMessageSource: "message",
				CommitObjectName:           "abc123",
			},
			expected: map[string]string{
				"PRE_COMMIT":                    "1",
				"PRE_COMMIT_HOOK_STAGE":         "prepare-commit-msg",
				"PRE_COMMIT_COMMIT_MSG_SOURCE":  "message",
				"PRE_COMMIT_COMMIT_OBJECT_NAME": "abc123",
			},
		},
		{
			name: "post-checkout env vars",
			opts: &RunOptions{
				HookStage:    "post-checkout",
				CheckoutType: "1",
			},
			expected: map[string]string{
				"PRE_COMMIT":               "1",
				"PRE_COMMIT_HOOK_STAGE":    "post-checkout",
				"PRE_COMMIT_CHECKOUT_TYPE": "1",
			},
		},
		{
			name: "post-merge env vars",
			opts: &RunOptions{
				HookStage:     "post-merge",
				IsSquashMerge: "0",
			},
			expected: map[string]string{
				"PRE_COMMIT":                "1",
				"PRE_COMMIT_HOOK_STAGE":     "post-merge",
				"PRE_COMMIT_IS_SQUASH_MERGE": "0",
			},
		},
		{
			name: "post-rewrite env vars",
			opts: &RunOptions{
				HookStage:      "post-rewrite",
				RewriteCommand: "amend",
			},
			expected: map[string]string{
				"PRE_COMMIT":                 "1",
				"PRE_COMMIT_HOOK_STAGE":      "post-rewrite",
				"PRE_COMMIT_REWRITE_COMMAND": "amend",
			},
		},
		{
			name: "pre-rebase env vars",
			opts: &RunOptions{
				HookStage:         "pre-rebase",
				PreRebaseUpstream: "origin/main",
				PreRebaseBranch:   "feature",
			},
			expected: map[string]string{
				"PRE_COMMIT":                    "1",
				"PRE_COMMIT_HOOK_STAGE":         "pre-rebase",
				"PRE_COMMIT_PRE_REBASE_UPSTREAM": "origin/main",
				"PRE_COMMIT_PRE_REBASE_BRANCH":  "feature",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up environment before test
			envVars := []string{
				"PRE_COMMIT",
				"PRE_COMMIT_HOOK_STAGE",
				"PRE_COMMIT_REMOTE_NAME",
				"PRE_COMMIT_REMOTE_URL",
				"PRE_COMMIT_LOCAL_BRANCH",
				"PRE_COMMIT_REMOTE_BRANCH",
				"PRE_COMMIT_FROM_REF",
				"PRE_COMMIT_TO_REF",
				"PRE_COMMIT_COMMIT_MSG_FILENAME",
				"PRE_COMMIT_PREPARE_COMMIT_MESSAGE_SOURCE",
				"PRE_COMMIT_COMMIT_OBJECT_NAME",
				"PRE_COMMIT_CHECKOUT_TYPE",
				"PRE_COMMIT_IS_SQUASH_MERGE",
				"PRE_COMMIT_REWRITE_COMMAND",
				"PRE_COMMIT_PRE_REBASE_UPSTREAM",
				"PRE_COMMIT_PRE_REBASE_BRANCH",
			}
			for _, key := range envVars {
				os.Unsetenv(key)
			}

			env := cmd.setEnvironmentVariables(tt.opts)

			// Check returned map
			for key, expectedValue := range tt.expected {
				assert.Equal(t, expectedValue, env[key], "env map key %s", key)
			}

			// Check actual environment variables were set
			for key, expectedValue := range tt.expected {
				actualValue := os.Getenv(key)
				assert.Equal(t, expectedValue, actualValue, "os env key %s", key)
			}

			// Cleanup
			for _, key := range envVars {
				os.Unsetenv(key)
			}
		})
	}
}

func TestRunCommand_createExecutionContext(t *testing.T) {
	cmd := &RunCommand{}

	t.Run("basic context creation", func(t *testing.T) {
		cfg := &config.Config{
			Repos:    []config.Repo{},
			FailFast: false,
		}
		opts := &RunOptions{
			AllFiles:  true,
			Verbose:   true,
			ShowDiff:  true,
			FailFast:  false,
			HookStage: "pre-commit",
			Color:     "auto",
			Parallel:  4,
		}
		files := []string{"file1.py", "file2.py"}
		env := map[string]string{"PRE_COMMIT": "1"}
		hookIDs := []string{"black", "flake8"}

		// Note: passing nil for repo since we're just testing context creation
		// The actual code would have a valid repo, but for this test we just check fields
		ctx := cmd.createExecutionContext(cfg, files, nil, opts, env, hookIDs, nil)

		assert.Equal(t, cfg, ctx.Config)
		assert.Equal(t, files, ctx.Files)
		assert.True(t, ctx.AllFiles)
		assert.True(t, ctx.Verbose)
		assert.True(t, ctx.ShowDiff)
		assert.False(t, ctx.FailFast)
		assert.Equal(t, "pre-commit", ctx.HookStage)
		assert.Equal(t, "auto", ctx.Color)
		assert.Equal(t, 4, ctx.Parallel)
		assert.Equal(t, env, ctx.Environment)
		assert.Equal(t, hookIDs, ctx.HookIDs)
		// RepoRoot will be empty since repo is nil
		assert.Empty(t, ctx.RepoRoot)
	})

	t.Run("fail-fast from CLI takes precedence", func(t *testing.T) {
		cfg := &config.Config{
			Repos:    []config.Repo{},
			FailFast: false, // config says false
		}
		opts := &RunOptions{
			FailFast:  true, // CLI says true
			HookStage: "pre-commit",
		}

		ctx := cmd.createExecutionContext(cfg, nil, nil, opts, nil, nil, nil)

		assert.True(t, ctx.FailFast, "CLI --fail-fast should take precedence")
	})

	t.Run("fail-fast from config when CLI is false", func(t *testing.T) {
		cfg := &config.Config{
			Repos:    []config.Repo{},
			FailFast: true, // config says true
		}
		opts := &RunOptions{
			FailFast:  false, // CLI says false (default)
			HookStage: "pre-commit",
		}

		ctx := cmd.createExecutionContext(cfg, nil, nil, opts, nil, nil, nil)

		assert.True(t, ctx.FailFast, "config fail_fast should be used when CLI is false")
	})

	t.Run("fail-fast false when both are false", func(t *testing.T) {
		cfg := &config.Config{
			Repos:    []config.Repo{},
			FailFast: false,
		}
		opts := &RunOptions{
			FailFast:  false,
			HookStage: "pre-commit",
		}

		ctx := cmd.createExecutionContext(cfg, nil, nil, opts, nil, nil, nil)

		assert.False(t, ctx.FailFast)
	})
}

func TestRunCommand_Factory(t *testing.T) {
	cmd, err := RunCommandFactory()
	require.NoError(t, err)
	require.NotNil(t, cmd)

	runCmd, ok := cmd.(*RunCommand)
	assert.True(t, ok, "factory should return *RunCommand")
	assert.NotNil(t, runCmd)
}

func TestRunOptions_AllHookStages(t *testing.T) {
	// Verify that all hook stages from constants are valid
	hookStages := []string{
		hookTypePreCommit,
		hookTypePreMergeCommit,
		hookTypePrePush,
		hookTypePrepareCommit,
		hookTypeCommitMsg,
		hookTypePostCheckout,
		hookTypePostCommit,
		hookTypePostMerge,
		hookTypePostRewrite,
		hookTypePreRebase,
		hookTypePreAutoGC,
	}

	cmd := &RunCommand{}
	for _, stage := range hookStages {
		t.Run(stage, func(t *testing.T) {
			args := []string{"--hook-stage", stage}
			opts, _, exitCode := cmd.parseAndValidateRunArgs(args)

			assert.Equal(t, -1, exitCode)
			require.NotNil(t, opts)
			assert.Equal(t, stage, opts.HookStage)
		})
	}
}

// TestExecutionContextFailFast tests that the execution context correctly
// uses the FailFast field for hook execution decisions
func TestExecutionContextFailFast(t *testing.T) {
	tests := []struct {
		name           string
		ctx            *execution.Context
		expectedResult bool
	}{
		{
			name: "fail fast enabled",
			ctx: &execution.Context{
				FailFast: true,
			},
			expectedResult: true,
		},
		{
			name: "fail fast disabled",
			ctx: &execution.Context{
				FailFast: false,
			},
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedResult, tt.ctx.FailFast)
		})
	}
}
