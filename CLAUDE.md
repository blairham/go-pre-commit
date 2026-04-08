# go-pre-commit

A Go reimplementation of [pre-commit](https://github.com/pre-commit/pre-commit) — a framework for managing and maintaining multi-language pre-commit hooks.

## Build & Test

```bash
make build          # Build binary to build/pre-commit
make test           # Run tests with race detector: go test -v -race ./...
make lint           # Run golangci-lint (requires golangci-lint installed)
make fmt            # Format code: gofmt -s -w .
make vet            # Run go vet
make check          # fmt + vet + test
make tidy           # go mod tidy
```

## Architecture

- **CLI framework**: `mitchellh/cli` for command dispatch, `jessevdk/go-flags` for flag parsing
- **Coloring**: `charmbracelet/lipgloss` for terminal styling
- **Entry point**: `cmd/pre-commit/main.go` → `internal/cli.Run()`

### Internal packages

| Package | Purpose |
|---------|---------|
| `cli` | Command definitions — each command is a struct implementing `cli.Command` |
| `config` | YAML config parsing (`.pre-commit-config.yaml`) |
| `git` | Git operations (staging, refs, hooks dir) |
| `hook` | Hook execution engine and runner |
| `identify` | File type identification by extension, filename, shebang |
| `languages` | 21 language backends (python, node, go, rust, docker, etc.) |
| `output` | Terminal output formatting with lipgloss styles |
| `pcre` | PCRE regex support via `dlclark/regexp2` |
| `repository` | Hook repository resolution and caching |
| `staged` | Stash management for staged files |
| `store` | On-disk cache for cloned hook repos |
| `xargs` | Parallel execution with batching |

## Conventions

- Commands use `mitchellh/cli.Command` interface: `Run(args []string) int`, `Help() string`, `Synopsis() string`
- Each command embeds `*Meta` for shared state and has a flags struct embedding `GlobalFlags`
- Flag parsing uses `jessevdk/go-flags` struct tags
- Error output goes to stderr, return `1` for failure, `0` for success
- The project mirrors the Python pre-commit CLI interface exactly (drop-in replacement)
