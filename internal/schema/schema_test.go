package schema

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/CGuiho/zeze/internal/doc"
	"github.com/CGuiho/zeze/internal/jsonc"
)

// sentinel is a fake secret; it must never appear in any Issue or error.
const sentinel = "S3NTINEL-do-not-leak-7f3a9c"

func parse(t *testing.T, s string) *doc.Value {
	t.Helper()
	v, err := jsonc.Parse([]byte(s), false)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return v
}

func compile(t *testing.T, s string) *Schema {
	t.Helper()
	sch, err := Compile(parse(t, s), "test.schema.json")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return sch
}

func noLeak(t *testing.T, issues []Issue) {
	t.Helper()
	for _, is := range issues {
		for _, f := range []string{is.Pointer, is.Keyword, is.Message} {
			if strings.Contains(f, sentinel) || strings.Contains(f, "do-not-leak") {
				t.Fatalf("issue leaks the sentinel value: %+v", is)
			}
		}
	}
}

func has(issues []Issue, ptr, kw, msgPart string) bool {
	for _, is := range issues {
		if is.Pointer == ptr && is.Keyword == kw && strings.Contains(is.Message, msgPart) {
			return true
		}
	}
	return false
}

const appSchema = `{
  // comment allowed in jsonc
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "database": {
      "type": "object",
      "properties": {
        "url": {"type": "string", "format": "uri", "x-zeze-secret": true},
        "pool": {"type": "integer", "minimum": 1, "maximum": 100},
        "ratio": {"type": "number", "exclusiveMaximum": 0.5}
      },
      "required": ["url"],
      "additionalProperties": false
    },
    "mode": {"enum": ["dev", "prod"]},
    "token": {"type": "string", "pattern": "^tok_", "minLength": 50, "x-zeze-secret": true},
    "email": {"type": "string", "format": "email"},
    "tags": {"type": "array", "items": {"type": "string"}, "maxItems": 2},
    "nullable": {"anyOf": [{"type": "string", "maxLength": 3}, {"type": "null"}]},
    "ref": {"$ref": "#/$defs/port"},
    "creds": {"type": "object", "x-zeze-secret": true, "properties": {"user": {"type": "string"}}}
  },
  "required": ["database"],
  "additionalProperties": false,
  "$defs": {"port": {"type": "integer", "minimum": 1, "maximum": 65535}}
}`

func TestValidateIssues(t *testing.T) {
	s := compile(t, appSchema)
	tests := []struct {
		name, doc     string
		ptr, kw, part string
	}{
		{"ok", `{"database": {"url": "https://x.example/db"}}`, "", "", ""},
		{"maximum", `{"database": {"url": "https://x", "pool": 1000}}`, "/database/pool", "maximum", "maximum 100"},
		{"exclusiveMaximum exact text", `{"database": {"url": "https://x", "ratio": 0.9}}`, "/database/ratio", "exclusiveMaximum", "less than 0.5"},
		{"required", `{"database": {}}`, "/database", "required", `required property "url" missing`},
		{"additional", `{"database": {"url": "https://x"}, "extra": "` + sentinel + `"}`, "/extra", "additionalProperties", `property "extra" is not allowed`},
		{"type", `{"database": {"url": "https://x", "pool": "` + sentinel + `"}}`, "/database/pool", "type", "must be of type integer"},
		{"format uri", `{"database": {"url": "` + sentinel + `"}}`, "/database/url", "format", "valid uri"},
		{"format email", `{"database": {"url": "https://x"}, "email": "` + sentinel + `"}`, "/email", "format", "valid email"},
		{"enum", `{"database": {"url": "https://x"}, "mode": "` + sentinel + `"}`, "/mode", "enum", "one of the 2 values"},
		{"pattern", `{"database": {"url": "https://x"}, "token": "` + sentinel + `"}`, "/token", "pattern", `"^tok_"`},
		{"minLength", `{"database": {"url": "https://x"}, "token": "tok_` + sentinel + `"}`, "/token", "minLength", "at least 50"},
		{"maxItems", `{"database": {"url": "https://x"}, "tags": ["a", "b", "c"]}`, "/tags", "maxItems", "at most 2"},
		{"items type", `{"database": {"url": "https://x"}, "tags": [1]}`, "/tags/0", "type", "string"},
		{"anyOf", `{"database": {"url": "https://x"}, "nullable": "` + sentinel + `"}`, "/nullable", "anyOf", "none of the 2"},
		{"ref", `{"database": {"url": "https://x"}, "ref": 70000}`, "/ref", "maximum", "maximum 65535"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := s.Validate(parse(t, tt.doc))
			noLeak(t, issues)
			if tt.kw == "" {
				if len(issues) != 0 {
					t.Fatalf("unexpected issues: %v", issues)
				}
				return
			}
			if !has(issues, tt.ptr, tt.kw, tt.part) {
				t.Fatalf("missing issue %s %s %q in %v", tt.ptr, tt.kw, tt.part, issues)
			}
		})
	}
}

