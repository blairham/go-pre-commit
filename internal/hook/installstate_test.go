// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/blairham/go-pre-commit/v4/internal/languages"
)

// The bytes are Python's: json.dumps({'additional_dependencies': deps}) on
// CPython, captured, not derived from this code.
func TestInstallStateV1JSON(t *testing.T) {
	for _, tc := range []struct {
		want string
		deps []string
	}{
		{deps: nil, want: `{"additional_dependencies": []}`},
		{deps: []string{"a", "b@1"}, want: `{"additional_dependencies": ["a", "b@1"]}`},
		{
			deps: []string{"pyyaml>=6", "\u00e9", "\U0001F600", `q"uo\te`, "tab\there", "\x7f", "<&>"},
			want: `{"additional_dependencies": ["pyyaml>=6", "\u00e9", "\ud83d\ude00", "q\"uo\\te", "tab\there", "\u007f", "<&>"]}`,
		},
	} {
		if got := string(installStateV1JSON(tc.deps)); got != tc.want {
			t.Errorf("installStateV1JSON(%q) =\n%s\nwant\n%s", tc.deps, got, tc.want)
		}
		if !installStateV1Matches([]byte(tc.want), tc.deps) {
			t.Errorf("%s does not match the deps it records, %q", tc.want, tc.deps)
		}
	}
}

// An environment counts as installed by upstream's state files -- whichever
// tool wrote them -- or by the file this port wrote before it wrote those.
func TestInstallStateRecognized(t *testing.T) {
	lang := &healthLang{healthy: true}
	languages.Register("statetest", lang)
	deps := []string{"a", "b"}

	for _, tc := range []struct {
		files   map[string]string
		name    string
		install bool
	}{
		{name: "no state", files: nil, install: true},
		{name: ".install_state_v2", files: map[string]string{installStateV2: ""}},
		{name: ".install_state_v1, same deps", files: map[string]string{
			installStateV1: `{"additional_dependencies": ["a", "b"]}`,
		}},
		{name: ".install_state_v1, other deps", install: true, files: map[string]string{
			installStateV1: `{"additional_dependencies": ["a"]}`,
		}},
		{name: ".install_state_v1, extra key", install: true, files: map[string]string{
			installStateV1: `{"additional_dependencies": ["a", "b"], "x": 1}`,
		}},
		{name: ".install_state_v1, null deps", install: true, files: map[string]string{
			installStateV1: `{"additional_dependencies": null}`,
		}},
		{name: "this port's old state", files: map[string]string{legacyInstallState: "KEY"}},
		{name: "this port's old state, other key", install: true, files: map[string]string{
			legacyInstallState: "KEY-other",
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := &Hook{
				ID: "h", Repo: "r", RepoDir: t.TempDir(), Language: "statetest",
				LanguageVersion: "default", AdditionalDependencies: deps,
			}
			env := filepath.Join(h.RepoDir, "health_env-default")
			if err := os.MkdirAll(env, 0o755); err != nil {
				t.Fatal(err)
			}
			for name, body := range tc.files {
				if body == "KEY" {
					body = h.InstallKey()
				}
				if err := os.WriteFile(filepath.Join(env, name), []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			before := lang.installs
			if err := InstallEnvironments(context.Background(), []*Hook{h}, nil); err != nil {
				t.Fatal(err)
			}
			if installed := lang.installs > before; installed != tc.install {
				t.Errorf("installed = %v, want %v", installed, tc.install)
			}
		})
	}
}

// An install leaves upstream's two files, and nothing else of its own.
func TestInstallWritesUpstreamState(t *testing.T) {
	languages.Register("statetest", &healthLang{healthy: true})
	h := &Hook{
		ID: "h", Repo: "r", RepoDir: t.TempDir(), Language: "statetest",
		LanguageVersion: "default", AdditionalDependencies: []string{"a", "b"},
	}
	if err := InstallEnvironments(context.Background(), []*Hook{h}, nil); err != nil {
		t.Fatal(err)
	}
	env := filepath.Join(h.RepoDir, "health_env-default")
	entries, err := os.ReadDir(env)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(env, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		got[e.Name()] = string(data)
	}
	want := map[string]string{
		".install_state_v1": `{"additional_dependencies": ["a", "b"]}`,
		".install_state_v2": "",
	}
	if len(got) != len(want) || got[installStateV1] != want[installStateV1] ||
		got[installStateV2] != want[installStateV2] {
		t.Errorf("env holds %q, want %q", got, want)
	}
}
