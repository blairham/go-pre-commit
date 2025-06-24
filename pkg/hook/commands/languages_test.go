package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blairham/go-pre-commit/pkg/config"
)

func TestBuilder_buildPythonCommand(t *testing.T) {
	builder := &Builder{}

	tests := []struct {
		expectedCmd  string
		name         string
		entry        string
		repoPath     string
		expectedArgs []string
		args         []string
		hook         config.Hook
	}{
		{
			name:         "basic python script",
			entry:        "script.py",
			args:         []string{"--help"},
			repoPath:     "/test/repo",
			hook:         config.Hook{},
			expectedCmd:  "python3",
			expectedArgs: []string{"python3", "script.py", "--help"},
		},
		{
			name:         "python command entry",
			entry:        "python -m black",
			args:         []string{"--check", "."},
			repoPath:     "/test/repo",
			hook:         config.Hook{},
			expectedCmd:  "python",
			expectedArgs: []string{"python", "-m black", "--check", "."},
		},
		{
			name:         "direct module call",
			entry:        "-m flake8",
			args:         []string{"src/"},
			repoPath:     "/test/repo",
			hook:         config.Hook{},
			expectedCmd:  "python3",
			expectedArgs: []string{"python3", "-m flake8", "src/"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := builder.buildPythonCommand(tt.entry, tt.args, tt.repoPath, tt.hook, map[string]string{})
			require.NoError(t, err)
			assert.Equal(t, tt.expectedArgs, cmd.Args)
			assert.Equal(t, tt.repoPath, cmd.Dir)
		})
	}
}

func TestBuilder_buildNodeCommand(t *testing.T) {
	builder := &Builder{}

	tests := []struct {
		name         string
		entry        string
		args         []string
		repoPath     string
		expectedArgs []string
	}{
		{
			name:         "basic node script without environment",
			entry:        "script.js",
			args:         []string{"--verbose"},
			repoPath:     "/test/repo",
			expectedArgs: []string{"script.js", "--verbose"},
		},
		{
			name:         "node with no args without environment",
			entry:        "app.js",
			args:         []string{},
			repoPath:     "/test/repo",
			expectedArgs: []string{"app.js"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := builder.buildNodeCommand(tt.entry, tt.args, tt.repoPath, map[string]string{})
			require.NoError(t, err)
			assert.Equal(t, tt.expectedArgs, cmd.Args)
			assert.Equal(t, tt.repoPath, cmd.Dir)
		})
	}
}

func TestBuilder_buildGoCommand(t *testing.T) {
	builder := &Builder{}

	tests := []struct {
		name         string
		entry        string
		args         []string
		expectedCmd  string
		expectedArgs []string
	}{
		{
			name:         "go run command",
			entry:        "go run",
			args:         []string{"main.go"},
			expectedCmd:  "go",
			expectedArgs: []string{"go", "run", "main.go"},
		},
		{
			name:         "go build command",
			entry:        "go build",
			args:         []string{"-o", "app", "."},
			expectedCmd:  "go",
			expectedArgs: []string{"go", "build", "-o", "app", "."},
		},
		{
			name:         "go source file",
			entry:        "main.go",
			args:         []string{},
			expectedCmd:  "go",
			expectedArgs: []string{"go", "run", "main.go"},
		},
		{
			name:         "direct executable",
			entry:        "gofmt",
			args:         []string{"-d", "."},
			expectedCmd:  "gofmt",
			expectedArgs: []string{"gofmt", "-d", "."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := builder.buildGoCommand(tt.entry, tt.args)
			assert.Equal(t, tt.expectedArgs, cmd.Args)
		})
	}
}

func TestBuilder_buildRustCommand(t *testing.T) {
	builder := &Builder{}

	tests := []struct {
		name         string
		entry        string
		args         []string
		expectedCmd  string
		expectedArgs []string
	}{
		{
			name:         "rust source file",
			entry:        "main.rs",
			args:         []string{},
			expectedCmd:  "rustc",
			expectedArgs: []string{"rustc", "main.rs"},
		},
		{
			name:         "rust source with args",
			entry:        "hello.rs",
			args:         []string{"-O"},
			expectedCmd:  "rustc",
			expectedArgs: []string{"rustc", "hello.rs", "-O"},
		},
		{
			name:         "rust executable",
			entry:        "cargo",
			args:         []string{"check"},
			expectedCmd:  "cargo",
			expectedArgs: []string{"cargo", "check"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := builder.buildRustCommand(tt.entry, tt.args)
			assert.Equal(t, tt.expectedArgs, cmd.Args)
		})
	}
}

