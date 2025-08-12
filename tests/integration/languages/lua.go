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
	*BaseBidirectionalTest
}

// NewLuaLanguageTest creates a new Lua language test
func NewLuaLanguageTest(testDir string) *LuaLanguageTest {
	return &LuaLanguageTest{
		BaseLanguageTest:      NewBaseLanguageTest(LangLua, testDir),
		BaseBidirectionalTest: NewBaseBidirectionalTest(testDir),
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

// GetPreCommitConfig returns the .pre-commit-config.yaml content for Lua testing
func (lt *LuaLanguageTest) GetPreCommitConfig() string {
	return `repos:
  - repo: local
    hooks:
      - id: test-lua
        name: Test Lua Hook
        entry: echo "Testing Lua"
        language: lua
        files: \.lua$
`
}

// GetTestFiles returns test files needed for Lua testing
func (lt *LuaLanguageTest) GetTestFiles() map[string]string {
	return map[string]string{
		"main.lua": `print("Hello from Lua!")

function greet(name)
    print("Hello, " .. name .. "!")
end

greet("World")
`,
		"test.lua": `-- Test file for Lua
require("main")

print("Test completed")
`,
	}
}

// GetExpectedDirectories returns the directories expected in Lua environments
func (lt *LuaLanguageTest) GetExpectedDirectories() []string {
	return []string{
		"lua_modules", // Lua modules directory
		"lib",         // Lua libraries
		"src",         // Lua source
	}
}

// GetExpectedStateFiles returns state files expected in Lua environments
func (lt *LuaLanguageTest) GetExpectedStateFiles() []string {
	return []string{
		"rockspec",     // Lua rock specification
		".luarocks",    // LuaRocks configuration
		"luarocks.cfg", // LuaRocks config file
	}
}

// TestBidirectionalCacheCompatibility tests cache compatibility between Go and Python implementations
func (lt *LuaLanguageTest) TestBidirectionalCacheCompatibility(
	t *testing.T,
	pythonBinary, goBinary, tempDir string,
) error {
	t.Helper()
	t.Logf("🔄 Testing Lua bidirectional cache compatibility")
	t.Logf("   📋 Lua environments manage modules and rocks - testing cache compatibility")

	// Use the base bidirectional test framework
	if err := lt.BaseBidirectionalTest.RunBidirectionalCacheTest(t, lt, pythonBinary, goBinary, tempDir); err != nil {
		return fmt.Errorf("lua bidirectional cache test failed: %w", err)
	}

	t.Logf("✅ Lua bidirectional cache compatibility test completed")
	return nil
}
