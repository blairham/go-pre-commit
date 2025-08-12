package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/blairham/go-pre-commit/pkg/config"
)

// buildPythonCommand builds a Python command
func (b *Builder) buildPythonCommand(
	entry string,
	args []string,
	repoPath string,
	_ config.Hook,
	env map[string]string,
) (*exec.Cmd, error) {
	pythonExe := "python3"

	// If we have an environment with VIRTUAL_ENV set, check if the entry is an installed executable
	if virtualEnv, exists := env["VIRTUAL_ENV"]; exists && virtualEnv != "" {
		envPythonExe := filepath.Join(virtualEnv, "bin", "python")
		if _, err := os.Stat(envPythonExe); err == nil {
			pythonExe = envPythonExe
		}

		// Check if the entry is an executable in the virtual environment
		envEntryExe := filepath.Join(virtualEnv, "bin", entry)
		if _, err := os.Stat(envEntryExe); err == nil {
			// The entry is an installed executable, run it directly
			cmd := exec.Command(envEntryExe, args...)
			cmd.Dir = repoPath
			return cmd, nil
		}
	}

	// Handle entry that starts with python command
	if strings.HasPrefix(entry, "python ") {
		parts := strings.Fields(entry)
		pythonExe = parts[0]
		if len(parts) > 1 {
			entry = strings.Join(parts[1:], " ")
		}
	}

	cmdArgs := []string{entry}
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.Command(pythonExe, cmdArgs...)
	cmd.Dir = repoPath
	return cmd, nil
}

// buildNodeCommand builds a Node.js command
func (b *Builder) buildNodeCommand(
	entry string,
	args []string,
	repoPath string,
	env map[string]string,
) (*exec.Cmd, error) {
	// Check if we have a Node.js environment with NODE_VIRTUAL_ENV set
	if nodeEnv, exists := env["NODE_VIRTUAL_ENV"]; exists && nodeEnv != "" {
		// Check if the entry is an executable in the Node.js environment's bin directory
		envEntryExe := filepath.Join(nodeEnv, "bin", entry)
		if _, err := os.Stat(envEntryExe); err == nil {
			// The entry is an installed executable (like ESLint), run it directly
			cmd := exec.Command(envEntryExe, args...)
			cmd.Dir = repoPath
			return cmd, nil
		}
	}

	// For environment-based Node.js hooks, the entry should be an executable
	// from the environment's bin directory, not a script to run with 'node'
	// This matches Python pre-commit's behavior
	cmdArgs := append([]string{}, args...)
	cmd := exec.Command(entry, cmdArgs...)
	cmd.Dir = repoPath
	return cmd, nil
}

// buildGoCommand builds a Go command
func (b *Builder) buildGoCommand(entry string, args []string) *exec.Cmd {
	if strings.HasPrefix(entry, "go ") {
		// Handle "go run", "go build", etc.
		parts := strings.Fields(entry)
		goArgs := parts[1:] // Skip "go"
		goArgs = append(goArgs, args...)
		return exec.Command("go", goArgs...)
	}

	// Direct go executable or script
	if strings.HasSuffix(entry, ".go") {
		// Go script file - use "go run"
		cmdArgs := append([]string{"run", entry}, args...)
		return exec.Command("go", cmdArgs...)
	}

	return exec.Command(entry, args...)
}

// buildRustCommand builds a Rust command
func (b *Builder) buildRustCommand(entry string, args []string) *exec.Cmd {
	if strings.HasSuffix(entry, ".rs") {
		// Rust source file - compile and run
		cmdArgs := append([]string{entry}, args...)
		return exec.Command("rustc", cmdArgs...)
	}
	return exec.Command(entry, args...)
}

// buildRubyCommand builds a Ruby command
func (b *Builder) buildRubyCommand(entry string, args []string) *exec.Cmd {
	cmdArgs := append([]string{entry}, args...)
	return exec.Command("ruby", cmdArgs...)
}

