package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/blairham/go-pre-commit/internal/output"
	flags "github.com/jessevdk/go-flags"
)

// HookImplCommand is the hidden hook-impl command.
// This is the command that the installed git hook scripts call back into.
// It translates git hook arguments into pre-commit run arguments.
type HookImplCommand struct {
	Meta *Meta
}

type hookImplFlags struct {
	Config              string `long:"config" default:".pre-commit-config.yaml" description:"Path to config file."`
	HookType            string `long:"hook-type" default:"pre-commit" description:"The hook type being run."`
	HookDir             string `long:"hook-dir" description:"The hook directory."`
	SkipOnMissingConfig bool   `long:"skip-on-missing-config" description:"Skip if config file is missing."`
	Color               string `long:"color" default:"auto" description:"Whether to use color in output."`
}

func (c *HookImplCommand) Run(args []string) int {
	var opts hookImplFlags
	remaining, err := flags.NewParser(&opts, flags.HelpFlag|flags.PassDoubleDash).ParseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	output.SetColorModeFromString(opts.Color)

	// Check if config exists when --skip-on-missing-config is set.
	if opts.SkipOnMissingConfig {
		if _, err := os.Stat(opts.Config); os.IsNotExist(err) {
			return 0
		}
	}

	// Check PRE_COMMIT_ALLOW_NO_CONFIG.
	if os.Getenv("PRE_COMMIT_ALLOW_NO_CONFIG") != "" {
		if _, err := os.Stat(opts.Config); os.IsNotExist(err) {
			return 0
		}
	}

	// Run legacy hook first (e.g., .pre-commit.legacy).
	if err := runLegacyHook(opts.HookType, remaining); err != nil {
		output.Warn("Legacy hook failed: %v", err)
	}

	// Build args for the run command based on hook type.
	runArgs := []string{}

	// Add the config.
	if opts.Config != "" {
		runArgs = append(runArgs, "--config", opts.Config)
	}

	// Add stage.
	runArgs = append(runArgs, "--hook-stage", opts.HookType)

	// Map hook-type-specific arguments.
	switch opts.HookType {
	case "pre-commit", "pre-merge-commit":
		// No additional args.

	case "pre-push":
		// Args: <remote-name> <remote-url>
		if len(remaining) >= 2 {
			runArgs = append(runArgs, "--remote-name", remaining[0])
			runArgs = append(runArgs, "--remote-url", remaining[1])
		}
		// Read stdin for refs (pre-push receives ref info on stdin).
		stdinRefs := readPrePushStdin()
		if stdinRefs != nil {
			for _, line := range stdinRefs {
				parts := strings.Fields(line)
				if len(parts) >= 4 {
					localRef := parts[1]
					remoteRef := parts[3]
					z40 := "0000000000000000000000000000000000000000"
					if localRef != z40 && remoteRef != z40 {
						runArgs = append(runArgs, "--from-ref", remoteRef, "--to-ref", localRef)
					}
					runArgs = append(runArgs, "--local-branch", parts[0])
					runArgs = append(runArgs, "--remote-branch", parts[2])
				}
			}
		}

	case "commit-msg":
		if len(remaining) >= 1 {
			runArgs = append(runArgs, "--commit-msg-filename", remaining[0])
		}

	case "prepare-commit-msg":
		if len(remaining) >= 1 {
			runArgs = append(runArgs, "--commit-msg-filename", remaining[0])
		}
		if len(remaining) >= 2 {
			runArgs = append(runArgs, "--prepare-commit-message-source", remaining[1])
		}
		if len(remaining) >= 3 {
			runArgs = append(runArgs, "--commit-object-name", remaining[2])
		}

	case "post-checkout":
		if len(remaining) >= 3 {
			runArgs = append(runArgs, "--from-ref", remaining[0])
			runArgs = append(runArgs, "--to-ref", remaining[1])
			runArgs = append(runArgs, "--checkout-type", remaining[2])
		}
		runArgs = append(runArgs, "--all-files")

	case "post-commit":
		runArgs = append(runArgs, "--all-files")

	case "post-merge":
		if len(remaining) >= 1 {
			runArgs = append(runArgs, "--is-squash-merge", remaining[0])
		}
		runArgs = append(runArgs, "--all-files")

	case "post-rewrite":
		if len(remaining) >= 1 {
			runArgs = append(runArgs, "--rewrite-command", remaining[0])
		}
		runArgs = append(runArgs, "--all-files")

	case "pre-rebase":
		// Args: <upstream> [<branch>]
		if len(remaining) >= 1 {
			runArgs = append(runArgs, "--pre-rebase-upstream", remaining[0])
		}
		if len(remaining) >= 2 {
			runArgs = append(runArgs, "--pre-rebase-branch", remaining[1])
		}
	}

	// Execute the run command directly.
	runCmd := &RunCommand{Meta: c.Meta}
	return runCmd.Run(runArgs)
}

func (c *HookImplCommand) Help() string {
	return strings.TrimSpace(`
Usage: pre-commit hook-impl [options] [-- args...]

  Implementation of git hooks (internal use only).

Options:

      --config=FILE              Path to config file.
      --hook-type=TYPE           The hook type being run.
      --hook-dir=DIR             The hook directory.
      --skip-on-missing-config   Skip if config file is missing.
      --color=MODE               Whether to use color (auto, always, never).
`)
}

func (c *HookImplCommand) Synopsis() string {
	return "Implementation of git hooks (internal use only)"
}

// readPrePushStdin reads ref info from stdin for pre-push hooks.
func readPrePushStdin() []string {
	info, _ := os.Stdin.Stat()
	if info.Mode()&os.ModeCharDevice != 0 {
		return nil // No piped input.
	}

	var lines []string
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// runLegacyHook runs the legacy hook script if it exists.
func runLegacyHook(hookType string, args []string) error {
	// Look for .pre-commit.legacy hook.
	gitDir := os.Getenv("GIT_DIR")
	if gitDir == "" {
		gitDir = ".git"
	}
	legacyPath := filepath.Join(gitDir, "hooks", hookType+".legacy")
	if _, err := os.Stat(legacyPath); os.IsNotExist(err) {
		return nil // No legacy hook.
	}

	cmd := exec.Command(legacyPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
