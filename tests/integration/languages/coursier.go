package languages

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/blairham/go-pre-commit/pkg/language"
	"github.com/blairham/go-pre-commit/pkg/repository/languages"
)

// CoursierLanguageTest implements LanguageTestRunner and BidirectionalTestRunner for Coursier (Scala/JVM)
type CoursierLanguageTest struct {
	*BaseLanguageTest
	*BaseBidirectionalTest
}

// NewCoursierLanguageTest creates a new Coursier language test
func NewCoursierLanguageTest(testDir string) *CoursierLanguageTest {
	return &CoursierLanguageTest{
		BaseLanguageTest:      NewBaseLanguageTest(LangCoursier, testDir),
		BaseBidirectionalTest: NewBaseBidirectionalTest(LangCoursier),
	}
}

// GetLanguageName returns the language name
func (ct *CoursierLanguageTest) GetLanguageName() string {
	return LangCoursier
}

// SetupRepositoryFiles creates Coursier-specific repository files
func (ct *CoursierLanguageTest) SetupRepositoryFiles(repoPath string) error {
	// Create .pre-commit-hooks.yaml
	hooksFile := filepath.Join(repoPath, ".pre-commit-hooks.yaml")
	hooksContent := `-   id: scalafmt
    name: Scalafmt
    description: Format Scala code using scalafmt
    entry: scalafmt
    language: coursier
    files: \.scala$
    additional_dependencies: ['scalafmt']
-   id: scalafix
    name: Scalafix
    description: Lint and refactor Scala code using scalafix
    entry: scalafix
    language: coursier
    files: \.scala$
    additional_dependencies: ['scalafix']
`
	if err := os.WriteFile(hooksFile, []byte(hooksContent), 0o600); err != nil {
		return fmt.Errorf("failed to create hooks file: %w", err)
	}

	// Create build.sbt
	buildFile := filepath.Join(repoPath, "build.sbt")
	buildContent := `name := "test-coursier-hooks"

version := "0.1.0"

scalaVersion := "2.13.10"

libraryDependencies ++= Seq(
  "org.scalatest" %% "scalatest" % "3.2.15" % Test
)
`
	if err := os.WriteFile(buildFile, []byte(buildContent), 0o600); err != nil {
		return fmt.Errorf("failed to create build.sbt: %w", err)
	}

	// Create src/main/scala directory and Main.scala
	srcDir := filepath.Join(repoPath, "src", "main", "scala")
	if err := os.MkdirAll(srcDir, 0o750); err != nil {
		return fmt.Errorf("failed to create src directory: %w", err)
	}

	mainFile := filepath.Join(srcDir, "Main.scala")
	mainContent := `object Main {
  def main(args: Array[String]): Unit = {
    println("Hello, Scala!")
  }
}
`
	if err := os.WriteFile(mainFile, []byte(mainContent), 0o600); err != nil {
		return fmt.Errorf("failed to create Main.scala: %w", err)
	}

	return nil
}

// GetLanguageManager returns the Coursier language manager
func (ct *CoursierLanguageTest) GetLanguageManager() (language.Manager, error) {
	return languages.NewCoursierLanguage(), nil
}

// GetAdditionalValidations returns Coursier-specific validation tests
func (ct *CoursierLanguageTest) GetAdditionalValidations() []ValidationStep {
	return []ValidationStep{
		{
			Name:        "coursier-version-check",
			Description: "Coursier version validation",
			Execute: func(_ *testing.T, _, _ string, lang language.Manager) error {
				if lang.GetName() != "coursier" {
					return fmt.Errorf("expected coursier language, got %s", lang.GetName())
				}
				return nil
			},
		},
	}
}

// GetPreCommitConfig returns the .pre-commit-config.yaml content for Coursier testing
func (ct *CoursierLanguageTest) GetPreCommitConfig() string {
	return `repos:
  - repo: local
    hooks:
      - id: test-coursier
        name: Test Coursier Hook
        entry: echo "Testing Coursier"
        language: coursier
        files: \.scala$
        additional_dependencies: ['scalafmt']
`
}

// GetTestFiles returns test files needed for Coursier testing
func (ct *CoursierLanguageTest) GetTestFiles() map[string]string {
	return map[string]string{
		"test.scala": `object TestApp extends App {
  println("Hello from Coursier!")
}
`,
	}
}

// GetExpectedDirectories returns the directories expected in Coursier environments
func (ct *CoursierLanguageTest) GetExpectedDirectories() []string {
	return []string{
		"bin",   // Coursier executables
		"lib",   // JAR libraries
		"cache", // Coursier cache
	}
}

// GetExpectedStateFiles returns state files expected in Coursier environments
func (ct *CoursierLanguageTest) GetExpectedStateFiles() []string {
	return []string{
		"build.sbt",     // SBT build file
		"project",       // SBT project directory
		"coursier.json", // Coursier configuration
	}
}

// TestBidirectionalCacheCompatibility tests cache compatibility between Go and Python implementations
func (ct *CoursierLanguageTest) TestBidirectionalCacheCompatibility(
	t *testing.T,
	pythonBinary, goBinary, tempDir string,
) error {
	t.Helper()
	t.Logf("🔄 Testing Coursier bidirectional cache compatibility")
	t.Logf("   📋 Coursier environments manage JVM dependencies - testing cache compatibility")

	// Use the base bidirectional test framework
	if err := ct.BaseBidirectionalTest.RunBidirectionalCacheTest(t, ct, pythonBinary, goBinary, tempDir); err != nil {
		return fmt.Errorf("coursier bidirectional cache test failed: %w", err)
	}

	t.Logf("✅ Coursier bidirectional cache compatibility test completed")
	return nil
}
