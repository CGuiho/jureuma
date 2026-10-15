package doc

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// ---- small builders used by the tables ----

type kv struct {
	k string
	v *Value
}

func obj(pairs ...kv) *Value {
	o := NewObject()
	for _, p := range pairs {
		o.Put(p.k, p.v)
	}
	return o
}

func arr(vs ...*Value) *Value { return NewArray(vs...) }

func mustNum(s string) *Value {
	v, err := NewNumber(s)
	if err != nil {
		panic(err)
	}
	return v
}

// ---- JSON Pointer (RFC 6901) ----

func TestParsePointer(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
		err  bool
	}{
		{"root", "", nil, false},
		{"single token", "/a", []string{"a"}, false},
		{"two tokens", "/a/b", []string{"a", "b"}, false},
		{"empty token", "/", []string{""}, false},
		{"empty token between", "/a//b", []string{"a", "", "b"}, false},
		{"digit token", "/0", []string{"0"}, false},
		{"spaces kept", "/a b/c d", []string{"a b", "c d"}, false},
		{"slash escape", "/a~1b", []string{"a/b"}, false},
		{"slash escape alone", "/~1", []string{"/"}, false},
		{"tilde escape", "/m~0n", []string{"m~n"}, false},
		{"tilde escape alone", "/~0", []string{"~"}, false},
		{"tilde escape decodes before slash escape", "/~01", []string{"~1"}, false},
		{"zero zero decodes to tilde zero", "/~00", []string{"~0"}, false},
		{"both escapes in one token", "/~01~1~0", []string{"~1/~"}, false},
		{"token with both kinds", "/a~1b~0c", []string{"a/b~c"}, false},
		// errors
		{"no leading slash", "a/b", nil, true},
		{"bare tilde", "/~", nil, true},
		{"trailing tilde", "/a~", nil, true},
		{"bad escape digit", "/~2", nil, true},
		{"bad escape later token", "/a/~x", nil, true},
		{"double tilde", "/~~0", nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParsePointer(c.in)
			if c.err {
				if err == nil {
					t.Fatalf("ParsePointer(%q) = %v, want error", c.in, got)
				}
				if !errors.Is(err, ErrPointer) {
					t.Fatalf("error %v does not wrap ErrPointer", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParsePointer(%q): %v", c.in, err)
			}
			if len(got) != len(c.want) {
				t.Fatalf("ParsePointer(%q) = %v, want %v", c.in, got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("ParsePointer(%q) = %v, want %v", c.in, got, c.want)
				}
			}
			if c.in != "" {
				if back := FormatPointer(c.want); back != c.in {
					t.Fatalf("FormatPointer round trip = %q, want %q", back, c.in)
				}
			}
		})
	}
}

// ---- SetAt / DeleteAt ----

func TestSetAt(t *testing.T) {
	one := mustNum("1")
	two := mustNum("2")
	three := mustNum("3")
	nine := mustNum("9")
	cases := []struct {
		name    string
		root    *Value
		tokens  []string
		val     *Value
		want    string
		wantErr bool
	}{
		{"object new member keeps order", obj(kv{"a", one}), []string{"b"}, two, `{"a":1,"b":2}`, false},
		{"object replace keeps order", obj(kv{"b", one}, kv{"a", two}), []string{"b"}, nine, `{"b":9,"a":2}`, false},
		{"creates missing objects", NewObject(), []string{"a", "b"}, NewString("x"), `{"a":{"b":"x"}}`, false},
		{"through scalar errors", obj(kv{"a", one}), []string{"a", "b"}, two, "", true},
		{"empty tokens error", obj(kv{"a", one}), nil, two, "", true},
		{"array replace element", arr(one, two), []string{"0"}, nine, `[9,2]`, false},
		{"array append by index", arr(one), []string{"1"}, two, `[1,2]`, false},
		{"array append by dash", arr(one, two), []string{"-"}, three, `[1,2,3]`, false},
		{"array dash on empty", NewArray(), []string{"-"}, one, `[1]`, false},
		{"array index beyond length errors", arr(one), []string{"5"}, two, "", true},
		{"array leading zero index errors", arr(one), []string{"01"}, two, "", true},
		{"array negative index errors", arr(one), []string{"-1"}, two, "", true},
		{"append creates object then member", NewArray(), []string{"-", "x"}, one, `[{"x":1}]`, false},
		{"existing element then member", arr(obj(kv{"a", one})), []string{"0", "b"}, two, `[{"a":1,"b":2}]`, false},
		{"through array element scalar errors", arr(one), []string{"0", "x"}, two, "", true},
		{"nested set", obj(kv{"a", arr(one)}), []string{"a", "-"}, two, `{"a":[1,2]}`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := SetAt(c.root, c.tokens, c.val)
			if c.wantErr {
				if err == nil {
					t.Fatalf("SetAt(%v) succeeded, want error", c.tokens)
				}
				return
			}
			if err != nil {
				t.Fatalf("SetAt(%v): %v", c.tokens, err)
			}
			if got := string(MarshalCompact(c.root)); got != c.want {
				t.Fatalf("got %s, want %s", got, c.want)
			}
		})
	}
}

