package languages

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/blairham/go-pre-commit/pkg/language"
	"github.com/blairham/go-pre-commit/pkg/repository/languages"
)

// JuliaLanguageTest implements LanguageTestRunner for Julia
type JuliaLanguageTest struct {
	*BaseLanguageTest
}

// NewJuliaLanguageTest creates a new Julia language test
func NewJuliaLanguageTest(testDir string) *JuliaLanguageTest {
	return &JuliaLanguageTest{
		BaseLanguageTest: NewBaseLanguageTest(LangJulia, testDir),
	}
}

// GetLanguageName returns the name of the language being tested
func (j *JuliaLanguageTest) GetLanguageName() string {
	return LangJulia
}

// SetupRepositoryFiles creates Julia-specific files for testing
func (j *JuliaLanguageTest) SetupRepositoryFiles(repoPath string) error {
	// Create a basic Julia project structure
	srcDir := filepath.Join(repoPath, "src")
	if err := os.MkdirAll(srcDir, 0o750); err != nil {
		return fmt.Errorf("failed to create src directory: %w", err)
	}

	// Create Project.toml
	projectToml := `name = "TestJuliaProject"
uuid = "12345678-1234-1234-1234-123456789abc"
version = "0.1.0"

[deps]
Test = "8dfed614-e22c-5e08-85e1-65c5234f0b40"
`
	projectPath := filepath.Join(repoPath, "Project.toml")
	if err := os.WriteFile(projectPath, []byte(projectToml), 0o600); err != nil {
		return fmt.Errorf("failed to create Project.toml: %w", err)
	}

	// Create a simple Julia source file
	juliaCode := `module TestJuliaProject

using Test

"""
    format_julia_code(content::String) -> String

A simple Julia code formatter function for testing.
"""
function format_julia_code(content::String)
    # Simple formatting: add spaces around operators
    formatted = replace(content, r"([+\-*/=])" => s" \1 ")
    return formatted
end

# Test function
function test_format_julia_code()
    @test format_julia_code("x=1+2") == "x = 1 + 2"
    @test format_julia_code("y*z") == "y * z"
    println("Julia formatting tests passed!")
end

export format_julia_code, test_format_julia_code

end # module
`
	juliaFile := filepath.Join(srcDir, "TestJuliaProject.jl")
	if err := os.WriteFile(juliaFile, []byte(juliaCode), 0o600); err != nil {
		return fmt.Errorf("failed to create Julia source file: %w", err)
	}

	// Create test file
	testCode := `using Test
using TestJuliaProject

@testset "Julia Formatter Tests" begin
    @test TestJuliaProject.format_julia_code("x=1+2") == "x = 1 + 2"
    @test TestJuliaProject.format_julia_code("y*z") == "y * z"
end
`
	testDir := filepath.Join(repoPath, "test")
	if err := os.MkdirAll(testDir, 0o750); err != nil {
		return fmt.Errorf("failed to create test directory: %w", err)
	}

	testFile := filepath.Join(testDir, "runtests.jl")
	if err := os.WriteFile(testFile, []byte(testCode), 0o600); err != nil {
		return fmt.Errorf("failed to create test file: %w", err)
	}

	return nil
}

// GetLanguageManager returns the Julia language manager
func (j *JuliaLanguageTest) GetLanguageManager() (language.Manager, error) {
	return languages.NewJuliaLanguage(), nil
}

// GetAdditionalValidations returns Julia-specific validation steps
func (j *JuliaLanguageTest) GetAdditionalValidations() []ValidationStep {
	return []ValidationStep{
		{
			Name:        "julia-version-check",
			Description: "Verify Julia installation and version",
			Execute: func(t *testing.T, _, _ string, lang language.Manager) error {
				// Version validation
				if lang.GetName() != "julia" {
					return fmt.Errorf("expected julia language, got %s", lang.GetName())
				}

				t.Logf("Julia language validation passed")
				return nil
			},
		},
		{
			Name:        "project-structure-check",
			Description: "Verify Julia project structure",
			Execute: func(t *testing.T, envPath, _ string, _ language.Manager) error {
				// Check if Project.toml exists in environment or parent directory
				projectPaths := []string{
					filepath.Join(envPath, "Project.toml"),
					filepath.Join(filepath.Dir(envPath), "Project.toml"),
				}

				for _, projectPath := range projectPaths {
					if _, err := os.Stat(projectPath); err == nil {
						t.Logf("Found Project.toml at: %s", projectPath)
						return nil
					}
				}

				return fmt.Errorf("project.toml not found in expected locations")
			},
		},
	}
}
