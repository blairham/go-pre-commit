// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package languages

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// upstream's rust.get_default_version asks cargo itself: a cargo that is on
// PATH but cannot run (rustup with no toolchain) is not a system rust.
func TestRustDefaultVersion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake cargo is a sh script")
	}
	for script, want := range map[string]string{
		"#!/bin/sh\nexit 0\n": "system",
		"#!/bin/sh\nexit 1\n": "default",
		"":                    "default", // no cargo at all
	} {
		dir := t.TempDir()
		if script != "" {
			if err := os.WriteFile(filepath.Join(dir, "cargo"), []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		t.Setenv("PATH", dir)
		if got := rustDefaultVersion(); got != want {
			t.Errorf("cargo %q: rustDefaultVersion = %q, want %q", script, got, want)
		}
	}
}

// rustFakes puts recording fakes of cargo (and, when withRustup, rustup) on
// PATH and serves a rustup-init that installs a recording rustup into
// $CARGO_HOME/bin, as the real one does.
func rustFakes(t *testing.T, withRustup bool) (record string) {
	t.Helper()
	dir := t.TempDir()
	record = filepath.Join(t.TempDir(), "calls")
	logLine := `echo "$(basename "$0") $*|CARGO_HOME=${CARGO_HOME-}|RUSTUP_HOME=${RUSTUP_HOME:+set}|RUSTUP_TOOLCHAIN=${RUSTUP_TOOLCHAIN-unset}" >> ` + record + "\n"
	fake := "#!/bin/sh\n" + logLine + "exit 0\n"
	write := func(path, body string) {
		if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(dir, "cargo"), fake)
	if withRustup {
		write(filepath.Join(dir, "rustup"), fake)
	}
	initScript := "#!/bin/sh\n" + logLine +
		"mkdir -p \"$CARGO_HOME/bin\"\n" +
		"cat > \"$CARGO_HOME/bin/rustup\" <<'EOS'\n" + fake + "EOS\n" +
		"chmod +x \"$CARGO_HOME/bin/rustup\"\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(initScript))
	}))
	t.Cleanup(srv.Close)
	old := rustupInitURL
	rustupInitURL = srv.URL
	t.Cleanup(func() { rustupInitURL = old })
	t.Setenv("PATH", dir+string(os.PathListSeparator)+"/usr/bin"+string(os.PathListSeparator)+"/bin")
	t.Setenv("CARGO_HOME", "")
	os.Unsetenv("CARGO_HOME")
	return record
}

func readCalls(t *testing.T, record string) []string {
	t.Helper()
	raw, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(raw)), "\n")
}

// upstream's install_environment for each kind of version.
func TestRustInstallEnvironment(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fakes are sh scripts")
	}
	deps := []string{"cli:ripgrep:14.0.0", "cli:fd-find", "serde:1.0", "anyhow"}

	t.Run("system", func(t *testing.T) {
		record := rustFakes(t, false)
		prefix := t.TempDir()
		if err := (&Rust{}).InstallEnvironment(prefix, "system", deps); err != nil {
			t.Fatal(err)
		}
		env := filepath.Join(prefix, "rustenv-system")
		tail := "|CARGO_HOME=|RUSTUP_HOME=|RUSTUP_TOOLCHAIN=unset"
		assertSliceEqual(t, readCalls(t, record), []string{
			"cargo add serde@1.0 anyhow@*" + tail,
			"cargo install --bins --root " + env + " --path ." + tail,
			"cargo install --bins --root " + env + " ripgrep --version 14.0.0" + tail,
			"cargo install --bins --root " + env + " fd-find" + tail,
		})
	})

	t.Run("default downloads rustup", func(t *testing.T) {
		record := rustFakes(t, false)
		prefix := t.TempDir()
		if err := (&Rust{}).InstallEnvironment(prefix, "default", nil); err != nil {
			t.Fatal(err)
		}
		env := filepath.Join(prefix, "rustenv-default")
		assertSliceEqual(t, readCalls(t, record), []string{
			"rustup-init -y --quiet --no-modify-path --default-toolchain none|CARGO_HOME=" + env + "|RUSTUP_HOME=set|RUSTUP_TOOLCHAIN=stable",
			"rustup toolchain install --no-self-update stable|CARGO_HOME=" + env + "|RUSTUP_HOME=set|RUSTUP_TOOLCHAIN=stable",
			"cargo install --bins --root " + env + " --path .|CARGO_HOME=|RUSTUP_HOME=set|RUSTUP_TOOLCHAIN=stable",
		})
	})

	t.Run("explicit toolchain with rustup present", func(t *testing.T) {
		record := rustFakes(t, true)
		prefix := t.TempDir()
		if err := (&Rust{}).InstallEnvironment(prefix, "1.80.0", nil); err != nil {
			t.Fatal(err)
		}
		env := filepath.Join(prefix, "rustenv-1.80.0")
		assertSliceEqual(t, readCalls(t, record), []string{
			"rustup toolchain install --no-self-update 1.80.0|CARGO_HOME=" + env + "|RUSTUP_HOME=set|RUSTUP_TOOLCHAIN=1.80.0",
			"cargo install --bins --root " + env + " --path .|CARGO_HOME=|RUSTUP_HOME=set|RUSTUP_TOOLCHAIN=1.80.0",
		})
	})
}

func TestRustEnvPatch(t *testing.T) {
	t.Setenv("PATH", "/usr/bin")
	path := "PATH=" + filepath.Join("/e", "bin") + string(os.PathListSeparator) + "/usr/bin"
	assertSliceEqual(t, rustEnvPatch("/e", "system"), []string{path})
	assertSliceEqual(t, rustEnvPatch("/e", "default"), []string{path, "RUSTUP_TOOLCHAIN=stable"})
	assertSliceEqual(t, rustEnvPatch("/e", "nightly"), []string{path, "RUSTUP_TOOLCHAIN=nightly"})
}
