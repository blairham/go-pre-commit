package languages

import (
	"context"
	"fmt"
	"testing"
)

func TestSimpleLanguage_Name(t *testing.T) {
	lang := &SimpleLanguage{LangName: "test"}
	if lang.Name() != "test" {
		t.Errorf("expected 'test', got %q", lang.Name())
	}
}

func TestSimpleLanguage_EnvironmentDir(t *testing.T) {
	lang := &SimpleLanguage{EnvDirName: "test_env"}
	if lang.EnvironmentDir() != "test_env" {
		t.Errorf("expected 'test_env', got %q", lang.EnvironmentDir())
	}
}

func TestSimpleLanguage_GetDefaultVersion(t *testing.T) {
	lang := &SimpleLanguage{}
	if lang.GetDefaultVersion() != "default" {
		t.Errorf("expected 'default', got %q", lang.GetDefaultVersion())
	}

	lang2 := &SimpleLanguage{DefaultVersion: "3.11"}
	if lang2.GetDefaultVersion() != "3.11" {
		t.Errorf("expected '3.11', got %q", lang2.GetDefaultVersion())
	}
}

func TestSimpleLanguage_HealthCheck_Cmd(t *testing.T) {
	lang := &SimpleLanguage{
		HealthCmd: []string{"true"},
	}
	if err := lang.HealthCheck("", ""); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestSimpleLanguage_HealthCheck_CmdFails(t *testing.T) {
	lang := &SimpleLanguage{
		HealthCmd: []string{"false"},
	}
	if err := lang.HealthCheck("", ""); err == nil {
		t.Error("expected error for failing command")
	}
}

func TestSimpleLanguage_HealthCheck_NoCmd(t *testing.T) {
	lang := &SimpleLanguage{}
	if err := lang.HealthCheck("", ""); err != nil {
		t.Errorf("expected no error with no health cmd, got %v", err)
	}
}

func TestSimpleLanguage_HealthCheck_FnOverride(t *testing.T) {
	called := false
	lang := &SimpleLanguage{
		HealthCmd: []string{"false"}, // would fail
		HealthCheckFn: func(prefix, version string) error {
			called = true
			return nil
		},
	}
	if err := lang.HealthCheck("", ""); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected HealthCheckFn to be called")
	}
}

func TestSimpleLanguage_HealthCheck_FnError(t *testing.T) {
	lang := &SimpleLanguage{
		HealthCheckFn: func(prefix, version string) error {
			return fmt.Errorf("custom error")
		},
	}
	err := lang.HealthCheck("", "")
	if err == nil || err.Error() != "custom error" {
		t.Errorf("expected 'custom error', got %v", err)
	}
}

func TestSimpleLanguage_Install_Fn(t *testing.T) {
	called := false
	lang := &SimpleLanguage{
		EnvDirName: "test_env",
		InstallFn: func(prefix, version, envDirName string, deps []string) error {
			called = true
			if envDirName != "test_env" {
				t.Errorf("expected envDirName 'test_env', got %q", envDirName)
			}
			return nil
		},
	}
	if err := lang.InstallEnvironment("/prefix", "default", nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected InstallFn to be called")
	}
}

func TestSimpleLanguage_Install_Cmd(t *testing.T) {
	prefix := t.TempDir()
	lang := &SimpleLanguage{
		EnvDirName: "test_env",
		InstallCmd: func(envDir, pfx string) (string, []string) {
			return "true", nil
		},
	}
	if err := lang.InstallEnvironment(prefix, "default", nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSimpleLanguage_Install_CmdFails(t *testing.T) {
	prefix := t.TempDir()
	lang := &SimpleLanguage{
		EnvDirName: "test_env",
		InstallCmd: func(envDir, pfx string) (string, []string) {
			return "false", nil
		},
	}
	if err := lang.InstallEnvironment(prefix, "default", nil); err == nil {
		t.Error("expected error for failing install command")
	}
}

func TestSimpleLanguage_Install_DepsFn(t *testing.T) {
	prefix := t.TempDir()
	depsCalled := false
	lang := &SimpleLanguage{
		EnvDirName: "test_env",
		InstallCmd: func(envDir, pfx string) (string, []string) {
			return "true", nil
		},
		InstallDepsFn: func(envDir, pfx string, deps []string) error {
			depsCalled = true
			if len(deps) != 2 {
				t.Errorf("expected 2 deps, got %d", len(deps))
			}
			return nil
		},
	}
	if err := lang.InstallEnvironment(prefix, "default", []string{"dep1", "dep2"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !depsCalled {
		t.Error("expected InstallDepsFn to be called")
	}
}

func TestSimpleLanguage_Install_DepsFnNotCalledWhenNoDeps(t *testing.T) {
	prefix := t.TempDir()
	depsCalled := false
	lang := &SimpleLanguage{
		EnvDirName: "test_env",
		InstallCmd: func(envDir, pfx string) (string, []string) {
			return "true", nil
		},
		InstallDepsFn: func(envDir, pfx string, deps []string) error {
			depsCalled = true
			return nil
		},
	}
	if err := lang.InstallEnvironment(prefix, "default", nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if depsCalled {
		t.Error("expected InstallDepsFn NOT to be called with no deps")
	}
}

func TestSimpleLanguage_Run_Default(t *testing.T) {
	lang := &SimpleLanguage{
		EnvDirName: "test_env",
	}
	// Run will call RunHookCommand which needs a real binary.
	// Just test that it doesn't panic and returns something.
	code, _, err := lang.Run(context.Background(), "/prefix", t.TempDir(), "true", nil, nil, "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
}

func TestSimpleLanguage_Run_FnOverride(t *testing.T) {
	called := false
	lang := &SimpleLanguage{
		EnvDirName: "test_env",
		RunFn: func(ctx context.Context, prefix, workDir, entry string, args, fileArgs []string, version, envDirName string) (int, []byte, error) {
			called = true
			if envDirName != "test_env" {
				t.Errorf("expected envDirName 'test_env', got %q", envDirName)
			}
			return 42, []byte("custom"), nil
		},
	}
	code, out, err := lang.Run(context.Background(), "/prefix", "/work", "entry", nil, nil, "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected RunFn to be called")
	}
	if code != 42 {
		t.Errorf("expected code 42, got %d", code)
	}
	if string(out) != "custom" {
		t.Errorf("expected 'custom', got %q", string(out))
	}
}

func TestSimpleLanguage_Run_EnvFnOverride(t *testing.T) {
	lang := &SimpleLanguage{
		EnvDirName: "test_env",
		RunEnvFn: func(envDir string) []string {
			return []string{"CUSTOM_VAR=hello"}
		},
	}
	code, _, err := lang.Run(context.Background(), "/prefix", t.TempDir(), "true", nil, nil, "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
}

func TestSimpleLanguage_ImplementsInterface(t *testing.T) {
	// Compile-time check that SimpleLanguage implements Language.
	var _ Language = (*SimpleLanguage)(nil)
}
