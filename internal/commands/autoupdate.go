// Package commands implements the CLI commands for the pre-commit tool.
package commands

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"

	"github.com/jessevdk/go-flags"
	"github.com/mitchellh/cli"
	"gopkg.in/yaml.v3"

	"github.com/blairham/go-pre-commit/pkg/config"
	"github.com/blairham/go-pre-commit/pkg/git"
	"github.com/blairham/go-pre-commit/pkg/repository"
)

// AutoupdateCommand handles the autoupdate command functionality
type AutoupdateCommand struct{}

// AutoupdateOptions holds command-line options for the autoupdate command
type AutoupdateOptions struct {
	Color        string   `long:"color"         description:"Whether to use color in output"                                default:"auto"                    choice:"auto"`
	Config       string   `long:"config"        description:"Path to alternate config file"                                 default:".pre-commit-config.yaml"               short:"c"`
	Repo         []string `long:"repo"          description:"Only update this repository (may be specified multiple times)"`
	Jobs         int      `long:"jobs"          description:"Number of threads to use"                                      default:"1"                                     short:"j"`
	DryRun       bool     `long:"dry-run"       description:"Show what would be updated without making changes"                                                             short:"n"`
	BleedingEdge bool     `long:"bleeding-edge" description:"Update to bleeding edge of HEAD vs latest tag"`
	Freeze       bool     `long:"freeze"        description:"Store frozen hashes in rev instead of tag names"`
	Help         bool     `long:"help"          description:"Show this help message"                                                                                        short:"h"`
}

// Help returns the help text for the autoupdate command
func (c *AutoupdateCommand) Help() string {
	var opts AutoupdateOptions
	parser := flags.NewParser(&opts, flags.Default)
	parser.Usage = "[OPTIONS]"

	formatter := &HelpFormatter{
		Command:     "autoupdate",
		Description: "Auto-update hook repositories to the latest version.",
		Examples: []Example{
			{Command: "pre-commit autoupdate", Description: "Update all repositories"},
			{Command: "pre-commit autoupdate --dry-run", Description: "Show what would be updated"},
			{
				Command:     "pre-commit autoupdate --bleeding-edge",
				Description: "Use HEAD instead of latest tag",
			},
			{
				Command:     "pre-commit autoupdate --freeze",
				Description: "Use commit hashes instead of tags",
			},
			{
				Command:     "pre-commit autoupdate --repo https://github.com/psf/black",
				Description: "Update specific repo",
			},
			{Command: "pre-commit autoupdate --jobs 4", Description: "Use 4 parallel threads"},
		},
		Notes: []string{
			"This command will check for newer versions of the hooks in your",
			".pre-commit-config.yaml and update them to the latest available version.",
			"",
			"Options:",
			"  --bleeding-edge: Update to HEAD instead of latest tagged version",
			"  --freeze: Store frozen hashes in rev instead of tag names",
			"  --repo: Only update specific repository (can specify multiple times)",
			"  --jobs: Number of parallel threads for updates",
		},
	}

	return formatter.FormatHelp(parser)
}

// Synopsis returns a short description of the autoupdate command
func (c *AutoupdateCommand) Synopsis() string {
	return "Update hook repositories to latest versions"
}

// Helper functions to reduce cognitive complexity in AutoupdateCommand.Run

func (c *AutoupdateCommand) parseAndValidateArgs(args []string) (*AutoupdateOptions, int) {
	var opts AutoupdateOptions
	parser := flags.NewParser(&opts, flags.Default)
	parser.Usage = "[OPTIONS]"

	_, err := parser.ParseArgs(args)
	if err != nil {
		var flagsErr *flags.Error
		if errors.As(err, &flagsErr) {
			if flagsErr.Type == flags.ErrHelp {
				return &opts, 0
			}
		}
		fmt.Printf("Error parsing flags: %v\n", err)
		return &opts, 1
	}

	if opts.Help {
		fmt.Print(c.Help())
		return &opts, 0
	}

	return &opts, -1 // Continue processing
}

