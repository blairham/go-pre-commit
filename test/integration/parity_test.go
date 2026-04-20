// Package integration tests compare the Go pre-commit binary against the
// Python pre-commit tool to ensure CLI parity.
//
// These tests require:
//   - Python pre-commit installed and on PATH
//   - Go binary buildable from the repo root
//
// Run with: go test -v -tags=integration -timeout=600s ./test/integration/
package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

var goBinary string

// ---------------------------------------------------------------------------
// Report infrastructure
// ---------------------------------------------------------------------------

var parityReport struct {
	mu      sync.Mutex
	results []parityResult
}

type parityResult struct {
	Command  string `json:"command"`
	Category string `json:"category"`
	Test     string `json:"test"`
	PyExit   int    `json:"py_exit"`
	GoExit   int    `json:"go_exit"`
	Match    bool   `json:"match"`
	Detail   string `json:"detail,omitempty"`
}

func addResult(command, category, test string, pyExit, goExit int, match bool, detail string) {
	parityReport.mu.Lock()
	defer parityReport.mu.Unlock()
	parityReport.results = append(parityReport.results, parityResult{
		Command:  command,
		Category: category,
		Test:     test,
		PyExit:   pyExit,
		GoExit:   goExit,
		Match:    match,
		Detail:   detail,
	})
}

// shorthand for common categories
func addExitResult(cmd, test string, py, go_ int, match bool, detail string) {
	addResult(cmd, "exit code", test, py, go_, match, detail)
}

func addFSResult(cmd, test string, match bool, detail string) {
	addResult(cmd, "filesystem", test, 0, 0, match, detail)
}

func addOutputResult(cmd, test string, match bool, detail string) {
	addResult(cmd, "output", test, 0, 0, match, detail)
}

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "go-pre-commit-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create temp dir: %v\n", err)
		os.Exit(1)
	}

	goBinary = filepath.Join(tmp, "pre-commit")

	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to find repo root: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command("go", "build", "-o", goBinary, ".")
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to build Go binary: %v\n%s\n", err, out)
		os.Exit(1)
	}

	exitCode := m.Run()
	printParityReport()
	os.RemoveAll(tmp)
	os.Exit(exitCode)
}

