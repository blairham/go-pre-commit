// Package execution handles the core hook execution logic
package execution

import (
	"log"
	"os"
	"time"

	"github.com/blairham/go-pre-commit/pkg/config"
)

// isTimingDebugEnabled checks if timing debug is enabled by reading environment variable
func isTimingDebugEnabled() bool {
	return os.Getenv("GO_PRECOMMIT_TIMING_DEBUG") != ""
}

// LogTiming logs the duration of a phase if timing debug is enabled
func LogTiming(phase string, start time.Time) {
	if isTimingDebugEnabled() {
		log.Printf("[TIMING] %s took %v", phase, time.Since(start))
	}
}

// Hook execution constants
const (
	JSExt      = ".js"
	JSXExt     = ".jsx"
	TSExt      = ".ts"
	TSXExt     = ".tsx"
	YamlExt    = ".yaml"
	YmlExt     = ".yml"
	HTMLExt    = ".html"
	DockerCmd  = "docker"
	PythonCmd  = "python"
	Python3Cmd = "python3"
)

// Context holds context for hook execution
type Context struct {
	RepoManager any
	Config      *config.Config
	Environment map[string]string
	HookType    string
	RepoRoot    string
	HookStage   string
	Color       string
	HookIDs     []string
	Files       []string
	Timeout     time.Duration
	Parallel    int
	AllFiles    bool
	Verbose     bool
	ShowDiff    bool
}

// Result represents the result of hook execution
type Result struct {
	Output   string
	Error    string
	Files    []string
	Hook     config.Hook
	Duration time.Duration
	ExitCode int
	Success  bool
	Timeout  bool
	Skipped  bool
}

// RunItem represents a hook to be executed with its repository context
type RunItem struct {
	RepoPath string
	Repo     config.Repo
	Hook     config.Hook
}

// SkipResult represents the result of checking if a hook should be skipped
type SkipResult struct {
	Result Result
	Skip   bool
}

// HookResult wraps a result with its execution index for parallel processing
type HookResult struct {
	Result Result
	Index  int
}
