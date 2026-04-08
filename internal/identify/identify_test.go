package identify

import (
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// TagsForFile – "file" tag is always present
// ---------------------------------------------------------------------------

func TestTagsForFileAlwaysIncludesFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "anything.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	tags := TagsForFile(path)
	if !tags["file"] {
		t.Error("TagsForFile should always include 'file'")
	}
}

func TestTagsForFileBinaryStillHasFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "data.bin")
	// Write a file with null bytes to be detected as binary.
	if err := os.WriteFile(path, []byte("hello\x00world"), 0o644); err != nil {
		t.Fatal(err)
	}
	tags := TagsForFile(path)
	if !tags["file"] {
		t.Error("binary file should still have 'file' tag")
	}
	if !tags["binary"] {
		t.Error("file with null bytes should have 'binary' tag")
	}
}

// ---------------------------------------------------------------------------
// TagsForFile – extension-based tags
// ---------------------------------------------------------------------------

func TestTagsForFilePython(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "script.py")
	if err := os.WriteFile(path, []byte("print('hi')"), 0o644); err != nil {
		t.Fatal(err)
	}
	tags := TagsForFile(path)
	if !tags["python"] {
		t.Error("script.py should have 'python' tag")
	}
}

func TestTagsForFileJavaScript(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "app.js")
	if err := os.WriteFile(path, []byte("console.log()"), 0o644); err != nil {
		t.Fatal(err)
	}
	tags := TagsForFile(path)
	if !tags["javascript"] {
		t.Error("app.js should have 'javascript' tag")
	}
}

func TestTagsForFileGo(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "main.go")
	if err := os.WriteFile(path, []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	tags := TagsForFile(path)
	if !tags["go"] {
		t.Error("main.go should have 'go' tag")
	}
}

func TestTagsForFileRust(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "lib.rs")
	if err := os.WriteFile(path, []byte("fn main() {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	tags := TagsForFile(path)
	if !tags["rust"] {
		t.Error("lib.rs should have 'rust' tag")
	}
}

func TestTagsForFileYAML(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yml")
	if err := os.WriteFile(path, []byte("key: value"), 0o644); err != nil {
		t.Fatal(err)
	}
	tags := TagsForFile(path)
	if !tags["yaml"] {
		t.Error("config.yml should have 'yaml' tag")
	}
}

func TestTagsForFileTypeScript(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "index.ts")
	if err := os.WriteFile(path, []byte("const x = 1;"), 0o644); err != nil {
		t.Fatal(err)
	}
	tags := TagsForFile(path)
	if !tags["typescript"] {
		t.Error("index.ts should have 'typescript' tag")
	}
}

// ---------------------------------------------------------------------------
// TagsForFile – "text" tag for non-binary files
// ---------------------------------------------------------------------------

func TestTagsForFileTextForNonBinary(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "readme.txt")
	if err := os.WriteFile(path, []byte("just text"), 0o644); err != nil {
		t.Fatal(err)
	}
	tags := TagsForFile(path)
	if !tags["text"] {
		t.Error("non-binary file should have 'text' tag")
	}
	if tags["binary"] {
		t.Error("non-binary file should not have 'binary' tag")
	}
}

func TestTagsForFileBinaryHasNoText(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "image.bin")
	if err := os.WriteFile(path, []byte("\x00\x01\x02\x03"), 0o644); err != nil {
		t.Fatal(err)
	}
	tags := TagsForFile(path)
	if tags["text"] {
		t.Error("binary file should not have 'text' tag")
	}
}

// ---------------------------------------------------------------------------
// TagsForFile – filename-based tags
// ---------------------------------------------------------------------------

func TestTagsForFileMakefile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "Makefile")
	if err := os.WriteFile(path, []byte("all:\n\techo hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	tags := TagsForFile(path)
	if !tags["makefile"] {
		t.Error("Makefile should have 'makefile' tag")
	}
}

func TestTagsForFileDockerfile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "Dockerfile")
	if err := os.WriteFile(path, []byte("FROM alpine"), 0o644); err != nil {
		t.Fatal(err)
	}
	tags := TagsForFile(path)
	if !tags["dockerfile"] {
		t.Error("Dockerfile should have 'dockerfile' tag")
	}
}

// ---------------------------------------------------------------------------
// TagsForFile – shebang detection
// ---------------------------------------------------------------------------

