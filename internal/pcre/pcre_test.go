package pcre

import (
	"testing"
)

// ---------------------------------------------------------------------------
// Compile – basic RE2 patterns
// ---------------------------------------------------------------------------

func TestCompileSimplePattern(t *testing.T) {
	re, err := Compile(`hello`)
	if err != nil {
		t.Fatalf("Compile(hello): %v", err)
	}
	if re == nil {
		t.Fatal("Compile(hello) returned nil")
	}
}

func TestCompileCharacterClass(t *testing.T) {
	re, err := Compile(`[a-z]+\d{2,4}`)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if re == nil {
		t.Fatal("Compile returned nil")
	}
}

func TestCompileAlternation(t *testing.T) {
	re, err := Compile(`foo|bar|baz`)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if re == nil {
		t.Fatal("Compile returned nil")
	}
}

// ---------------------------------------------------------------------------
// Compile – PCRE-only features (lookahead, lookbehind, backreferences)
// ---------------------------------------------------------------------------

func TestCompileLookahead(t *testing.T) {
	re, err := Compile(`foo(?=bar)`)
	if err != nil {
		t.Fatalf("Compile(lookahead): %v", err)
	}
	ok, _ := re.MatchString("foobar")
	if !ok {
		t.Error("lookahead pattern should match 'foobar'")
	}
	ok, _ = re.MatchString("foobaz")
	if ok {
		t.Error("lookahead pattern should not match 'foobaz'")
	}
}

func TestCompileNegativeLookahead(t *testing.T) {
	re, err := Compile(`foo(?!bar)`)
	if err != nil {
		t.Fatalf("Compile(negative lookahead): %v", err)
	}
	ok, _ := re.MatchString("foobaz")
	if !ok {
		t.Error("negative lookahead should match 'foobaz'")
	}
	ok, _ = re.MatchString("foobar")
	if ok {
		t.Error("negative lookahead should not match 'foobar'")
	}
}

func TestCompileLookbehind(t *testing.T) {
	re, err := Compile(`(?<=foo)bar`)
	if err != nil {
		t.Fatalf("Compile(lookbehind): %v", err)
	}
	ok, _ := re.MatchString("foobar")
	if !ok {
		t.Error("lookbehind pattern should match 'foobar'")
	}
	ok, _ = re.MatchString("bazbar")
	if ok {
		t.Error("lookbehind pattern should not match 'bazbar'")
	}
}

func TestCompileBackreference(t *testing.T) {
	re, err := Compile(`(a+)b\1`)
	if err != nil {
		t.Fatalf("Compile(backreference): %v", err)
	}
	ok, _ := re.MatchString("aabaa")
	if !ok {
		t.Error("backreference pattern should match 'aabaa'")
	}
	ok, _ = re.MatchString("aabx")
	if ok {
		t.Error("backreference pattern should not match 'aabx'")
	}
}

// ---------------------------------------------------------------------------
// MatchString – positive and negative cases
// ---------------------------------------------------------------------------

func TestMatchStringPositive(t *testing.T) {
	ok, err := MatchString(`\d+`, "abc123def")
	if err != nil {
		t.Fatalf("MatchString: %v", err)
	}
	if !ok {
		t.Error("MatchString should match digits in 'abc123def'")
	}
}

func TestMatchStringNegative(t *testing.T) {
	ok, err := MatchString(`\d+`, "abcdef")
	if err != nil {
		t.Fatalf("MatchString: %v", err)
	}
	if ok {
		t.Error("MatchString should not match digits in 'abcdef'")
	}
}

func TestMatchStringEmptyPattern(t *testing.T) {
	ok, err := MatchString(``, "anything")
	if err != nil {
		t.Fatalf("MatchString: %v", err)
	}
	if !ok {
		t.Error("empty pattern should match any string")
	}
}

func TestMatchStringEmptyString(t *testing.T) {
	ok, err := MatchString(`abc`, "")
	if err != nil {
		t.Fatalf("MatchString: %v", err)
	}
	if ok {
		t.Error("pattern 'abc' should not match empty string")
	}
}

// ---------------------------------------------------------------------------
// Match – helper function
// ---------------------------------------------------------------------------

func TestMatchPositive(t *testing.T) {
	re, err := Compile(`world`)
	if err != nil {
		t.Fatal(err)
	}
	if !Match(re, "hello world") {
		t.Error("Match should return true for 'hello world'")
	}
}

func TestMatchNegative(t *testing.T) {
	re, err := Compile(`xyz`)
	if err != nil {
		t.Fatal(err)
	}
	if Match(re, "hello world") {
		t.Error("Match should return false for 'hello world' with pattern 'xyz'")
	}
}

// ---------------------------------------------------------------------------
// FindString – returns correct match
// ---------------------------------------------------------------------------

func TestFindStringReturnsFirstMatch(t *testing.T) {
	re, err := Compile(`\d+`)
	if err != nil {
		t.Fatal(err)
	}
	got := FindString(re, "abc 123 def 456")
	if got != "123" {
		t.Errorf("FindString = %q, want %q", got, "123")
	}
}

func TestFindStringNoMatch(t *testing.T) {
	re, err := Compile(`\d+`)
	if err != nil {
		t.Fatal(err)
	}
	got := FindString(re, "abcdef")
	if got != "" {
		t.Errorf("FindString = %q, want empty string", got)
	}
}

func TestFindStringWithGroups(t *testing.T) {
	re, err := Compile(`(foo)(bar)`)
	if err != nil {
		t.Fatal(err)
	}
	got := FindString(re, "foobar baz")
	if got != "foobar" {
		t.Errorf("FindString = %q, want %q", got, "foobar")
	}
}

// ---------------------------------------------------------------------------
// Invalid pattern returns error
// ---------------------------------------------------------------------------

func TestCompileInvalidPattern(t *testing.T) {
	_, err := Compile(`[invalid`)
	if err == nil {
		t.Error("Compile with invalid pattern should return error")
	}
}

func TestCompileInvalidPatternUnmatchedParen(t *testing.T) {
	_, err := Compile(`(abc`)
	if err == nil {
		t.Error("Compile with unmatched paren should return error")
	}
}

func TestMatchStringInvalidPattern(t *testing.T) {
	_, err := MatchString(`[invalid`, "test")
	if err == nil {
		t.Error("MatchString with invalid pattern should return error")
	}
}

// ---------------------------------------------------------------------------
// MustCompile – panics on bad pattern
// ---------------------------------------------------------------------------

func TestMustCompileValid(t *testing.T) {
	// Should not panic.
	re := MustCompile(`\d+`)
	if re == nil {
		t.Fatal("MustCompile returned nil for valid pattern")
	}
}

func TestMustCompilePanicsOnInvalid(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustCompile should panic on invalid pattern")
		}
	}()
	MustCompile(`[invalid`)
}

// ---------------------------------------------------------------------------
// Compile sets timeout
// ---------------------------------------------------------------------------

func TestCompileSetsTimeout(t *testing.T) {
	re, err := Compile(`abc`)
	if err != nil {
		t.Fatal(err)
	}
	if re.MatchTimeout != DefaultTimeout {
		t.Errorf("MatchTimeout = %v, want %v", re.MatchTimeout, DefaultTimeout)
	}
}
