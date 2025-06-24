package languages

import (
	"testing"
)

func TestScriptLanguage(t *testing.T) {
	script := NewScriptLanguage()

	// Use shared helper for comprehensive simple language testing
	testSimpleLanguageInterface(t, script, "script")

	// Additional script-specific tests
	t.Run("NewScriptLanguage_TypeChecks", func(t *testing.T) {
		if script == nil {
			t.Error("NewScriptLanguage() returned nil")
			return
		}
		if script.GenericLanguage == nil {
			t.Error("NewScriptLanguage() returned instance with nil GenericLanguage")
		}
		if script.Base == nil {
			t.Error("NewScriptLanguage() returned instance with nil Base")
		}
	})
}
