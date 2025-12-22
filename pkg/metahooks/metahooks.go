// Package metahooks implements the built-in meta hooks for pre-commit
// These match the Python pre-commit meta hooks behavior
package metahooks

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/blairham/go-pre-commit/pkg/config"
	"github.com/blairham/go-pre-commit/pkg/hook/matching"
)

// MetaHookExecutor handles execution of built-in meta hooks
type MetaHookExecutor struct {
	configPath string
	verbose    bool
}

// NewMetaHookExecutor creates a new meta hook executor
func NewMetaHookExecutor(configPath string, verbose bool) *MetaHookExecutor {
	return &MetaHookExecutor{
		configPath: configPath,
		verbose:    verbose,
	}
}

// Identity implements the identity meta hook
// It simply prints each filename passed to it
func (e *MetaHookExecutor) Identity(filenames []string) (int, string) {
	var output strings.Builder
	for _, filename := range filenames {
		output.WriteString(filename)
		output.WriteString("\n")
	}
	return 0, output.String()
}

// CheckHooksApply implements the check-hooks-apply meta hook
// It ensures all hooks in the config apply to at least one file in the repo
func (e *MetaHookExecutor) CheckHooksApply(configFilenames []string) (int, string) {
	retval := 0
	var output strings.Builder

	for _, configFile := range configFilenames {
		cfg, err := config.LoadConfig(configFile)
		if err != nil {
			output.WriteString(fmt.Sprintf("Error loading config %s: %v\n", configFile, err))
			retval = 1
			continue
		}

		// Get all files in the repository
		allFiles, err := getAllRepoFiles()
		if err != nil {
			output.WriteString(fmt.Sprintf("Error getting repo files: %v\n", err))
			retval = 1
			continue
		}

		matcher := matching.NewMatcher()

		for _, repo := range cfg.Repos {
			// Skip local and meta repos for this check
			if repo.Repo == "local" || repo.Repo == "meta" {
				continue
			}

			for _, hook := range repo.Hooks {
				files := matcher.GetFilesForHook(hook, allFiles, true)
				if len(files) == 0 {
					output.WriteString(fmt.Sprintf(
						"%s does not apply to this repository\n",
						hook.ID,
					))
					retval = 1
				}
			}
		}
	}

	return retval, output.String()
}

// CheckUselessExcludes implements the check-useless-excludes meta hook
// It detects exclude patterns that don't match any files
func (e *MetaHookExecutor) CheckUselessExcludes(configFilenames []string) (int, string) {
	retval := 0
	var output strings.Builder

	for _, configFile := range configFilenames {
		cfg, err := config.LoadConfig(configFile)
		if err != nil {
			output.WriteString(fmt.Sprintf("Error loading config %s: %v\n", configFile, err))
			retval = 1
			continue
		}

		// Get all files in the repository
		allFiles, err := getAllRepoFiles()
		if err != nil {
			output.WriteString(fmt.Sprintf("Error getting repo files: %v\n", err))
			retval = 1
			continue
		}

		// Check top-level exclude
		if cfg.ExcludeRegex != "" {
			if !patternMatchesAnyFile(cfg.ExcludeRegex, allFiles) {
				output.WriteString(fmt.Sprintf(
					"The top-level exclude pattern %q does not match any files\n",
					cfg.ExcludeRegex,
				))
				retval = 1
			}
		}

		for _, repo := range cfg.Repos {
			for _, hook := range repo.Hooks {
				// Check hook-level exclude
				if hook.ExcludeRegex != "" {
					// Get files that would match this hook (before exclude)
					matchingFiles := getFilesMatchingHookWithoutExclude(hook, allFiles)
					if !patternMatchesAnyFile(hook.ExcludeRegex, matchingFiles) {
						output.WriteString(fmt.Sprintf(
							"The exclude pattern %q for hook %s does not match any files\n",
							hook.ExcludeRegex,
							hook.ID,
						))
						retval = 1
					}
				}
			}
		}
	}

	return retval, output.String()
}

// getAllRepoFiles returns all files in the current git repository
func getAllRepoFiles() ([]string, error) {
	// Use git ls-files to get all tracked files
	cmd := exec.Command("git", "ls-files")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list git files: %w", err)
	}

	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}

	return files, nil
}

// patternMatchesAnyFile checks if a regex pattern matches any of the given files
func patternMatchesAnyFile(pattern string, files []string) bool {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false // Invalid pattern doesn't match anything
	}

	for _, file := range files {
		if re.MatchString(file) {
			return true
		}
	}
	return false
}

// getFilesMatchingHookWithoutExclude returns files that match a hook's patterns
// (files and types) but ignores the exclude pattern
func getFilesMatchingHookWithoutExclude(hook config.Hook, allFiles []string) []string {
	// Create a hook without the exclude to get base matches
	hookWithoutExclude := hook
	hookWithoutExclude.ExcludeRegex = ""

	matcher := matching.NewMatcher()
	return matcher.GetFilesForHook(hookWithoutExclude, allFiles, true)
}

// IsMetaHook checks if a hook ID is a built-in meta hook
func IsMetaHook(hookID string) bool {
	switch hookID {
	case "identity", "check-hooks-apply", "check-useless-excludes":
		return true
	default:
		return false
	}
}

// ExecuteMetaHook runs a meta hook and returns (exitCode, output)
func ExecuteMetaHook(hookID string, files []string, configPath string, verbose bool) (int, string) {
	executor := NewMetaHookExecutor(configPath, verbose)

	switch hookID {
	case "identity":
		return executor.Identity(files)
	case "check-hooks-apply":
		// check-hooks-apply runs on config files, not regular files
		if len(files) == 0 {
			files = []string{".pre-commit-config.yaml"}
		}
		return executor.CheckHooksApply(files)
	case "check-useless-excludes":
		// check-useless-excludes runs on config files, not regular files
		if len(files) == 0 {
			files = []string{".pre-commit-config.yaml"}
		}
		return executor.CheckUselessExcludes(files)
	default:
		return 1, fmt.Sprintf("Unknown meta hook: %s\n", hookID)
	}
}
