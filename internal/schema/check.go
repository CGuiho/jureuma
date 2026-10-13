package schema

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/CGuiho/jureuma/internal/doc"
)

// keyword classes of the supported draft 2020-12 subset.
var (
	// keywords whose value is a single subschema.
	kwSchema = map[string]bool{
		"not": true, "if": true, "then": true, "else": true, "items": true,
		"contains": true, "additionalProperties": true, "propertyNames": true,
		"unevaluatedItems": true, "unevaluatedProperties": true, "contentSchema": true,
	}
	// keywords whose value is an array of subschemas.
	kwSchemaArray = map[string]bool{"allOf": true, "anyOf": true, "oneOf": true, "prefixItems": true}
	// keywords whose value is an object of subschemas.
	kwSchemaMap = map[string]bool{"$defs": true, "properties": true, "patternProperties": true, "dependentSchemas": true}
	// keywords that carry no subschema.
	kwPlain = map[string]bool{
		"$schema": true, "$id": true, "$ref": true, "$anchor": true, "$dynamicRef": true,
		"$dynamicAnchor": true, "$comment": true, "$vocabulary": true,
		"type": true, "const": true, "enum": true,
		"multipleOf": true, "maximum": true, "exclusiveMaximum": true, "minimum": true, "exclusiveMinimum": true,
		"maxLength": true, "minLength": true, "pattern": true,
		"maxItems": true, "minItems": true, "uniqueItems": true, "maxContains": true, "minContains": true,
		"maxProperties": true, "minProperties": true, "required": true, "dependentRequired": true,
		"format": true,
		"title":  true, "description": true, "default": true, "deprecated": true, "readOnly": true,
		"writeOnly": true, "examples": true,
		"contentEncoding": true, "contentMediaType": true,
	}
	// hints for keywords from older drafts.
	kwHints = map[string]string{
		"definitions":      "use \"$defs\"",
		"dependencies":     "use \"dependentRequired\" or \"dependentSchemas\"",
		"additionalItems":  "use \"items\" with \"prefixItems\"",
		"$recursiveRef":    "use \"$dynamicRef\"",
		"$recursiveAnchor": "use \"$dynamicAnchor\"",
	}
)

// checkSchema enforces the jureuma subset: only draft 2020-12, no unknown
// keywords except x-*, only asserted formats, only local references.
// Errors name schema locations and keywords, never values.
func checkSchema(n *doc.Value, ptr string, isRoot bool, anchors map[string]string) error {
	switch n.Kind {
	case doc.Bool:
		return nil
	case doc.Object:
	default:
		return fmt.Errorf("%s: a schema must be an object or a boolean", where(ptr))
	}
	for _, k := range n.Keys {
		v := n.M[k]
		kp := ptr + "/" + doc.EscapeToken(k)
		switch {
		case strings.HasPrefix(k, "x-"):
			if k == SecretKeyword && v.Kind != doc.Bool {
				return fmt.Errorf("%s: %q must be a boolean", where(kp), SecretKeyword)
			}
		case kwSchema[k]:
			if err := checkSchema(v, kp, false, anchors); err != nil {
				return err
			}
		case kwSchemaArray[k]:
			if v.Kind != doc.Array {
				return fmt.Errorf("%s: %q must be an array of schemas", where(kp), k)
			}
			for i, e := range v.A {
				if err := checkSchema(e, kp+"/"+strconv.Itoa(i), false, anchors); err != nil {
					return err
				}
			}
		case kwSchemaMap[k]:
			if v.Kind != doc.Object {
				return fmt.Errorf("%s: %q must be an object of schemas", where(kp), k)
			}
			for _, name := range v.Keys {
				if err := checkSchema(v.M[name], kp+"/"+doc.EscapeToken(name), false, anchors); err != nil {
					return err
				}
			}
		case kwPlain[k]:
			if err := checkPlain(k, v, kp, isRoot, ptr, anchors); err != nil {
				return err
			}
		default:
			if h, ok := kwHints[k]; ok {
				return fmt.Errorf("%s: keyword %q is not supported in draft 2020-12; %s", where(kp), k, h)
			}
			return fmt.Errorf("%s: unknown keyword %q (custom annotations must start with \"x-\")", where(kp), k)
		}
	}
	return nil
}

func checkPlain(k string, v *doc.Value, kp string, isRoot bool, ptr string, anchors map[string]string) error {
	switch k {
	case "$schema":
		if v.Kind != doc.String || strings.TrimSuffix(v.S, "#") != Draft202012 {
			return fmt.Errorf("%s: unsupported $schema (only %s is supported)", where(kp), Draft202012)
		}
		if !isRoot {
			return fmt.Errorf("%s: $schema is only allowed at the schema root", where(kp))
		}
	case "$id":
		if !isRoot {
			return fmt.Errorf("%s: nested $id is not supported", where(kp))
		}
	case "$ref", "$dynamicRef":
		if v.Kind != doc.String {
			return fmt.Errorf("%s: %s must be a string", where(kp), k)
		}
		if !strings.HasPrefix(v.S, "#") {
			return fmt.Errorf("%s: only local references (starting with \"#\") are supported", where(kp))
		}
	case "$anchor", "$dynamicAnchor":
		if v.Kind == doc.String && k == "$anchor" {
			anchors[v.S] = ptr
		}
	case "format":
		if v.Kind != doc.String {
			return fmt.Errorf("%s: format must be a string", where(kp))
		}
		if !slices.Contains(AssertedFormats, v.S) {
			return fmt.Errorf("%s: unsupported format %q (supported: %s)", where(kp), v.S, strings.Join(AssertedFormats, ", "))
		}
	}
	return nil
}

func where(ptr string) string {
	if ptr == "" {
		return "schema root"
	}
	return "at " + ptr
}
