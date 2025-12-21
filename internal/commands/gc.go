package commands

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jessevdk/go-flags"
	_ "github.com/mattn/go-sqlite3" // SQLite driver
	"github.com/mitchellh/cli"

	"github.com/blairham/go-pre-commit/pkg/config"
)

// GcCommand handles the garbage collection command functionality
type GcCommand struct{}

// GcOptions holds command-line options for the gc command
type GcOptions struct {
	Help  bool   `long:"help"  description:"show this help message and exit" short:"h"`
	Color string `long:"color" description:"Whether to use color in output. Defaults to BTICK_auto_BTICK." choice:"auto" choice:"always" choice:"never"`
}

// Help returns the help text for the gc command
func (c *GcCommand) Help() string {
	var opts GcOptions
	parser := flags.NewParser(&opts, flags.Default)
	parser.Usage = "[-h] [--color {auto,always,never}]"

	formatter := &HelpFormatter{
		Command:     "gc",
		Description: "",
		Examples:    []Example{},
		Notes:       []string{},
	}

	return formatter.FormatHelp(parser)
}

// Synopsis returns a short description of the gc command
func (c *GcCommand) Synopsis() string {
	return "Clean unused cached data"
}

// Run executes the gc command
func (c *GcCommand) Run(args []string) int {
	var opts GcOptions
	parser := flags.NewParser(&opts, flags.Default)
	parser.Usage = "[-h] [--color {auto,always,never}]"

	_, err := parser.ParseArgs(args)
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

	// Get cache directory (using same logic as clean command)
	cacheDir := getCacheDirectory()
	dbPath := filepath.Join(cacheDir, "db.db")

	// Check if cache directory exists
	if _, statErr := os.Stat(cacheDir); os.IsNotExist(statErr) {
		fmt.Printf("0 repo(s) removed.\n")
		return 0
	}

	// Check if database exists
	if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
		fmt.Printf("0 repo(s) removed.\n")
		return 0
	}

	removedCount, err := c.gcRepos(cacheDir, dbPath)
	if err != nil {
		fmt.Printf("Error during garbage collection: %v\n", err)
		return 1
	}

	fmt.Printf("%d repo(s) removed.\n", removedCount)
	return 0
}

// Helper functions to reduce cognitive complexity in gcRepos

func (c *GcCommand) initializeDatabase(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	return db, nil
}

func (c *GcCommand) loadConfigsAndRepos(db *sql.DB) ([]string, []repoRecord, error) {
	configs, err := c.selectAllConfigs(db)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get configs: %w", err)
	}

	repos, err := c.selectAllRepos(db)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get repos: %w", err)
	}

	return configs, repos, nil
}

func (c *GcCommand) categorizeConfigs(configs []string) ([]string, []string) {
	var deadConfigs []string
	var liveConfigs []string

	for _, configPath := range configs {
		if _, statErr := os.Stat(configPath); os.IsNotExist(statErr) {
			deadConfigs = append(deadConfigs, configPath)
		} else {
			liveConfigs = append(liveConfigs, configPath)
		}
	}

	return deadConfigs, liveConfigs
}

func (c *GcCommand) buildRepoMaps(repos []repoRecord) (map[string]string, map[string]string) {
	allRepos := make(map[string]string) // repo_name:ref -> path
	unusedRepos := make(map[string]string)

	for _, repo := range repos {
		key := repo.Name + ":" + repo.Ref
		allRepos[key] = repo.Path
		unusedRepos[key] = repo.Path
	}

	return allRepos, unusedRepos
}

func (c *GcCommand) markReposAsUsed(liveConfigs []string, unusedRepos map[string]string) []string {
	var deadConfigs []string

	for _, configPath := range liveConfigs {
		cfg, loadErr := config.LoadConfig(configPath)
		if loadErr != nil {
			deadConfigs = append(deadConfigs, configPath)
			continue
		}

		// Mark repos as used
		for _, repo := range cfg.Repos {
			if repo.Repo == "local" || repo.Repo == "meta" {
				continue // Skip local and meta repos
			}

			key := repo.Repo + ":" + repo.Rev
			delete(unusedRepos, key)
		}
	}
	return deadConfigs
}

