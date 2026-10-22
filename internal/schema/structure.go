package schema

import (
	"fmt"
	"math/big"
	"regexp"
	"strconv"

	"github.com/CGuiho/zeze/internal/doc"
)

// ValidateStructure checks only structural keywords against a document whose
// leaves are opaque (encrypted): type (container versus scalar), required,
// dependentRequired, additionalProperties, unevaluatedProperties,
// propertyNames, patternProperties names, minItems/maxItems,
// minProperties/maxProperties, and unevaluatedItems/items false. Leaf values
// are ignored: a leaf satisfies any schema that allows some scalar type.
func (s *Schema) ValidateStructure(v *doc.Value) []Issue {
	return normalize(s.structural(s.raw, "", v, nil, 0))
}

func (s *Schema) structural(n *doc.Value, sptr string, inst *doc.Value, at []string, depth int) []Issue {
	if depth > maxDepth {
		return nil
	}
	here := doc.FormatPointer(at)
	if n.Kind == doc.Bool {
		if n.B {
			return nil
		}
		return []Issue{{Pointer: here, Keyword: "false", Message: "no value is allowed here"}}
	}
	if n.Kind != doc.Object {
		return nil
	}
	var out []Issue
	if r := n.Get("$ref"); r != nil && r.Kind == doc.String {
		if tn, tp, ok := s.resolveRef(r.S); ok {
			out = append(out, s.structural(tn, tp, inst, at, depth+1)...)
		}
	}
	if a := n.Get("allOf"); a != nil && a.Kind == doc.Array {
		for i, b := range a.A {
			out = append(out, s.structural(b, sptr+"/allOf/"+strconv.Itoa(i), inst, at, depth+1)...)
		}
	}
	for _, kw := range []string{"anyOf", "oneOf"} {
		if a := n.Get(kw); a != nil && a.Kind == doc.Array {
			out = append(out, s.anyStructural(a.A, sptr+"/"+kw, kw, inst, at, depth)...)
		}
	}
	if n.Get("if") != nil {
		th, el := n.Get("then"), n.Get("else")
		if th != nil && el != nil {
			out = append(out, s.anyStructural([]*doc.Value{th, el}, "", "if", inst, at, depth, sptr+"/then", sptr+"/else")...)
		}
	}
	if ts := typeSet(n.Get("type")); ts != nil {
		ok := false
		switch inst.Kind {
		case doc.Object:
			ok = ts["object"]
		case doc.Array:
			ok = ts["array"]
		default:
			ok = ts["string"] || ts["number"] || ts["integer"] || ts["boolean"] || ts["null"]
		}
		if !ok {
			out = append(out, Issue{Pointer: here, Keyword: "type", Message: "must be of type " + typeList(ts)})
		}
	}
	switch inst.Kind {
	case doc.Object:
		out = append(out, s.structObject(n, sptr, inst, at, depth)...)
	case doc.Array:
		out = append(out, s.structArray(n, sptr, inst, at, depth)...)
	}
	return out
}

// anyStructural accepts if any branch accepts. ptrs optionally overrides the
// schema pointers of the branches.
func (s *Schema) anyStructural(branches []*doc.Value, base, kw string, inst *doc.Value, at []string, depth int, ptrs ...string) []Issue {
	var all []Issue
	for i, b := range branches {
		p := base + "/" + strconv.Itoa(i)
		if i < len(ptrs) {
			p = ptrs[i]
		}
		is := s.structural(b, p, inst, at, depth+1)
		if len(is) == 0 {
			return nil
		}
		all = append(all, is...)
	}
	if kw == "if" {
		kw = "then/else"
	}
	all = append(all, Issue{Pointer: doc.FormatPointer(at), Keyword: kw,
		Message: fmt.Sprintf("structure matches none of the %d alternatives (%s)", len(branches), kw)})
	return all
}

