// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

// Package integration tests compare the Go pre-commit binary against the
// Python pre-commit tool to ensure CLI parity.
//
// These tests require:
//   - Python pre-commit installed and on PATH
//   - Go binary buildable from the repo root
//
// Run with: go test -v -tags=integration -timeout=600s ./test/integration/
//
//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
	failed := printParityReport()
	os.RemoveAll(tmp)

	// A recorded mismatch is a divergence from upstream, which is the one bug
	// class this project exists to prevent -- so it fails the run. Until this
	// existed the checks were only ever printed: every Go assertion passed
	// whatever the comparison said, so a regression lowered the number in the
	// report and left the job green. That is how `--files a b` shipped broken
	// under a 78/78 headline.
	if exitCode == 0 && failed > 0 {
		fmt.Fprintf(os.Stderr, "\n%d parity check(s) diverged from Python pre-commit; see the report above\n", failed)
		exitCode = 1
	}

	// A run that measured nothing passed every check it ran, which is how a
	// suite of skips renders as a green job. Under PARITY_REQUIRE that is the
	// failure it looks like everywhere else.
	if exitCode == 0 && len(parityReport.results) == 0 && os.Getenv("PARITY_REQUIRE") != "" {
		fmt.Fprintln(os.Stderr, "PARITY_REQUIRE is set but zero parity checks ran")
		exitCode = 1
	}
	os.Exit(exitCode)
}

// printParityReport writes the human-readable report and the JSON artifact,
// and returns the number of checks that did not match.
func printParityReport() int {
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
	if pythonVer != "" {
		fmt.Fprintf(w, "  Measured against: %s (%s)\n", pythonVer, pythonPath)
	} else {
		fmt.Fprintf(w, "  Measured against: NOTHING - no Python pre-commit was resolved\n")
	}
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
		Generated       string         `json:"generated"`
		MeasuredAgainst pythonInfo     `json:"measured_against"`
		Total           int            `json:"total"`
		Pass            int            `json:"pass"`
		Fail            int            `json:"fail"`
		Parity          string         `json:"parity"`
		Results         []parityResult `json:"results"`
	}
	report := jsonReport{
		Generated:       time.Now().Format(time.RFC3339),
		MeasuredAgainst: pythonInfo{Version: pythonVer},
		Total:           total,
		Pass:            pass,
		Fail:            fail,
		Parity:          fmt.Sprintf("%.1f%%", pct),
		Results:         parityReport.results,
	}
	data, _ := json.MarshalIndent(report, "", "  ")
	os.WriteFile(reportPath, data, 0o644)
	fmt.Fprintf(w, "\n  JSON report: %s\n", reportPath)

	return fail
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// parityTarget is the Python pre-commit minor line this project claims parity
// with, and it is the meaning of our own version number: v4.6.x means "behaves
// like Python pre-commit 4.6.x". Comparing against any other line measures
// something we are not claiming, so resolvePython refuses to do it.
const parityTarget = "4.6"

// pythonInfo describes the counterpart binary a parity run measured against.
// It goes into the JSON report because a parity percentage without the version
// it was taken against is not a claim anyone can check.
type pythonInfo struct {
	Version string `json:"version"`
}

var (
	pythonOnce sync.Once
	pythonPath string
	pythonVer  string
	pythonErr  error
)

// pythonPreCommit returns the path to the real Python pre-commit.
//
// The obvious implementation — LookPath("pre-commit") — is wrong here, and
// wrong in the most expensive way: this project installs a binary called
// pre-commit, so on any machine that has it, LookPath finds *us*. A parity
// suite that silently diffs the tool against an older build of itself reports
// a healthy-looking number that means nothing. (It did: the report committed
// before this change recorded py="pre-commit 4.5.0 (build v4.5.3)", which is
// this project's version format, not Python's.)
//
// So: walk every PATH entry rather than taking the first hit, and make each
// candidate prove it is Python by its --version output.
func pythonPreCommit(t *testing.T) string {
	t.Helper()
	pythonOnce.Do(resolvePython)
	if pythonErr != nil {
		// In CI the whole point of the job is this comparison, so a missing or
		// wrong counterpart is a failure. Locally it is a skip, because not
		// every contributor wants a Python toolchain — but a loud one.
		if os.Getenv("PARITY_REQUIRE") != "" {
			t.Fatalf("PARITY_REQUIRE is set and no usable Python pre-commit was found: %v", pythonErr)
		}
		t.Skipf("skipping parity check: %v", pythonErr)
	}
	return pythonPath
}

