package languages

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

// IsRuntimeAvailable always returns true for script language (uses shell commands)
func (s *ScriptLanguage) IsRuntimeAvailable() bool {
	return true
}
