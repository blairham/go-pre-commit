package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// HazmatCdCommand implements "hazmat cd" - changes to a subdirectory and runs a command.
type HazmatCdCommand struct {
	Meta *Meta
}

func (c *HazmatCdCommand) Run(args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "Error: usage: hazmat cd <subdir> <cmd> [args...]\n")
		return 1
	}

	subdir := args[0]
	cmdArgs := args[1:]

	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Dir = subdir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			return e.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}

func (c *HazmatCdCommand) Help() string {
	return strings.TrimSpace(`
Usage: pre-commit hazmat cd <subdir> <cmd> [args...]

  Change to a subdirectory and run a command.
`)
}

func (c *HazmatCdCommand) Synopsis() string {
	return "cd to a subdir and run the command"
}

// HazmatIgnoreExitCodeCommand wraps a command and ignores its exit code.
type HazmatIgnoreExitCodeCommand struct {
	Meta *Meta
}

func (c *HazmatIgnoreExitCodeCommand) Run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Error: no command specified\n")
		return 1
	}

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run() // Ignore exit code.
	return 0
}

func (c *HazmatIgnoreExitCodeCommand) Help() string {
	return strings.TrimSpace(`
Usage: pre-commit hazmat ignore-exit-code <cmd> [args...]

  Run a command but always return exit code 0.
`)
}

func (c *HazmatIgnoreExitCodeCommand) Synopsis() string {
	return "Run a command but always return exit code 0"
}

// HazmatN1Command runs a command once per file argument (like xargs -n1).
type HazmatN1Command struct {
	Meta *Meta
}

func (c *HazmatN1Command) Run(args []string) int {
	// Split on -- separator.
	separatorIdx := -1
	for i, a := range args {
		if a == "--" {
			separatorIdx = i
			break
		}
	}

	if separatorIdx < 0 {
		fmt.Fprintf(os.Stderr, "Error: usage: hazmat n1 <cmd> [args...] -- <files...>\n")
		return 1
	}

	cmdArgs := args[:separatorIdx]
	files := args[separatorIdx+1:]

	if len(cmdArgs) == 0 {
		fmt.Fprintf(os.Stderr, "Error: no command specified\n")
		return 1
	}

	exitCode := 0
	for _, f := range files {
		allArgs := append(cmdArgs[1:], f)
		cmd := exec.Command(cmdArgs[0], allArgs...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			if e, ok := err.(*exec.ExitError); ok {
				if e.ExitCode() != 0 {
					exitCode = e.ExitCode()
					fmt.Fprintf(os.Stderr, "Command failed for %s: %v\n",
						f, strings.Join(append(cmdArgs, f), " "))
				}
			}
		}
	}

	return exitCode
}

func (c *HazmatN1Command) Help() string {
	return strings.TrimSpace(`
Usage: pre-commit hazmat n1 <cmd> [args...] -- <files...>

  Run a command once per file (like xargs -n1).
`)
}

func (c *HazmatN1Command) Synopsis() string {
	return "Run a command once per file (like xargs -n1)"
}
