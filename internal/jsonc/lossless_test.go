package jsonc

import (
	"errors"
	"strings"
	"testing"

	"github.com/CGuiho/zeze/internal/doc"
)

// The comment-preserving JSONC writer (issue #12): minimal byte-span splices,
// golden preservation of comments, blank lines, number text and string-escape
// spelling, and refusal instead of rewriting.

// edit applies mutations (JSON Pointer steps) to a copy of the parsed
// original and returns the edited text.
func edit(t *testing.T, original string, steps ...func(*doc.Value)) string {
	t.Helper()
	old, err := Parse([]byte(original), false)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	next := old.Clone()
	for _, step := range steps {
		step(next)
	}
	out, err := Edit([]byte(original), old, next)
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	return string(out)
}

func setAt(pointer string, rawJSON string) func(*doc.Value) {
	return func(v *doc.Value) {
		toks, err := doc.ParsePointer(pointer)
		if err != nil {
			panic(err)
		}
		val, err := Parse([]byte(rawJSON), true)
		if err != nil {
			panic(err)
		}
		if err := doc.SetAt(v, toks, val); err != nil {
			panic(err)
		}
	}
}

func delAt(pointer string) func(*doc.Value) {
	return func(v *doc.Value) {
		toks, err := doc.ParsePointer(pointer)
		if err != nil {
			panic(err)
		}
		if err := doc.DeleteAt(v, toks); err != nil {
			panic(err)
		}
	}
}

