// Package commands implements the CLI commands for the pre-commit tool.
package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/jessevdk/go-flags"
	"github.com/mitchellh/cli"
	"gopkg.in/yaml.v3"

	"github.com/blairham/go-pre-commit/pkg/config"
	"github.com/blairham/go-pre-commit/pkg/git"
	"github.com/blairham/go-pre-commit/pkg/repository"
)

// revLineRE matches a YAML rev line with groups for:
// 1: indentation, 2: space after colon, 3: quote char, 4: revision value, 5: trailing content (comment etc)
// This matches Python's REV_LINE_RE = re.compile(r'^(\s+)rev:(\s*)([\'"]?)([^\s#]+)(.*)$')
var revLineRE = regexp.MustCompile(`^(\s+)rev:(\s*)(['"]?)([^\s#'"\n]+)['"]?(.*)$`)

// RevLineMatch holds the parsed components of a rev line
type RevLineMatch struct {
	FullMatch   string // The entire matched line
	Indent      string // Leading whitespace (group 1)
	SpaceAfter  string // Space after the colon (group 2)
	QuoteChar   string // Quote character: ', ", or empty (group 3)
	RevValue    string // The revision value (group 4)
	Trailing    string // Trailing content like comments (group 5)
	LineEnding  string // Line ending: \r\n or \n
	MatchedLine int    // The line number in the file
}

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
	Jobs         int      `long:"jobs"          description:"Number of threads to use. 0 for number of cores. (default 1)." default:"1"                                     short:"j" value-name:"JOBS"`
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

// normalizeJobsCount normalizes the jobs count following Python's logic:
// - 0 means use CPU count
// - Limit to number of repos to process
// - Minimum of 1
func (c *AutoupdateCommand) normalizeJobsCount(jobs int, repoCount int) int {
	// 0 => number of CPUs (matches Python's `jobs = jobs or xargs.cpu_count()`)
	if jobs == 0 {
		jobs = runtime.NumCPU()
	}

	// Max 1 thread per repo (matches Python's `jobs = min(jobs, len(repos) or len(config_repos))`)
	if jobs > repoCount && repoCount > 0 {
		jobs = repoCount
	}

	// At least one thread (matches Python's `jobs = max(jobs, 1)`)
	if jobs < 1 {
		jobs = 1
	}

	return jobs
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
	Revision  string
	FreezeTag string   // Original tag name for frozen revisions
	HookIDs   []string // Hook IDs available at this revision
}

// RepositoryCannotBeUpdatedError represents an error when a repository cannot be updated
type RepositoryCannotBeUpdatedError struct {
	Repo    string
	Message string
}

func (e *RepositoryCannotBeUpdatedError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Repo, e.Message)
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

	// Validate manifest at the new revision and get hook IDs
	hookIDs, err := c.getHookIDsAtRevision(repo.Repo, info.Revision)
	if err != nil {
		return nil, &RepositoryCannotBeUpdatedError{
			Repo:    repo.Repo,
			Message: fmt.Sprintf("failed to load manifest: %v", err),
		}
	}
	info.HookIDs = hookIDs

	return info, nil
}

// getHookIDsAtRevision fetches the manifest at a specific revision and returns hook IDs
func (c *AutoupdateCommand) getHookIDsAtRevision(repoURL, revision string) ([]string, error) {
	// Clone or open the repository
	repoMgr, err := repository.NewManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create repository manager: %w", err)
	}
	defer func() {
		if closeErr := repoMgr.Close(); closeErr != nil {
			// Ignore close errors in this helper function
		}
	}()

	// Clone the repository at the specified revision
	repoConfig := config.Repo{
		Repo: repoURL,
		Rev:  revision,
	}
	repoPath, err := repoMgr.CloneOrUpdateRepo(context.Background(), repoConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to clone repository: %w", err)
	}

	// Load the manifest file
	manifestPath := repoPath + "/.pre-commit-hooks.yaml"
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	// Parse the manifest to extract hook IDs
	var hooks []struct {
		ID string `yaml:"id"`
	}
	if err := yaml.Unmarshal(manifestData, &hooks); err != nil {
		return nil, fmt.Errorf("invalid manifest YAML: %w", err)
	}

	// Extract hook IDs
	hookIDs := make([]string, 0, len(hooks))
	for _, hook := range hooks {
		if hook.ID != "" {
			hookIDs = append(hookIDs, hook.ID)
		}
	}

	return hookIDs, nil
}

