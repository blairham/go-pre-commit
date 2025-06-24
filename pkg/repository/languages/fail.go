package languages

// FailLanguage handles hooks that should always fail (used for testing and validation)
type FailLanguage struct {
	*GenericLanguage
}

// NewFailLanguage creates a new fail language handler
func NewFailLanguage() *FailLanguage {
	return &FailLanguage{
		GenericLanguage: NewGenericLanguage("fail", "", "", ""),
	}
}

// IsRuntimeAvailable always returns true for fail language
func (f *FailLanguage) IsRuntimeAvailable() bool {
	return true
}