// buildPerlCommand builds a Perl command
func (b *Builder) buildPerlCommand(entry string, args []string) *exec.Cmd {
	cmdArgs := append([]string{entry}, args...)
	return exec.Command("perl", cmdArgs...)
}

// buildLuaCommand builds a Lua command
func (b *Builder) buildLuaCommand(entry string, args []string) *exec.Cmd {
	cmdArgs := append([]string{entry}, args...)
	return exec.Command("lua", cmdArgs...)
}

// buildSwiftCommand builds a Swift command
func (b *Builder) buildSwiftCommand(entry string, args []string) *exec.Cmd {
	cmdArgs := append([]string{entry}, args...)
	return exec.Command("swift", cmdArgs...)
}

// buildRCommand builds an R command
func (b *Builder) buildRCommand(entry string, args []string) *exec.Cmd {
	cmdArgs := append([]string{entry}, args...)
	return exec.Command("Rscript", cmdArgs...)
}

// buildHaskellCommand builds a Haskell command
func (b *Builder) buildHaskellCommand(entry string, args []string) *exec.Cmd {
	cmdArgs := append([]string{entry}, args...)
	return exec.Command("runhaskell", cmdArgs...)
}

// buildFailCommand builds a fail command (always fails)
func (b *Builder) buildFailCommand() *exec.Cmd {
	// Create a command that will always fail
	return exec.Command("sh", "-c", "exit 1")
}

// buildScriptCommand builds a script command
func (b *Builder) buildScriptCommand(entry string, args []string, repoPath string) (*exec.Cmd, error) {
	// For script language, if the entry doesn't contain a path separator,
	// assume it's a script in the repository root
	var scriptPath string
	if !strings.Contains(entry, "/") && !strings.Contains(entry, "\\") {
		// Entry is just a filename, look for it in the repository
		scriptPath = filepath.Join(repoPath, entry)
		// Check if the script exists in the repository
		if _, err := os.Stat(scriptPath); err == nil {
			entry = scriptPath
		}
		// If not found in repo, fall back to system PATH (original behavior)
	}

	cmd := exec.Command(entry, args...)
	if repoPath != "" {
		cmd.Dir = repoPath
	}

	// Ensure the script inherits the current environment, especially PATH
	cmd.Env = os.Environ()

	return cmd, nil
}

// buildSystemCommand builds a system command
func (b *Builder) buildSystemCommand(entry string, args []string, repoPath string) (*exec.Cmd, error) {
	// Handle empty commands
	if entry == "" {
		return nil, fmt.Errorf("empty command")
	}

	var cmd *exec.Cmd

	// Handle simple commands (no spaces or shell features)
	if !strings.Contains(entry, " ") {
		cmd = exec.Command(entry, args...)
	} else {
		var err error
		cmd, err = b.buildComplexSystemCommand(entry, args)
		if err != nil {
			return nil, err
		}
	}

	// Set the working directory to the repository path so executables can be found
	if repoPath != "" {
		cmd.Dir = repoPath
	}

	return cmd, nil
}

// buildComplexSystemCommand handles complex system commands with spaces
func (b *Builder) buildComplexSystemCommand(entry string, args []string) (*exec.Cmd, error) {
	// Check if this is a shell command pattern (sh -c, bash -c, etc.)
	if strings.HasPrefix(entry, "sh -c ") || strings.HasPrefix(entry, "bash -c ") {
		return b.buildShellCommand(entry, args)
	}

	// Handle other complex commands using simple field splitting
	parts := strings.Fields(entry)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	cmdArgs := parts[1:]
	cmdArgs = append(cmdArgs, args...)
	return exec.Command(parts[0], cmdArgs...), nil
}

