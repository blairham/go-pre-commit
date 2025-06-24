package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// FileGenerator creates language-specific test files
type FileGenerator struct{}

// NewFileGenerator creates a new file generator
func NewFileGenerator() *FileGenerator {
	return &FileGenerator{}
}

// CreatePythonFiles creates Python test files
func (fg *FileGenerator) CreatePythonFiles(t *testing.T, repoDir string) {
	t.Helper()
	pyFile := filepath.Join(repoDir, "test.py")
	content := `#!/usr/bin/env python3
"""Test Python file for pre-commit testing."""

def hello_world():
    """Print hello world."""
    print("Hello, World!")


if __name__ == "__main__":
    hello_world()
`
	if err := os.WriteFile(pyFile, []byte(content), 0o600); err != nil {
		t.Fatalf("Failed to create test Python file: %v", err)
	}
}

// CreateNodeFiles creates Node.js test files
func (fg *FileGenerator) CreateNodeFiles(t *testing.T, repoDir string) {
	t.Helper()
	jsFile := filepath.Join(repoDir, "test.js")
	content := `#!/usr/bin/env node
/**
 * Test JavaScript file for pre-commit testing.
 */

function helloWorld() {
    console.log("Hello, World!");
}

if (require.main === module) {
    helloWorld();
}

module.exports = { helloWorld };
`
	if err := os.WriteFile(jsFile, []byte(content), 0o600); err != nil {
		t.Fatalf("Failed to create test JavaScript file: %v", err)
	}

	// Create package.json
	packageJSON := filepath.Join(repoDir, "package.json")
	packageContent := `{
  "name": "precommit-test",
  "version": "1.0.0",
  "description": "Test package for pre-commit",
  "main": "test.js",
  "scripts": {
    "test": "echo \"Error: no test specified\" && exit 1"
  },
  "keywords": [],
  "author": "",
  "license": "ISC"
}`
	if err := os.WriteFile(packageJSON, []byte(packageContent), 0o600); err != nil {
		t.Fatalf("Failed to create package.json: %v", err)
	}
}

// CreateGoFiles creates Go test files
func (fg *FileGenerator) CreateGoFiles(t *testing.T, repoDir string) {
	t.Helper()
	goFile := filepath.Join(repoDir, "test.go")
	content := `package main

import "fmt"

// HelloWorld prints a greeting message
func HelloWorld() {
	fmt.Println("Hello, World!")
}

func main() {
	HelloWorld()
}
`
	if err := os.WriteFile(goFile, []byte(content), 0o600); err != nil {
		t.Fatalf("Failed to create test Go file: %v", err)
	}

	// Create go.mod
	goMod := filepath.Join(repoDir, "go.mod")
	modContent := `module precommit-test

go 1.19
`
	if err := os.WriteFile(goMod, []byte(modContent), 0o600); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}
}

// CreateRubyFiles creates Ruby test files
func (fg *FileGenerator) CreateRubyFiles(t *testing.T, repoDir string) {
	t.Helper()
	rbFile := filepath.Join(repoDir, "test.rb")
	content := `#!/usr/bin/env ruby
# Test Ruby file for pre-commit testing

def hello_world
  puts 'Hello, World!'
end

hello_world if __FILE__ == $PROGRAM_NAME
`
	if err := os.WriteFile(rbFile, []byte(content), 0o600); err != nil {
		t.Fatalf("Failed to create test Ruby file: %v", err)
	}

	// Create Gemfile
	gemfile := filepath.Join(repoDir, "Gemfile")
	gemContent := `source 'https://rubygems.org'

ruby '>=2.7.0'

gem 'rake'
`
	if err := os.WriteFile(gemfile, []byte(gemContent), 0o600); err != nil {
		t.Fatalf("Failed to create Gemfile: %v", err)
	}
}

// CreateRustFiles creates Rust test files
func (fg *FileGenerator) CreateRustFiles(t *testing.T, repoDir string) {
	t.Helper()
	rsFile := filepath.Join(repoDir, "test.rs")
	content := `fn main() {
    println!("Hello, World!");
}

#[cfg(test)]
mod tests {
    #[test]
    fn test_hello_world() {
        // This test always passes
        assert_eq!(2 + 2, 4);
    }
}
`
	if err := os.WriteFile(rsFile, []byte(content), 0o600); err != nil {
		t.Fatalf("Failed to create test Rust file: %v", err)
	}

	// Create Cargo.toml
	cargoToml := filepath.Join(repoDir, "Cargo.toml")
	cargoContent := `[package]
name = "precommit-test"
version = "0.1.0"
edition = "2021"

[dependencies]
`
	if err := os.WriteFile(cargoToml, []byte(cargoContent), 0o600); err != nil {
		t.Fatalf("Failed to create Cargo.toml: %v", err)
	}
}

// CreateDartFiles creates Dart test files
func (fg *FileGenerator) CreateDartFiles(t *testing.T, repoDir string) {
	t.Helper()
	dartFile := filepath.Join(repoDir, "test.dart")
	content := `void main() {
  print('Hello, World!');
}
`
	if err := os.WriteFile(dartFile, []byte(content), 0o600); err != nil {
		t.Fatalf("Failed to create test Dart file: %v", err)
	}

	// Create pubspec.yaml
	pubspec := filepath.Join(repoDir, "pubspec.yaml")
	pubspecContent := `name: precommit_test
description: Test package for pre-commit
version: 1.0.0

environment:
  sdk: '>=2.19.0 <4.0.0'
`
	if err := os.WriteFile(pubspec, []byte(pubspecContent), 0o600); err != nil {
		t.Fatalf("Failed to create pubspec.yaml: %v", err)
	}
}

