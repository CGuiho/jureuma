package pair

import (
	"sort"
	"strconv"

	"github.com/CGuiho/zeze/internal/doc"
)

// ConcreteTokens replaces "-" array tokens with the concrete append index, so
// the pointer bound into ciphertexts is exact. Tokens below a missing member
// are returned unchanged (SetAt creates objects there).
func ConcreteTokens(root *doc.Value, tokens []string) []string {
	out := append([]string(nil), tokens...)
	cur := root
	for i, t := range out {
		if cur == nil {
			break
		}
		switch cur.Kind {
		case doc.Array:
			if t == "-" {
				out[i] = strconv.Itoa(len(cur.A))
				cur = nil
				continue
			}
			idx, err := strconv.Atoi(t)
			if err != nil || idx < 0 || idx >= len(cur.A) {
				cur = nil
				continue
			}
			cur = cur.A[idx]
		case doc.Object:
			cur = cur.M[t]
		default:
			cur = nil
		}
	}
	return out
}

// LeafPointersUnder returns the leaf pointers of root at or below prefix.
func LeafPointersUnder(root *doc.Value, tokens []string) []string {
	v, err := doc.Lookup(root, tokens)
	if err != nil {
		return nil
	}
	base := doc.FormatPointer(tokens)
	var out []string
	for _, l := range doc.Leaves(v) {
		out = append(out, base+l.Pointer)
	}
	return out
}

// AppendWithoutArray reports whether tokens use "-" where the parent is not
// an existing array (a missing parent, or a "-" after the appended element),
// so "-" would otherwise become an object member named "-". A "-" member of
// an existing object is a plain key and is not reported.
func AppendWithoutArray(root *doc.Value, tokens []string) bool {
	cur := root
	for _, t := range tokens {
		if cur == nil {
			if t == "-" {
				return true
			}
			continue
		}
		switch cur.Kind {
		case doc.Array:
			if t == "-" {
				cur = nil
				continue
			}
			idx, err := strconv.Atoi(t)
			if err != nil || idx < 0 || idx >= len(cur.A) {
				cur = nil
				continue
			}
			cur = cur.A[idx]
		case doc.Object:
			cur = cur.M[t]
		default:
			return t == "-"
		}
	}
	return false
}

// EmptyContainersOnlyIn returns the pointers of empty objects and arrays in
// plain whose location holds no container in body (sorted). Status compares
// leaves only, so such a structural edit (a new, still empty section) is
// otherwise invisible; decrypt uses this to avoid overwriting it.
func EmptyContainersOnlyIn(plain, body *doc.Value) []string {
	var out []string
	var walk func(v *doc.Value, toks []string)
	walk = func(v *doc.Value, toks []string) {
		switch v.Kind {
		case doc.Object:
			if len(v.M) == 0 && len(toks) > 0 {
				if b, err := doc.Lookup(body, toks); err != nil || b.Kind != doc.Object {
					out = append(out, doc.FormatPointer(toks))
				}
				return
			}
			for k, c := range v.M {
				walk(c, append(append([]string(nil), toks...), k))
			}
		case doc.Array:
			if len(v.A) == 0 {
				if b, err := doc.Lookup(body, toks); err != nil || b.Kind != doc.Array {
					out = append(out, doc.FormatPointer(toks))
				}
				return
			}
			for i, c := range v.A {
				walk(c, append(append([]string(nil), toks...), strconv.Itoa(i)))
			}
		}
	}
	if plain != nil {
		walk(plain, nil)
	}
	sort.Strings(out)
	return out
}
