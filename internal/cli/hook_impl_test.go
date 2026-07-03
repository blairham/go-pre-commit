package cli

import (
	"path/filepath"
	"testing"
)

func TestLegacyHookDir(t *testing.T) {
	t.Setenv("GIT_DIR", "")

	if got, want := legacyHookDir("/custom/hooks"), "/custom/hooks"; got != want {
		t.Errorf("legacyHookDir with --hook-dir = %q, want %q", got, want)
	}
	if got, want := legacyHookDir(""), filepath.Join(".git", "hooks"); got != want {
		t.Errorf("legacyHookDir without GIT_DIR = %q, want %q", got, want)
	}

	t.Setenv("GIT_DIR", "/repo/.git")
	if got, want := legacyHookDir(""), filepath.Join("/repo/.git", "hooks"); got != want {
		t.Errorf("legacyHookDir with GIT_DIR = %q, want %q", got, want)
	}
	// --hook-dir wins over GIT_DIR, matching Python pre-commit's hook-impl.
	if got, want := legacyHookDir("/custom/hooks"), "/custom/hooks"; got != want {
		t.Errorf("legacyHookDir precedence = %q, want %q", got, want)
	}
}