// checkHooksStillExist verifies that all configured hooks still exist at the new revision
func (c *AutoupdateCommand) checkHooksStillExist(
	repo *config.Repo,
	revInfo *RevisionInfo,
) error {
	// Get configured hook IDs
	configuredHooks := make(map[string]bool)
	for _, hook := range repo.Hooks {
		configuredHooks[hook.ID] = true
	}

	// Build set of available hooks at new revision
	availableHooks := make(map[string]bool)
	for _, hookID := range revInfo.HookIDs {
		availableHooks[hookID] = true
	}

	// Find missing hooks
	missingHooks := []string{}
	for hookID := range configuredHooks {
		if !availableHooks[hookID] {
			missingHooks = append(missingHooks, hookID)
		}
	}

	if len(missingHooks) > 0 {
		slices.Sort(missingHooks)
		return &RepositoryCannotBeUpdatedError{
			Repo:    repo.Repo,
			Message: fmt.Sprintf("Cannot update because the update target is missing these hooks: %s", strings.Join(missingHooks, ", ")),
		}
	}

	return nil
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
	fmt.Printf("[%s] already up to date!\n", repo.Repo)
	return false
}

func (c *AutoupdateCommand) printFinalStatus(updated int, opts *AutoupdateOptions) {
	// Python version doesn't print a final status message
	// Just silently complete
}

// repoUpdateResult holds the result of updating a single repository
type repoUpdateResult struct {
	Index       int
	Repo        *config.Repo
	RevInfo     *RevisionInfo
	Updated     bool
	Error       error
	IsCannotUpd bool // True if error is RepositoryCannotBeUpdatedError
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

	// Auto-migrate config if needed (matches Python's migrate_config(quiet=True))
	if err := c.migrateConfigIfNeeded(opts.Config); err != nil {
		// Migration errors are not fatal - just log and continue
		// This matches Python behavior where migration failures don't block autoupdate
		fmt.Printf("⚠️  Warning: config migration check failed: %v\n", err)
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
	updated, hasChanges, freezeTags, returnCode := c.processRepositoryUpdates(cfg, opts)

	// Write updated configuration back to file (only if there are changes)
	// Uses writeConfigWithFallback which reformats YAML if rev lines can't be found
	if hasChanges {
		if err := c.writeConfigWithFallback(cfg, opts.Config, freezeTags); err != nil {
			fmt.Printf("Error: failed to write updated configuration: %v\n", err)
			return 1
		}
	}

	c.printFinalStatus(updated, opts)
	return returnCode
}

// migrateConfigIfNeeded checks if the config needs migration and performs it silently
// This matches Python's migrate_config(config_file, quiet=True) call
func (c *AutoupdateCommand) migrateConfigIfNeeded(configPath string) error {
	// Read the config file
	content, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	configStr := string(content)

	// Check if migration is needed using the same heuristic as MigrateConfigCommand
	// Old format: starts with "- repo:" without "repos:" key
	// New format: has "repos:" key
	if !c.configNeedsMigration(configStr) {
		return nil // No migration needed
	}

	// Perform the migration
	migratedContent := c.performConfigMigration(configStr)

	// Write the migrated configuration back
	if err := os.WriteFile(configPath, []byte(migratedContent), 0o600); err != nil {
		return fmt.Errorf("failed to write migrated config: %w", err)
	}

	return nil
}

// configNeedsMigration checks if the configuration needs migration from old to new format
func (c *AutoupdateCommand) configNeedsMigration(configStr string) bool {
	// Old format: doesn't have "repos:" key but has "- repo:" entries
	return !strings.Contains(configStr, "repos:") && strings.Contains(configStr, "- repo:")
}

// performConfigMigration converts old format config to new format
func (c *AutoupdateCommand) performConfigMigration(configStr string) string {
	// Add "repos:" at the beginning and indent the rest
	lines := strings.Split(configStr, "\n")
	var migratedLines []string
	migratedLines = append(migratedLines, "repos:")

	for _, line := range lines {
		// Skip empty lines at the very beginning after repos:
		if len(migratedLines) == 1 && strings.TrimSpace(line) == "" {
			continue
		}

		// Indent each line by 2 spaces
		if strings.TrimSpace(line) != "" {
			migratedLines = append(migratedLines, "  "+line)
		} else {
			migratedLines = append(migratedLines, line)
		}
	}

	return strings.Join(migratedLines, "\n")
}

// getLatestRevision gets the latest tag/revision for a git repository
func (c *AutoupdateCommand) getLatestRevision(repoURL string) (string, error) {
	// Use pure Go implementation to get latest version tag
	latestTag, latestHash, err := git.GetLatestVersionTag(repoURL)
	if err != nil {
		// If no version tags found, get the HEAD commit hash
		return c.getHeadRevision(repoURL)
	}

	// Get the best candidate in case multiple tags point to this commit
	bestTag, err := git.GetBestCandidateTag(latestHash, repoURL)
	if err != nil {
		// If GetBestCandidateTag fails, just use the tag we found
		return latestTag, nil
	}
	return bestTag, nil
}

// getHeadRevision gets the HEAD commit hash for a repository
func (c *AutoupdateCommand) getHeadRevision(repoURL string) (string, error) {
	return git.GetRemoteHEAD(repoURL)
}

// getCommitHash gets the commit hash for a given ref
func (c *AutoupdateCommand) getCommitHash(repoURL, ref string) (string, error) {
	return git.GetCommitForRef(repoURL, ref)
}

// countRevLines counts the number of rev lines in the content that match the regex pattern.
// This is used to detect if the YAML needs to be reformatted.
func (c *AutoupdateCommand) countRevLines(content string, lineEnding string) int {
	lines := strings.Split(content, lineEnding)
	count := 0
	for _, line := range lines {
		if c.parseRevLine(line, lineEnding) != nil {
			count++
		}
	}
	return count
}

// reformatYAML parses and re-serializes the YAML content to normalize formatting.
// This is used as a fallback when rev lines can't be found with regex.
// Returns the reformatted content or the original if reformatting fails.
func (c *AutoupdateCommand) reformatYAML(content string) (string, error) {
	var data map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &data); err != nil {
		return content, fmt.Errorf("failed to parse YAML: %w", err)
	}

	reformatted, err := yaml.Marshal(data)
	if err != nil {
		return content, fmt.Errorf("failed to reformat YAML: %w", err)
	}

	return string(reformatted), nil
}