func TestDeleteAt(t *testing.T) {
	one := mustNum("1")
	two := mustNum("2")
	three := mustNum("3")
	cases := []struct {
		name    string
		root    *Value
		tokens  []string
		want    string
		wantErr bool
	}{
		{"object member", obj(kv{"a", one}, kv{"b", two}), []string{"a"}, `{"b":2}`, false},
		{"object missing errors", obj(kv{"a", one}), []string{"z"}, "", true},
		{"object empty key", obj(kv{"", one}, kv{"b", two}), []string{""}, `{"b":2}`, false},
		{"array middle shifts left", arr(one, two, three), []string{"1"}, `[1,3]`, false},
		{"array last", arr(one, two, three), []string{"2"}, `[1,2]`, false},
		{"array out of range errors", arr(one), []string{"1"}, "", true},
		{"array dash errors", arr(one, two), []string{"-"}, "", true},
		{"array bad index errors", arr(one, two), []string{"01"}, "", true},
		{"root errors", obj(kv{"a", one}), nil, "", true},
		{"through scalar errors", obj(kv{"a", one}), []string{"a", "b"}, "", true},
		{"nested", obj(kv{"a", arr(one, obj(kv{"b", two}))}), []string{"a", "1", "b"}, `{"a":[1,{}]}`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := DeleteAt(c.root, c.tokens)
			if c.wantErr {
				if err == nil {
					t.Fatalf("DeleteAt(%v) succeeded, want error", c.tokens)
				}
				return
			}
			if err != nil {
				t.Fatalf("DeleteAt(%v): %v", c.tokens, err)
			}
			if got := string(MarshalCompact(c.root)); got != c.want {
				t.Fatalf("got %s, want %s", got, c.want)
			}
		})
	}
}

// ---- Canonical ----

func TestCanonical(t *testing.T) {
	cases := []struct {
		name string
		v    *Value
		want string
	}{
		{"null", NewNull(), "null"},
		{"true", NewBool(true), "true"},
		{"false", NewBool(false), "false"},
		{"number verbatim", mustNum("1.50E+010"), "1.50E+010"},
		{"number negative zero", mustNum("-0"), "-0"},
		{"number bignum", mustNum("123456789012345678901234567890"), "123456789012345678901234567890"},
		{"number fraction", mustNum("0.10"), "0.10"},
		{"empty string", NewString(""), `""`},
		{"quote", NewString(`a"b`), `"a\"b"`},
		{"backslash", NewString(`a\b`), `"a\\b"`},
		{"solidus not escaped", NewString("a/b"), `"a/b"`},
		{"short controls", NewString("\b\f\n\r\t"), `"\b\f\n\r\t"`},
		{"low controls as lowercase escapes", NewString("\x00\x01\x1f"), `"\u0000\u0001\u001f"`},
		{"space and tilde raw", NewString("a ~ b"), `"a ~ b"`},
		{"unicode raw", NewString("é中😀"), `"é中😀"`},
		{"del raw", NewString("\x7f"), "\"\x7f\""},
		// RuneError is emitted literally for invalid UTF-8 input bytes.
		{"invalid utf8 becomes replacement", NewString("a\xffb"), "\"a\uFFFDb\""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := string(Canonical(c.v)); got != c.want {
				t.Fatalf("got %s, want %s", got, c.want)
			}
		})
	}
}

