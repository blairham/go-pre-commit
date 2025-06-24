#!/bin/bash

# Test script to compare Python pre-commit vs Go pre-commit output
# This script runs both implementations and compares their output format
# Uses a temporary directory to avoid affecting the main workspace

# Don't exit on errors since pre-commit hooks are expected to fail in our test
set +e

REPO_DIR="/Users/bhamilton/Developer/github.com/blairham/go-pre-commit"
GO_PRECOMMIT="$REPO_DIR/bin/pre-commit"
PYTHON_PRECOMMIT="pre-commit"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Create a temporary test directory
TEMP_DIR=$(mktemp -d)
PYTHON_TEST_REPO="$TEMP_DIR/python-test-repo"
GO_TEST_REPO="$TEMP_DIR/go-test-repo"

# Cleanup function
cleanup() {
  echo -e "\n${YELLOW}Cleaning up temporary directories...${NC}"
  rm -rf "$TEMP_DIR"
}

# Set trap to cleanup on exit
trap cleanup EXIT

# Function to create a test repository
create_test_repo() {
  local test_repo=$1
  local repo_name=$2

  echo -e "${YELLOW}Setting up $repo_name test repository in $test_repo${NC}"

  # Create minimal test repository
  mkdir -p "$test_repo"
  cd "$test_repo" || exit 1

  # Initialize git repo
  git init
  git config user.name "Test User"
  git config user.email "test@example.com"
  git config commit.gpgsign false
  git config tag.gpgsign false
  git config tag.forceSignAnnotated false
  git config gpg.program ""

  # Create test configuration file
  cat >"$test_repo/.pre-commit-config.yaml" <<EOF
repos:
  - repo: local
    hooks:
      - id: test-success-multiple
        name: Test Success Hook Multiple Files
        entry: sh -c 'echo "Processing files:" && for f in "$@"; do echo "  $f"; done' --
        language: system
        files: '\.(go|py|js)$'
        pass_filenames: true
      - id: test-failure-multiple
        name: Test Failure Hook Multiple Files
        entry: sh -c 'echo "Found files:" && for f in "$@"; do echo "  - $f"; done && echo "This hook fails" && exit 1' --
        language: system
        files: '\.(go|py)$'
        pass_filenames: true
      - id: test-skipped
        name: Test Skipped Hook
        entry: echo "This hook is skipped"
        language: system
        files: "nonexistent_pattern"
EOF

  # Create multiple test files to trigger hooks
  echo "package main

import \"fmt\"

func main() {
    fmt.Println(\"test\")
}" >"$test_repo/test_file1.go"

  echo "package utils

func Helper() string {
    return \"helper\"
}" >"$test_repo/test_file2.go"

  echo "def hello():
    print('hello world')" >"$test_repo/test_file.py"

  echo "console.log('hello');" >"$test_repo/test_file.js"

  # Add the test files and commit them
  git add test_file1.go test_file2.go test_file.py test_file.js .pre-commit-config.yaml
  git commit -m "Add test files and pre-commit config"

  # Now modify the files to create changes for pre-commit to check
  echo "// Modified" >>test_file1.go
  echo "// Modified" >>test_file2.go
  echo "# Modified" >>test_file.py
  echo "// Modified" >>test_file.js

  # Add the modified files to staging
  git add test_file1.go test_file2.go test_file.py test_file.js
}

echo -e "${BLUE}Pre-commit Output Comparison Test${NC}"
echo "========================================"

# Create separate test repositories for Python and Go
create_test_repo "$PYTHON_TEST_REPO" "Python"
create_test_repo "$GO_TEST_REPO" "Go"

echo -e "\n${YELLOW}Running Python pre-commit...${NC}"
echo "----------------------------------------"
if command -v "$PYTHON_PRECOMMIT" >/dev/null 2>&1; then
  cd "$PYTHON_TEST_REPO" || exit 1
  # Clean pre-commit cache before running
  $PYTHON_PRECOMMIT clean || true
  # Force color output and verbose mode
  FORCE_COLOR=1 $PYTHON_PRECOMMIT run --color=always --verbose 2>&1 | tee "$TEMP_DIR/python_output.txt" || true
else
  echo "Python pre-commit not found. Skipping Python comparison." | tee "$TEMP_DIR/python_output.txt"
fi

echo -e "\n${YELLOW}Running Go pre-commit...${NC}"
echo "----------------------------------------"
cd "$GO_TEST_REPO" || exit 1
# Clean any potential cache before running
"$GO_PRECOMMIT" clean || true
# Force color output and verbose mode (Go uses space-separated format)
FORCE_COLOR=1 "$GO_PRECOMMIT" run --color=always --verbose 2>&1 | tee "$TEMP_DIR/go_output.txt" || true

# Change back to temp dir for analysis
cd "$TEMP_DIR" || exit 1

echo -e "\n${BLUE}Comparison Results:${NC}"
echo "========================================"

# Basic line count comparison
PYTHON_LINES=$(wc -l <python_output.txt 2>/dev/null || echo "0")
GO_LINES=$(wc -l <go_output.txt 2>/dev/null || echo "0")

echo "Python output lines: $PYTHON_LINES"
echo "Go output lines: $GO_LINES"

# Look for key patterns in both outputs
echo -e "\n${YELLOW}Pattern Analysis:${NC}"
echo "----------------------------------------"

# Check for hook status patterns
PYTHON_PASSED=$(grep -c "Passed" python_output.txt 2>/dev/null || echo "0")
GO_PASSED=$(grep -c "Passed" go_output.txt 2>/dev/null || echo "0")

PYTHON_FAILED=$(grep -c "Failed" python_output.txt 2>/dev/null || echo "0")
GO_FAILED=$(grep -c "Failed" go_output.txt 2>/dev/null || echo "0")

PYTHON_SKIPPED=$(grep -c "Skipped" python_output.txt 2>/dev/null || echo "0")
GO_SKIPPED=$(grep -c "Skipped" go_output.txt 2>/dev/null || echo "0")

echo "Passed hooks  - Python: $PYTHON_PASSED, Go: $GO_PASSED"
echo "Failed hooks  - Python: $PYTHON_FAILED, Go: $GO_FAILED"
echo "Skipped hooks - Python: $PYTHON_SKIPPED, Go: $GO_SKIPPED"

# Check for detail patterns (hook id, duration, exit code)
PYTHON_HOOK_IDS=$(grep -c "hook id:" python_output.txt 2>/dev/null || echo "0")
GO_HOOK_IDS=$(grep -c "hook id:" go_output.txt 2>/dev/null || echo "0")

PYTHON_DURATIONS=$(grep -c "duration:" python_output.txt 2>/dev/null || echo "0")
GO_DURATIONS=$(grep -c "duration:" go_output.txt 2>/dev/null || echo "0")

PYTHON_EXIT_CODES=$(grep -c "exit code:" python_output.txt 2>/dev/null || echo "0")
GO_EXIT_CODES=$(grep -c "exit code:" go_output.txt 2>/dev/null || echo "0")

echo "Hook IDs     - Python: $PYTHON_HOOK_IDS, Go: $GO_HOOK_IDS"
echo "Durations    - Python: $PYTHON_DURATIONS, Go: $GO_DURATIONS"
echo "Exit codes   - Python: $PYTHON_EXIT_CODES, Go: $GO_EXIT_CODES"

# Show visual diff of the outputs (first 50 lines)
echo -e "\n${YELLOW}Visual Diff (first 50 lines):${NC}"
echo "----------------------------------------"
echo -e "${BLUE}Python output:${NC}"
head -50 python_output.txt

echo -e "\n${BLUE}Go output:${NC}"
head -50 go_output.txt

echo -e "\n${GREEN}Test completed!${NC}"
