package schema

import (
	"fmt"
	"math/big"
	neturl "net/url"
	"strings"

	"github.com/CGuiho/jureuma/internal/doc"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
)

// translate converts a library error tree into Issues. Only the instance
// location, the keyword, and schema-side parameters are used: neither the
// library's message text nor any instance value is ever copied.
func translate(ve *jsonschema.ValidationError, prefix []string, raw *doc.Value) []Issue {
	var out []Issue
	var walk func(e *jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		loc := make([]string, 0, len(prefix)+len(e.InstanceLocation))
		loc = append(loc, prefix...)
		loc = append(loc, e.InstanceLocation...)
		ptr := doc.FormatPointer(loc)
		add := func(kw, msg string) { out = append(out, Issue{Pointer: ptr, Keyword: kw, Message: msg}) }
		node := schemaNode(raw, e.SchemaURL)
		switch k := e.ErrorKind.(type) {
		case *kind.Schema, *kind.Group, *kind.Reference, *kind.AllOf:
			if len(e.Causes) == 0 {
				add(keywordOf(e), "does not match the schema")
			}
		case *kind.AnyOf:
			add("anyOf", fmt.Sprintf("matches none of the %s alternatives (anyOf)", count(node, "anyOf", len(e.Causes))))
		case *kind.OneOf:
			if len(k.Subschemas) >= 2 {
				add("oneOf", fmt.Sprintf("matches more than one alternative (oneOf branches %d and %d)", k.Subschemas[0], k.Subschemas[1]))
			} else {
				add("oneOf", fmt.Sprintf("matches none of the %s alternatives (oneOf)", count(node, "oneOf", len(e.Causes))))
			}
		case *kind.Not:
			add("not", "must not match the schema under \"not\"")
		case *kind.FalseSchema:
			if n := len(e.InstanceLocation); n > 0 {
				add("false", fmt.Sprintf("property or item %q is not allowed here", e.InstanceLocation[n-1]))
			} else {
				add("false", "no value is allowed here")
			}
		case *kind.Type:
			add("type", "must be of type "+strings.Join(k.Want, " or "))
		case *kind.Enum:
			add("enum", fmt.Sprintf("must be one of the %d values allowed by enum", len(k.Want)))
		case *kind.Const:
			add("const", "must equal the value fixed by const")
		case *kind.Format:
			add("format", fmt.Sprintf("must be a valid %s (format)", k.Want))
		case *kind.MinProperties:
			add("minProperties", fmt.Sprintf("must have at least %d properties", k.Want))
		case *kind.MaxProperties:
			add("maxProperties", fmt.Sprintf("must have at most %d properties", k.Want))
		case *kind.MinItems:
			add("minItems", fmt.Sprintf("must have at least %d items", k.Want))
		case *kind.MaxItems:
			add("maxItems", fmt.Sprintf("must have at most %d items", k.Want))
		case *kind.AdditionalItems:
			add("additionalItems", "has more items than allowed")
		case *kind.Required:
			for _, m := range k.Missing {
				add("required", fmt.Sprintf("required property %q missing", m))
			}
		case *kind.Dependency:
			for _, m := range k.Missing {
				add("dependencies", fmt.Sprintf("property %q requires property %q", k.Prop, m))
			}
		case *kind.DependentRequired:
			for _, m := range k.Missing {
				add("dependentRequired", fmt.Sprintf("property %q requires property %q", k.Prop, m))
			}
		case *kind.AdditionalProperties:
			for _, p := range k.Properties {
				out = append(out, Issue{
					Pointer: doc.FormatPointer(append(append([]string(nil), loc...), p)),
					Keyword: "additionalProperties",
					Message: fmt.Sprintf("property %q is not allowed (additionalProperties)", p),
				})
			}
		case *kind.PropertyNames:
			add("propertyNames", fmt.Sprintf("property name %q does not satisfy propertyNames", k.Property))
			return // causes describe the key only; the summary is enough
		case *kind.UniqueItems:
			add("uniqueItems", fmt.Sprintf("items %d and %d are equal (uniqueItems)", k.Duplicates[0], k.Duplicates[1]))
		case *kind.Contains:
			add("contains", "no item matches the contains schema")
		case *kind.MinContains:
			add("minContains", fmt.Sprintf("at least %d items must match the contains schema", k.Want))
		case *kind.MaxContains:
			add("maxContains", fmt.Sprintf("at most %d items may match the contains schema", k.Want))
		case *kind.MinLength:
			add("minLength", fmt.Sprintf("must be at least %d characters long", k.Want))
		case *kind.MaxLength:
			add("maxLength", fmt.Sprintf("must be at most %d characters long", k.Want))
		case *kind.Pattern:
			add("pattern", fmt.Sprintf("must match pattern %q", k.Want))
		case *kind.ContentEncoding:
			add("contentEncoding", fmt.Sprintf("must be %s encoded", k.Want))
		case *kind.ContentMediaType:
			add("contentMediaType", fmt.Sprintf("must be of media type %s", k.Want))
		case *kind.ContentSchema:
			add("contentSchema", "content does not match contentSchema")
		case *kind.Minimum:
			add("minimum", "minimum "+limit(node, "minimum", k.Want))
		case *kind.Maximum:
			add("maximum", "maximum "+limit(node, "maximum", k.Want))
		case *kind.ExclusiveMinimum:
			add("exclusiveMinimum", "must be greater than "+limit(node, "exclusiveMinimum", k.Want))
		case *kind.ExclusiveMaximum:
			add("exclusiveMaximum", "must be less than "+limit(node, "exclusiveMaximum", k.Want))
		case *kind.MultipleOf:
			add("multipleOf", "must be a multiple of "+limit(node, "multipleOf", k.Want))
		case *kind.RefCycle:
			add("$ref", "reference cycle in schema")
		default:
			add(keywordOf(e), "does not satisfy keyword "+keywordOf(e))
		}
		for _, c := range e.Causes {
			walk(c)
		}
	}
	walk(ve)
	return out
}

func keywordOf(e *jsonschema.ValidationError) string {
	if e.ErrorKind == nil {
		return ""
	}
	p := e.ErrorKind.KeywordPath()
	if len(p) == 0 {
		return ""
	}
	return p[len(p)-1]
}

// schemaNode returns the raw schema object an error's SchemaURL points at.
func schemaNode(raw *doc.Value, schemaURL string) *doc.Value {
	if raw == nil {
		return nil
	}
	i := strings.IndexByte(schemaURL, '#')
	if i < 0 || !strings.HasPrefix(schemaURL, resourceURL) {
		return nil
	}
	frag, err := neturl.PathUnescape(schemaURL[i+1:])
	if err != nil {
		return nil
	}
	toks, err := doc.ParsePointer(frag)
	if err != nil {
		return nil
	}
	n, err := doc.Lookup(raw, toks)
	if err != nil || n.Kind != doc.Object {
		return nil
	}
	return n
}

// limit renders a numeric schema parameter, preferring its exact source text.
func limit(node *doc.Value, kw string, want *big.Rat) string {
	if node != nil {
		if v := node.Get(kw); v != nil && v.Kind == doc.Number {
			return v.N
		}
	}
	if want == nil {
		return "(schema limit)"
	}
	if want.IsInt() {
		return want.Num().String()
	}
	return want.FloatString(10)
}

func count(node *doc.Value, kw string, fallback int) string {
	if node != nil {
		if v := node.Get(kw); v != nil && v.Kind == doc.Array {
			return fmt.Sprint(len(v.A))
		}
	}
	return fmt.Sprint(fallback)
}

func urlUnescape(s string) (string, error) { return neturl.PathUnescape(s) }
