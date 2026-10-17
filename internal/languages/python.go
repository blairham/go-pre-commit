// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package languages

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// Python implements the Language interface for Python hooks.
type Python struct{}

// Name is the language's name in a hook manifest.
func (p *Python) Name() string { return "python" }

// EnvironmentDir is upstream's ENVIRONMENT_DIR.
func (p *Python) EnvironmentDir() string { return "py_env" }

// GetDefaultVersion is upstream python.get_default_version (4.7.0), cached
// for the process as upstream's lru_cache does.
func (p *Python) GetDefaultVersion() string { return pythonDefaultVersionOnce() }

var pythonDefaultVersionOnce = sync.OnceValue(func() string {
	return pythonDefaultVersion(pythonHostOnce())
})

// pythonHost stands in for the interpreter upstream runs under: its
// get_default_version and norm_version are phrased in terms of
// sys.executable and sys.version_info. This port has no interpreter of its
// own, so it takes the one a pip-installed pre-commit would most likely run
// under — python3 on PATH (python on Windows, where python3 is usually the
// Store stub).
type pythonHost struct {
	exe          string // path, as sys.executable
	major, minor string // sys.version_info[:2]
}

var pythonHostOnce = sync.OnceValue(findPythonHost)

func findPythonHost() pythonHost {
	names := []string{"python3", "python"}
	if runtime.GOOS == "windows" {
		names = []string{"python", "python3"}
	}
	for _, name := range names {
		exe, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		out, err := exec.Command(exe, "-c", "import sys; print('%d %d' % sys.version_info[:2])").Output()
		if err != nil {
			continue
		}
		major, minor, ok := strings.Cut(strings.TrimSpace(string(out)), " ")
		if !ok {
			continue
		}
		return pythonHost{exe: exe, major: major, minor: minor}
	}
	return pythonHost{}
}

// pythonFind is upstream's parse_shebang.find_executable on this PATH.
func pythonFind(exe string) (string, bool) {
	return pyFindExecutable(exe, os.Getenv("PATH"), os.Getenv("PATHEXT"), pathextSet())
}

// pythonDefaultVersion is get_default_version for host: the likely
// interpreter name for host's version (python3.12, then python3) if it is on
// PATH, then sys.executable's own name, then on Windows the py launcher;
// otherwise `default`.
func pythonDefaultVersion(host pythonHost) string {
	if host.exe == "" {
		return "default"
	}
	for _, potential := range []string{host.major + "." + host.minor, host.major} {
		if _, ok := pythonFind("python" + potential); ok {
			return "python" + potential
		}
	}
	// _find_by_sys_executable's _norm.
	exe := strings.ToLower(filepath.Base(host.exe))
	exe, _, _ = strings.Cut(exe, ".exe")
	if exe != "python" && exe != "pythonw" {
		if _, ok := pythonFind(exe); ok {
			return exe
		}
	}
	if runtime.GOOS == "windows" {
		if pyLauncher("python"+host.major+"."+host.minor) != "" {
			return "python" + host.major + "." + host.minor
		}
	}
	return "default"
}