// writeConfigWithFallback attempts to write config preserving formatting,
// falling back to YAML reformat if rev lines can't be found.
// This matches Python's _original_lines fallback behavior.
func (c *AutoupdateCommand) writeConfigWithFallback(cfg *config.Config, filename string, freezeTags map[int]string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	content := string(data)

	// Detect line ending
	lineEnding := "\n"
	if strings.Contains(content, "\r\n") {
		lineEnding = "\r\n"
	}

	// Count repos that need rev updates (excluding local/meta)
	expectedRevCount := len(cfg.Repos)

	// Count how many rev lines we can find with regex
	foundRevCount := c.countRevLines(content, lineEnding)

	// If counts don't match, try reformatting the YAML first
	if foundRevCount != expectedRevCount {
		reformatted, err := c.reformatYAML(content)
		if err == nil {
			// Write reformatted content and retry
			if err := os.WriteFile(filename, []byte(reformatted), 0o600); err != nil {
				return fmt.Errorf("failed to write reformatted config: %w", err)
			}
			// Reload and check again
			newFoundCount := c.countRevLines(reformatted, lineEnding)
			if newFoundCount != expectedRevCount {
				// Still can't find all rev lines, proceed anyway with best effort
				// The writeConfig function has string-based fallback
			}
		}
		// If reformat failed, proceed with original content (string-based fallback will help)
	}

	// Now do the actual write with formatting preservation
	return c.writeConfig(cfg, filename, freezeTags)
}

