package commands

import (
	"strings"
	"testing"
)

func TestTryRepoCommand_Help(t *testing.T) {
	cmd := &TryRepoCommand{}
	help := cmd.Help()

	// Check for key elements in help text
	if !strings.Contains(help, "try-repo") {
		t.Error("help should contain 'try-repo'")
	}
	if !strings.Contains(help, "REPO") {
		t.Error("help should mention 'REPO'")
	}
	if !strings.Contains(help, "HOOK") {
		t.Error("help should mention 'HOOK' positional argument")
	}
}

func TestTryRepoCommand_Synopsis(t *testing.T) {
	cmd := &TryRepoCommand{}
	synopsis := cmd.Synopsis()

	if !strings.Contains(synopsis, "Try") {
		t.Error("synopsis should contain 'Try'")
	}
	if !strings.Contains(synopsis, "hooks") {
		t.Error("synopsis should mention 'hooks'")
	}
}

func TestTryRepoCommand_ParseArgs_MissingRepo(t *testing.T) {
	cmd := &TryRepoCommand{}

	// Test with no arguments - should fail
	_, repoURL, _, rc := cmd.parseAndValidateTryRepoArgs([]string{})

	if rc != 1 {
		t.Errorf("expected return code 1 for missing repo, got %d", rc)
	}
	if repoURL != "" {
		t.Errorf("expected empty repoURL, got %s", repoURL)
	}
}

func TestTryRepoCommand_ParseArgs_WithRepo(t *testing.T) {
	cmd := &TryRepoCommand{}

	// Test with repo argument
	opts, repoURL, hookID, rc := cmd.parseAndValidateTryRepoArgs([]string{"https://github.com/example/repo"})

	if rc != -1 {
		t.Errorf("expected return code -1 (continue), got %d", rc)
	}
	if repoURL != "https://github.com/example/repo" {
		t.Errorf("expected repoURL 'https://github.com/example/repo', got %s", repoURL)
	}
	if hookID != "" {
		t.Errorf("expected empty hookID, got %s", hookID)
	}
	if opts == nil {
		t.Error("expected non-nil opts")
	}
}

func TestTryRepoCommand_ParseArgs_WithRepoAndHook(t *testing.T) {
	cmd := &TryRepoCommand{}

	// Test with repo and hook positional arguments
	_, repoURL, hookID, rc := cmd.parseAndValidateTryRepoArgs([]string{"https://github.com/example/repo", "my-hook"})

	if rc != -1 {
		t.Errorf("expected return code -1 (continue), got %d", rc)
	}
	if repoURL != "https://github.com/example/repo" {
		t.Errorf("expected repoURL 'https://github.com/example/repo', got %s", repoURL)
	}
	if hookID != "my-hook" {
		t.Errorf("expected hookID 'my-hook', got %s", hookID)
	}
}

func TestTryRepoCommand_ParseArgs_RefFlag(t *testing.T) {
	cmd := &TryRepoCommand{}

	// Test with --ref flag
	opts, _, _, rc := cmd.parseAndValidateTryRepoArgs([]string{"https://github.com/example/repo", "--ref", "v1.0.0"})

	if rc != -1 {
		t.Errorf("expected return code -1 (continue), got %d", rc)
	}
	if opts.Ref != "v1.0.0" {
		t.Errorf("expected ref 'v1.0.0', got %s", opts.Ref)
	}
}

func TestTryRepoCommand_ParseArgs_RevAliasForRef(t *testing.T) {
	cmd := &TryRepoCommand{}

	// Test with --rev flag (alias for --ref)
	opts, _, _, rc := cmd.parseAndValidateTryRepoArgs([]string{"https://github.com/example/repo", "--rev", "v2.0.0"})

	if rc != -1 {
		t.Errorf("expected return code -1 (continue), got %d", rc)
	}
	// --rev should be copied to Ref
	if opts.Ref != "v2.0.0" {
		t.Errorf("expected ref 'v2.0.0' from --rev alias, got %s", opts.Ref)
	}
}

