# Comprehensive Testing Guide

This document provides complete documentation for the integration tests used in the go-pre-commit implementation, covering architecture, implementation details, usage, and troubleshooting.

## Table of Contents

1. [Overview](#overview)
2. [Test Architecture](#test-architecture)
3. [Quick Reference](#quick-reference)
4. [Language Test Matrix](#language-test-matrix)
5. [Implementation Details](#implementation-details)
6. [Test Phases](#test-phases)
7. [Configuration and Customization](#configuration-and-customization)
8. [Test Measurements and Pass/Fail Criteria](#test-measurements-and-passfail-criteria)
9. [Performance Analysis](#performance-analysis)
10. [Troubleshooting Guide](#troubleshooting-guide)
11. [Development Guide](#development-guide)

## Overview

The integration test suite validates compatibility between the Go implementation of pre-commit and the original Python implementation. Tests are structured hierarchically with base functionality tests and language-specific extensions, covering:

- **Cross-Implementation Compatibility**: Ensuring Go and Python implementations produce equivalent results
- **Performance Benchmarking**: Measuring and comparing execution speeds
- **Cache Compatibility**: Validating cache interoperability (bidirectional testing for Python)
- **Environment Management**: Testing language runtime setup and isolation
- **Repository Operations**: Validating git operations and repository caching

## Test Architecture

### Hierarchical Structure

```
TestAllLanguagesCompatibility (main test)
├── Base Validation Framework (base.go)
├── Language-Specific Tests (per language)
├── Bidirectional Cache Testing (Python only)
├── Performance Benchmarking
└── Cross-Implementation Compatibility
```

### Core Components

```
tests/
├── integration_test.go           # Main test entry points
├── integration/                  # Test framework
│   ├── executor.go              # Test execution engine
│   ├── suite.go                 # Test suite management
│   ├── workspace.go             # Workspace management
│   ├── types.go                 # Data structures and results
│   ├── reports.go               # Result reporting and analysis
│   └── languages/               # Language-specific tests
│       ├── base.go              # Base test framework
│       ├── python.go            # Python tests (with bidirectional cache)
│       ├── node.go              # Node.js tests
│       ├── go.go                # Go tests
│       ├── rust.go              # Rust tests
│       ├── ruby.go              # Ruby tests
│       ├── system.go            # System tests
│       ├── script.go            # Script tests
│       ├── fail.go              # Failure tests
│       ├── pygrep.go            # Pattern matching tests
│       └── [other languages]    # Additional language tests
└── helpers/                     # Test utilities
```

### Interface Definitions

```go
type LanguageTestRunner interface {
    // SetupRepositoryFiles creates language-specific files in the test repository
    SetupRepositoryFiles(repoPath string) error

    // GetLanguageManager returns the language manager for this language
    GetLanguageManager() (language.Manager, error)

    // GetAdditionalValidations returns language-specific validation steps
    GetAdditionalValidations() []ValidationStep

    // GetLanguageName returns the name of the language being tested
    GetLanguageName() string
}

type BidirectionalTestRunner interface {
    LanguageTestRunner

    // TestBidirectionalCacheCompatibility tests cache compatibility between implementations
    TestBidirectionalCacheCompatibility(t *testing.T, pythonBinary, goBinary, testRepo string) error
}

type ValidationStep struct {
    Name        string                                                    // Unique identifier
    Description string                                                    // Human-readable description
    Execute     func(t *testing.T, envPath, version string, lang language.Manager) error  // Validation logic
}
```

## Quick Reference

### Essential Commands

```bash
# Full test suite (30+ minutes)
go test -v ./tests/ -run TestAllLanguagesCompatibility -timeout 30m

# Save output to file
go test -v ./tests/ -run TestAllLanguagesCompatibility -timeout 30m 2>&1 | tee test-results.log

# Core programming languages (Python, Node, Go, Rust, Ruby)
go test -v ./tests/ -run TestCoreLanguages -timeout 15m

# System-level languages (System, Script, Fail, Pygrep)
go test -v ./tests/ -run TestSystemLanguages -timeout 10m

# Container-based languages (Docker, Docker Image)
go test -v ./tests/ -run TestContainerLanguages -timeout 10m

# Package manager languages (Conda, Coursier)
go test -v ./tests/ -run TestPackageManagerLanguages -timeout 10m

# Individual language tests
go test -v ./tests/ -run TestAllLanguagesCompatibility/python -timeout 10m
go test -v ./tests/ -run TestAllLanguagesCompatibility/node -timeout 5m
go test -v ./tests/ -run TestAllLanguagesCompatibility/golang -timeout 3m
go test -v ./tests/ -run TestAllLanguagesCompatibility/system -timeout 2m
```

### Test Measurements Summary

| Measurement | Description | Pass Criteria | Typical Values |
|-------------|-------------|---------------|----------------|
| **Repository/Environment Setup** | Language environment creation | ✅ Setup succeeds, health check passes | Pass/Fail |
| **Environment Isolation** | Separate environments per test | ✅ No conflicts between tests | Always ✅ |
| **Version Management** | Multiple version support | ✅ Multiple versions tested successfully | ✅ if len(versions) > 1 |
| **Functional Equivalence** | Go/Python compatibility | ✅ Both implementations work | ✅ when both succeed |
| **Go Install Time** | Hook installation performance | ✅ Installation succeeds | 4-200ms typical |
| **Python Install Time** | Baseline comparison | ✅ Installation succeeds | 50-500ms typical |
| **Performance Ratio** | Speed improvement | ✅ Any positive ratio | 10-50x faster |
| **Go Cache Efficiency** | Cache performance gain | ✅ 20%+ for cacheable ops | 40-90% typical |
| **Python Cache Efficiency** | Baseline cache performance | ✅ 20%+ for cacheable ops | 30-80% typical |
| **Bidirectional Cache** | Cross-implementation cache | ✅ Cache sharing works | Python only |

### Output Interpretation

#### Success Indicators
- `🚀 Starting comprehensive compatibility test for [language]`
- `✅ Repository/Environment setup completed successfully`
- `🎉 Language compatibility test PASSED for [language] in [duration]`

#### Progress Indicators
- `🧪 Testing version: [version]`
- `📁 Mock repository created at: [path]`
- `🏗️ Testing environment setup for [language] version [version]`
- `🔍 Running validation: [validation-name]`
- `🔄 Running performance benchmarks for [language]`

#### Warning Indicators
- `⚠️ Warning: [description]`
- `⚠️ Environment health check failed for [language] version [version]`

#### Error Indicators
- `❌ Repository/Environment setup failed: [error]`
- `💥 Language compatibility test FAILED for [language] in [duration] (errors: [count])`

## Language Test Matrix

| Language | Versions Tested | Hook Used | Bidirectional Cache | Runtime Required | Notes |
|----------|----------------|-----------|-------------------|-----------------|-------|
| Python | 3.8, 3.9, 3.10, 3.11, 3.12 | black | ✅ Yes | ❌ No | Full compatibility testing |
| Node.js | 14, 16, 18, 20 | prettier | ❌ No | ❌ No | Standard environment testing |
| Go | default, 1.19, 1.20, 1.21 | gofmt | ❌ No | ❌ No | Build and module testing |
| Rust | default, 1.70, 1.71 | rustfmt | ❌ No | ❌ No | Cargo integration testing |
| Ruby | default, 3.0, 3.1, 3.2 | rubocop | ❌ No | ❌ No | Gem management testing |
| Swift | default, 5.7, 5.8 | SwiftFormat | ❌ No | ⚠️ Optional | Requires Xcode/Swift tools |
| Lua | default, 5.3, 5.4 | LuaFormatter | ❌ No | ⚠️ Optional | System Lua if available |
| Perl | default, 5.32, 5.34 | perlcritic | ❌ No | ⚠️ Optional | System Perl if available |
| R | default | styler | ❌ No | ✅ Yes | Requires R installation |
| Haskell | default, system | hindent | ❌ No | ⚠️ Optional | GHC and Stack if available |
| Julia | default, 1.8, 1.9, 1.10 | julia-formatter | ❌ No | ✅ Yes | Requires Julia installation |
| .NET | default, 6.0, 7.0, 8.0 | dotnet-format | ❌ No | ⚠️ Optional | .NET SDK if available |
| Coursier | default | scalafmt | ❌ No | ✅ Yes | Requires cs/coursier |
| Docker | default | hadolint | ❌ No | ✅ Yes | Requires Docker daemon |
| Docker Image | default | hadolint | ❌ No | ✅ Yes | Requires Docker daemon |
| Conda | default, 3.8, 3.9, 3.10, 3.11 | black | ❌ No | ✅ Yes | Requires Conda installation |
| System | default | trailing-whitespace | ❌ No | ❌ No | Uses system commands |
| Script | default | custom script | ❌ No | ❌ No | Shell script execution |
| Fail | default | no-commit-to-branch | ❌ No | ❌ No | Failure handling testing |
| Pygrep | default | python-no-eval | ❌ No | ❌ No | Pattern matching testing |

### Legend
- ✅ Yes: Feature is supported/required
- ❌ No: Feature is not supported/not required
- ⚠️ Optional: Feature works if runtime is installed

## Implementation Details

### Python Language Implementation

**File**: `tests/integration/languages/python.go`  
**Special Features**: Bidirectional cache testing, comprehensive compatibility validation

#### Repository Setup
```go
func (pt *PythonLanguageTest) SetupRepositoryFiles(repoPath string) error {
    setupContent := "from setuptools import setup\nsetup(name='test')"
    if err := os.WriteFile(filepath.Join(repoPath, "setup.py"), []byte(setupContent), 0o600); err != nil {
        return fmt.Errorf("failed to create setup.py: %w", err)
    }
    return nil
}
```

#### Core Validations

1. **python-executable-check**: Validates Python interpreter availability
2. **pip-check**: Validates package manager functionality
3. **virtualenv-structure-check**: Validates environment structure
4. **python-version-compatibility-test**: Cross-implementation version validation
5. **cache-database-compatibility-test**: Database schema compatibility
6. **cache-hit-performance-test**: Cache performance measurement

#### Bidirectional Cache Testing

**Critical Requirement**: True bidirectional cache compatibility means:
1. Implementation A creates complete cache (install hooks + run once)
2. Implementation B uses A's cache (run only) - MUST CHANGE NOTHING
3. Cache state before/after must be bit-for-bit identical

```go
func (pt *PythonLanguageTest) TestBidirectionalCacheCompatibility(
    t *testing.T,
    pythonBinary, goBinary string,
    testRepo string,
) error {
    // Test 1: Create cache with Go, use with Python (no changes allowed)
    if err := pt.testGoCacheWithPython(t, goBinary, pythonBinary, repoDir); err != nil {
        return fmt.Errorf("Go→Python cache test failed: %w", err)
    }

    // Test 2: Create cache with Python, use with Go (no changes allowed)
    if err := pt.testPythonCacheWithGo(t, pythonBinary, goBinary, repoDir); err != nil {
        return fmt.Errorf("Python→Go cache test failed: %w", err)
    }

    return nil
}
```

### Node.js Language Implementation

**File**: `tests/integration/languages/node.go`

#### Repository Setup
```go
func (nt *NodeLanguageTest) SetupRepositoryFiles(repoPath string) error {
    packageJSON := `{
  "name": "test-project",
  "version": "1.0.0",
  "description": "Test project for pre-commit integration",
  "main": "index.js",
  "scripts": {
    "test": "echo \"Error: no test specified\" && exit 1"
  },
  "author": "",
  "license": "ISC"
}`
    // Write package.json and index.js files...
}
```

#### Core Validations

1. **node-executable-check**: Node.js runtime validation
2. **npm-executable-check**: NPM package manager validation
3. **node-version-check**: Version compatibility validation
4. **package-installation-test**: Package installation capability

### Go Language Implementation

**File**: `tests/integration/languages/go.go`

#### Repository Setup
```go
func (gt *GoLanguageTest) SetupRepositoryFiles(repoPath string) error {
    goMod := `module test-project

go 1.19
`
    mainGo := `package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
}
`
    // Write go.mod and main.go files...
}
```

#### Core Validations

1. **go-executable-check**: Go compiler validation
2. **go-version-check**: Version compatibility validation
3. **go-mod-support-check**: Go modules support
4. **build-capability-test**: Build functionality validation

### System Languages

#### Pygrep Implementation
**File**: `tests/integration/languages/pygrep.go`

Complex validation with database and repository management:

```go
func (pt *PygrepLanguageTest) validateRepositorySync(t *testing.T, envPath, version string, lang language.Manager) error {
    // Create test repository
    repoHash := pt.generateRepositoryHash()
    repoPath := filepath.Join(envPath, "repos", repoHash)
    
    // Setup environment and create database records
    if err := lang.SetupEnvironment(envPath, version, []string{}); err != nil {
        return fmt.Errorf("failed to setup environment: %w", err)
    }

    // Test file locking and database validation
    if err := pt.testFileLocking(lockPath); err != nil {
        return fmt.Errorf("file locking test failed: %w", err)
    }

    return nil
}
```

## Test Phases

### Phase 1: Repository and Environment Setup

**Duration**: 30 seconds - 5 minutes per language  
**Purpose**: Validate environment creation and repository management

**Test Flow**:
1. Create temporary test workspace
2. For each supported version:
   - Create mock repository with language-specific files
   - Setup language environment (virtualenv, nodeenv, etc.)
   - Validate environment structure and health
   - Run language-specific validations
3. Test repository synchronization and caching

**Components Tested**:
- Repository cloning and caching
- Environment creation (virtualenv, nodeenv, etc.)
- Language runtime detection
- Dependency installation
- Environment health checks

### Phase 2: Performance Benchmarking

**Duration**: 1-3 minutes per language  
**Purpose**: Measure and compare performance between implementations

**Metrics Collected**:
- Repository setup time
- Environment creation time
- Hook installation time
- Cache efficiency percentage
- Overall execution time

**Benchmarks**:
- **Cold Start**: First-time repository and environment setup
- **Warm Cache**: Subsequent runs with cached environments
- **Cross-Implementation**: Go binary vs Python pre-commit

```go
func (te *TestExecutor) runPerformanceBenchmarks(
    t *testing.T,
    test LanguageCompatibilityTest,
    testDir string,
    result *TestResults,
) {
    // Benchmark Go implementation
    goTime, err := te.benchmarkGoImplementation(test, testDir)
    if err == nil {
        result.GoInstallTime = goTime
    }

    // Benchmark Python implementation
    pythonTime, err := te.benchmarkPythonImplementation(test, testDir)
    if err == nil {
        result.PythonInstallTime = pythonTime
    }

    // Test cache performance
    te.testCachePerformance(t, test, testDir, result)
}
```

### Phase 3: Bidirectional Cache Testing (Python Only)

**Duration**: 2-5 minutes  
**Purpose**: Validate cache compatibility between implementations

**Test Scenarios**:
- Go implementation creates cache → Python reads and validates
- Python creates cache → Go implementation reads and validates
- Database schema compatibility verification
- File lock compatibility testing
- Repository hash validation

**Success Criteria**:
- Cache files remain unchanged when used by opposite implementation
- Database schemas are compatible
- Performance benefits maintained across implementations

## Configuration and Customization

### Test Configuration Structure

```go
type LanguageCompatibilityTest struct {
    PythonPrecommitBinary    string        // Path to Python pre-commit binary
    Language                 string        // Language name
    TestRepository           string        // Test repository URL
    TestCommit               string        // Specific commit to test
    HookID                   string        // Hook ID to test
    GoPrecommitBinary        string        // Path to Go pre-commit binary
    Name                     string        // Test name
    ExpectedFiles            []string      // Expected files after setup
    TestVersions             []string      // Versions to test
    AdditionalDependencies   []string      // Additional dependencies
    TestTimeout              time.Duration // Test timeout
    NeedsRuntimeInstalled    bool          // Whether runtime must be pre-installed
    CacheTestEnabled         bool          // Whether to run cache tests
    BiDirectionalTestEnabled bool          // Whether to run bidirectional tests
}
```

### Custom Test Configurations

```go
// Python with comprehensive testing
func createPythonTest() LanguageCompatibilityTest {
    return LanguageCompatibilityTest{
        Language:                 "python",
        TestVersions:             []string{"3.8", "3.9", "3.10", "3.11", "3.12"},
        HookID:                   "black",
        TestRepository:           "https://github.com/psf/black",
        TestCommit:               "23.3.0",
        CacheTestEnabled:         true,
        BiDirectionalTestEnabled: true,
        TestTimeout:              5 * time.Minute,
    }
}

// Go with module testing
func createGoTest() LanguageCompatibilityTest {
    return LanguageCompatibilityTest{
        Language:                 "golang",
        TestVersions:             []string{"default", "1.19", "1.20", "1.21"},
        HookID:                   "gofmt",
        TestRepository:           "https://github.com/dnephin/pre-commit-golang",
        CacheTestEnabled:         true,
        BiDirectionalTestEnabled: false,
        TestTimeout:              3 * time.Minute,
    }
}
```

## Test Measurements and Pass/Fail Criteria

This section provides a comprehensive overview of all measurements collected during testing and the criteria used to determine test success or failure.

### Core Test Measurements

#### 1. Repository and Environment Setup
**What it measures**: Ability to create working language environments and repositories  
**Pass criteria**: 
- ✅ Repository successfully cloned/created
- ✅ Language environment successfully set up (if `NeedsEnvironmentSetup() == true`)
- ✅ Environment health check passes (if `NeedsRuntimeInstalled == true`)
- ✅ All expected files created in repository

**Fail criteria**:
- ❌ Repository creation fails
- ❌ Environment setup fails when required
- ❌ Health check fails when runtime is required (`NeedsRuntimeInstalled: true`)
- ❌ Missing required dependencies when `NeedsRuntimeInstalled: true`

#### 2. Environment Isolation (`EnvironmentIsolation`)
**What it measures**: Whether language environments are properly isolated between tests  
**Pass criteria**: 
- ✅ Each test gets its own separate environment directory
- ✅ No dependency conflicts between language environments
- ✅ Tests can run in parallel without interference

**Fail criteria**:
- ❌ Environment conflicts detected
- ❌ Shared state between test runs

#### 3. Version Management (`VersionManagement`)
**What it measures**: Support for multiple language versions  
**Pass criteria**: 
- ✅ Multiple versions successfully tested (when `len(TestVersions) > 1`)
- ✅ Version-specific environments created correctly

**Automatic pass**: Languages with only one test version (e.g., `["default"]`)

#### 4. Functional Equivalence (`FunctionalEquivalence`)
**What it measures**: Whether Go and Python implementations produce equivalent results  
**Pass criteria**: 
- ✅ Both Go and Python implementations can install and run hooks successfully
- ✅ Go implementation extends Python capabilities (e.g., coursier, dotnet support)
- ✅ Hook execution produces compatible results

**Fail criteria**:
- ❌ Go implementation fails when Python succeeds
- ❌ Incompatible output between implementations

### Performance Measurements

#### 5. Go Install Time (`GoInstallTime`)
**What it measures**: Time to install hooks using Go pre-commit implementation  
**Pass criteria**: 
- ✅ Hook installation completes successfully
- ✅ Reasonable performance (typically 4-50ms for most languages)

**Fail criteria**:
- ❌ Installation fails
- ❌ Timeout exceeded

#### 6. Python Install Time (`PythonInstallTime`)
**What it measures**: Time to install hooks using Python pre-commit implementation  
**Pass criteria**: 
- ✅ Hook installation completes successfully (when Python binary available)
- ✅ Comparable functionality to Go implementation

**Skip conditions**: 
- Python pre-commit binary not available
- Language not supported by Python pre-commit (e.g., coursier, dotnet)

#### 7. Performance Ratio (`PerformanceRatio`)
**What it measures**: Speed comparison between Python and Go implementations  
**Calculation**: `PythonInstallTime / GoInstallTime`  
**Typical results**: 
- ✅ 10-50x faster (typical range)
- ✅ Any positive ratio indicates Go performance benefit

**Skip conditions**: Either implementation unavailable

### Cache Performance Measurements

#### 8. Go Cache Efficiency (`GoCacheEfficiency`)
**What it measures**: Performance improvement from caching in Go implementation  
**Calculation**: `((FirstRunTime - CachedRunTime) / FirstRunTime) * 100`  
**Pass criteria**: 
- ✅ 20%+ improvement for cacheable operations
- ✅ 5%+ improvement for fast operations (<200ms)
- ✅ 0% acceptable for very fast operations where cache overhead dominates

**Special handling**:
- Compiled languages (Go, Rust): Use estimated cache efficiency based on build system characteristics
- Fast operations: Limited cache benefit expected and acceptable

#### 9. Python Cache Efficiency (`PythonCacheEfficiency`)
**What it measures**: Performance improvement from caching in Python implementation  
**Same calculation and criteria as Go Cache Efficiency**

#### 10. Bidirectional Cache Compatibility (`CacheBidirectional`)
**What it measures**: Whether Go and Python implementations can share cache artifacts  
**Pass criteria**: 
- ✅ Go can use cache created by Python
- ✅ Python can use cache created by Go
- ✅ No cache corruption or conflicts

**Tested for**: Python language only (where both implementations are feature-complete)

### Test Execution Status

#### 11. Overall Success (`Success`)
**What it determines**: Final test result  
**Pass criteria**: ALL of the following must be true:
- ✅ Repository/Environment setup completed successfully
- ✅ No errors recorded during test execution
- ✅ Required runtime available (when `NeedsRuntimeInstalled: true`)
- ✅ All mandatory validations passed

**Fail criteria**: ANY of the following:
- ❌ Setup phase failed
- ❌ Errors recorded in `Errors` array
- ❌ Required runtime missing when mandatory
- ❌ Critical validation failures

### Language-Specific Considerations

#### Runtime Requirements
- **Strict requirement** (`NeedsRuntimeInstalled: true`): Lua, Perl, R, Julia, Haskell, Docker, Coursier
  - Test fails if runtime not available
- **Optional runtime** (`NeedsRuntimeInstalled: false`): System, Script, Fail, Pygrep
  - Test continues with warnings if runtime unavailable

#### Expected Performance Ranges
- **Very fast** (1-10ms): System, Script, Fail, Pygrep languages
- **Fast** (10-50ms): Node, Go, Rust (with caching)
- **Moderate** (50-200ms): Python, Ruby, Dart
- **Slow** (200ms+): Julia, Haskell, Docker (first run)

#### Cache Efficiency Expectations
- **High** (70-90%): Conda, Docker, compiled languages
- **Moderate** (40-70%): Python, Ruby, Node
- **Limited** (<40%): System commands, simple scripts

### Error Handling and Warnings

#### Test Warnings (Non-failing)
- Cache test skipped due to missing dependencies
- Environment health check failed for optional runtime
- Performance measurement unavailable
- Python implementation not available for comparison

#### Test Errors (Failing)
- Repository setup failure
- Required runtime missing
- Environment creation failure when mandatory
- Hook installation failure
- Critical validation errors

### Interpreting Test Results

#### Success Indicators in Logs
```
🚀 Starting comprehensive compatibility test for [language]
✅ Repository/Environment setup completed successfully  
✅ Environment health check passed for [language] version [version]
✅ Functional equivalence confirmed: both implementations working
🎉 Language compatibility test PASSED for [language] in [duration]
```

#### Failure Indicators in Logs
```
❌ Repository/Environment setup failed: [error]
❌ Environment health check failed for [language] (runtime required): [error]  
💥 Language compatibility test FAILED for [language] in [duration] (errors: [count])
```

#### Warning Indicators in Logs
```
⚠️ Warning: Environment health check failed for [language] version [version]
⚠️ Cache test skipped for [language] - [reason]
ℹ️ Language [language] does not support multiple versions
```

This measurement framework ensures comprehensive validation of compatibility, performance, and functionality across all supported languages while providing clear criteria for determining test success or failure.

## Performance Analysis

### Cache Efficiency Calculation

Cache efficiency is calculated as performance improvement from cache usage:

```go
func (te *TestExecutor) measureCachePerformanceImprovement(
    t *testing.T,
    testRepo string,
    binary string,
) (float64, error) {
    // First run: Creates cache
    firstRunTime, err := te.measureSingleRun(testRepo, binary)
    if err != nil {
        return 0, err
    }

    // Subsequent runs: Use cache
    var cachedTimes []time.Duration
    for i := 0; i < 3; i++ {
        cachedTime, err := te.measureSingleRun(testRepo, binary)
        if err != nil {
            continue
        }
        cachedTimes = append(cachedTimes, cachedTime)
    }

    // Calculate improvement percentage
    avgCachedTime := te.calculateAverage(cachedTimes)
    improvement := (float64(firstRunTime) - float64(avgCachedTime)) / float64(firstRunTime) * 100
    
    return improvement, nil
}
```

### Performance Thresholds

- **Cache Efficiency**: Expected >80% improvement for cached operations
- **Environment Reuse**: Subsequent runs should be significantly faster
- **Cross-Implementation Parity**: Go implementation should be competitive with Python

### Performance Metrics Output

Test results include detailed performance data:

```json
{
  "timestamp": "2025-07-18T19:29:37.804745-04:00",
  "language": "python",
  "functional_equivalence": true,
  "cache_bidirectional": true,
  "environment_isolation": true,
  "python_install_time": 221.78,
  "go_install_time": 4.81,
  "test_duration": 125970.42,
  "performance_ratio": 46.1,
  "python_cache_efficiency": 89,
  "go_cache_efficiency": 94.9
}
```

## Troubleshooting Guide

### Common Issues and Solutions

#### Runtime Not Found Errors
```
❌ Repository/Environment setup failed: [language] runtime not found
```

**Solutions**:
1. **Install the runtime**: Follow installation instructions in error message
2. **Skip the language**: Runtime is optional for most languages
3. **Use system runtime**: Some languages can use system-installed versions

#### Permission Errors
```
❌ Repository/Environment setup failed: permission denied
```

**Solutions**:
1. **Check directory permissions**: Ensure write access to test directories
2. **Run with proper permissions**: May need elevated permissions on some systems
3. **Change temp directory**: Use `TMPDIR` environment variable if needed

#### Network/Download Errors
```
❌ Repository/Environment setup failed: failed to git commit
```

**Solutions**:
1. **Check internet connection**: Tests download repositories and dependencies
2. **Check firewall settings**: Ensure access to GitHub and package repositories
3. **Use cached repositories**: Some failures are due to repository access issues

#### Timeout Errors
```
panic: test timed out after 30m0s
```

**Solutions**:
1. **Increase timeout**: Add `-timeout 60m` for slower systems
2. **Run subset of tests**: Use specific test groups instead of full suite
3. **Check system resources**: Ensure adequate CPU and memory

#### Docker-Related Errors
```
❌ Repository/Environment setup failed: docker daemon is not accessible
```

**Solutions**:
1. **Start Docker**: Ensure Docker daemon is running
2. **Check Docker permissions**: User must have Docker access
3. **Skip Docker tests**: Docker tests are optional

### Debug Information Collection

#### Enable Verbose Output
```bash
go test -v ./tests/ -run TestAllLanguagesCompatibility 2>&1 | tee debug.log
```

#### Log Analysis Patterns
Look for these patterns in logs:
- **Setup Issues**: Environment creation failures
- **Runtime Issues**: Language runtime problems
- **Performance Issues**: Unexpected timing results
- **Compatibility Issues**: Cross-implementation problems

### Test Result Files

Tests generate comprehensive output in `test-output/` directory:
```
test-output/
├── [language]_test_results.json    # Individual language results
├── test_results_summary.json       # Overall summary
├── test_summary.md                 # Human-readable summary
└── categories/                     # Results by category
    ├── core_languages.json
    ├── system_languages.json
    └── container_languages.json
```

## Development Guide

### Adding New Language Tests

1. **Create language file**: `tests/integration/languages/newlang.go`
2. **Implement interface**: Implement `LanguageTestRunner` interface
3. **Define repository setup**: Create `SetupRepositoryFiles()` method
4. **Add validations**: Implement `GetAdditionalValidations()` method
5. **Add to suite**: Register language in test configuration
6. **Test thoroughly**: Run tests to ensure proper functionality

### Example New Language Implementation

```go
package languages

import (
    "fmt"
    "os"
    "path/filepath"
    "testing"
)

type NewLanguageTest struct {
    *BaseLanguageTest
}

func NewNewLanguageTest(testDir string) *NewLanguageTest {
    return &NewLanguageTest{
        BaseLanguageTest: NewBaseLanguageTest("newlang", testDir),
    }
}

func (nt *NewLanguageTest) SetupRepositoryFiles(repoPath string) error {
    // Create language-specific files
    configContent := "# Language configuration"
    if err := os.WriteFile(filepath.Join(repoPath, "config.conf"), []byte(configContent), 0o600); err != nil {
        return fmt.Errorf("failed to create config.conf: %w", err)
    }
    return nil
}

func (nt *NewLanguageTest) GetLanguageManager() (language.Manager, error) {
    // Return language manager implementation
    registry := languages.NewLanguageRegistry()
    langImpl, exists := registry.GetLanguage("newlang")
    if !exists {
        return nil, fmt.Errorf("language newlang not found in registry")
    }
    return langImpl.(language.Manager), nil
}

func (nt *NewLanguageTest) GetAdditionalValidations() []ValidationStep {
    return []ValidationStep{
        {
            Name:        "newlang-executable-check",
            Description: "New language executable validation",
            Execute: func(t *testing.T, envPath, _ string, _ language.Manager) error {
                // Implement validation logic
                executable := filepath.Join(envPath, "bin", "newlang")
                if _, err := os.Stat(executable); os.IsNotExist(err) {
                    return fmt.Errorf("newlang executable not found")
                }
                return nil
            },
        },
    }
}

func (nt *NewLanguageTest) GetLanguageName() string {
    return "newlang"
}
```

### Modifying Existing Tests

1. **Update validation logic**: Modify `GetAdditionalValidations()` method
2. **Change test versions**: Update `TestVersions` in test configuration
3. **Modify hooks**: Change `HookID` and `TestRepository` if needed
4. **Update documentation**: Keep docs in sync with changes

### Best Practices

#### Test Development
1. **Follow existing patterns**: Use existing language tests as templates
2. **Add comprehensive logging**: Include informative log messages
3. **Handle errors gracefully**: Provide clear error messages
4. **Test incrementally**: Validate individual components before integration

#### Running Tests During Development
```bash
# Quick validation of specific language
go test -v ./tests/ -run TestAllLanguagesCompatibility/python -timeout 5m

# Test specific phase
go test -v ./tests/ -run TestCoreLanguages -timeout 10m

# Debug specific validation
go test -v ./tests/ -run TestAllLanguagesCompatibility/python 2>&1 | grep "validation"
```

#### Performance Testing
```bash
# Analyze cache performance
go test -v ./tests/ -run TestAllLanguagesCompatibility/python 2>&1 | grep "cache.*efficiency"

# Compare implementations
go test -v ./tests/ -run TestAllLanguagesCompatibility 2>&1 | grep "performance.*ratio"
```

### Error Handling Patterns

#### Graceful Failure Handling
```go
func (te *TestExecutor) handleTestError(t *testing.T, test LanguageCompatibilityTest, err error) {
    // Log error with context
    t.Logf("❌ Test failed for %s: %v", test.Language, err)

    // Continue with other tests instead of failing completely
    te.suite.AddFailedTest(test.Language, err)
}
```

#### Resource Cleanup
```go
func (te *TestExecutor) cleanupTestDirectory(t *testing.T, testDir string) {
    if te.suite.preserveCache {
        t.Logf("🔍 Preserving cache directory for inspection: %s", testDir)
        return
    }
    
    if err := os.RemoveAll(testDir); err != nil {
        t.Logf("⚠️ Failed to cleanup test directory %s: %v", testDir, err)
    }
}
```

## Quick Command Reference

### Essential Commands
```bash
# Full test suite
make test-integration                           # Complete integration test suite
make test-core                                 # Core languages only
make test-python                               # Python tests only

# Manual test execution
go test -v ./tests/ -run [pattern] -timeout 30m     # Run specific tests
go test -v ./tests/ -run [pattern] 2>&1 | tee log   # Save output
go test -v ./tests/ -run [pattern] | grep "❌\|⚠️"  # Show only issues
```

### Analysis Commands
```bash
# Success/failure analysis
grep "🎉.*PASSED" test-results.log             # Count successful tests
grep "💥.*FAILED" test-results.log             # Count failed tests

# Performance analysis
grep "speedup\|efficiency" test-results.log    # Performance metrics
grep "performance.*ratio" test-results.log     # Cross-implementation comparison
```

### Test-Specific Commands
```bash
# Language-specific testing
TEST_LANGUAGE=python ./scripts/test-language-implementations.sh
TEST_LANGUAGE=node ./scripts/test-language-implementations.sh

# Bidirectional cache testing (Python only)
go test -v ./tests/ -run TestAllLanguagesCompatibility/python | grep "bidirectional\|cache"

# Performance benchmarking
go test -v ./tests/ -run TestAllLanguagesCompatibility | grep "⏱️\|performance"
```

---

This comprehensive guide covers all aspects of the integration testing framework, from quick usage to detailed implementation. For specific issues or advanced customization, refer to the individual sections or examine the source code in the `tests/integration/` directory.