func TestBuilder_buildRubyCommand(t *testing.T) {
	builder := &Builder{}

	tests := []struct {
		name         string
		entry        string
		args         []string
		expectedArgs []string
	}{
		{
			name:         "ruby script",
			entry:        "script.rb",
			args:         []string{"--help"},
			expectedArgs: []string{"ruby", "script.rb", "--help"},
		},
		{
			name:         "ruby with no args",
			entry:        "app.rb",
			args:         []string{},
			expectedArgs: []string{"ruby", "app.rb"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := builder.buildRubyCommand(tt.entry, tt.args)
			assert.Equal(t, tt.expectedArgs, cmd.Args)
		})
	}
}

func TestBuilder_buildPerlCommand(t *testing.T) {
	builder := &Builder{}

	tests := []struct {
		name         string
		entry        string
		args         []string
		expectedArgs []string
	}{
		{
			name:         "perl script",
			entry:        "script.pl",
			args:         []string{"-w"},
			expectedArgs: []string{"perl", "script.pl", "-w"},
		},
		{
			name:         "perl module",
			entry:        "Module.pm",
			args:         []string{},
			expectedArgs: []string{"perl", "Module.pm"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := builder.buildPerlCommand(tt.entry, tt.args)
			assert.Equal(t, tt.expectedArgs, cmd.Args)
		})
	}
}

func TestBuilder_buildLuaCommand(t *testing.T) {
	builder := &Builder{}

	tests := []struct {
		name         string
		entry        string
		args         []string
		expectedArgs []string
	}{
		{
			name:         "lua script",
			entry:        "script.lua",
			args:         []string{"arg1", "arg2"},
			expectedArgs: []string{"lua", "script.lua", "arg1", "arg2"},
		},
		{
			name:         "lua with no extra args",
			entry:        "main.lua",
			args:         []string{},
			expectedArgs: []string{"lua", "main.lua"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := builder.buildLuaCommand(tt.entry, tt.args)
			assert.Equal(t, tt.expectedArgs, cmd.Args)
		})
	}
}

func TestBuilder_buildSwiftCommand(t *testing.T) {
	builder := &Builder{}

	tests := []struct {
		name         string
		entry        string
		args         []string
		expectedArgs []string
	}{
		{
			name:         "swift script",
			entry:        "script.swift",
			args:         []string{},
			expectedArgs: []string{"swift", "script.swift"},
		},
		{
			name:         "swift with args",
			entry:        "main.swift",
			args:         []string{"-O"},
			expectedArgs: []string{"swift", "main.swift", "-O"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := builder.buildSwiftCommand(tt.entry, tt.args)
			assert.Equal(t, tt.expectedArgs, cmd.Args)
		})
	}
}

func TestBuilder_buildRCommand(t *testing.T) {
	builder := &Builder{}

	tests := []struct {
		name         string
		entry        string
		args         []string
		expectedArgs []string
	}{
		{
			name:         "R script",
			entry:        "script.R",
			args:         []string{},
			expectedArgs: []string{"Rscript", "script.R"},
		},
		{
			name:         "R script with args",
			entry:        "analysis.r",
			args:         []string{"--vanilla"},
			expectedArgs: []string{"Rscript", "analysis.r", "--vanilla"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := builder.buildRCommand(tt.entry, tt.args)
			assert.Equal(t, tt.expectedArgs, cmd.Args)
		})
	}
}

func TestBuilder_buildHaskellCommand(t *testing.T) {
	builder := &Builder{}

	tests := []struct {
		name         string
		entry        string
		args         []string
		expectedArgs []string
	}{
		{
			name:         "haskell script",
			entry:        "Main.hs",
			args:         []string{},
			expectedArgs: []string{"runhaskell", "Main.hs"},
		},
		{
			name:         "haskell with args",
			entry:        "script.hs",
			args:         []string{"-O2"},
			expectedArgs: []string{"runhaskell", "script.hs", "-O2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := builder.buildHaskellCommand(tt.entry, tt.args)
			assert.Equal(t, tt.expectedArgs, cmd.Args)
		})
	}
}

func TestBuilder_buildFailCommand(t *testing.T) {
	builder := &Builder{}

	cmd := builder.buildFailCommand("fail", []string{})
	assert.NotNil(t, cmd)
	assert.Equal(t, "sh", cmd.Args[0])
	assert.Contains(t, cmd.Args, "exit 1")
}