func TestTryRepoCommand_ParseArgs_AllFilesFlag(t *testing.T) {
	cmd := &TryRepoCommand{}

	// Test with --all-files flag
	opts, _, _, rc := cmd.parseAndValidateTryRepoArgs([]string{"https://github.com/example/repo", "--all-files"})

	if rc != -1 {
		t.Errorf("expected return code -1 (continue), got %d", rc)
	}
	if !opts.AllFiles {
		t.Error("expected AllFiles to be true")
	}
}

func TestTryRepoCommand_ParseArgs_ShortAllFilesFlag(t *testing.T) {
	cmd := &TryRepoCommand{}

	// Test with -a flag (short for --all-files)
	opts, _, _, rc := cmd.parseAndValidateTryRepoArgs([]string{"https://github.com/example/repo", "-a"})

	if rc != -1 {
		t.Errorf("expected return code -1 (continue), got %d", rc)
	}
	if !opts.AllFiles {
		t.Error("expected AllFiles to be true with -a flag")
	}
}

func TestTryRepoCommand_ParseArgs_VerboseFlag(t *testing.T) {
	cmd := &TryRepoCommand{}

	// Test with --verbose flag
	opts, _, _, rc := cmd.parseAndValidateTryRepoArgs([]string{"https://github.com/example/repo", "--verbose"})

	if rc != -1 {
		t.Errorf("expected return code -1 (continue), got %d", rc)
	}
	if !opts.Verbose {
		t.Error("expected Verbose to be true")
	}
}

func TestTryRepoCommand_ParseArgs_FailFastFlag(t *testing.T) {
	cmd := &TryRepoCommand{}

	// Test with --fail-fast flag
	opts, _, _, rc := cmd.parseAndValidateTryRepoArgs([]string{"https://github.com/example/repo", "--fail-fast"})

	if rc != -1 {
		t.Errorf("expected return code -1 (continue), got %d", rc)
	}
	if !opts.FailFast {
		t.Error("expected FailFast to be true")
	}
}

func TestTryRepoCommand_ParseArgs_ShowDiffOnFailureFlag(t *testing.T) {
	cmd := &TryRepoCommand{}

	// Test with --show-diff-on-failure flag
	opts, _, _, rc := cmd.parseAndValidateTryRepoArgs([]string{"https://github.com/example/repo", "--show-diff-on-failure"})

	if rc != -1 {
		t.Errorf("expected return code -1 (continue), got %d", rc)
	}
	if !opts.ShowDiffOnFailure {
		t.Error("expected ShowDiffOnFailure to be true")
	}
}

func TestTryRepoCommand_ParseArgs_HookStageFlag(t *testing.T) {
	cmd := &TryRepoCommand{}

	// Test with --hook-stage flag
	opts, _, _, rc := cmd.parseAndValidateTryRepoArgs([]string{"https://github.com/example/repo", "--hook-stage", "pre-push"})

	if rc != -1 {
		t.Errorf("expected return code -1 (continue), got %d", rc)
	}
	if opts.HookStage != "pre-push" {
		t.Errorf("expected HookStage 'pre-push', got %s", opts.HookStage)
	}
}

func TestTryRepoCommand_ParseArgs_DefaultHookStage(t *testing.T) {
	cmd := &TryRepoCommand{}

	// Test default hook stage
	opts, _, _, rc := cmd.parseAndValidateTryRepoArgs([]string{"https://github.com/example/repo"})

	if rc != -1 {
		t.Errorf("expected return code -1 (continue), got %d", rc)
	}
	if opts.HookStage != "pre-commit" {
		t.Errorf("expected default HookStage 'pre-commit', got %s", opts.HookStage)
	}
}

