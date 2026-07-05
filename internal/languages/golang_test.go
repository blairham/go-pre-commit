package languages

import (
	"slices"
	"testing"
)

// ---------------------------------------------------------------------------
// Golang — install env composition
// ---------------------------------------------------------------------------

func TestGoInstallEnvDefaultsToLocalToolchain(t *testing.T) {
	t.Setenv("GOTOOLCHAIN", "")

	env := goInstallEnv("/prefix/go_env-default")
	if !slices.Contains(env, "GOTOOLCHAIN=local") {
		t.Errorf("env %v should pin GOTOOLCHAIN=local when the caller sets nothing", env)
	}
}

func TestGoInstallEnvRespectsCallerToolchain(t *testing.T) {
	// CI pins a repo-matching toolchain so hooks whose module requires a
	// newer Go than PATH's can still build; the install must not override it.
	t.Setenv("GOTOOLCHAIN", "go1.26.4")

	env := goInstallEnv("/prefix/go_env-default")
	if slices.Contains(env, "GOTOOLCHAIN=local") {
		t.Errorf("env %v must not force GOTOOLCHAIN=local over the caller's pin", env)
	}
}
