# Language Implementation Testing

This directory contains scripts for comprehensive compatibility testing between the Go and Python implementations of pre-commit.

## Scripts Overview

### `test-language-implementations.sh`

The main script for running comprehensive compatibility tests between Go and Python pre-commit implementations.

#### Key Features

- **Dual Implementation Testing**: Automatically runs tests with both Go and Python implementations when available
- **Performance Comparison**: Measures and compares execution times between implementations
- **Functional Equivalence**: Verifies that both implementations produce the same results
- **Cache Compatibility**: Tests bidirectional cache compatibility between implementations
- **CLI Compatibility**: Validates that common CLI commands behave identically

#### Test Categories

- **Core Languages**: python, node, golang, rust, ruby, conda
- **Mobile Languages**: dart, swift
- **Scripting Languages**: lua, perl, r
- **Academic Languages**: haskell, julia
- **Enterprise Languages**: dotnet, coursier
- **Container Languages**: docker, docker_image
- **System Languages**: system, script, fail, pygrep

#### Usage

```bash
# Run all compatibility tests
./scripts/test-language-implementations.sh all

# Run specific category tests
./scripts/test-language-implementations.sh core

# Run single language tests
./scripts/test-language-implementations.sh python

# Run with custom binaries
./scripts/test-language-implementations.sh --go-binary ./bin/pre-commit --python-binary pre-commit all

# Get help
./scripts/test-language-implementations.sh --help
```

#### Prerequisites

1. **Go pre-commit binary** (required): Build with `mage build:binary`
2. **Python pre-commit** (recommended): Install with `pip install pre-commit`

Without Python pre-commit, only Go implementation tests will run.

#### What Gets Tested

When both implementations are available:

1. **Performance Benchmarking**
   - Installation time comparison
   - Cache efficiency measurement
   - Performance ratio calculation

2. **Functional Equivalence**
   - Same environment setup behavior
   - Identical hook execution results
   - Compatible cache structures

3. **CLI Compatibility**
   - Same exit codes for common commands
   - Compatible output formats
   - Consistent help and version information

4. **Cache Interoperability**
   - Bidirectional cache usage
   - Cross-implementation environment reuse

#### Output

Test results are saved to the `test-output/` directory:

- Individual language results: `{language}_test_results.json`
- Summary report: `test_summary.md`
- Comprehensive report: `compatibility_test_report.md`

### `compare_output.sh`

A specialized script for comparing the output format of the `run` command between implementations.

#### Usage

```bash
# Compare run command output
./scripts/compare_output.sh
```

This script creates temporary test repositories and compares the detailed output format when running hooks.

## Environment Variables

- `GO_PRECOMMIT_BINARY`: Path to the Go pre-commit binary
- `PYTHON_PRECOMMIT_BINARY`: Path to the Python pre-commit binary
- `TEST_TIMEOUT`: Test timeout duration (default: 60m)

## Understanding Results

### Performance Metrics

- **Install Time**: Time to set up language environments
- **Cache Efficiency**: Performance improvement from caching (as percentage)
- **Performance Ratio**: How much faster Go is compared to Python

### Expected Cache Efficiency by Category

- **Core Languages** (60-80%): High reuse of environment setups
- **Mobile/Academic** (40-60%): Moderate toolchain reuse
- **System Languages** (5-15%): Minimal caching opportunity (expected)

### Success Criteria

A test passes when:
1. Both implementations can set up the language environment
2. Performance metrics are within expected ranges
3. CLI commands produce compatible output
4. Caches are interoperable between implementations

## Troubleshooting

### Common Issues

1. **Python pre-commit not found**
   ```bash
   pip install pre-commit
   ```

2. **Go binary not found**
   ```bash
   mage build:binary
   ```

3. **Language runtime missing**
   - Install the required language runtime (e.g., Node.js, Python, etc.)
   - Some tests are expected to fail if runtimes aren't available

4. **Test timeouts**
   - Increase timeout: `export TEST_TIMEOUT=120m`
   - Check network connectivity for dependency downloads

### Debug Mode

Enable verbose output for detailed debugging:

```bash
./scripts/test-language-implementations.sh --verbose core
```

## Development

### Adding New Languages

1. Add the language to the appropriate category in `LANGUAGE_CATEGORIES`
2. Create a language-specific test in `tests/integration/languages/`
3. Update the test registry in the Go code
4. Add expected cache efficiency ranges

### Modifying Test Behavior

The Go test framework in `tests/integration/` handles the actual test execution. The shell script primarily:
- Sets up environment variables
- Validates binary availability
- Provides user-friendly output
- Coordinates test execution

## Integration with CI/CD

These scripts are designed to be used in CI/CD pipelines to ensure compatibility across implementations:

```yaml
# Example GitHub Actions usage
- name: Build Go binary
  run: mage build:binary

- name: Install Python pre-commit
  run: pip install pre-commit

- name: Run compatibility tests
  run: ./scripts/test-language-implementations.sh all
```
