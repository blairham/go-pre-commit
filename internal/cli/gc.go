package cli

import (
	"fmt"
	"os"
	"strings"

	flags "github.com/jessevdk/go-flags"

	"github.com/blairham/go-pre-commit/v4/internal/config"
	"github.com/blairham/go-pre-commit/v4/internal/store"
)

// GCCommand implements the "gc" command.
type GCCommand struct {
	Meta *Meta
}

func (c *GCCommand) Run(args []string) int {
	var opts GlobalFlags
	_, err := flags.ParseArgs(&opts, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	s := store.New("")

	// Gather used repos from all known config files.
	usedRepos := make(map[string]bool)

	// Always check the current config.
	configPaths := []string{opts.Config}

	// Also check any configs tracked by MarkConfigUsed.
	trackedConfigs, err := s.GetTrackedConfigs()
	if err == nil {
		for _, c := range trackedConfigs {
			// Deduplicate.
			found := false
			for _, existing := range configPaths {
				if existing == c {
					found = true
					break
				}
			}
			if !found {
				configPaths = append(configPaths, c)
			}
		}
	}

	for _, cfgPath := range configPaths {
		if cfg, err := config.LoadConfig(cfgPath); err == nil {
			for _, repo := range cfg.Repos {
				if !repo.IsLocal() && !repo.IsMeta() {
					usedRepos[repo.Repo+"@"+repo.Rev] = true
				}
			}
		}
	}

	if err := s.GC(usedRepos); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to run GC: %v\n", err)
		return 1
	}
	fmt.Println("Garbage collection complete.")
	return 0
}

func (c *GCCommand) Help() string {
	return strings.TrimSpace(`
Usage: pre-commit gc [options]

  Clean unused cached repos. Repos that are no longer referenced by any
  config file will be removed from the cache.

Options:

  -c, --config=FILE   Path to alternate config file.
      --color=MODE    Whether to use color (auto, always, never).
`)
}

func (c *GCCommand) Synopsis() string {
	return "Clean unused cached repos"
}
