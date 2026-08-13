// Package fsutil provides filesystem helpers that tolerate the read-only trees
// language backends leave behind.
package fsutil

import (
	"io/fs"
	"os"
	"path/filepath"
)

// RemoveAll removes path and everything under it, retrying with write
// permission restored if the first attempt is blocked.
//
// Why this exists: `go install` populates a module cache whose files are mode
// 0444 inside directories mode 0555. On POSIX, unlinking an entry needs write
// and execute on its *parent directory* — not on the file — so a plain
// os.RemoveAll fails with EACCES partway through and leaves the tree in place.
// Other backends (cargo, some npm caches) do the same thing.
//
// A caller that ignores that error while updating its own bookkeeping strands
// the directory: the store derives cache paths from repo+rev, so the next
// clone targets the leftover directory, finds it non-empty, and fails there
// forever. Always use this instead of os.RemoveAll for anything that may hold
// an installed language environment.
func RemoveAll(path string) error {
	err := os.RemoveAll(path)
	if err == nil {
		return nil
	}

	// Grant write+execute on every directory in the tree, then retry. Walk
	// errors are deliberately swallowed — the retry below reports the real
	// failure, and a partially-walkable tree is exactly the case to push on.
	_ = filepath.WalkDir(path, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		info, statErr := d.Info()
		if statErr != nil {
			return nil
		}
		if mode := info.Mode().Perm(); mode&0o300 != 0o300 {
			_ = os.Chmod(p, mode|0o300)
		}
		return nil
	})

	return os.RemoveAll(path)
}
