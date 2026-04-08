# go-pre-commit

A Go reimplementation of [pre-commit](https://github.com/pre-commit/pre-commit) — a framework for managing and maintaining multi-language pre-commit hooks.

## Features

- **Drop-in replacement** — identical CLI interface to the Python pre-commit tool
- **21 supported languages**: Python, Node, Go, Ruby, Rust, Docker, Docker Image, Conda, Coursier, Dart, Dotnet, Haskell, Julia, Lua, Perl, R, Swift, Fail, Pygrep, System, Script
- **All hook types**: pre-commit, pre-merge-commit, pre-push, commit-msg, post-checkout, post-commit, post-merge, post-rewrite, prepare-commit-msg, pre-rebase
- **File type identification** by extension, filename, and shebang
- **Parallel hook execution** with xargs-style batching
- **Automatic caching** of hook repositories

## Installation

```bash
go install github.com/blairham/go-pre-commit/cmd/pre-commit@latest
```

Or build from source:

```bash
make build
```

## Usage

```bash
# Install git hooks into the current repo
pre-commit install

# Run all hooks against staged files
pre-commit run

# Run all hooks against all files
pre-commit run --all-files

# Run a specific hook
pre-commit run <hook-id>

# Auto-update hook repos to latest versions
pre-commit autoupdate

# Try a repo without adding it to config
pre-commit try-repo <repo> [hook-id]

# Generate sample config
pre-commit sample-config

# Validate config
pre-commit validate-config .pre-commit-config.yaml

# Clean cached repos
pre-commit clean

# Garbage collect unused repos
pre-commit gc
```

## Configuration

Create a `.pre-commit-config.yaml` in your repository root:

```yaml
repos:
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v5.0.0
    hooks:
      - id: trailing-whitespace
      - id: end-of-file-fixer
      - id: check-yaml
  - repo: local
    hooks:
      - id: my-local-hook
        name: My Local Hook
        entry: ./scripts/check.sh
        language: script
        files: '\.go$'
```

## Commands

| Command | Description |
|---------|-------------|
| `run` | Run hooks against staged files (or specified files) |
| `install` | Install the git hook script |
| `uninstall` | Uninstall the git hook script |
| `install-hooks` | Install all hook environments |
| `autoupdate` | Auto-update hook repo revisions |
| `clean` | Clean out cached repos |
| `gc` | Garbage collect unused repos |
| `sample-config` | Print a sample configuration |
| `validate-config` | Validate a config file |
| `validate-manifest` | Validate a manifest file |
| `try-repo` | Try hooks from a repo |
| `init-templatedir` | Install hook into a template directory |
| `migrate-config` | Migrate config from old format |

## Development

```bash
make build       # Build binary
make test        # Run tests
make lint        # Run linter
make fmt         # Format code
make vet         # Run go vet
make check       # Format + vet + test
```

## License

MIT
