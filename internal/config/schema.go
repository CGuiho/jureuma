package config

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strings"

	"github.com/CGuiho/jureuma/internal/doc"
	"github.com/CGuiho/jureuma/internal/xerr"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
)

//go:embed schemas/jureuma.schema.json schemas/jureuma.global.schema.json
var schemaFS embed.FS

//go:embed examples/jureuma.yaml examples/jureuma.global.yaml
var exampleFS embed.FS

func mustRead(fs embed.FS, name string) []byte {
	b, err := fs.ReadFile(name)
	if err != nil {
		panic("config: missing embedded file " + name)
	}
	return b
}

var (
	projectSchemaBytes = mustRead(schemaFS, "schemas/"+ProjectSchemaFile)
	globalSchemaBytes  = mustRead(schemaFS, "schemas/"+GlobalSchemaFile)

	projectSchema = mustCompile(ProjectSchemaFile, projectSchemaBytes)
	globalSchema  = mustCompile(GlobalSchemaFile, globalSchemaBytes)
)

// ProjectSchemaJSON returns the embedded JSON Schema for jureuma.yaml.
func ProjectSchemaJSON() []byte { return bytes.Clone(projectSchemaBytes) }

// GlobalSchemaJSON returns the embedded JSON Schema for jureuma.global.yaml.
func GlobalSchemaJSON() []byte { return bytes.Clone(globalSchemaBytes) }

// ProjectExampleYAML returns the complete example jureuma.yaml.
func ProjectExampleYAML() []byte { return mustRead(exampleFS, "examples/"+ProjectFile) }

// GlobalExampleYAML returns the complete example jureuma.global.yaml.
func GlobalExampleYAML() []byte { return mustRead(exampleFS, "examples/"+GlobalFile) }

func mustCompile(name string, data []byte) *jsonschema.Schema {
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		panic("config: embedded schema " + name + " is not JSON")
	}
	url := "https://github.com/CGuiho/jureuma/schemas/" + name
	c := jsonschema.NewCompiler()
	if err := c.AddResource(url, v); err != nil {
		panic("config: embedded schema " + name + ": " + err.Error())
	}
	s, err := c.Compile(url)
	if err != nil {
		panic("config: embedded schema " + name + ": " + err.Error())
	}
	return s
}

// validateSchema validates a generic YAML value against an embedded schema.
// Errors report only instance locations (known keys and indices) and the
// failing keyword, never values or library message text.
func validateSchema(s *jsonschema.Schema, raw any, file string) error {
	if !jsonRepresentable(raw) {
		return xerr.Validation("%s: contains a value that is not representable as JSON (non-string key or non-finite number)", file)
	}
	js, err := json.Marshal(raw)
	if err != nil {
		return xerr.Validation("%s: contains a value that is not representable as JSON (non-string key or non-finite number)", file)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(js))
	if err != nil {
		return xerr.Validation("%s: contains a value that is not representable as JSON", file)
	}
	err = s.Validate(inst)
	if err == nil {
		return nil
	}
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return xerr.Validation("%s: schema validation failed", file)
	}
	seen := map[string]bool{}
	var msgs []string
	collectLeaves(ve, func(e *jsonschema.ValidationError) {
		ptr := doc.FormatPointer(e.InstanceLocation)
		if ptr == "" {
			ptr = "(root)"
		}
		kp := e.ErrorKind.KeywordPath()
		kw := ""
		if len(kp) > 0 {
			kw = kp[len(kp)-1]
		} else if _, ok := e.ErrorKind.(*kind.Not); ok {
			kw = "not"
		}
		m := ptr + ": " + keywordReason(kw)
		if !seen[m] {
			seen[m] = true
			msgs = append(msgs, m)
		}
	})
	if len(msgs) == 0 {
		return xerr.Validation("%s: schema validation failed", file)
	}
	sort.Strings(msgs)
	return xerr.Validation("%s: %s", file, strings.Join(msgs, "; "))
}

// jsonRepresentable reports whether v survives a JSON round-trip: every map
// key is a string and every number is finite. YAML decoding can produce
// map[any]any with non-string keys (for example the integer key `1`), which
// encoding/json would silently turn into the object key "1"; configuration
// files reject those instead.
func jsonRepresentable(v any) bool {
	switch t := v.(type) {
	case map[string]any:
		for _, x := range t {
			if !jsonRepresentable(x) {
				return false
			}
		}
		return true
	case map[any]any:
		for k, x := range t {
			if _, ok := k.(string); !ok {
				return false
			}
			if !jsonRepresentable(x) {
				return false
			}
		}
		return true
	case []any:
		for _, x := range t {
			if !jsonRepresentable(x) {
				return false
			}
		}
		return true
	case float64:
		return !math.IsNaN(t) && !math.IsInf(t, 0)
	}
	return true
}

func collectLeaves(e *jsonschema.ValidationError, f func(*jsonschema.ValidationError)) {
	if len(e.Causes) == 0 {
		f(e)
		return
	}
	for _, c := range e.Causes {
		collectLeaves(c, f)
	}
}

func keywordReason(kw string) string {
	switch kw {
	case "additionalProperties", "false":
		return "unknown field"
	case "enum":
		return "value not allowed"
	case "type":
		return "wrong type"
	case "pattern":
		return "does not match the required pattern"
	case "required":
		return "missing required field"
	case "minLength":
		return "must not be empty"
	case "not":
		return "not allowed (must be a relative path without .. segments)"
	case "":
		return "invalid"
	}
	return "violates " + kw
}