func TestBuilder_buildScriptCommand(t *testing.T) {
	builder := &Builder{}

	tests := []struct {
		name         string
		entry        string
		args         []string
		expectedArgs []string
	}{
		{
			name:         "simple script",
			entry:        "script.sh",
			args:         []string{"arg1"},
			expectedArgs: []string{"script.sh", "arg1"},
		},
		{
			name:         "script with no args",
			entry:        "test.py",
			args:         []string{},
			expectedArgs: []string{"test.py"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := builder.buildScriptCommand(tt.entry, tt.args)
			assert.Equal(t, tt.expectedArgs, cmd.Args)
		})
	}
}

func TestBuilder_buildSystemCommand(t *testing.T) {
	builder := &Builder{}

	tests := []struct {
		name         string
		entry        string
		args         []string
		expectedArgs []string
		expectError  bool
	}{
		{
			name:         "simple command",
			entry:        "ls",
			args:         []string{"-la"},
			expectedArgs: []string{"ls", "-la"},
			expectError:  false,
		},
		{
			name:         "complex command",
			entry:        "git log",
			args:         []string{"--oneline"},
			expectedArgs: []string{"git", "log", "--oneline"},
			expectError:  false,
		},
		{
			name:        "empty command",
			entry:       "",
			args:        []string{},
			expectError: true, // strings.Fields("") returns [], so len(parts) == 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := builder.buildSystemCommand(tt.entry, tt.args, "/test/repo")

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, cmd)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedArgs, cmd.Args)
			}
		})
	}
}

func TestBuilder_buildGenericCommand(t *testing.T) {
	builder := &Builder{}

	tests := []struct {
		name         string
		entry        string
		args         []string
		repoPath     string
		expectedArgs []string
	}{
		{
			name:         "generic command",
			entry:        "custom-tool",
			args:         []string{"--config", "file.conf"},
			repoPath:     "/test/path",
			expectedArgs: []string{"custom-tool", "--config", "file.conf"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := builder.buildGenericCommand(tt.entry, tt.args, tt.repoPath)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedArgs, cmd.Args)
			assert.Equal(t, tt.repoPath, cmd.Dir)
		})
	}
}

func TestBuilder_buildPythonCommand_WithVirtualEnv(t *testing.T) {
	builder := &Builder{}

	// Create a temporary directory to simulate a virtual environment
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o755))

	// Create a mock Python executable
	pythonExePath := filepath.Join(binDir, "python")
	pythonScript := `#!/bin/bash
echo "virtual env python"
`
	require.NoError(t, os.WriteFile(pythonExePath, []byte(pythonScript), 0o755))

	// Create a mock installed executable (like trailing-whitespace-fixer)
	installedExePath := filepath.Join(binDir, "my-script")
	installedScript := `#!/bin/bash
echo "installed executable"
`
	require.NoError(t, os.WriteFile(installedExePath, []byte(installedScript), 0o755))

	hook := config.Hook{}
	env := map[string]string{
		"VIRTUAL_ENV": tmpDir,
	}

	cmd, err := builder.buildPythonCommand("my-script", []string{"arg1"}, "/test/repo", hook, env)
	require.NoError(t, err)

	// Should use the installed executable directly (not python + script)
	assert.Equal(t, installedExePath, cmd.Path)
	assert.Equal(t, []string{installedExePath, "arg1"}, cmd.Args)
	assert.Equal(t, "/test/repo", cmd.Dir)
}

func TestBuilder_buildPythonCommand_WithoutVirtualEnv(t *testing.T) {
	builder := &Builder{}

	hook := config.Hook{}
	env := map[string]string{} // No VIRTUAL_ENV

	cmd, err := builder.buildPythonCommand("my-script", []string{"arg1"}, "/test/repo", hook, env)
	require.NoError(t, err)

	// Should use the default python3 (check the last part of the path since it might be shimmed)
	assert.True(t, strings.HasSuffix(cmd.Path, "python3"))
	// Args[0] should still be "python3" (not the resolved path)
	assert.Equal(t, []string{"python3", "my-script", "arg1"}, cmd.Args)
	assert.Equal(t, "/test/repo", cmd.Dir)
}

func TestBuilder_buildPythonCommand_WithVirtualEnv_ScriptNotInstalled(t *testing.T) {
	builder := &Builder{}

	// Create a temporary directory to simulate a virtual environment
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o755))

	// Create a mock Python executable
	pythonExePath := filepath.Join(binDir, "python")
	pythonScript := `#!/bin/bash
echo "virtual env python"
`
	require.NoError(t, os.WriteFile(pythonExePath, []byte(pythonScript), 0o755))

	hook := config.Hook{}
	env := map[string]string{
		"VIRTUAL_ENV": tmpDir,
	}

	// Try to run a script that's not installed as an executable
	cmd, err := builder.buildPythonCommand("some-python-script.py", []string{"arg1"}, "/test/repo", hook, env)
	require.NoError(t, err)

	// Should fall back to using python executable with the script as argument
	assert.Equal(t, pythonExePath, cmd.Path)
	assert.Equal(t, []string{pythonExePath, "some-python-script.py", "arg1"}, cmd.Args)
	assert.Equal(t, "/test/repo", cmd.Dir)
}