func TestCompileErrors(t *testing.T) {
	tests := []struct{ name, schema, part string }{
		{"unknown keyword", `{"type": "object", "propertys": {}}`, `unknown keyword "propertys"`},
		{"unknown nested", `{"properties": {"a": {"maxLenght": 3}}}`, `"maxLenght"`},
		{"unknown format", `{"type": "string", "format": "duration"}`, `unsupported format "duration"`},
		{"definitions", `{"definitions": {}}`, `use "$defs"`},
		{"draft 7", `{"$schema": "http://json-schema.org/draft-07/schema#"}`, "unsupported $schema"},
		{"remote ref", `{"$ref": "https://example.com/s.json"}`, "only local references"},
		{"bad secret", `{"x-zeze-secret": "yes"}`, "must be a boolean"},
		{"meta invalid", `{"type": "strin"}`, "not a valid draft 2020-12 schema"},
		{"bad ref target", `{"$ref": "#/$defs/missing"}`, "$ref"},
		{"not schema", `{"properties": {"a": 3}}`, "must be an object or a boolean"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Compile(parse(t, tt.schema), "x.schema.json")
			if err == nil || !strings.Contains(err.Error(), tt.part) {
				t.Fatalf("got %v, want error containing %q", err, tt.part)
			}
		})
	}
	ok := []string{
		`true`, `{}`, `{"x-anything": {"z": 1}, "type": "string", "format": "uuid"}`,
		`{"$schema": "https://json-schema.org/draft/2020-12/schema#"}`,
		`{"$defs": {"a": {"$anchor": "foo", "type": "string"}}, "properties": {"b": {"$ref": "#foo"}}}`,
		`{"if": {"properties": {"a": {"const": 1}}}, "then": {"required": ["b"]}, "else": {}}`,
	}
	for _, s := range ok {
		if _, err := Compile(parse(t, s), "x"); err != nil {
			t.Errorf("Compile(%s): %v", s, err)
		}
	}
}

