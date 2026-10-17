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
)

// extractTarGz is Python's tarfile.open(fileobj=r).extractall(dest) under
// the "data" filter Python 3.14 applies by default: regular files,
// directories and symlinks, none of which may land or point outside dest.
// Modes keep their permission bits, so the executables in a Go distribution
// or rbenv stay executable. Every write goes through an os.Root on dest, so
// not even a chain of in-tree symlinks can carry one outside it.
func extractTarGz(r io.Reader, dest string) (err error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, gz.Close()) }()
	root, err := openArchiveRoot(dest)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	tr := tar.NewReader(gz)
	for {
		hdr, nextErr := tr.Next()
		if nextErr == io.EOF {
			return nil
		}
		if nextErr != nil {
			return nextErr
		}
		if memberErr := extractTarMember(root, tr, hdr); memberErr != nil {
			return memberErr
		}
	}
}

// extractTarMember writes one tar member under root.
func extractTarMember(root *os.Root, r io.Reader, hdr *tar.Header) error {
	name, err := archiveMemberName(hdr.Name)
	if err != nil {
		return err
	}
	switch hdr.Typeflag {
	case tar.TypeDir:
		return root.MkdirAll(name, 0o755)
	case tar.TypeReg:
		return writeArchiveFile(root, name, r, hdr.FileInfo().Mode().Perm())
	case tar.TypeSymlink:
		link := filepath.FromSlash(hdr.Linkname)
		if filepath.IsAbs(link) || !filepath.IsLocal(filepath.Join(filepath.Dir(name), link)) {
			return fmt.Errorf("archive symlink %q points outside the destination", hdr.Name)
		}
		if err := root.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			return err
		}
		return root.Symlink(link, name)
	default:
		// Hard links, devices and fifos appear in neither a Go distribution
		// nor upstream's rbenv tarballs; the data filter would refuse the
		// dangerous ones, so skipping is the safe subset.
		return nil
	}
}

// extractZip is zipfile.ZipFile(path).extractall(dest), with the same
// containment as extractTarGz.
func extractZip(path, dest string) (err error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, zr.Close()) }()
	root, err := openArchiveRoot(dest)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	for _, f := range zr.File {
		if memberErr := extractZipMember(root, f); memberErr != nil {
			return memberErr
		}
	}
	return nil
}

func extractZipMember(root *os.Root, f *zip.File) (err error) {
	name, err := archiveMemberName(f.Name)
	if err != nil {
		return err
	}
	if f.FileInfo().IsDir() {
		return root.MkdirAll(name, 0o755)
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
	return writeArchiveFile(root, name, rc, mode)
}

func openArchiveRoot(dest string) (*os.Root, error) {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return nil, err
	}
	return os.OpenRoot(dest)
}

// archiveMemberName is an archive member's name as a local path; a name
// that is absolute or climbs out with .. is an error.
func archiveMemberName(name string) (string, error) {
	local := filepath.Clean(filepath.FromSlash(name))
	if !filepath.IsLocal(local) {
		return "", fmt.Errorf("archive member %q is outside the destination", name)
	}
	return local, nil
}

func writeArchiveFile(root *os.Root, name string, r io.Reader, mode os.FileMode) error {
	if err := root.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return err
	}
	f, err := root.OpenFile(name, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, r)
	return errors.Join(err, f.Close())
}
