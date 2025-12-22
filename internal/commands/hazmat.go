package commands

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jessevdk/go-flags"
	"github.com/mitchellh/cli"
)

// HazmatCommand handles the hazmat command functionality
// Hazmat provides composable tools for rare use in hook `entry` fields
type HazmatCommand struct{}

// HazmatOptions holds command-line options for the hazmat command
type HazmatOptions struct {
	Help  bool   `long:"help"  description:"show this help message and exit" short:"h"`
	Color string `long:"color" description:"Whether to use color in output" default:"auto" choice:"auto" choice:"always" choice:"never"`
}

// Help returns the help text for the hazmat command
func (c *HazmatCommand) Help() string {
	return `usage: pre-commit hazmat [-h] [--color {auto,always,never}] {cd,ignore-exit-code,n1} ...

Composable tools for rare use in hook ` + "`entry`" + `.

positional arguments:
  {cd,ignore-exit-code,n1}
    cd                  cd to a subdir and run the command
    ignore-exit-code    run the command but ignore the exit code
    n1                  run the command once per filename

optional arguments:
  -h, --help            show this help message and exit
  --color {auto,always,never}
                        Whether to use color in output (default: auto)
`
}

// Synopsis returns a short description of the hazmat command
func (c *HazmatCommand) Synopsis() string {
	return "Composable tools for rare use in hook `entry`"
}

// HazmatCommandFactory creates a new hazmat command instance
func HazmatCommandFactory() (cli.Command, error) {
	return &HazmatCommand{}, nil
}

// Run executes the hazmat command
func (c *HazmatCommand) Run(args []string) int {
	if len(args) == 0 {
		fmt.Print(c.Help())
		return 0
	}

	// Check for help flag
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			fmt.Print(c.Help())
			return 0
		}
	}

	// Parse subcommand
	subcommand := args[0]
	subargs := args[1:]

	switch subcommand {
	case "cd":
		return c.runCD(subargs)
	case "ignore-exit-code":
		return c.runIgnoreExitCode(subargs)
	case "n1":
		return c.runN1(subargs)
	default:
		fmt.Printf("error: argument {cd,ignore-exit-code,n1}: invalid choice: '%s'\n", subcommand)
		return 1
	}
}

// cmdFilenames splits command arguments at "--" to separate command from filenames
// Returns (command, filenames) or error if no "--" found
func cmdFilenames(cmd []string) ([]string, []string, error) {
	for idx := len(cmd) - 1; idx >= 0; idx-- {
		if cmd[idx] == "--" {
			return cmd[:idx], cmd[idx+1:], nil
		}
	}
	return nil, nil, errors.New("hazmat entry must end with `--`")
}

// normalizeCmd handles shebang normalization for commands
// This matches Python's parse_shebang.normalize_cmd behavior
func normalizeCmd(cmd []string) []string {
	if len(cmd) == 0 {
		return cmd
	}
	// For now, just return the command as-is
	// Python's normalize_cmd handles shebang parsing for scripts
	return cmd
}

// runCD implements the "cd" subcommand
// Usage: pre-commit hazmat cd <subdir> <cmd> -- <files>
func (c *HazmatCommand) runCD(args []string) int {
	if len(args) < 2 {
		fmt.Println("usage: pre-commit hazmat cd subdir cmd [cmd ...] -- [filenames]")
		fmt.Println("error: the following arguments are required: subdir, cmd")
		return 1
	}

	// Check for help
	if args[0] == "-h" || args[0] == "--help" {
		fmt.Println("usage: pre-commit hazmat cd [-h] subdir cmd [cmd ...]")
		fmt.Println("")
		fmt.Println("positional arguments:")
		fmt.Println("  subdir      subdirectory to change to")
		fmt.Println("  cmd         command to run")
		fmt.Println("")
		fmt.Println("optional arguments:")
		fmt.Println("  -h, --help  show this help message and exit")
		return 0
	}

	subdir := args[0]
	cmdAndFiles := args[1:]

	cmd, filenames, err := cmdFilenames(cmdAndFiles)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	if len(cmd) == 0 {
		fmt.Println("error: the following arguments are required: cmd")
		return 1
	}

	// Filter filenames that start with the subdir prefix
	prefix := subdir + "/"
	var newFilenames []string
	for _, filename := range filenames {
		if !strings.HasPrefix(filename, prefix) {
			fmt.Fprintf(os.Stderr, "unexpected file without prefix=%s: %s\n", prefix, filename)
			return 1
		}
		newFilenames = append(newFilenames, strings.TrimPrefix(filename, prefix))
	}

	// Normalize and run command
	cmd = normalizeCmd(cmd)

	// Build full command with new filenames
	fullCmd := append(cmd, newFilenames...)

	execCmd := exec.Command(fullCmd[0], fullCmd[1:]...)
	execCmd.Dir = subdir
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	execCmd.Stdin = os.Stdin

	err = execCmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "error running command: %v\n", err)
		return 1
	}

	return 0
}

// runIgnoreExitCode implements the "ignore-exit-code" subcommand
// Usage: pre-commit hazmat ignore-exit-code <cmd> -- [files]
func (c *HazmatCommand) runIgnoreExitCode(args []string) int {
	if len(args) == 0 {
		fmt.Println("usage: pre-commit hazmat ignore-exit-code cmd [cmd ...] -- [filenames]")
		fmt.Println("error: the following arguments are required: cmd")
		return 1
	}

	// Check for help
	if args[0] == "-h" || args[0] == "--help" {
		fmt.Println("usage: pre-commit hazmat ignore-exit-code [-h] cmd [cmd ...]")
		fmt.Println("")
		fmt.Println("positional arguments:")
		fmt.Println("  cmd         command to run")
		fmt.Println("")
		fmt.Println("optional arguments:")
		fmt.Println("  -h, --help  show this help message and exit")
		return 0
	}

	// For ignore-exit-code, we run the command as-is (no -- separation needed)
	// but we still need to support it for consistency
	cmd := normalizeCmd(args)

	execCmd := exec.Command(cmd[0], cmd[1:]...)
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	execCmd.Stdin = os.Stdin

	// Run command but ignore the exit code
	_ = execCmd.Run()

	return 0
}

