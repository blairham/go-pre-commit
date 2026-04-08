package cli

import (
	"fmt"
	"os"
	"strings"

	flags "github.com/jessevdk/go-flags"
)

// MigrateConfigCommand implements the "migrate-config" command.
type MigrateConfigCommand struct {
	Meta *Meta
}

func (c *MigrateConfigCommand) Run(args []string) int {
	var opts GlobalFlags
	_, err := flags.ParseArgs(&opts, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	content, err := os.ReadFile(opts.Config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to read config: %v\n", err)
		return 1
	}

	raw := string(content)
	migrated := false

	// Replace old-style sha: with rev:
	if strings.Contains(raw, "\n    sha:") || strings.Contains(raw, "\n  sha:") {
		raw = strings.ReplaceAll(raw, "\n    sha:", "\n    rev:")
		raw = strings.ReplaceAll(raw, "\n  sha:", "\n  rev:")
		migrated = true
	}

	// Migrate language: python_venv -> language: python
	if strings.Contains(raw, "language: python_venv") {
		raw = strings.ReplaceAll(raw, "language: python_venv", "language: python")
		migrated = true
	}

	// Migrate old stage names to new names.
	stageReplacements := map[string]string{
		"- commit":       "- pre-commit",
		"- push":         "- pre-push",
		"- merge-commit": "- pre-merge-commit",
	}

	for old, new_ := range stageReplacements {
		lines := strings.Split(raw, "\n")
		inStages := false
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "stages:") {
				inStages = true
				continue
			}
			if inStages {
				if strings.HasPrefix(trimmed, "- ") {
					if trimmed == old {
						lines[i] = strings.Replace(line, old, new_, 1)
						migrated = true
					}
				} else {
					inStages = false
				}
			}
		}
		raw = strings.Join(lines, "\n")
	}

	if migrated {
		if err := os.WriteFile(opts.Config, []byte(raw), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to write config: %v\n", err)
			return 1
		}
		fmt.Println("Configuration has been migrated.")
	} else {
		fmt.Println("Configuration is already up to date.")
	}

	return 0
}

func (c *MigrateConfigCommand) Help() string {
	return strings.TrimSpace(`
Usage: pre-commit migrate-config [options]

  Migrate a .pre-commit-config.yaml from the old list format to the current
  map format. Also handles sha: -> rev: migration, python_venv -> python
  language rename, and old stage name migrations.

Options:

  -c, --config=FILE   Path to alternate config file.
      --color=MODE    Whether to use color (auto, always, never).
`)
}

func (c *MigrateConfigCommand) Synopsis() string {
	return "Migrate list configuration to new map configuration"
}
