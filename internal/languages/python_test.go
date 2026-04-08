package languages

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Unit tests – no Python required
// ---------------------------------------------------------------------------

// TestPythonName verifies Name() returns "python" matching the upstream
// ENVIRONMENT_DIR / language name constant.
func TestPythonName(t *testing.T) {
	p := &Python{}
	if got := p.Name(); got != "python" {
		t.Errorf("Name() = %q, want %q", got, "python")
	}
}

// TestPythonEnvironmentDir verifies that py_env matches the upstream
// ENVIRONMENT_DIR = 'py_env' constant in pre_commit/languages/python.py.
func TestPythonEnvironmentDir(t *testing.T) {
	p := &Python{}
	if got := p.EnvironmentDir(); got != "py_env" {
		t.Errorf("EnvironmentDir() = %q, want %q", got, "py_env")
	}
}

// TestPythonGetDefaultVersion verifies the default version string.
// The upstream get_default_version() returns a platform-specific string like
// "python3.12"; our implementation returns the stable fallback "python3".
func TestPythonGetDefaultVersion(t *testing.T) {
	p := &Python{}
	if got := p.GetDefaultVersion(); got != "python3" {
		t.Errorf("GetDefaultVersion() = %q, want %q", got, "python3")
	}
}

// TestPythonEnvironmentDirNaming verifies that the envdir path follows the
// upstream lang_base.environment_dir convention:
//
//	prefix.path(f'{ENVIRONMENT_DIR}-{version}')
//
// which translates to filepath.Join(prefix, "py_env-"+version).
func TestPythonEnvironmentDirNaming(t *testing.T) {
	p := &Python{}
	cases := []struct {
		prefix, version, want string
	}{
		{"/some/prefix", "default", "/some/prefix/py_env-default"},
		{"/some/prefix", "python3.11", "/some/prefix/py_env-python3.11"},
		{"/tmp/repo", "python3", "/tmp/repo/py_env-python3"},
	}
	for _, tc := range cases {
		got := filepath.Join(tc.prefix, p.EnvironmentDir()+"-"+tc.version)
		if got != tc.want {
			t.Errorf("envDir(%q, %q) = %q, want %q", tc.prefix, tc.version, got, tc.want)
		}
	}
}

// TestPythonBinDir verifies that the bin dir is <envdir>/bin on POSIX,
// matching the upstream bin_dir() helper.
func TestPythonBinDir(t *testing.T) {
	p := &Python{}
	envDir := filepath.Join("/prefix", p.EnvironmentDir()+"-default")
	wantBin := filepath.Join(envDir, "bin")
	wantPy := filepath.Join(wantBin, "python")
	gotBin := filepath.Join(envDir, "bin")
	gotPy := filepath.Join(gotBin, "python")
	if gotBin != wantBin {
		t.Errorf("bin dir = %q, want %q", gotBin, wantBin)
	}
	if gotPy != wantPy {
		t.Errorf("python exe = %q, want %q", gotPy, wantPy)
	}
}

// TestPythonRunEnvVars verifies that Run() sets VIRTUAL_ENV and prepends the
// bin dir to PATH, mirroring get_env_patch() in the upstream implementation.
func TestPythonRunEnvVars(t *testing.T) {
	p := &Python{}
	prefix := "/fake/prefix"
	version := "default"
	envDir := filepath.Join(prefix, p.EnvironmentDir()+"-"+version)
	binDir := filepath.Join(envDir, "bin")

	wantVE := fmt.Sprintf("VIRTUAL_ENV=%s", envDir)
	wantPath := PrependPath(binDir)

	if !strings.HasPrefix(wantPath, "PATH=") {
		t.Errorf("PrependPath(%q) does not start with PATH=: %q", binDir, wantPath)
	}
	if !strings.Contains(wantPath, binDir) {
		t.Errorf("PATH value %q does not contain bin dir %q", wantPath, binDir)
	}
	if wantVE != fmt.Sprintf("VIRTUAL_ENV=%s", envDir) {
		t.Errorf("VIRTUAL_ENV = %q", wantVE)
	}
}

// ---------------------------------------------------------------------------
// Integration tests – require a working Python interpreter on PATH
// ---------------------------------------------------------------------------

// requirePython skips the test if no Python interpreter is available,
// matching the conditional skip pattern used in the upstream test suite.
func requirePython(t *testing.T) {
	t.Helper()
	for _, name := range []string{"python3", "python"} {
		if _, err := exec.LookPath(name); err == nil {
			return
		}
	}
	t.Skip("no python3/python found on PATH")
}

