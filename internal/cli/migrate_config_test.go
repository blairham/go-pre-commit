// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blairham/go-pre-commit/v4/internal/fatal"
)

// TestMigrateConfigMatchesUpstream runs migrateConfig over configs whose
// migrated text and message were captured from upstream's migrate_config
// (pre-commit 4.7.0); want is that file verbatim.
func TestMigrateConfigMatchesUpstream(t *testing.T) {
	cases := []struct {
		name, in, want, msg string
	}{
		{
			name: "clean",
			in:   "repos:\n-   repo: https://x\n    rev: v1\n    hooks:\n    -   id: a\n",
			want: "repos:\n-   repo: https://x\n    rev: v1\n    hooks:\n    -   id: a\n",
			msg:  "Configuration is already migrated.\n",
		},
		{
			name: "crlf",
			in:   "repos:\r\n-   repo: https://x\r\n    sha: v1\r\n    hooks:\r\n    -   id: a\r\n",
			want: "repos:\n-   repo: https://x\n    rev: v1\n    hooks:\n    -   id: a\n",
			msg:  "Configuration has been migrated.\n",
		},
		{
			name: "crlf_clean",
			in:   "repos:\r\n-   repo: https://x\r\n    rev: v1\r\n    hooks:\r\n    -   id: a\r\n",
			want: "repos:\r\n-   repo: https://x\r\n    rev: v1\r\n    hooks:\r\n    -   id: a\r\n",
			msg:  "Configuration is already migrated.\n",
		},
		{name: "empty", in: "", want: "", msg: "Configuration is already migrated.\n"},
		{
			name: "list_flow",
			in:   "# header\n\n---\n-   repo: https://x\n    sha: v1\n    hooks:\n    -   id: a\n",
			want: "# header\n\n---\nrepos:\n-   repo: https://x\n    rev: v1\n    hooks:\n    -   id: a\n",
			msg:  "Configuration has been migrated.\n",
		},
		{
			name: "list_flowstyle",
			in:   "[{repo: https://x, sha: v1, hooks: [{id: a}]}]\n",
			want: "repos:\n    [{repo: https://x, rev: v1, hooks: [{id: a}]}]\n",
			msg:  "Configuration has been migrated.\n",
		},
		{
			name: "list_flowstyle_header",
			in:   "# c\n[{repo: https://x, sha: v1, hooks: [{id: a}]}]\n",
			want: "# c\nrepos:\n    [{repo: https://x, rev: v1, hooks: [{id: a}]}]\n",
			msg:  "Configuration has been migrated.\n",
		},
		{
			name: "list_indent",
			in:   "-   repo: local\n    hooks:\n    -   id: a\n        name: a\n        entry: echo\n        language: python_venv\n",
			want: "repos:\n-   repo: local\n    hooks:\n    -   id: a\n        name: a\n        entry: echo\n        language: python\n",
			msg:  "Configuration has been migrated.\n",
		},
		{
			name: "list_noindent",
			in:   "- repo: https://x\n  rev: v1\n  hooks:\n  - id: a\n",
			want: "repos:\n- repo: https://x\n  rev: v1\n  hooks:\n  - id: a\n",
			msg:  "Configuration has been migrated.\n",
		},
		{
			name: "sha",
			in:   "repos:\n-   repo: https://x\n    sha: v1\n    hooks:\n    -   id: a\n-   repo: https://y\n    'sha': v2\n    hooks: []\n",
			want: "repos:\n-   repo: https://x\n    rev: v1\n    hooks:\n    -   id: a\n-   repo: https://y\n    'rev': v2\n    hooks: []\n",
			msg:  "Configuration has been migrated.\n",
		},
		{
			name: "stages",
			in:   "default_stages: [commit, 'push', \"merge-commit\", manual]\nrepos:\n- repo: local\n  hooks:\n  - id: a\n    name: a\n    entry: echo\n    language: \"python_venv\"\n    stages:\n    - commit   # c\n    - push\n    - pre-commit\n",
			want: "default_stages: [pre-commit, 'pre-push', \"pre-merge-commit\", manual]\nrepos:\n- repo: local\n  hooks:\n  - id: a\n    name: a\n    entry: echo\n    language: \"python\"\n    stages:\n    - pre-commit   # c\n    - pre-push\n    - pre-commit\n",
			msg:  "Configuration has been migrated.\n",
		},
		{
			name: "tagged",
			in:   "default_stages: [!!str commit, &a push]\nrepos: []\n",
			want: "default_stages: [pre-commit, pre-push]\nrepos: []\n",
			msg:  "Configuration has been migrated.\n",
		},
		{
			name: "unicode",
			in:   "# café\nrepos:\n- repo: local  # éé\n  hooks:\n  - {id: é, name: é, entry: é, language: python_venv, stages: [commit]}\n",
			want: "# café\nrepos:\n- repo: local  # éé\n  hooks:\n  - {id: é, name: é, entry: é, language: python, stages: [pre-commit]}\n",
			msg:  "Configuration has been migrated.\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "c.yaml")
			if err := os.WriteFile(path, []byte(tc.in), 0o644); err != nil {
				t.Fatal(err)
			}
			var msg strings.Builder
			if err := migrateConfig(path, false, func(s string) { msg.WriteString(s) }); err != nil {
				t.Fatal(err)
			}
			got, _ := os.ReadFile(path)
			if string(got) != tc.want {
				t.Errorf("config\n got: %q\nwant: %q", got, tc.want)
			}
			if msg.String() != tc.msg {
				t.Errorf("message = %q, want %q", msg.String(), tc.msg)
			}
		})
	}
}

