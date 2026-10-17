// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package languages

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type tarMember struct {
	name, link, body string
	typ              byte
	mode             int64
}

func tarGz(t *testing.T, members ...tarMember) *bytes.Reader {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, m := range members {
		hdr := &tar.Header{Name: m.name, Linkname: m.link, Typeflag: m.typ, Mode: m.mode, Size: int64(len(m.body))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(m.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(buf.Bytes())
}

func TestExtractTarGz(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks and exec bits")
	}
	dest := t.TempDir()
	err := extractTarGz(tarGz(t,
		tarMember{name: "r/", typ: tar.TypeDir, mode: 0o755},
		tarMember{name: "r/libexec/tool", typ: tar.TypeReg, mode: 0o755, body: "#!/bin/sh\n"},
		tarMember{name: "r/bin/tool", typ: tar.TypeSymlink, link: "../libexec/tool"},
	), dest)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dest, "r", "bin", "tool"))
	if err != nil {
		t.Fatalf("symlink does not resolve: %v", err)
	}
	if info.Mode().Perm()&0o100 == 0 {
		t.Errorf("mode %v lost the exec bit", info.Mode())
	}
}

func TestExtractTarGzRefusesEscapes(t *testing.T) {
	for name, m := range map[string]tarMember{
		"dotdot":        {name: "../evil", typ: tar.TypeReg, mode: 0o644},
		"absolute":      {name: "/evil", typ: tar.TypeReg, mode: 0o644},
		"link-out":      {name: "a/l", typ: tar.TypeSymlink, link: "../../evil"},
		"link-absolute": {name: "a/l", typ: tar.TypeSymlink, link: "/etc/passwd"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := extractTarGz(tarGz(t, m), t.TempDir()); err == nil {
				t.Error("an archive member outside the destination must be refused")
			}
		})
	}
}

func TestExtractZip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.zip")
	write := func(names ...string) {
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		zw := zip.NewWriter(f)
		for _, n := range names {
			w, err := zw.Create(n)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = w.Write([]byte("x"))
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	write("go/bin/go.exe")
	dest := t.TempDir()
	if err := extractZip(path, dest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "go", "bin", "go.exe")); err != nil {
		t.Error(err)
	}
	write("../evil")
	if err := extractZip(path, t.TempDir()); err == nil {
		t.Error("a zip member outside the destination must be refused")
	}
}

// Each symlink is in-tree on its own, but through the first the second
// points at the destination's parent: a write through it must not land there.
func TestExtractTarGzSymlinkChain(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks")
	}
	parent := t.TempDir()
	dest := filepath.Join(parent, "dest")
	err := extractTarGz(tarGz(t,
		tarMember{name: "a", typ: tar.TypeSymlink, link: "."},
		tarMember{name: "a/b", typ: tar.TypeSymlink, link: ".."},
		tarMember{name: "a/b/escaped", typ: tar.TypeReg, mode: 0o644, body: "x"},
	), dest)
	if err == nil {
		t.Error("a write through a chain of symlinks out of the destination must fail")
	}
	if _, statErr := os.Stat(filepath.Join(parent, "escaped")); !os.IsNotExist(statErr) {
		t.Errorf("file escaped the destination (stat: %v)", statErr)
	}
}