func TestTagsForFileShebangPython(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "myscript")
	if err := os.WriteFile(path, []byte("#!/usr/bin/env python3\nprint('hi')\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	tags := TagsForFile(path)
	if !tags["python"] {
		t.Error("script with python shebang should have 'python' tag")
	}
}

func TestTagsForFileShebangBash(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "run.sh")
	if err := os.WriteFile(path, []byte("#!/bin/bash\necho hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	tags := TagsForFile(path)
	if !tags["bash"] {
		t.Error("script with bash shebang should have 'bash' tag")
	}
	if !tags["shell"] {
		t.Error("script with bash shebang should have 'shell' tag")
	}
}

// ---------------------------------------------------------------------------
// MatchesTypes – AND logic (types)
// ---------------------------------------------------------------------------

func TestMatchesTypesANDAllMatch(t *testing.T) {
	tags := map[string]bool{"file": true, "text": true, "python": true}
	if !MatchesTypes(tags, []string{"file", "python"}, nil, nil) {
		t.Error("all types present, should match")
	}
}

func TestMatchesTypesANDOneMissing(t *testing.T) {
	tags := map[string]bool{"file": true, "text": true}
	if MatchesTypes(tags, []string{"file", "python"}, nil, nil) {
		t.Error("'python' missing, should not match")
	}
}

// ---------------------------------------------------------------------------
// MatchesTypes – OR logic (types_or)
// ---------------------------------------------------------------------------

func TestMatchesTypesOROnePresent(t *testing.T) {
	tags := map[string]bool{"file": true, "text": true, "python": true}
	if !MatchesTypes(tags, nil, []string{"python", "ruby"}, nil) {
		t.Error("'python' present, should match OR")
	}
}

func TestMatchesTypesORNonePresent(t *testing.T) {
	tags := map[string]bool{"file": true, "text": true, "go": true}
	if MatchesTypes(tags, nil, []string{"python", "ruby"}, nil) {
		t.Error("neither 'python' nor 'ruby' present, should not match OR")
	}
}

// ---------------------------------------------------------------------------
// MatchesTypes – exclude logic
// ---------------------------------------------------------------------------

func TestMatchesTypesExcludeMatch(t *testing.T) {
	tags := map[string]bool{"file": true, "text": true, "python": true}
	if MatchesTypes(tags, nil, nil, []string{"python"}) {
		t.Error("'python' excluded and present, should not match")
	}
}

func TestMatchesTypesExcludeNoMatch(t *testing.T) {
	tags := map[string]bool{"file": true, "text": true, "go": true}
	if !MatchesTypes(tags, nil, nil, []string{"python"}) {
		t.Error("'python' excluded but not present, should match")
	}
}

// ---------------------------------------------------------------------------
// MatchesTypes – combined filters
// ---------------------------------------------------------------------------

func TestMatchesTypesCombinedAllPass(t *testing.T) {
	tags := map[string]bool{"file": true, "text": true, "python": true}
	if !MatchesTypes(tags, []string{"file"}, []string{"python", "ruby"}, []string{"binary"}) {
		t.Error("all conditions satisfied, should match")
	}
}

func TestMatchesTypesCombinedExcludeFails(t *testing.T) {
	tags := map[string]bool{"file": true, "text": true, "python": true}
	if MatchesTypes(tags, []string{"file"}, []string{"python"}, []string{"text"}) {
		t.Error("'text' excluded and present, should not match")
	}
}

// ---------------------------------------------------------------------------
// MatchesTypes – edge cases: empty types, empty typesOr
// ---------------------------------------------------------------------------

func TestMatchesTypesAllEmpty(t *testing.T) {
	tags := map[string]bool{"file": true, "text": true}
	if !MatchesTypes(tags, nil, nil, nil) {
		t.Error("no filters at all, should match")
	}
}

func TestMatchesTypesEmptySlices(t *testing.T) {
	tags := map[string]bool{"file": true, "text": true}
	if !MatchesTypes(tags, []string{}, []string{}, []string{}) {
		t.Error("empty slices should behave like nil, should match")
	}
}

func TestMatchesTypesEmptyTypesOrMeansNoORCheck(t *testing.T) {
	// When typesOr is empty, the OR check is skipped entirely.
	tags := map[string]bool{"file": true}
	if !MatchesTypes(tags, []string{"file"}, []string{}, nil) {
		t.Error("empty typesOr should not block match")
	}
}

func TestMatchesTypesEmptyTags(t *testing.T) {
	tags := map[string]bool{}
	if !MatchesTypes(tags, nil, nil, nil) {
		t.Error("empty tags with no filters should match")
	}
	if MatchesTypes(tags, []string{"file"}, nil, nil) {
		t.Error("empty tags should not satisfy 'file' type requirement")
	}
}
