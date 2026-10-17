// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package languages

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// R is upstream's pre_commit/languages/r.py (4.7.0): hooks run through
// Rscript inside a renv project built from the hook repo's renv.lock.
type R struct{}

// Name is the language's name in a hook manifest.
func (r *R) Name() string { return "r" }

// EnvironmentDir is upstream's ENVIRONMENT_DIR.
func (r *R) EnvironmentDir() string { return "renv" }

// GetDefaultVersion is upstream's basic_get_default_version.
func (r *R) GetDefaultVersion() string { return "default" }

// rRenvActivatedOpts is upstream's _RENV_ACTIVATED_OPTS.
var rRenvActivatedOpts = []string{"--no-save", "--no-restore", "--no-site-file", "--no-environ"}

// rUnset is the part of upstream's get_env_patch that removes a variable.
var rUnset = []string{"RENV_PROJECT"}

// rEnvPatch is upstream's get_env_patch: R starts by sourcing the env's
// activate.R, which loads the renv project.
func rEnvPatch(envDir string) []string {
	return []string{"R_PROFILE_USER=" + filepath.Join(envDir, "activate.R")}
}

// rscriptExec is upstream's _rscript_exec: R_HOME's Rscript when R_HOME is
// set, else Rscript on PATH.
func rscriptExec() string {
	rHome, ok := os.LookupEnv("R_HOME")
	if !ok {
		return "Rscript"
	}
	name := "Rscript"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(rHome, "bin", name)
}

// rInlineSetup is upstream's _inline_r_setup: options R only takes once it
// has started, ahead of the code.
func rInlineSetup(code string) string {
	return "options(\n" +
		"    install.packages.compile.from.source = \"never\",\n" +
		"    pkgType = \"binary\"\n" +
		")\n" +
		"\n" + code
}

// executeR is upstream's _execute_r: code is written to a script file (never
// spliced into `-e`), run by Rscript with cliOpts and args inside the env,
// and its stdout returned without trailing newlines. A non-zero exit is an
// error, as cmd_output's check is.
func executeR(code, envDir, cwd string, cliOpts, args []string) (_ string, err error) {
	dir, err := os.MkdirTemp("", "pre-commit-r-")
	if err != nil {
		return "", err
	}
	defer func() { err = errors.Join(err, os.RemoveAll(dir)) }()
	script := filepath.Join(dir, "script.R")
	if writeErr := os.WriteFile(script, []byte(rInlineSetup(code)), 0o644); writeErr != nil {
		return "", writeErr
	}

	env := rEnvPatch(envDir)
	argv := append(append(append([]string{rscriptExec()}, cliOpts...), script), args...)
	bin, err := lookPathInEnv(argv[0], env)
	if err != nil {
		bin = argv[0]
	}
	cmd := exec.Command(bin, argv[1:]...)
	cmd.Dir = cwd
	cmd.Env = append(environWithout(os.Environ(), rUnset), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s failed: %w\nstdout:\n%s\nstderr:\n%s",
			strings.Join(argv, " "), err, stdout.String(), stderr.String())
	}
	return strings.TrimRight(stdout.String(), "\n"), nil
}

func executeRInRenv(code, envDir string, args ...string) (string, error) {
	return executeR(code, envDir, envDir, rRenvActivatedOpts, args)
}

// HealthCheck is upstream r.health_check: the R version the env was
// installed for must be the version of the R that runs now.
func (r *R) HealthCheck(prefix, version string) error {
	envDir := filepath.Join(prefix, r.EnvironmentDir()+"-"+version)
	installed, err := executeRInRenv("cat(renv::settings$r.version())", envDir)
	if err != nil {
		return err
	}
	current, err := executeRInRenv("cat(as.character(getRversion()))", envDir)
	if err != nil {
		return err
	}
	switch {
	case installed == "NULL" || installed == "":
		return fmt.Errorf( //nolint:staticcheck // upstream's wording
			"Hooks were installed with an unknown R version. R version for hook repo now set to %s",
			current,
		)
	case installed != current:
		return fmt.Errorf( //nolint:staticcheck // upstream's wording
			"Hooks were installed for R version %s, but current R executable has version %s",
			installed,
			current,
		)
	}
	return nil
}