func (s *Schema) structObject(n *doc.Value, sptr string, inst *doc.Value, at []string, depth int) []Issue {
	var out []Issue
	here := doc.FormatPointer(at)
	cnt := big.NewRat(int64(len(inst.Keys)), 1)
	if lim, txt, ok := numParam(n, "minProperties"); ok && cnt.Cmp(lim) < 0 {
		out = append(out, Issue{Pointer: here, Keyword: "minProperties", Message: "must have at least " + txt + " properties"})
	}
	if lim, txt, ok := numParam(n, "maxProperties"); ok && cnt.Cmp(lim) > 0 {
		out = append(out, Issue{Pointer: here, Keyword: "maxProperties", Message: "must have at most " + txt + " properties"})
	}
	if r := n.Get("required"); r != nil && r.Kind == doc.Array {
		for _, name := range r.A {
			if name.Kind == doc.String && inst.Get(name.S) == nil {
				out = append(out, Issue{Pointer: here, Keyword: "required", Message: fmt.Sprintf("required property %q missing", name.S)})
			}
		}
	}
	if dr := n.Get("dependentRequired"); dr != nil && dr.Kind == doc.Object {
		for _, prop := range dr.Keys {
			if inst.Get(prop) == nil || dr.M[prop].Kind != doc.Array {
				continue
			}
			for _, need := range dr.M[prop].A {
				if need.Kind == doc.String && inst.Get(need.S) == nil {
					out = append(out, Issue{Pointer: here, Keyword: "dependentRequired",
						Message: fmt.Sprintf("property %q requires property %q", prop, need.S)})
				}
			}
		}
	}
	props := n.Get("properties")
	pats := n.Get("patternProperties")
	ap := n.Get("additionalProperties")
	up := n.Get("unevaluatedProperties")
	pn := n.Get("propertyNames")
	for _, key := range inst.Keys {
		child := inst.M[key]
		next := append(append([]string(nil), at...), key)
		matched := false
		if props != nil && props.Kind == doc.Object {
			if c := props.Get(key); c != nil {
				matched = true
				out = append(out, s.structural(c, sptr+"/properties/"+doc.EscapeToken(key), child, next, depth+1)...)
			}
		}
		if pats != nil && pats.Kind == doc.Object {
			for _, pat := range pats.Keys {
				if re, err := regexp.Compile(pat); err == nil && re.MatchString(key) {
					matched = true
					out = append(out, s.structural(pats.M[pat], sptr+"/patternProperties/"+doc.EscapeToken(pat), child, next, depth+1)...)
				}
			}
		}
		if !matched && ap != nil {
			matched = true
			if ap.Kind == doc.Bool && !ap.B {
				out = append(out, Issue{Pointer: doc.FormatPointer(next), Keyword: "additionalProperties",
					Message: fmt.Sprintf("property %q is not allowed (additionalProperties)", key)})
			} else {
				out = append(out, s.structural(ap, sptr+"/additionalProperties", child, next, depth+1)...)
			}
		}
		if up != nil && !matched && !s.evaluatesKey(n, key, true, depth) {
			if up.Kind == doc.Bool && !up.B {
				out = append(out, Issue{Pointer: doc.FormatPointer(next), Keyword: "unevaluatedProperties",
					Message: fmt.Sprintf("property %q is not allowed (unevaluatedProperties)", key)})
			} else {
				out = append(out, s.structural(up, sptr+"/unevaluatedProperties", child, next, depth+1)...)
			}
		}
		if pn != nil {
			if is := s.validateAt(sptr+"/propertyNames", doc.NewString(key), nil); len(is) > 0 {
				out = append(out, Issue{Pointer: doc.FormatPointer(next), Keyword: "propertyNames",
					Message: fmt.Sprintf("property name %q does not satisfy propertyNames", key)})
			}
		}
	}
	return out
}

// evaluatesKey reports whether any subschema reachable in place from n
// (via $ref, allOf, anyOf, oneOf, then, else, dependentSchemas) could
// evaluate key. self controls whether n's own properties are considered.
func (s *Schema) evaluatesKey(n *doc.Value, key string, skipSelf bool, depth int) bool {
	if depth > maxDepth || n.Kind != doc.Object {
		return false
	}
	if !skipSelf {
		if p := n.Get("properties"); p != nil && p.Kind == doc.Object && p.Get(key) != nil {
			return true
		}
		if p := n.Get("patternProperties"); p != nil && p.Kind == doc.Object {
			for _, pat := range p.Keys {
				if re, err := regexp.Compile(pat); err == nil && re.MatchString(key) {
					return true
				}
			}
		}
		if n.Get("additionalProperties") != nil || n.Get("unevaluatedProperties") != nil {
			return true
		}
	}
	if r := n.Get("$ref"); r != nil && r.Kind == doc.String {
		if tn, _, ok := s.resolveRef(r.S); ok && s.evaluatesKey(tn, key, false, depth+1) {
			return true
		}
	}
	for _, kw := range []string{"allOf", "anyOf", "oneOf"} {
		if a := n.Get(kw); a != nil && a.Kind == doc.Array {
			for _, b := range a.A {
				if s.evaluatesKey(b, key, false, depth+1) {
					return true
				}
			}
		}
	}
	for _, kw := range []string{"then", "else"} {
		if b := n.Get(kw); b != nil && s.evaluatesKey(b, key, false, depth+1) {
			return true
		}
	}
	if d := n.Get("dependentSchemas"); d != nil && d.Kind == doc.Object {
		for _, k := range d.Keys {
			if s.evaluatesKey(d.M[k], key, false, depth+1) {
				return true
			}
		}
	}
	return false
}

