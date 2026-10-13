package schema

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/CGuiho/jureuma/internal/doc"
)

// maxDepth bounds $ref expansion so recursive schemas terminate.
const maxDepth = 64

// expr is the set of schema constraints that could apply to one location,
// as an and/or tree over subschemas.
type expr struct {
	op     exprOp
	node   *doc.Value // opLeaf: subschema that applies to the location itself
	ptr    string     // opLeaf: its schema pointer
	kids   []*expr    // opAnd, opOr
	issues []Issue    // opFail
}

type exprOp int

const (
	opTrue exprOp = iota // nothing known: accepts anything (undetermined)
	opFail               // known to reject
	opLeaf
	opAnd
	opOr
)

var exTrue = &expr{op: opTrue}

func and(kids []*expr) *expr {
	var ks []*expr
	for _, k := range kids {
		if k.op != opTrue {
			ks = append(ks, k)
		}
	}
	switch len(ks) {
	case 0:
		return exTrue
	case 1:
		return ks[0]
	}
	return &expr{op: opAnd, kids: ks}
}

func or(kids []*expr) *expr {
	for _, k := range kids {
		if k.op == opTrue {
			return exTrue
		}
	}
	switch len(kids) {
	case 0:
		return exTrue
	case 1:
		return kids[0]
	}
	return &expr{op: opOr, kids: kids}
}

func fail(is Issue) *expr { return &expr{op: opFail, issues: []Issue{is}} }

// resolveRef returns the raw node and schema pointer a local $ref names.
func (s *Schema) resolveRef(ref string) (*doc.Value, string, bool) {
	if !strings.HasPrefix(ref, "#") {
		return nil, "", false
	}
	frag := ref[1:]
	if frag != "" && frag[0] != '/' {
		p, ok := s.anchors[frag]
		if !ok {
			return nil, "", false
		}
		frag = p
	}
	if un, err := urlUnescape(frag); err == nil {
		frag = un
	}
	toks, err := doc.ParsePointer(frag)
	if err != nil {
		return nil, "", false
	}
	n, err := doc.Lookup(s.raw, toks)
	if err != nil {
		return nil, "", false
	}
	return n, doc.FormatPointer(toks), true
}

// resolve builds the constraint expression for the instance location reached
// by following toks from a value validated by node (at schema pointer sptr).
// at holds the instance tokens consumed so far.
func (s *Schema) resolve(node *doc.Value, sptr string, toks, at []string, depth int) *expr {
	if depth > maxDepth {
		return exTrue
	}
	if len(toks) == 0 {
		if node.Kind == doc.Bool && node.B {
			return exTrue
		}
		return &expr{op: opLeaf, node: node, ptr: sptr}
	}
	if node.Kind == doc.Bool {
		if node.B {
			return exTrue
		}
		return fail(Issue{Pointer: doc.FormatPointer(at), Keyword: "false", Message: "no value is allowed here"})
	}
	if node.Kind != doc.Object {
		return exTrue
	}
	var parts []*expr
	if r := node.Get("$ref"); r != nil && r.Kind == doc.String {
		if tn, tp, ok := s.resolveRef(r.S); ok {
			parts = append(parts, s.resolve(tn, tp, toks, at, depth+1))
		}
	}
	if a := node.Get("allOf"); a != nil && a.Kind == doc.Array {
		for i, b := range a.A {
			parts = append(parts, s.resolve(b, sptr+"/allOf/"+strconv.Itoa(i), toks, at, depth+1))
		}
	}
	for _, kw := range []string{"anyOf", "oneOf"} {
		if a := node.Get(kw); a != nil && a.Kind == doc.Array {
			var alts []*expr
			for i, b := range a.A {
				alts = append(alts, s.resolve(b, sptr+"/"+kw+"/"+strconv.Itoa(i), toks, at, depth+1))
			}
			parts = append(parts, or(alts))
		}
	}
	if node.Get("if") != nil {
		th, el := node.Get("then"), node.Get("else")
		if th != nil && el != nil {
			parts = append(parts, or([]*expr{
				s.resolve(th, sptr+"/then", toks, at, depth+1),
				s.resolve(el, sptr+"/else", toks, at, depth+1),
			}))
		}
	}
	parts = append(parts, s.child(node, sptr, toks, at, depth))
	return and(parts)
}