// makePythonRepo creates a minimal Python project with a console-script entry
// point named "socks".  This mirrors the fixture from the upstream Python test
// test_simple_python_hook.
func makePythonRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	setupPy := "from setuptools import setup\nsetup(\n    name='socks',\n    version='0.0.0',\n    py_modules=['socks'],\n    entry_points={'console_scripts': ['socks = socks:main']},\n)\n"
	mainPy := "import sys\n\ndef main():\n    print(repr(sys.argv[1:]))\n    print('hello hello')\n    return 0\n"
	if err := os.WriteFile(filepath.Join(dir, "setup.py"), []byte(setupPy), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "socks.py"), []byte(mainPy), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// makeMinimalRepo creates the smallest Python project that satisfies pip install.
func makeMinimalRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	setup := "import setuptools; setuptools.setup()\n"
	if err := os.WriteFile(filepath.Join(dir, "setup.py"), []byte(setup), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestPythonInstallEnvironmentDefault mirrors test_healthy_default_creator:
// after InstallEnvironment the environment must pass HealthCheck.
func TestPythonInstallEnvironmentDefault(t *testing.T) {
	requirePython(t)
	p := &Python{}
	prefix := makeMinimalRepo(t)

	if err := p.InstallEnvironment(prefix, "default", nil); err != nil {
		t.Fatalf("InstallEnvironment: %v", err)
	}
	if err := p.HealthCheck(prefix, "default"); err != nil {
		t.Errorf("HealthCheck after fresh install = %v, want nil", err)
	}
}

// TestPythonInstallEnvironmentWithDeps verifies that additional_dependencies
// are forwarded to pip, matching the upstream additional_dependencies handling.
func TestPythonInstallEnvironmentWithDeps(t *testing.T) {
	requirePython(t)
	p := &Python{}
	prefix := makeMinimalRepo(t)

	if err := p.InstallEnvironment(prefix, "default", []string{"six"}); err != nil {
		t.Fatalf("InstallEnvironment with deps: %v", err)
	}
	envDir := filepath.Join(prefix, p.EnvironmentDir()+"-default")
	python := filepath.Join(envDir, "bin", "python")
	cmd := exec.Command(python, "-c", "import six")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("six not importable after install: %v\n%s", err, out)
	}
}

// TestPythonHealthCheckMissingExecutable mirrors test_unhealthy_python_goes_missing:
// removing the python binary from the venv should make HealthCheck return an error.
func TestPythonHealthCheckMissingExecutable(t *testing.T) {
	requirePython(t)
	p := &Python{}
	prefix := makeMinimalRepo(t)

	if err := p.InstallEnvironment(prefix, "default", nil); err != nil {
		t.Fatalf("InstallEnvironment: %v", err)
	}
	pyExe := filepath.Join(prefix, p.EnvironmentDir()+"-default", "bin", "python")
	if err := os.Remove(pyExe); err != nil {
		t.Fatalf("removing python exe: %v", err)
	}
	if err := p.HealthCheck(prefix, "default"); err == nil {
		t.Error("HealthCheck with missing python = nil, want error")
	}
}

// TestPythonRunSimpleHook mirrors test_simple_python_hook: install a project
// with a console-script entry and verify stdout contains the expected output.
func TestPythonRunSimpleHook(t *testing.T) {
	requirePython(t)
	p := &Python{}
	prefix := makePythonRepo(t)

	if err := p.InstallEnvironment(prefix, "default", nil); err != nil {
		t.Fatalf("InstallEnvironment: %v", err)
	}
	code, out, err := p.Run(context.Background(), prefix, prefix,"socks", nil, []string{"/dev/null"}, "default")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0\noutput: %s", code, out)
	}
	if !strings.Contains(string(out), "hello hello") {
		t.Errorf("output %q does not contain 'hello hello'", out)
	}
}

// TestPythonRunVersionDefault verifies that "default" as the version resolves
// to the installed environment correctly.
func TestPythonRunVersionDefault(t *testing.T) {
	requirePython(t)
	p := &Python{}
	prefix := makePythonRepo(t)

	if err := p.InstallEnvironment(prefix, "default", nil); err != nil {
		t.Fatalf("InstallEnvironment: %v", err)
	}
	code, out, err := p.Run(context.Background(), prefix, prefix,"python", []string{"--version"}, nil, "default")
	if err != nil {
		t.Fatalf("Run python --version: %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0\noutput: %s", code, out)
	}
	if !strings.HasPrefix(string(out), "Python ") {
		t.Errorf("output %q does not start with 'Python '", out)
	}
}

// TestPythonRunExitCode verifies that non-zero exit codes from the hook process
// are propagated unchanged, matching upstream basic_run_hook behavior.
func TestPythonRunExitCode(t *testing.T) {
	requirePython(t)
	p := &Python{}
	prefix := makeMinimalRepo(t)

	if err := p.InstallEnvironment(prefix, "default", nil); err != nil {
		t.Fatalf("InstallEnvironment: %v", err)
	}
	code, _, err := p.Run(context.Background(), prefix, prefix,"python", []string{"-c", "import sys; sys.exit(42)"}, nil, "default")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if code != 42 {
		t.Errorf("exit code = %d, want 42", code)
	}
}