func (c *AutoupdateCommand) validateGitRepository() error {
	_, err := git.NewRepository("")
	if err != nil {
		return fmt.Errorf("not in a git repository: %w", err)
	}
	return nil
}

func (c *AutoupdateCommand) loadAndValidateConfig(configFile string) (*config.Config, error) {
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file not found: %s", configFile)
		}
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	return cfg, nil
}

func (c *AutoupdateCommand) initializeRepositoryManager(
	configFile string,
) (*repository.Manager, error) {
	repoManager, err := repository.NewManager()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize repository manager: %w", err)
	}

	// Mark this config as used in the database so gc knows it's active
	if err := repoManager.MarkConfigUsed(configFile); err != nil {
		fmt.Printf("⚠️  Warning: failed to mark config as used: %v\n", err)
	}

	return repoManager, nil
}

func (c *AutoupdateCommand) shouldUpdateRepo(repo *config.Repo, filterRepos []string) bool {
	// Skip local and meta repositories
	if repo.Repo == LocalRepo || repo.Repo == MetaRepo {
		return false
	}

	// Filter by specific repositories if specified
	if len(filterRepos) > 0 {
		return slices.Contains(filterRepos, repo.Repo)
	}

	return true
}

func (c *AutoupdateCommand) getLatestRevisionForRepo(
	repo *config.Repo,
	opts *AutoupdateOptions,
) (string, error) {
	var latestRev string
	var err error

	if opts.BleedingEdge {
		latestRev, err = c.getHeadRevision(repo.Repo)
	} else {
		latestRev, err = c.getLatestRevision(repo.Repo)
	}

	if err != nil {
		return "", err
	}

	// Convert to frozen hash if requested
	if opts.Freeze && !opts.BleedingEdge {
		if frozenRev, err := c.getCommitHash(repo.Repo, latestRev); err == nil {
			latestRev = frozenRev
		}
	}

	return latestRev, nil
}

func (c *AutoupdateCommand) updateRepositoryRevision(
	repo *config.Repo,
	latestRev string,
	opts *AutoupdateOptions,
) bool {
	if repo.Rev != latestRev {
		if opts.DryRun {
			fmt.Printf("Would update %s: %s -> %s\n", repo.Repo, repo.Rev, latestRev)
		} else {
			fmt.Printf("Updating %s: %s -> %s\n", repo.Repo, repo.Rev, latestRev)
			repo.Rev = latestRev
		}
		return true
	}

	fmt.Printf("Already up to date: %s (%s)\n", repo.Repo, repo.Rev)
	return false
}

func (c *AutoupdateCommand) printFinalStatus(updated int, opts *AutoupdateOptions) {
	switch {
	case updated == 0:
		fmt.Println("\nAll repositories are already up to date!")
	case opts.DryRun:
		fmt.Printf("\nDry run: %d repositories would be updated\n", updated)
	default:
		fmt.Printf("\nSuccessfully updated %d repositories\n", updated)
	}
}

// Run executes the autoupdate command
func (c *AutoupdateCommand) Run(args []string) int {
	// Parse and validate arguments
	opts, exitCode := c.parseAndValidateArgs(args)
	if exitCode != -1 {
		return exitCode
	}

	// Validate git repository
	if err := c.validateGitRepository(); err != nil {
		fmt.Printf("Error: %v\n", err)
		return 1
	}

	// Load and validate configuration
	cfg, err := c.loadAndValidateConfig(opts.Config)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return 1
	}

	// Initialize repository manager
	repoManager, err := c.initializeRepositoryManager(opts.Config)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return 1
	}
	defer func() {
		if closeErr := repoManager.Close(); closeErr != nil {
			fmt.Printf("⚠️  Warning: failed to close repository manager: %v\n", closeErr)
		}
	}()

	// Process repository updates
	updated, hasChanges := c.processRepositoryUpdates(cfg, opts)

	// Write updated configuration back to file
	if hasChanges && !opts.DryRun {
		if err := c.writeConfig(cfg, opts.Config); err != nil {
			fmt.Printf("Error: failed to write updated configuration: %v\n", err)
			return 1
		}
		fmt.Printf("\nUpdated configuration written to %s\n", opts.Config)
	}

	c.printFinalStatus(updated, opts)
	return 0
}

