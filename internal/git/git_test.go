package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initTestRepo creates a temp git repo with an initial commit and returns its path.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
		}
	}

	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")

	// Create an initial file and commit.
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "README.md")
	run("commit", "-m", "initial commit")

	return dir
}

// --- NoGitEnv tests ---

func TestNoGitEnv(t *testing.T) {
	// Set some GIT_ env vars that should be stripped.
	t.Setenv("GIT_DIR", "/some/path")
	t.Setenv("GIT_WORK_TREE", "/other/path")
	t.Setenv("GIT_AUTHOR_NAME", "test")
	t.Setenv("GIT_INDEX_FILE", "/index")

	env := NoGitEnv()

	stripped := map[string]bool{
		"GIT_DIR":         true,
		"GIT_WORK_TREE":   true,
		"GIT_AUTHOR_NAME": true,
		"GIT_INDEX_FILE":  true,
	}
	for _, e := range env {
		name := strings.SplitN(e, "=", 2)[0]
		if stripped[name] {
			t.Errorf("NoGitEnv should strip %s but it was preserved", name)
		}
	}

	// Verify non-GIT vars are preserved.
	hasPath := false
	for _, e := range env {
		if strings.HasPrefix(e, "PATH=") || strings.HasPrefix(e, "HOME=") {
			hasPath = true
			break
		}
	}
	if !hasPath {
		t.Error("expected non-GIT_ vars to be preserved")
	}
}

func TestNoGitEnv_AllowedVars(t *testing.T) {
	// Allowed GIT_ vars should be preserved.
	t.Setenv("GIT_SSH", "ssh-custom")
	t.Setenv("GIT_SSH_COMMAND", "ssh -o Opt=val")
	t.Setenv("GIT_EXEC_PATH", "/usr/lib/git")
	t.Setenv("GIT_CONFIG_KEY_0", "user.name")
	t.Setenv("GIT_CONFIG_VALUE_0", "test")

	env := NoGitEnv()

	allowed := map[string]bool{
		"GIT_SSH":            false,
		"GIT_SSH_COMMAND":    false,
		"GIT_EXEC_PATH":      false,
		"GIT_CONFIG_KEY_0":   false,
		"GIT_CONFIG_VALUE_0": false,
	}
	for _, e := range env {
		name := strings.SplitN(e, "=", 2)[0]
		if _, ok := allowed[name]; ok {
			allowed[name] = true
		}
	}
	for name, found := range allowed {
		if !found {
			t.Errorf("NoGitEnv should preserve allowed var %s", name)
		}
	}
}

func TestNoGitEnv_NoGitVars(t *testing.T) {
	// Even without GIT_ vars set, should return a valid env.
	env := NoGitEnv()
	if len(env) == 0 {
		t.Error("expected non-empty environment")
	}
}

// --- Init tests ---

func TestInit(t *testing.T) {
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	// Verify .git directory exists.
	if _, err := os.Stat(filepath.Join(dir, ".git")); os.IsNotExist(err) {
		t.Error("expected .git directory after Init")
	}
}

// --- GetRoot tests ---

func TestGetRootInDir(t *testing.T) {
	dir := initTestRepo(t)
	root, err := GetRootInDir(dir)
	if err != nil {
		t.Fatalf("GetRootInDir failed: %v", err)
	}
	// Resolve symlinks for comparison (macOS /tmp is symlinked).
	expected, _ := filepath.EvalSymlinks(dir)
	actual, _ := filepath.EvalSymlinks(root)
	if actual != expected {
		t.Errorf("expected root %q, got %q", expected, actual)
	}
}

func TestGetRootInDir_NotGitRepo(t *testing.T) {
	dir := t.TempDir()
	_, err := GetRootInDir(dir)
	if err == nil {
		t.Error("expected error for non-git directory")
	}
}

// --- GetGitDir tests ---

