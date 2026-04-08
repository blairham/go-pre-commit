package languages

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/blairham/go-pre-commit/internal/pcre"
)

// Fail implements the Language interface for the "fail" pseudo-language.
// It always fails with the entry message.
type Fail struct{}

func (f *Fail) Name() string           { return "fail" }
func (f *Fail) EnvironmentDir() string  { return "" }
func (f *Fail) GetDefaultVersion() string { return "default" }

func (f *Fail) HealthCheck(prefix, version string) error { return nil }

func (f *Fail) InstallEnvironment(prefix, version string, additionalDeps []string) error {
	return nil
}

func (f *Fail) Run(ctx context.Context, prefix, workDir, entry string, args, fileArgs []string, version string) (int, []byte, error) {
	return 1, []byte(entry + "\n"), nil
}

// Pygrep implements the Language interface for pygrep hooks.
// pygrep is a regex-based grep that uses Go's regexp.
type Pygrep struct{}

func (p *Pygrep) Name() string           { return "pygrep" }
func (p *Pygrep) EnvironmentDir() string  { return "" }
func (p *Pygrep) GetDefaultVersion() string { return "default" }

func (p *Pygrep) HealthCheck(prefix, version string) error { return nil }

func (p *Pygrep) InstallEnvironment(prefix, version string, additionalDeps []string) error {
	return nil
}

func (p *Pygrep) Run(ctx context.Context, prefix, workDir, entry string, args, fileArgs []string, version string) (int, []byte, error) {
	// Parse args for flags.
	caseInsensitive := false
	multiline := false
	negate := false

	for _, arg := range args {
		switch arg {
		case "-i":
			caseInsensitive = true
		case "--multiline":
			multiline = true
		case "--negate":
			negate = true
		}
	}

	pattern := entry
	if caseInsensitive {
		pattern = "(?i)" + pattern
	}
	if multiline {
		pattern = "(?s)" + pattern
	}

	re, err := pcre.Compile(pattern)
	if err != nil {
		return -1, nil, fmt.Errorf("invalid regex pattern %q: %w", entry, err)
	}

	var output bytes.Buffer
	foundMatch := false

	for _, filename := range fileArgs {
		data, err := os.ReadFile(filename)
		if err != nil {
			continue
		}

		if multiline {
			if pcre.Match(re, string(data)) {
				foundMatch = true
				fmt.Fprintf(&output, "%s: matched pattern %q\n", filename, entry)
			}
		} else {
			lines := strings.Split(string(data), "\n")
			for i, line := range lines {
				if pcre.Match(re, line) {
					foundMatch = true
					fmt.Fprintf(&output, "%s:%d: %s\n", filename, i+1, line)
				}
			}
		}
	}

	if negate {
		// With --negate, fail if no files matched.
		if !foundMatch {
			return 1, []byte("no files matched\n"), nil
		}
		return 0, nil, nil
	}

	if foundMatch {
		return 1, output.Bytes(), nil
	}
	return 0, nil, nil
}

// Unsupported implements the Language interface for system hooks.
type Unsupported struct{}

func (u *Unsupported) Name() string           { return "unsupported" }
func (u *Unsupported) EnvironmentDir() string  { return "" }
func (u *Unsupported) GetDefaultVersion() string { return "default" }
func (u *Unsupported) HealthCheck(prefix, version string) error { return nil }

func (u *Unsupported) InstallEnvironment(prefix, version string, additionalDeps []string) error {
	return nil
}

func (u *Unsupported) Run(ctx context.Context, prefix, workDir, entry string, args, fileArgs []string, version string) (int, []byte, error) {
	return RunHookCommand(ctx, workDir, entry, args, fileArgs, nil)
}

// UnsupportedScript implements the Language interface for script hooks.
type UnsupportedScript struct{}

func (u *UnsupportedScript) Name() string           { return "unsupported_script" }
func (u *UnsupportedScript) EnvironmentDir() string  { return "" }
func (u *UnsupportedScript) GetDefaultVersion() string { return "default" }
func (u *UnsupportedScript) HealthCheck(prefix, version string) error { return nil }

func (u *UnsupportedScript) InstallEnvironment(prefix, version string, additionalDeps []string) error {
	return nil
}

func (u *UnsupportedScript) Run(ctx context.Context, prefix, workDir, entry string, args, fileArgs []string, version string) (int, []byte, error) {
	// For script hooks, entry is relative to the hook repo.
	fullEntry := entry
	if prefix != "" {
		fullEntry = prefix + "/" + entry
	}
	return RunHookCommand(ctx, workDir, fullEntry, args, fileArgs, nil)
}
