#!/usr/bin/env zsh
# shellcheck shell=bash
# shellcheck disable=SC2129,SC2155

# Language Integration Test Script
# This script runs comprehensive language compatibility tests between the Go and Python implementations of pre-commit

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "$0")" &>/dev/null && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
TEST_OUTPUT_DIR="$PROJECT_ROOT/test-output"
GO_BINARY="${GO_PRECOMMIT_BINARY:-$PROJECT_ROOT/bin/pre-commit}"
PYTHON_BINARY="${PYTHON_PRECOMMIT_BINARY:-pre-commit}"
TEST_TIMEOUT="${TEST_TIMEOUT:-60m}"

# Language categories
typeset -A LANGUAGE_CATEGORIES
LANGUAGE_CATEGORIES=(
  ["core"]="python python3 node golang rust ruby"
  ["mobile"]="dart swift"
  ["scripting"]="lua perl r"
  ["academic"]="haskell julia"
  ["enterprise"]="dotnet coursier"
  ["container"]="docker docker_image conda"
  ["system"]="system script fail pygrep"
)

# Cache efficiency expectations by category
typeset -A CACHE_EXPECTATIONS
CACHE_EXPECTATIONS=(
  ["core"]="60-80% (environment setup reuse)"
  ["mobile"]="40-60% (toolchain reuse)"
  ["scripting"]="30-50% (runtime setup)"
  ["academic"]="40-60% (package management)"
  ["enterprise"]="50-70% (SDK and dependencies)"
  ["container"]="50-70% (image and runtime caching)"
  ["system"]="5-15% (config parsing only - low cache value expected)"
)

# Usage information
usage() {
  echo "Usage: $0 [COMMAND] [OPTIONS]"
  echo ""
  echo "Commands:"
  echo "  all                    Run tests for all languages (default)"
  echo "  core                   Run tests for core programming languages"
  echo "  mobile                 Run tests for mobile development languages"
  echo "  scripting              Run tests for scripting languages"
  echo "  academic               Run tests for academic/functional languages"
  echo "  enterprise             Run tests for enterprise/JVM languages"
  echo "  container              Run tests for container-based languages"
  echo "  system                 Run tests for system/utility languages"
  echo "  categories             Run tests for all languages grouped by category"
  echo "  list                   List all configured languages"
  echo "  <language>             Run tests for a specific language"
  echo "  system-lang            Run tests for the system language specifically"
  echo ""
  echo "Options:"
  echo "  -h, --help             Show this help message"
  echo "  -v, --verbose          Enable verbose output"
  echo "  -t, --timeout TIMEOUT  Set test timeout (default: 60m)"
  echo "  --go-binary PATH       Path to Go pre-commit binary"
  echo "  --python-binary PATH   Path to Python pre-commit binary"
  echo "  --output-dir PATH      Output directory for test results"
  echo ""
  echo "Environment Variables:"
  echo "  GO_PRECOMMIT_BINARY    Path to Go pre-commit binary"
  echo "  PYTHON_PRECOMMIT_BINARY Path to Python pre-commit binary"
  echo "  TEST_TIMEOUT           Test timeout (e.g., 30m, 1h)"
  echo ""
  echo "Examples:"
  echo "  $0 all                 # Run all language tests"
  echo "  $0 core                # Run core language tests"
  echo "  $0 python              # Run Python-specific tests"
  echo "  $0 list                # List all languages"
  echo "  $0 --verbose core      # Run core tests with verbose output"
}

# Logging functions
log_info() {
  echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
  echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
  echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
  echo -e "${RED}[ERROR]${NC} $1"
}

# Check if binary exists and is executable
check_binary() {
  local binary_path="$1"
  local binary_name="$2"

  if [[ ! -f "$binary_path" ]]; then
    log_error "$binary_name binary not found at: $binary_path"
    return 1
  fi

  if [[ ! -x "$binary_path" ]]; then
    log_error "$binary_name binary is not executable: $binary_path"
    return 1
  fi

  log_info "$binary_name binary found: $binary_path"
  return 0
}

