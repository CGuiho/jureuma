// Package schema loads, checks, and applies jureuma's JSON Schemas (draft
// 2020-12, stored as <stem>.schema.json or <stem>.schema.jsonc).
//
// Full validation is delegated to santhosh-tekuri/jsonschema/v6. Every
// validation failure is translated into an Issue whose Message is built only
// from schema-side parameters (keyword names, limits, property names); the
// library's own message text and instance values never reach an Issue.
package schema

import (
	"errors"
	"fmt"
	neturl "net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/CGuiho/jureuma/internal/doc"
	"github.com/CGuiho/jureuma/internal/jsonc"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

// Draft202012 is the only supported meta-schema URI.
const Draft202012 = "https://json-schema.org/draft/2020-12/schema"

// SecretKeyword is the jureuma annotation that marks a location as secret.
const SecretKeyword = "x-jureuma-secret"

// resourceURL is the internal retrieval URL of every compiled schema. Using a
// fixed URL keeps file-system paths out of compilation and makes fragment
// lookups independent of the platform.
const resourceURL = "file:///jureuma/schema.json"

// AssertedFormats lists the formats jureuma asserts; any other format value is
// a schema error.
var AssertedFormats = []string{"date-time", "date", "time", "email", "hostname", "ipv4", "ipv6", "uri", "uuid"}

// Issue is one validation problem. Message is built from schema-side
// parameters only and never contains an instance value.
type Issue struct {
	Pointer string
	Keyword string
	Message string
}

// String renders the issue as "<pointer>: <message>" ("(root)" for "").
func (i Issue) String() string {
	p := i.Pointer
	if p == "" {
		p = "(root)"
	}
	return p + ": " + i.Message
}

// Schema is a compiled validator plus the raw schema tree.
type Schema struct {
	raw      *doc.Value
	location string
	compiler *jsonschema.Compiler
	root     *jsonschema.Schema
	mu       sync.Mutex // guards frags and compiler
	frags    map[string]*jsonschema.Schema
	anchors  map[string]string // $anchor name -> schema pointer
}

// Raw returns the raw schema tree. Callers must not modify it.
func (s *Schema) Raw() *doc.Value { return s.raw }

// Location returns the location the schema was compiled from.
func (s *Schema) Location() string { return s.location }

// Load reads a .json or .jsonc schema file (draft 2020-12 assumed if $schema
// is absent) and compiles it.
func Load(path string) (*Schema, error) {
	ext := strings.ToLower(filepath.Ext(path))
	var strict bool
	switch ext {
	case ".json":
		strict = true
	case ".jsonc":
		strict = false
	default:
		return nil, fmt.Errorf("schema %s: unsupported extension %q (use .json or .jsonc)", path, ext)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read schema: %w", err)
	}
	raw, err := jsonc.Parse(data, strict)
	if err != nil {
		// jsonc.SyntaxError never includes source text.
		return nil, fmt.Errorf("schema %s: %w", path, err)
	}
	return Compile(raw, path)
}

// Compile checks and compiles a raw schema tree. location is used in error
// messages only.
func Compile(raw *doc.Value, location string) (*Schema, error) {
	if raw == nil {
		return nil, fmt.Errorf("schema %s: empty schema", location)
	}
	anchors := map[string]string{}
	if err := checkSchema(raw, "", true, anchors); err != nil {
		return nil, fmt.Errorf("schema %s: %w", location, err)
	}
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	c.AssertFormat()
	c.UseLoader(noLoader{})
	if err := c.AddResource(resourceURL, doc.ToAny(raw)); err != nil {
		return nil, fmt.Errorf("schema %s: %s", location, describeCompileError(err))
	}
	root, err := c.Compile(resourceURL)
	if err != nil {
		return nil, fmt.Errorf("schema %s: %s", location, describeCompileError(err))
	}
	return &Schema{
		raw:      raw,
		location: location,
		compiler: c,
		root:     root,
		frags:    map[string]*jsonschema.Schema{"": root},
		anchors:  anchors,
	}, nil
}

// noLoader refuses every external reference: jureuma validates offline and
// only supports references inside the schema document.
type noLoader struct{}

func (noLoader) Load(string) (any, error) {
	return nil, errors.New("external schema references are not supported")
}

// describeCompileError turns a library compile error into a message that
// names only schema locations.
func describeCompileError(err error) string {
	var sve *jsonschema.SchemaValidationError
	if errors.As(err, &sve) {
		var ve *jsonschema.ValidationError
		if errors.As(sve.Err, &ve) {
			issues := translate(ve, nil, nil)
			if len(issues) > 3 {
				issues = issues[:3]
			}
			if len(issues) > 0 {
				parts := make([]string, len(issues))
				for i, is := range issues {
					parts[i] = is.String()
				}
				return "not a valid draft 2020-12 schema: " + strings.Join(parts, "; ")
			}
		}
		return "not a valid draft 2020-12 schema"
	}
	var rx *jsonschema.InvalidRegexError
	if errors.As(err, &rx) {
		return "invalid regular expression in schema"
	}
	var le *jsonschema.LoadURLError
	if errors.As(err, &le) {
		return "external schema references are not supported"
	}
	var anf *jsonschema.AnchorNotFoundError
	if errors.As(err, &anf) {
		return "unresolvable $ref (anchor not found)"
	}
	var pnf *jsonschema.JSONPointerNotFoundError
	if errors.As(err, &pnf) {
		return "unresolvable $ref (JSON Pointer not found)"
	}
	return fmt.Sprintf("cannot compile schema (%T)", err)
}

// fragment compiles (and caches) the subschema at schema pointer ptr.
func (s *Schema) fragment(ptr string) (*jsonschema.Schema, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sch, ok := s.frags[ptr]; ok {
		return sch, nil
	}
	toks, err := doc.ParsePointer(ptr)
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	for _, t := range toks {
		b.WriteByte('/')
		b.WriteString(neturl.PathEscape(doc.EscapeToken(t)))
	}
	sch, err := s.compiler.Compile(resourceURL + "#" + b.String())
	if err != nil {
		return nil, errors.New(describeCompileError(err))
	}
	s.frags[ptr] = sch
	return sch, nil
}

// Validate validates a whole document. Issues never contain instance values.
func (s *Schema) Validate(v *doc.Value) []Issue {
	return s.validateWith(s.root, v, nil)
}

func (s *Schema) validateWith(sch *jsonschema.Schema, v *doc.Value, prefix []string) []Issue {
	err := sch.Validate(doc.ToAny(v))
	if err == nil {
		return nil
	}
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return []Issue{{Pointer: doc.FormatPointer(prefix), Keyword: "", Message: "validation could not be performed"}}
	}
	return normalize(translate(ve, prefix, s.raw))
}

