package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- SampleConfigCommand tests ---

func TestSampleConfigCommand_Run(t *testing.T) {
	cmd := &SampleConfigCommand{Meta: &Meta{}}

	// Capture stdout by redirecting.
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := cmd.Run(nil)

	w.Close()
	os.Stdout = old

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if !strings.Contains(output, "repos:") {
		t.Error("expected sample config to contain 'repos:'")
	}
	if !strings.Contains(output, "pre-commit-hooks") {
		t.Error("expected sample config to reference pre-commit-hooks")
	}
}

// --- ValidateConfigCommand tests ---

func TestValidateConfigCommand_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `repos:
-   repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.0.0
    hooks:
    -   id: trailing-whitespace
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := &ValidateConfigCommand{Meta: &Meta{}}

	// Capture stdout.
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := cmd.Run([]string{cfgPath})

	w.Close()
	os.Stdout = old

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	out := string(buf[:n])

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(out, "valid") {
		t.Errorf("expected 'valid' in output, got %q", out)
	}
}

func TestValidateConfigCommand_InvalidFile(t *testing.T) {
	cmd := &ValidateConfigCommand{Meta: &Meta{}}
	code := cmd.Run([]string{"/nonexistent/file.yaml"})
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
}

func TestValidateConfigCommand_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(cfgPath, []byte("{{{{not yaml"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := &ValidateConfigCommand{Meta: &Meta{}}
	code := cmd.Run([]string{cfgPath})
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
}

// --- ValidateManifestCommand tests ---

func TestValidateManifestCommand_ValidManifest(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, ".pre-commit-hooks.yaml")
	content := `-   id: my-hook
    name: My Hook
    entry: echo hello
    language: system
`
	if err := os.WriteFile(manifestPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := &ValidateManifestCommand{Meta: &Meta{}}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := cmd.Run([]string{manifestPath})

	w.Close()
	os.Stdout = old

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	out := string(buf[:n])

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(out, "valid") {
		t.Errorf("expected 'valid' in output, got %q", out)
	}
}

func TestValidateManifestCommand_InvalidFile(t *testing.T) {
	cmd := &ValidateManifestCommand{Meta: &Meta{}}
	code := cmd.Run([]string{"/nonexistent/manifest.yaml"})
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
}

// --- MigrateConfigCommand tests ---

func TestMigrateConfigCommand_ShaToRev(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `repos:
-   repo: https://github.com/example/hooks
    sha: abc123
    hooks:
    -   id: my-hook
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := &MigrateConfigCommand{Meta: &Meta{}}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := cmd.Run([]string{"--config", cfgPath})

	w.Close()
	os.Stdout = old

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	out := string(buf[:n])

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(out, "migrated") {
		t.Errorf("expected 'migrated' in output, got %q", out)
	}

	// Verify file was updated.
	updated, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(updated), "sha:") {
		t.Error("expected sha: to be replaced with rev:")
	}
	if !strings.Contains(string(updated), "rev:") {
		t.Error("expected rev: in migrated config")
	}
}

func TestMigrateConfigCommand_PythonVenvToLanguage(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `repos:
-   repo: https://github.com/example/hooks
    rev: v1.0.0
    hooks:
    -   id: my-hook
        language: python_venv
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := &MigrateConfigCommand{Meta: &Meta{}}

	// Suppress stdout.
	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w
	code := cmd.Run([]string{"--config", cfgPath})
	w.Close()
	os.Stdout = old

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	updated, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(updated), "python_venv") {
		t.Error("expected python_venv to be replaced with python")
	}
	if !strings.Contains(string(updated), "language: python") {
		t.Error("expected 'language: python' in migrated config")
	}
}

func TestMigrateConfigCommand_AlreadyUpToDate(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `repos:
-   repo: https://github.com/example/hooks
    rev: v1.0.0
    hooks:
    -   id: my-hook
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := &MigrateConfigCommand{Meta: &Meta{}}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	code := cmd.Run([]string{"--config", cfgPath})
	w.Close()
	os.Stdout = old

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	out := string(buf[:n])

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(out, "already up to date") {
		t.Errorf("expected 'already up to date' in output, got %q", out)
	}
}

func TestMigrateConfigCommand_MissingFile(t *testing.T) {
	cmd := &MigrateConfigCommand{Meta: &Meta{}}
	code := cmd.Run([]string{"--config", "/nonexistent/config.yaml"})
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
}

func TestMigrateConfigCommand_StageNameMigration(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `repos:
-   repo: https://github.com/example/hooks
    rev: v1.0.0
    hooks:
    -   id: my-hook
        stages:
        - commit
        - push
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := &MigrateConfigCommand{Meta: &Meta{}}

	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w
	code := cmd.Run([]string{"--config", cfgPath})
	w.Close()
	os.Stdout = old

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	updated, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(updated)
	if strings.Contains(s, "- commit\n") && !strings.Contains(s, "- pre-commit") {
		t.Error("expected '- commit' to be migrated to '- pre-commit'")
	}
	if strings.Contains(s, "- push\n") && !strings.Contains(s, "- pre-push") {
		t.Error("expected '- push' to be migrated to '- pre-push'")
	}
}

// --- InitTemplateDirCommand tests ---

func TestInitTemplateDirCommand_CreatesHook(t *testing.T) {
	dir := t.TempDir()
	templateDir := filepath.Join(dir, "template")

	cmd := &InitTemplateDirCommand{Meta: &Meta{}}

	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w
	code := cmd.Run([]string{templateDir})
	w.Close()
	os.Stdout = old

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	hookFile := filepath.Join(templateDir, "hooks", "pre-commit")
	content, err := os.ReadFile(hookFile)
	if err != nil {
		t.Fatalf("expected hook file to exist: %v", err)
	}
	if !strings.Contains(string(content), "hook-impl") {
		t.Error("expected hook content to contain 'hook-impl'")
	}

	// Verify file is executable.
	info, _ := os.Stat(hookFile)
	if info.Mode()&0o111 == 0 {
		t.Error("expected hook file to be executable")
	}
}

func TestInitTemplateDirCommand_CustomHookType(t *testing.T) {
	dir := t.TempDir()
	templateDir := filepath.Join(dir, "template")

	cmd := &InitTemplateDirCommand{Meta: &Meta{}}

	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w
	code := cmd.Run([]string{"-t", "pre-push", templateDir})
	w.Close()
	os.Stdout = old

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	hookFile := filepath.Join(templateDir, "hooks", "pre-push")
	if _, err := os.Stat(hookFile); os.IsNotExist(err) {
		t.Fatal("expected pre-push hook file to exist")
	}
}

func TestInitTemplateDirCommand_NoArgs(t *testing.T) {
	cmd := &InitTemplateDirCommand{Meta: &Meta{}}
	code := cmd.Run(nil)
	if code != 1 {
		t.Fatalf("expected exit code 1 for missing args, got %d", code)
	}
}

// --- CleanCommand tests ---

func TestCleanCommand_Run(t *testing.T) {
	// Set up a temp cache dir.
	dir := t.TempDir()
	t.Setenv("PRE_COMMIT_HOME", dir)

	// Create the db file that store.Clean would remove.
	dbDir := filepath.Join(dir, "db.db")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := &CleanCommand{Meta: &Meta{}}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	code := cmd.Run(nil)
	w.Close()
	os.Stdout = old

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	out := string(buf[:n])

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(out, "Cleaned") {
		t.Errorf("expected 'Cleaned' in output, got %q", out)
	}
}
