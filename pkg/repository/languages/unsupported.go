// Package languages provides language-specific implementations for pre-commit hook environments
package languages

import (
	"fmt"

	"github.com/blairham/go-pre-commit/pkg/language"
)

// UnsupportedLanguage represents a hook language that requires implementation
// but is not currently supported. This exists to provide clear error messages
// for hooks that specify language types needing special handling.
//
// This matches Python pre-commit's unsupported.py which provides:
// - ENVIRONMENT_DIR = None (no environment setup)
// - install_environment = lang_base.no_install
// - in_env = lang_base.no_env
// - run_hook = lang_base.basic_run_hook
type UnsupportedLanguage struct {
	*language.Base
	languageType string
}

// NewUnsupportedLanguage creates a new unsupported language handler
func NewUnsupportedLanguage() *UnsupportedLanguage {
	return &UnsupportedLanguage{
		Base:         language.NewBase("unsupported", "", "", ""),
		languageType: "unsupported",
	}
}

// GetDefaultVersion returns 'default' for unsupported language
func (u *UnsupportedLanguage) GetDefaultVersion() string {
	return language.VersionDefault
}

// IsRuntimeAvailable always returns false for unsupported language
// This language exists to provide helpful error messages
func (u *UnsupportedLanguage) IsRuntimeAvailable() bool {
	return false
}

// NeedsEnvironmentSetup returns false - no environment setup for unsupported
func (u *UnsupportedLanguage) NeedsEnvironmentSetup() bool {
	return false
}

// SetupEnvironmentWithRepo returns an error indicating this language is not supported
func (u *UnsupportedLanguage) SetupEnvironmentWithRepo(
	_, _, _, _ string,
	_ []string,
) (string, error) {
	return "", fmt.Errorf(
		"language '%s' is not supported by pre-commit. "+
			"Please check the hook configuration or use a different language",
		u.languageType,
	)
}

// SetupEnvironmentWithRepoInfo returns an error indicating this language is not supported
func (u *UnsupportedLanguage) SetupEnvironmentWithRepoInfo(
	_, _, _, _ string,
	_ []string,
) (string, error) {
	return "", fmt.Errorf(
		"language '%s' is not supported by pre-commit. "+
			"Please check the hook configuration or use a different language",
		u.languageType,
	)
}

// InstallDependencies returns an error - unsupported languages can't install deps
func (u *UnsupportedLanguage) InstallDependencies(_ string, _ []string) error {
	return fmt.Errorf("cannot install dependencies for unsupported language")
}

// CheckHealth returns an error - unsupported languages are never healthy
func (u *UnsupportedLanguage) CheckHealth(_, _ string) error {
	return fmt.Errorf("unsupported language is not available")
}

// GetEnvPatch returns an empty map - no environment patching for unsupported
func (u *UnsupportedLanguage) GetEnvPatch(_, _ string) map[string]string {
	return make(map[string]string)
}

// UnsupportedScriptLanguage represents a script-based language that is not
// currently supported. This is similar to UnsupportedLanguage but specifically
// for script-type languages.
//
// This matches Python pre-commit's unsupported_script.py
type UnsupportedScriptLanguage struct {
	*language.Base
	languageType string
}

// NewUnsupportedScriptLanguage creates a new unsupported script language handler
func NewUnsupportedScriptLanguage() *UnsupportedScriptLanguage {
	return &UnsupportedScriptLanguage{
		Base:         language.NewBase("unsupported_script", "", "", ""),
		languageType: "unsupported_script",
	}
}

// GetDefaultVersion returns 'default' for unsupported script language
func (u *UnsupportedScriptLanguage) GetDefaultVersion() string {
	return language.VersionDefault
}

// IsRuntimeAvailable always returns false for unsupported script language
func (u *UnsupportedScriptLanguage) IsRuntimeAvailable() bool {
	return false
}

// NeedsEnvironmentSetup returns false - no environment setup for unsupported script
func (u *UnsupportedScriptLanguage) NeedsEnvironmentSetup() bool {
	return false
}

// SetupEnvironmentWithRepo returns an error indicating this script language is not supported
func (u *UnsupportedScriptLanguage) SetupEnvironmentWithRepo(
	_, _, _, _ string,
	_ []string,
) (string, error) {
	return "", fmt.Errorf(
		"script language '%s' is not supported. "+
			"Please ensure the hook uses a supported script language or 'script' type",
		u.languageType,
	)
}

// SetupEnvironmentWithRepoInfo returns an error indicating this script language is not supported
func (u *UnsupportedScriptLanguage) SetupEnvironmentWithRepoInfo(
	_, _, _, _ string,
	_ []string,
) (string, error) {
	return "", fmt.Errorf(
		"script language '%s' is not supported. "+
			"Please ensure the hook uses a supported script language or 'script' type",
		u.languageType,
	)
}

// InstallDependencies returns an error - unsupported script languages can't install deps
func (u *UnsupportedScriptLanguage) InstallDependencies(_ string, _ []string) error {
	return fmt.Errorf("cannot install dependencies for unsupported script language")
}

// CheckHealth returns an error - unsupported script languages are never healthy
func (u *UnsupportedScriptLanguage) CheckHealth(_, _ string) error {
	return fmt.Errorf("unsupported script language is not available")
}

// GetEnvPatch returns an empty map - no environment patching for unsupported script
func (u *UnsupportedScriptLanguage) GetEnvPatch(_, _ string) map[string]string {
	return make(map[string]string)
}