func resolvePython() {
	var tried []string

	check := func(path string) bool {
		out, err := exec.Command(path, "--version").CombinedOutput()
		if err != nil {
			tried = append(tried, fmt.Sprintf("%s (--version failed: %v)", path, err))
			return false
		}
		version := strings.TrimSpace(string(out))

		// Our own binary stamps build metadata onto the version line; Python's
		// prints "pre-commit X.Y.Z" and nothing else. That difference is the
		// only reliable way to tell the two apart by execution alone.
		if strings.Contains(version, "(build ") {
			tried = append(tried, fmt.Sprintf("%s (this project, not Python: %q)", path, version))
			return false
		}

		fields := strings.Fields(version)
		if len(fields) < 2 || fields[0] != "pre-commit" {
			tried = append(tried, fmt.Sprintf("%s (unrecognized --version output: %q)", path, version))
			return false
		}

		if !strings.HasPrefix(fields[1], parityTarget+".") && fields[1] != parityTarget {
			tried = append(tried, fmt.Sprintf("%s (version %s is not on the %s line this project claims parity with)",
				path, fields[1], parityTarget))
			return false
		}

		pythonPath, pythonVer = path, version
		return true
	}

	// An explicit override wins, and fails loudly if it is not what it claims —
	// silently falling back to a search would hide the operator's mistake.
	if override := os.Getenv("PARITY_PYTHON_PRE_COMMIT"); override != "" {
		if check(override) {
			return
		}
		pythonErr = fmt.Errorf("PARITY_PYTHON_PRE_COMMIT=%s is unusable: %s", override, strings.Join(tried, "; "))
		return
	}

	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, "pre-commit")
		if info, err := os.Stat(candidate); err != nil || info.IsDir() {
			continue
		}
		if check(candidate) {
			return
		}
	}

	detail := "no pre-commit found on PATH"
	if len(tried) > 0 {
		detail = "rejected every candidate: " + strings.Join(tried, "; ")
	}
	pythonErr = fmt.Errorf("Python pre-commit %s.x is required (set PARITY_PYTHON_PRE_COMMIT to point at it); %s",
		parityTarget, detail)
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

// runSplit is runCmd with stdout and stderr kept apart. runCmd merges them,
// which is why --version and --help going to the wrong stream (#81) went
// unseen.
func runSplit(t *testing.T, dir, name string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	var out, errb bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			exitCode = e.ExitCode()
		} else {
			t.Fatalf("failed to run %s %v: %v", name, args, err)
		}
	}
	return out.String(), errb.String(), exitCode
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

// Which stream each front-door command writes to is part of the contract:
// `v=$(pre-commit --version)` and `pre-commit --help | less` depend on it.
func TestOutputStreams(t *testing.T) {
	pyBin := pythonPreCommit(t)
	for _, args := range [][]string{{"--version"}, {"--help"}, {"run", "--help"}} {
		pyOut, pyErr, _ := runSplit(t, ".", pyBin, args...)
		goOut, goErr, _ := runSplit(t, ".", goBinary, args...)
		addOutputResult(strings.Join(args, " "), "writes to stdout, not stderr",
			pyOut != "" && pyErr == "" && goOut != "" && goErr == "",
			fmt.Sprintf("py stdout=%d stderr=%d, go stdout=%d stderr=%d", len(pyOut), len(pyErr), len(goOut), len(goErr)))
	}
	pyOut, pyErr, pyExit := runSplit(t, ".", pyBin, "no-such-command")
	goOut, goErr, goExit := runSplit(t, ".", goBinary, "no-such-command")
	addOutputResult("no-such-command", "fails, on stderr",
		pyExit != 0 && goExit != 0 && pyOut == "" && goOut == "" && pyErr != "" && goErr != "",
		fmt.Sprintf("py exit=%d stdout=%d, go exit=%d stdout=%d", pyExit, len(pyOut), goExit, len(goOut)))
}

