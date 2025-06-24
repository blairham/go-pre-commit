package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jessevdk/go-flags"
	"github.com/mitchellh/cli"

	"github.com/blairham/go-pre-commit/pkg/config"
	"github.com/blairham/go-pre-commit/pkg/git"
	"github.com/blairham/go-pre-commit/pkg/hook"
	"github.com/blairham/go-pre-commit/pkg/hook/execution"
	"github.com/blairham/go-pre-commit/pkg/hook/formatting"
	"github.com/blairham/go-pre-commit/pkg/repository"
)

// TryRepoCommand handles the try-repo command functionality
type TryRepoCommand struct{}

// TryRepoOptions holds command-line options for the try-repo command
type TryRepoOptions struct {
	Config   string   `long:"config"    description:"Path to config file"                  short:"c"`
	Ref      string   `long:"ref"       description:"Manually select a rev to run against"`
	Hook     string   `long:"hook"      description:"A single hook-id to run"`
	Color    string   `long:"color"     description:"Whether to use color in output"                 choice:"auto" default:"auto"`
	Files    []string `long:"files"     description:"Specific filenames to run hooks on"`
	Verbose  bool     `long:"verbose"   description:"Verbose output"                       short:"v"`
	AllFiles bool     `long:"all-files" description:"Run on all files in the repo"         short:"a"`
	Help     bool     `long:"help"      description:"Show this help message"               short:"h"`
}

// Help returns the help text for the try-repo command
func (c *TryRepoCommand) Help() string {
	var opts TryRepoOptions
	parser := flags.NewParser(&opts, flags.Default)
	parser.Usage = "REPO [OPTIONS]"

	formatter := &HelpFormatter{
		Command:     "try-repo",
		Description: "Try the hooks in a repository, useful for developing new hooks.",
		Examples: []Example{
			{
				Command:     "pre-commit try-repo https://github.com/psf/black",
				Description: "Try hooks from remote repo",
			},
			{
				Command:     "pre-commit try-repo ../my-hooks-repo --ref main",
				Description: "Try hooks from local repo with specific ref",
			},
			{
				Command:     "pre-commit try-repo /path/to/local/repo --hook mypy",
				Description: "Try specific hook from local repo",
			},
			{
				Command:     "pre-commit try-repo https://github.com/pre-commit/mirrors-eslint --all-files",
				Description: "Run on all files",
			},
			{
				Command:     "pre-commit try-repo . --files src/main.py",
				Description: "Run on specific files",
			},
		},
		Notes: []string{
			"positional arguments:",
			"  REPO                  git repository URL or local path",
			"",
			"This command allows you to test hooks from a repository without installing",
			"them in your current project. It's particularly useful when developing",
			"new hooks or testing hooks from a fork.",
			"",
			"REPO can be:",
			"  - A git repository URL (https://github.com/user/repo)",
			"  - A local path to a git repository",
			"  - '.' for the current repository",
		},
	}

	return formatter.FormatHelp(parser)
}

// Synopsis returns a short description of the try-repo command
func (c *TryRepoCommand) Synopsis() string {
	return "Try the hooks in a repository, useful for developing new hooks"
}

// Helper functions to reduce cognitive complexity in TryRepoCommand.Run

func (c *TryRepoCommand) parseAndValidateTryRepoArgs(args []string) (*TryRepoOptions, string, int) {
	var opts TryRepoOptions
	parser := flags.NewParser(&opts, flags.Default)
	parser.Usage = "REPO [OPTIONS]"

	remaining, err := parser.ParseArgs(args)
	if err != nil {
		var flagsErr *flags.Error
		if errors.As(err, &flagsErr) && flagsErr.Type == flags.ErrHelp {
			return &opts, "", 0
		}
		fmt.Printf("Error parsing arguments: %v\n", err)
		return &opts, "", 1
	}

	if len(remaining) == 0 {
		fmt.Println("Error: repository argument is required")
		fmt.Println("Usage: pre-commit try-repo REPO [OPTIONS]")
		return &opts, "", 1
	}

	repoURL := remaining[0]
	if opts.Verbose {
		fmt.Printf("Trying repository: %s\n", repoURL)
		if opts.Ref != "" {
			fmt.Printf("Using ref: %s\n", opts.Ref)
		}
	}

	return &opts, repoURL, -1 // Continue processing
}

func (c *TryRepoCommand) prepareRepository(
	repoURL, ref string,
	verbose bool,
) (*repository.Manager, string, error) {
	// Create repository manager
	repoMgr, err := repository.NewManager()
	if err != nil {
		return nil, "", fmt.Errorf("failed to create repository manager: %w", err)
	}

	if verbose {
		fmt.Printf("Preparing repository: %s\n", repoURL)
	}

	// Create a temporary repo config to use existing functionality
	tempRepo := config.Repo{
		Repo: repoURL,
		Rev:  ref,
	}
	if tempRepo.Rev == "" {
		tempRepo.Rev = "HEAD"
	}

	repoPath, err := repoMgr.CloneOrUpdateRepo(context.Background(), tempRepo)
	if err != nil {
		return nil, "", fmt.Errorf("failed to prepare repository: %w", err)
	}

	return repoMgr, repoPath, nil
}

