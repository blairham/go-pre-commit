package commands

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateManifestCommand_ValidFile(t *testing.T) {
	// Create a temp directory with a valid manifest
	tmpDir, err := os.MkdirTemp("", "validate-manifest-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	validManifest := `- id: my-hook
  name: My Hook
  entry: echo hello
  language: system
`
	manifestPath := filepath.Join(tmpDir, ".pre-commit-hooks.yaml")
	if err := os.WriteFile(manifestPath, []byte(validManifest), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := &ValidateManifestCommand{}
	ret := cmd.Run([]string{manifestPath})

	if ret != 0 {
		t.Errorf("expected return code 0, got %d", ret)
	}
}

func TestValidateManifestCommand_InvalidFile(t *testing.T) {
	// Create a temp directory with an invalid YAML file
	tmpDir, err := os.MkdirTemp("", "validate-manifest-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	invalidYAML := `{invalid yaml`
	manifestPath := filepath.Join(tmpDir, "invalid.yaml")
	if err := os.WriteFile(manifestPath, []byte(invalidYAML), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := &ValidateManifestCommand{}
	ret := cmd.Run([]string{manifestPath})

	if ret != 1 {
		t.Errorf("expected return code 1 for invalid YAML, got %d", ret)
	}
}

func TestValidateManifestCommand_NonExistentFile(t *testing.T) {
	cmd := &ValidateManifestCommand{}
	ret := cmd.Run([]string{"/nonexistent/file.yaml"})

	if ret != 1 {
		t.Errorf("expected return code 1 for non-existent file, got %d", ret)
	}
}

func TestValidateManifestCommand_MultipleFiles(t *testing.T) {
	// Create a temp directory with multiple valid manifests
	tmpDir, err := os.MkdirTemp("", "validate-manifest-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	validManifest := `- id: my-hook
  name: My Hook
  entry: echo hello
  language: system
`
	manifest1 := filepath.Join(tmpDir, "manifest1.yaml")
	manifest2 := filepath.Join(tmpDir, "manifest2.yaml")
	if err := os.WriteFile(manifest1, []byte(validManifest), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest2, []byte(validManifest), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := &ValidateManifestCommand{}
	ret := cmd.Run([]string{manifest1, manifest2})

	if ret != 0 {
		t.Errorf("expected return code 0 for multiple valid files, got %d", ret)
	}
}

func TestValidateManifestCommand_MultipleFilesOneFails(t *testing.T) {
	// Create a temp directory with one valid and one invalid manifest
	tmpDir, err := os.MkdirTemp("", "validate-manifest-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	validManifest := `- id: my-hook
  name: My Hook
  entry: echo hello
  language: system
`
	invalidYAML := `{invalid yaml`
	validPath := filepath.Join(tmpDir, "valid.yaml")
	invalidPath := filepath.Join(tmpDir, "invalid.yaml")
	if err := os.WriteFile(validPath, []byte(validManifest), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(invalidPath, []byte(invalidYAML), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := &ValidateManifestCommand{}
	// Pass both files - should continue processing even after first error
	ret := cmd.Run([]string{validPath, invalidPath})

	if ret != 1 {
		t.Errorf("expected return code 1 when one file fails, got %d", ret)
	}
}

func TestValidateManifestCommand_NoFiles(t *testing.T) {
	// Python behavior: no files = validate 0 files = return 0
	cmd := &ValidateManifestCommand{}
	ret := cmd.Run([]string{})

	if ret != 0 {
		t.Errorf("expected return code 0 with no files (Python behavior), got %d", ret)
	}
}

func TestValidateManifestCommand_SilentOnSuccess(t *testing.T) {
	// Create a temp directory with a valid manifest
	tmpDir, err := os.MkdirTemp("", "validate-manifest-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	validManifest := `- id: my-hook
  name: My Hook
  entry: echo hello
  language: system
`
	manifestPath := filepath.Join(tmpDir, ".pre-commit-hooks.yaml")
	if err := os.WriteFile(manifestPath, []byte(validManifest), 0644); err != nil {
		t.Fatal(err)
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := &ValidateManifestCommand{}
	cmd.Run([]string{manifestPath})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if output != "" {
		t.Errorf("expected silent output on success (Python behavior), got %q", output)
	}
}

func TestValidateManifestCommand_EmptyManifest(t *testing.T) {
	// Create a temp directory with an empty manifest (valid - no hooks)
	tmpDir, err := os.MkdirTemp("", "validate-manifest-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	emptyManifest := `[]`
	manifestPath := filepath.Join(tmpDir, ".pre-commit-hooks.yaml")
	if err := os.WriteFile(manifestPath, []byte(emptyManifest), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := &ValidateManifestCommand{}
	ret := cmd.Run([]string{manifestPath})

	if ret != 0 {
		t.Errorf("expected return code 0 for empty manifest, got %d", ret)
	}
}

func TestValidateManifestCommand_Help(t *testing.T) {
	cmd := &ValidateManifestCommand{}
	help := cmd.Help()

	if !strings.Contains(help, "validate-manifest") {
		t.Error("help should contain 'validate-manifest'")
	}
	if !strings.Contains(help, "filenames") {
		t.Error("help should mention 'filenames'")
	}
}

func TestValidateManifestCommand_Synopsis(t *testing.T) {
	cmd := &ValidateManifestCommand{}
	synopsis := cmd.Synopsis()

	if !strings.Contains(synopsis, "Validate") {
		t.Error("synopsis should contain 'Validate'")
	}
	if !strings.Contains(synopsis, ".pre-commit-hooks.yaml") {
		t.Error("synopsis should mention '.pre-commit-hooks.yaml'")
	}
}

func TestValidateManifestCommand_ContinuesOnError(t *testing.T) {
	// Verify that processing continues even when files have errors
	tmpDir, err := os.MkdirTemp("", "validate-manifest-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	validManifest := `- id: my-hook
  name: My Hook
  entry: echo hello
  language: system
`
	invalidYAML := `{invalid`

	// Create 3 files: invalid, valid, invalid
	invalid1 := filepath.Join(tmpDir, "invalid1.yaml")
	valid := filepath.Join(tmpDir, "valid.yaml")
	invalid2 := filepath.Join(tmpDir, "invalid2.yaml")

	os.WriteFile(invalid1, []byte(invalidYAML), 0644)
	os.WriteFile(valid, []byte(validManifest), 0644)
	os.WriteFile(invalid2, []byte(invalidYAML), 0644)

	// Capture stdout to verify errors are printed
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := &ValidateManifestCommand{}
	ret := cmd.Run([]string{invalid1, valid, invalid2})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if ret != 1 {
		t.Errorf("expected return code 1, got %d", ret)
	}

	// Should have processed all files - count the number of error messages
	// Each invalid file should produce an error line
	errorCount := strings.Count(output, "failed to parse YAML")
	if errorCount != 2 {
		t.Errorf("expected 2 YAML parse errors (one for each invalid file), got %d", errorCount)
	}
}
