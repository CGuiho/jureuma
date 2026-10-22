package jsonc

import (
	"bytes"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/CGuiho/zeze/internal/doc"
)

func compact(t *testing.T, v *doc.Value) string {
	t.Helper()
	return string(doc.MarshalCompact(v))
}

func TestParseValid(t *testing.T) {
	cases := []struct {
		name   string
		in     string
		strict bool // also valid in strict mode
		want   string
	}{
		{"null", `null`, true, `null`},
		{"true", ` true `, true, `true`},
		{"false", "\n\tfalse\r\n", true, `false`},
		{"zero", `0`, true, `0`},
		{"negzero", `-0`, true, `-0`},
		{"number exact text", `1.50E+010`, true, `1.50E+010`},
		{"bignum", `123456789012345678901234567890`, true, `123456789012345678901234567890`},
		{"string", `"abc"`, true, `"abc"`},
		{"empty string", `""`, true, `""`},
		{"escapes", `"\"\\\/\b\f\n\r\t"`, true, `"\"\\/\b\f\n\r\t"`},
		{"unicode escape", `"\u00e9\u4e2D"`, true, `"é中"`},
		{"surrogate pair", `"\ud83d\ude00"`, true, `"😀"`},
		{"raw utf8", `"héllo 😀"`, true, `"héllo 😀"`},
		{"empty object", `{}`, true, `{}`},
		{"empty array", `[ ]`, true, `[]`},
		{"key order kept", `{"b":1,"a":2,"c":{"z":[],"y":{}}}`, true, `{"b":1,"a":2,"c":{"z":[],"y":{}}}`},
		{"nested arrays", `[[1,[2,[3]]],"x",null,true]`, true, `[[1,[2,[3]]],"x",null,true]`},
		{"bom", "\xEF\xBB\xBF{\"a\":1}", true, `{"a":1}`},
		{"empty key", `{"":1}`, true, `{"":1}`},
		{"keys differ by escape", `{"a":1,"\u0062":2}`, true, `{"a":1,"b":2}`},
		// lenient only
		{"line comment", "// hi\n{\"a\":1} // tail", false, `{"a":1}`},
		{"line comment eof", `1 //`, false, `1`},
		{"block comment", `/* x */ [1, /* y */ 2] /**/`, false, `[1,2]`},
		{"block comment stars", `/*** x ***/1`, false, `1`},
		{"comment in key position", `{/*c*/"a"/*c*/:/*c*/1/*c*/}`, false, `{"a":1}`},
		{"trailing comma array", `[1,2,]`, false, `[1,2]`},
		{"trailing comma object", "{\"a\":1,\n}", false, `{"a":1}`},
		{"trailing comma comment", `[1, // x
]`, false, `[1]`},
		{"comment chars in string", `"// not /* a comment"`, true, `"// not /* a comment"`},
		{"cr line comment", "// x\r1", false, `1`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v, err := Parse([]byte(c.in), false)
			if err != nil {
				t.Fatalf("lenient: unexpected error %v", err)
			}
			if got := compact(t, v); got != c.want {
				t.Fatalf("lenient: got %s want %s", got, c.want)
			}
			v, err = Parse([]byte(c.in), true)
			if c.strict {
				if err != nil {
					t.Fatalf("strict: unexpected error %v", err)
				}
				if got := compact(t, v); got != c.want {
					t.Fatalf("strict: got %s want %s", got, c.want)
				}
			} else if err == nil {
				t.Fatalf("strict: expected error")
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	cases := []struct {
		name      string
		in        string
		strict    bool
		line, col int
		class     string
	}{
		{"empty", ``, false, 1, 1, ClassUnexpectedEOF},
		{"only space", "  \n ", false, 2, 2, ClassUnexpectedEOF},
		{"only bom", "\xEF\xBB\xBF", true, 1, 1, ClassUnexpectedEOF},
		{"only comment", `// x`, false, 1, 5, ClassUnexpectedEOF},
		{"bad literal", `nul`, true, 1, 4, ClassUnexpectedEOF},
		{"bad literal char", `nulL`, true, 1, 4, ClassUnexpectedChar},
		{"capital True", `True`, true, 1, 1, ClassUnexpectedChar},
		{"single quote", `'a'`, false, 1, 1, ClassUnexpectedChar},
		{"bare word", `{a:1}`, false, 1, 2, ClassUnexpectedChar},
		{"plus number", `+1`, true, 1, 1, ClassUnexpectedChar},
		{"leading zero", `01`, true, 1, 1, ClassInvalidNumber},
		{"minus only", `[-]`, true, 1, 2, ClassInvalidNumber},
		{"dot no frac", `1.`, true, 1, 1, ClassInvalidNumber},
		{"leading dot", `.5`, true, 1, 1, ClassUnexpectedChar},
		{"exp no digits", `1e+`, true, 1, 1, ClassInvalidNumber},
		{"hex", `0x10`, true, 1, 2, ClassTrailingContent},
		{"NaN", `NaN`, false, 1, 1, ClassUnexpectedChar},
		{"bad escape", `"a\x"`, true, 1, 3, ClassInvalidEscape},
		{"bad unicode escape", `"\u12G4"`, true, 1, 2, ClassInvalidEscape},
		{"short unicode eof", `"\u12`, true, 1, 6, ClassUnexpectedEOF},
		{"lone low surrogate", `"\udc00"`, true, 1, 2, ClassInvalidEscape},
		{"lone high surrogate", `"\ud800x"`, true, 1, 2, ClassInvalidEscape},
		{"high then non-low", `"\ud800\u0041"`, true, 1, 2, ClassInvalidEscape},
		{"high then eof", `"\ud800`, true, 1, 8, ClassUnexpectedEOF},
		{"NUL escape", `{"k":"a\u0000b"}`, true, 1, 8, ClassNULInString},
		{"NUL escape in key", `{"\u0000":1}`, true, 1, 3, ClassNULInString},
		{"raw NUL", "\"a\x00\"", true, 1, 3, ClassNULInString},
		{"raw newline in string", "\"a\nb\"", true, 1, 3, ClassUnexpectedChar},
		{"raw tab in string", "\"a\tb\"", true, 1, 3, ClassUnexpectedChar},
		{"unterminated string", `"abc`, true, 1, 5, ClassUnexpectedEOF},
		{"unterminated escape", `"abc\`, true, 1, 6, ClassUnexpectedEOF},
		{"invalid utf8", "{\n \"k\": \"ab\xff\"}", true, 2, 10, ClassInvalidUTF8},
		{"invalid utf8 in comment", "1 // \xc3\x28", false, 1, 6, ClassInvalidUTF8},
		{"overlong utf8", "\"\xc0\xaf\"", true, 1, 2, ClassInvalidUTF8},
		{"utf16 surrogate in utf8", "\"\xed\xa0\x80\"", true, 1, 2, ClassInvalidUTF8},
		{"duplicate key", "{\"a\":1,\n  \"a\":2}", true, 2, 3, ClassDuplicateKey},
		{"duplicate key via escape", `{"a":1,"\u0061":2}`, true, 1, 8, ClassDuplicateKey},
		{"nested duplicate", `{"x":{"a":1,"b":2,"a":3}}`, true, 1, 19, ClassDuplicateKey},
		{"trailing content", `{} {}`, true, 1, 4, ClassTrailingContent},
		{"trailing literal", `truefalse`, true, 1, 5, ClassTrailingContent},
		{"trailing comma", `1,`, false, 1, 2, ClassTrailingContent},
		{"strict trailing comma array", `[1,]`, true, 1, 4, ClassUnexpectedChar},
		{"strict trailing comma object", `{"a":1,}`, true, 1, 8, ClassUnexpectedChar},
		{"strict comment", `/*x*/1`, true, 1, 1, ClassUnexpectedChar},
		{"strict line comment after", "1 // x", true, 1, 3, ClassUnexpectedChar},
		{"lone comma array", `[,]`, false, 1, 2, ClassUnexpectedChar},
		{"double comma", `[1,,2]`, false, 1, 4, ClassUnexpectedChar},
		{"empty object comma", `{,}`, false, 1, 2, ClassUnexpectedChar},
		{"missing colon", `{"a" 1}`, true, 1, 6, ClassUnexpectedChar},
		{"missing colon eof", `{"a"`, true, 1, 5, ClassUnexpectedEOF},
		{"missing value", `{"a":}`, true, 1, 6, ClassUnexpectedChar},
		{"missing comma", `[1 2]`, true, 1, 4, ClassUnexpectedChar},
		{"unclosed array", "[1,\n2", true, 2, 2, ClassUnexpectedEOF},
		{"unclosed object", `{"a":1`, true, 1, 7, ClassUnexpectedEOF},
		{"mismatched close", `[1}`, true, 1, 3, ClassUnexpectedChar},
		{"unterminated block comment", "1 /* x\n y", false, 1, 3, ClassUnterminatedComment},
		{"slash alone", `1 /`, false, 1, 3, ClassUnexpectedChar},
		{"slash other", `1 /x`, false, 1, 3, ClassUnexpectedChar},
		{"column counts runes", `"é€😀" x`, true, 1, 7, ClassTrailingContent},
		{"crlf lines", "[\r\n1\r\n,]", true, 3, 2, ClassUnexpectedChar},
		{"cr lines", "[\r1\r,]", true, 3, 2, ClassUnexpectedChar},
		{"bom not a column", "\xEF\xBB\xBF x", true, 1, 2, ClassUnexpectedChar},
		{"bom in middle", "1\xEF\xBB\xBF", true, 1, 2, ClassTrailingContent},
		{"non-breaking space", "\u00a01", true, 1, 1, ClassUnexpectedChar},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v, err := Parse([]byte(c.in), c.strict)
			if err == nil {
				t.Fatalf("expected error, got %s", doc.MarshalCompact(v))
			}
			if v != nil {
				t.Fatalf("value must be nil on error")
			}
			var se *SyntaxError
			if !errors.As(err, &se) {
				t.Fatalf("error type %T", err)
			}
			if se.Line != c.line || se.Column != c.col || se.Class != c.class {
				t.Fatalf("got %d:%d %q want %d:%d %q", se.Line, se.Column, se.Class, c.line, c.col, c.class)
			}
		})
	}
}

func TestSyntaxErrorFormat(t *testing.T) {
	e := &SyntaxError{Line: 3, Column: 14, Class: ClassDuplicateKey}
	if got := e.Error(); got != "line 3, column 14: duplicate key" {
		t.Fatalf("got %q", got)
	}
}

func TestDepthLimit(t *testing.T) {
	ok := strings.Repeat("[", MaxDepth) + strings.Repeat("]", MaxDepth)
	if _, err := Parse([]byte(ok), true); err != nil {
		t.Fatalf("depth %d: %v", MaxDepth, err)
	}
	deep := strings.Repeat(`{"a":`, MaxDepth+1)
	_, err := Parse([]byte(deep), true)
	var se *SyntaxError
	if !errors.As(err, &se) || se.Class != ClassTooDeep {
		t.Fatalf("got %v", err)
	}
	// Very deep input must not crash.
	if _, err := Parse(bytes.Repeat([]byte("["), 5_000_000), false); err == nil {
		t.Fatal("expected error")
	}
}

func TestHasComments(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{`{}`, false},
		{`{"a":"//x"}`, false},
		{`{"a":"/*x*/"}`, false},
		{`{"a":"\"//"}`, false},
		{`{"a":"\\"//c` + "\n}", true},
		{"{} // c", true},
		{"/* c */{}", true},
		{"{\"url\":\"http://x\"}", false},
		{`1 / 2`, false},
		{"\xEF\xBB\xBF//x\n1", true},
		{`"unterminated //`, false},
		{`/`, false},
	}
	for _, c := range cases {
		if got := HasComments([]byte(c.in)); got != c.want {
			t.Errorf("HasComments(%q) = %v want %v", c.in, got, c.want)
		}
	}
}

var errFormat = regexp.MustCompile(`^line [1-9][0-9]*, column [1-9][0-9]*: (unexpected character|unexpected end of input|invalid escape|invalid number|duplicate key|NUL in string|invalid UTF-8|unterminated comment|trailing content|nesting too deep)$`)

// TestErrorsNeverContainInput proves no error message echoes source bytes:
// every input embeds a secret marker at or around the failure point, and the
// message must match the fixed format and contain no marker substring.
func TestErrorsNeverContainInput(t *testing.T) {
	const secret = "S3cr3tVal"
	templates := []string{
		`{"k":"%s`, `{"k":"%s\q"}`, `{"k":"%s\u00"}`, `{"k":"%s\u0000"}`,
		`{"%s":1,"%s":2}`, `{"k":%s}`, `[%s]`, `%s`, `"%s" %s`,
		`{"k":"%s"} %s`, `{"k":"%s"} /* %s`, `{"k":-%s}`, `{"k":1%s}`,
		`{"k":"%s` + "\x00" + `"}`, `{"k":"%s` + "\xff" + `"}`, `{"k":"%s` + "\n" + `"}`,
		`{%s:1}`, `{"k" %s}`, `{"k":"v",%s}`, `["%s",,]`, `/%s`, `{"k":t%s}`,
		`{"k":"\ud800%s"}`, `{"k":0%s}`, `{"k":1e%s}`,
	}
	for _, tpl := range templates {
		in := strings.ReplaceAll(tpl, "%s", secret)
		for _, strict := range []bool{true, false} {
			_, err := Parse([]byte(in), strict)
			if err == nil {
				t.Fatalf("expected error for template %q", tpl)
			}
			msg := err.Error()
			if !errFormat.MatchString(msg) {
				t.Fatalf("message %q does not match fixed format", msg)
			}
			for i := 0; i+3 <= len(secret); i++ {
				if strings.Contains(msg, secret[i:i+3]) {
					t.Fatalf("message %q contains input fragment %q", msg, secret[i:i+3])
				}
			}
		}
	}
}

// TestTableFuzz mutates valid documents byte-by-byte (deletion, duplication,
// replacement with interesting bytes, truncation) and checks the invariants of
// Parse on every mutant.
func TestTableFuzz(t *testing.T) {
	seeds := []string{
		`{"a":[1,-2.5e3,true,false,null],"b":{"c":"d\u00e9\n"},"e":""}`,
		"// c\n{\"a\":1, /* x */ \"b\":[1,2,],}",
		`"\ud83d\ude00"`,
		`[[[{}]]]`,
	}
	interesting := []byte{0, '"', '\\', '/', '*', ',', ':', '[', ']', '{', '}', 'u', '0', '-', 'e', '.', ' ', '\n', 0xff, 0xc3, 0xef}
	n := 0
	for _, s := range seeds {
		src := []byte(s)
		for i := 0; i <= len(src); i++ {
			checkInvariants(t, src[:i])
			if i == len(src) {
				break
			}
			del := append(append([]byte{}, src[:i]...), src[i+1:]...)
			checkInvariants(t, del)
			dup := append(append([]byte{}, src[:i+1]...), src[i:]...)
			checkInvariants(t, dup)
			for _, b := range interesting {
				rep := append([]byte{}, src...)
				rep[i] = b
				checkInvariants(t, rep)
				n++
			}
		}
	}
	if n == 0 {
		t.Fatal("no mutants")
	}
}

func checkInvariants(t *testing.T, data []byte) {
	t.Helper()
	for _, strict := range []bool{true, false} {
		v, err := Parse(data, strict)
		if err != nil {
			var se *SyntaxError
			if !errors.As(err, &se) {
				t.Fatalf("%q: error type %T", data, err)
			}
			if v != nil {
				t.Fatalf("%q: non-nil value with error", data)
			}
			if !errFormat.MatchString(err.Error()) {
				t.Fatalf("%q: bad message %q", data, err.Error())
			}
			continue
		}
		out := doc.MarshalCompact(v)
		// Output is strict JSON and round-trips to an equal tree.
		v2, err := Parse(out, true)
		if err != nil {
			t.Fatalf("%q: re-parse of %q failed: %v", data, out, err)
		}
		if !doc.Equal(v, v2) {
			t.Fatalf("%q: round trip changed value", data)
		}
		if !utf8.Valid(out) {
			t.Fatalf("%q: output not UTF-8", data)
		}
		body := bytes.TrimPrefix(data, []byte("\xEF\xBB\xBF"))
		if strict && !json.Valid(body) {
			t.Fatalf("%q: strict accepted input encoding/json rejects", data)
		}
		if !strict && !HasComments(data) {
			// Without comments, lenient differs from strict only in trailing commas.
			if _, serr := Parse(data, true); serr != nil {
				var se *SyntaxError
				errors.As(serr, &se)
				if se.Class != ClassUnexpectedChar {
					t.Fatalf("%q: lenient ok, strict class %q", data, se.Class)
				}
			}
		}
	}
	// Strict rejecting what encoding/json accepts is limited to our extra rules.
	body := bytes.TrimPrefix(data, []byte("\xEF\xBB\xBF"))
	if json.Valid(body) && utf8.Valid(body) {
		if _, err := Parse(data, true); err != nil {
			var se *SyntaxError
			errors.As(err, &se)
			switch se.Class {
			case ClassDuplicateKey, ClassNULInString, ClassTooDeep, ClassInvalidEscape:
				// ClassInvalidEscape: lone surrogate escapes, which encoding/json tolerates.
			default:
				t.Fatalf("%q: encoding/json accepts, strict says %q", data, se.Class)
			}
		}
	}
}

func FuzzParse(f *testing.F) {
	for _, s := range []string{
		`{}`, `[]`, `{"a":1}`, `"\u0000"`, `{"a":1,"a":2}`, "// x\n[1,]", `/* */1`,
		"\xEF\xBB\xBF{}", `"\ud800"`, `1e5`, `-0.0`, "\"\xff\"", `[1,,]`,
	} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		checkInvariants(t, data)
		HasComments(data)
	})
}