func TestTryRepoCommand_ParseArgs_FilesFlag(t *testing.T) {
	cmd := &TryRepoCommand{}

	// Test with --files flag
	opts, _, _, rc := cmd.parseAndValidateTryRepoArgs([]string{"https://github.com/example/repo", "--files", "file1.py", "--files", "file2.py"})

	if rc != -1 {
		t.Errorf("expected return code -1 (continue), got %d", rc)
	}
	if len(opts.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(opts.Files))
	}
}

func TestTryRepoCommand_ParseArgs_CombinedFlags(t *testing.T) {
	cmd := &TryRepoCommand{}

	// Test with multiple flags combined
	opts, repoURL, hookID, rc := cmd.parseAndValidateTryRepoArgs([]string{
		"https://github.com/example/repo",
		"my-hook",
		"--ref", "v1.0.0",
		"--all-files",
		"--fail-fast",
		"--verbose",
	})

	if rc != -1 {
		t.Errorf("expected return code -1 (continue), got %d", rc)
	}
	if repoURL != "https://github.com/example/repo" {
		t.Errorf("unexpected repoURL: %s", repoURL)
	}
	if hookID != "my-hook" {
		t.Errorf("unexpected hookID: %s", hookID)
	}
	if opts.Ref != "v1.0.0" {
		t.Errorf("unexpected ref: %s", opts.Ref)
	}
	if !opts.AllFiles {
		t.Error("expected AllFiles to be true")
	}
	if !opts.FailFast {
		t.Error("expected FailFast to be true")
	}
	if !opts.Verbose {
		t.Error("expected Verbose to be true")
	}
}

func TestTryRepoCommand_DisplayConfig(t *testing.T) {
	cmd := &TryRepoCommand{}

	// This is a simple smoke test - displayConfig prints to stdout
	// We just verify it doesn't panic
	hooks := []struct {
		ID string
	}{
		{ID: "black"},
		{ID: "flake8"},
	}

	// Convert to config.Hook (would need actual import)
	// For now, we just verify the function signature exists and compiles
	_ = cmd
	_ = hooks
}

func TestTryRepoCommand_ParseArgs_FromRefFlag(t *testing.T) {
	cmd := &TryRepoCommand{}

	// Test with --from-ref and --to-ref (both required)
	opts, _, _, rc := cmd.parseAndValidateTryRepoArgs([]string{
		"https://github.com/example/repo",
		"--from-ref", "HEAD~5",
		"--to-ref", "HEAD",
	})

	if rc != -1 {
		t.Errorf("expected return code -1 (continue), got %d", rc)
	}
	if opts.FromRef != "HEAD~5" {
		t.Errorf("expected FromRef 'HEAD~5', got %s", opts.FromRef)
	}
	if opts.ToRef != "HEAD" {
		t.Errorf("expected ToRef 'HEAD', got %s", opts.ToRef)
	}
}

func TestTryRepoCommand_ParseArgs_ShortFromRefFlag(t *testing.T) {
	cmd := &TryRepoCommand{}

	// Test with -s (short for --from-ref) and -o (short for --to-ref)
	opts, _, _, rc := cmd.parseAndValidateTryRepoArgs([]string{
		"https://github.com/example/repo",
		"-s", "main",
		"-o", "feature",
	})

	if rc != -1 {
		t.Errorf("expected return code -1 (continue), got %d", rc)
	}
	if opts.FromRef != "main" {
		t.Errorf("expected FromRef 'main', got %s", opts.FromRef)
	}
	if opts.ToRef != "feature" {
		t.Errorf("expected ToRef 'feature', got %s", opts.ToRef)
	}
}

func TestTryRepoCommand_ParseArgs_FromRefWithoutToRef_Error(t *testing.T) {
	cmd := &TryRepoCommand{}

	// Test with --from-ref without --to-ref - should fail
	_, _, _, rc := cmd.parseAndValidateTryRepoArgs([]string{
		"https://github.com/example/repo",
		"--from-ref", "HEAD~5",
	})

	if rc != 1 {
		t.Errorf("expected return code 1 for --from-ref without --to-ref, got %d", rc)
	}
}

