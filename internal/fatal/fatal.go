// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

// Package fatal holds upstream's FatalError family: the errors pre-commit
// expects, which its error_handler reports as "An error has occurred" and
// exit 1, as opposed to any other error, which it reports as unexpected and
// exits 3.
package fatal

import "fmt"

// Error is upstream's FatalError or one of its subclasses. Type is the
// Python class name the error handler prints (FatalError,
// InvalidConfigError, InvalidManifestError), Msg its str().
type Error struct {
	Type string
	Msg  string
}

func (e *Error) Error() string { return e.Msg }

// New is a plain FatalError.
func New(msg string) *Error { return &Error{Type: "FatalError", Msg: msg} }

// Newf is New with formatting.
func Newf(format string, args ...any) *Error {
	return New(fmt.Sprintf(format, args...))
}
