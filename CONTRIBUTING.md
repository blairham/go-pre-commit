# Contributing to go-pre-commit

Thanks for your interest in contributing! This guide covers development setup, workflow, and the release process.

## Development Setup

### Prerequisites

- Go 1.25.7+
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
   git tag -a v0.1.0 -m "v0.1.0"
   git push origin v0.1.0
   ```
   Use [semantic versioning](https://semver.org/). Pre-release tags (e.g., `v0.1.0-rc.1`) are automatically marked as pre-releases on GitHub.

3. **Create a GitHub release from the tag:**
   ```bash
   gh release create v0.1.0 --generate-notes
   ```
   Or use the GitHub web UI: go to **Releases > Draft a new release**, select the tag, and publish.

4. **CI takes over.** The `release` job in `.github/workflows/ci.yml` runs GoReleaser, which:
   - Builds binaries for all platforms (`CGO_ENABLED=0`)
   - Creates archives (`.tar.gz` for Linux/macOS, `.zip` for Windows)
   - Generates a `checksums.txt`
   - Uploads everything to the GitHub release

### Version Numbering

The project uses two version numbers:

- **Compatibility version** (`internal/config/config.go: Version`): Tracks the Python pre-commit version this tool is compatible with (currently `4.5.0`). Update this when adding support for features from a newer Python pre-commit release.
- **Build version**: Set automatically from git tags via ldflags. Shown in `--version` output as build metadata.

### Verifying a Release

After CI completes, check the release page for:
- Archives for all 5 platform/arch combinations
- `checksums.txt`
- Auto-generated changelog

```bash
# Verify checksums after downloading
sha256sum -c checksums.txt
```
