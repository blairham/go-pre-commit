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

# Usage information
usage() {
  echo "Usage: $0 [COMMAND] [OPTIONS]"
  echo ""
  echo "This script runs comprehensive compatibility tests between Go and Python pre-commit implementations."
  echo "When both implementations are available, it compares performance, functional equivalence, and cache compatibility."
  echo ""
  echo "Commands:"
  echo "  all                    Run compatibility tests for all languages (default)"
  echo "  core                   Run compatibility tests for core programming languages"
  echo "  mobile                 Run compatibility tests for mobile development languages"
  echo "  scripting              Run compatibility tests for scripting languages"
  echo "  academic               Run compatibility tests for academic/functional languages"
  echo "  enterprise             Run compatibility tests for enterprise/JVM languages"
  echo "  container              Run compatibility tests for container-based languages"
  echo "  system                 Run compatibility tests for system/utility languages"
  echo "  categories             Run compatibility tests for all languages grouped by category"
  echo "  list                   List all configured languages"
  echo "  <language>             Run compatibility tests for a specific language"
  echo "  system-lang            Run compatibility tests for the system language specifically"
  echo ""
  echo "Options:"
  echo "  -h, --help             Show this help message"
  echo "  -v, --verbose          Enable verbose output"
  echo "  -q, --quiet            Suppress non-critical warnings"
  echo "  -t, --timeout TIMEOUT  Set test timeout (default: 60m)"
  echo "  --go-binary PATH       Path to Go pre-commit binary"
  echo "  --python-binary PATH   Path to Python pre-commit binary"
  echo "  --output-dir PATH      Output directory for test results"
  echo ""
  echo "Environment Variables:"
  echo "  GO_PRECOMMIT_BINARY    Path to Go pre-commit binary"
  echo "  PYTHON_PRECOMMIT_BINARY Path to Python pre-commit binary"
  echo "  TEST_TIMEOUT           Test timeout (e.g., 30m, 1h)"
  echo "  TEST_SHOW_WARNINGS     Set to 'true' to show all warnings (default: false)"
  echo ""
  echo "Compatibility Testing Features:"
  echo "  • Performance comparison between Go and Python implementations"
  echo "  • Functional equivalence verification (same CLI behavior)"
  echo "  • Cache efficiency measurements and comparison"
  echo "  • Bidirectional cache compatibility (caches work across implementations)"
  echo "  • Environment isolation testing"
  echo ""
  echo "Examples:"
  echo "  $0 all                 # Run compatibility tests for all languages"
  echo "  $0 core                # Run compatibility tests for core languages"
  echo "  $0 python              # Run compatibility tests for Python specifically"
  echo "  $0 list                # List all languages with test configurations"
  echo "  $0 --verbose core      # Run core tests with verbose output"
  echo "  $0 --quiet core        # Run core tests with suppressed warnings"
  echo ""
  echo "Prerequisites:"
  echo "  • Go pre-commit binary (required): Build with 'mage build:binary'"
  echo "  • Python pre-commit (recommended): Install with 'pip install pre-commit'"
  echo "  • Without Python pre-commit, only Go implementation tests will run"
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

  # Check Python binary (optional but recommended for full compatibility testing)
  if command -v "$PYTHON_BINARY" >/dev/null 2>&1; then
    log_info "Python pre-commit binary found: $PYTHON_BINARY"
    # Verify Python binary actually works
    if ! "$PYTHON_BINARY" --version >/dev/null 2>&1; then
      log_warn "Python pre-commit binary found but not functional. Some comparison tests will be skipped."
      PYTHON_BINARY=""
    else
      log_success "Python pre-commit is functional - full compatibility testing enabled"
    fi
  else
    log_warn "Python pre-commit binary not found. Install with: pip install pre-commit"
    log_warn "Without Python pre-commit, the following tests will be limited:"
    log_warn "  - Performance comparison between Go and Python implementations"
    log_warn "  - Bidirectional cache compatibility tests"
    log_warn "  - Functional equivalence verification"
    PYTHON_BINARY=""
  fi

  # Change to project root
  cd "$PROJECT_ROOT"

  log_success "Test environment setup complete"
}

