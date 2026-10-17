// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/blairham/go-pre-commit/v4/internal/output"
)

// InitTemplateDirCommand implements the "init-templatedir" command.
type InitTemplateDirCommand struct {
	Meta *Meta
}

type initTemplateDirFlags struct {
	GlobalFlags
	HookTypes      []string `short:"t" long:"hook-type"               description:"Which hook type to install. May be specified multiple times. (default: pre-commit)"`
	NoAllowMissing bool     `          long:"no-allow-missing-config" description:"Assume cloned repos should have a pre-commit config."`
}

func (c *InitTemplateDirCommand) Run(args []string) int {
	var opts initTemplateDirFlags
	remaining, code, ok := parseArgs("init-templatedir", &opts, args, args)
	if !ok {
		return code
	}
	templateDir := remaining[0]

	output.SetColorModeFromString(opts.Color)

	// Upstream's init_templatedir: install into the directory, overwriting
	// any legacy hook, then say so when git would not copy from it.
	if code := installHookScripts(opts.Config, opts.HookTypes, true, !opts.NoAllowMissing, templateDir); code != 0 {
		return code
	}

	dest := realpath(templateDir)
	if configured, ok := gitInitTemplateDir(); !ok || realpath(expandUser(configured)) != dest {
		output.Warn("`init.templateDir` not set to the target directory")
		output.Warn("maybe `git config --global init.templateDir %s`?", dest)
	}
	return 0
}

// gitInitTemplateDir is `git config init.templateDir`, from the current
// directory as upstream runs it; ok is false when git fails, as it does when
// the key is unset.
func gitInitTemplateDir() (string, bool) {
	out, err := exec.Command("git", "config", "init.templateDir").Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

// expandUser is os.path.expanduser for "~" and "~/...": the home directory
// from $HOME, the path unchanged otherwise.
func expandUser(p string) string {
	if p != "~" && !strings.HasPrefix(p, "~/") {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	return home + p[1:]
}

// realpath is os.path.realpath: an absolute path with every symlink resolved,
// where a part that does not exist is kept as written.
func realpath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	var rest []string
	for dir := abs; ; dir = filepath.Dir(dir) {
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			return filepath.Join(append([]string{resolved}, rest...)...)
		}
		if parent := filepath.Dir(dir); parent == dir {
			return abs
		}
		rest = append([]string{filepath.Base(dir)}, rest...)
	}
}

func (c *InitTemplateDirCommand) Help() string {
	return strings.TrimSpace(`
Usage: pre-commit init-templatedir [options] DIRECTORY

  Install hook script in a directory intended for use with
  git config init.templateDir.

Options:

  -t, --hook-type=TYPE              The hook type to install. May be repeated. (default: pre-commit)
      --no-allow-missing-config    Assume cloned repos should have a pre-commit config.
  -c, --config=FILE            Path to alternate config file.
      --color=MODE             Whether to use color (auto, always, never).
`)
}

func (c *InitTemplateDirCommand) Synopsis() string {
	return "Install hook script in a directory for use with git init.templateDir"
}
