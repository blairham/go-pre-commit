package languages

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/blairham/go-pre-commit/pkg/language"
	"github.com/blairham/go-pre-commit/pkg/repository/languages"
)

// DotnetLanguageTest implements LanguageTestRunner for .NET
type DotnetLanguageTest struct {
	*BaseLanguageTest
}

// NewDotnetLanguageTest creates a new .NET language test
func NewDotnetLanguageTest(testDir string) *DotnetLanguageTest {
	return &DotnetLanguageTest{
		BaseLanguageTest: NewBaseLanguageTest(LangDotnet, testDir),
	}
}

// GetLanguageName returns the language name
func (dt *DotnetLanguageTest) GetLanguageName() string {
	return LangDotnet
}

// SetupRepositoryFiles creates .NET-specific repository files
func (dt *DotnetLanguageTest) SetupRepositoryFiles(repoPath string) error {
	// Create .pre-commit-hooks.yaml
	hooksFile := filepath.Join(repoPath, ".pre-commit-hooks.yaml")
	hooksContent := `-   id: dotnet-format
    name: .NET Format
    description: Format .NET code using dotnet format
    entry: dotnet format
    language: dotnet
    files: \.(cs|vb|fs)$
-   id: dotnet-test
    name: .NET Test
    description: Run .NET tests
    entry: dotnet test
    language: dotnet
    files: \.(cs|vb|fs)$
`
	if err := os.WriteFile(hooksFile, []byte(hooksContent), 0o600); err != nil {
		return fmt.Errorf("failed to create hooks file: %w", err)
	}

	// Create .csproj file
	csprojFile := filepath.Join(repoPath, "TestDotnetHooks.csproj")
	csprojContent := `<Project Sdk="Microsoft.NET.Sdk">

  <PropertyGroup>
    <TargetFramework>net6.0</TargetFramework>
    <OutputType>Exe</OutputType>
    <ImplicitUsings>enable</ImplicitUsings>
    <Nullable>enable</Nullable>
  </PropertyGroup>

</Project>
`
	if err := os.WriteFile(csprojFile, []byte(csprojContent), 0o600); err != nil {
		return fmt.Errorf("failed to create .csproj file: %w", err)
	}

	// Create Program.cs
	programFile := filepath.Join(repoPath, "Program.cs")
	programContent := `using System;

namespace TestDotnetHooks
{
    class Program
    {
        static void Main(string[] args)
        {
            Console.WriteLine("Hello, .NET!");
        }
    }
}
`
	if err := os.WriteFile(programFile, []byte(programContent), 0o600); err != nil {
		return fmt.Errorf("failed to create Program.cs: %w", err)
	}

	return nil
}

// GetLanguageManager returns the .NET language manager
func (dt *DotnetLanguageTest) GetLanguageManager() (language.Manager, error) {
	return languages.NewDotnetLanguage(), nil
}

// GetAdditionalValidations returns .NET-specific validation tests
func (dt *DotnetLanguageTest) GetAdditionalValidations() []ValidationStep {
	return []ValidationStep{
		{
			Name:        "dotnet-version-check",
			Description: ".NET version validation",
			Execute: func(_ *testing.T, _, _ string, lang language.Manager) error {
				if lang.GetName() != "dotnet" {
					return fmt.Errorf("expected dotnet language, got %s", lang.GetName())
				}
				return nil
			},
		},
	}
}