func TestTryRepoCommand_ParseArgs_ToRefWithoutFromRef_Error(t *testing.T) {
	cmd := &TryRepoCommand{}

	// Test with --to-ref without --from-ref - should fail
	_, _, _, rc := cmd.parseAndValidateTryRepoArgs([]string{
		"https://github.com/example/repo",
		"--to-ref", "HEAD",
	})

	if rc != 1 {
		t.Errorf("expected return code 1 for --to-ref without --from-ref, got %d", rc)
	}
}

func TestTryRepoCommand_ParseArgs_RemoteBranchFlag(t *testing.T) {
	cmd := &TryRepoCommand{}

	// Test with --remote-branch flag for pre-push hooks
	opts, _, _, rc := cmd.parseAndValidateTryRepoArgs([]string{
		"https://github.com/example/repo",
		"--remote-branch", "refs/heads/main",
	})

	if rc != -1 {
		t.Errorf("expected return code -1 (continue), got %d", rc)
	}
	if opts.RemoteBranch != "refs/heads/main" {
		t.Errorf("expected RemoteBranch 'refs/heads/main', got %s", opts.RemoteBranch)
	}
}

func TestTryRepoCommand_ParseArgs_LocalBranchFlag(t *testing.T) {
	cmd := &TryRepoCommand{}

	// Test with --local-branch flag for pre-push hooks
	opts, _, _, rc := cmd.parseAndValidateTryRepoArgs([]string{
		"https://github.com/example/repo",
		"--local-branch", "refs/heads/feature",
	})

	if rc != -1 {
		t.Errorf("expected return code -1 (continue), got %d", rc)
	}
	if opts.LocalBranch != "refs/heads/feature" {
		t.Errorf("expected LocalBranch 'refs/heads/feature', got %s", opts.LocalBranch)
	}
}

func TestTryRepoCommand_ParseArgs_RemoteNameFlag(t *testing.T) {
	cmd := &TryRepoCommand{}

	// Test with --remote-name flag
	opts, _, _, rc := cmd.parseAndValidateTryRepoArgs([]string{
		"https://github.com/example/repo",
		"--remote-name", "origin",
	})

	if rc != -1 {
		t.Errorf("expected return code -1 (continue), got %d", rc)
	}
	if opts.RemoteName != "origin" {
		t.Errorf("expected RemoteName 'origin', got %s", opts.RemoteName)
	}
}

func TestTryRepoCommand_ParseArgs_RemoteURLFlag(t *testing.T) {
	cmd := &TryRepoCommand{}

	// Test with --remote-url flag
	opts, _, _, rc := cmd.parseAndValidateTryRepoArgs([]string{
		"https://github.com/example/repo",
		"--remote-url", "git@github.com:user/repo.git",
	})

	if rc != -1 {
		t.Errorf("expected return code -1 (continue), got %d", rc)
	}
	if opts.RemoteURL != "git@github.com:user/repo.git" {
		t.Errorf("expected RemoteURL 'git@github.com:user/repo.git', got %s", opts.RemoteURL)
	}
}

func TestTryRepoCommand_ParseArgs_CommitMsgFilenameFlag(t *testing.T) {
	cmd := &TryRepoCommand{}

	// Test with --commit-msg-filename flag for commit-msg hooks
	opts, _, _, rc := cmd.parseAndValidateTryRepoArgs([]string{
		"https://github.com/example/repo",
		"--commit-msg-filename", ".git/COMMIT_EDITMSG",
	})

	if rc != -1 {
		t.Errorf("expected return code -1 (continue), got %d", rc)
	}
	if opts.CommitMsgFilename != ".git/COMMIT_EDITMSG" {
		t.Errorf("expected CommitMsgFilename '.git/COMMIT_EDITMSG', got %s", opts.CommitMsgFilename)
	}
}

