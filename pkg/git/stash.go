package git

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ErrNoUnstagedChanges is returned when there are no unstaged changes to stash.
var ErrNoUnstagedChanges = errors.New("no unstaged changes to stash")

// StashInfo holds information about a stashed set of changes
type StashInfo struct {
	PatchFile string
	Files     []string
}

// HasUnstagedChanges checks if there are any unstaged changes to tracked files
func (r *Repository) HasUnstagedChanges() (bool, error) {
	// Check for modified files (excluding untracked files)
	cmd := exec.Command("git", "diff", "--quiet", "--exit-code")
	cmd.Dir = r.Root
	err := cmd.Run()
	if err != nil {
		// Exit code 1 means there are differences
		var exitError *exec.ExitError
		if errors.As(err, &exitError) && exitError.ExitCode() == 1 {
			return true, nil
		}
		return false, fmt.Errorf("failed to check for unstaged changes: %w", err)
	}

	return false, nil
}

// GetUnstagedChangesFiles returns the list of files with unstaged changes
func (r *Repository) GetUnstagedChangesFiles() ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only")
	cmd.Dir = r.Root
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get unstaged files: %w", err)
	}

	var files []string
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}

	return files, nil
}

// StashUnstagedChanges creates a patch file of unstaged changes and resets working directory to staged content
func (r *Repository) StashUnstagedChanges(cacheDir string) (*StashInfo, error) {
	// Check if there are unstaged changes
	hasChanges, err := r.HasUnstagedChanges()
	if err != nil {
		return nil, err
	}

	if !hasChanges {
		return nil, ErrNoUnstagedChanges
	}

	// Get list of files with unstaged changes
	files, err := r.GetUnstagedChangesFiles()
	if err != nil {
		return nil, err
	}

	// Create patch of unstaged changes
	patchFile, err := r.createPatchFile(cacheDir)
	if err != nil {
		return nil, err
	}

	// Create the patch (diff between index and working tree)
	cmd := exec.Command("git", "diff", "--binary")
	cmd.Dir = r.Root
	patchContent, err := cmd.Output()
	if err != nil {
		if rmErr := os.Remove(patchFile); rmErr != nil {
			fmt.Printf("⚠️  Warning: failed to remove patch file: %v\n", rmErr)
		}
		return nil, fmt.Errorf("failed to create patch: %w", err)
	}

	// Write patch to file
	if err := os.WriteFile(patchFile, patchContent, 0o600); err != nil {
		if rmErr := os.Remove(patchFile); rmErr != nil {
			fmt.Printf("⚠️  Warning: failed to remove patch file: %v\n", rmErr)
		}
		return nil, fmt.Errorf("failed to write patch file: %w", err)
	}

	// Reset working directory to match staged content exactly
	// For tracked files, checkout the staged version
	for _, file := range files {
		if err := r.checkoutFileFromHEAD(file); err != nil {
			// If file is newly added (staged but not in HEAD), use staged version
			if err := r.writeFileFromStaged(file); err != nil {
				// Restore from patch if we fail
				if restoreErr := r.RestoreFromStash(&StashInfo{PatchFile: patchFile, Files: files}); restoreErr != nil {
					fmt.Printf("⚠️  Warning: failed to restore from stash: %v\n", restoreErr)
				}
				return nil, fmt.Errorf("failed to write staged content for %s: %w", file, err)
			}
		}
	}

	return &StashInfo{
		PatchFile: patchFile,
		Files:     files,
	}, nil
}

// checkoutFileFromHEAD checks out a file from HEAD
func (r *Repository) checkoutFileFromHEAD(file string) error {
	cmd := exec.Command("git", "checkout", "HEAD", "--", file)
	cmd.Dir = r.Root
	return cmd.Run()
}

// writeFileFromStaged writes the staged version of a file to the working directory
func (r *Repository) writeFileFromStaged(file string) error {
	cmd := exec.Command("git", "show", ":"+file)
	cmd.Dir = r.Root
	content, err := cmd.Output()
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(r.Root, file), content, 0o600)
}

// CanApplyStash checks if a stash can be applied without conflicts
func (r *Repository) CanApplyStash(stash *StashInfo) (bool, error) {
	if stash == nil {
		return true, nil
	}

	// Try to apply the patch in dry-run mode
	cmd := exec.Command("git", "apply", "--check", stash.PatchFile)
	cmd.Dir = r.Root
	err := cmd.Run()
	if err != nil {
		// If apply --check fails, there are conflicts - this is expected behavior
		return false, nil //nolint:nilerr // expected behavior when stash can't be applied
	}

	return true, nil
}

// RestoreFromStash applies the stashed changes back to the working directory
func (r *Repository) RestoreFromStash(stash *StashInfo) error {
	if stash == nil {
		return nil // Nothing to restore
	}

	// Apply the patch
	cmd := exec.Command("git", "apply", stash.PatchFile)
	cmd.Dir = r.Root
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to restore stashed changes: %w", err)
	}

	// Print restore message to match Python pre-commit behavior
	fmt.Printf("[INFO] Restored changes from %s.\n", stash.PatchFile)

	// Clean up patch file
	if err := os.Remove(stash.PatchFile); err != nil {
		fmt.Printf("⚠️  Warning: failed to remove patch file: %v\n", err)
	}

	return nil
}

// ResetToStaged resets working directory to match staged content exactly
func (r *Repository) ResetToStaged() error {
	// Reset tracked files to staged content
	cmd := exec.Command("git", "checkout-index", "-a", "-f")
	cmd.Dir = r.Root
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to reset to staged content: %w", err)
	}

	return nil
}

// CleanupStash removes the stash patch file
func (r *Repository) CleanupStash(stash *StashInfo) {
	if stash != nil {
		if err := os.Remove(stash.PatchFile); err != nil {
			fmt.Printf("⚠️  Warning: failed to remove patch file: %v\n", err)
		}
	}
}

// createPatchFile generates a unique patch file name in the cache directory
func (r *Repository) createPatchFile(cacheDir string) (string, error) {
	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(cacheDir, 0o750); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Generate unique filename
	timestamp := time.Now().Unix()
	randomBytes := make([]byte, 4)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	filename := fmt.Sprintf("patch%d-%x", timestamp, randomBytes)
	return filepath.Join(cacheDir, filename), nil
}