// getLatestRevision gets the latest tag/revision for a git repository
func (c *AutoupdateCommand) getLatestRevision(repoURL string) (string, error) {
	// Use git ls-remote to get the latest tag
	cmd := exec.Command("git", "ls-remote", "--tags", "--sort=-version:refname", repoURL)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to fetch remote tags: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 {
		return "", fmt.Errorf("no tags found")
	}

	// Find the latest version tag (skip pre-release versions)
	versionRegex := regexp.MustCompile(`refs/tags/(v?\d+\.\d+\.\d+)$`)
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			matches := versionRegex.FindStringSubmatch(parts[1])
			if len(matches) > 1 {
				return matches[1], nil
			}
		}
	}

	// If no version tags found, try to get the default branch
	cmd = exec.Command("git", "ls-remote", "--symref", repoURL, "HEAD")
	output, err = cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to fetch HEAD ref: %w", err)
	}

	lines = strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if after, ok := strings.CutPrefix(line, "ref: refs/heads/"); ok {
			branch := after
			return branch, nil
		}
	}

	return "main", nil // fallback to main
}

// getHeadRevision gets the HEAD commit hash for a repository
func (c *AutoupdateCommand) getHeadRevision(repoURL string) (string, error) {
	cmd := exec.Command("git", "ls-remote", repoURL, "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD revision: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) > 0 && lines[0] != "" {
		parts := strings.Fields(lines[0])
		if len(parts) > 0 {
			return parts[0][:7], nil // Return short hash
		}
	}

	return "", fmt.Errorf("no HEAD revision found")
}

// getCommitHash gets the commit hash for a given ref
func (c *AutoupdateCommand) getCommitHash(repoURL, ref string) (string, error) {
	cmd := exec.Command("git", "ls-remote", repoURL, ref)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get commit hash: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) > 0 && lines[0] != "" {
		parts := strings.Fields(lines[0])
		if len(parts) > 0 {
			return parts[0][:7], nil // Return short hash
		}
	}

	return "", fmt.Errorf("no commit hash found for ref %s", ref)
}

// writeConfig writes the configuration back to file
func (c *AutoupdateCommand) writeConfig(cfg *config.Config, filename string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(filename, data, 0o600)
}

// processRepositoryUpdates processes updates for all repositories in the config
func (c *AutoupdateCommand) processRepositoryUpdates(
	cfg *config.Config,
	opts *AutoupdateOptions,
) (int, bool) {
	updated := 0
	hasChanges := false

	fmt.Println("Updating repositories...")

	for i := range cfg.Repos {
		repo := &cfg.Repos[i]

		// Check if we should update this repository
		if !c.shouldUpdateRepo(repo, opts.Repo) {
			continue
		}

		// Get latest revision for this repository
		latestRev, err := c.getLatestRevisionForRepo(repo, opts)
		if err != nil {
			fmt.Printf("⚠️  Warning: failed to get latest revision for %s: %v\n", repo.Repo, err)
			continue
		}

		// Update repository revision if needed
		if c.updateRepositoryRevision(repo, latestRev, opts) {
			updated++
			if !opts.DryRun {
				hasChanges = true
			}
		}
	}

	return updated, hasChanges
}

// AutoupdateCommandFactory creates a new autoupdate command instance
func AutoupdateCommandFactory() (cli.Command, error) {
	return &AutoupdateCommand{}, nil
}