func printParityReport() {
	parityReport.mu.Lock()
	defer parityReport.mu.Unlock()

	pass, fail, total := 0, 0, len(parityReport.results)
	for _, r := range parityReport.results {
		if r.Match {
			pass++
		} else {
			fail++
		}
	}

	pct := float64(0)
	if total > 0 {
		pct = float64(pass) / float64(total) * 100
	}

	w := os.Stderr
	fmt.Fprintln(w)
	fmt.Fprintln(w, "================================================================================")
	fmt.Fprintln(w, "                          PRE-COMMIT PARITY REPORT")
	fmt.Fprintln(w, "================================================================================")
	fmt.Fprintf(w, "  Generated: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(w, "  Total checks: %d   Pass: %d   Fail: %d   Parity: %.1f%%\n", total, pass, fail, pct)
	fmt.Fprintln(w, "================================================================================")

	currentCmd := ""
	for _, r := range parityReport.results {
		if r.Command != currentCmd {
			currentCmd = r.Command
			fmt.Fprintln(w)
			fmt.Fprintf(w, "  Command: %s\n", currentCmd)
			fmt.Fprintf(w, "  %s\n", strings.Repeat("-", 70))
		}
		icon := "PASS"
		if !r.Match {
			icon = "FAIL"
		}
		fmt.Fprintf(w, "    [%s] [%-10s] %s", icon, r.Category, r.Test)
		if r.Category == "exit code" {
			fmt.Fprintf(w, " (py=%d go=%d)", r.PyExit, r.GoExit)
		}
		fmt.Fprintln(w)
		if !r.Match && r.Detail != "" {
			fmt.Fprintf(w, "           -> %s\n", r.Detail)
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "================================================================================")
	fmt.Fprintf(w, "  SUMMARY: %d/%d checks passed (%.1f%% parity)\n", pass, total, pct)
	if fail > 0 {
		fmt.Fprintf(w, "  FAILURES: %d checks failed\n", fail)
	}
	fmt.Fprintln(w, "================================================================================")

	// Write JSON report.
	repoRoot, _ := filepath.Abs(filepath.Join("..", ".."))
	reportPath := filepath.Join(repoRoot, "test", "integration", "parity_report.json")
	type jsonReport struct {
		Generated string         `json:"generated"`
		Total     int            `json:"total"`
		Pass      int            `json:"pass"`
		Fail      int            `json:"fail"`
		Parity    string         `json:"parity"`
		Results   []parityResult `json:"results"`
	}
	report := jsonReport{
		Generated: time.Now().Format(time.RFC3339),
		Total:     total,
		Pass:      pass,
		Fail:      fail,
		Parity:    fmt.Sprintf("%.1f%%", pct),
		Results:   parityReport.results,
	}
	data, _ := json.MarshalIndent(report, "", "  ")
	os.WriteFile(reportPath, data, 0o644)
	fmt.Fprintf(w, "\n  JSON report: %s\n", reportPath)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func pythonPreCommit(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("pre-commit")
	if err != nil {
		t.Skip("Python pre-commit not found on PATH")
	}
	return path
}

func runCmd(t *testing.T, dir, name string, args ...string) (combined string, exitCode int) {
	t.Helper()
	var buf bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	exitCode = 0
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			exitCode = e.ExitCode()
		} else {
			t.Fatalf("failed to run %s %v: %v", name, args, err)
		}
	}
	return buf.String(), exitCode
}

func initTestRepo(t *testing.T, cfg, testFileContent string) string {
	t.Helper()
	tmp := t.TempDir()
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = tmp
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	if cfg != "" {
		if err := os.WriteFile(filepath.Join(tmp, ".pre-commit-config.yaml"), []byte(cfg), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if testFileContent != "" {
		if err := os.WriteFile(filepath.Join(tmp, "test.txt"), []byte(testFileContent), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = tmp
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %v\n%s", err, out)
	}
	return tmp
}

func readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func extractHookResults(output string) []hookResult {
	var results []hookResult
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		dotIdx := strings.Index(line, ".")
		if dotIdx < 0 {
			continue
		}
		afterDots := strings.TrimLeft(line[dotIdx:], ".")
		afterDots = strings.TrimSpace(afterDots)
		switch strings.ToLower(afterDots) {
		case "passed", "failed", "skipped", "error":
			results = append(results, hookResult{
				Name:   strings.TrimSpace(line[:dotIdx]),
				Status: strings.ToLower(afterDots),
			})
		}
	}
	return results
}

type hookResult struct {
	Name   string
	Status string
}

var standardCfg = `repos:
-   repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v5.0.0
    hooks:
    -   id: trailing-whitespace
    -   id: end-of-file-fixer
    -   id: check-yaml
`

// ---------------------------------------------------------------------------
// Tests: version
// ---------------------------------------------------------------------------

func TestVersion(t *testing.T) {
	pyBin := pythonPreCommit(t)

	pyOut, pyExit := runCmd(t, ".", pyBin, "--version")
	goOut, goExit := runCmd(t, ".", goBinary, "--version")

	exitMatch := pyExit == goExit
	addExitResult("--version", "exits 0", pyExit, goExit, exitMatch, "")

	pyHas := strings.Contains(pyOut, "pre-commit")
	goHas := strings.Contains(goOut, "pre-commit")
	addOutputResult("--version", "output contains 'pre-commit'", pyHas && goHas,
		fmt.Sprintf("py=%q go=%q", strings.TrimSpace(pyOut), strings.TrimSpace(goOut)))
}

// ---------------------------------------------------------------------------
// Tests: help
// ---------------------------------------------------------------------------

func TestHelp(t *testing.T) {
	pyBin := pythonPreCommit(t)

	pyOut, _ := runCmd(t, ".", pyBin, "help")
	goOut, _ := runCmd(t, ".", goBinary, "--help")

	for _, cmd := range []string{
		"autoupdate", "clean", "gc", "init-templatedir", "install",
		"install-hooks", "migrate-config", "run", "sample-config",
		"try-repo", "uninstall", "validate-config", "validate-manifest",
	} {
		pyHas := strings.Contains(pyOut, cmd)
		goHas := strings.Contains(goOut, cmd)
		addOutputResult("help", fmt.Sprintf("lists %q", cmd), pyHas == goHas,
			fmt.Sprintf("py=%v go=%v", pyHas, goHas))
	}
}

// ---------------------------------------------------------------------------
// Tests: sample-config
// ---------------------------------------------------------------------------

func TestSampleConfig(t *testing.T) {
	pyBin := pythonPreCommit(t)

	pyOut, pyExit := runCmd(t, ".", pyBin, "sample-config")
	goOut, goExit := runCmd(t, ".", goBinary, "sample-config")

	addExitResult("sample-config", "exits 0", pyExit, goExit, pyExit == goExit, "")

	pyLines := strings.Split(strings.TrimSpace(pyOut), "\n")
	goLines := strings.Split(strings.TrimSpace(goOut), "\n")
	addOutputResult("sample-config", "same line count",
		len(pyLines) == len(goLines),
		fmt.Sprintf("py=%d go=%d", len(pyLines), len(goLines)))

	if len(pyLines) == len(goLines) {
		allMatch := true
		for i := range pyLines {
			py := strings.TrimSpace(pyLines[i])
			go_ := strings.TrimSpace(goLines[i])
			if strings.HasPrefix(py, "rev:") && strings.HasPrefix(go_, "rev:") {
				continue
			}
			if py != go_ {
				allMatch = false
			}
		}
		addOutputResult("sample-config", "content matches (ignoring rev)", allMatch, "")
	}
}

// ---------------------------------------------------------------------------
// Tests: validate-config
// ---------------------------------------------------------------------------

func TestValidateConfig(t *testing.T) {
	pyBin := pythonPreCommit(t)

	t.Run("missing file", func(t *testing.T) {
		_, pyExit := runCmd(t, ".", pyBin, "validate-config", "/nonexistent/file.yaml")
		_, goExit := runCmd(t, ".", goBinary, "validate-config", "/nonexistent/file.yaml")
		addExitResult("validate-config", "missing file fails", pyExit, goExit,
			(pyExit != 0) && (goExit != 0), "")
	})

	t.Run("valid file", func(t *testing.T) {
		tmp := t.TempDir()
		cfgPath := filepath.Join(tmp, ".pre-commit-config.yaml")
		os.WriteFile(cfgPath, []byte(standardCfg), 0o644)
		_, pyExit := runCmd(t, ".", pyBin, "validate-config", cfgPath)
		_, goExit := runCmd(t, ".", goBinary, "validate-config", cfgPath)
		addExitResult("validate-config", "valid file succeeds", pyExit, goExit, pyExit == goExit, "")
	})

	t.Run("invalid YAML", func(t *testing.T) {
		tmp := t.TempDir()
		cfgPath := filepath.Join(tmp, "bad.yaml")
		os.WriteFile(cfgPath, []byte("not: [valid: yaml: {{{"), 0o644)
		_, pyExit := runCmd(t, ".", pyBin, "validate-config", cfgPath)
		_, goExit := runCmd(t, ".", goBinary, "validate-config", cfgPath)
		addExitResult("validate-config", "invalid YAML fails", pyExit, goExit,
			(pyExit != 0) && (goExit != 0), "")
	})

	t.Run("empty file", func(t *testing.T) {
		tmp := t.TempDir()
		cfgPath := filepath.Join(tmp, "empty.yaml")
		os.WriteFile(cfgPath, []byte(""), 0o644)
		_, pyExit := runCmd(t, ".", pyBin, "validate-config", cfgPath)
		_, goExit := runCmd(t, ".", goBinary, "validate-config", cfgPath)
		addExitResult("validate-config", "empty file", pyExit, goExit,
			(pyExit != 0) == (goExit != 0), "")
	})
}

// ---------------------------------------------------------------------------
// Tests: validate-manifest
// ---------------------------------------------------------------------------

func TestValidateManifest(t *testing.T) {
	pyBin := pythonPreCommit(t)

	t.Run("missing file", func(t *testing.T) {
		_, pyExit := runCmd(t, ".", pyBin, "validate-manifest", "/nonexistent/hooks.yaml")
		_, goExit := runCmd(t, ".", goBinary, "validate-manifest", "/nonexistent/hooks.yaml")
		addExitResult("validate-manifest", "missing file fails", pyExit, goExit,
			(pyExit != 0) && (goExit != 0), "")
	})

	t.Run("valid manifest", func(t *testing.T) {
		tmp := t.TempDir()
		mPath := filepath.Join(tmp, ".pre-commit-hooks.yaml")
		os.WriteFile(mPath, []byte("- id: my-hook\n  name: My Hook\n  entry: my-hook\n  language: system\n"), 0o644)
		_, pyExit := runCmd(t, ".", pyBin, "validate-manifest", mPath)
		_, goExit := runCmd(t, ".", goBinary, "validate-manifest", mPath)
		addExitResult("validate-manifest", "valid manifest succeeds", pyExit, goExit, pyExit == goExit, "")
	})
}

// ---------------------------------------------------------------------------
// Tests: clean
// ---------------------------------------------------------------------------

func TestClean(t *testing.T) {
	pyBin := pythonPreCommit(t)
	_, pyExit := runCmd(t, ".", pyBin, "clean")
	_, goExit := runCmd(t, ".", goBinary, "clean")
	addExitResult("clean", "exits 0", pyExit, goExit, pyExit == goExit, "")
}

// ---------------------------------------------------------------------------
// Tests: gc
// ---------------------------------------------------------------------------

func TestGC(t *testing.T) {
	pyBin := pythonPreCommit(t)
	repo := initTestRepo(t, standardCfg, "test\n")
	_, pyExit := runCmd(t, repo, pyBin, "gc")
	_, goExit := runCmd(t, repo, goBinary, "gc")
	addExitResult("gc", "exits 0 with config", pyExit, goExit, pyExit == goExit, "")
}

// ---------------------------------------------------------------------------
// Tests: install (exit codes + filesystem)
// ---------------------------------------------------------------------------

func TestInstall(t *testing.T) {
	pyBin := pythonPreCommit(t)

	t.Run("creates hook file", func(t *testing.T) {
		pyRepo := initTestRepo(t, standardCfg, "test\n")
		goRepo := initTestRepo(t, standardCfg, "test\n")

		_, pyExit := runCmd(t, pyRepo, pyBin, "install")
		_, goExit := runCmd(t, goRepo, goBinary, "install")
		addExitResult("install", "exits 0", pyExit, goExit, pyExit == goExit, "")

		pyHookPath := filepath.Join(pyRepo, ".git", "hooks", "pre-commit")
		goHookPath := filepath.Join(goRepo, ".git", "hooks", "pre-commit")

		pyExists := fileExists(pyHookPath)
		goExists := fileExists(goHookPath)
		addFSResult("install", "hook file exists", pyExists == goExists,
			fmt.Sprintf("py=%v go=%v", pyExists, goExists))

		// Both hook files should contain "hook-impl".
		pyContent := readFile(pyHookPath)
		goContent := readFile(goHookPath)
		pyHasImpl := strings.Contains(pyContent, "hook-impl")
		goHasImpl := strings.Contains(goContent, "hook-impl")
		addFSResult("install", "hook file contains hook-impl", pyHasImpl && goHasImpl,
			fmt.Sprintf("py=%v go=%v", pyHasImpl, goHasImpl))

		// Both should be executable.
		pyInfo, _ := os.Stat(pyHookPath)
		goInfo, _ := os.Stat(goHookPath)
		pyExec := pyInfo != nil && pyInfo.Mode()&0o111 != 0
		goExec := goInfo != nil && goInfo.Mode()&0o111 != 0
		addFSResult("install", "hook file is executable", pyExec && goExec,
			fmt.Sprintf("py=%v go=%v", pyExec, goExec))
	})

	t.Run("installs pre-push hook type", func(t *testing.T) {
		pyRepo := initTestRepo(t, standardCfg, "test\n")
		goRepo := initTestRepo(t, standardCfg, "test\n")

		_, pyExit := runCmd(t, pyRepo, pyBin, "install", "-t", "pre-push")
		_, goExit := runCmd(t, goRepo, goBinary, "install", "-t", "pre-push")
		addExitResult("install", "-t pre-push exits 0", pyExit, goExit, pyExit == goExit, "")

		pyExists := fileExists(filepath.Join(pyRepo, ".git", "hooks", "pre-push"))
		goExists := fileExists(filepath.Join(goRepo, ".git", "hooks", "pre-push"))
		addFSResult("install", "pre-push hook file exists", pyExists && goExists,
			fmt.Sprintf("py=%v go=%v", pyExists, goExists))

		// pre-commit hook should NOT be created when only pre-push is requested.
		pyNoDefault := !fileExists(filepath.Join(pyRepo, ".git", "hooks", "pre-commit"))
		goNoDefault := !fileExists(filepath.Join(goRepo, ".git", "hooks", "pre-commit"))
		addFSResult("install", "only requested hook type created", pyNoDefault == goNoDefault,
			fmt.Sprintf("py_no_default=%v go_no_default=%v", pyNoDefault, goNoDefault))
	})

	t.Run("without config succeeds", func(t *testing.T) {
		pyRepo := initTestRepo(t, "", "test\n")
		goRepo := initTestRepo(t, "", "test\n")

		_, pyExit := runCmd(t, pyRepo, pyBin, "install")
		_, goExit := runCmd(t, goRepo, goBinary, "install")
		addExitResult("install", "no config succeeds", pyExit, goExit, pyExit == goExit, "")
	})

	t.Run("allow-missing-config", func(t *testing.T) {
		pyRepo := initTestRepo(t, "", "test\n")
		goRepo := initTestRepo(t, "", "test\n")

		_, pyExit := runCmd(t, pyRepo, pyBin, "install", "--allow-missing-config")
		_, goExit := runCmd(t, goRepo, goBinary, "install", "--allow-missing-config")
		addExitResult("install", "--allow-missing-config exits 0", pyExit, goExit, pyExit == goExit, "")
	})

	t.Run("overwrite existing hook", func(t *testing.T) {
		pyRepo := initTestRepo(t, standardCfg, "test\n")
		goRepo := initTestRepo(t, standardCfg, "test\n")

		// Create a non-pre-commit hook file first.
		pyHookPath := filepath.Join(pyRepo, ".git", "hooks", "pre-commit")
		goHookPath := filepath.Join(goRepo, ".git", "hooks", "pre-commit")
		os.MkdirAll(filepath.Dir(pyHookPath), 0o755)
		os.MkdirAll(filepath.Dir(goHookPath), 0o755)
		os.WriteFile(pyHookPath, []byte("#!/bin/sh\necho custom\n"), 0o755)
		os.WriteFile(goHookPath, []byte("#!/bin/sh\necho custom\n"), 0o755)

		_, pyExit := runCmd(t, pyRepo, pyBin, "install", "-f")
		_, goExit := runCmd(t, goRepo, goBinary, "install", "-f")
		addExitResult("install", "--overwrite exits 0", pyExit, goExit, pyExit == goExit, "")

		// Hook should now contain hook-impl.
		pyContent := readFile(pyHookPath)
		goContent := readFile(goHookPath)
		pyOK := strings.Contains(pyContent, "hook-impl")
		goOK := strings.Contains(goContent, "hook-impl")
		addFSResult("install", "overwritten hook contains hook-impl", pyOK && goOK,
			fmt.Sprintf("py=%v go=%v", pyOK, goOK))
	})

	t.Run("backs up legacy hook", func(t *testing.T) {
		pyRepo := initTestRepo(t, standardCfg, "test\n")
		goRepo := initTestRepo(t, standardCfg, "test\n")

		// Create a non-pre-commit hook file (without -f, it should be backed up).
		pyHookPath := filepath.Join(pyRepo, ".git", "hooks", "pre-commit")
		goHookPath := filepath.Join(goRepo, ".git", "hooks", "pre-commit")
		os.MkdirAll(filepath.Dir(pyHookPath), 0o755)
		os.MkdirAll(filepath.Dir(goHookPath), 0o755)
		os.WriteFile(pyHookPath, []byte("#!/bin/sh\necho legacy\n"), 0o755)
		os.WriteFile(goHookPath, []byte("#!/bin/sh\necho legacy\n"), 0o755)

		runCmd(t, pyRepo, pyBin, "install")
		runCmd(t, goRepo, goBinary, "install")

		pyLegacy := fileExists(pyHookPath + ".legacy")
		goLegacy := fileExists(goHookPath + ".legacy")
		addFSResult("install", "legacy hook backed up", pyLegacy == goLegacy,
			fmt.Sprintf("py=%v go=%v", pyLegacy, goLegacy))
	})
}

// ---------------------------------------------------------------------------
// Tests: uninstall (exit codes + filesystem)
// ---------------------------------------------------------------------------

func TestUninstall(t *testing.T) {
	pyBin := pythonPreCommit(t)

	t.Run("removes hook", func(t *testing.T) {
		pyRepo := initTestRepo(t, standardCfg, "test\n")
		goRepo := initTestRepo(t, standardCfg, "test\n")

		runCmd(t, pyRepo, pyBin, "install")
		runCmd(t, goRepo, goBinary, "install")

		_, pyExit := runCmd(t, pyRepo, pyBin, "uninstall")
		_, goExit := runCmd(t, goRepo, goBinary, "uninstall")
		addExitResult("uninstall", "exits 0", pyExit, goExit, pyExit == goExit, "")

		pyGone := !fileExists(filepath.Join(pyRepo, ".git", "hooks", "pre-commit"))
		goGone := !fileExists(filepath.Join(goRepo, ".git", "hooks", "pre-commit"))
		addFSResult("uninstall", "hook file removed", pyGone && goGone,
			fmt.Sprintf("py=%v go=%v", pyGone, goGone))
	})

	t.Run("noop when no hook", func(t *testing.T) {
		pyRepo := initTestRepo(t, standardCfg, "test\n")
		goRepo := initTestRepo(t, standardCfg, "test\n")

		_, pyExit := runCmd(t, pyRepo, pyBin, "uninstall")
		_, goExit := runCmd(t, goRepo, goBinary, "uninstall")
		addExitResult("uninstall", "noop exits 0", pyExit, goExit, pyExit == goExit, "")
	})

	t.Run("restores legacy hook", func(t *testing.T) {
		pyRepo := initTestRepo(t, standardCfg, "test\n")
		goRepo := initTestRepo(t, standardCfg, "test\n")

		// Create legacy hook, install (backs up), then uninstall (restores).
		for _, dir := range []string{pyRepo, goRepo} {
			hookPath := filepath.Join(dir, ".git", "hooks", "pre-commit")
			os.MkdirAll(filepath.Dir(hookPath), 0o755)
			os.WriteFile(hookPath, []byte("#!/bin/sh\necho legacy\n"), 0o755)
		}

		runCmd(t, pyRepo, pyBin, "install")
		runCmd(t, goRepo, goBinary, "install")
		runCmd(t, pyRepo, pyBin, "uninstall")
		runCmd(t, goRepo, goBinary, "uninstall")

		pyRestored := strings.Contains(readFile(filepath.Join(pyRepo, ".git", "hooks", "pre-commit")), "legacy")
		goRestored := strings.Contains(readFile(filepath.Join(goRepo, ".git", "hooks", "pre-commit")), "legacy")
		addFSResult("uninstall", "legacy hook restored", pyRestored == goRestored,
			fmt.Sprintf("py=%v go=%v", pyRestored, goRestored))
	})
}

// ---------------------------------------------------------------------------
// Tests: run (exit codes + output + filesystem modifications)
// ---------------------------------------------------------------------------

func TestRun(t *testing.T) {
	pyBin := pythonPreCommit(t)

	t.Run("no config fails", func(t *testing.T) {
		tmp := t.TempDir()
		_, pyExit := runCmd(t, tmp, pyBin, "run", "--config", filepath.Join(tmp, "nope.yaml"))
		_, goExit := runCmd(t, tmp, goBinary, "run", "--config", filepath.Join(tmp, "nope.yaml"))
		addExitResult("run", "no config fails", pyExit, goExit,
			(pyExit != 0) && (goExit != 0), "")
	})

	t.Run("trailing whitespace is fixed", func(t *testing.T) {
		pyRepo := initTestRepo(t, standardCfg, "hello   \nworld\n")
		goRepo := initTestRepo(t, standardCfg, "hello   \nworld\n")

		pyOut, pyExit := runCmd(t, pyRepo, pyBin, "run", "--all-files", "--color=never")
		goOut, goExit := runCmd(t, goRepo, goBinary, "run", "--all-files", "--color=never")

		addExitResult("run", "trailing whitespace fails", pyExit, goExit,
			(pyExit != 0) == (goExit != 0), "")

		// Compare hook result statuses.
		pyHooks := extractHookResults(pyOut)
		goHooks := extractHookResults(goOut)
		addOutputResult("run", "hook count matches",
			len(pyHooks) == len(goHooks),
			fmt.Sprintf("py=%d go=%d", len(pyHooks), len(goHooks)))

		if len(pyHooks) == len(goHooks) {
			for i := range pyHooks {
				addOutputResult("run",
					fmt.Sprintf("hook %q status", pyHooks[i].Name),
					pyHooks[i].Status == goHooks[i].Status,
					fmt.Sprintf("py=%s go=%s", pyHooks[i].Status, goHooks[i].Status))
			}
		}

		// Verify FILESYSTEM: both should have fixed trailing whitespace in test.txt.
		pyFixed := readFile(filepath.Join(pyRepo, "test.txt"))
		goFixed := readFile(filepath.Join(goRepo, "test.txt"))
		addFSResult("run", "trailing whitespace removed from file",
			pyFixed == goFixed,
			fmt.Sprintf("py=%q go=%q", pyFixed, goFixed))

		// Both should produce "hello\nworld\n".
		expected := "hello\nworld\n"
		addFSResult("run", "file content matches expected",
			pyFixed == expected && goFixed == expected,
			fmt.Sprintf("expected=%q py=%q go=%q", expected, pyFixed, goFixed))
	})

	t.Run("clean repo passes", func(t *testing.T) {
		pyRepo := initTestRepo(t, standardCfg, "hello\n")
		goRepo := initTestRepo(t, standardCfg, "hello\n")

		_, pyExit := runCmd(t, pyRepo, pyBin, "run", "--all-files", "--color=never")
		_, goExit := runCmd(t, goRepo, goBinary, "run", "--all-files", "--color=never")
		addExitResult("run", "clean repo passes", pyExit, goExit, pyExit == goExit, "")

		// Files should NOT be modified.
		pyContent := readFile(filepath.Join(pyRepo, "test.txt"))
		goContent := readFile(filepath.Join(goRepo, "test.txt"))
		addFSResult("run", "clean files unmodified",
			pyContent == "hello\n" && goContent == "hello\n", "")
	})

	t.Run("specific hook-id", func(t *testing.T) {
		pyRepo := initTestRepo(t, standardCfg, "hello   \n")
		goRepo := initTestRepo(t, standardCfg, "hello   \n")

		pyOut, pyExit := runCmd(t, pyRepo, pyBin, "run", "trailing-whitespace", "--all-files", "--color=never")
		goOut, goExit := runCmd(t, goRepo, goBinary, "run", "trailing-whitespace", "--all-files", "--color=never")
		addExitResult("run", "specific hook-id fails", pyExit, goExit,
			(pyExit != 0) == (goExit != 0), "")

		pyHooks := extractHookResults(pyOut)
		goHooks := extractHookResults(goOut)
		addOutputResult("run", "only 1 hook runs",
			len(pyHooks) == 1 && len(goHooks) == 1,
			fmt.Sprintf("py=%d go=%d", len(pyHooks), len(goHooks)))
	})

	t.Run("verbose flag", func(t *testing.T) {
		pyRepo := initTestRepo(t, standardCfg, "hello\n")
		goRepo := initTestRepo(t, standardCfg, "hello\n")

		pyOut, pyExit := runCmd(t, pyRepo, pyBin, "run", "check-yaml", "--all-files", "--color=never", "--verbose")
		goOut, goExit := runCmd(t, goRepo, goBinary, "run", "check-yaml", "--all-files", "--color=never", "--verbose")
		addExitResult("run", "verbose exits 0", pyExit, goExit, pyExit == goExit, "")

		pyVerbose := strings.Contains(pyOut, "hook id:")
		goVerbose := strings.Contains(goOut, "hook id:")
		addOutputResult("run", "verbose shows hook id", pyVerbose == goVerbose,
			fmt.Sprintf("py=%v go=%v", pyVerbose, goVerbose))
	})

	t.Run("nonexistent hook-id fails", func(t *testing.T) {
		pyRepo := initTestRepo(t, standardCfg, "hello\n")
		goRepo := initTestRepo(t, standardCfg, "hello\n")

		_, pyExit := runCmd(t, pyRepo, pyBin, "run", "nonexistent-hook", "--all-files", "--color=never")
		_, goExit := runCmd(t, goRepo, goBinary, "run", "nonexistent-hook", "--all-files", "--color=never")
		addExitResult("run", "nonexistent hook-id fails", pyExit, goExit,
			(pyExit != 0) && (goExit != 0), "")
	})

	t.Run("files flag", func(t *testing.T) {
		pyRepo := initTestRepo(t, standardCfg, "hello   \n")
		goRepo := initTestRepo(t, standardCfg, "hello   \n")

		_, pyExit := runCmd(t, pyRepo, pyBin, "run", "--files", "test.txt", "--color=never")
		_, goExit := runCmd(t, goRepo, goBinary, "run", "--files", "test.txt", "--color=never")
		addExitResult("run", "--files flag", pyExit, goExit,
			(pyExit != 0) == (goExit != 0), "")

		// Both should fix the file.
		pyFixed := readFile(filepath.Join(pyRepo, "test.txt"))
		goFixed := readFile(filepath.Join(goRepo, "test.txt"))
		addFSResult("run", "--files: file modified identically",
			pyFixed == goFixed,
			fmt.Sprintf("py=%q go=%q", pyFixed, goFixed))
	})

	t.Run("end-of-file-fixer adds newline", func(t *testing.T) {
		cfg := `repos:
-   repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v5.0.0
    hooks:
    -   id: end-of-file-fixer
`
		pyRepo := initTestRepo(t, cfg, "no newline at end")
		goRepo := initTestRepo(t, cfg, "no newline at end")

		_, pyExit := runCmd(t, pyRepo, pyBin, "run", "--all-files", "--color=never")
		_, goExit := runCmd(t, goRepo, goBinary, "run", "--all-files", "--color=never")
		addExitResult("run", "end-of-file-fixer exit agreement", pyExit, goExit,
			(pyExit != 0) == (goExit != 0), "")

		pyContent := readFile(filepath.Join(pyRepo, "test.txt"))
		goContent := readFile(filepath.Join(goRepo, "test.txt"))
		addFSResult("run", "end-of-file-fixer adds newline identically",
			pyContent == goContent,
			fmt.Sprintf("py=%q go=%q", pyContent, goContent))

		expected := "no newline at end\n"
		addFSResult("run", "end-of-file content correct",
			pyContent == expected && goContent == expected,
			fmt.Sprintf("expected=%q", expected))
	})
}

// ---------------------------------------------------------------------------
// Tests: migrate-config (exit codes + filesystem)
// ---------------------------------------------------------------------------

func TestMigrateConfig(t *testing.T) {
	pyBin := pythonPreCommit(t)

	t.Run("already up to date", func(t *testing.T) {
		pyRepo := initTestRepo(t, standardCfg, "test\n")
		goRepo := initTestRepo(t, standardCfg, "test\n")

		pyOut, pyExit := runCmd(t, pyRepo, pyBin, "migrate-config")
		goOut, goExit := runCmd(t, goRepo, goBinary, "migrate-config")
		addExitResult("migrate-config", "up-to-date exits 0", pyExit, goExit, pyExit == goExit, "")

		pyLower := strings.ToLower(pyOut)
		goLower := strings.ToLower(goOut)
		pyOK := strings.Contains(pyLower, "already") || strings.Contains(pyLower, "migrated")
		goOK := strings.Contains(goLower, "already") || strings.Contains(goLower, "up to date")
		addOutputResult("migrate-config", "reports no changes needed", pyOK && goOK,
			fmt.Sprintf("py=%q go=%q", strings.TrimSpace(pyOut), strings.TrimSpace(goOut)))

		// Config should be unchanged.
		pyCfg := readFile(filepath.Join(pyRepo, ".pre-commit-config.yaml"))
		goCfg := readFile(filepath.Join(goRepo, ".pre-commit-config.yaml"))
		addFSResult("migrate-config", "config unchanged", pyCfg == goCfg, "")
	})

	t.Run("migrates sha to rev", func(t *testing.T) {
		oldCfg := `repos:
-   repo: https://github.com/pre-commit/pre-commit-hooks
    sha: v5.0.0
    hooks:
    -   id: trailing-whitespace
`
		pyRepo := initTestRepo(t, oldCfg, "test\n")
		goRepo := initTestRepo(t, oldCfg, "test\n")

		_, pyExit := runCmd(t, pyRepo, pyBin, "migrate-config")
		_, goExit := runCmd(t, goRepo, goBinary, "migrate-config")
		addExitResult("migrate-config", "sha->rev exits 0", pyExit, goExit, pyExit == goExit, "")

		pyCfg := readFile(filepath.Join(pyRepo, ".pre-commit-config.yaml"))
		goCfg := readFile(filepath.Join(goRepo, ".pre-commit-config.yaml"))

		pyHasRev := strings.Contains(pyCfg, "rev:") && !strings.Contains(pyCfg, "sha:")
		goHasRev := strings.Contains(goCfg, "rev:") && !strings.Contains(goCfg, "sha:")
		addFSResult("migrate-config", "sha replaced with rev", pyHasRev && goHasRev,
			fmt.Sprintf("py=%v go=%v", pyHasRev, goHasRev))
	})
}

// ---------------------------------------------------------------------------
// Tests: init-templatedir (exit codes + filesystem)
// ---------------------------------------------------------------------------

func TestInitTemplateDir(t *testing.T) {
	pyBin := pythonPreCommit(t)

	t.Run("creates hook in template dir", func(t *testing.T) {
		pyRepo := initTestRepo(t, standardCfg, "test\n")
		goRepo := initTestRepo(t, standardCfg, "test\n")

		pyTmpl := filepath.Join(pyRepo, "tmpl")
		goTmpl := filepath.Join(goRepo, "tmpl")

		_, pyExit := runCmd(t, pyRepo, pyBin, "init-templatedir", pyTmpl)
		_, goExit := runCmd(t, goRepo, goBinary, "init-templatedir", goTmpl)
		addExitResult("init-templatedir", "exits 0", pyExit, goExit, pyExit == goExit, "")

		pyHookPath := filepath.Join(pyTmpl, "hooks", "pre-commit")
		goHookPath := filepath.Join(goTmpl, "hooks", "pre-commit")

		pyExists := fileExists(pyHookPath)
		goExists := fileExists(goHookPath)
		addFSResult("init-templatedir", "hook file created", pyExists && goExists,
			fmt.Sprintf("py=%v go=%v", pyExists, goExists))

		// Both should be executable.
		pyInfo, _ := os.Stat(pyHookPath)
		goInfo, _ := os.Stat(goHookPath)
		pyExec := pyInfo != nil && pyInfo.Mode()&0o111 != 0
		goExec := goInfo != nil && goInfo.Mode()&0o111 != 0
		addFSResult("init-templatedir", "hook file is executable", pyExec && goExec,
			fmt.Sprintf("py=%v go=%v", pyExec, goExec))

		// Both should contain hook-impl.
		pyContent := readFile(pyHookPath)
		goContent := readFile(goHookPath)
		pyHasImpl := strings.Contains(pyContent, "hook-impl")
		goHasImpl := strings.Contains(goContent, "hook-impl")
		addFSResult("init-templatedir", "hook contains hook-impl", pyHasImpl && goHasImpl,
			fmt.Sprintf("py=%v go=%v", pyHasImpl, goHasImpl))
	})
}

// ---------------------------------------------------------------------------
// Tests: autoupdate (exit codes + filesystem)
// ---------------------------------------------------------------------------

func TestAutoupdate(t *testing.T) {
	pyBin := pythonPreCommit(t)

	t.Run("updates config", func(t *testing.T) {
		pyRepo := initTestRepo(t, standardCfg, "test\n")
		goRepo := initTestRepo(t, standardCfg, "test\n")

		pyOut, pyExit := runCmd(t, pyRepo, pyBin, "autoupdate", "--color=never")
		goOut, goExit := runCmd(t, goRepo, goBinary, "autoupdate", "--color=never")
		addExitResult("autoupdate", "exits 0", pyExit, goExit, pyExit == goExit, "")

		pyLower := strings.ToLower(pyOut)
		goLower := strings.ToLower(goOut)
		pyOK := strings.Contains(pyLower, "updating") || strings.Contains(pyLower, "up to date")
		goOK := strings.Contains(goLower, "updating") || strings.Contains(goLower, "up to date")
		addOutputResult("autoupdate", "produces update output", pyOK && goOK,
			fmt.Sprintf("py=%v go=%v", pyOK, goOK))

		// Both configs should have the same rev after update.
		pyCfg := readFile(filepath.Join(pyRepo, ".pre-commit-config.yaml"))
		goCfg := readFile(filepath.Join(goRepo, ".pre-commit-config.yaml"))

		pyRev := extractRev(pyCfg)
		goRev := extractRev(goCfg)
		addFSResult("autoupdate", "configs have same rev after update",
			pyRev == goRev,
			fmt.Sprintf("py=%s go=%s", pyRev, goRev))
	})
}

func extractRev(cfg string) string {
	for _, line := range strings.Split(cfg, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "rev:") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "rev:"))
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Tests: try-repo
// ---------------------------------------------------------------------------

func TestTryRepo(t *testing.T) {
	pyBin := pythonPreCommit(t)

	t.Run("no args fails", func(t *testing.T) {
		repo := initTestRepo(t, standardCfg, "test\n")
		_, pyExit := runCmd(t, repo, pyBin, "try-repo")
		_, goExit := runCmd(t, repo, goBinary, "try-repo")
		addExitResult("try-repo", "no args fails", pyExit, goExit,
			(pyExit != 0) && (goExit != 0), "")
	})
}

// ---------------------------------------------------------------------------
// Tests: install-hooks
// ---------------------------------------------------------------------------

func TestInstallHooks(t *testing.T) {
	pyBin := pythonPreCommit(t)

	t.Run("exit code", func(t *testing.T) {
		pyRepo := initTestRepo(t, standardCfg, "test\n")
		goRepo := initTestRepo(t, standardCfg, "test\n")

		_, pyExit := runCmd(t, pyRepo, pyBin, "install-hooks")
		_, goExit := runCmd(t, goRepo, goBinary, "install-hooks")
		addExitResult("install-hooks", "exits 0", pyExit, goExit, pyExit == goExit, "")
	})
}
