// Package commands handles building executable commands for different hook languages
package commands

import (
	"os/exec"

	"github.com/blairham/go-pre-commit/pkg/config"
)

// Language constants
const (
	LanguagePython  = "python"
	LanguagePython3 = "python3"
	LanguageDocker  = "docker"
	LanguageSystem  = "system"
	LanguageFail    = "fail"
)

// Builder handles building commands for different hook languages
type Builder struct {
	repoRoot string
}

// NewBuilder creates a new command builder
func NewBuilder(repoRoot string) *Builder {
	return &Builder{repoRoot: repoRoot}
}

// buildLanguageCommand is a helper that handles the language-specific command building
// nolint:gocyclo,cyclop // Language dispatcher switch statement is inherently complex but straightforward
func (b *Builder) buildLanguageCommand(
	language, entry string,
	args []string,
	repoPath string,
	hook config.Hook,
	env map[string]string,
) (*exec.Cmd, error) {
	switch language {
	case LanguagePython, LanguagePython3:
		return b.buildPythonCommand(entry, args, repoPath, hook, env)
	case "node":
		return b.buildNodeCommand(entry, args, repoPath, env)
	case LanguageDocker:
		return b.buildDockerCommand(entry, args, hook)
	case "docker_image":
		return b.buildDockerImageCommand(entry, args, hook)
	case "golang":
		return b.buildGoCommand(entry, args), nil
	case LanguageFail:
		return b.buildFailCommand(), nil
	case LanguageSystem:
		return b.buildSystemCommand(entry, args, repoPath)
	case "script":
		return b.buildScriptCommand(entry, args, repoPath)
	case "rust":
		return b.buildRustCommand(entry, args), nil
	case "ruby":
		return b.buildRubyCommand(entry, args), nil
	case "perl":
		return b.buildPerlCommand(entry, args), nil
	case "lua":
		return b.buildLuaCommand(entry, args), nil
	case "swift":
		return b.buildSwiftCommand(entry, args), nil
	case "r":
		return b.buildRCommand(entry, args), nil
	case "haskell":
		return b.buildHaskellCommand(entry, args), nil
	case "conda":
		return b.buildCondaCommand(entry, args, env), nil
	case "coursier":
		return b.buildCoursierCommand(entry, args), nil
	case "dart":
		return b.buildDartCommand(entry, args), nil
	case "dotnet":
		return b.buildDotnetCommand(entry, args), nil
	case "julia":
		return b.buildJuliaCommand(entry, args, env), nil
	case "pygrep":
		return b.buildPygrepCommand(entry, args), nil
	default:
		return b.buildGenericCommand(entry, args, repoPath)
	}
}

// BuildCommand builds an executable command for the given hook
func (b *Builder) BuildCommand(
	hook config.Hook,
	files []string,
	repoPath string,
	_ config.Repo,
	env map[string]string,
) (*exec.Cmd, error) {
	language := hook.Language
	if language == "" {
		language = LanguageSystem
	}

	entry := hook.Entry
	args := hook.Args

	// Add filenames if the hook accepts them
	if shouldPassFilenames(hook) && len(files) > 0 {
		args = append(args, files...)
	}

	return b.buildLanguageCommand(language, entry, args, repoPath, hook, env)
}

// shouldPassFilenames determines if filenames should be passed to the hook
func shouldPassFilenames(hook config.Hook) bool {
	if hook.PassFilenames != nil {
		return *hook.PassFilenames
	}
	// Default behavior based on language
	return hook.Language != "docker" && hook.Language != "docker_image"
}