func TestGetGitDir(t *testing.T) {
	dir := initTestRepo(t)
	gitDir, err := GetGitDir(dir)
	if err != nil {
		t.Fatalf("GetGitDir failed: %v", err)
	}
	expected, _ := filepath.EvalSymlinks(filepath.Join(dir, ".git"))
	actual, _ := filepath.EvalSymlinks(gitDir)
	if actual != expected {
		t.Errorf("expected %q, got %q", expected, actual)
	}
}

// --- GetGitCommonDir tests ---

func TestGetGitCommonDir(t *testing.T) {
	dir := initTestRepo(t)
	commonDir, err := GetGitCommonDir(dir)
	if err != nil {
		t.Fatalf("GetGitCommonDir failed: %v", err)
	}
	// For non-worktree repos, common dir == git dir.
	gitDir, _ := GetGitDir(dir)
	expectedCommon, _ := filepath.EvalSymlinks(commonDir)
	expectedGit, _ := filepath.EvalSymlinks(gitDir)
	if expectedCommon != expectedGit {
		t.Errorf("expected common dir %q to equal git dir %q", expectedCommon, expectedGit)
	}
}

// --- IsInsideWorkTreeInDir tests ---

func TestIsInsideWorkTreeInDir_True(t *testing.T) {
	dir := initTestRepo(t)
	if !IsInsideWorkTreeInDir(dir) {
		t.Error("expected true for git repo")
	}
}

func TestIsInsideWorkTreeInDir_False(t *testing.T) {
	dir := t.TempDir()
	if IsInsideWorkTreeInDir(dir) {
		t.Error("expected false for non-git directory")
	}
}

// --- GetHooksDir tests ---

func TestGetHooksDir(t *testing.T) {
	dir := initTestRepo(t)
	hooksDir, err := GetHooksDir(dir)
	if err != nil {
		t.Fatalf("GetHooksDir failed: %v", err)
	}
	expected, _ := filepath.EvalSymlinks(filepath.Join(dir, ".git", "hooks"))
	actual, _ := filepath.EvalSymlinks(hooksDir)
	if actual != expected {
		t.Errorf("expected %q, got %q", expected, actual)
	}
}

// --- GetHeadSHA tests ---

func TestGetHeadSHA(t *testing.T) {
	dir := initTestRepo(t)
	sha, err := GetHeadSHA(dir)
	if err != nil {
		t.Fatalf("GetHeadSHA failed: %v", err)
	}
	if len(sha) != 40 {
		t.Errorf("expected 40-char SHA, got %q (len=%d)", sha, len(sha))
	}
}

// --- HasUnstagedChanges / HasStagedChanges tests ---

func TestHasUnstagedChanges_Clean(t *testing.T) {
	dir := initTestRepo(t)
	has, err := HasUnstagedChanges(dir)
	if err != nil {
		t.Fatalf("HasUnstagedChanges failed: %v", err)
	}
	if has {
		t.Error("expected no unstaged changes in clean repo")
	}
}

