// Package matching handles filtering and type matching for hooks
package matching

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/blairham/go-pre-commit/pkg/config"
)

// Matcher handles file filtering and type matching
type Matcher struct {
	typeMatchers map[string]TypeMatcher
}

// TypeMatcher is a function that determines if a file matches a type
type TypeMatcher func(ext, fileName, file string) bool

// NewMatcher creates a new file matcher
func NewMatcher() *Matcher {
	m := &Matcher{}
	m.typeMatchers = m.initializeTypeMatchers()
	return m
}

// GetFilesForHook returns files that match the given hook's criteria
func (m *Matcher) GetFilesForHook(
	hook config.Hook,
	contextFiles []string,
	_ /* allFiles */ bool,
) []string {
	// For now, we use contextFiles regardless of allFiles
	// Future enhancement: implement getting all files from git when allFiles is true
	baseFiles := contextFiles

	var matchingFiles []string
	for _, file := range baseFiles {
		if m.FileMatchesHook(file, hook) {
			matchingFiles = append(matchingFiles, file)
		}
	}

	return matchingFiles
}

// FileMatchesHook determines if a file matches a hook's filtering criteria
func (m *Matcher) FileMatchesHook(file string, hook config.Hook) bool {
	// Check include patterns
	if hook.Files != "" {
		if !m.matchesFilePattern(file, hook.Files) {
			return false
		}
	}

	// Check exclude patterns
	if hook.ExcludeRegex != "" {
		if m.matchesExcludePattern(file, hook.ExcludeRegex) {
			return false
		}
	}

	// Check type filters
	if len(hook.Types) > 0 || len(hook.ExcludeTypes) > 0 || len(hook.TypesOr) > 0 {
		if !m.matchesTypeFilters(file, hook) {
			return false
		}
	}

	return true
}

// matchesFilePattern checks if a file matches a given pattern
func (m *Matcher) matchesFilePattern(file, pattern string) bool {
	if pattern == "" {
		return true
	}

	// Try matching against both full path and basename
	if matched, err := regexp.MatchString(pattern, file); err == nil && matched {
		return true
	}

	basename := filepath.Base(file)
	if matched, err := regexp.MatchString(pattern, basename); err == nil && matched {
		return true
	}

	return false
}

// matchesExcludePattern checks if a file matches an exclude regex pattern
func (m *Matcher) matchesExcludePattern(file, excludeRegex string) bool {
	if excludeRegex == "" {
		return false
	}

	matched, err := regexp.MatchString(excludeRegex, file)
	return err == nil && matched
}

// matchesTypeFilters checks if a file matches the type inclusion/exclusion criteria
func (m *Matcher) matchesTypeFilters(file string, hook config.Hook) bool {
	// Check types (all must match - AND logic)
	if len(hook.Types) > 0 && !m.matchesAllTypes(file, hook.Types) {
		return false
	}

	// Check types_or (any must match - OR logic)
	if len(hook.TypesOr) > 0 && !m.matchesTypes(file, hook.TypesOr) {
		return false
	}

	// Check exclude types
	if len(hook.ExcludeTypes) > 0 && m.matchesTypes(file, hook.ExcludeTypes) {
		return false
	}

	return true
}

// matchesTypes checks if a file matches any of the given types
func (m *Matcher) matchesTypes(file string, types []string) bool {
	ext := strings.ToLower(filepath.Ext(file))
	fileName := filepath.Base(file)

	for _, fileType := range types {
		if matcher, exists := m.typeMatchers[fileType]; exists {
			if matcher(ext, fileName, file) {
				return true
			}
		}
	}

	return false
}

// matchesAllTypes checks if a file matches all of the given types
func (m *Matcher) matchesAllTypes(file string, types []string) bool {
	ext := strings.ToLower(filepath.Ext(file))
	fileName := filepath.Base(file)

	for _, fileType := range types {
		if matcher, exists := m.typeMatchers[fileType]; exists {
			if !matcher(ext, fileName, file) {
				return false
			}
		} else {
			// Unknown type means no match
			return false
		}
	}

	return true
}
