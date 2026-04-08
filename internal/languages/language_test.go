package languages

import (
	"context"
	"testing"
)

// ---------------------------------------------------------------------------
// Registry – Get / Register behavior
// ---------------------------------------------------------------------------

// TestAllLanguagesRegistered verifies that every language that the upstream
// Python pre-commit tool supports is registered in the Go implementation.
// This mirrors the LANGUAGES dict in pre_commit/languages/__init__.py and the
// exhaustive mapping in pre_commit/repository.py.
func TestAllLanguagesRegistered(t *testing.T) {
	langs := []string{
		"python",
		"node",
		"golang",
		"ruby",
		"rust",
		"docker",
		"docker_image",
		"fail",
		"pygrep",
		"conda",
		"coursier",
		"dart",
		"dotnet",
		"haskell",
		"julia",
		"lua",
		"perl",
		"r",
		"swift",
	}
	for _, name := range langs {
		if _, err := Get(name); err != nil {
			t.Errorf("Get(%q) returned error: %v", name, err)
		}
	}
}

// TestGetUnknownLanguageReturnsError verifies that requesting an unregistered
// language name returns an error.
func TestGetUnknownLanguageReturnsError(t *testing.T) {
	if _, err := Get("__totally_made_up_language__"); err == nil {
		t.Error("Get(unknown) = nil, want error")
	}
}

// TestGetCaseInsensitive verifies that language names are normalised to
// lowercase before lookup, matching the upstream Python behavior which
// also lowercases the language string.
func TestGetCaseInsensitive(t *testing.T) {
	cases := []string{"Python", "PYTHON", "python", "PyThOn"}
	for _, name := range cases {
		lang, err := Get(name)
		if err != nil {
			t.Errorf("Get(%q) = error %v, want python", name, err)
			continue
		}
		if lang.Name() != "python" {
			t.Errorf("Get(%q).Name() = %q, want %q", name, lang.Name(), "python")
		}
	}
}

// ---------------------------------------------------------------------------
// Aliases – the upstream Python implementation maps several aliases so that
// old config files keep working.
// ---------------------------------------------------------------------------

// TestAliasPythonVenv verifies that "python_venv" resolves to the Python
// language, matching the upstream alias in pre_commit/languages/__init__.py.
func TestAliasPythonVenv(t *testing.T) {
	lang, err := Get("python_venv")
	if err != nil {
		t.Fatalf("Get(%q): %v", "python_venv", err)
	}
	if lang.Name() != "python" {
		t.Errorf("alias python_venv → Name() = %q, want %q", lang.Name(), "python")
	}
}

// TestAliasSystem verifies that "system" resolves without error and returns
// the unsupported language handler. The upstream pre-commit treats "system" as
// an alias so that old configs keep working.
func TestAliasSystem(t *testing.T) {
	lang, err := Get("system")
	if err != nil {
		t.Fatalf("Get(%q): %v", "system", err)
	}
	// The alias maps to "unsupported" internally.
	if lang.Name() != "unsupported" && lang.Name() != "system" {
		t.Errorf("alias system → unexpected Name() = %q", lang.Name())
	}
}

// TestAliasScript verifies that "script" resolves without error.
func TestAliasScript(t *testing.T) {
	lang, err := Get("script")
	if err != nil {
		t.Fatalf("Get(%q): %v", "script", err)
	}
	// The alias maps to "unsupported_script" internally.
	if lang.Name() != "unsupported_script" && lang.Name() != "script" {
		t.Errorf("alias script → unexpected Name() = %q", lang.Name())
	}
}

// ---------------------------------------------------------------------------
// Language interface contract – every registered language must satisfy the
// interface and return non-empty values for Name and EnvironmentDir (or empty
// when no env is needed, which is also acceptable).
// ---------------------------------------------------------------------------

func TestAllLanguagesSatisfyInterface(t *testing.T) {
	names := []string{
		"python", "node", "golang", "ruby", "rust",
		"docker", "docker_image", "fail", "pygrep",
		"conda", "coursier", "dart", "dotnet", "haskell",
		"julia", "lua", "perl", "r", "swift",
		// Note: "system" and "script" are aliases; test them via Get() with alias names.
		"unsupported", "unsupported_script",
	}
	for _, name := range names {
		lang, err := Get(name)
		if err != nil {
			t.Errorf("Get(%q): %v", name, err)
			continue
		}
		if lang.Name() == "" {
			t.Errorf("language %q: Name() is empty", name)
		}
		// GetDefaultVersion must return a non-empty string.
		if lang.GetDefaultVersion() == "" {
			t.Errorf("language %q: GetDefaultVersion() is empty", name)
		}
	}
}

// TestRegisterCustomLanguage verifies that the Register function works
// correctly for a new language, and Get retrieves it by name.
func TestRegisterCustomLanguage(t *testing.T) {
	Register("testlang", &testLanguage{name: "testlang"})
	lang, err := Get("testlang")
	if err != nil {
		t.Fatalf("Get(testlang): %v", err)
	}
	if lang.Name() != "testlang" {
		t.Errorf("Name() = %q, want %q", lang.Name(), "testlang")
	}
}

// TestPythonEnvironmentDirValue verifies the key constant ENVIRONMENT_DIR
// matches the upstream Python value "py_env".
func TestPythonEnvironmentDirValue(t *testing.T) {
	lang, err := Get("python")
	if err != nil {
		t.Fatal(err)
	}
	if lang.EnvironmentDir() != "py_env" {
		t.Errorf("python EnvironmentDir() = %q, want %q", lang.EnvironmentDir(), "py_env")
	}
}

// testLanguage is a minimal Language implementation used only in tests.
type testLanguage struct {
	name string
}

func (tl *testLanguage) Name() string                                     { return tl.name }
func (tl *testLanguage) EnvironmentDir() string                           { return "" }
func (tl *testLanguage) GetDefaultVersion() string                        { return "default" }
func (tl *testLanguage) HealthCheck(_, _ string) error                    { return nil }
func (tl *testLanguage) InstallEnvironment(_, _ string, _ []string) error { return nil }
func (tl *testLanguage) Run(_ context.Context, _, _, _ string, _, _ []string, _ string) (int, []byte, error) {
	return 0, nil, nil
}
