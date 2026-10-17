// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package languages

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// upstream's _entry_validate and _cmd_from_hook.
func TestRCmdFromHook(t *testing.T) {
	opts := strings.Join(rRenvActivatedOpts, " ")
	for _, c := range []struct {
		entry   string
		want    string
		wantErr string
		local   bool
	}{
		{entry: "Rscript hook.R", want: "Rscript " + opts + " /p/hook.R a"},
		{entry: "Rscript hook.R", local: true, want: "Rscript " + opts + " hook.R a"},
		{entry: "Rscript /abs/hook.R", want: "Rscript " + opts + " /abs/hook.R a"},
		{entry: "Rscript -e 'cat(1)'", want: "Rscript " + opts + " -e cat(1) a"},
		{entry: "Rscript -e 'cat(1)'", local: true, want: "Rscript " + opts + " -e cat(1) a"},
		{entry: "R hook.R", wantErr: "entry must start with `Rscript`."},
		{entry: "Rscript -e a b", wantErr: "You can supply at most one expression."},
		{entry: "Rscript hook.R extra", wantErr: "The only valid syntax is `Rscript -e {expr}`or `Rscript path/to/hook/script`"},
	} {
		if runtime.GOOS == "windows" && strings.Contains(c.want, "/") {
			continue
		}
		argv, err := rCmdFromHook("/p", c.entry, []string{"a"}, c.local)
		if c.wantErr != "" {
			if err == nil || err.Error() != c.wantErr {
				t.Errorf("%q: err = %v, want %q", c.entry, err, c.wantErr)
			}
			continue
		}
		if err != nil || strings.Join(argv, " ") != c.want {
			t.Errorf("%q (local=%v): %q, %v; want %q", c.entry, c.local, strings.Join(argv, " "), err, c.want)
		}
	}
}

// fakeRscript puts an Rscript on PATH that records its argv, the env it saw
// and the script it was handed, and answers the two version queries with
// $FAKE_R_INSTALLED and $FAKE_R_CURRENT.
func fakeRscript(t *testing.T) (record string) {
	t.Helper()
	dir := t.TempDir()
	record = filepath.Join(t.TempDir(), "rscript.log")
	script := `#!/bin/sh
echo "argv: $*" >> ` + record + `
echo "R_PROFILE_USER=$R_PROFILE_USER RENV_PROJECT=${RENV_PROJECT-unset} pwd=$(pwd)" >> ` + record + `
for a in "$@"; do
  case "$a" in
    *script.R)
      cat "$a" >> ` + record + `
      if grep -q 'renv::settings\$r.version())' "$a"; then printf '%s\n\n' "$FAKE_R_INSTALLED"; fi
      if grep -q 'cat(as.character(getRversion()))' "$a"; then printf '%s\n' "$FAKE_R_CURRENT"; fi
      ;;
  esac
done
exit 0
`
	if err := os.WriteFile(filepath.Join(dir, "Rscript"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("RENV_PROJECT", "/should/be/unset")
	os.Unsetenv("R_HOME")
	return record
}

func TestRRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake Rscript is a sh script")
	}
	record := fakeRscript(t)
	prefix := t.TempDir()
	code, out, err := (&R{}).Run(
		context.Background(),
		prefix,
		t.TempDir(),
		"Rscript hook.R",
		[]string{"--x"},
		[]string{"f1"},
		"default",
	)
	if err != nil || code != 0 {
		t.Fatalf("Run: %d %v %s", code, err, out)
	}
	raw, _ := os.ReadFile(record)
	got := string(raw)
	if !strings.Contains(
		got,
		"argv: --no-save --no-restore --no-site-file --no-environ "+filepath.Join(prefix, "hook.R")+" --x f1\n",
	) {
		t.Errorf("argv not upstream's:\n%s", got)
	}
	if !strings.Contains(
		got,
		"R_PROFILE_USER="+filepath.Join(prefix, "renv-default", "activate.R")+" RENV_PROJECT=unset",
	) {
		t.Errorf("env not upstream's get_env_patch:\n%s", got)
	}

	// A local hook's script is left as written.
	if err := os.Truncate(record, 0); err != nil {
		t.Fatal(err)
	}
	ctx := WithLocalHook(context.Background(), true)
	if _, _, err := (&R{}).Run(ctx, prefix, t.TempDir(), "Rscript tools/hook.R", nil, nil, "default"); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(record)
	if !strings.Contains(string(raw), "argv: --no-save --no-restore --no-site-file --no-environ tools/hook.R\n") {
		t.Errorf("local hook argv:\n%s", raw)
	}
}

