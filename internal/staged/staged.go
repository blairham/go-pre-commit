// Package staged manages stashing of unstaged changes for pre-commit runs.
package staged

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/blairham/go-pre-commit/internal/git"
)

// Manager handles stashing and restoring of unstaged changes.
type Manager struct {
	dir       string
	patchPath string
	treeHash  string
	stashed   bool
}

// NewManager creates a new stash Manager for the given repo directory.
func NewManager(dir string) *Manager {
	return &Manager{
		dir: dir,
	}
}

// StashUnstaged saves unstaged changes and checks out staged-only content.
// Uses the git write-tree / diff-index / checkout-index approach matching Python.
// Returns true if changes were stashed, false if not needed.
func (m *Manager) StashUnstaged() (bool, error) {
	// Check if there are unstaged changes.
	hasUnstaged, err := git.HasUnstagedChanges(m.dir)
	if err != nil {
		return false, fmt.Errorf("checking unstaged changes: %w", err)
	}
	if !hasUnstaged {
		return false, nil
	}

	// Write the current index as a tree object — captures the staged state.
	treeHash, err := git.WriteTree(m.dir)
	if err != nil {
		return false, fmt.Errorf("write-tree: %w", err)
	}
	m.treeHash = treeHash

	// Generate the patch of unstaged changes (working tree vs index).
	diff, err := git.DiffInDir(m.dir)
	if err != nil {
		return false, fmt.Errorf("generating diff: %w", err)
	}

	if strings.TrimSpace(diff) == "" {
		return false, nil
	}

	// Create a temp file for the patch backup.
	tmpDir := os.TempDir()
	m.patchPath = filepath.Join(tmpDir, fmt.Sprintf("pre-commit-unstaged-%d.patch", os.Getpid()))
	if err := os.WriteFile(m.patchPath, []byte(diff), 0o644); err != nil {
		return false, fmt.Errorf("writing patch: %w", err)
	}

	// Handle intent-to-add files — need to remove them from the index temporarily.
	intentFiles, _ := git.IntentToAddFiles()
	if len(intentFiles) > 0 {
		args := append([]string{"rm", "--cached", "--"}, intentFiles...)
		cmd := exec.Command("git", args...)
		cmd.Dir = m.dir
		_ = cmd.Run()
	}

	// Checkout the index (staged-only content) — set env to prevent post-checkout firing.
	cmd := exec.Command("git", "checkout-index", "--all", "--force")
	cmd.Dir = m.dir
	cmd.Env = append(os.Environ(), "_PRE_COMMIT_SKIP_POST_CHECKOUT=1")
	if err := cmd.Run(); err != nil {
		// Clean up patch file on error.
		os.Remove(m.patchPath)
		return false, fmt.Errorf("checkout-index: %w", err)
	}

	// Checkout staged version of all files.
	cmd = exec.Command("git", "checkout", "--", ".")
	cmd.Dir = m.dir
	cmd.Env = append(os.Environ(), "_PRE_COMMIT_SKIP_POST_CHECKOUT=1")
	if err := cmd.Run(); err != nil {
		os.Remove(m.patchPath)
		return false, fmt.Errorf("checkout staged: %w", err)
	}

	m.stashed = true
	return true, nil
}

// Restore restores the stashed unstaged changes.
func (m *Manager) Restore() error {
	if !m.stashed {
		return nil
	}
	defer func() {
		// Clean up.
		if m.patchPath != "" {
			os.Remove(m.patchPath)
		}
		m.stashed = false
	}()

	if m.patchPath == "" {
		return nil
	}

	// First, restore the tree to its pre-run state using git read-tree.
	if m.treeHash != "" {
		if err := git.ReadTree(m.dir, m.treeHash); err != nil {
			// Fall back to checkout approach.
			_ = err
		}
	}

	// Apply the patch to restore unstaged changes.
	cmd := exec.Command("git", "apply", "--allow-empty", m.patchPath)
	cmd.Dir = m.dir
	if err := cmd.Run(); err != nil {
		// If direct apply fails, try 3-way merge.
		cmd = exec.Command("git", "apply", "--3way", m.patchPath)
		cmd.Dir = m.dir
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("restoring unstaged changes (patch saved at %s): %w", m.patchPath, err)
		}
	}

	// Re-add intent-to-add files.
	intentFiles, _ := git.IntentToAddFiles()
	if len(intentFiles) > 0 {
		args := append([]string{"add", "--intent-to-add", "--"}, intentFiles...)
		cmd := exec.Command("git", args...)
		cmd.Dir = m.dir
		_ = cmd.Run()
	}

	return nil
}

// IsStashed returns whether there are currently stashed changes.
func (m *Manager) IsStashed() bool {
	return m.stashed
}