// The run report is compared byte for byte: every status line, its width and
// postfix, the per-hook block, and which stream it goes to. Until this existed
// only the Passed/Failed words were compared, so a line one column too wide, a
// report on stderr, and missing stash messages all passed (#86). The only
// normalization is for values that legitimately differ run to run: the stash
// patch's path and a hook's duration.
func TestRunReport(t *testing.T) {
	pyBin := pythonPreCommit(t)

	reportCfg := `repos:
-   repo: local
    hooks:
    -   id: ok
        name: ok
        entry: "true"
        language: system
    -   id: fails-with-output
        name: fails with output
        entry: sh -c 'echo some output; echo more; exit 3'
        language: system
    -   id: modifies
        name: modifies a file
        entry: sh -c 'echo changed >> test.txt'
        language: system
        pass_filenames: false
    -   id: skipped-by-env
        name: skipped by SKIP
        entry: "true"
        language: system
    -   id: no-files
        name: matches no files
        entry: "true"
        language: system
        files: '\.nomatch$'
    -   id: missing-exe
        name: missing executable
        entry: no-such-executable-86
        language: system
`
	longCfg := `repos:
-   repo: local
    hooks:
    -   id: long
        name: a hook whose name is long enough to widen every status line in the report
        entry: "true"
        language: system
    -   id: short
        name: short
        entry: "true"
        language: system
`
	patchRe := regexp.MustCompile(`\S*patch\d+-\d+`)
	durationRe := regexp.MustCompile(`- duration: [0-9.]+s`)
	norm := func(s string) string {
		return durationRe.ReplaceAllString(patchRe.ReplaceAllString(s, "<patch>"), "- duration: <n>s")
	}

	cases := []struct {
		name, cfg string
		args      []string
		env       []string
		unstaged  bool
	}{
		{"every status and block", reportCfg, []string{"--all-files"}, []string{"SKIP=skipped-by-env"}, false},
		{"verbose", reportCfg, []string{"--all-files", "--verbose"}, []string{"SKIP=skipped-by-env"}, false},
		{"width follows the longest name", longCfg, []string{"--all-files"}, nil, false},
		{"unstaged changes are stashed and restored", longCfg, nil, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			var outs, errs [2]string
			for i, bin := range []string{pyBin, goBinary} {
				repo := initTestRepo(t, tc.cfg, "hello\n")
				if tc.unstaged {
					runCmd(t, repo, "git", "-c", "core.hooksPath=/dev/null", "-c", "commit.gpgsign=false", "commit", "-qm", "init")
					if err := os.WriteFile(filepath.Join(repo, "test.txt"), []byte("hello\nstaged\n"), 0o644); err != nil {
						t.Fatal(err)
					}
					runCmd(t, repo, "git", "add", "test.txt")
					if err := os.WriteFile(filepath.Join(repo, "test.txt"), []byte("hello\nstaged\nunstaged\n"), 0o644); err != nil {
						t.Fatal(err)
					}
				}
				cmd := exec.Command(bin, append([]string{"run", "--color=never"}, tc.args...)...)
				cmd.Dir = repo
				cmd.Env = append(append(os.Environ(), "PRE_COMMIT_HOME="+filepath.Join(home, fmt.Sprint(i))), tc.env...)
				var o, e bytes.Buffer
				cmd.Stdout, cmd.Stderr = &o, &e
				_ = cmd.Run()
				outs[i], errs[i] = norm(o.String()), norm(e.String())
				if tc.unstaged {
					got := readFile(filepath.Join(repo, "test.txt"))
					addFSResult("run", "unstaged change restored ("+[]string{"py", "go"}[i]+")",
						got == "hello\nstaged\nunstaged\n", fmt.Sprintf("%q", got))
				}
			}
			addOutputResult("run", tc.name+": stdout identical", outs[0] != "" && outs[0] == outs[1],
				fmt.Sprintf("\n--- python ---\n%s--- go ---\n%s", outs[0], outs[1]))
			addOutputResult("run", tc.name+": stderr identical", errs[0] == errs[1],
				fmt.Sprintf("\n--- python ---\n%s--- go ---\n%s", errs[0], errs[1]))
		})
	}
}