func TestRInstallEnvironment(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake Rscript is a sh script")
	}
	record := fakeRscript(t)
	prefix := t.TempDir()
	for name, body := range map[string]string{
		"renv.lock":       "{}\n",
		"renv/activate.R": "# activate\n",
		"renv/LICENSE":    "MIT\n",
	} {
		path := filepath.Join(prefix, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := (&R{}).InstallEnvironment(prefix, "default", []string{"dep1", "dep two"}); err != nil {
		t.Fatalf("InstallEnvironment: %v", err)
	}
	envDir := filepath.Join(prefix, "renv-default")
	for _, f := range []string{"renv.lock", "renv/activate.R", "renv/LICENSE"} {
		if _, err := os.Stat(filepath.Join(envDir, filepath.FromSlash(f))); err != nil {
			t.Errorf("%s not copied into the env: %v", f, err)
		}
	}
	raw, _ := os.ReadFile(record)
	got := string(raw)
	// The install script: vanilla R, in the env, the prefix as an argument.
	if !strings.Contains(got, "argv: --vanilla ") || !strings.Contains(got, "script.R "+prefix+"\n") {
		t.Errorf("install script not run as upstream does, with the prefix as its argument:\n%s", got)
	}
	if strings.Contains(got, "prefix_dir <- '"+prefix) {
		t.Errorf("the prefix must not be spliced into the R source:\n%s", got)
	}
	realEnv, _ := filepath.EvalSymlinks(envDir)
	if !strings.Contains(got, "pwd="+envDir) && !strings.Contains(got, "pwd="+realEnv) {
		t.Errorf("R should run in the env dir:\n%s", got)
	}
	if !strings.Contains(got, "options(\n    install.packages.compile.from.source = \"never\",") {
		t.Errorf("scripts should start with upstream's inline options:\n%s", got)
	}
	if !strings.Contains(got, "renv::settings$r.version(as.character(getRversion()))") {
		t.Errorf("the R version should be recorded:\n%s", got)
	}
	if !strings.Contains(got, "script.R dep1 dep two\n") {
		t.Errorf("additional dependencies should be passed as arguments:\n%s", got)
	}

	err := (&R{}).InstallEnvironment(t.TempDir(), "4.3.0", nil)
	want := "for now, pre-commit requires system-installed r -- you selected `language_version: 4.3.0`"
	if err == nil || err.Error() != want {
		t.Errorf("explicit version: %v, want %q", err, want)
	}
}

func TestRHealthCheck(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake Rscript is a sh script")
	}
	fakeRscript(t)
	prefix := t.TempDir()
	if err := os.MkdirAll(filepath.Join(prefix, "renv-default"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ installed, current, want string }{
		{"4.4.1", "4.4.1", ""},
		{"NULL", "4.4.1", "Hooks were installed with an unknown R version. R version for hook repo now set to 4.4.1"},
		{"", "4.4.1", "Hooks were installed with an unknown R version. R version for hook repo now set to 4.4.1"},
		{"4.3.0", "4.4.1", "Hooks were installed for R version 4.3.0, but current R executable has version 4.4.1"},
	} {
		t.Setenv("FAKE_R_INSTALLED", c.installed)
		t.Setenv("FAKE_R_CURRENT", c.current)
		got := ""
		if err := (&R{}).HealthCheck(prefix, "default"); err != nil {
			got = err.Error()
		}
		if got != c.want {
			t.Errorf("installed %q current %q: %q, want %q", c.installed, c.current, got, c.want)
		}
	}
}

func TestRscriptExec(t *testing.T) {
	os.Unsetenv("R_HOME")
	if got := rscriptExec(); got != "Rscript" {
		t.Errorf("without R_HOME: %q", got)
	}
	t.Setenv("R_HOME", "/opt/R")
	want := filepath.Join("/opt/R", "bin", "Rscript")
	if runtime.GOOS == "windows" {
		want += ".exe"
	}
	if got := rscriptExec(); got != want {
		t.Errorf("with R_HOME: %q, want %q", got, want)
	}
}