// normalize sorts and de-duplicates issues so output is deterministic.
func normalize(in []Issue) []Issue {
	if len(in) == 0 {
		return nil
	}
	sort.SliceStable(in, func(i, j int) bool {
		if in[i].Pointer != in[j].Pointer {
			return in[i].Pointer < in[j].Pointer
		}
		if in[i].Keyword != in[j].Keyword {
			return in[i].Keyword < in[j].Keyword
		}
		return in[i].Message < in[j].Message
	})
	out := in[:0]
	for i, is := range in {
		if i > 0 && is == out[len(out)-1] {
			continue
		}
		out = append(out, is)
	}
	return out
}

// Starter returns {"$schema": "...2020-12/schema", "type": "object"}.
func Starter() *doc.Value {
	o := doc.NewObject()
	o.Put("$schema", doc.NewString(Draft202012))
	o.Put("type", doc.NewString("object"))
	return o
}

// IsStarter reports whether raw is (still) the permissive starter schema.
func IsStarter(raw *doc.Value) bool {
	if raw == nil || raw.Kind != doc.Object {
		return false
	}
	if len(raw.Keys) == 1 {
		// A starter without $schema is equivalent.
		t := raw.Get("type")
		return t != nil && t.Kind == doc.String && t.S == "object"
	}
	c := raw.Clone()
	if sv := c.Get("$schema"); sv != nil && sv.Kind == doc.String && strings.TrimSuffix(sv.S, "#") == Draft202012 {
		c.Put("$schema", doc.NewString(Draft202012))
	}
	return doc.Equal(c, Starter())
}