// CreateSwiftFiles creates Swift test files
func (fg *FileGenerator) CreateSwiftFiles(t *testing.T, repoDir string) {
	t.Helper()
	swiftFile := filepath.Join(repoDir, "test.swift")
	content := `import Foundation

func helloWorld() {
    print("Hello, World!")
}

helloWorld()
`
	if err := os.WriteFile(swiftFile, []byte(content), 0o600); err != nil {
		t.Fatalf("Failed to create test Swift file: %v", err)
	}
}

// CreateLuaFiles creates Lua test files
func (fg *FileGenerator) CreateLuaFiles(t *testing.T, repoDir string) {
	t.Helper()
	luaFile := filepath.Join(repoDir, "test.lua")
	content := `#!/usr/bin/env lua
-- Test Lua file for pre-commit testing

function hello_world()
    print("Hello, World!")
end

hello_world()
`
	if err := os.WriteFile(luaFile, []byte(content), 0o600); err != nil {
		t.Fatalf("Failed to create test Lua file: %v", err)
	}
}

// CreatePerlFiles creates Perl test files
func (fg *FileGenerator) CreatePerlFiles(t *testing.T, repoDir string) {
	t.Helper()
	plFile := filepath.Join(repoDir, "test.pl")
	content := `#!/usr/bin/env perl
# Test Perl file for pre-commit testing

use strict;
use warnings;

sub hello_world {
    print "Hello, World!\n";
}

hello_world();
`
	if err := os.WriteFile(plFile, []byte(content), 0o600); err != nil {
		t.Fatalf("Failed to create test Perl file: %v", err)
	}
}

// CreateRFiles creates R test files
func (fg *FileGenerator) CreateRFiles(t *testing.T, repoDir string) {
	t.Helper()
	rFile := filepath.Join(repoDir, "test.R")
	content := `#!/usr/bin/env Rscript
# Test R file for pre-commit testing

hello_world <- function() {
  cat("Hello, World!\n")
}

hello_world()
`
	if err := os.WriteFile(rFile, []byte(content), 0o600); err != nil {
		t.Fatalf("Failed to create test R file: %v", err)
	}
}

// CreateHaskellFiles creates Haskell test files
func (fg *FileGenerator) CreateHaskellFiles(t *testing.T, repoDir string) {
	t.Helper()
	hsFile := filepath.Join(repoDir, "test.hs")
	content := `-- Test Haskell file for pre-commit testing

main :: IO ()
main = putStrLn "Hello, World!"
`
	if err := os.WriteFile(hsFile, []byte(content), 0o600); err != nil {
		t.Fatalf("Failed to create test Haskell file: %v", err)
	}
}

// CreateDotNetFiles creates .NET test files
func (fg *FileGenerator) CreateDotNetFiles(t *testing.T, repoDir string) {
	t.Helper()
	csFile := filepath.Join(repoDir, "test.cs")
	content := `using System;

namespace PrecommitTest
{
    class Program
    {
        static void Main(string[] args)
        {
            Console.WriteLine("Hello, World!");
        }
    }
}
`
	if err := os.WriteFile(csFile, []byte(content), 0o600); err != nil {
		t.Fatalf("Failed to create test C# file: %v", err)
	}

	// Create project file
	csproj := filepath.Join(repoDir, "precommit-test.csproj")
	csprojContent := `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <OutputType>Exe</OutputType>
    <TargetFramework>net6.0</TargetFramework>
  </PropertyGroup>
</Project>
`
	if err := os.WriteFile(csproj, []byte(csprojContent), 0o600); err != nil {
		t.Fatalf("Failed to create project file: %v", err)
	}
}

// CreateScriptFiles creates script test files
func (fg *FileGenerator) CreateScriptFiles(t *testing.T, repoDir string) {
	t.Helper()
	scriptFile := filepath.Join(repoDir, "test-script.sh")
	content := `#!/bin/bash
# Test script for pre-commit testing

echo "Hello, World from script!"
exit 0
`
	//nolint:gosec // Script files need to be executable
	if err := os.WriteFile(scriptFile, []byte(content), 0o700); err != nil {
		t.Fatalf("Failed to create test script file: %v", err)
	}

	// Create a text file that the script can process
	txtFile := filepath.Join(repoDir, "test.txt")
	txtContent := "This is a test text file for script processing."
	if err := os.WriteFile(txtFile, []byte(txtContent), 0o600); err != nil {
		t.Fatalf("Failed to create test text file: %v", err)
	}
}

// CreateReadme creates a README file for all languages
func (fg *FileGenerator) CreateReadme(t *testing.T, repoDir string, test LanguageCompatibilityTest) {
	t.Helper()
	readmeFile := filepath.Join(repoDir, "README.md")
	readmeContent := fmt.Sprintf(`# Pre-commit Test Repository

This is a test repository for %s language compatibility testing.

## Purpose

This repository is used to test:
- Installation performance
- Cache behavior
- Functional equivalence
- Environment isolation
- Version management

## Language: %s

Repository: %s
Hook ID: %s
`, test.Language, test.Language, test.TestRepository, test.HookID)
	if err := os.WriteFile(readmeFile, []byte(readmeContent), 0o600); err != nil {
		t.Fatalf("Failed to create README.md: %v", err)
	}
}
