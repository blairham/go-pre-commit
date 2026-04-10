// Package languages defines the Language interface and the language registry.
package languages

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// Language defines the interface that each language handler must implement.
type Language interface {
	// Name returns the canonical name of the language.
	Name() string

	// EnvironmentDir returns the subdirectory name for the environment,
	// or empty string if no environment is needed.
	EnvironmentDir() string

	// GetDefaultVersion returns the default language version.
	GetDefaultVersion() string

	// HealthCheck verifies the language runtime is available.
	HealthCheck(prefix, version string) error

	// InstallEnvironment creates the hook environment.
	InstallEnvironment(prefix, version string, additionalDeps []string) error

	// Run executes a hook command.
	// prefix is the hook repo clone directory (used for environment resolution).
	// workDir is the user's git repository root (where the command runs).
	Run(ctx context.Context, prefix, workDir, entry string, args, fileArgs []string, version string) (int, []byte, error)
}

var (
	registry   = make(map[string]Language)
	registryMu sync.RWMutex
)

// Register registers a language handler.
func Register(name string, lang Language) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[strings.ToLower(name)] = lang
}

// Get returns the language handler for the given name.
func Get(name string) (Language, error) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	// Normalize name.
	normalized := strings.ToLower(name)

	// Handle aliases.
	switch normalized {
	case "system":
		normalized = "unsupported"
	case "script":
		normalized = "unsupported_script"
	case "python_venv":
		normalized = "python"
	}

	lang, ok := registry[normalized]
	if !ok {
		return nil, fmt.Errorf("unknown language: %q", name)
	}
	return lang, nil
}

func init() {
	Register("python", &Python{})
	Register("node", &Node{})
	Register("golang", &Golang{})
	Register("ruby", &Ruby{})
	Register("rust", &Rust{})
	Register("docker", &Docker{})
	Register("docker_image", &DockerImage{})
	Register("fail", &Fail{})
	Register("pygrep", &Pygrep{})
	Register("unsupported", &Unsupported{})
	Register("unsupported_script", &UnsupportedScript{})
	Register("conda", condaLang)
	Register("coursier", coursierLang)
	Register("dart", dartLang)
	Register("dotnet", dotnetLang)
	Register("haskell", haskellLang)
	Register("julia", &Julia{})
	Register("lua", luaLang)
	Register("perl", perlLang)
	Register("r", rLang)
	Register("swift", &Swift{})
}
