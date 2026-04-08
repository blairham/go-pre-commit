package output

import (
	"testing"
)

func TestSetColorModeFromStringAlways(t *testing.T) {
	SetColorModeFromString("always")
	if currentColorMode != ColorAlways {
		t.Fatalf("expected ColorAlways, got %d", currentColorMode)
	}
}

func TestSetColorModeFromStringNever(t *testing.T) {
	SetColorModeFromString("never")
	if currentColorMode != ColorNever {
		t.Fatalf("expected ColorNever, got %d", currentColorMode)
	}
}

func TestSetColorModeFromStringAuto(t *testing.T) {
	SetColorModeFromString("auto")
	if currentColorMode != ColorAuto {
		t.Fatalf("expected ColorAuto, got %d", currentColorMode)
	}
}

func TestSetColorModeFromStringCaseInsensitive(t *testing.T) {
	SetColorModeFromString("ALWAYS")
	if currentColorMode != ColorAlways {
		t.Fatalf("expected ColorAlways for uppercase input, got %d", currentColorMode)
	}

	SetColorModeFromString("Never")
	if currentColorMode != ColorNever {
		t.Fatalf("expected ColorNever for mixed case input, got %d", currentColorMode)
	}
}

func TestSetColorModeFromStringUnknownDefaultsToAuto(t *testing.T) {
	SetColorModeFromString("bogus")
	if currentColorMode != ColorAuto {
		t.Fatalf("expected ColorAuto for unknown string, got %d", currentColorMode)
	}
}

func TestHookResultStringPassed(t *testing.T) {
	if ResultPassed.String() != "Passed" {
		t.Fatalf("expected Passed, got %s", ResultPassed.String())
	}
}

func TestHookResultStringFailed(t *testing.T) {
	if ResultFailed.String() != "Failed" {
		t.Fatalf("expected Failed, got %s", ResultFailed.String())
	}
}

func TestHookResultStringSkipped(t *testing.T) {
	if ResultSkipped.String() != "Skipped" {
		t.Fatalf("expected Skipped, got %s", ResultSkipped.String())
	}
}

func TestHookResultStringError(t *testing.T) {
	if ResultError.String() != "Error" {
		t.Fatalf("expected Error, got %s", ResultError.String())
	}
}

func TestHookResultStringUnknown(t *testing.T) {
	unknown := HookResult(99)
	if unknown.String() != "Unknown" {
		t.Fatalf("expected Unknown, got %s", unknown.String())
	}
}

func TestTerminalWidthDefault(t *testing.T) {
	t.Setenv("COLUMNS", "")
	w := TerminalWidth()
	if w != 80 {
		t.Fatalf("expected default width 80, got %d", w)
	}
}

func TestTerminalWidthFromEnv(t *testing.T) {
	t.Setenv("COLUMNS", "120")
	w := TerminalWidth()
	if w != 120 {
		t.Fatalf("expected 120, got %d", w)
	}
}

func TestTerminalWidthInvalidEnvFallsBack(t *testing.T) {
	t.Setenv("COLUMNS", "notanumber")
	w := TerminalWidth()
	if w != 80 {
		t.Fatalf("expected fallback width 80 for invalid COLUMNS, got %d", w)
	}
}