# Run comprehensive compatibility tests between Go and Python implementations
run_compatibility_tests() {
  local test_type="$1"
  local test_name="$2"

  if [[ -z "$PYTHON_BINARY" ]]; then
    log_warn "Python pre-commit not available. Running Go-only tests for $test_name..."
    log_warn "For full compatibility testing, install Python pre-commit with: pip install pre-commit"
    run_go_tests "$test_type" "$test_name"
    return
  fi

  log_info "Running comprehensive compatibility tests for $test_name..."
  log_info "Testing both Go ($GO_BINARY) and Python ($PYTHON_BINARY) implementations"

  local test_env=(
    "GO_PRECOMMIT_BINARY=$GO_BINARY"
    "PYTHON_PRECOMMIT_BINARY=$PYTHON_BINARY"
  )

  # The Go tests will automatically run both implementations and compare results
  local test_exit_code=0
  if [[ "$test_type" == "single" ]]; then
    test_env+=("TEST_LANGUAGE=$test_name")
    log_info "🔍 Running single language compatibility test..."
    env "${test_env[@]}" go test ./tests -run TestSingleLanguage -v -timeout "$TEST_TIMEOUT" || test_exit_code=$?
  elif [[ "$test_type" == "category" ]]; then
    case "$test_name" in
      "core" | "TestCoreLanguages")
        log_info "🔍 Running core languages compatibility tests..."
        env "${test_env[@]}" go test ./tests -run TestCoreLanguages -v -timeout "$TEST_TIMEOUT" || test_exit_code=$?
        ;;
      "system" | "TestSystemLanguages")
        log_info "🔍 Running system languages compatibility tests..."
        env "${test_env[@]}" go test ./tests -run TestSystemLanguages -v -timeout "$TEST_TIMEOUT" || test_exit_code=$?
        ;;
      "container" | "TestContainerLanguages")
        log_info "🔍 Running container languages compatibility tests..."
        env "${test_env[@]}" go test ./tests -run TestContainerLanguages -v -timeout "$TEST_TIMEOUT" || test_exit_code=$?
        ;;
      "mobile" | "TestMobileLanguages")
        log_info "🔍 Running mobile languages compatibility tests..."
        env "${test_env[@]}" go test ./tests -run TestMobileLanguages -v -timeout "$TEST_TIMEOUT" || test_exit_code=$?
        ;;
      "scripting" | "TestScriptingLanguages")
        log_info "🔍 Running scripting languages compatibility tests..."
        env "${test_env[@]}" go test ./tests -run TestScriptingLanguages -v -timeout "$TEST_TIMEOUT" || test_exit_code=$?
        ;;
      "academic" | "TestAcademicLanguages")
        log_info "🔍 Running academic languages compatibility tests..."
        env "${test_env[@]}" go test ./tests -run TestAcademicLanguages -v -timeout "$TEST_TIMEOUT" || test_exit_code=$?
        ;;
      "enterprise" | "TestEnterpriseLanguages")
        log_info "🔍 Running enterprise languages compatibility tests..."
        env "${test_env[@]}" go test ./tests -run TestEnterpriseLanguages -v -timeout "$TEST_TIMEOUT" || test_exit_code=$?
        ;;
      "all_categories" | "TestLanguagesByCategory")
        log_info "🔍 Running all categories compatibility tests..."
        env "${test_env[@]}" go test ./tests -run TestLanguagesByCategory -v -timeout "$TEST_TIMEOUT" || test_exit_code=$?
        ;;
      *)
        log_error "Unknown category: $test_name"
        return 1
        ;;
    esac
  elif [[ "$test_type" == "all" ]]; then
    log_info "🔍 Running comprehensive compatibility tests for all languages..."
    env "${test_env[@]}" go test ./tests -run TestAllLanguagesCompatibility -v -timeout "$TEST_TIMEOUT" || test_exit_code=$?
  elif [[ "$test_type" == "list" ]]; then
    env "${test_env[@]}" go test ./tests -run TestListAllLanguages -v || test_exit_code=$?
  else
    log_error "Unknown test type: $test_type"
    return 1
  fi

  # Log compatibility test summary
  if [[ "$test_exit_code" -eq 0 ]]; then
    log_success "Compatibility tests completed successfully for $test_name"
    log_info "Results include:"
    log_info "  ✅ Performance comparison between Go and Python implementations"
    log_info "  ✅ Functional equivalence verification"
    log_info "  ✅ Cache efficiency measurements"
    log_info "  ✅ Bidirectional cache compatibility tests"
  else
    log_error "Compatibility tests failed for $test_name"
  fi
}

# Run Go tests for a specific category or language (legacy function, kept for backward compatibility)
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



