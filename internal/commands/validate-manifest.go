package commands

import (
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/jessevdk/go-flags"
	"github.com/mitchellh/cli"
	"gopkg.in/yaml.v3"
)

// ValidateManifestCommand handles the validate-manifest command functionality
type ValidateManifestCommand struct{}

// ValidateManifestOptions holds command-line options for the validate-manifest command
type ValidateManifestOptions struct {
	Verbose bool `short:"v" long:"verbose" description:"Verbose output"`
	Help    bool `short:"h" long:"help"    description:"Show this help message"`
}

// Hook represents a hook definition in the manifest
type Hook struct {
	ID           string   `yaml:"id"`
	Name         string   `yaml:"name,omitempty"`
	Entry        string   `yaml:"entry,omitempty"`
	Language     string   `yaml:"language,omitempty"`
	Files        string   `yaml:"files,omitempty"`
	ExcludeTypes []string `yaml:"exclude_types,omitempty"`
	Types        []string `yaml:"types,omitempty"`
	Args         []string `yaml:"args,omitempty"`
	Pass         bool     `yaml:"pass_filenames,omitempty"`
}

// Help returns the help text for the validate-manifest command
func (c *ValidateManifestCommand) Help() string {
	var opts ValidateManifestOptions
	parser := flags.NewParser(&opts, flags.Default)
	parser.Usage = "[FILENAMES...]"

	formatter := &HelpFormatter{
		Command:     "validate-manifest",
		Description: "Validate .pre-commit-hooks.yaml files.",
		Examples: []Example{
			{
				Command:     "pre-commit validate-manifest",
				Description: "Validate .pre-commit-hooks.yaml in current directory",
			},
			{
				Command:     "pre-commit validate-manifest hooks.yaml",
				Description: "Validate specific manifest file",
			},
			{
				Command:     "pre-commit validate-manifest --verbose",
				Description: "Show detailed validation output",
			},
		},
		Notes: []string{
			"positional arguments:",
			"  FILENAMES             manifest files to validate (default: .pre-commit-hooks.yaml)",
			"",
			"This command validates the manifest files that define hooks in repositories.",
			"These files describe the hooks available in a repository and their configuration.",
			"",
			"If no filenames are provided, it will look for .pre-commit-hooks.yaml in the",
			"current directory.",
			"",
			"The manifest file should contain a list of hooks with the following structure:",
			"  - id: hook-id",
			"    name: Human readable name",
			"    entry: Command to run",
			"    language: Language of the hook",
			"    files: File pattern to match (regex)",
			"    types: [file_types]",
		},
	}

	return formatter.FormatHelp(parser)
}

// Synopsis returns a short description of the validate-manifest command
func (c *ValidateManifestCommand) Synopsis() string {
	return "Validate .pre-commit-hooks.yaml files"
}

func (c *ValidateManifestCommand) validateHook(hook Hook) []string {
	var validationErrors []string

	// Required fields
	if hook.ID == "" {
		validationErrors = append(validationErrors, "hook must have an 'id'")
	}

	// Language validation
	validLanguages := []string{
		"conda", "coursier", "dart", "docker", "docker_image", "dotnet", "fail",
		"golang", "haskell", "lua", "node", "perl", "python", "python3", "r", "ruby",
		"rust", "swift", "system", "pygrep", "script",
	}

	if hook.Language != "" {
		valid := slices.Contains(validLanguages, hook.Language)
		if !valid {
			validationErrors = append(
				validationErrors,
				fmt.Sprintf("'%s' is not a valid language", hook.Language),
			)
		}
	}

	return validationErrors
}

func (c *ValidateManifestCommand) validateManifest(filename string, verbose bool) (bool, error) {
	file, err := os.Open(filename) // #nosec G304 -- tool validates user-specified manifest files
	if err != nil {
		return false, fmt.Errorf("error opening file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			fmt.Printf("⚠️  Warning: failed to close file: %v\n", closeErr)
		}
	}()

	var hooks []Hook
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&hooks); err != nil {
		return false, fmt.Errorf("error parsing YAML: %w", err)
	}

	if len(hooks) == 0 {
		// An empty manifest is valid - just means no hooks are defined yet
		fmt.Println("Manifest is valid (empty)")
		return true, nil
	}

	valid := true
	for i, hook := range hooks {
		hookErrors := c.validateHook(hook)
		if len(hookErrors) > 0 {
			valid = false
			fmt.Printf("Hook %d (%s):\n", i+1, hook.ID)
			for _, hookError := range hookErrors {
				fmt.Printf("  - %s\n", hookError)
			}
		} else if verbose {
			fmt.Printf("Hook %d (%s): OK\n", i+1, hook.ID)
		}
	}

	return valid, nil
}

// Run executes the validate-manifest command
func (c *ValidateManifestCommand) Run(args []string) int {
	var opts ValidateManifestOptions
	parser := flags.NewParser(&opts, flags.Default)
	parser.Usage = "[FILENAMES...]"

	remaining, err := parser.ParseArgs(args)
	if err != nil {
		var flagsErr *flags.Error
		if errors.As(err, &flagsErr) && flagsErr.Type == flags.ErrHelp {
			return 0
		}
		fmt.Printf("Error parsing arguments: %v\n", err)
		return 1
	}

	if opts.Help {
		fmt.Print(c.Help())
		return 0
	}

	// Default to .pre-commit-hooks.yaml if no files specified
	filenames := remaining
	if len(filenames) == 0 {
		filenames = []string{".pre-commit-hooks.yaml"}
	}

	allValid := true
	for _, filename := range filenames {
		if opts.Verbose {
			fmt.Printf("Validating %s...\n", filename)
		}

		valid, err := c.validateManifest(filename, opts.Verbose)
		if err != nil {
			fmt.Printf("Error validating %s: %v\n", filename, err)
			allValid = false
			continue
		}

		if !valid {
			allValid = false
		} else if opts.Verbose {
			fmt.Printf("%s: All hooks are valid\n", filename)
		}
	}

	if allValid {
		if opts.Verbose {
			fmt.Println("All manifest files are valid!")
		}
		return 0
	}

	return 1
}

// ValidateManifestCommandFactory creates a new validate-manifest command instance
func ValidateManifestCommandFactory() (cli.Command, error) {
	return &ValidateManifestCommand{}, nil
}
