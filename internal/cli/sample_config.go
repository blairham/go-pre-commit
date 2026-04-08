package cli

import (
	"fmt"
	"strings"

	"github.com/blairham/go-pre-commit/internal/config"
)

// SampleConfigCommand implements the "sample-config" command.
type SampleConfigCommand struct {
	Meta *Meta
}

func (c *SampleConfigCommand) Run(args []string) int {
	fmt.Print(config.SampleConfig())
	return 0
}

func (c *SampleConfigCommand) Help() string {
	return strings.TrimSpace(`
Usage: pre-commit sample-config

  Print a sample .pre-commit-config.yaml to stdout.
`)
}

func (c *SampleConfigCommand) Synopsis() string {
	return "Produce a sample .pre-commit-config.yaml file"
}