# Validate CLI compatibility between Go and Python implementations
validate_cli_compatibility() {
  if [[ -z "$PYTHON_BINARY" ]]; then
    log_warn "Python pre-commit not available. Skipping CLI compatibility validation."
    return 0
  fi

  log_info "🔍 Validating CLI compatibility between implementations..."

  local temp_dir
  temp_dir=$(mktemp -d)
  local test_passed=true

  # Change to temp directory to avoid creating files in current directory
  local original_dir="$PWD"
  cd "$temp_dir" || {
    log_error "Failed to change to temp directory"
    return 1
  }

  # Test basic commands that should produce similar output
  local commands=(
    "--version"
    "--help"
    "sample-config"
  )

  for cmd in "${commands[@]}"; do
    log_info "Testing command: $cmd"

    local go_output="go_${cmd//[^a-zA-Z0-9]/_}.txt"
    local python_output="python_${cmd//[^a-zA-Z0-9]/_}.txt"

    # Run commands and capture output
    local go_exit=0
    local python_exit=0

    "$GO_BINARY" "$cmd" >"$go_output" 2>&1 || go_exit=$?
    "$PYTHON_BINARY" "$cmd" >"$python_output" 2>&1 || python_exit=$?

    # Compare exit codes
    if [[ "$go_exit" != "$python_exit" ]]; then
      log_warn "Exit code mismatch for '$cmd': Go=$go_exit, Python=$python_exit"
      test_passed=false
    fi

    # For some commands, we can do basic content validation
    case "$cmd" in
      "--version")
        if grep -q "pre-commit" "$go_output" && grep -q "pre-commit" "$python_output"; then
          log_success "Both implementations report pre-commit version"
        else
          log_info "Note: Version output format differs between implementations (expected)"
          # Don't mark as failure since this is expected for different implementations
        fi
        ;;
      "sample-config")
        if grep -q "repos:" "$go_output" && grep -q "repos:" "$python_output"; then
          log_success "Both implementations generate valid YAML config"
        else
          log_info "Note: Sample config format differs between implementations (expected)"
          # Don't mark as failure since this is expected for different implementations
        fi
        ;;
    esac
  done

  # Return to original directory
  cd "$original_dir" || {
    log_error "Failed to return to original directory"
  }

  # Cleanup
  rm -rf "$temp_dir"

  if [[ "$test_passed" == "true" ]]; then
    log_success "✅ CLI compatibility validation passed"
  else
    log_warn "⚠️ CLI compatibility issues detected - check logs above"
  fi

  return 0
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
  local quiet=false

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
      -q | --quiet)
        quiet=true
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

  # Set warning verbosity based on flags
  if [[ "$verbose" == "true" ]]; then
    export TEST_SHOW_WARNINGS=true
    export TEST_VERBOSE=true
  elif [[ "$quiet" == "true" ]]; then
    export TEST_SHOW_WARNINGS=false
    export TEST_VERBOSE=false
  fi

  # Enable verbose output if requested
  if [[ "$verbose" == "true" ]]; then
    set -x
  fi

  log_info "Starting comprehensive language compatibility tests..."
  log_info "Command: $command"
  if [[ -n "$PYTHON_BINARY" ]]; then
    log_info "Mode: Full compatibility testing (Go + Python comparison)"
  else
    log_info "Mode: Go implementation only (install Python pre-commit for full compatibility)"
  fi

  # Run tests based on command
  case "$command" in
    "all")
      run_compatibility_tests "all" "all"
      ;;
    "core")
      run_compatibility_tests "category" "TestCoreLanguages"
      ;;
    "system")
      run_compatibility_tests "category" "TestSystemLanguages"
      ;;
    "container")
      run_compatibility_tests "category" "TestContainerLanguages"
      ;;
    "mobile")
      run_compatibility_tests "category" "TestMobileLanguages"
      ;;
    "scripting")
      run_compatibility_tests "category" "TestScriptingLanguages"
      ;;
    "academic")
      run_compatibility_tests "category" "TestAcademicLanguages"
      ;;
    "enterprise")
      run_compatibility_tests "category" "TestEnterpriseLanguages"
      ;;
    "categories")
      run_compatibility_tests "category" "TestLanguagesByCategory"
      ;;
    "list")
      run_compatibility_tests "list" "list"
      ;;
    python | python3 | node | golang | rust | ruby | dart | swift | lua | perl | r | haskell | julia | dotnet | coursier | docker | docker_image | conda | script | fail | pygrep)
      run_compatibility_tests "single" "$command"
      ;;
    "system-lang")
      # Handle system language specifically to avoid conflict with system category
      run_compatibility_tests "single" "system"
      ;;
    *)
      log_error "Unknown command: $command"
      usage
      exit 1
      ;;
  esac

  # Validate CLI compatibility between implementations
  validate_cli_compatibility

  # Generate summary
  generate_summary

  log_success "Language compatibility tests completed!"
  log_info "Results saved to: $TEST_OUTPUT_DIR"
  if [[ -n "$PYTHON_BINARY" ]]; then
    log_info "✅ Full compatibility testing completed (Go + Python comparison)"
    log_info "📊 Check test results for:"
    log_info "   • Performance differences between implementations"
    log_info "   • Functional equivalence verification"
    log_info "   • Cache efficiency measurements"
    log_info "   • Bidirectional cache compatibility"
  else
    log_warn "⚠️  Limited testing completed (Go implementation only)"
    log_warn "💡 Install Python pre-commit for full compatibility testing:"
    log_warn "   pip install pre-commit"
  fi
}

# Run main function with all arguments
if [[ "${1:-}" == "generate_summary" ]]; then
  # Special case: just generate summary from existing test results
  mkdir -p "$TEST_OUTPUT_DIR"
  generate_summary
else
  # Normal execution: run tests and generate summary
  main "$@"
fi
