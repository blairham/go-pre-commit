// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package languages

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// extractTarGz is Python's tarfile.open(fileobj=r).extractall(dest) under
// the "data" filter Python 3.14 applies by default: regular files,
// directories and symlinks, none of which may land or point outside dest.
// Modes keep their permission bits, so the executables in a Go distribution
// or rbenv stay executable.
func extractTarGz(r io.Reader, dest string) (err error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, gz.Close()) }()
	tr := tar.NewReader(gz)
	for {
		hdr, nextErr := tr.Next()
		if nextErr == io.EOF {
			return nil
		}
		if nextErr != nil {
			return nextErr
		}
		if memberErr := extractTarMember(tr, hdr, dest); memberErr != nil {
			return memberErr
		}
	}
}

// extractTarMember writes one tar member under dest.
func extractTarMember(r io.Reader, hdr *tar.Header, dest string) error {
	target, err := archiveTarget(dest, hdr.Name)
	if err != nil {
		return err
	}
	switch hdr.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(target, 0o755)
	case tar.TypeReg:
		return writeArchiveFile(target, r, hdr.FileInfo().Mode().Perm())
	case tar.TypeSymlink:
		if filepath.IsAbs(hdr.Linkname) {
			return fmt.Errorf("archive symlink %s points to an absolute path", hdr.Name)
		}
		if _, err := archiveTarget(dest, filepath.Join(filepath.Dir(hdr.Name), hdr.Linkname)); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.Symlink(hdr.Linkname, target)
	default:
		// Hard links, devices and fifos appear in neither a Go distribution
		// nor upstream's rbenv tarballs; the data filter would refuse the
		// dangerous ones, so skipping is the safe subset.
		return nil
	}
}

// extractZip is zipfile.ZipFile(path).extractall(dest), with the same
// containment check as extractTarGz.
func extractZip(path, dest string) (err error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, zr.Close()) }()
	for _, f := range zr.File {
		if memberErr := extractZipMember(f, dest); memberErr != nil {
			return memberErr
		}
	}
	return nil
}

func extractZipMember(f *zip.File, dest string) (err error) {
	target, err := archiveTarget(dest, f.Name)
	if err != nil {
		return err
	}
	if f.FileInfo().IsDir() {
		return os.MkdirAll(target, 0o755)
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, rc.Close()) }()
	mode := f.Mode().Perm()
	if mode == 0 {
		mode = 0o644
	}
	return writeArchiveFile(target, rc, mode)
}

// archiveTarget is where an archive member named name lands under dest; a
// name that would escape dest is an error.
func archiveTarget(dest, name string) (string, error) {
	target := filepath.Join(dest, filepath.FromSlash(name))
	rel, err := filepath.Rel(dest, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(name) {
		return "", fmt.Errorf("archive member %q is outside the destination", name)
	}
	return target, nil
}

func writeArchiveFile(target string, r io.Reader, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, r)
	return errors.Join(err, f.Close())
}