// child resolves the step from node into member/element toks[0].
func (s *Schema) child(node *doc.Value, sptr string, toks, at []string, depth int) *expr {
	tok, rest := toks[0], toks[1:]
	next := append(append([]string(nil), at...), tok)
	here := doc.FormatPointer(at)
	types := typeSet(node.Get("type"))
	objOK := types == nil || types["object"]
	arrOK := (types == nil || types["array"]) && isIndexToken(tok)

	var objE, arrE *expr
	objKnown, arrKnown := false, false
	if objOK {
		var ps []*expr
		matched := false
		if p := node.Get("properties"); p != nil && p.Kind == doc.Object {
			objKnown = true
			if c := p.Get(tok); c != nil {
				matched = true
				ps = append(ps, s.resolve(c, sptr+"/properties/"+doc.EscapeToken(tok), rest, next, depth+1))
			}
		}
		if p := node.Get("patternProperties"); p != nil && p.Kind == doc.Object {
			objKnown = true
			for _, pat := range p.Keys {
				if re, err := regexp.Compile(pat); err == nil && re.MatchString(tok) {
					matched = true
					ps = append(ps, s.resolve(p.M[pat], sptr+"/patternProperties/"+doc.EscapeToken(pat), rest, next, depth+1))
				}
			}
		}
		if ap := node.Get("additionalProperties"); ap != nil {
			objKnown = true
			if !matched {
				if ap.Kind == doc.Bool && !ap.B {
					ps = append(ps, fail(Issue{Pointer: doc.FormatPointer(next), Keyword: "additionalProperties",
						Message: fmt.Sprintf("property %q is not allowed (additionalProperties)", tok)}))
				} else {
					ps = append(ps, s.resolve(ap, sptr+"/additionalProperties", rest, next, depth+1))
				}
			}
		}
		if pn := node.Get("propertyNames"); pn != nil {
			objKnown = true
			if issues := s.validateAt(sptr+"/propertyNames", doc.NewString(tok), nil); len(issues) > 0 {
				ps = append(ps, fail(Issue{Pointer: doc.FormatPointer(next), Keyword: "propertyNames",
					Message: fmt.Sprintf("property name %q does not satisfy propertyNames", tok)}))
			}
		}
		if types != nil && types["object"] {
			objKnown = true
		}
		objE = and(ps)
	}
	if arrOK {
		pre := node.Get("prefixItems")
		items := node.Get("items")
		if (pre != nil && pre.Kind == doc.Array) || items != nil || (types != nil && types["array"]) {
			arrKnown = true
		}
		switch {
		case tok == "-":
			if pre != nil && pre.Kind == doc.Array && len(pre.A) > 0 {
				arrE = exTrue // index unknown: any prefix item or items could apply
			} else if items != nil {
				arrE = s.resolve(items, sptr+"/items", rest, next, depth+1)
			} else {
				arrE = exTrue
			}
		default:
			i, _ := strconv.Atoi(tok)
			if pre != nil && pre.Kind == doc.Array && i < len(pre.A) {
				arrE = s.resolve(pre.A[i], sptr+"/prefixItems/"+tok, rest, next, depth+1)
			} else if items != nil {
				if items.Kind == doc.Bool && !items.B {
					arrE = fail(Issue{Pointer: doc.FormatPointer(next), Keyword: "items", Message: "no item is allowed at this index (items)"})
				} else {
					arrE = s.resolve(items, sptr+"/items", rest, next, depth+1)
				}
			} else {
				arrE = exTrue
			}
		}
	}
	switch {
	case objE != nil && arrE != nil:
		if objKnown && !arrKnown {
			return objE
		}
		if arrKnown && !objKnown {
			return arrE
		}
		return or([]*expr{objE, arrE})
	case objE != nil:
		return objE
	case arrE != nil:
		return arrE
	}
	// The schema's type excludes any container that could hold tok.
	return fail(Issue{Pointer: here, Keyword: "type", Message: "must be of type " + typeList(types)})
}

func isIndexToken(t string) bool {
	if t == "-" {
		return true
	}
	if t == "" || (len(t) > 1 && t[0] == '0') {
		return false
	}
	for i := 0; i < len(t); i++ {
		if t[i] < '0' || t[i] > '9' {
			return false
		}
	}
	return len(t) < 10
}

// typeSet returns the set named by a "type" keyword, or nil if unconstrained.
func typeSet(t *doc.Value) map[string]bool {
	if t == nil {
		return nil
	}
	m := map[string]bool{}
	switch t.Kind {
	case doc.String:
		m[t.S] = true
	case doc.Array:
		for _, e := range t.A {
			if e.Kind == doc.String {
				m[e.S] = true
			}
		}
	default:
		return nil
	}
	return m
}

var typeOrder = []string{"string", "number", "integer", "boolean", "object", "array", "null"}

