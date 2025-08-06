package languages

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/blairham/go-pre-commit/pkg/language"
	"github.com/blairham/go-pre-commit/pkg/repository/languages"
)

// LuaLanguageTest implements LanguageTestRunner for Lua
type LuaLanguageTest struct {
	*BaseLanguageTest
}

// NewLuaLanguageTest creates a new Lua language test
func NewLuaLanguageTest(testDir string) *LuaLanguageTest {
	return &LuaLanguageTest{
		BaseLanguageTest: NewBaseLanguageTest(LangLua, testDir),
	}
}

// GetLanguageName returns the language name
func (lt *LuaLanguageTest) GetLanguageName() string {
	return LangLua
}

// SetupRepositoryFiles creates Lua-specific repository files
func (lt *LuaLanguageTest) SetupRepositoryFiles(repoPath string) error {
	// Create .pre-commit-hooks.yaml
	hooksFile := filepath.Join(repoPath, ".pre-commit-hooks.yaml")
	hooksContent := `-   id: lua-syntax-check
    name: Lua Syntax Check
    description: Check Lua syntax
    entry: luac
    language: lua
    files: \.lua$
    args: ['-p']
`
	if err := os.WriteFile(hooksFile, []byte(hooksContent), 0o600); err != nil {
		return fmt.Errorf("failed to create hooks file: %w", err)
	}

	// Create a simple Lua script
	luaFile := filepath.Join(repoPath, "test.lua")
	luaContent := `print("Hello, Lua!")
`
	if err := os.WriteFile(luaFile, []byte(luaContent), 0o600); err != nil {
		return fmt.Errorf("failed to create test.lua: %w", err)
	}

	return nil
}

// GetLanguageManager returns the Lua language manager
func (lt *LuaLanguageTest) GetLanguageManager() (language.Manager, error) {
	return languages.NewLuaLanguage(), nil
}

// GetAdditionalValidations returns Lua-specific validation tests
func (lt *LuaLanguageTest) GetAdditionalValidations() []ValidationStep {
	return []ValidationStep{
		{
			Name:        "lua-version-check",
			Description: "Lua version validation",
			Execute: func(_ *testing.T, _, _ string, lang language.Manager) error {
				if lang.GetName() != "lua" {
					return fmt.Errorf("expected lua language, got %s", lang.GetName())
				}
				return nil
			},
		},
	}
}
