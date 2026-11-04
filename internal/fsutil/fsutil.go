// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

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

	// Grant write+execute on every directory in the tree, then retry. What
	// the walk cannot read or stat is passed over, not reported: the retry
	// below reports the real failure, and a partially-walkable tree is
	// exactly the case to push on.
	_ = filepath.WalkDir(path, func(p string, d fs.DirEntry, walkErr error) error {
		info, ok := reachableDir(d, walkErr)
		if !ok {
			return nil
		}
		if mode := info.Mode().Perm(); mode&0o300 != 0o300 {
			_ = os.Chmod(p, mode|0o300)
		}
		return nil
	})

	return os.RemoveAll(path)
}

// reachableDir is the FileInfo of the directory a WalkDir callback was handed,
// with ok false for anything else: a file, or an entry the walk could not
// read (walkErr) or stat.
func reachableDir(d fs.DirEntry, walkErr error) (fs.FileInfo, bool) {
	if walkErr != nil || !d.IsDir() {
		return nil, false
	}
	info, err := d.Info()
	return info, err == nil
}