func typeList(m map[string]bool) string {
	var out []string
	for _, t := range typeOrder {
		if m[t] {
			out = append(out, t)
		}
	}
	return strings.Join(out, " or ")
}

// validateAt fully validates v against the subschema at schema pointer sptr;
// issue pointers are prefixed with prefix.
func (s *Schema) validateAt(sptr string, v *doc.Value, prefix []string) []Issue {
	sch, err := s.fragment(sptr)
	if err != nil {
		return nil // cannot be determined; the whole-schema compile already succeeded
	}
	return s.validateWith(sch, v, prefix)
}

// eval applies e to v (the value at target). determined reports whether some
// subschema actually constrained the location.
func (s *Schema) eval(e *expr, v *doc.Value, target []string) (issues []Issue, determined bool) {
	switch e.op {
	case opTrue:
		return nil, false
	case opFail:
		return e.issues, true
	case opLeaf:
		return s.validateAt(e.ptr, v, target), constraining(e.node)
	case opAnd:
		for _, k := range e.kids {
			is, d := s.eval(k, v, target)
			issues = append(issues, is...)
			determined = determined || d
		}
		return issues, determined
	case opOr:
		var all []Issue
		for _, k := range e.kids {
			is, d := s.eval(k, v, target)
			if len(is) == 0 {
				return nil, d
			}
			all = append(all, is...)
			determined = true
		}
		all = append(all, Issue{Pointer: doc.FormatPointer(target), Keyword: "anyOf",
			Message: fmt.Sprintf("matches none of the %d schema branches that could apply here", len(e.kids))})
		return all, determined
	}
	return nil, false
}

// constraining reports whether a subschema constrains values at all.
func constraining(n *doc.Value) bool {
	switch n.Kind {
	case doc.Bool:
		return !n.B
	case doc.Object:
		for _, k := range n.Keys {
			switch {
			case strings.HasPrefix(k, "x-"):
			case k == "title" || k == "description" || k == "default" || k == "examples" ||
				k == "$comment" || k == "deprecated" || k == "readOnly" || k == "writeOnly" ||
				k == "$schema" || k == "$id" || k == "$anchor" || k == "$dynamicAnchor" || k == "$defs":
			default:
				return true
			}
		}
	}
	return false
}

// ValidateValue provisionally validates one value at pointer with siblings
// unknown: it accepts if any schema branch that could apply to the location
// accepts it.
func (s *Schema) ValidateValue(pointer string, v *doc.Value) []Issue {
	issues, _ := s.ValidateValueUnknown(pointer, v)
	return issues
}

// ValidateValueUnknown is ValidateValue that also reports unknown=true when
// no subschema constraining the location could be determined.
func (s *Schema) ValidateValueUnknown(pointer string, v *doc.Value) (issues []Issue, unknown bool) {
	toks, err := doc.ParsePointer(pointer)
	if err != nil {
		return []Issue{{Pointer: pointer, Keyword: "", Message: "invalid JSON Pointer"}}, false
	}
	e := s.resolve(s.raw, "", toks, nil, 0)
	issues, det := s.eval(e, v, toks)
	return normalize(issues), !det
}

// ExpectedTypes returns the JSON type(s) the schema declares at pointer, or
// nil if not determinable. A non-nil empty slice means no type can satisfy it.
func (s *Schema) ExpectedTypes(pointer string) []string {
	toks, err := doc.ParsePointer(pointer)
	if err != nil {
		return nil
	}
	set := s.exprTypes(s.resolve(s.raw, "", toks, nil, 0), 0)
	if set == nil {
		return nil
	}
	out := []string{}
	for _, t := range typeOrder {
		if set[t] {
			out = append(out, t)
		}
	}
	return out
}

func (s *Schema) exprTypes(e *expr, depth int) map[string]bool {
	switch e.op {
	case opLeaf:
		return s.nodeTypes(e.node, depth)
	case opFail:
		return map[string]bool{}
	case opAnd:
		var acc map[string]bool
		for _, k := range e.kids {
			acc = intersect(acc, s.exprTypes(k, depth))
		}
		return acc
	case opOr:
		acc := map[string]bool{}
		for _, k := range e.kids {
			t := s.exprTypes(k, depth)
			if t == nil {
				return nil
			}
			for x := range t {
				acc[x] = true
			}
		}
		return acc
	}
	return nil
}

