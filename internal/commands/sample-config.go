package commands

import (
	"errors"
	"fmt"
	"os"

	"github.com/jessevdk/go-flags"
	"github.com/mitchellh/cli"
	"gopkg.in/yaml.v3"

	"github.com/blairham/go-pre-commit/pkg/config"
)

// SampleConfigCommand handles the sample-config command functionality
type SampleConfigCommand struct{}

// SampleConfigOptions holds command-line options for the sample-config command
type SampleConfigOptions struct {
	Force bool `short:"f" long:"force" description:"Overwrite existing configuration file"`
	Help  bool `short:"h" long:"help"  description:"Show this help message"`
}

// Help returns the help text for the sample-config command
func (c *SampleConfigCommand) Help() string {
	var opts SampleConfigOptions
	parser := flags.NewParser(&opts, flags.Default)
	parser.Usage = OptionsUsage

	formatter := &HelpFormatter{
		Command:     "sample-config",
		Description: "Generate a sample .pre-commit-config.yaml file.",
		Examples: []Example{
			{Command: "pre-commit sample-config", Description: "Generate sample config"},
			{Command: "pre-commit sample-config --force", Description: "Overwrite existing config"},
		},
		Notes: []string{
			"This creates a basic .pre-commit-config.yaml with common hooks.",
			"Use --force to overwrite an existing configuration file.",
		},
	}

	return formatter.FormatHelp(parser)
}

// Synopsis returns a short description of the sample-config command
func (c *SampleConfigCommand) Synopsis() string {
	return "Generate a sample configuration file"
}

// Run executes the sample-config command
func (c *SampleConfigCommand) Run(args []string) int {
	var opts SampleConfigOptions

	parser := flags.NewParser(&opts, flags.Default)
	parser.Usage = OptionsUsage

	_, err := parser.ParseArgs(args)
	if err != nil {
		var flagsErr *flags.Error
		if errors.As(err, &flagsErr) {
			if flagsErr.Type == flags.ErrHelp {
				return 0
			}
		}
		fmt.Printf("Error parsing flags: %v\n", err)
		return 1
	}

	configPath := config.ConfigFileName

	// Check if config already exists (unless force is specified)
	if _, statErr := os.Stat(configPath); statErr == nil && !opts.Force {
		fmt.Printf("Error: configuration file %s already exists\n", configPath)
		fmt.Println("Use --force to overwrite the existing file")
		return 1
	}

	// Generate default config
	cfg := config.DefaultConfig()

	// Marshal to YAML
	data, err := yaml.Marshal(cfg)
	if err != nil {
		fmt.Printf("Error: failed to marshal configuration: %v\n", err)
		return 1
	}

	// Write to file
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		fmt.Printf("Error: failed to write configuration file: %v\n", err)
		return 1
	}

	if opts.Force {
		fmt.Printf("Sample configuration written to %s (overwrote existing file)\n", configPath)
	} else {
		fmt.Printf("Sample configuration written to %s\n", configPath)
	}
	fmt.Println("Edit the file to customize your hooks, then run 'pre-commit install'")
	return 0
}

// SampleConfigCommandFactory creates a new sample-config command instance
func SampleConfigCommandFactory() (cli.Command, error) {
	return &SampleConfigCommand{}, nil
}
