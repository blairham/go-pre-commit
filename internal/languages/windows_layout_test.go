// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package languages

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Python pre-commit's python.bin_dir: Scripts on win32, bin elsewhere.
func TestVenvBinDirFor(t *testing.T) {
	env := filepath.Join("cache", "py_env-default")
	for goos, want := range map[string]string{
		"windows": filepath.Join(env, "Scripts"),
		"linux":   filepath.Join(env, "bin"),
		"darwin":  filepath.Join(env, "bin"),
	} {
		if got := venvBinDirFor(goos, env); got != want {
			t.Errorf("venvBinDirFor(%q) = %q, want %q", goos, got, want)
		}
	}
}

// Python pre-commit's node.get_env_patch: on win32 the npm prefix is the bin
// dir itself and NODE_PATH is under Scripts; elsewhere the prefix is the env
// and NODE_PATH is under lib.
func TestNodeEnvVarsFor(t *testing.T) {
	env := filepath.Join("cache", "node_env-default")
	for _, tc := range []struct {
		goos, prefix, nodePath, path string
	}{
		{"windows", filepath.Join(env, "Scripts"), filepath.Join(env, "Scripts", "node_modules"), filepath.Join(env, "Scripts")},
		{"linux", env, filepath.Join(env, "lib", "node_modules"), filepath.Join(env, "bin")},
	} {
		vars := nodeEnvVarsFor(tc.goos, env)
		for _, want := range []string{
			"NODE_VIRTUAL_ENV=" + env,
			"NPM_CONFIG_PREFIX=" + tc.prefix,
			"npm_config_prefix=" + tc.prefix,
			"NODE_PATH=" + tc.nodePath,
		} {
			if !slices.Contains(vars, want) {
				t.Errorf("%s: missing %q in %q", tc.goos, want, vars)
			}
		}
		if !slices.ContainsFunc(vars, func(v string) bool { return strings.HasPrefix(v, "PATH="+tc.path) }) {
			t.Errorf("%s: PATH does not start with %q: %q", tc.goos, tc.path, vars)
		}
	}
}
