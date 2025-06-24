package languages

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/blairham/go-pre-commit/pkg/language"
	"github.com/blairham/go-pre-commit/pkg/repository/languages"
)

// CoursierLanguageTest implements LanguageTestRunner for Coursier (Scala/JVM)
type CoursierLanguageTest struct {
	*BaseLanguageTest
}

// NewCoursierLanguageTest creates a new Coursier language test
func NewCoursierLanguageTest(testDir string) *CoursierLanguageTest {
	return &CoursierLanguageTest{
		BaseLanguageTest: NewBaseLanguageTest(LangCoursier, testDir),
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
    additional_dependencies: ['org.scalameta:scalafmt-cli_2.13:3.7.12']
-   id: scalafix
    name: Scalafix
    description: Lint and refactor Scala code using scalafix
    entry: scalafix
    language: coursier
    files: \.scala$
    additional_dependencies: ['ch.epfl.scala:scalafix-cli_2.13:0.11.0']
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