// buildShellCommand builds shell commands (sh -c, bash -c)
func (b *Builder) buildShellCommand(entry string, args []string) (*exec.Cmd, error) {
	// For shell commands, we need to preserve the quoting structure
	// Pattern: sh -c 'command' [--] or bash -c 'command' [--]
	var shell, command string
	var remaining string

	if strings.HasPrefix(entry, "sh -c ") {
		shell = "sh"
		remaining = strings.TrimPrefix(entry, "sh -c ")
	} else if strings.HasPrefix(entry, "bash -c ") {
		shell = "bash"
		remaining = strings.TrimPrefix(entry, "bash -c ")
	}

	// Handle the case where there's a trailing -- separator
	remaining = strings.TrimSuffix(remaining, " --")

	// Remove outer quotes from the command if present
	if len(remaining) >= 2 && remaining[0] == '\'' && remaining[len(remaining)-1] == '\'' {
		command = remaining[1 : len(remaining)-1]
	} else {
		command = remaining
	}

	// Build command: shell -c "command" args...
	// The args will be passed as positional parameters to the shell script
	cmdArgs := []string{"-c", command}
	cmdArgs = append(cmdArgs, args...)
	return exec.Command(shell, cmdArgs...), nil
}

// buildGenericCommand builds a generic command
func (b *Builder) buildGenericCommand(
	entry string,
	args []string,
	repoPath string,
) (*exec.Cmd, error) {
	cmd := exec.Command(entry, args...)
	cmd.Dir = repoPath
	return cmd, nil
}

// buildCondaCommand builds a Conda command
func (b *Builder) buildCondaCommand(entry string, args []string, env map[string]string) *exec.Cmd {
	// Check if we're in a conda environment from the provided environment variables
	if condaPrefix := env["CONDA_PREFIX"]; condaPrefix != "" {
		// Use conda run to execute in the environment
		cmdArgs := []string{"run", "-p", condaPrefix, entry}
		cmdArgs = append(cmdArgs, args...)
		return exec.Command("conda", cmdArgs...)
	}

	// No conda environment, run directly
	cmdArgs := append([]string{}, args...)
	cmd := exec.Command(entry, cmdArgs...)
	return cmd
}

// buildCoursierCommand builds a Coursier command
func (b *Builder) buildCoursierCommand(entry string, args []string) *exec.Cmd {
	cmdArgs := append([]string{}, args...)
	cmd := exec.Command(entry, cmdArgs...)
	return cmd
}

// buildDartCommand builds a Dart command
func (b *Builder) buildDartCommand(entry string, args []string) *exec.Cmd {
	if strings.HasSuffix(entry, ".dart") {
		// Dart source file - run with dart
		cmdArgs := []string{entry}
		cmdArgs = append(cmdArgs, args...)
		return exec.Command("dart", cmdArgs...)
	}
	// Direct executable
	cmdArgs := append([]string{}, args...)
	cmd := exec.Command(entry, cmdArgs...)
	return cmd
}

// buildDotnetCommand builds a .NET command
func (b *Builder) buildDotnetCommand(entry string, args []string) *exec.Cmd {
	if strings.HasPrefix(entry, "dotnet ") {
		// Handle "dotnet run", "dotnet build", etc.
		parts := strings.Fields(entry)
		cmdArgs := parts[1:]
		cmdArgs = append(cmdArgs, args...)
		return exec.Command("dotnet", cmdArgs...)
	}
	// Direct executable
	cmdArgs := append([]string{}, args...)
	cmd := exec.Command(entry, cmdArgs...)
	return cmd
}

// buildJuliaCommand builds a Julia command
func (b *Builder) buildJuliaCommand(entry string, args []string) *exec.Cmd {
	if strings.HasSuffix(entry, ".jl") {
		// Julia source file - run with julia
		cmdArgs := []string{entry}
		cmdArgs = append(cmdArgs, args...)
		return exec.Command("julia", cmdArgs...)
	}
	// Direct executable
	cmdArgs := append([]string{}, args...)
	cmd := exec.Command(entry, cmdArgs...)
	return cmd
}

// buildPygrepCommand builds a pygrep command
func (b *Builder) buildPygrepCommand(entry string, args []string) *exec.Cmd {
	cmdArgs := append([]string{}, args...)
	cmd := exec.Command(entry, cmdArgs...)
	return cmd
}