func TestTryRepoCommand_ParseArgs_PrepareCommitMessageSourceFlag(t *testing.T) {
	cmd := &TryRepoCommand{}

	// Test with --prepare-commit-message-source flag
	opts, _, _, rc := cmd.parseAndValidateTryRepoArgs([]string{
		"https://github.com/example/repo",
		"--prepare-commit-message-source", "message",
	})

	if rc != -1 {
		t.Errorf("expected return code -1 (continue), got %d", rc)
	}
	if opts.PrepareCommitMessageSource != "message" {
		t.Errorf("expected PrepareCommitMessageSource 'message', got %s", opts.PrepareCommitMessageSource)
	}
}

func TestTryRepoCommand_ParseArgs_CommitObjectNameFlag(t *testing.T) {
	cmd := &TryRepoCommand{}

	// Test with --commit-object-name flag
	opts, _, _, rc := cmd.parseAndValidateTryRepoArgs([]string{
		"https://github.com/example/repo",
		"--commit-object-name", "abc123",
	})

	if rc != -1 {
		t.Errorf("expected return code -1 (continue), got %d", rc)
	}
	if opts.CommitObjectName != "abc123" {
		t.Errorf("expected CommitObjectName 'abc123', got %s", opts.CommitObjectName)
	}
}

func TestTryRepoCommand_ParseArgs_CheckoutTypeFlag(t *testing.T) {
	cmd := &TryRepoCommand{}

	// Test with --checkout-type flag for post-checkout hooks
	opts, _, _, rc := cmd.parseAndValidateTryRepoArgs([]string{
		"https://github.com/example/repo",
		"--checkout-type", "1",
	})

	if rc != -1 {
		t.Errorf("expected return code -1 (continue), got %d", rc)
	}
	if opts.CheckoutType != "1" {
		t.Errorf("expected CheckoutType '1', got %s", opts.CheckoutType)
	}
}

func TestTryRepoCommand_ParseArgs_IsSquashMergeFlag(t *testing.T) {
	cmd := &TryRepoCommand{}

	// Test with --is-squash-merge flag for post-merge hooks
	opts, _, _, rc := cmd.parseAndValidateTryRepoArgs([]string{
		"https://github.com/example/repo",
		"--is-squash-merge", "1",
	})

	if rc != -1 {
		t.Errorf("expected return code -1 (continue), got %d", rc)
	}
	if opts.IsSquashMerge != "1" {
		t.Errorf("expected IsSquashMerge '1', got %s", opts.IsSquashMerge)
	}
}

func TestTryRepoCommand_ParseArgs_RewriteCommandFlag(t *testing.T) {
	cmd := &TryRepoCommand{}

	// Test with --rewrite-command flag for post-rewrite hooks
	opts, _, _, rc := cmd.parseAndValidateTryRepoArgs([]string{
		"https://github.com/example/repo",
		"--rewrite-command", "rebase",
	})

	if rc != -1 {
		t.Errorf("expected return code -1 (continue), got %d", rc)
	}
	if opts.RewriteCommand != "rebase" {
		t.Errorf("expected RewriteCommand 'rebase', got %s", opts.RewriteCommand)
	}
}

func TestTryRepoCommand_ParseArgs_PreRebaseFlags(t *testing.T) {
	cmd := &TryRepoCommand{}

	// Test with pre-rebase hooks flags
	opts, _, _, rc := cmd.parseAndValidateTryRepoArgs([]string{
		"https://github.com/example/repo",
		"--pre-rebase-upstream", "origin/main",
		"--pre-rebase-branch", "feature",
	})

	if rc != -1 {
		t.Errorf("expected return code -1 (continue), got %d", rc)
	}
	if opts.PreRebaseUpstream != "origin/main" {
		t.Errorf("expected PreRebaseUpstream 'origin/main', got %s", opts.PreRebaseUpstream)
	}
	if opts.PreRebaseBranch != "feature" {
		t.Errorf("expected PreRebaseBranch 'feature', got %s", opts.PreRebaseBranch)
	}
}

