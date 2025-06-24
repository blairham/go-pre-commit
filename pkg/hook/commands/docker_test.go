package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blairham/go-pre-commit/pkg/config"
)

func TestBuilder_buildDockerCommand(t *testing.T) {
	builder := &Builder{
		repoRoot: "/test/repo",
	}

	tests := []struct {
		name     string
		entry    string
		expected []string
		args     []string
		hook     config.Hook
	}{
		{
			name:  "basic docker command",
			entry: "alpine:latest",
			args:  []string{"echo", "hello"},
			hook:  config.Hook{},
			expected: []string{
				"docker", "run", "--rm",
				"-v", "/test/repo:/src",
				"-w", "/src",
				"alpine:latest",
				"echo", "hello",
			},
		},
		{
			name:  "docker command with language version",
			entry: "echo",
			args:  []string{"hello", "world"},
			hook: config.Hook{
				LanguageVersion: "python:3.9",
			},
			expected: []string{
				"docker", "run", "--rm",
				"-v", "/test/repo:/src",
				"-w", "/src",
				"python:3.9",
				"echo",
				"hello", "world",
			},
		},
		{
			name:  "docker command with multi-word entry",
			entry: "python -m flake8",
			args:  []string{"--config", ".flake8"},
			hook: config.Hook{
				LanguageVersion: "python:3.9",
			},
			expected: []string{
				"docker", "run", "--rm",
				"-v", "/test/repo:/src",
				"-w", "/src",
				"python:3.9",
				"python", "-m", "flake8",
				"--config", ".flake8",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := builder.buildDockerCommand(tt.entry, tt.args, tt.hook)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, cmd.Args)
		})
	}
}

func TestBuilder_buildDockerImageCommand(t *testing.T) {
	builder := &Builder{
		repoRoot: "/test/repo",
	}

	// Test that buildDockerImageCommand is just an alias for buildDockerCommand
	entry := "alpine:latest"
	args := []string{"echo", "test"}
	hook := config.Hook{}

	cmd1, err1 := builder.buildDockerCommand(entry, args, hook)
	require.NoError(t, err1)

	cmd2, err2 := builder.buildDockerImageCommand(entry, args, hook)
	require.NoError(t, err2)

	assert.Equal(t, cmd1.Args, cmd2.Args)
	assert.Equal(t, cmd1.Path, cmd2.Path)
}

func TestDockerCommandVariations(t *testing.T) {
	builder := &Builder{
		repoRoot: "/home/user/project",
	}

	tests := []struct {
		description string
		name        string
		entry       string
		args        []string
		hook        config.Hook
	}{
		{
			name:        "simple image",
			entry:       "node:16",
			args:        []string{},
			hook:        config.Hook{},
			description: "Should use image directly with no additional args",
		},
		{
			name:  "image with command",
			entry: "node",
			args:  []string{"--version"},
			hook: config.Hook{
				LanguageVersion: "node:16-alpine",
			},
			description: "Should use LanguageVersion as image and entry as command",
		},
		{
			name:        "complex command",
			entry:       "npm run lint",
			args:        []string{"--fix"},
			hook:        config.Hook{LanguageVersion: "node:16"},
			description: "Should handle multi-word commands properly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := builder.buildDockerCommand(tt.entry, tt.args, tt.hook)
			require.NoError(t, err, tt.description)

			// Verify basic docker command structure
			assert.Equal(t, "docker", cmd.Args[0])
			assert.Contains(t, cmd.Args, "run")
			assert.Contains(t, cmd.Args, "--rm")
			assert.Contains(t, cmd.Args, "-v")
			assert.Contains(t, cmd.Args, "/home/user/project:/src")
			assert.Contains(t, cmd.Args, "-w")
			assert.Contains(t, cmd.Args, "/src")

			// Verify image is present
			if tt.hook.LanguageVersion != "" {
				assert.Contains(t, cmd.Args, tt.hook.LanguageVersion)
			} else {
				assert.Contains(t, cmd.Args, tt.entry)
			}
		})
	}
}