# Setup test environment
setup_environment() {
  log_info "Setting up test environment..."

  # Create output directory
  mkdir -p "$TEST_OUTPUT_DIR"

  # Check Go binary
  if ! check_binary "$GO_BINARY" "Go pre-commit"; then
    log_error "Go pre-commit binary is required. Build it with: mage build:binary"
    exit 1
  fi

  # Check Python binary (optional)
  if command -v "$PYTHON_BINARY" >/dev/null 2>&1; then
    log_info "Python pre-commit binary found: $PYTHON_BINARY"
  else
    log_warn "Python pre-commit binary not found. Bidirectional cache tests will be skipped."
    PYTHON_BINARY=""
  fi

  # Change to project root
  cd "$PROJECT_ROOT"

  log_success "Test environment setup complete"
}

# Run Go tests for a specific category or language
run_go_tests() {
  local test_type="$1"
  local test_name="$2"

  log_info "Running Go tests for $test_name..."

  local test_env=(
    "GO_PRECOMMIT_BINARY=$GO_BINARY"
    "PYTHON_PRECOMMIT_BINARY=$PYTHON_BINARY"
  )

  if [[ "$test_type" == "single" ]]; then
    test_env+=("TEST_LANGUAGE=$test_name")
    env "${test_env[@]}" go test ./tests -run TestSingleLanguage -v -timeout "$TEST_TIMEOUT"
  elif [[ "$test_type" == "category" ]]; then
    case "$test_name" in
      "core" | "TestCoreLanguages")
        env "${test_env[@]}" go test ./tests -run TestCoreLanguages -v -timeout "$TEST_TIMEOUT"
        ;;
      "system" | "TestSystemLanguages")
        env "${test_env[@]}" go test ./tests -run TestSystemLanguages -v -timeout "$TEST_TIMEOUT"
        ;;
      "container" | "TestContainerLanguages")
        env "${test_env[@]}" go test ./tests -run TestContainerLanguages -v -timeout "$TEST_TIMEOUT"
        ;;
      "mobile" | "TestMobileLanguages")
        env "${test_env[@]}" go test ./tests -run TestMobileLanguages -v -timeout "$TEST_TIMEOUT"
        ;;
      "scripting" | "TestScriptingLanguages")
        env "${test_env[@]}" go test ./tests -run TestScriptingLanguages -v -timeout "$TEST_TIMEOUT"
        ;;
      "academic" | "TestAcademicLanguages")
        env "${test_env[@]}" go test ./tests -run TestAcademicLanguages -v -timeout "$TEST_TIMEOUT"
        ;;
      "enterprise" | "TestEnterpriseLanguages")
        env "${test_env[@]}" go test ./tests -run TestEnterpriseLanguages -v -timeout "$TEST_TIMEOUT"
        ;;
      "all_categories" | "TestLanguagesByCategory")
        env "${test_env[@]}" go test ./tests -run TestLanguagesByCategory -v -timeout "$TEST_TIMEOUT"
        ;;
      *)
        log_error "Unknown category: $test_name"
        return 1
        ;;
    esac
  elif [[ "$test_type" == "all" ]]; then
    env "${test_env[@]}" go test ./tests -run TestAllLanguagesCompatibility -v -timeout "$TEST_TIMEOUT"
  elif [[ "$test_type" == "list" ]]; then
    env "${test_env[@]}" go test ./tests -run TestListAllLanguages -v
  else
    log_error "Unknown test type: $test_type"
    return 1
  fi
}

# Run shell-based tests for a category
run_shell_tests() {
  local category="$1"
  local languages="${LANGUAGE_CATEGORIES[$category]}"

  log_info "Running shell tests for $category languages: $languages"

  local success_count=0
  local total_count=0

  for language in $languages; do
    total_count=$((total_count + 1))
    log_info "Testing $language..."

    # Create temporary test directory
    local temp_dir
    temp_dir=$(mktemp -d)

    # Setup basic test repository
    mkdir -p "$temp_dir/test-repo"
    cd "$temp_dir/test-repo"

    git init >/dev/null 2>&1

    # Configure git for testing (disable GPG signing and set user info)
    git config user.name "Test User" >/dev/null 2>&1
    git config user.email "test@example.com" >/dev/null 2>&1
    git config commit.gpgsign false >/dev/null 2>&1

    # Create basic pre-commit config
    cat >.pre-commit-config.yaml <<EOF
repos:
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.4.0
    hooks:
      - id: trailing-whitespace
        language: $language
EOF

    # Test installation (install Git hooks + create environments)
    if timeout "$TEST_TIMEOUT" "$GO_BINARY" install-hooks >/dev/null 2>&1; then
      log_success "$language environment setup test passed"
      success_count=$((success_count + 1))
    else
      log_warn "$language environment setup test failed (may be expected)"
    fi

    # Cleanup
    cd "$PROJECT_ROOT"
    rm -rf "$temp_dir"
  done

  log_info "$category category: $success_count/$total_count tests passed"
}