func TestEditReplacesOnlyTheValueSpan(t *testing.T) {
	in := `{
  // keep me
  "db": {
    "url": "u", // trailing note
    "pool": 1e3
  }
}
`
	want := `{
  // keep me
  "db": {
    "url": "v", // trailing note
    "pool": 1e3
  }
}
`
	if got := edit(t, in, setAt("/db/url", `"v"`)); got != want {
		t.Fatalf("replace:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestEditNumberAndEscapeSpellingPreserved(t *testing.T) {
	in := `{"n": 1e3, "m": -0, "k": "\u00e9 x", "s": "a\/b"}`
	want := `{"n": 1e3, "m": -0, "k": "\u00e9 x", "s": "a\/b", "j": 1}`
	if got := edit(t, in, setAt("/j", `1`)); got != want {
		t.Fatalf("spelling:\ngot  %s\nwant %s", got, want)
	}
	// A replaced number keeps the new text exactly.
	want2 := `{"n": 2.50, "m": -0, "k": "\u00e9 x", "s": "a\/b"}`
	if got := edit(t, in, setAt("/n", `2.50`)); got != want2 {
		t.Fatalf("number:\ngot  %s\nwant %s", got, want2)
	}
}

func TestEditInsertObjectNoTrailingComma(t *testing.T) {
	in := `{
  "a": 1
}
`
	want := `{
  "a": 1,
  "b": 2
}
`
	if got := edit(t, in, setAt("/b", `2`)); got != want {
		t.Fatalf("insert:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestEditInsertReusesTrailingComma(t *testing.T) {
	in := `{
  "a": 1,
}
`
	want := `{
  "a": 1,
  "b": 2
}
`
	got := edit(t, in, setAt("/b", `2`))
	if got != want {
		t.Fatalf("insert:\ngot:\n%s\nwant:\n%s", got, want)
	}
	// No new trailing comma is emitted.
	if strings.Contains(got, "2\n,") || strings.Contains(got, "2,\n}") {
		t.Fatalf("new trailing comma emitted:\n%s", got)
	}
}

func TestEditInsertSingleLineAndEmpty(t *testing.T) {
	if got, want := edit(t, `{ "a": 1 }`, setAt("/b", `2`)), `{ "a": 1, "b": 2 }`; got != want {
		t.Fatalf("single-line: got %s want %s", got, want)
	}
	if got, want := edit(t, `{"a":1}`, setAt("/b", `2`)), `{"a":1,"b":2}`; got != want {
		t.Fatalf("compact: got %s want %s", got, want)
	}
	if got, want := edit(t, `{}`, setAt("/a", `1`)), `{"a": 1}`; got != want {
		t.Fatalf("empty: got %s want %s", got, want)
	}
	if got, want := edit(t, "{\n}\n", setAt("/a", `1`)), "{\n  \"a\": 1\n}\n"; got != want {
		t.Fatalf("empty multiline: got %q want %q", got, want)
	}
}

func TestEditRemovals(t *testing.T) {
	if got, want := edit(t, `{"a": 1, "b": 2}`, delAt("/a")), `{ "b": 2}`; got != want {
		t.Fatalf("first member: got %s want %s", got, want)
	}
	if got, want := edit(t, `{"a": 1, "b": 2}`, delAt("/b")), `{"a": 1 }`; got != want {
		t.Fatalf("last member: got %s want %s", got, want)
	}
	if got, want := edit(t, `{ "a": 1, "b": 2, "c": 3 }`, delAt("/b")), `{ "a": 1,  "c": 3 }`; got != want {
		t.Fatalf("middle member: got %s want %s", got, want)
	}
	if got, want := edit(t, "{\n  \"a\": 1, // about a\n  \"b\": 2\n}\n", delAt("/a")),
		"{\n   // about a\n  \"b\": 2\n}\n"; got != want {
		t.Fatalf("comment outside the span survives:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestEditInteriorArrayRemovalTouchesOnlyThatElement(t *testing.T) {
	in := `[
  1, // one
  2,
  3
]
`
	// Only the removed element's span (and its comma) is edited: the two
	// spaces on its line survive.
	want := "[\n  1, // one\n  \n  3\n]\n"
	if got := edit(t, in, delAt("/1")); got != want {
		t.Fatalf("array removal:\ngot:\n%q\nwant:\n%q", got, want)
	}
	// The shifted-suffix case: the removed element is the only edit.
	in2 := `[10, 20, 30]`
	if got, want := edit(t, in2, delAt("/1")), `[10,  30]`; got != want {
		t.Fatalf("compact array removal: got %s want %s", got, want)
	}
}

func TestEditArrayAppendAndNestedValue(t *testing.T) {
	if got, want := edit(t, "[1, 2]", setAt("/-", `3`)), "[1, 2, 3]"; got != want {
		t.Fatalf("append: got %s want %s", got, want)
	}
	if got, want := edit(t, "[\n  1,\n  2\n]\n", setAt("/-", `3`)), "[\n  1,\n  2,\n  3\n]\n"; got != want {
		t.Fatalf("append multiline:\ngot:\n%s\nwant:\n%s", got, want)
	}
	in := "[\n  {\"a\": 1, \"b\": 2}\n]\n"
	want := "[\n  {\"a\": 9, \"b\": 2}\n]\n"
	if got := edit(t, in, setAt("/0/a", `9`)); got != want {
		t.Fatalf("nested element edit:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestEditCommentsAndBlankLinesSurviveByteForByte(t *testing.T) {
	in := "// file header\n\n{\n  /* block\n     comment */\n  \"a\": 1, // about a\n\n  \"b\": [2, 3,], // about b\n  \"c\": {},\n}\n"
	want := "// file header\n\n{\n  /* block\n     comment */\n  \"a\": 1, // about a\n\n  \"b\": [2, 3,], // about b\n  \"c\": {\"d\": 4},\n  \"e\": 5\n}\n"
	got := edit(t, in, setAt("/c/d", `4`), setAt("/e", `5`))
	if got != want {
		t.Fatalf("preservation:\ngot:\n%q\nwant:\n%q", got, want)
	}
}

func TestEditKeyOrderPreservedAndNewKeysAppended(t *testing.T) {
	in := "{\n  \"b\": 1,\n  \"a\": 2\n}\n"
	// new has a different key order: surviving keys keep the file order.
	want := "{\n  \"b\": 1,\n  \"a\": 2,\n  \"c\": 3\n}\n"
	old, err := Parse([]byte(in), false)
	if err != nil {
		t.Fatal(err)
	}
	next := doc.NewObject()
	next.Put("a", old.M["a"])
	next.Put("c", mustNum("3"))
	next.Put("b", old.M["b"])
	out, err := Edit([]byte(in), old, next)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != want {
		t.Fatalf("order:\ngot:\n%s\nwant:\n%s", out, want)
	}
}

func mustNum(text string) *doc.Value {
	v, err := doc.NewNumber(text)
	if err != nil {
		panic(err)
	}
	return v
}

func TestEditRefusals(t *testing.T) {
	// old does not match the file text.
	in := `{"a": 1}`
	other, _ := Parse([]byte(`{"a": 2}`), true)
	next := other.Clone()
	if _, err := Edit([]byte(in), other, next); !errors.Is(err, ErrMismatch) {
		t.Fatalf("mismatch: got %v", err)
	}
	// An interior array insertion is not representable as an end append.
	old, _ := Parse([]byte(`[1, 3]`), true)
	next3 := doc.NewArray(mustNum("1"), mustNum("2"), mustNum("3"))
	if _, err := Edit([]byte(`[1, 3]`), old, next3); !errors.Is(err, ErrUnrepresentable) {
		t.Fatalf("interior insert must refuse: got %v", err)
	}
}