func TestBuilder_buildNodeCommand_WithNodeEnv(t *testing.T) {
	builder := &Builder{}

	// Create a temporary directory to simulate a Node.js environment
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o755))

	// Create a mock Node.js executable
	nodeExePath := filepath.Join(binDir, "node")
	nodeScript := `#!/bin/bash
echo "node env node"
`
	require.NoError(t, os.WriteFile(nodeExePath, []byte(nodeScript), 0o755))

	// Create a mock installed executable (like ESLint)
	eslintExePath := filepath.Join(binDir, "eslint")
	eslintScript := `#!/bin/bash
echo "eslint from node env"
`
	require.NoError(t, os.WriteFile(eslintExePath, []byte(eslintScript), 0o755))

	env := map[string]string{
		"NODE_VIRTUAL_ENV": tmpDir,
	}

	cmd, err := builder.buildNodeCommand("eslint", []string{"--fix", "src/"}, "/test/repo", env)
	require.NoError(t, err)

	// Should use the installed executable directly from the Node.js environment
	assert.Equal(t, eslintExePath, cmd.Path)
	assert.Equal(t, []string{eslintExePath, "--fix", "src/"}, cmd.Args)
	assert.Equal(t, "/test/repo", cmd.Dir)
}

func TestBuilder_buildNodeCommand_WithoutNodeEnv(t *testing.T) {
	builder := &Builder{}

	env := map[string]string{} // No NODE_VIRTUAL_ENV

	cmd, err := builder.buildNodeCommand("eslint", []string{"--fix", "src/"}, "/test/repo", env)
	require.NoError(t, err)

	// Should use the entry directly (assuming it's in PATH or a script)
	assert.True(t, strings.HasSuffix(cmd.Path, "eslint"))
	assert.Equal(t, []string{"eslint", "--fix", "src/"}, cmd.Args)
	assert.Equal(t, "/test/repo", cmd.Dir)
}

func TestBuilder_buildNodeCommand_WithNodeEnv_ExecutableNotInstalled(t *testing.T) {
	builder := &Builder{}

	// Create a temporary directory to simulate a Node.js environment
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o755))

	// Create a mock Node.js executable
	nodeExePath := filepath.Join(binDir, "node")
	nodeScript := `#!/bin/bash
echo "node env node"
`
	require.NoError(t, os.WriteFile(nodeExePath, []byte(nodeScript), 0o755))

	env := map[string]string{
		"NODE_VIRTUAL_ENV": tmpDir,
	}

	// Try to run a tool that's not installed in the Node.js environment
	cmd, err := builder.buildNodeCommand("some-other-tool", []string{"arg1"}, "/test/repo", env)
	require.NoError(t, err)

	// Should fall back to using the entry directly (not from the Node.js environment)
	assert.True(t, strings.HasSuffix(cmd.Path, "some-other-tool"))
	assert.Equal(t, []string{"some-other-tool", "arg1"}, cmd.Args)
	assert.Equal(t, "/test/repo", cmd.Dir)
}

func TestBuilder_buildCondaCommand(t *testing.T) {
	builder := &Builder{}

	tests := []struct {
		name         string
		entry        string
		args         []string
		expectedArgs []string
	}{
		{
			name:         "conda script",
			entry:        "conda-script",
			args:         []string{"arg1", "arg2"},
			expectedArgs: []string{"conda-script", "arg1", "arg2"},
		},
		{
			name:         "conda with no extra args",
			entry:        "my-conda-tool",
			args:         []string{},
			expectedArgs: []string{"my-conda-tool"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := builder.buildCondaCommand(tt.entry, tt.args, map[string]string{})
			assert.Equal(t, tt.expectedArgs, cmd.Args)
		})
	}
}

