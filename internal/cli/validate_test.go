// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blairham/go-pre-commit/v4/internal/config"
)

func writeValidateFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func loadConfigOnly(filename string) error {
	_, err := config.LoadConfig(filename)
	return err
}

func loadManifestOnly(filename string) error {
	_, err := config.LoadManifest(filename)
	return err
}

// Upstream's validate_config / validate_manifest print nothing for a valid
// file (#145).
func TestValidateFilesSilentOnSuccess(t *testing.T) {
	dir := t.TempDir()
	cfg := writeValidateFile(t, dir, "c.yaml",
		"repos:\n- repo: local\n  hooks:\n  - id: x\n    name: x\n    entry: x\n    language: system\n")
	man := writeValidateFile(t, dir, "m.yaml", "- id: x\n  name: x\n  entry: x\n  language: system\n")

	for _, tc := range []struct {
		load func(string) error
		name string
		file string
	}{
		{name: "config", file: cfg, load: loadConfigOnly},
		{name: "manifest", file: man, load: loadManifestOnly},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, rc := validateOutput([]string{tc.file}, tc.load)
			if rc != 0 {
				t.Fatalf("rc = %d, want 0", rc)
			}
			if out != "" {
				t.Errorf("output = %q, want none", out)
			}
		})
	}
}

// With no filenames upstream (argparse nargs='*') validates nothing and
// exits 0; it does not fall back to the default config.
func TestValidateFilesNoArgs(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, cmd := range []interface{ Run([]string) int }{
		&ValidateConfigCommand{},
		&ValidateManifestCommand{},
	} {
		if rc := cmd.Run(nil); rc != 0 {
			t.Errorf("%T.Run(nil) = %d, want 0", cmd, rc)
		}
	}
}

// A missing path fails before cfgv enters the File context.
func TestValidateFilesMissing(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.yaml")
	for _, path := range []string{missing, dir} {
		out, rc := validateOutput([]string{path}, loadConfigOnly)
		if rc != 1 {
			t.Fatalf("rc = %d, want 1", rc)
		}
		if want := "\n=====> " + path + " is not a file\n"; out != want {
			t.Errorf("output = %q, want %q", out, want)
		}
	}
}

// An invalid file is reported under "==> File", and every file is checked.
func TestValidateFilesInvalid(t *testing.T) {
	dir := t.TempDir()
	bad := writeValidateFile(t, dir, "bad.yaml", "repos:\n- repo: local\n  hooks:\n  - id: x\n")
	missing := filepath.Join(dir, "missing.yaml")

	out, rc := validateOutput([]string{bad, missing}, loadConfigOnly)
	if rc != 1 {
		t.Fatalf("rc = %d, want 1", rc)
	}
	if !strings.HasPrefix(out, "\n==> File "+bad+"\n=====> ") {
		t.Errorf("output does not start with the File context: %q", out)
	}
	if strings.Contains(out, "invalid config") {
		t.Errorf("output carries our wrapping prefix: %q", out)
	}
	if !strings.HasSuffix(out, "\n\n=====> "+missing+" is not a file\n") {
		t.Errorf("second file not reported: %q", out)
	}
}