func TestHasUnstagedChanges_WithChanges(t *testing.T) {
	dir := initTestRepo(t)
	// Modify tracked file without staging.
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("modified\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	has, err := HasUnstagedChanges(dir)
	if err != nil {
		t.Fatalf("HasUnstagedChanges failed: %v", err)
	}
	if !has {
		t.Error("expected unstaged changes after modifying tracked file")
	}
}

func TestHasStagedChanges_None(t *testing.T) {
	dir := initTestRepo(t)
	has, err := HasStagedChanges(dir)
	if err != nil {
		t.Fatalf("HasStagedChanges failed: %v", err)
	}
	if has {
		t.Error("expected no staged changes in clean repo")
	}
}

func TestHasStagedChanges_WithChanges(t *testing.T) {
	dir := initTestRepo(t)
	// Modify and stage.
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("modified\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", "README.md")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	has, err := HasStagedChanges(dir)
	if err != nil {
		t.Fatalf("HasStagedChanges failed: %v", err)
	}
	if !has {
		t.Error("expected staged changes after git add")
	}
}

// --- GetAllFiles tests ---

func TestGetAllFiles(t *testing.T) {
	dir := initTestRepo(t)

	// GetAllFiles works on the current directory, so we need to chdir.
	old, _ := os.Getwd()
	defer os.Chdir(old)
	os.Chdir(dir)

	files, err := GetAllFiles()
	if err != nil {
		t.Fatalf("GetAllFiles failed: %v", err)
	}
	if len(files) != 1 || files[0] != "README.md" {
		t.Errorf("expected [README.md], got %v", files)
	}
}

// --- GetStagedFiles tests ---

func TestGetStagedFiles_None(t *testing.T) {
	dir := initTestRepo(t)

	old, _ := os.Getwd()
	defer os.Chdir(old)
	os.Chdir(dir)

	files, err := GetStagedFiles()
	if err != nil {
		t.Fatalf("GetStagedFiles failed: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected no staged files, got %v", files)
	}
}

func TestGetStagedFiles_WithStaged(t *testing.T) {
	dir := initTestRepo(t)

	old, _ := os.Getwd()
	defer os.Chdir(old)
	os.Chdir(dir)

	// Create and stage a new file.
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", "new.txt")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	files, err := GetStagedFiles()
	if err != nil {
		t.Fatalf("GetStagedFiles failed: %v", err)
	}
	if len(files) != 1 || files[0] != "new.txt" {
		t.Errorf("expected [new.txt], got %v", files)
	}
}

func TestGetStagedFiles_ExcludesDeleted(t *testing.T) {
	dir := initTestRepo(t)

	old, _ := os.Getwd()
	defer os.Chdir(old)
	os.Chdir(dir)

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
		}
	}

	// Create and commit a second file.
	if err := os.WriteFile(filepath.Join(dir, "delete-me.yaml"), []byte("test: content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "delete-me.yaml")
	run("commit", "-m", "add file to delete")

	// Stage deletion of the file.
	run("rm", "delete-me.yaml")

	// Also stage a modification to another file.
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("modified\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "README.md")

	files, err := GetStagedFiles()
	if err != nil {
		t.Fatalf("GetStagedFiles failed: %v", err)
	}

	// Should only contain README.md, not delete-me.yaml.
	for _, f := range files {
		if f == "delete-me.yaml" {
			t.Error("GetStagedFiles should not include deleted files")
		}
	}
	if len(files) != 1 || files[0] != "README.md" {
		t.Errorf("expected [README.md], got %v", files)
	}
}

// --- ListTags tests ---

func TestListTags_NoTags(t *testing.T) {
	dir := initTestRepo(t)
	tags, err := ListTags(dir)
	if err != nil {
		t.Fatalf("ListTags failed: %v", err)
	}
	if len(tags) != 0 {
		t.Errorf("expected no tags, got %v", tags)
	}
}

func TestListTags_WithTags(t *testing.T) {
	dir := initTestRepo(t)

	// Create tags.
	for _, tag := range []string{"v1.0.0", "v2.0.0", "v0.1.0"} {
		cmd := exec.Command("git", "tag", tag)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Fatal(err)
		}
	}

	tags, err := ListTags(dir)
	if err != nil {
		t.Fatalf("ListTags failed: %v", err)
	}
	if len(tags) != 3 {
		t.Fatalf("expected 3 tags, got %d: %v", len(tags), tags)
	}
	// Should be sorted by version.
	if tags[0] != "v0.1.0" || tags[1] != "v1.0.0" || tags[2] != "v2.0.0" {
		t.Errorf("expected [v0.1.0 v1.0.0 v2.0.0], got %v", tags)
	}
}

// --- GetLatestTag tests ---

func TestGetLatestTag(t *testing.T) {
	dir := initTestRepo(t)

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
		}
	}

	// Tag first commit.
	run("tag", "v1.0.0")

	// Create a second commit and tag it.
	if err := os.WriteFile(filepath.Join(dir, "file2.txt"), []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "file2.txt")
	run("commit", "-m", "second commit")
	run("tag", "v2.0.0")

	tag, err := GetLatestTag(dir)
	if err != nil {
		t.Fatalf("GetLatestTag failed: %v", err)
	}
	if tag != "v2.0.0" {
		t.Errorf("expected v2.0.0, got %q", tag)
	}
}