func (s *Schema) nodeTypes(n *doc.Value, depth int) map[string]bool {
	if depth > maxDepth {
		return nil
	}
	if n.Kind == doc.Bool {
		if n.B {
			return nil
		}
		return map[string]bool{}
	}
	if n.Kind != doc.Object {
		return nil
	}
	acc := typeSet(n.Get("type"))
	if r := n.Get("$ref"); r != nil && r.Kind == doc.String {
		if tn, _, ok := s.resolveRef(r.S); ok {
			acc = intersect(acc, s.nodeTypes(tn, depth+1))
		}
	}
	if a := n.Get("allOf"); a != nil && a.Kind == doc.Array {
		for _, b := range a.A {
			acc = intersect(acc, s.nodeTypes(b, depth+1))
		}
	}
	for _, kw := range []string{"anyOf", "oneOf"} {
		if a := n.Get(kw); a != nil && a.Kind == doc.Array {
			u := map[string]bool{}
			for _, b := range a.A {
				t := s.nodeTypes(b, depth+1)
				if t == nil {
					u = nil
					break
				}
				for x := range t {
					u[x] = true
				}
			}
			acc = intersect(acc, u)
		}
	}
	return acc
}

// intersect intersects two type sets; nil means "any". integer is a subset of number.
func intersect(a, b map[string]bool) map[string]bool {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	out := map[string]bool{}
	for t := range a {
		switch {
		case b[t]:
			out[t] = true
		case t == "integer" && b["number"]:
			out["integer"] = true
		case t == "number" && b["integer"]:
			out["integer"] = true
		}
	}
	return out
}

// Secret reports whether the schema marks the pointer, or any ancestor of it,
// with "x-jureuma-secret": true in any branch that could apply.
func (s *Schema) Secret(pointer string) bool {
	toks, err := doc.ParsePointer(pointer)
	if err != nil {
		return false
	}
	for i := 0; i <= len(toks); i++ {
		if s.exprSecret(s.resolve(s.raw, "", toks[:i], nil, 0), 0) {
			return true
		}
	}
	return false
}

func (s *Schema) exprSecret(e *expr, depth int) bool {
	switch e.op {
	case opLeaf:
		return s.nodeSecret(e.node, depth)
	case opAnd, opOr:
		for _, k := range e.kids {
			if s.exprSecret(k, depth) {
				return true
			}
		}
	}
	return false
}

func (s *Schema) nodeSecret(n *doc.Value, depth int) bool {
	if depth > maxDepth || n.Kind != doc.Object {
		return false
	}
	if v := n.Get(SecretKeyword); v != nil && v.Kind == doc.Bool && v.B {
		return true
	}
	if r := n.Get("$ref"); r != nil && r.Kind == doc.String {
		if tn, _, ok := s.resolveRef(r.S); ok && s.nodeSecret(tn, depth+1) {
			return true
		}
	}
	for _, kw := range []string{"allOf", "anyOf", "oneOf"} {
		if a := n.Get(kw); a != nil && a.Kind == doc.Array {
			for _, b := range a.A {
				if s.nodeSecret(b, depth+1) {
					return true
				}
			}
		}
	}
	for _, kw := range []string{"then", "else"} {
		if b := n.Get(kw); b != nil && s.nodeSecret(b, depth+1) {
			return true
		}
	}
	return false
}

// SecretValueKeywords reports schema locations marked x-jureuma-secret that
// also carry default, examples, const, or enum, which could embed secrets
// (used by "jureuma check" warnings). Pointers are schema pointers.
func (s *Schema) SecretValueKeywords() []Issue {
	var out []Issue
	var walk func(n *doc.Value, ptr string)
	walk = func(n *doc.Value, ptr string) {
		switch n.Kind {
		case doc.Object:
			if v := n.Get(SecretKeyword); v != nil && v.Kind == doc.Bool && v.B {
				for _, kw := range []string{"default", "examples", "const", "enum"} {
					if n.Get(kw) != nil {
						out = append(out, Issue{Pointer: ptr, Keyword: kw,
							Message: fmt.Sprintf("secret location carries %q, which could embed a secret", kw)})
					}
				}
			}
			for _, k := range n.Keys {
				if kwSchema[k] || kwSchemaArray[k] || kwSchemaMap[k] {
					kp := ptr + "/" + doc.EscapeToken(k)
					c := n.M[k]
					switch {
					case kwSchema[k]:
						walk(c, kp)
					case c.Kind == doc.Array:
						for i, e := range c.A {
							walk(e, kp+"/"+strconv.Itoa(i))
						}
					case c.Kind == doc.Object:
						for _, name := range c.Keys {
							walk(c.M[name], kp+"/"+doc.EscapeToken(name))
						}
					}
				}
			}
		}
	}
	walk(s.raw, "")
	return out
}
