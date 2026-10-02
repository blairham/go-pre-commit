// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package identify

import (
	"os"
	"path/filepath"
	"testing"
)

// FuzzTagsForFile feeds arbitrary file contents through binary detection and
// shebang parsing, which read whatever is in the repository being checked.
// Beyond not panicking, a regular file is always tagged "file" and is exactly
// one of "text" or "binary": the default `types: [file]` filter and every
// `types: [text]` hook depend on that.
func FuzzTagsForFile(f *testing.F) {
	for _, seed := range []string{
		"",
		"#!",
		"#!/usr/bin/env python3\nprint(1)\n",
		"#!/usr/bin/env \n",
		"#!/bin/sh",
		"#! /usr/bin/env -S node --flag\n",
		"plain text\n",
		"\x00\x01\x02",
	} {
		f.Add([]byte(seed))
	}
	dir := f.TempDir()
	f.Fuzz(func(t *testing.T, data []byte) {
		path := filepath.Join(dir, "script")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		tags := TagsForFile(path)
		if !tags["file"] {
			t.Fatalf("regular file not tagged file: %v", tags)
		}
		if tags["text"] == tags["binary"] {
			t.Fatalf("want exactly one of text/binary, got %v", tags)
		}
	})
}
