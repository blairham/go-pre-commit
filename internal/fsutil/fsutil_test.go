package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

// writeModuleCache builds the shape `go install` leaves behind: files mode
// 0444 inside directories mode 0555. Unlinking needs write+execute on the
// parent directory, so a plain os.RemoveAll cannot clear this.
func writeModuleCache(t *testing.T, root string) string {
	t.Helper()

	envDir := filepath.Join(root, "go_env-default")
	modDir := filepath.Join(envDir, "pkg", "mod", "example.com", "dep@v1.0.0")
	nested := filepath.Join(modDir, "internal")

	for _, d := range []string{modDir, nested} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{
		filepath.Join(modDir, "dep.go"),
		filepath.Join(nested, "deep.go"),
	} {
		if err := os.WriteFile(f, []byte("package dep\n"), 0o444); err != nil {
			t.Fatal(err)
		}
	}

	// Lock down deepest-first, mirroring the real cache.
	for _, d := range []string{nested, modDir, filepath.Dir(modDir)} {
		if err := os.Chmod(d, 0o555); err != nil {
			t.Fatal(err)
		}
	}

	// Always restore write bits so t.TempDir cleanup can succeed even if the
	// test fails partway.
	t.Cleanup(func() {
		_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
			if err == nil && info.IsDir() {
				_ = os.Chmod(p, 0o755)
			}
			return nil
		})
	})

	return envDir
}

// This is the precondition for the whole bug: the standard library call the
// store used to make cannot remove an installed Go environment.
func TestPlainRemoveAllCannotClearAModuleCache(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "repo-abc")
	writeModuleCache(t, target)

	if err := os.RemoveAll(target); err == nil {
		t.Skip("os.RemoveAll cleared a read-only tree on this platform; " +
			"the wedge this package guards against cannot occur here")
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("expected the directory to survive the failed removal: %v", err)
	}
}

func TestRemoveAllClearsAReadOnlyModuleCache(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "repo-abc")
	writeModuleCache(t, target)

	if err := RemoveAll(target); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("directory still present after RemoveAll (stat err = %v)", err)
	}
}

func TestRemoveAllIsIdempotentOnMissingPath(t *testing.T) {
	if err := RemoveAll(filepath.Join(t.TempDir(), "never-existed")); err != nil {
		t.Errorf("removing a missing path should succeed, got %v", err)
	}
}

func TestRemoveAllHandlesAPlainTree(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "plain")
	if err := os.MkdirAll(filepath.Join(target, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "a", "b", "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := RemoveAll(target); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("directory still present (stat err = %v)", err)
	}
}

func TestRemoveAllOnASingleReadOnlyFile(t *testing.T) {
	root := t.TempDir()
	f := filepath.Join(root, "ro.txt")
	if err := os.WriteFile(f, []byte("x"), 0o444); err != nil {
		t.Fatal(err)
	}
	if err := RemoveAll(f); err != nil {
		t.Fatalf("RemoveAll on a read-only file: %v", err)
	}
	if _, err := os.Stat(f); !os.IsNotExist(err) {
		t.Errorf("file still present (stat err = %v)", err)
	}
}