// runN1 implements the "n1" subcommand
// Usage: pre-commit hazmat n1 <cmd> -- <files>
// Runs the command once per filename instead of batching
func (c *HazmatCommand) runN1(args []string) int {
	if len(args) == 0 {
		fmt.Println("usage: pre-commit hazmat n1 cmd [cmd ...] -- [filenames]")
		fmt.Println("error: the following arguments are required: cmd")
		return 1
	}

	// Check for help
	if args[0] == "-h" || args[0] == "--help" {
		fmt.Println("usage: pre-commit hazmat n1 [-h] cmd [cmd ...]")
		fmt.Println("")
		fmt.Println("positional arguments:")
		fmt.Println("  cmd         command to run")
		fmt.Println("")
		fmt.Println("optional arguments:")
		fmt.Println("  -h, --help  show this help message and exit")
		return 0
	}

	cmd, filenames, err := cmdFilenames(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	if len(cmd) == 0 {
		fmt.Println("error: the following arguments are required: cmd")
		return 1
	}

	cmd = normalizeCmd(cmd)

	// Run command once per filename
	ret := 0
	for _, filename := range filenames {
		fullCmd := append(cmd, filename)
		execCmd := exec.Command(fullCmd[0], fullCmd[1:]...)
		execCmd.Stdout = os.Stdout
		execCmd.Stderr = os.Stderr
		execCmd.Stdin = os.Stdin

		err := execCmd.Run()
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				ret |= exitErr.ExitCode()
			} else {
				ret |= 1
			}
		}
	}

	return ret
}

// HazmatCDCommand is a subcommand handler for "hazmat cd"
type HazmatCDCommand struct{}

func (c *HazmatCDCommand) Help() string {
	return `usage: pre-commit hazmat cd [-h] subdir cmd [cmd ...]

cd to a subdir and run the command

positional arguments:
  subdir      subdirectory to change to
  cmd         command to run

optional arguments:
  -h, --help  show this help message and exit
`
}

func (c *HazmatCDCommand) Synopsis() string {
	return "cd to a subdir and run the command"
}

func (c *HazmatCDCommand) Run(args []string) int {
	hazmat := &HazmatCommand{}
	return hazmat.runCD(args)
}

// HazmatIgnoreExitCodeCommand is a subcommand handler
type HazmatIgnoreExitCodeCommand struct{}

func (c *HazmatIgnoreExitCodeCommand) Help() string {
	return `usage: pre-commit hazmat ignore-exit-code [-h] cmd [cmd ...]

run the command but ignore the exit code

positional arguments:
  cmd         command to run

optional arguments:
  -h, --help  show this help message and exit
`
}

func (c *HazmatIgnoreExitCodeCommand) Synopsis() string {
	return "run the command but ignore the exit code"
}

func (c *HazmatIgnoreExitCodeCommand) Run(args []string) int {
	hazmat := &HazmatCommand{}
	return hazmat.runIgnoreExitCode(args)
}

// HazmatN1Command is a subcommand handler
type HazmatN1Command struct{}

func (c *HazmatN1Command) Help() string {
	return `usage: pre-commit hazmat n1 [-h] cmd [cmd ...]

run the command once per filename

positional arguments:
  cmd         command to run

optional arguments:
  -h, --help  show this help message and exit
`
}

func (c *HazmatN1Command) Synopsis() string {
	return "run the command once per filename"
}

func (c *HazmatN1Command) Run(args []string) int {
	hazmat := &HazmatCommand{}
	return hazmat.runN1(args)
}

// ValidateHazmatEntry checks if a hook entry uses hazmat and validates the subcommand
func ValidateHazmatEntry(entry string) error {
	parts := strings.Fields(entry)
	if len(parts) < 3 {
		return nil // Not a hazmat entry
	}

	// Check if it's a hazmat entry: "pre-commit hazmat <subcommand>"
	if parts[0] != "pre-commit" || parts[1] != "hazmat" {
		return nil
	}

	subcommand := parts[2]
	validSubcommands := []string{"cd", "ignore-exit-code", "n1"}
	for _, valid := range validSubcommands {
		if subcommand == valid {
			return nil
		}
	}

	return fmt.Errorf("invalid hazmat subcommand: %s (valid: cd, ignore-exit-code, n1)", subcommand)
}

// IsHazmatEntry checks if a hook entry uses the hazmat command
func IsHazmatEntry(entry string) bool {
	parts := strings.Fields(entry)
	if len(parts) < 2 {
		return false
	}
	return parts[0] == "pre-commit" && parts[1] == "hazmat"
}

// TransformHazmatEntry transforms a hazmat entry to use the current executable
// This matches Python's behavior in lang_base.hook_cmd
func TransformHazmatEntry(entry string, executable string) string {
	if !IsHazmatEntry(entry) {
		return entry
	}

	parts := strings.Fields(entry)
	// Replace "pre-commit" with the actual executable path
	parts[0] = executable

	return strings.Join(parts, " ")
}

// Ensure unused imports are used
var _ = filepath.Join
var _ = flags.Default
