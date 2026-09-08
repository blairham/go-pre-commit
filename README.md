# go-pre-commit

[![CI](https://github.com/blairham/go-pre-commit/actions/workflows/ci.yml/badge.svg)](https://github.com/blairham/go-pre-commit/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/blairham/go-pre-commit/v4)](https://goreportcard.com/report/github.com/blairham/go-pre-commit/v4)
[![GoDoc](https://pkg.go.dev/badge/github.com/blairham/go-pre-commit/v4)](https://pkg.go.dev/github.com/blairham/go-pre-commit/v4)
[![License](https://img.shields.io/github/license/blairham/go-pre-commit)](https://github.com/blairham/go-pre-commit/blob/main/LICENSE)

**Run your existing pre-commit hooks without installing Python.**

Same `.pre-commit-config.yaml`, same hook repositories, same commands — as a
single binary. An independent Go reimplementation of
[pre-commit](https://github.com/pre-commit/pre-commit), not a fork and not
affiliated with it.

Measured against Python pre-commit 4.6.2 on every pull request:
**82 of 82 differential checks pass** ([how that is measured](docs/parity.md)).

## Is this for you?

Probably not, and that is worth two minutes of your time:
**[should you use this instead of Python pre-commit?](docs/comparison.md)**

The short version — if you already have Python and pre-commit working, keep
them. The case for this tool is a repo whose *only* reason to install a Python
toolchain is to run its hooks.

## What it does

- **Drop-in** — the same CLI, config format, hook repositories and cache
  location as the Python tool
- **One binary** — no interpreter, no virtualenv, no `pip`
- **All hook types**: pre-commit, pre-merge-commit, pre-push, commit-msg,
  post-checkout, post-commit, post-merge, post-rewrite, prepare-commit-msg,
  pre-rebase
- **22 languages** implemented — though not equally proven; the
  [parity grading](docs/parity.md#language-support-graded) says which are
  well-trodden and which you would be the first to try
- **File type identification** by extension, filename, and shebang
- **Parallel hook execution** with xargs-style batching

## Installation

> **It installs a binary called `pre-commit`, and that is deliberate.**
> `pre-commit install` writes a git hook that invokes `pre-commit` by name, so a
> drop-in has to answer to that name. On Homebrew it shadows
> `homebrew/core/pre-commit`, and `brew` will say so. If you keep both tools,
> know which one you are getting: `pre-commit --version` prints a `(build …)`
> suffix here and nothing of the sort upstream. The cache directory is shared
> with the Python tool too — details, including what `clean` removes, are in
> [docs/parity.md](docs/parity.md#deliberate-behaviors-that-surprise-people).

### Homebrew

```bash
brew install blairham/tap/pre-commit
```

### Pre-built binaries

Download the latest release from the [Releases page](https://github.com/blairham/go-pre-commit/releases). Archives are available for Linux, macOS, and Windows (amd64/arm64).

> **On Windows, hooks that install an environment do not work yet** (`python`,
> `node`, `ruby`, `golang`). The binary installs and runs, and `system`,
> `script`, `pygrep` and `fail` hooks work. See
> [platform support](docs/parity.md#platform-support).

```bash
# Example: macOS arm64
curl -Lo pre-commit.tar.gz https://github.com/blairham/go-pre-commit/releases/latest/download/pre-commit_Darwin_arm64.tar.gz
tar xzf pre-commit.tar.gz
sudo mv pre-commit /usr/local/bin/
```

### Go install

```bash
go install github.com/blairham/go-pre-commit/v4@latest
```

Note: `go install` names the binary `go-pre-commit` (Go strips the `/v4` suffix). Rename it to `pre-commit` on your PATH — the git hooks it installs invoke it by that name.

### Build from source

```bash
git clone https://github.com/blairham/go-pre-commit.git
cd go-pre-commit
make build
# Binary is at build/pre-commit
```

## GitHub Actions

Run your hooks in CI without setting up Python — this repo doubles as a composite action that installs the release binary and runs `pre-commit run`. It is a drop-in replacement for [pre-commit/action](https://github.com/pre-commit/action):

```yaml
jobs:
  pre-commit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
      - uses: blairham/go-pre-commit@v4
```

Inputs:

```yaml
      - uses: blairham/go-pre-commit@v4
        with:
          version: latest         # release to install, e.g. "v4.6.6"
          extra_args: --all-files # passed to `pre-commit run`
          cache: 'true'           # cache hook environments between runs
          install-only: 'false'   # set 'true' to install the binary but skip `pre-commit run`
```

Hooks run as `pre-commit run --show-diff-on-failure --color=always <extra_args>`, and hook environments (`~/.cache/pre-commit`) are cached keyed on `.pre-commit-config.yaml`, so warm runs skip environment setup entirely. Hooks that need extra tools on `PATH` (e.g. `language: system` hooks) still require you to install those tools in earlier steps. With `install-only: 'true'` the action puts the binary on `PATH` (and still caches) but skips the run step, for workflows that drive pre-commit themselves.

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

## Performance

Benchmarked against Python pre-commit v4.5.1 on the same config (macOS, Apple Silicon, warm caches, 5 iterations averaged).

### Startup time (no staged files)

| Tool | Avg | Min | Max |
|------|-----|-----|-----|
| **Go** | **0.161s** | 0.155s | 0.167s |
| Python | 0.269s | 0.267s | 0.272s |

**1.7x faster** — Go's compiled binary avoids Python interpreter startup overhead.

### Per-hook execution (`--all-files`)

| Hook | Go | Python | Speedup |
|------|-----|--------|---------|
| trailing-whitespace | 0.058s | 0.232s | **4.0x** |
| end-of-file-fixer | 0.053s | 0.225s | **4.2x** |
| check-yaml | 0.074s | 0.223s | **3.0x** |
| check-added-large-files | 0.079s | 0.286s | **3.6x** |
| check-merge-conflict | 0.066s | 0.250s | **3.8x** |
| golangci-lint | 0.919s | 0.905s | 1.0x |
| go-vet-mod | 0.560s | 0.711s | **1.3x** |

Python-based hooks (pre-commit-hooks) see the largest improvement since Go avoids spawning a Python interpreter for each hook. Hooks that shell out to external tools (golangci-lint) show similar performance since the tool itself dominates.

Run the benchmark yourself: `bash .github/bench.sh`

## Development

```bash
make build       # Build binary to build/pre-commit
make test        # Run tests (with -race)
make test-cover  # Tests + HTML coverage report
make lint        # Run golangci-lint
make fmt         # Format code (gofumpt)
make vet         # Run go vet
make tidy        # go mod tidy
make check       # Format + vet + test
```

Note that `make check` does not run the linter — run `make lint` separately before opening a PR.

## Releasing

Releases are automated with [GoReleaser](https://goreleaser.com) via GitHub Actions. To create a release:

```bash
git tag v4.6.7
git push origin v4.6.7
```

The release workflow also moves the `v4` alias tag to the new release, which is
what `uses: blairham/go-pre-commit@v4` resolves to. Its tag trigger deliberately
matches full versions only, so moving the alias does not start a second release.

CI builds, signs, and notarizes cross-platform binaries, publishes a GitHub release, and updates the Homebrew formula in [blairham/homebrew-tap](https://github.com/blairham/homebrew-tap) automatically. Versions track upstream parity: `v4.6.x` means feature parity with Python pre-commit 4.6.

## Documentation

| | |
|---|---|
| [Should you use this?](docs/comparison.md) | The case for staying on Python pre-commit, and the narrow case against it |
| [Parity](docs/parity.md) | What is measured, what is not, and which languages are actually proven |
| [Stability](docs/stability.md) | What the version number means, and what is frozen |
| [Contributing](CONTRIBUTING.md) | How to report a divergence — the most useful thing you can send |
| [Security](SECURITY.md) | What is in scope, and how to report privately |

## Attribution

This is an independent reimplementation, not a fork and not an official
project. It is not affiliated with, endorsed by, or supported by the pre-commit
project or its maintainers.

The behavior it copies — the CLI, the config and manifest schemas, the cache
layout, exit codes and output formatting — is the design of two MIT-licensed
projects, and the file-type tag tables in `internal/identify` follow upstream
`identify`'s data:

- [pre-commit](https://github.com/pre-commit/pre-commit) — © 2014 pre-commit dev
  team: Anthony Sottile, Ken Struys
- [identify](https://github.com/pre-commit/identify) — © 2017 Chris Kuehl,
  Anthony Sottile

Their MIT license is reproduced in [NOTICE](NOTICE). Bugs you find here are
this project's bugs, not theirs — please report them
[here](https://github.com/blairham/go-pre-commit/issues) rather than upstream.

## Contributing

The most valuable contribution is a **divergence report**: upstream does X,
this does Y, with both commands and both outputs. That is the one thing this
project cannot generate for itself — see
[CONTRIBUTING.md](CONTRIBUTING.md).

Divergence is a bug here, never a feature. If you want behavior Python
pre-commit does not have, [upstream](https://github.com/pre-commit/pre-commit)
is the place to ask; if they ship it, it arrives here as parity work.

## License

[Apache-2.0](LICENSE), with third-party notices in [NOTICE](NOTICE).