func TestFormats(t *testing.T) {
	for _, f := range AssertedFormats {
		s := compile(t, `{"type": "string", "format": "`+f+`"}`)
		issues := s.Validate(doc.NewString(sentinel))
		noLeak(t, issues)
		if f == "uri" || f == "hostname" {
			// sentinel is a syntactically valid hostname; skip positive check
			if f == "uri" && len(issues) == 0 {
				t.Errorf("format %s not asserted", f)
			}
			continue
		}
		if len(issues) == 0 {
			t.Errorf("format %s not asserted", f)
		}
	}
	s := compile(t, `{"type": "string", "format": "uuid"}`)
	if is := s.Validate(doc.NewString("123e4567-e89b-12d3-a456-426614174000")); len(is) != 0 {
		t.Fatalf("valid uuid rejected: %v", is)
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	jc := filepath.Join(dir, "config.schema.jsonc")
	if err := os.WriteFile(jc, []byte(appSchema), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(jc); err != nil {
		t.Fatalf("Load jsonc: %v", err)
	}
	j := filepath.Join(dir, "config.schema.json")
	if err := os.WriteFile(j, []byte(appSchema), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(j); err == nil {
		t.Fatal("strict .json accepted comments")
	}
	if err := os.WriteFile(j, []byte(`{"type": "object"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(j); err != nil {
		t.Fatalf("Load json: %v", err)
	}
	if _, err := Load(filepath.Join(dir, "x.schema.yaml")); err == nil {
		t.Fatal("yaml accepted")
	}
}

func TestValidateValue(t *testing.T) {
	s := compile(t, appSchema)
	tests := []struct {
		name, ptr, val string
		wantIssue      string // keyword, "" = accept
		unknown        bool
	}{
		{"ok pool", "/database/pool", `20`, "", false},
		{"pool max", "/database/pool", `1000`, "maximum", false},
		{"pool string", "/database/pool", `"` + sentinel + `"`, "type", false},
		{"url format", "/database/url", `"` + sentinel + `"`, "format", false},
		{"unknown prop", "/database/nope", `"` + sentinel + `"`, "additionalProperties", false},
		{"top unknown", "/nope", `1`, "additionalProperties", false},
		{"nullable null", "/nullable", `null`, "", false},
		{"nullable long", "/nullable", `"` + sentinel + `"`, "anyOf", false},
		{"ref", "/ref", `70000`, "maximum", false},
		{"items", "/tags/0", `"a"`, "", false},
		{"items append", "/tags/-", `3`, "type", false},
		{"object value", "/database", `{"url": "https://x", "pool": 0}`, "minimum", false},
		{"no schema", "/creds/other", `"x"`, "", true},
		{"through leaf type", "/token/x", `"x"`, "type", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues, unknown := s.ValidateValueUnknown(tt.ptr, parse(t, tt.val))
			noLeak(t, issues)
			if tt.wantIssue == "" && len(issues) > 0 {
				t.Fatalf("unexpected: %v", issues)
			}
			if tt.wantIssue != "" {
				found := false
				for _, is := range issues {
					if is.Keyword == tt.wantIssue {
						found = true
					}
				}
				if !found {
					t.Fatalf("want %s issue, got %v", tt.wantIssue, issues)
				}
			}
			if unknown != tt.unknown {
				t.Fatalf("unknown = %v, want %v", unknown, tt.unknown)
			}
			if !reflect.DeepEqual(issues, s.ValidateValue(tt.ptr, parse(t, tt.val))) {
				t.Fatal("ValidateValue differs from ValidateValueUnknown")
			}
		})
	}
}

func TestValidateValueBranches(t *testing.T) {
	s := compile(t, `{
	  "oneOf": [
	    {"properties": {"kind": {"const": "a"}, "port": {"type": "integer", "maximum": 10}}},
	    {"properties": {"kind": {"const": "b"}, "port": {"type": "string"}}}
	  ],
	  "allOf": [{"properties": {"port": {"not": {"const": 5}}}}],
	  "if": {"properties": {"kind": {"const": "a"}}},
	  "then": {"properties": {"extra": {"type": "boolean"}}},
	  "else": {"properties": {"extra": {"type": "null"}}}
	}`)
	accept := map[string]string{"/port": `7`, "/extra": `true`}
	for p, v := range accept {
		if is := s.ValidateValue(p, parse(t, v)); len(is) != 0 {
			t.Errorf("%s=%s rejected: %v", p, v, is)
		}
	}
	if is := s.ValidateValue("/port", parse(t, `"x"`)); len(is) != 0 {
		t.Errorf("string port should be accepted by branch b: %v", is)
	}
	reject := map[string]string{"/port": `5`, "/extra": `1`}
	for p, v := range reject {
		if is := s.ValidateValue(p, parse(t, v)); len(is) == 0 {
			t.Errorf("%s=%s accepted", p, v)
		}
	}
	if is := s.ValidateValue("/port", parse(t, `50`)); len(is) == 0 {
		t.Error("50 accepted although integer branch max is 10 and other branch wants string")
	}
}

func TestValidateStructure(t *testing.T) {
	s := compile(t, appSchema)
	ct := `"zeze:1:k_00:` + sentinel + `"`
	tests := []struct {
		name, doc, ptr, kw string
	}{
		{"ok", `{"database": {"url": ` + ct + `, "pool": ` + ct + `}}`, "", ""},
		{"leaf ignores type and format", `{"database": {"url": ` + ct + `}, "ref": ` + ct + `}`, "", ""},
		{"required", `{"database": {"pool": ` + ct + `}}`, "/database", "required"},
		{"root required", `{}`, "", "required"},
		{"additional", `{"database": {"url": ` + ct + `, "x": ` + ct + `}}`, "/database/x", "additionalProperties"},
		{"maxItems", `{"database": {"url": ` + ct + `}, "tags": [` + ct + `,` + ct + `,` + ct + `]}`, "/tags", "maxItems"},
		{"container for scalar", `{"database": {"url": {"a": ` + ct + `}}}`, "/database/url", "type"},
		{"scalar for container", `{"database": ` + ct + `}`, "/database", "type"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := s.ValidateStructure(parse(t, tt.doc))
			noLeak(t, issues)
			if tt.kw == "" {
				if len(issues) != 0 {
					t.Fatalf("unexpected: %v", issues)
				}
				return
			}
			if !has(issues, tt.ptr, tt.kw, "") {
				t.Fatalf("want %s %s in %v", tt.ptr, tt.kw, issues)
			}
		})
	}
	s2 := compile(t, `{"propertyNames": {"pattern": "^[a-z]+$"}, "minProperties": 2, "unevaluatedProperties": false,
	  "properties": {"a": true}, "allOf": [{"properties": {"b": true}}], "dependentRequired": {"a": ["b"]}}`)
	issues := s2.ValidateStructure(parse(t, `{"a": 1, "B": 2}`))
	for _, want := range [][2]string{{"/B", "propertyNames"}, {"/B", "unevaluatedProperties"}, {"", "dependentRequired"}} {
		if !has(issues, want[0], want[1], "") {
			t.Errorf("want %v in %v", want, issues)
		}
	}
	if is := s2.ValidateStructure(parse(t, `{"a": 1, "b": 2}`)); len(is) != 0 {
		t.Errorf("unexpected: %v", is)
	}
	if is := s2.ValidateStructure(parse(t, `{"b": 2}`)); !has(is, "", "minProperties", "at least 2") {
		t.Errorf("want minProperties in %v", is)
	}
}

func TestExpectedTypes(t *testing.T) {
	s := compile(t, appSchema+``)
	tests := map[string][]string{
		"/database/pool": {"integer"},
		"/database/url":  {"string"},
		"/database":      {"object"},
		"/nullable":      {"string", "null"},
		"/ref":           {"integer"},
		"/tags/3":        {"string"},
		"/mode":          nil,
		"/creds/other":   nil,
		"":               {"object"},
	}
	for p, want := range tests {
		if got := s.ExpectedTypes(p); !reflect.DeepEqual(got, want) {
			t.Errorf("ExpectedTypes(%q) = %v, want %v", p, got, want)
		}
	}
	s2 := compile(t, `{"properties": {"n": {"type": "number", "allOf": [{"type": ["integer", "string"]}]}}}`)
	if got := s2.ExpectedTypes("/n"); !reflect.DeepEqual(got, []string{"integer"}) {
		t.Errorf("intersection = %v", got)
	}
}

func TestSecret(t *testing.T) {
	s := compile(t, appSchema)
	tests := map[string]bool{
		"/database/url":  true,
		"/database/pool": false,
		"/token":         true,
		"/creds":         true,
		"/creds/user":    true, // ancestor marked
		"/mode":          false,
		"":               false,
	}
	for p, want := range tests {
		if got := s.Secret(p); got != want {
			t.Errorf("Secret(%q) = %v, want %v", p, got, want)
		}
	}
	s2 := compile(t, `{"$defs": {"sec": {"type": "string", "x-zeze-secret": true, "default": "x"}},
	  "properties": {"a": {"$ref": "#/$defs/sec"}, "b": {"anyOf": [{"$ref": "#/$defs/sec"}, {"type": "null"}]}}}`)
	if !s2.Secret("/a") || !s2.Secret("/b") {
		t.Error("secret through $ref/anyOf not detected")
	}
	w := s2.SecretValueKeywords()
	if len(w) != 1 || w[0].Keyword != "default" || w[0].Pointer != "/$defs/sec" {
		t.Errorf("SecretValueKeywords = %v", w)
	}
}

func TestInfer(t *testing.T) {
	v := parse(t, `{"db": {"url": "`+sentinel+`", "port": 5432, "tls": true, "n": null},
	  "hosts": ["a", "b"], "mixed": [1, "x"], "empty": [], "objs": [{"a": 1}, {"a": 2}]}`)
	got := Infer(v, true)
	if strings.Contains(string(doc.Marshal(got)), sentinel) {
		t.Fatal("Infer leaked a value")
	}
	want := parse(t, `{
	  "$schema": "https://json-schema.org/draft/2020-12/schema",
	  "type": "object",
	  "properties": {
	    "db": {"type": "object", "properties": {"url": {"type": "string"}, "port": {"type": "number"},
	      "tls": {"type": "boolean"}, "n": {"type": "null"}}, "required": ["url", "port", "tls", "n"], "additionalProperties": false},
	    "hosts": {"type": "array", "items": {"type": "string"}},
	    "mixed": {"type": "array", "prefixItems": [{"type": "number"}, {"type": "string"}]},
	    "empty": {"type": "array", "items": {}},
	    "objs": {"type": "array", "items": {"type": "object", "properties": {"a": {"type": "number"}}, "required": ["a"], "additionalProperties": false}}
	  },
	  "required": ["db", "hosts", "mixed", "empty", "objs"],
	  "additionalProperties": false
	}`)
	if !doc.Equal(got, want) {
		t.Fatalf("Infer =\n%s", doc.Marshal(got))
	}
	if got.Keys[0] != "$schema" {
		t.Fatal("$schema should be first")
	}
	sch, err := Compile(got, "inferred")
	if err != nil {
		t.Fatalf("inferred schema does not compile: %v", err)
	}
	if is := sch.Validate(v); len(is) != 0 {
		t.Fatalf("source does not validate against inferred schema: %v", is)
	}
	untyped := Infer(parse(t, `{"a": "zeze:1:k:x", "b": ["c1", "c2"]}`), false)
	wantU := parse(t, `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
	  "properties": {"a": {}, "b": {"type": "array", "items": {}}}, "required": ["a", "b"], "additionalProperties": false}`)
	if !doc.Equal(untyped, wantU) {
		t.Fatalf("Infer untyped =\n%s", doc.Marshal(untyped))
	}
	if _, err := Compile(untyped, "x"); err != nil {
		t.Fatal(err)
	}
}

func TestStarter(t *testing.T) {
	st := Starter()
	if !IsStarter(st) {
		t.Fatal("Starter is not a starter")
	}
	if _, err := Compile(st, "starter"); err != nil {
		t.Fatal(err)
	}
	if !IsStarter(parse(t, `{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema#"}`)) {
		t.Fatal("equivalent starter not recognised")
	}
	for _, s := range []string{`{"type": "object", "properties": {}}`, `{}`, `true`, `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "array"}`} {
		if IsStarter(parse(t, s)) {
			t.Errorf("IsStarter(%s) = true", s)
		}
	}
}

func TestPointerEscapes(t *testing.T) {
	s := compile(t, `{"properties": {"a/b c%": {"type": "object", "properties": {"x~y": {"type": "integer", "maximum": 3}}}}}`)
	is := s.ValidateValue("/a~1b c%/x~0y", parse(t, `9`))
	if !has(is, "/a~1b c%/x~0y", "maximum", "maximum 3") {
		t.Fatalf("got %v", is)
	}
	is = s.Validate(parse(t, `{"a/b c%": {"x~y": 9}}`))
	if !has(is, "/a~1b c%/x~0y", "maximum", "maximum 3") {
		t.Fatalf("got %v", is)
	}
}