func (s *Schema) structArray(n *doc.Value, sptr string, inst *doc.Value, at []string, depth int) []Issue {
	var out []Issue
	here := doc.FormatPointer(at)
	cnt := big.NewRat(int64(len(inst.A)), 1)
	if lim, txt, ok := numParam(n, "minItems"); ok && cnt.Cmp(lim) < 0 {
		out = append(out, Issue{Pointer: here, Keyword: "minItems", Message: "must have at least " + txt + " items"})
	}
	if lim, txt, ok := numParam(n, "maxItems"); ok && cnt.Cmp(lim) > 0 {
		out = append(out, Issue{Pointer: here, Keyword: "maxItems", Message: "must have at most " + txt + " items"})
	}
	pre := n.Get("prefixItems")
	items := n.Get("items")
	ui := n.Get("unevaluatedItems")
	npre := 0
	if pre != nil && pre.Kind == doc.Array {
		npre = len(pre.A)
	}
	for i, el := range inst.A {
		idx := strconv.Itoa(i)
		next := append(append([]string(nil), at...), idx)
		switch {
		case i < npre:
			out = append(out, s.structural(pre.A[i], sptr+"/prefixItems/"+idx, el, next, depth+1)...)
		case items != nil:
			if items.Kind == doc.Bool && !items.B {
				out = append(out, Issue{Pointer: doc.FormatPointer(next), Keyword: "items", Message: "no item is allowed at this index (items)"})
			} else {
				out = append(out, s.structural(items, sptr+"/items", el, next, depth+1)...)
			}
		case ui != nil && !s.evaluatesItem(n, i, depth):
			if ui.Kind == doc.Bool && !ui.B {
				out = append(out, Issue{Pointer: doc.FormatPointer(next), Keyword: "unevaluatedItems", Message: "no item is allowed at this index (unevaluatedItems)"})
			} else {
				out = append(out, s.structural(ui, sptr+"/unevaluatedItems", el, next, depth+1)...)
			}
		}
	}
	return out
}

// evaluatesItem reports whether an in-place applicator of n could evaluate index i.
func (s *Schema) evaluatesItem(n *doc.Value, i int, depth int) bool {
	if depth > maxDepth || n.Kind != doc.Object {
		return false
	}
	var subs []*doc.Value
	if r := n.Get("$ref"); r != nil && r.Kind == doc.String {
		if tn, _, ok := s.resolveRef(r.S); ok {
			subs = append(subs, tn)
		}
	}
	for _, kw := range []string{"allOf", "anyOf", "oneOf"} {
		if a := n.Get(kw); a != nil && a.Kind == doc.Array {
			subs = append(subs, a.A...)
		}
	}
	for _, kw := range []string{"then", "else"} {
		if b := n.Get(kw); b != nil {
			subs = append(subs, b)
		}
	}
	for _, b := range subs {
		if b.Kind != doc.Object {
			continue
		}
		if b.Get("items") != nil || b.Get("contains") != nil || b.Get("unevaluatedItems") != nil {
			return true
		}
		if p := b.Get("prefixItems"); p != nil && p.Kind == doc.Array && i < len(p.A) {
			return true
		}
		if s.evaluatesItem(b, i, depth+1) {
			return true
		}
	}
	return false
}

// numParam reads a numeric schema keyword as an exact rational plus its source text.
func numParam(n *doc.Value, kw string) (*big.Rat, string, bool) {
	v := n.Get(kw)
	if v == nil || v.Kind != doc.Number {
		return nil, "", false
	}
	r, ok := new(big.Rat).SetString(v.N)
	if !ok {
		return nil, "", false
	}
	return r, v.N, true
}
