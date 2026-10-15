package xerr

import (
	"errors"
	"fmt"
	"testing"
)

// TestExitCodeTaxonomy pins the shared exit-code and kind taxonomy exactly as
// xerr defines it.
func TestExitCodeTaxonomy(t *testing.T) {
	codes := map[string][2]int{
		"CodeOK":         {CodeOK, 0},
		"CodeFailure":    {CodeFailure, 1},
		"CodeUsage":      {CodeUsage, 2},
		"CodeProtected":  {CodeProtected, 3},
		"CodeValidation": {CodeValidation, 4},
		"CodeKey":        {CodeKey, 5},
		"CodeRun":        {CodeRun, 125},
	}
	for name, v := range codes {
		if v[0] != v[1] {
			t.Errorf("%s = %d, want %d", name, v[0], v[1])
		}
	}
	kinds := map[string]string{
		"KindFailure":    KindFailure,
		"KindUsage":      KindUsage,
		"KindProtected":  KindProtected,
		"KindValidation": KindValidation,
		"KindKey":        KindKey,
		"KindExec":       KindExec,
	}
	wantKinds := map[string]string{
		"KindFailure":    "ERROR",
		"KindUsage":      "USAGE",
		"KindProtected":  "PROTECTED",
		"KindValidation": "VALIDATION",
		"KindKey":        "KEY",
		"KindExec":       "EXEC",
	}
	for name, got := range kinds {
		if got != wantKinds[name] {
			t.Errorf("%s = %q, want %q", name, got, wantKinds[name])
		}
	}
}

func TestConstructors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code int
		kind string
		msg  string
	}{
		{"failure", Failure("f %s", "x"), CodeFailure, KindFailure, "f x"},
		{"usage", Usage("u"), CodeUsage, KindUsage, "u"},
		{"protected", Protected("p"), CodeProtected, KindProtected, "p"},
		{"validation", Validation("v %d", 3), CodeValidation, KindValidation, "v 3"},
		{"key", Key("k"), CodeKey, KindKey, "k"},
	}
	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			if got := CodeOf(c.err); got != c.code {
				t.Errorf("CodeOf = %d, want %d", got, c.code)
			}
			if got := KindOf(c.err); got != c.kind {
				t.Errorf("KindOf = %q, want %q", got, c.kind)
			}
			if got := c.err.Error(); got != c.msg {
				t.Errorf("Error() = %q, want %q", got, c.msg)
			}
			var e *Error
			if !errors.As(c.err, &e) || e.Code != c.code || e.Kind != c.kind || e.Msg != c.msg {
				t.Errorf("errors.As = %+v", e)
			}
			if e.Unwrap() != nil {
				t.Errorf("Unwrap() = %v, want nil", e.Unwrap())
			}
		})
	}
}

func TestErrorMessageForms(t *testing.T) {
	base := errors.New("base")
	tests := []struct {
		name string
		err  *Error
		want string
	}{
		{"msg only", &Error{Msg: "only msg"}, "only msg"},
		{"err only", &Error{Err: base}, "base"},
		{"msg and err", &Error{Msg: "m", Err: base}, "m: base"},
		{"empty", &Error{}, ""},
	}
	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			if got := c.err.Error(); got != c.want {
				t.Errorf("Error() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestWrap(t *testing.T) {
	base := errors.New("base")

	if Wrap(CodeKey, KindKey, nil) != nil || WrapMsg(CodeRun, KindExec, "m", nil) != nil {
		t.Fatal("wrapping nil must return nil")
	}

	w := Wrap(CodeKey, KindKey, base)
	if w.Error() != "base" || CodeOf(w) != CodeKey || KindOf(w) != KindKey {
		t.Errorf("Wrap: %q code=%d kind=%q", w.Error(), CodeOf(w), KindOf(w))
	}
	if !errors.Is(w, base) {
		t.Error("Wrap must keep the wrapped error inspectable")
	}

	wm := WrapMsg(CodeRun, KindExec, "cannot exec", base)
	if wm.Error() != "cannot exec: base" || CodeOf(wm) != CodeRun || KindOf(wm) != KindExec {
		t.Errorf("WrapMsg: %q code=%d kind=%q", wm.Error(), CodeOf(wm), KindOf(wm))
	}
}

func TestClassification(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code int
		kind string
	}{
		{"nil", nil, CodeOK, KindFailure},
		{"plain error", errors.New("plain"), CodeFailure, KindFailure},
		{"formatted plain", fmt.Errorf("outer: %w", errors.New("plain")), CodeFailure, KindFailure},
		{"wrapped deep", fmt.Errorf("outer: %w", Validation("v")), CodeValidation, KindValidation},
		{"empty kind falls back", &Error{Code: CodeKey}, CodeKey, KindFailure},
		{"outermost wins", Wrap(CodeUsage, KindUsage, fmt.Errorf("outer: %w", Key("k"))), CodeUsage, KindUsage},
	}
	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			if got := CodeOf(c.err); got != c.code {
				t.Errorf("CodeOf = %d, want %d", got, c.code)
			}
			if got := KindOf(c.err); got != c.kind {
				t.Errorf("KindOf = %q, want %q", got, c.kind)
			}
		})
	}
}