// writeConfig writes the configuration back to file while preserving formatting.
// Uses regex-based parsing (matching Python's REV_LINE_RE) for robust rev line handling.
func (c *AutoupdateCommand) writeConfig(cfg *config.Config, filename string, freezeTags map[int]string) error {
	// Read the original file content with original line endings
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	content := string(data)

	// Preserve original line endings (detect \r\n vs \n)
	lineEnding := "\n"
	if strings.Contains(content, "\r\n") {
		lineEnding = "\r\n"
	}

	lines := strings.Split(content, lineEnding)

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
					// Try regex-based parsing first (matching Python's REV_LINE_RE)
					match := c.parseRevLine(revLine, lineEnding)
					if match != nil {
						// Use regex match to build the new line
						freezeTag := ""
						if ft, ok := freezeTags[repoIndex]; ok {
							freezeTag = ft
						}
						lines[j] = c.buildRevLine(match, cfg.Repos[repoIndex].Rev, freezeTag)
						repoIndex++
						break
					}

					// Fallback to string-based parsing if regex doesn't match
					// This handles edge cases the regex might miss
					indent := revLine[:len(revLine)-len(strings.TrimLeft(revLine, " \t"))]
					quoteStyle := c.extractQuoteStyle(revTrimmed)

					// Preserve existing comment (unless it's a frozen comment we need to update)
					existingComment := ""
					if commentIdx := strings.Index(revLine, "#"); commentIdx != -1 {
						comment := revLine[commentIdx:]
						// Only preserve if it's NOT a frozen comment
						if !strings.HasPrefix(strings.TrimSpace(comment), "# frozen:") {
							existingComment = "  " + comment
						}
					}

					// Format the new revision with the same quote style
					newRev := c.formatRevWithQuotes(cfg.Repos[repoIndex].Rev, quoteStyle)

					// Build the new rev line
					if freezeTag, ok := freezeTags[repoIndex]; ok {
						// Adding a frozen comment
						lines[j] = fmt.Sprintf("%srev: %s  # frozen: %s", indent, newRev, freezeTag)
					} else {
						// No freeze tag, preserve existing comments
						lines[j] = fmt.Sprintf("%srev: %s%s", indent, newRev, existingComment)
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

	// Write the updated content back with original line endings
	updatedContent := strings.Join(lines, lineEnding)
	return os.WriteFile(filename, []byte(updatedContent), 0o600)
}

// parseRevLine parses a rev line using regex and returns structured match data.
// This matches Python's REV_LINE_RE pattern for parsing rev lines.
// Returns nil if the line doesn't match the expected rev line format.
func (c *AutoupdateCommand) parseRevLine(line string, lineEnding string) *RevLineMatch {
	matches := revLineRE.FindStringSubmatch(line)
	if matches == nil {
		return nil
	}

	return &RevLineMatch{
		FullMatch:  matches[0],
		Indent:     matches[1],
		SpaceAfter: matches[2],
		QuoteChar:  matches[3],
		RevValue:   matches[4],
		Trailing:   matches[5],
		LineEnding: lineEnding,
	}
}

// extractQuoteStyle extracts the quote style used in a rev line using regex matching.
// Returns: 's' for single quotes, 'd' for double quotes, 'n' for no quotes.
// This now uses the same regex pattern as Python's REV_LINE_RE for consistency.
func (c *AutoupdateCommand) extractQuoteStyle(revLine string) rune {
	match := c.parseRevLine(revLine, "")
	if match != nil && match.QuoteChar != "" {
		switch match.QuoteChar {
		case "'":
			return 's' // single quote
		case "\"":
			return 'd' // double quote
		}
	}

	// Fallback to string-based parsing for edge cases
	parts := strings.SplitN(revLine, ":", 2)
	if len(parts) < 2 {
		return 'n'
	}

	value := strings.TrimSpace(parts[1])

	// Remove any trailing comment
	if commentIdx := strings.Index(value, "#"); commentIdx != -1 {
		value = strings.TrimSpace(value[:commentIdx])
	}

	if len(value) == 0 {
		return 'n'
	}

	// Check what quote character starts the value
	if value[0] == '\'' {
		return 's' // single quote
	}
	if value[0] == '"' {
		return 'd' // double quote
	}
	return 'n' // no quotes
}

// formatRevWithQuotes formats a revision string with the specified quote style
func (c *AutoupdateCommand) formatRevWithQuotes(rev string, quoteStyle rune) string {
	switch quoteStyle {
	case 's':
		return "'" + rev + "'"
	case 'd':
		return "\"" + rev + "\""
	default:
		return rev
	}
}

// buildRevLine builds a new rev line using parsed match data and a new revision value.
// This reconstructs the line preserving the original formatting (indentation, spacing, comments).
func (c *AutoupdateCommand) buildRevLine(match *RevLineMatch, newRev string, freezeTag string) string {
	// Format the revision with the original quote style
	quoteStyle := 'n'
	if match.QuoteChar == "'" {
		quoteStyle = 's'
	} else if match.QuoteChar == "\"" {
		quoteStyle = 'd'
	}
	quotedRev := c.formatRevWithQuotes(newRev, quoteStyle)

	// Handle trailing content (comments)
	trailing := match.Trailing

	// If we have a freeze tag, replace any existing frozen comment
	if freezeTag != "" {
		// Remove existing frozen comment if present
		if idx := strings.Index(trailing, "# frozen:"); idx != -1 {
			trailing = strings.TrimRight(trailing[:idx], " \t")
		}
		// Add the new frozen comment
		return fmt.Sprintf("%srev:%s%s  # frozen: %s", match.Indent, match.SpaceAfter, quotedRev, freezeTag)
	}

	// Preserve non-frozen comments
	if idx := strings.Index(trailing, "# frozen:"); idx != -1 {
		// Remove the frozen comment but keep other content
		trailing = strings.TrimRight(trailing[:idx], " \t")
	}

	return fmt.Sprintf("%srev:%s%s%s", match.Indent, match.SpaceAfter, quotedRev, trailing)
}

// repoJob represents a repository to update with its index in the config
type repoJob struct {
	index int
	repo  *config.Repo
}

// processRepositoryUpdates processes updates for all repositories in the config
// Uses concurrent goroutines when jobs > 1 (matches Python's ThreadPoolExecutor)
func (c *AutoupdateCommand) processRepositoryUpdates(
	cfg *config.Config,
	opts *AutoupdateOptions,
) (int, bool, map[int]string, int) {
	// Build list of repos to update (filtering local/meta and repo filters)
	var reposToUpdate []repoJob
	for i := range cfg.Repos {
		repo := &cfg.Repos[i]
		if c.shouldUpdateRepo(repo, opts.Repo) {
			reposToUpdate = append(reposToUpdate, repoJob{index: i, repo: repo})
		}
	}

	if len(reposToUpdate) == 0 {
		return 0, false, make(map[int]string), 0
	}

	// Normalize jobs count (Python parity: 0 => CPU count, cap to repo count, min 1)
	jobs := c.normalizeJobsCount(opts.Jobs, len(reposToUpdate))

	// Process repositories concurrently
	results := c.processReposConcurrently(reposToUpdate, opts, jobs)

	// Collect results and print output in original order
	updated := 0
	hasChanges := false
	freezeTags := make(map[int]string)
	returnCode := 0

	for _, result := range results {
		if result.Error != nil {
			if result.IsCannotUpd {
				fmt.Println(result.Error.Error())
				returnCode = 1
			} else {
				fmt.Printf("⚠️  Warning: failed to get latest revision for %s: %v\n", result.Repo.Repo, result.Error)
			}
			continue
		}

		// Check that all configured hooks still exist at the new revision
		if err := c.checkHooksStillExist(result.Repo, result.RevInfo); err != nil {
			fmt.Println(err.Error())
			returnCode = 1
			continue
		}

		// Track freeze tag if present
		if result.RevInfo.FreezeTag != "" {
			freezeTags[result.Index] = result.RevInfo.FreezeTag
		}

		// Update repository revision if needed
		if c.updateRepositoryRevision(result.Repo, result.RevInfo, opts) {
			updated++
			hasChanges = true
		}
	}

	return updated, hasChanges, freezeTags, returnCode
}

// processReposConcurrently processes repositories using a worker pool pattern
// Matches Python's concurrent.futures.ThreadPoolExecutor behavior
func (c *AutoupdateCommand) processReposConcurrently(
	reposToUpdate []repoJob,
	opts *AutoupdateOptions,
	jobs int,
) []repoUpdateResult {
	results := make([]repoUpdateResult, len(reposToUpdate))

	// Use sequential processing if jobs == 1 (default)
	if jobs <= 1 {
		for i, job := range reposToUpdate {
			results[i] = c.updateSingleRepo(job.index, job.repo, opts)
		}
		return results
	}

	// Concurrent processing with worker pool
	type workItem struct {
		resultIdx int
		jobIndex  int
		repo      *config.Repo
	}

	workChan := make(chan workItem, len(reposToUpdate))
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Start worker goroutines
	for w := 0; w < jobs; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for work := range workChan {
				result := c.updateSingleRepo(work.jobIndex, work.repo, opts)
				mu.Lock()
				results[work.resultIdx] = result
				mu.Unlock()
			}
		}()
	}

	// Send work to workers
	for i, job := range reposToUpdate {
		workChan <- workItem{
			resultIdx: i,
			jobIndex:  job.index,
			repo:      job.repo,
		}
	}
	close(workChan)

	// Wait for all workers to complete
	wg.Wait()

	return results
}

// updateSingleRepo fetches the latest revision for a single repository
func (c *AutoupdateCommand) updateSingleRepo(
	index int,
	repo *config.Repo,
	opts *AutoupdateOptions,
) repoUpdateResult {
	result := repoUpdateResult{
		Index: index,
		Repo:  repo,
	}

	revInfo, err := c.getLatestRevisionForRepo(repo, opts)
	if err != nil {
		result.Error = err
		var cannotUpdateErr *RepositoryCannotBeUpdatedError
		result.IsCannotUpd = errors.As(err, &cannotUpdateErr)
		return result
	}

	result.RevInfo = revInfo
	return result
}

// AutoupdateCommandFactory creates a new autoupdate command instance
func AutoupdateCommandFactory() (cli.Command, error) {
	return &AutoupdateCommand{}, nil
}
