package languages

const (
	// scriptSystemVersion represents the system version for script language
	scriptSystemVersion = "system"
)

// ScriptLanguage handles script execution without specific runtime requirements
type ScriptLanguage struct {
	*GenericLanguage
}

// NewScriptLanguage creates a new script language handler
func NewScriptLanguage() *ScriptLanguage {
	return &ScriptLanguage{
		GenericLanguage: NewGenericLanguage("script", "", "", ""),
	}
}

// GetDefaultVersion returns the default script version (always 'system')
func (s *ScriptLanguage) GetDefaultVersion() string {
	return scriptSystemVersion
}

// IsRuntimeAvailable always returns true for script language (uses shell commands)
func (s *ScriptLanguage) IsRuntimeAvailable() bool {
	return true
}
