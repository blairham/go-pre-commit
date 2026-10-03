// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

// Package staged manages stashing of unstaged changes for pre-commit runs.
package staged

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/blairham/go-pre-commit/v4/internal/git"
	"github.com/blairham/go-pre-commit/v4/internal/output"
)

// Manager handles stashing and restoring of unstaged changes.
//
// It mirrors upstream pre-commit's staged_files_only: intent-to-add entries are
// dropped from the index for the run, the unstaged changes are saved as a
// binary patch of the working tree against the index, the working tree is
// reset to the index, and afterwards the patch is applied back. Every git
// command runs against the host's index as git handed it to the hook
// (git.HostIndexEnv), which is not the default index under `git commit -a` or
// `git commit <paths>`.
type Manager struct {
	dir         string
	patchDir    string // where the patch is kept: the store, as upstream does
	patchPath   string
	intentFiles []string
	stashed     bool
}

// NewManager creates a stash Manager for the repo at dir that keeps its patch
// files in patchDir (the store directory, as upstream does).
func NewManager(dir, patchDir string) *Manager {
	return &Manager{
		dir:      dir,
		patchDir: patchDir,
	}
}

// gitCmd builds a git command against the host index in the manager's repo.
func (m *Manager) gitCmd(args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Dir = m.dir
	cmd.Env = git.HostIndexEnv()
	return cmd
}

func (m *Manager) run(args ...string) error {
	cmd := m.gitCmd(args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s: %w\nstderr: %s", strings.Join(args, " "), err, stderr.String())
	}
	return nil
}

// StashUnstaged saves unstaged changes and checks out staged-only content.
// Returns true if changes were stashed, false if there were none.
func (m *Manager) StashUnstaged() (bool, error) {
	if err := m.clearIntentToAdd(); err != nil {
		return false, err
	}
	stashed, err := m.stash()
	if !stashed {
		m.restoreIntentToAdd()
	}
	return stashed, err
}

func (m *Manager) stash() (bool, error) {
	// Write the index as a tree so the patch is taken against exactly what is
	// being committed.
	tree, err := git.WriteTree(m.dir)
	if err != nil {
		return false, fmt.Errorf("write-tree: %w", err)
	}

	// The raw stdout is the patch: trimming it would corrupt it (git apply
	// needs the trailing newline), and --binary keeps binary files appliable.
	cmd := m.gitCmd("diff-index", "--ignore-submodules", "--binary", "--exit-code",
		"--no-color", "--no-ext-diff", tree, "--")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	diff, err := cmd.Output()
	if err == nil {
		return false, nil // exit 0: no unstaged changes
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		return false, fmt.Errorf("diff-index: %w\nstderr: %s", err, stderr.String())
	}
	if len(bytes.TrimSpace(diff)) == 0 {
		return false, nil // e.g. only submodule or mode-only noise git declines to print
	}

	// Upstream's name and place: patch<unix time>-<pid> in the store, kept
	// after the run, because the path it prints is how someone recovers their
	// unstaged work when a hook or the process dies before the restore.
	m.patchPath = filepath.Join(m.patchDir, fmt.Sprintf("patch%d-%d", time.Now().Unix(), os.Getpid()))
	output.Warn("Unstaged files detected.")
	output.Info("Stashing unstaged files to %s.", m.patchPath)
	if err := os.MkdirAll(m.patchDir, 0o755); err != nil {
		return false, fmt.Errorf("creating patch directory: %w", err)
	}
	if err := os.WriteFile(m.patchPath, diff, 0o644); err != nil {
		m.patchPath = ""
		return false, fmt.Errorf("writing patch: %w", err)
	}

	if err := m.checkoutIndex(); err != nil {
		return false, fmt.Errorf("checkout (patch saved at %s): %w", m.patchPath, err)
	}

	m.stashed = true
	return true, nil
}

// checkoutIndex resets the working tree to the index. The post-checkout skip
// flag stops our own post-checkout hook from running recursively.
func (m *Manager) checkoutIndex() error {
	args := []string{"-c", "submodule.recurse=0", "checkout", "--", "."}
	cmd := m.gitCmd(args...)
	cmd.Env = append(cmd.Env, "_PRE_COMMIT_SKIP_POST_CHECKOUT=1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s: %w\nstderr: %s", strings.Join(args, " "), err, stderr.String())
	}
	return nil
}

func (m *Manager) applyPatch() error {
	if err := m.run("apply", "--whitespace=nowarn", m.patchPath); err != nil {
		// Upstream retries with autocrlf off for CRLF working trees.
		return m.run("-c", "core.autocrlf=false", "apply", "--whitespace=nowarn", m.patchPath)
	}
	return nil
}

// clearIntentToAdd drops intent-to-add entries from the index for the run:
// they have no content to stash or check out, and git refuses to write-tree
// some of them.
func (m *Manager) clearIntentToAdd() error {
	files, err := git.IntentToAddFiles(m.dir)
	if err != nil {
		return fmt.Errorf("listing intent-to-add files: %w", err)
	}
	if len(files) == 0 {
		return nil
	}
	output.Warn("Unstaged intent-to-add files detected.")
	if err := m.run(append([]string{"rm", "--cached", "--"}, files...)...); err != nil {
		return err
	}
	m.intentFiles = files
	return nil
}

func (m *Manager) restoreIntentToAdd() {
	if len(m.intentFiles) == 0 {
		return
	}
	if err := m.run(append([]string{"add", "--intent-to-add", "--"}, m.intentFiles...)...); err != nil {
		output.Warn("Failed to re-add intent-to-add files: %v", err)
	}
	m.intentFiles = nil
}

// Restore restores the stashed unstaged changes.
func (m *Manager) Restore() error {
	if !m.stashed {
		return nil
	}
	defer m.restoreIntentToAdd()
	m.stashed = false

	if err := m.applyPatch(); err != nil {
		// Presumably the hooks' fixes conflict with the stashed changes: roll
		// the fixes back and apply again, as upstream does.
		output.Warn("Stashed changes conflicted with hook auto-fixes... Rolling back fixes...")
		if err := m.checkoutIndex(); err != nil {
			return fmt.Errorf("restoring unstaged changes (patch saved at %s): %w", m.patchPath, err)
		}
		if err := m.applyPatch(); err != nil {
			return fmt.Errorf("restoring unstaged changes (patch saved at %s): %w", m.patchPath, err)
		}
	}
	output.Info("Restored changes from %s.", m.patchPath)
	return nil
}

// IsStashed returns whether there are currently stashed changes.
func (m *Manager) IsStashed() bool {
	return m.stashed
}