func TestBuilder_buildCondaCommand_WithEnvironment(t *testing.T) {
	builder := &Builder{}

	tests := []struct {
		name         string
		entry        string
		args         []string
		env          map[string]string
		expectedCmd  string
		expectedArgs []string
	}{
		{
			name:  "conda with environment uses conda run",
			entry: "black",
			args:  []string{"--check"},
			env: map[string]string{
				"CONDA_PREFIX": "/path/to/conda/env",
			},
			expectedCmd:  "conda",
			expectedArgs: []string{"conda", "run", "-p", "/path/to/conda/env", "black", "--check"},
		},
		{
			name:         "conda without environment runs directly",
			entry:        "black",
			args:         []string{"--check"},
			env:          map[string]string{},
			expectedCmd:  "black",
			expectedArgs: []string{"black", "--check"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := builder.buildCondaCommand(tt.entry, tt.args, tt.env)
			assert.Equal(t, tt.expectedCmd, filepath.Base(cmd.Path))
			assert.Equal(t, tt.expectedArgs, cmd.Args)
		})
	}
}

func TestBuilder_buildCoursierCommand(t *testing.T) {
	builder := &Builder{}

	tests := []struct {
		name         string
		entry        string
		args         []string
		expectedArgs []string
	}{
		{
			name:         "coursier script",
			entry:        "scalafmt",
			args:         []string{"--check"},
			expectedArgs: []string{"scalafmt", "--check"},
		},
		{
			name:         "coursier with no args",
			entry:        "scalafix",
			args:         []string{},
			expectedArgs: []string{"scalafix"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := builder.buildCoursierCommand(tt.entry, tt.args)
			assert.Equal(t, tt.expectedArgs, cmd.Args)
		})
	}
}

func TestBuilder_buildDartCommand(t *testing.T) {
	builder := &Builder{}

	tests := []struct {
		name         string
		entry        string
		args         []string
		expectedArgs []string
	}{
		{
			name:         "dart source file",
			entry:        "main.dart",
			args:         []string{},
			expectedArgs: []string{"dart", "main.dart"},
		},
		{
			name:         "dart source with args",
			entry:        "app.dart",
			args:         []string{"--verbose"},
			expectedArgs: []string{"dart", "app.dart", "--verbose"},
		},
		{
			name:         "dart executable",
			entry:        "dartfmt",
			args:         []string{"-w", "."},
			expectedArgs: []string{"dartfmt", "-w", "."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := builder.buildDartCommand(tt.entry, tt.args)
			assert.Equal(t, tt.expectedArgs, cmd.Args)
		})
	}
}

func TestBuilder_buildDotnetCommand(t *testing.T) {
	builder := &Builder{}

	tests := []struct {
		name         string
		entry        string
		args         []string
		expectedArgs []string
	}{
		{
			name:         "dotnet run command",
			entry:        "dotnet run",
			args:         []string{"--project", "MyApp"},
			expectedArgs: []string{"dotnet", "run", "--project", "MyApp"},
		},
		{
			name:         "dotnet build command",
			entry:        "dotnet build",
			args:         []string{"-c", "Release"},
			expectedArgs: []string{"dotnet", "build", "-c", "Release"},
		},
		{
			name:         "direct executable",
			entry:        "dotnet-format",
			args:         []string{"--check"},
			expectedArgs: []string{"dotnet-format", "--check"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := builder.buildDotnetCommand(tt.entry, tt.args)
			assert.Equal(t, tt.expectedArgs, cmd.Args)
		})
	}
}

func TestBuilder_buildJuliaCommand(t *testing.T) {
	builder := &Builder{}

	tests := []struct {
		name         string
		entry        string
		args         []string
		expectedArgs []string
	}{
		{
			name:         "julia source file",
			entry:        "script.jl",
			args:         []string{},
			expectedArgs: []string{"julia", "script.jl"},
		},
		{
			name:         "julia source with args",
			entry:        "analysis.jl",
			args:         []string{"--threads", "4"},
			expectedArgs: []string{"julia", "analysis.jl", "--threads", "4"},
		},
		{
			name:         "julia executable",
			entry:        "julia-formatter",
			args:         []string{"src/"},
			expectedArgs: []string{"julia-formatter", "src/"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := builder.buildJuliaCommand(tt.entry, tt.args)
			assert.Equal(t, tt.expectedArgs, cmd.Args)
		})
	}
}

func TestBuilder_buildPygrepCommand(t *testing.T) {
	builder := &Builder{}

	tests := []struct {
		name         string
		entry        string
		args         []string
		expectedArgs []string
	}{
		{
			name:         "pygrep script",
			entry:        "pygrep-hook",
			args:         []string{"--pattern", "TODO"},
			expectedArgs: []string{"pygrep-hook", "--pattern", "TODO"},
		},
		{
			name:         "pygrep with no args",
			entry:        "check-todos",
			args:         []string{},
			expectedArgs: []string{"check-todos"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := builder.buildPygrepCommand(tt.entry, tt.args)
			assert.Equal(t, tt.expectedArgs, cmd.Args)
		})
	}
}
