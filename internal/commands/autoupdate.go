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

	"github.com/blairham/go-pre-commit/pkg/config"
	"github.com/blairham/go-pre-commit/pkg/git"
	"github.com/blairham/go-pre-commit/pkg/repository"
)

// AutoupdateCommand handles the autoupdate command functionality
type AutoupdateCommand struct{}

// AutoupdateOptions holds command-line options for the autoupdate command
type AutoupdateOptions struct {
	Help         bool     `long:"help"          description:"show this help message and exit"                                                                               short:"h"`
	Color        string   `long:"color"         description:"Whether to use color in output. Defaults to BTICK_auto_BTICK."   default:"auto"                    choice:"auto" choice:"always" choice:"never"`
	Config       string   `long:"config"        description:"Path to alternate config file"                                 default:".pre-commit-config.yaml"               short:"c" value-name:"CONFIG"`
	BleedingEdge bool     `long:"bleeding-edge" description:"Update to the bleeding edge of BTICK_HEAD_BTICK instead of the latest tagged version (the default behavior)."`
	Freeze       bool     `long:"freeze"        description:"Store DQUOTE_frozen_DQUOTE hashes in BTICK_rev_BTICK instead of tag names"`
	Repo         []string `long:"repo"          description:"Only update this repository -- may be specified multiple times."                                              value-name:"REPO"`
	Jobs         int      `long:"jobs"          description:"Number of threads to use. (default 1)."                        default:"1"                                     short:"j" value-name:"JOBS"`
}

// Help returns the help text for the autoupdate command
func (c *AutoupdateCommand) Help() string {
	var opts AutoupdateOptions
	parser := flags.NewParser(&opts, flags.Default)
	parser.Usage = "[-h] [--color {auto,always,never}] [-c CONFIG] [--bleeding-edge] [--freeze] [--repo REPO] [-j JOBS]"

	formatter := &HelpFormatter{
		Command:     "autoupdate",
		Description: "",
		Examples:    []Example{},
		Notes:       []string{},
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

// RevisionInfo holds revision and optional freeze tag
type RevisionInfo struct {
	Revision string
	FreezeTag string // Original tag name for frozen revisions
}

func (c *AutoupdateCommand) getLatestRevisionForRepo(
	repo *config.Repo,
	opts *AutoupdateOptions,
) (*RevisionInfo, error) {
	var latestRev string
	var err error

	if opts.BleedingEdge {
		latestRev, err = c.getHeadRevision(repo.Repo)
	} else {
		latestRev, err = c.getLatestRevision(repo.Repo)
	}

	if err != nil {
		return nil, err
	}

	info := &RevisionInfo{Revision: latestRev}

	// Convert to frozen hash if requested
	if opts.Freeze && !opts.BleedingEdge {
		if frozenRev, err := c.getCommitHash(repo.Repo, latestRev); err == nil {
			info.FreezeTag = latestRev // Store the tag name
			info.Revision = frozenRev   // Use the commit hash
		}
	}

	return info, nil
}

func (c *AutoupdateCommand) updateRepositoryRevision(
	repo *config.Repo,
	revInfo *RevisionInfo,
	opts *AutoupdateOptions,
) bool {
	if repo.Rev != revInfo.Revision {
		if revInfo.FreezeTag != "" {
			fmt.Printf("[%s] updating %s -> %s (frozen)\n", repo.Repo, repo.Rev, revInfo.FreezeTag)
		} else {
			fmt.Printf("[%s] updating %s -> %s\n", repo.Repo, repo.Rev, revInfo.Revision)
		}
		repo.Rev = revInfo.Revision
		return true
	}
	return false
}

func (c *AutoupdateCommand) printFinalStatus(updated int, opts *AutoupdateOptions) {
	// Python version doesn't print a final status message
	// Just silently complete
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
	updated, hasChanges, freezeTags := c.processRepositoryUpdates(cfg, opts)

	// Write updated configuration back to file
	if hasChanges {
		if err := c.writeConfig(cfg, opts.Config, freezeTags); err != nil {
			fmt.Printf("Error: failed to write updated configuration: %v\n", err)
			return 1
		}
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

	// If no version tags found, get the HEAD commit hash
	return c.getHeadRevision(repoURL)
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
			return parts[0], nil // Return full hash
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
			return parts[0], nil // Return full hash
		}
	}

	return "", fmt.Errorf("no commit hash found for ref %s", ref)
}

// writeConfig writes the configuration back to file while preserving formatting
func (c *AutoupdateCommand) writeConfig(cfg *config.Config, filename string, freezeTags map[int]string) error {
	// Read the original file content
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	// Track which repo we're currently processing
	repoIndex := 0
	inReposSection := false

	// Update rev fields while preserving all formatting
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Detect when we enter the repos section
		if trimmed == "repos:" {
			inReposSection = true
			continue
		}

		// If we're in the repos section and find a repo URL
		if inReposSection && strings.Contains(trimmed, "repo:") && repoIndex < len(cfg.Repos) {
			// Look for the rev field in subsequent lines
			for j := i + 1; j < len(lines) && j < i+10; j++ {
				revLine := lines[j]
				revTrimmed := strings.TrimSpace(revLine)

				if strings.HasPrefix(revTrimmed, "rev:") {
					// Extract indentation and update the rev value
					indent := revLine[:len(revLine)-len(strings.TrimLeft(revLine, " \t"))]
					// Add freeze comment if present
					if freezeTag, ok := freezeTags[repoIndex]; ok {
						lines[j] = fmt.Sprintf("%srev: %s  # frozen: %s", indent, cfg.Repos[repoIndex].Rev, freezeTag)
					} else {
						lines[j] = fmt.Sprintf("%srev: %s", indent, cfg.Repos[repoIndex].Rev)
					}
					repoIndex++
					break
				}

				// Stop if we hit another repo or the hooks section
				if strings.Contains(revTrimmed, "repo:") || strings.HasPrefix(revTrimmed, "hooks:") {
					break
				}
			}
		}

		// Exit repos section when we hit a top-level key
		if inReposSection && len(line) > 0 && line[0] != ' ' && line[0] != '\t' && line[0] != '-' && trimmed != "repos:" {
			inReposSection = false
		}
	}

	// Write the updated content back
	updatedContent := strings.Join(lines, "\n")
	return os.WriteFile(filename, []byte(updatedContent), 0o600)
}

// processRepositoryUpdates processes updates for all repositories in the config
func (c *AutoupdateCommand) processRepositoryUpdates(
	cfg *config.Config,
	opts *AutoupdateOptions,
) (int, bool, map[int]string) {
	updated := 0
	hasChanges := false
	freezeTags := make(map[int]string)

	for i := range cfg.Repos {
		repo := &cfg.Repos[i]

		// Check if we should update this repository
		if !c.shouldUpdateRepo(repo, opts.Repo) {
			continue
		}

		// Get latest revision for this repository
		revInfo, err := c.getLatestRevisionForRepo(repo, opts)
		if err != nil {
			fmt.Printf("⚠️  Warning: failed to get latest revision for %s: %v\n", repo.Repo, err)
			continue
		}

		// Track freeze tag if present
		if revInfo.FreezeTag != "" {
			freezeTags[i] = revInfo.FreezeTag
		}

		// Update repository revision if needed
		if c.updateRepositoryRevision(repo, revInfo, opts) {
			updated++
			hasChanges = true
		}
	}

	return updated, hasChanges, freezeTags
}

// AutoupdateCommandFactory creates a new autoupdate command instance
func AutoupdateCommandFactory() (cli.Command, error) {
	return &AutoupdateCommand{}, nil
}