# Generate test summary
generate_summary() {
  log_info "Generating test summary..."

  local summary_file="$TEST_OUTPUT_DIR/test_summary.md"

  cat >"$summary_file" <<EOF
# Language Integration Test Summary

Generated: $(date -Iseconds)

## Test Configuration

- Go Binary: \`$GO_BINARY\`
- Python Binary: \`${PYTHON_BINARY:-Not Available}\`
- Test Timeout: $TEST_TIMEOUT
- Output Directory: \`$TEST_OUTPUT_DIR\`

## Test Results

EOF

  # Add individual test results if they exist
  if ls "$TEST_OUTPUT_DIR"/*.json >/dev/null 2>&1; then
    echo "### Individual Language Results" >>"$summary_file"
    echo "" >>"$summary_file"

    for result_file in "$TEST_OUTPUT_DIR"/*.json; do
      if [[ "$result_file" != *"summary"* ]]; then
        local language
        language=$(basename "$result_file" _test_results.json)
        echo "- [$language](./${language}_test_results.json)" >>"$summary_file"
      fi
    done

    echo "" >>"$summary_file"

    # Add performance metrics section
    echo "### Performance Metrics" >>"$summary_file"
    echo "" >>"$summary_file"
    echo "| Language | Go Install Time | Python Install Time | Performance Ratio | Cache Efficiency |" >>"$summary_file"
    echo "|----------|-----------------|---------------------|-------------------|------------------|" >>"$summary_file"

    for result_file in "$TEST_OUTPUT_DIR"/*.json; do
      if [[ "$result_file" != *"summary"* ]]; then
        local language
        language=$(basename "$result_file" _test_results.json)

        # Extract metrics from JSON using python/jq if available, otherwise use grep/sed
        if command -v jq >/dev/null 2>&1; then
          local go_install_ms=$(jq -r '.go_install_time // 0' "$result_file")
          local python_install_ms=$(jq -r '.python_install_time // 0' "$result_file")
          local performance_ratio=$(jq -r '.performance_ratio // 0' "$result_file")
          local cache_efficiency=$(jq -r '.go_cache_efficiency // 0' "$result_file")
        else
          # Fallback to grep/sed if jq is not available
          local go_install_ms=$(grep '"go_install_time"' "$result_file" | sed 's/.*: *\([0-9.]*\).*/\1/')
          local python_install_ms=$(grep '"python_install_time"' "$result_file" | sed 's/.*: *\([0-9.]*\).*/\1/')
          local performance_ratio=$(grep '"performance_ratio"' "$result_file" | sed 's/.*: *\([0-9.]*\).*/\1/')
          local cache_efficiency=$(grep '"go_cache_efficiency"' "$result_file" | sed 's/.*: *\([0-9.]*\).*/\1/')
        fi

        # Values are already in milliseconds, just format them nicely
        local go_install_display=$(echo "scale=2; $go_install_ms" | bc 2>/dev/null || echo "N/A")
        local python_install_display=$(echo "scale=2; $python_install_ms" | bc 2>/dev/null || echo "N/A")
        local speedup_display=$(echo "scale=1; $performance_ratio" | bc 2>/dev/null || echo "N/A")
        local cache_rate=$(printf "%.1f" "$cache_efficiency" 2>/dev/null || echo "N/A")

        echo "| $language | ${go_install_display}ms | ${python_install_display}ms | ${speedup_display}x faster | ${cache_rate}% |" >>"$summary_file"
      fi
    done

    echo "" >>"$summary_file"
  fi

  # Add summary results if available
  if [[ -f "$TEST_OUTPUT_DIR/test_results_summary.json" ]]; then
    echo "### Summary Report" >>"$summary_file"
    echo "" >>"$summary_file"
    echo "- [JSON Summary](./test_results_summary.json)" >>"$summary_file"
    echo "- [Detailed Report](./compatibility_test_report.md)" >>"$summary_file"
    echo "" >>"$summary_file"
  fi

  cat >>"$summary_file" <<EOF
## How to Interpret Results

- **Success**: All test phases passed for the language
- **Install Time**: Time taken to install and setup the language environment
- **Cache Efficiency**: Performance improvement percentage from cache usage
  - **Core Languages** (python, node, rust, etc.): Expect 60-80% (environment reuse)
  - **Mobile/Academic** (dart, swift, haskell, etc.): Expect 40-60% (toolchain reuse)
  - **System Languages** (system, script, fail, pygrep): Expect 5-15% (config parsing only)
  - **Negative values indicate no meaningful caching opportunity (expected for simple hooks)**
- **Functional Equivalence**: Whether Go and Python implementations produce similar results
- **Bidirectional Cache**: Whether caches created by one implementation work with the other
- **Environment Isolation**: Whether different environments don't interfere with each other

## Troubleshooting

If tests fail:
1. Check that required language runtimes are installed
2. Verify network connectivity for downloading dependencies
3. Check available disk space for language environments
4. Review individual test logs for specific error messages

EOF

  log_success "Test summary generated: $summary_file"
}

# Main function
main() {
  local command="all"
  local verbose=false

  # Parse command line arguments
  while [[ $# -gt 0 ]]; do
    case $1 in
      -h | --help)
        usage
        exit 0
        ;;
      -v | --verbose)
        verbose=true
        shift
        ;;
      -t | --timeout)
        TEST_TIMEOUT="$2"
        shift 2
        ;;
      --go-binary)
        GO_BINARY="$2"
        shift 2
        ;;
      --python-binary)
        PYTHON_BINARY="$2"
        shift 2
        ;;
      --output-dir)
        TEST_OUTPUT_DIR="$2"
        shift 2
        ;;
      all | core | mobile | scripting | academic | enterprise | container | system | list)
        command="$1"
        shift
        ;;
      python | python3 | node | golang | rust | ruby | dart | swift | lua | perl | r | haskell | julia | dotnet | coursier | docker | docker_image | conda | script | fail | pygrep)
        command="$1"
        shift
        ;;
      system-lang)
        command="system-lang"
        shift
        ;;
      categories)
        command="categories"
        shift
        ;;
      *)
        log_error "Unknown option: $1"
        usage
        exit 1
        ;;
    esac
  done

  # Setup environment
  setup_environment

  # Enable verbose output if requested
  if [[ "$verbose" == "true" ]]; then
    set -x
  fi

  log_info "Starting language integration tests..."
  log_info "Command: $command"

  # Run tests based on command
  case "$command" in
    "all")
      run_go_tests "all" "all"
      ;;
    "core")
      run_go_tests "category" "TestCoreLanguages"
      ;;
    "system")
      run_go_tests "category" "TestSystemLanguages"
      ;;
    "container")
      run_go_tests "category" "TestContainerLanguages"
      ;;
    "mobile")
      run_go_tests "category" "TestMobileLanguages"
      ;;
    "scripting")
      run_go_tests "category" "TestScriptingLanguages"
      ;;
    "academic")
      run_go_tests "category" "TestAcademicLanguages"
      ;;
    "enterprise")
      run_go_tests "category" "TestEnterpriseLanguages"
      ;;
    "categories")
      run_go_tests "category" "TestLanguagesByCategory"
      ;;
    "list")
      run_go_tests "list" "list"
      ;;
    python | python3 | node | golang | rust | ruby | dart | swift | lua | perl | r | haskell | julia | dotnet | coursier | docker | docker_image | conda | script | fail | pygrep)
      run_go_tests "single" "$command"
      ;;
    "system-lang")
      # Handle system language specifically to avoid conflict with system category
      run_go_tests "single" "system"
      ;;
    *)
      log_error "Unknown command: $command"
      usage
      exit 1
      ;;
  esac

  # Generate summary
  generate_summary

  log_success "Language integration tests completed!"
  log_info "Results saved to: $TEST_OUTPUT_DIR"
}

# Run main function with all arguments
main "$@"
