package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	flags "github.com/jessevdk/go-flags"
)

// InitTemplateDirCommand implements the "init-templatedir" command.
type InitTemplateDirCommand struct {
	Meta *Meta
}

type initTemplateDirFlags struct {
	GlobalFlags
	HookTypes      []string `short:"t" long:"hook-type" description:"Which hook type to install. May be specified multiple times. (default: pre-commit)"`
	NoAllowMissing bool     `long:"no-allow-missing-config" description:"Assume cloned repos should have a pre-commit config."`
}

func (c *InitTemplateDirCommand) Run(args []string) int {
	var opts initTemplateDirFlags
	remaining, err := flags.ParseArgs(&opts, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	if len(remaining) != 1 {
		fmt.Fprintf(os.Stderr, "Error: expected exactly 1 argument (directory), got %d\n", len(remaining))
		return 1
	}
	templateDir := remaining[0]

	typesToInstall := opts.HookTypes
	if len(typesToInstall) == 0 {
		typesToInstall = []string{"pre-commit"}
	}
	for _, ht := range typesToInstall {
		if _, ok := hookTypes[ht]; !ok {
			fmt.Fprintf(os.Stderr, "Error: unknown hook type: %s. Choose from: %s\n", ht, strings.Join(sortedHookTypes(), ", "))
			return 1
		}
	}

	hooksDir := filepath.Join(templateDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to create hooks directory: %v\n", err)
		return 1
	}

	for _, ht := range typesToInstall {
		hookFile := filepath.Join(hooksDir, ht)
		installID := "pre-commit-" + ht
		content := fmt.Sprintf(hookTemplate, installID, opts.Config, ht)

		if err := os.WriteFile(hookFile, []byte(content), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to write hook: %v\n", err)
			return 1
		}

		fmt.Printf("pre-commit installed at %s\n", hookFile)
	}

	if opts.NoAllowMissing {
		if _, err := os.Stat(opts.Config); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr,
				"Warning: config file %s not found.\n",
				opts.Config,
			)
		}
	}

	return 0
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