// pyLauncher is upstream's _find_by_py_launcher: the interpreter `py -X.Y`
// runs, or "" when there is none.
func pyLauncher(version string) string {
	num, ok := strings.CutPrefix(version, "python")
	if !ok {
		return ""
	}
	cmd := exec.Command("py", "-"+num, "-c", "import sys; print(sys.executable)")
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=UTF-8")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// sysExecutableMatches is upstream's _sys_executable_matches against host.
func sysExecutableMatches(version string, host pythonHost) bool {
	if version == "python" {
		return true
	}
	num, ok := strings.CutPrefix(version, "python")
	if !ok || host.exe == "" {
		return false
	}
	have := []string{host.major, host.minor}
	parts := strings.Split(num, ".")
	if len(parts) > len(have) {
		return false
	}
	for i, part := range parts {
		if _, err := strconv.Atoi(part); err != nil || part != have[i] {
			return false
		}
	}
	return true
}

// normPythonVersion is upstream's norm_version: the interpreter to hand
// virtualenv as -p, or "" for virtualenv's default (the host interpreter):
// `default`, or a version the host already is.
func normPythonVersion(version string, host pythonHost) string {
	if version == "default" || sysExecutableMatches(version, host) {
		return ""
	}
	if runtime.GOOS == "windows" {
		if exe := pyLauncher(version); exe != "" {
			return exe
		}
		if exe, ok := pythonFind(version); ok && exe != version {
			return exe
		}
	}
	// Otherwise assume it is a path.
	if rest, ok := strings.CutPrefix(version, "~"); ok && (rest == "" || strings.ContainsAny(rest[:1], `/\`)) {
		return pyHomeDir() + rest
	}
	return version
}

// HealthCheck is upstream python.health_check (4.7.0): the env's
// pyvenv.cfg must exist and record the interpreter version, the env's python
// must still be that version, and so must the base interpreter it was made
// from — an upgraded or removed host Python leaves a broken env behind,
// which this sends back to be rebuilt.
//
// Upstream always builds with virtualenv, which records `version_info`.
// This port falls back to the standard library's venv where virtualenv is
// not installed, and venv records only `version` (X.Y.Z); such an env is
// checked against the same version, compared on those three parts.
func (p *Python) HealthCheck(prefix, version string) error {
	envDir := filepath.Join(prefix, p.EnvironmentDir()+"-"+version)
	cfgPath := filepath.Join(envDir, "pyvenv.cfg")
	if _, err := os.Stat(cfgPath); err != nil {
		return errors.New("pyvenv.cfg does not exist (old virtualenv?)")
	}
	cfg, err := readPyvenvCfg(cfgPath)
	if err != nil {
		return err
	}

	want, ok := cfg["version_info"]
	trim := func(v string) string { return v }
	if !ok {
		if want, ok = cfg["version"]; !ok {
			return errors.New("created virtualenv's pyvenv.cfg is missing `version_info`")
		}
		trim = firstVersionParts
	}

	pyExe := filepath.Join(venvBinDir(envDir), "python")
	if runtime.GOOS == "windows" {
		pyExe += ".exe"
	}
	if got := trim(pythonVersionInfo(pyExe)); got != want {
		return fmt.Errorf( //nolint:staticcheck // upstream's wording, trailing newline and all
			"virtualenv python version did not match created version:\n"+
				"- actual version: %s\n"+
				"- expected version: %s\n",
			got, want,
		)
	}

	// Made with an older virtualenv: no base-executable to check.
	base, ok := cfg["base-executable"]
	if !ok {
		return nil
	}
	if got := trim(pythonVersionInfo(base)); got != want {
		return fmt.Errorf( //nolint:staticcheck // upstream's wording, trailing newline and all
			"base executable python version does not match created version:\n"+
				"- base-executable version: %s\n"+
				"- expected version: %s\n",
			got, want,
		)
	}
	return nil
}

// firstVersionParts is the X.Y.Z of a version_info string, as venv's
// pyvenv.cfg spells the version.
func firstVersionParts(v string) string {
	parts := strings.Split(v, ".")
	if len(parts) > 3 {
		parts = parts[:3]
	}
	return strings.Join(parts, ".")
}

// pythonVersionInfo is upstream's _version_info (uncached): exe's
// sys.version_info joined with dots, or a marker naming exe when it cannot
// be run.
func pythonVersionInfo(exe string) string {
	out, err := exec.Command(exe, "-S", "-c", `import sys;print(".".join(str(p) for p in sys.version_info))`).Output()
	if err != nil {
		return fmt.Sprintf("<<error retrieving version from %s>>", exe)
	}
	return strings.TrimSpace(string(out))
}

// readPyvenvCfg is upstream's _read_pyvenv_cfg: `key = value` lines, and a
// line without exactly one `=` skipped.
func readPyvenvCfg(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Count(line, "=") != 1 {
			continue
		}
		k, v, _ := strings.Cut(line, "=")
		cfg[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return cfg, nil
}

// InstallEnvironment is upstream's install_environment: virtualenv, run by
// the host interpreter, with -p naming any other interpreter the version
// asks for; then the hook package and its dependencies. Where virtualenv is
// not installed, the standard library's venv, run by that interpreter,
// stands in.
func (p *Python) InstallEnvironment(prefix, version string, additionalDeps []string) error {
	envDir := filepath.Join(prefix, p.EnvironmentDir()+"-"+version)
	host := pythonHostOnce()
	target := normPythonVersion(version, host)

	base := host.exe
	if base == "" {
		base = "python3"
		if runtime.GOOS == "windows" {
			base = "python"
		}
	}
	args := []string{"-mvirtualenv", envDir}
	if target != "" {
		args = append(args, "-p", target)
	}
	cmd := exec.Command(base, args...)
	cmd.Dir = prefix
	if out, err := cmd.CombinedOutput(); err != nil {
		venvPython := target
		if venvPython == "" {
			venvPython = base
		}
		cmd = exec.Command(venvPython, "-m", "venv", envDir)
		cmd.Dir = prefix
		if out2, err2 := cmd.CombinedOutput(); err2 != nil {
			return fmt.Errorf("failed to create virtualenv: %s\nfailed to create venv: %s", string(out), string(out2))
		}
	}

	// Install the hook package.
	pip := filepath.Join(venvBinDir(envDir), "pip")
	pipArgs := append([]string{"install", "."}, additionalDeps...)
	cmd = exec.Command(pip, pipArgs...)
	cmd.Dir = prefix
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pip install failed: %s: %w", string(out), err)
	}

	return nil
}

// Run implements Language.
func (p *Python) Run(
	ctx context.Context,
	prefix, workDir, entry string,
	args, fileArgs []string,
	version string,
) (int, []byte, error) {
	envDir := filepath.Join(prefix, p.EnvironmentDir()+"-"+version)
	env := []string{
		PrependPath(venvBinDir(envDir)),
		fmt.Sprintf("VIRTUAL_ENV=%s", envDir),
	}
	return RunHookCommand(ctx, workDir, entry, args, fileArgs, env)
}
