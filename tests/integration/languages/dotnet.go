package languages

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/blairham/go-pre-commit/pkg/language"
	"github.com/blairham/go-pre-commit/pkg/repository/languages"
)

// DotnetLanguageTest implements LanguageTestRunner and BidirectionalTestRunner for .NET
type DotnetLanguageTest struct {
	*BaseLanguageTest
	*BaseBidirectionalTest
}

// NewDotnetLanguageTest creates a new .NET language test
func NewDotnetLanguageTest(testDir string) *DotnetLanguageTest {
	return &DotnetLanguageTest{
		BaseLanguageTest:      NewBaseLanguageTest(LangDotnet, testDir),
		BaseBidirectionalTest: NewBaseBidirectionalTest(LangDotnet),
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

// GetPreCommitConfig returns the .pre-commit-config.yaml content for .NET testing
func (dt *DotnetLanguageTest) GetPreCommitConfig() string {
	return `repos:
  - repo: local
    hooks:
      - id: test-dotnet
        name: Test .NET Hook
        entry: echo "Testing .NET"
        language: dotnet
        files: \.cs$
`
}

// GetTestFiles returns test files needed for .NET testing
func (dt *DotnetLanguageTest) GetTestFiles() map[string]string {
	return map[string]string{
		"Program.cs": `using System;

namespace TestApp
{
    class Program
    {
        static void Main(string[] args)
        {
            Console.WriteLine("Hello from .NET!");
        }
    }
}
`,
	}
}

// GetExpectedDirectories returns the directories expected in .NET environments
func (dt *DotnetLanguageTest) GetExpectedDirectories() []string {
	return []string{
		"bin",      // .NET build output
		"obj",      // .NET object files
		"packages", // NuGet packages
	}
}

// GetExpectedStateFiles returns state files expected in .NET environments
func (dt *DotnetLanguageTest) GetExpectedStateFiles() []string {
	return []string{
		"TestApp.csproj",  // C# project file
		"TestApp.sln",     // Visual Studio solution file
		"packages.config", // NuGet packages configuration
	}
}

// TestBidirectionalCacheCompatibility tests cache compatibility between Go and Python implementations
func (dt *DotnetLanguageTest) TestBidirectionalCacheCompatibility(
	t *testing.T,
	pythonBinary, goBinary, tempDir string,
) error {
	t.Helper()
	t.Logf("🔄 Testing .NET bidirectional cache compatibility")
	t.Logf("   📋 .NET environments manage NuGet packages and builds - testing cache compatibility")

	// Use the base bidirectional test framework
	if err := dt.BaseBidirectionalTest.RunBidirectionalCacheTest(t, dt, pythonBinary, goBinary, tempDir); err != nil {
		return fmt.Errorf(".NET bidirectional cache test failed: %w", err)
	}

	t.Logf("✅ .NET bidirectional cache compatibility test completed")
	return nil
}