func TestCanonicalPanicsOnContainers(t *testing.T) {
	for _, v := range []*Value{NewObject(), NewArray(), arr(NewNull())} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("Canonical(%s) did not panic", v.Kind)
				}
			}()
			_ = Canonical(v)
		}()
	}
}

// ---- Marshal ----

func TestMarshal(t *testing.T) {
	cases := []struct {
		name string
		v    *Value
		want string
	}{
		{"scalar", NewString("x"), "\"x\"\n"},
		{"empty object", NewObject(), "{}\n"},
		{"empty array", NewArray(), "[]\n"},
		{"nested key order", obj(kv{"b", mustNum("1")}, kv{"a", arr(mustNum("1"), mustNum("2"))}),
			"{\n  \"b\": 1,\n  \"a\": [\n    1,\n    2\n  ]\n}\n"},
		{"object member", obj(kv{"a", NewObject()}), "{\n  \"a\": {}\n}\n"},
		{"escaped key", obj(kv{`a"b`, mustNum("1")}), "{\n  \"a\\\"b\": 1\n}\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := string(Marshal(c.v)); got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestMarshalCompact(t *testing.T) {
	cases := []struct {
		name string
		v    *Value
		want string
	}{
		{"scalar", mustNum("1"), "1"},
		{"empty object", NewObject(), "{}"},
		{"nested", obj(kv{"a", arr(mustNum("1"), NewString("x"))}, kv{"b", NewNull()}), `{"a":[1,"x"],"b":null}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := string(MarshalCompact(c.v)); got != c.want {
				t.Fatalf("got %s, want %s", got, c.want)
			}
		})
	}
}

// ---- FromAny ----

func TestFromAny(t *testing.T) {
	cases := []struct {
		name    string
		in      any
		want    string
		wantErr bool
	}{
		{"nil", nil, "null", false},
		{"bool true", true, "true", false},
		{"bool false", false, "false", false},
		{"string", "s", `"s"`, false},
		{"json number verbatim", json.Number("1.50"), "1.50", false},
		{"json number invalid", json.Number("1.2.3"), "", true},
		{"int", -7, "-7", false},
		{"float64", 2.5, "2.5", false},
		{"float64 whole", 1.0, "1", false},
		{"float64 exponent", 1e21, "1e+21", false},
		{"map keys sorted", map[string]any{"b": 1, "a": 2}, `{"a":2,"b":1}`, false},
		{"array", []any{nil, true, "x", json.Number("3")}, `[null,true,"x",3]`, false},
		{"nested", map[string]any{"a": []any{map[string]any{"z": 1}}}, `{"a":[{"z":1}]}`, false},
		{"unsupported struct", struct{}{}, "", true},
		{"unsupported float32", float32(1), "", true},
		{"unsupported nested", []any{struct{}{}}, "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v, err := FromAny(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatalf("FromAny(%v) succeeded, want error", c.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("FromAny(%v): %v", c.in, err)
			}
			if got := string(MarshalCompact(v)); got != c.want {
				t.Fatalf("got %s, want %s", got, c.want)
			}
		})
	}
}

func TestValidNumber(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"0", true}, {"-0", true}, {"1", true}, {"1.50E+010", true}, {"1e-3", true},
		{"", false}, {"-", false}, {"01", false}, {"1.", false}, {".5", false},
		{"1e", false}, {"1e+", false}, {"+1", false}, {"1.2.3", false}, {"1 ", false},
		{"NaN", false}, {"Infinity", false}, {"0x10", false},
	}
	for _, c := range cases {
		if got := ValidNumber(c.in); got != c.want {
			t.Errorf("ValidNumber(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestErrorsNeverContainInput guards the package rule that no error message
// carries document content.
func TestErrorsNeverContainInput(t *testing.T) {
	secret := "dummy-value-not-a-secret"
	root := obj(kv{"a", NewString(secret)})
	if err := SetAt(root, []string{"a", "b"}, NewNull()); err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("SetAt error leaked input: %v", err)
	}
	if err := DeleteAt(root, []string{"missing"}); err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("DeleteAt error leaked input: %v", err)
	}
}
