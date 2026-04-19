# Contributing to go-pre-commit

Thanks for your interest in contributing! This guide covers development setup, workflow, and the release process.

## Development Setup

### Prerequisites

- Go 1.26+ (see `.tool-versions` for the exact version; works with [asdf](https://asdf-vm.com/))
- [golangci-lint](https://golangci-lint.run/welcome/install/) (for linting)

### Building

```bash
git clone https://github.com/blairham/go-pre-commit.git
cd go-pre-commit
make build
```

The binary is output to `build/pre-commit`.

### Running Tests

```bash
make test        # Unit tests with race detector
make test-cover  # Tests with coverage report (outputs coverage.html)
make lint        # golangci-lint
make check       # fmt + vet + test (the full suite)
```

### Integration Tests

Integration tests compare behavior against the Python `pre-commit` and require additional setup:

```bash
# Install Python pre-commit and Node.js, then:
go test -v -tags=integration -timeout=600s ./test/integration/
```

These run automatically in CI on pushes to `main` and on PRs labeled `test-languages`.

## Project Structure

```
cmd/pre-commit/     Entry point — calls internal/cli.Run()
internal/
  cli/              Command definitions (one per file, implements mitchellh/cli.Command)
  config/           YAML config parsing (.pre-commit-config.yaml)
  git/              Git operations (staging, refs, hooks dir)
  hook/             Hook execution engine and runner
  identify/         File type identification by extension, filename, shebang
  languages/        Language backends (see "Adding a New Language" below)
  output/           Terminal output formatting with lipgloss styles
  pcre/             PCRE regex support via dlclark/regexp2
  repository/       Hook repository resolution and caching
  staged/           Stash management for staged files
  store/            On-disk cache for cloned hook repos
  xargs/            Parallel execution with batching
test/integration/   Parity tests against Python pre-commit
```

## Adding a New Language

Most languages follow the same pattern: check a CLI tool exists, install into an env directory, prepend PATH, and run. The `SimpleLanguage` struct in `internal/languages/configlang.go` handles this declaratively:

```go
var myLang = &SimpleLanguage{
    LangName:     "mylang",
    EnvDirName:   "mylang_env",
    HealthCmd:    []string{"mylang", "--version"},
    RunBinSubdir: "bin",
    InstallCmd: func(envDir, prefix string) (string, []string) {
        return "mylang", []string{"install", "--dir", envDir}
    },
}
```

Then register it in `internal/languages/language.go`:

```go
Register("mylang", myLang)
```

For languages that need custom install or run logic, you can use the override fields (`InstallFn`, `RunFn`, `HealthCheckFn`, `RunEnvFn`), or implement the `Language` interface directly as a standalone struct (see Julia and Swift in `others.go`).

## Pull Request Workflow

1. Fork the repo and create a feature branch from `main`.
2. Make your changes, adding tests where appropriate.
3. Run `make check` to verify formatting, vet, and tests pass.
4. Open a PR against `main`. CI will run tests, linting, and a build.

Keep PRs focused — one feature or fix per PR makes review easier.

## Release Process

Releases are fully automated via [GoReleaser](https://goreleaser.com) and GitHub Actions. When a new GitHub release is created, CI builds cross-platform binaries (Linux, macOS, Windows / amd64, arm64) and attaches them to the release.

### Creating a Release

1. **Ensure `main` is clean and CI is green.**

2. **Tag the release:**
   ```bash
   git tag v4.6.0
   git push origin v4.6.0
   ```
   Use [semantic versioning](https://semver.org/). Pre-release tags (e.g., `v4.6.0-rc.1`) are automatically marked as pre-releases on GitHub.

3. **CI takes over.** The GoReleaser workflow (`.github/workflows/goreleaser.yml`) triggers on tag push and:
   - Builds binaries for all platforms (`CGO_ENABLED=0`)
   - Creates archives (`.tar.gz` for Linux/macOS, `.zip` for Windows)
   - Generates a `checksums.txt`
   - Uploads everything to the GitHub release

### Version Numbering

The version is set dynamically via ldflags at build time from git tags. The module uses the `/v4` suffix to match major version 4.x.x (required by Go modules).

- **`go install`** users: `go install github.com/blairham/go-pre-commit/v4/cmd/pre-commit@latest`
- **GoReleaser** injects the tag version into `internal/config.Version` via `-X` ldflags
- **Local builds** (`make build`) use `git describe --tags` for the version

### Verifying a Release

After CI completes, check the release page for:
- Archives for all 5 platform/arch combinations
- `checksums.txt`
- Auto-generated changelog

```bash
# Verify checksums after downloading
sha256sum -c checksums.txt
```