// Upstream runs `run` when given no arguments at all.
func TestNoArgumentsRuns(t *testing.T) {
	pyBin := pythonPreCommit(t)
	t.Setenv("PRE_COMMIT_HOME", t.TempDir())
	pyRepo := initTestRepo(t, standardCfg, "trailing   \n")
	goRepo := initTestRepo(t, standardCfg, "trailing   \n")
	pyOut, pyExit := runCmd(t, pyRepo, pyBin)
	goOut, goExit := runCmd(t, goRepo, goBinary)
	addExitResult("(no arguments)", "runs the hooks, which fail", pyExit, goExit, pyExit != 0 && pyExit == goExit, "")
	compareHookStatuses(t, "(no arguments)", pyOut, goOut, 3)
	addFSResult("(no arguments)", "the staged file was fixed",
		readFile(filepath.Join(pyRepo, "test.txt")) == "trailing\n" && readFile(filepath.Join(goRepo, "test.txt")) == "trailing\n",
		fmt.Sprintf("py=%q go=%q", readFile(filepath.Join(pyRepo, "test.txt")), readFile(filepath.Join(goRepo, "test.txt"))))
}

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

	// `--files a b c` is the ordinary way to scope a run, and upstream declares
	// the flag with argparse's nargs='*' so one flag swallows every following
	// path. This tool bound one value per occurrence, so the second path fell
	// through as a stray positional and the run died with "expected at most 1
	// argument" without touching a file. Nothing in this suite compared a
	// multi-path invocation, so 78/78 stayed green while the most common
	// non-trivial form of the flag was broken.
	t.Run("--files takes several paths", func(t *testing.T) {
		pyRepo := initTestRepo(t, standardCfg, "hello\n")
		goRepo := initTestRepo(t, standardCfg, "hello\n")

		for _, repo := range []string{pyRepo, goRepo} {
			for _, name := range []string{"a.txt", "b.txt"} {
				if err := os.WriteFile(filepath.Join(repo, name), []byte("x   \n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command("git", "add", ".")
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git add failed: %v\n%s", err, out)
			}
		}

		pyOut, pyExit := runCmd(t, pyRepo, pyBin,
			"run", "trailing-whitespace", "--color=never", "--files", "a.txt", "b.txt")
		goOut, goExit := runCmd(t, goRepo, goBinary,
			"run", "trailing-whitespace", "--color=never", "--files", "a.txt", "b.txt")

		addExitResult("run", "--files with several paths exits alike", pyExit, goExit,
			(pyExit != 0) == (goExit != 0),
			fmt.Sprintf("py=%d go=%d", pyExit, goExit))

		// The regression was a parse error, so assert the run actually happened
		// rather than only that the exit codes agree.
		addOutputResult("run", "--files with several paths does not fail to parse",
			!strings.Contains(goOut, "expected at most"),
			fmt.Sprintf("go output=%q", goOut))
		_ = pyOut

		// Both paths must be fixed. Fixing only the first is the exact shape of
		// the bug this guards.
		for _, name := range []string{"a.txt", "b.txt"} {
			py := readFile(filepath.Join(pyRepo, name))
			goC := readFile(filepath.Join(goRepo, name))
			addFSResult("run", fmt.Sprintf("--files fixed %s", name),
				py == goC && goC == "x\n",
				fmt.Sprintf("py=%q go=%q", py, goC))
		}
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
// Tests: repo: local hooks (environment provisioning)
// ---------------------------------------------------------------------------

// The README's two everyday forms: a plain `run` (staged files only), and a
// commit going through the hook `install` wrote.
func TestReadmeEverydayFlow(t *testing.T) {
	pyBin := pythonPreCommit(t)

	t.Run("run checks staged files only", func(t *testing.T) {
		t.Setenv("PRE_COMMIT_HOME", t.TempDir())
		pyRepo := initTestRepo(t, standardCfg, "staged   \n")
		goRepo := initTestRepo(t, standardCfg, "staged   \n")
		for _, dir := range []string{pyRepo, goRepo} {
			// Committed earlier with trailing whitespace, and not staged now:
			// a run over staged files must leave it alone.
			if err := os.WriteFile(filepath.Join(dir, "old.txt"), []byte("old   \n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		_, pyExit := runCmd(t, pyRepo, pyBin, "run", "trailing-whitespace", "--color=never")
		_, goExit := runCmd(t, goRepo, goBinary, "run", "trailing-whitespace", "--color=never")
		addExitResult("run", "staged-only run fails on the staged file", pyExit, goExit, pyExit != 0 && goExit != 0, "")
		addFSResult("run", "staged file fixed, unstaged file untouched",
			readFile(filepath.Join(pyRepo, "test.txt")) == "staged\n" && readFile(filepath.Join(goRepo, "test.txt")) == "staged\n" &&
				readFile(filepath.Join(pyRepo, "old.txt")) == "old   \n" && readFile(filepath.Join(goRepo, "old.txt")) == "old   \n",
			fmt.Sprintf("py test=%q old=%q, go test=%q old=%q",
				readFile(filepath.Join(pyRepo, "test.txt")), readFile(filepath.Join(pyRepo, "old.txt")),
				readFile(filepath.Join(goRepo, "test.txt")), readFile(filepath.Join(goRepo, "old.txt"))))
	})

	t.Run("commit runs the installed hook", func(t *testing.T) {
		t.Setenv("PRE_COMMIT_HOME", t.TempDir())
		// The hook `install` writes runs `pre-commit` by name, so the binary
		// under test must come first on PATH, or this would grade whichever
		// pre-commit the machine happens to have.
		t.Setenv("PATH", filepath.Dir(goBinary)+string(os.PathListSeparator)+os.Getenv("PATH"))
		pyRepo := initTestRepo(t, standardCfg, "commit me   \n")
		goRepo := initTestRepo(t, standardCfg, "commit me   \n")
		runCmd(t, pyRepo, pyBin, "install")
		runCmd(t, goRepo, goBinary, "install")
		pyOut, pyExit := runCmd(t, pyRepo, "git", "-c", "commit.gpgsign=false", "commit", "-m", "x")
		goOut, goExit := runCmd(t, goRepo, "git", "-c", "commit.gpgsign=false", "commit", "-m", "x")
		addExitResult("install", "commit is stopped by the hook", pyExit, goExit, pyExit != 0 && goExit != 0, "")
		compareHookStatuses(t, "install", pyOut, goOut, 3)
		addFSResult("install", "hook fixed the file during the commit",
			readFile(filepath.Join(pyRepo, "test.txt")) == "commit me\n" && readFile(filepath.Join(goRepo, "test.txt")) == "commit me\n",
			fmt.Sprintf("py=%q go=%q", readFile(filepath.Join(pyRepo, "test.txt")), readFile(filepath.Join(goRepo, "test.txt"))))
		// The grading needs to have run the binary under test.
		got, _ := runCmd(t, goRepo, "sh", "-c", "command -v pre-commit")
		addOutputResult("install", "hook resolved the binary under test",
			strings.TrimSpace(got) == goBinary, fmt.Sprintf("resolved %q", strings.TrimSpace(got)))
	})
}

func TestLocalHooks(t *testing.T) {
	pyBin := pythonPreCommit(t)

	t.Run("python additional_dependencies are installed", func(t *testing.T) {
		cfg := `repos:
-   repo: local
    hooks:
    -   id: dep-probe
        name: dep probe
        entry: python
        args: ["-c", "import yaml; print(yaml.__version__)"]
        language: python
        additional_dependencies: ["pyyaml>=6"]
        pass_filenames: false
        always_run: true
`
		pyRepo := initTestRepo(t, cfg, "hello\n")
		goRepo := initTestRepo(t, cfg, "hello\n")

		_, pyExit := runCmd(t, pyRepo, pyBin, "run", "--all-files", "--color=never")
		_, goExit := runCmd(t, goRepo, goBinary, "run", "--all-files", "--color=never")
		addExitResult("run", "local python hook gets additional_dependencies",
			pyExit, goExit, pyExit == goExit && goExit == 0, "")
	})

	t.Run("additional_dependencies without an environment is an error", func(t *testing.T) {
		cfg := `repos:
-   repo: local
    hooks:
    -   id: sys-probe
        name: sys probe
        entry: echo
        language: system
        additional_dependencies: ["pyyaml"]
        pass_filenames: false
        always_run: true
`
		pyRepo := initTestRepo(t, cfg, "hello\n")
		goRepo := initTestRepo(t, cfg, "hello\n")

		_, pyExit := runCmd(t, pyRepo, pyBin, "run", "--all-files", "--color=never")
		_, goExit := runCmd(t, goRepo, goBinary, "run", "--all-files", "--color=never")
		addExitResult("run", "additional_dependencies on a language with no environment fails",
			pyExit, goExit, (pyExit != 0) == (goExit != 0), "")
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

	// The README's own example. Only "no args fails" was here, so try-repo
	// against a remote found no hooks at all (#74) under a full green report.
	const hooksRepo = "https://github.com/pre-commit/pre-commit-hooks"

	tryBoth := func(t *testing.T, content string, args ...string) (pyOut, goOut string, pyExit, goExit int) {
		t.Helper()
		t.Setenv("PRE_COMMIT_HOME", t.TempDir())
		pyRepo := initTestRepoFile(t, "test.yaml", content)
		goRepo := initTestRepoFile(t, "test.yaml", content)
		full := append([]string{"try-repo", hooksRepo}, args...)
		full = append(full, "--all-files", "--color=never")
		pyOut, pyExit = runCmd(t, pyRepo, pyBin, full...)
		goOut, goExit = runCmd(t, goRepo, goBinary, full...)
		return
	}

	t.Run("remote repo, named hook, valid input", func(t *testing.T) {
		pyOut, goOut, pyExit, goExit := tryBoth(t, "a: 1\n", "check-yaml")
		addExitResult("try-repo", "remote named hook passes", pyExit, goExit, pyExit == 0 && goExit == 0, "")
		compareHookStatuses(t, "try-repo", pyOut, goOut, 1)
		addOutputResult("try-repo", "same generated config",
			tryRepoConfig(pyOut) != "" && tryRepoConfig(pyOut) == tryRepoConfig(goOut),
			fmt.Sprintf("py=%q go=%q", tryRepoConfig(pyOut), tryRepoConfig(goOut)))
	})

	t.Run("remote repo, named hook, invalid input", func(t *testing.T) {
		pyOut, goOut, pyExit, goExit := tryBoth(t, "a: [\n", "check-yaml")
		addExitResult("try-repo", "remote named hook fails on bad input", pyExit, goExit, pyExit != 0 && goExit != 0, "")
		compareHookStatuses(t, "try-repo", pyOut, goOut, 1)
	})

	t.Run("remote repo, no hook id", func(t *testing.T) {
		pyOut, goOut, _, _ := tryBoth(t, "a: 1\n")
		pyCfg, goCfg := tryRepoConfig(pyOut), tryRepoConfig(goOut)
		addOutputResult("try-repo", "every manifest hook is listed, same rev",
			strings.Count(pyCfg, "-   id: ") > 1 && pyCfg == goCfg,
			fmt.Sprintf("py=%d go=%d hooks", strings.Count(pyCfg, "-   id: "), strings.Count(goCfg, "-   id: ")))
	})

	t.Run("--ref pins the rev", func(t *testing.T) {
		pyOut, goOut, pyExit, goExit := tryBoth(t, "a: 1\n", "check-yaml", "--ref", "v5.0.0")
		addExitResult("try-repo", "--ref run passes", pyExit, goExit, pyExit == 0 && goExit == 0, "")
		addOutputResult("try-repo", "--ref appears as the rev",
			strings.Contains(tryRepoConfig(pyOut), "rev: v5.0.0") && tryRepoConfig(pyOut) == tryRepoConfig(goOut),
			fmt.Sprintf("py=%q go=%q", tryRepoConfig(pyOut), tryRepoConfig(goOut)))
	})

	t.Run("local repo path", func(t *testing.T) {
		t.Setenv("PRE_COMMIT_HOME", t.TempDir())
		hooks := t.TempDir()
		for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"}, {"config", "commit.gpgsign", "false"}} {
			runCmd(t, hooks, "git", args...)
		}
		manifest := "-   id: no-todo\n    name: no TODO\n    entry: TODO\n    language: pygrep\n"
		if err := os.WriteFile(filepath.Join(hooks, ".pre-commit-hooks.yaml"), []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
		runCmd(t, hooks, "git", "add", "-A")
		runCmd(t, hooks, "git", "commit", "-qm", "hooks")

		pyRepo := initTestRepo(t, "", "TODO: fix\n")
		goRepo := initTestRepo(t, "", "TODO: fix\n")
		pyOut, pyExit := runCmd(t, pyRepo, pyBin, "try-repo", hooks, "no-todo", "--all-files", "--color=never")
		goOut, goExit := runCmd(t, goRepo, goBinary, "try-repo", hooks, "no-todo", "--all-files", "--color=never")
		addExitResult("try-repo", "local path hook rejects its input", pyExit, goExit, pyExit != 0 && goExit != 0, "")
		compareHookStatuses(t, "try-repo", pyOut, goOut, 1)
	})
}

// initTestRepoFile is initTestRepo with a named file, for hooks that select by
// file type (check-yaml ignores test.txt).
func initTestRepoFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := initTestRepo(t, "", "")
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	runCmd(t, dir, "git", "add", "-A")
	return dir
}

// compareHookStatuses records that both tools ran want hooks with the same
// statuses. A run with no hook lines matches trivially, so want guards it.
func compareHookStatuses(t *testing.T, cmd, pyOut, goOut string, want int) {
	t.Helper()
	pyHooks, goHooks := extractHookResults(pyOut), extractHookResults(goOut)
	ok := len(pyHooks) == want && len(goHooks) == want
	for i := 0; ok && i < want; i++ {
		ok = pyHooks[i].Status == goHooks[i].Status
	}
	addOutputResult(cmd, fmt.Sprintf("%d hook(s) ran with matching status", want), ok,
		fmt.Sprintf("py=%v go=%v", pyHooks, goHooks))
}

// tryRepoConfig extracts the "Using config:" block try-repo prints, so the
// generated config can be compared byte for byte.
func tryRepoConfig(out string) string {
	rule := strings.Repeat("=", 79)
	parts := strings.Split(out, rule+"\n")
	// rule, "Using config:", rule, <config>, rule
	for i := 0; i+2 < len(parts); i++ {
		if parts[i] == "Using config:\n" {
			return parts[i+1]
		}
	}
	return ""
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