func (c *GcCommand) cleanupDeadConfigs(db *sql.DB, deadConfigs []string) error {
	if len(deadConfigs) == 0 {
		return nil
	}

	err := c.deleteConfigs(db, deadConfigs)
	if err != nil {
		return fmt.Errorf("failed to delete dead configs: %w", err)
	}

	return nil
}

func (c *GcCommand) removeUnusedRepos(db *sql.DB, unusedRepos map[string]string) int {
	removedCount := 0

	for repoKey, repoPath := range unusedRepos {
		// Remove from filesystem
		if removeErr := os.RemoveAll(repoPath); removeErr != nil {
			// Silently continue on error
		}

		// Remove from database
		parts := strings.SplitN(repoKey, ":", 2)
		if len(parts) == 2 {
			deleteErr := c.deleteRepo(db, parts[0], parts[1])
			if deleteErr == nil {
				removedCount++
			}
		}
	}

	return removedCount
}

// gcRepos implements the core garbage collection logic
func (c *GcCommand) gcRepos(_ /* cacheDir */, dbPath string) (int, error) {
	// Open database
	db, err := c.initializeDatabase(dbPath)
	if err != nil {
		return 0, err
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			fmt.Printf("⚠️  Warning: failed to close database: %v\n", closeErr)
		}
	}()

	// Get all configs and repos from database
	configs, repos, err := c.loadConfigsAndRepos(db)
	if err != nil {
		return 0, err
	}

	// Categorize configs into live and dead
	deadConfigs, liveConfigs := c.categorizeConfigs(configs)

	// Create map of unused repos (we don't need allRepos)
	_, unusedRepos := c.buildRepoMaps(repos)

	// Check live configs to see which repos are still in use
	additionalDeadConfigs := c.markReposAsUsed(liveConfigs, unusedRepos)
	deadConfigs = append(deadConfigs, additionalDeadConfigs...)

	// Remove dead configs from database
	err = c.cleanupDeadConfigs(db, deadConfigs)
	if err != nil {
		return 0, err
	}

	// Remove unused repos
	removedCount := c.removeUnusedRepos(db, unusedRepos)

	return removedCount, nil
}

// Database helper structs and methods
type repoRecord struct {
	Name string
	Ref  string
	Path string
}

func (c *GcCommand) selectAllConfigs(db *sql.DB) ([]string, error) {
	rows, err := db.Query("SELECT path FROM configs")
	if err != nil {
		// Table might not exist in older databases
		return []string{}, nil
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			fmt.Printf("⚠️  Warning: failed to close database rows: %v\n", closeErr)
		}
	}()

	var configs []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		configs = append(configs, path)
	}
	return configs, rows.Err()
}

func (c *GcCommand) selectAllRepos(db *sql.DB) ([]repoRecord, error) {
	rows, err := db.Query("SELECT repo, ref, path FROM repos")
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			fmt.Printf("⚠️  Warning: failed to close database rows: %v\n", closeErr)
		}
	}()

	var repos []repoRecord
	for rows.Next() {
		var repo repoRecord
		if err := rows.Scan(&repo.Name, &repo.Ref, &repo.Path); err != nil {
			return nil, err
		}
		repos = append(repos, repo)
	}
	return repos, rows.Err()
}

func (c *GcCommand) deleteConfigs(db *sql.DB, configs []string) error {
	stmt, err := db.Prepare("DELETE FROM configs WHERE path = ?")
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := stmt.Close(); closeErr != nil {
			fmt.Printf("⚠️  Warning: failed to close database statement: %v\n", closeErr)
		}
	}()

	for _, config := range configs {
		_, err := stmt.Exec(config)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *GcCommand) deleteRepo(db *sql.DB, repo, ref string) error {
	_, err := db.Exec("DELETE FROM repos WHERE repo = ? AND ref = ?", repo, ref)
	return err
}

// GcCommandFactory creates a new gc command instance
func GcCommandFactory() (cli.Command, error) {
	return &GcCommand{}, nil
}