func (c *TryRepoCommand) loadAndFilterHooks(
	repoURL, repoPath, hookID string,
	verbose bool,
) ([]config.Hook, error) {
	// Load the hooks configuration from the repository
	hooksFile := filepath.Join(repoPath, ".pre-commit-hooks.yaml")
	if _, statErr := os.Stat(hooksFile); os.IsNotExist(statErr) {
		return nil, fmt.Errorf("no .pre-commit-hooks.yaml found in repository %s", repoURL)
	}

	// Parse hooks configuration
	hooks, err := config.LoadHooksConfig(hooksFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load hooks configuration: %w", err)
	}

	if verbose {
		fmt.Printf("Found %d hooks in repository:\n", len(hooks))
		for _, h := range hooks {
			fmt.Printf("  - %s: %s\n", h.ID, h.Name)
		}
	}

	// Filter hooks if specific hook requested
	if hookID != "" {
		for _, h := range hooks {
			if h.ID == hookID {
				return []config.Hook{h}, nil
			}
		}
		return nil, fmt.Errorf("hook '%s' not found in repository", hookID)
	}

	return hooks, nil
}

func (c *TryRepoCommand) determineFilesToProcess(
	opts *TryRepoOptions,
	currentDir string,
) ([]string, error) {
	var files []string
	var err error

	switch {
	case opts.AllFiles:
		// Get all files in current directory
		files, err = getAllFiles(currentDir)
		if err != nil {
			return nil, fmt.Errorf("failed to get files: %w", err)
		}
	case len(opts.Files) > 0:
		files = opts.Files
	default:
		// Try to get staged files if in git repo, otherwise use current directory files
		if repo, repoErr := git.NewRepository(currentDir); repoErr == nil {
			files, err = repo.GetStagedFiles()
			if err != nil || len(files) == 0 {
				// Fallback to modified files
				files, err = repo.GetUnstagedFiles()
				if err != nil {
					fmt.Printf("⚠️  Warning: Failed to get unstaged files: %v\n", err)
				}
			}
		}
		if len(files) == 0 {
			// Fallback to all files
			files, err = getAllFiles(currentDir)
			if err != nil {
				fmt.Printf("⚠️  Warning: Failed to get all files: %v\n", err)
			}
		}
	}

	return files, nil
}

func (c *TryRepoCommand) executeHooksAndPrintResults(
	repoURL, ref string,
	hooksToRun []config.Hook,
	files []string,
	opts *TryRepoOptions,
	currentDir string,
) int {
	if len(files) == 0 {
		fmt.Println("No files to process")
		return 0
	}

	// Create a temporary config for the try-repo run
	tempConfig := &config.Config{
		Repos: []config.Repo{
			{
				Repo:  repoURL,
				Rev:   ref,
				Hooks: hooksToRun,
			},
		},
	}

	// Create execution context directly for the orchestrator
	execCtx := &execution.Context{
		Config:      tempConfig,
		Files:       files,
		AllFiles:    opts.AllFiles,
		Verbose:     opts.Verbose,
		ShowDiff:    false,
		RepoRoot:    currentDir,
		HookStage:   "pre-commit", // Default stage for try-repo
		Environment: nil,
		HookIDs:     nil,
		Parallel:    0,
		Timeout:     0,
		Color:       opts.Color,
	}

	// Create orchestrator and run hooks directly
	orchestrator := hook.NewOrchestrator(execCtx)
	results, err := orchestrator.RunHooks(context.Background())
	if err != nil {
		fmt.Printf("Error running hooks: %v\n", err)
		return 1
	}

	// Print results using the new formatting package
	formatter := formatting.NewFormatter(opts.Color, opts.Verbose)
	formatter.PrintResults(results)

	// Return appropriate exit code
	failed := 0
	for _, result := range results {
		if !result.Success {
			failed++
		}
	}

	if failed > 0 {
		return 1
	}

	return 0
}

// Run executes the try-repo command
func (c *TryRepoCommand) Run(args []string) int {
	opts, repoURL, rc := c.parseAndValidateTryRepoArgs(args)
	if rc != -1 {
		return rc
	}

	// Prepare the repository
	repoMgr, repoPath, err := c.prepareRepository(repoURL, opts.Ref, opts.Verbose)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return 1
	}
	defer func() {
		if closeErr := repoMgr.Close(); closeErr != nil && opts.Verbose {
			fmt.Printf("⚠️  Warning: failed to close repository manager: %v\n", closeErr)
		}
	}()

	// Load and filter hooks
	hooksToRun, err := c.loadAndFilterHooks(repoURL, repoPath, opts.Hook, opts.Verbose)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return 1
	}

	// Get current working directory for context
	currentDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error: Failed to get current directory: %v\n", err)
		return 1
	}

	// Determine files to process
	files, err := c.determineFilesToProcess(opts, currentDir)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return 1
	}

	// Execute hooks and print results
	return c.executeHooksAndPrintResults(repoURL, opts.Ref, hooksToRun, files, opts, currentDir)
}

// getAllFiles gets all files in a directory recursively
func getAllFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			// Skip hidden files and directories
			relPath, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			if !strings.HasPrefix(relPath, ".") && !strings.Contains(relPath, "/.") {
				files = append(files, relPath)
			}
		}
		return nil
	})
	return files, err
}

// TryRepoCommandFactory creates a new try-repo command instance
func TryRepoCommandFactory() (cli.Command, error) {
	return &TryRepoCommand{}, nil
}