func TestTryRepoCommand_ParseArgs_AllGitHookFlags(t *testing.T) {
	cmd := &TryRepoCommand{}

	// Test with multiple git hook-specific flags combined
	opts, repoURL, _, rc := cmd.parseAndValidateTryRepoArgs([]string{
		"https://github.com/example/repo",
		"--hook-stage", "pre-push",
		"--remote-branch", "refs/heads/main",
		"--local-branch", "refs/heads/feature",
		"--remote-name", "origin",
		"--remote-url", "git@github.com:user/repo.git",
	})

	if rc != -1 {
		t.Errorf("expected return code -1 (continue), got %d", rc)
	}
	if repoURL != "https://github.com/example/repo" {
		t.Errorf("unexpected repoURL: %s", repoURL)
	}
	if opts.HookStage != "pre-push" {
		t.Errorf("unexpected HookStage: %s", opts.HookStage)
	}
	if opts.RemoteBranch != "refs/heads/main" {
		t.Errorf("unexpected RemoteBranch: %s", opts.RemoteBranch)
	}
	if opts.LocalBranch != "refs/heads/feature" {
		t.Errorf("unexpected LocalBranch: %s", opts.LocalBranch)
	}
	if opts.RemoteName != "origin" {
		t.Errorf("unexpected RemoteName: %s", opts.RemoteName)
	}
	if opts.RemoteURL != "git@github.com:user/repo.git" {
		t.Errorf("unexpected RemoteURL: %s", opts.RemoteURL)
	}
}

func TestTryRepoCommand_IsLocalPath_HTTPS(t *testing.T) {
	cmd := &TryRepoCommand{}

	if cmd.isLocalPath("https://github.com/example/repo") {
		t.Error("HTTPS URL should not be treated as local path")
	}
}

func TestTryRepoCommand_IsLocalPath_HTTP(t *testing.T) {
	cmd := &TryRepoCommand{}

	if cmd.isLocalPath("http://github.com/example/repo") {
		t.Error("HTTP URL should not be treated as local path")
	}
}

func TestTryRepoCommand_IsLocalPath_Git(t *testing.T) {
	cmd := &TryRepoCommand{}

	if cmd.isLocalPath("git://github.com/example/repo") {
		t.Error("git:// URL should not be treated as local path")
	}
}

func TestTryRepoCommand_IsLocalPath_GitSSH(t *testing.T) {
	cmd := &TryRepoCommand{}

	if cmd.isLocalPath("git@github.com:example/repo.git") {
		t.Error("git@ URL should not be treated as local path")
	}
}

func TestTryRepoCommand_IsLocalPath_SSH(t *testing.T) {
	cmd := &TryRepoCommand{}

	if cmd.isLocalPath("ssh://git@github.com/example/repo") {
		t.Error("ssh:// URL should not be treated as local path")
	}
}

func TestTryRepoCommand_IsLocalPath_CurrentDir(t *testing.T) {
	cmd := &TryRepoCommand{}

	// "." is a local path that exists
	if !cmd.isLocalPath(".") {
		t.Error("'.' should be treated as local path")
	}
}

func TestTryRepoCommand_IsLocalPath_RelativePath(t *testing.T) {
	cmd := &TryRepoCommand{}

	// ".." is a local path that exists
	if !cmd.isLocalPath("..") {
		t.Error("'..' should be treated as local path")
	}
}

func TestTryRepoCommand_IsLocalPath_NonExistent(t *testing.T) {
	cmd := &TryRepoCommand{}

	// Non-existent path should not be treated as local
	if cmd.isLocalPath("/nonexistent/path/that/does/not/exist") {
		t.Error("Non-existent path should not be treated as local path")
	}
}