// quiet, as autoupdate runs it, says nothing when there is nothing to do.
func TestMigrateConfigQuiet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.yaml")
	if err := os.WriteFile(path, []byte("repos: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var msg strings.Builder
	if err := migrateConfig(path, true, func(s string) { msg.WriteString(s) }); err != nil {
		t.Fatal(err)
	}
	if msg.Len() != 0 {
		t.Errorf("quiet migrate printed %q", msg.String())
	}
}

// A config that is not YAML is upstream's InvalidConfigError, framed by
// cfgv's file context, and the file is left alone.
func TestMigrateConfigInvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.yaml")
	const in = "repos: [\n"
	if err := os.WriteFile(path, []byte(in), 0o644); err != nil {
		t.Fatal(err)
	}
	err := migrateConfig(path, false, func(s string) { fmt.Print(s) })
	var ice *fatal.Error
	if !errors.As(err, &ice) || ice.Type != "InvalidConfigError" {
		t.Fatalf("err = %v, want an InvalidConfigError", err)
	}
	if !strings.HasPrefix(ice.Error(), "\n==> File "+path+"\n=====> ") {
		t.Errorf("message = %q", ice.Error())
	}
	if got, _ := os.ReadFile(path); string(got) != in {
		t.Errorf("config rewritten to %q", got)
	}
}

// Upstream rewrites a stage reached through an alias once per path to it,
// splicing the same replacement in twice ("[pre-commitpre-commit]"); the
// anchored scalar is one span of source and is rewritten once here.
func TestMigrateConfigAliasRewrittenOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.yaml")
	const in = "default_stages: &s [commit]\nrepos:\n- repo: local\n  hooks:\n  - id: a\n    name: a\n    entry: echo\n    language: system\n    stages: *s\n"
	if err := os.WriteFile(path, []byte(in), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := migrateConfig(path, true, func(string) {}); err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(in, "[commit]", "[pre-commit]", 1)
	if got, _ := os.ReadFile(path); string(got) != want {
		t.Errorf("config = %q, want %q", got, want)
	}
}

// Into a pipe, print() output waits for the process to exit.
func TestPyStdoutBuffersIntoAPipe(t *testing.T) {
	stdout, _ := captureOutput(t, func() {
		p := newPyStdout()
		p.Print("printed\n")
		fmt.Print("written\n")
		p.Flush()
	})
	if want := "written\nprinted\n"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}