// rInstallScript is upstream's r_code_inst_environment. The hook repo's path
// arrives as the script's argument rather than spliced into the source.
const rInstallScript = `prefix_dir <- commandArgs(trailingOnly = TRUE)[1]
options(
    repos = c(CRAN = "https://cran.rstudio.com"),
    renv.consent = TRUE
)
source("renv/activate.R")
renv::restore()
activate_statement <- paste0(
  'suppressWarnings({',
  'old <- setwd("', getwd(), '"); ',
  'source("renv/activate.R"); ',
  'setwd(old); ',
  'renv::load("', getwd(), '");})'
)
writeLines(activate_statement, 'activate.R')
is_package <- tryCatch(
  {
      path_desc <- file.path(prefix_dir, 'DESCRIPTION')
      suppressWarnings(desc <- read.dcf(path_desc))
      "Package" %in% colnames(desc)
  },
  error = function(...) FALSE
)
if (is_package) {
    renv::install(prefix_dir)
}
`

// InstallEnvironment is upstream r.install_environment: the hook repo's
// renv.lock and renv/ are copied into the env, renv restores the lockfile,
// writes the env's activate.R, installs the hook repo itself when it is an R
// package, records the R version, and installs the additional dependencies.
func (r *R) InstallEnvironment(prefix, version string, additionalDeps []string) error {
	if version != "default" {
		return fmt.Errorf(
			"for now, pre-commit requires system-installed r -- you selected `language_version: %s`",
			version,
		)
	}
	envDir := filepath.Join(prefix, r.EnvironmentDir()+"-"+version)
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		return err
	}
	if err := copyFile(filepath.Join(prefix, "renv.lock"), filepath.Join(envDir, "renv.lock")); err != nil {
		return err
	}
	if err := copyTree(filepath.Join(prefix, "renv"), filepath.Join(envDir, "renv")); err != nil {
		return err
	}

	if _, err := executeR(rInstallScript, envDir, envDir, []string{"--vanilla"}, []string{prefix}); err != nil {
		return err
	}
	if _, err := executeRInRenv("renv::settings$r.version(as.character(getRversion()))", envDir); err != nil {
		return err
	}
	if len(additionalDeps) > 0 {
		if _, err := executeRInRenv(
			"renv::install(commandArgs(trailingOnly = TRUE))",
			envDir,
			additionalDeps...,
		); err != nil {
			return err
		}
	}
	return nil
}

// copyFile is shutil.copy: contents and mode.
func copyFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, info.Mode().Perm())
}

// copyTree is shutil.copytree: dst must not exist yet.
func copyTree(src, dst string) error {
	if _, err := os.Lstat(dst); err == nil {
		return fmt.Errorf("[Errno 17] File exists: '%s'", dst)
	}
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0o755)
		case d.Type()&fs.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		default:
			return copyFile(path, target)
		}
	})
}

// rEntryValidate is upstream's _entry_validate: `Rscript -e <expr>` or
// `Rscript path/to/file`, nothing else.
func rEntryValidate(entry []string) error {
	if len(entry) == 0 || entry[0] != "Rscript" {
		return errors.New("entry must start with `Rscript`.") //nolint:staticcheck,revive // upstream's wording
	}
	if len(entry) < 2 {
		// upstream indexes entry[1] and raises IndexError.
		return errors.New("list index out of range")
	}
	if entry[1] == "-e" {
		if len(entry) > 3 {
			return errors.New("You can supply at most one expression.") //nolint:staticcheck,revive // upstream's wording
		}
	} else if len(entry) > 2 {
		return errors.New( //nolint:staticcheck,revive // upstream's wording, missing space and all
			"The only valid syntax is `Rscript -e {expr}`or `Rscript path/to/hook/script`",
		)
	}
	return nil
}

// rCmdFromHook is upstream's _cmd_from_hook: Rscript with renv's options,
// then the expression, or the script — resolved against the hook repo
// unless the hook is local (_prefix_if_file_entry) — then the hook's args.
func rCmdFromHook(prefix, entry string, args []string, isLocal bool) ([]string, error) {
	cmd := ParseEntry(entry)
	if err := rEntryValidate(cmd); err != nil {
		return nil, err
	}
	part := cmd[1:]
	if cmd[1] != "-e" && !isLocal {
		script := cmd[1]
		if !filepath.IsAbs(script) {
			script = filepath.Join(prefix, script)
		}
		part = []string{script}
	}
	argv := append([]string{cmd[0]}, rRenvActivatedOpts...)
	argv = append(argv, part...)
	return append(argv, args...), nil
}

// Run is upstream's r.run_hook, inside in_env.
func (r *R) Run(
	ctx context.Context,
	prefix, workDir, entry string,
	args, fileArgs []string,
	version string,
) (int, []byte, error) {
	argv, err := rCmdFromHook(prefix, entry, args, isLocalHook(ctx))
	if err != nil {
		return -1, nil, err
	}
	argv = append(argv, fileArgs...)
	envDir := filepath.Join(prefix, r.EnvironmentDir()+"-"+version)
	return runHookArgv(ctx, workDir, argv, rEnvPatch(envDir), rUnset)
}
