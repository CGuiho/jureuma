// Package xerr carries jureuma's exit-code classification on errors.
//
// Messages built here must never contain configuration values, plaintext, or
// key material; callers pass only paths, JSON Pointers, and fixed phrases.
package xerr

import (
	"errors"
	"fmt"
)

// Exit codes shared by every command except run.
const (
	CodeOK         = 0
	CodeFailure    = 1
	CodeUsage      = 2
	CodeProtected  = 3
	CodeValidation = 4
	CodeKey        = 5
	// CodeRun is returned by run for any jureuma-side failure.
	CodeRun = 125
)

// Kinds name the failure class; run prints them as "[JUREUMA:<KIND>]".
const (
	KindFailure    = "ERROR"
	KindUsage      = "USAGE"
	KindProtected  = "PROTECTED"
	KindValidation = "VALIDATION"
	KindKey        = "KEY"
	KindExec       = "EXEC"
)

// Error is an error with an exit code and kind.
type Error struct {
	Code int
	Kind string
	Msg  string
	Err  error
}

func (e *Error) Error() string {
	if e.Msg == "" && e.Err != nil {
		return e.Err.Error()
	}
	if e.Err != nil && e.Msg != "" {
		return e.Msg + ": " + e.Err.Error()
	}
	return e.Msg
}

func (e *Error) Unwrap() error { return e.Err }

func newf(code int, kind, format string, a ...any) *Error {
	return &Error{Code: code, Kind: kind, Msg: fmt.Sprintf(format, a...)}
}

// Usage returns an exit-2 error.
func Usage(format string, a ...any) error { return newf(CodeUsage, KindUsage, format, a...) }

// Validation returns an exit-4 error.
func Validation(format string, a ...any) error {
	return newf(CodeValidation, KindValidation, format, a...)
}

// Key returns an exit-5 error.
func Key(format string, a ...any) error { return newf(CodeKey, KindKey, format, a...) }

// Protected returns an exit-3 error.
func Protected(format string, a ...any) error {
	return newf(CodeProtected, KindProtected, format, a...)
}

// Failure returns an exit-1 error.
func Failure(format string, a ...any) error { return newf(CodeFailure, KindFailure, format, a...) }

// Wrap attaches code and kind to err, keeping err's text.
func Wrap(code int, kind string, err error) error {
	if err == nil {
		return nil
	}
	return &Error{Code: code, Kind: kind, Err: err}
}

// WrapMsg attaches code, kind and a prefix message to err.
func WrapMsg(code int, kind, msg string, err error) error {
	if err == nil {
		return nil
	}
	return &Error{Code: code, Kind: kind, Msg: msg, Err: err}
}

// CodeOf returns the exit code carried by err (1 when none).
func CodeOf(err error) int {
	if err == nil {
		return CodeOK
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return CodeFailure
}

// KindOf returns the kind carried by err (ERROR when none).
func KindOf(err error) string {
	var e *Error
	if errors.As(err, &e) && e.Kind != "" {
		return e.Kind
	}
	return KindFailure
}