// --- GetTagSHA tests ---

func TestGetTagSHA(t *testing.T) {
	dir := initTestRepo(t)
	headSHA, _ := GetHeadSHA(dir)

	cmd := exec.Command("git", "tag", "v1.0.0")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	sha, err := GetTagSHA(dir, "v1.0.0")
	if err != nil {
		t.Fatalf("GetTagSHA failed: %v", err)
	}
	if sha != headSHA {
		t.Errorf("expected %q, got %q", headSHA, sha)
	}
}

// --- WriteTree tests ---

func TestWriteTree(t *testing.T) {
	dir := initTestRepo(t)
	hash, err := WriteTree(dir)
	if err != nil {
		t.Fatalf("WriteTree failed: %v", err)
	}
	if len(hash) != 40 {
		t.Errorf("expected 40-char tree hash, got %q (len=%d)", hash, len(hash))
	}
}

// --- Checkout tests ---

func TestCheckout(t *testing.T) {
	dir := initTestRepo(t)

	// Create a branch and switch back.
	cmd := exec.Command("git", "branch", "test-branch")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	if err := Checkout(dir, "test-branch"); err != nil {
		t.Fatalf("Checkout failed: %v", err)
	}

	// Verify we're on the branch.
	out, err := CmdOutputInDir(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if out != "test-branch" {
		t.Errorf("expected test-branch, got %q", out)
	}
}

// --- CmdOutput / CmdOutputInDir tests ---

func TestCmdOutputInDir(t *testing.T) {
	dir := initTestRepo(t)
	out, err := CmdOutputInDir(dir, "status", "--porcelain")
	if err != nil {
		t.Fatalf("CmdOutputInDir failed: %v", err)
	}
	// Clean repo should have empty porcelain status.
	if out != "" {
		t.Errorf("expected empty status, got %q", out)
	}
}

func TestCmdOutputInDir_Error(t *testing.T) {
	dir := t.TempDir()
	_, err := CmdOutputInDir(dir, "log")
	if err == nil {
		t.Error("expected error for non-git directory")
	}
}

// --- DiffInDir tests ---

func TestDiffInDir_NoDiff(t *testing.T) {
	dir := initTestRepo(t)
	diff, err := DiffInDir(dir)
	if err != nil {
		t.Fatalf("DiffInDir failed: %v", err)
	}
	if diff != "" {
		t.Errorf("expected empty diff, got %q", diff)
	}
}

func TestDiffInDir_WithChanges(t *testing.T) {
	dir := initTestRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	diff, err := DiffInDir(dir)
	if err != nil {
		t.Fatalf("DiffInDir failed: %v", err)
	}
	if !strings.Contains(diff, "changed") {
		t.Errorf("expected diff to contain 'changed', got %q", diff)
	}
}

// --- CheckoutIndex tests ---

func TestCheckoutIndex(t *testing.T) {
	dir := initTestRepo(t)
	dest := t.TempDir()
	if err := CheckoutIndex(dir, dest); err != nil {
		t.Fatalf("CheckoutIndex failed: %v", err)
	}
	// Verify file was checked out.
	content, err := os.ReadFile(filepath.Join(dest, "README.md"))
	if err != nil {
		t.Fatalf("expected README.md in checkout dest: %v", err)
	}
	if string(content) != "# test\n" {
		t.Errorf("unexpected content: %q", string(content))
	}
}
