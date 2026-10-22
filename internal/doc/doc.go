// Package doc is zeze's ordered JSON value tree, JSON Pointer operations,
// canonical scalar encoding, and deterministic serialisation.
//
// Object key order is preserved and numbers keep their exact source text, so a
// document round-trips without reformatting values.
package doc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Kind identifies the JSON type of a Value.
type Kind int

const (
	Null Kind = iota
	Bool
	Number
	String
	Object
	Array
)

func (k Kind) String() string {
	switch k {
	case Null:
		return "null"
	case Bool:
		return "boolean"
	case Number:
		return "number"
	case String:
		return "string"
	case Object:
		return "object"
	case Array:
		return "array"
	}
	return "unknown"
}

// Value is one JSON value. Exactly the fields matching Kind are meaningful.
type Value struct {
	Kind Kind
	B    bool              // Bool
	N    string            // Number: exact, validated source text
	S    string            // String
	Keys []string          // Object: member names in document order
	M    map[string]*Value // Object: members
	A    []*Value          // Array: elements
}

// Constructors.
func NewNull() *Value             { return &Value{Kind: Null} }
func NewBool(b bool) *Value       { return &Value{Kind: Bool, B: b} }
func NewString(s string) *Value   { return &Value{Kind: String, S: s} }
func NewObject() *Value           { return &Value{Kind: Object, M: map[string]*Value{}} }
func NewArray(a ...*Value) *Value { return &Value{Kind: Array, A: a} }

// NewNumber validates text as an RFC 8259 number and keeps it verbatim.
func NewNumber(text string) (*Value, error) {
	if !ValidNumber(text) {
		return nil, fmt.Errorf("invalid number")
	}
	return &Value{Kind: Number, N: text}, nil
}

// ValidNumber reports whether s is an RFC 8259 number.
func ValidNumber(s string) bool {
	i := 0
	if i < len(s) && s[i] == '-' {
		i++
	}
	if i >= len(s) {
		return false
	}
	if s[i] == '0' {
		i++
	} else if s[i] >= '1' && s[i] <= '9' {
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
	} else {
		return false
	}
	if i < len(s) && s[i] == '.' {
		i++
		start := i
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if i == start {
			return false
		}
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		start := i
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if i == start {
			return false
		}
	}
	return i == len(s)
}

// IsLeaf reports whether v is a scalar (null, boolean, number, string).
func (v *Value) IsLeaf() bool { return v.Kind != Object && v.Kind != Array }

// Get returns an object member, or nil.
func (v *Value) Get(key string) *Value {
	if v.Kind != Object {
		return nil
	}
	return v.M[key]
}

// Put sets an object member, appending the key when new.
func (v *Value) Put(key string, val *Value) {
	if _, ok := v.M[key]; !ok {
		v.Keys = append(v.Keys, key)
	}
	v.M[key] = val
}

// Remove deletes an object member. It reports whether the key existed.
func (v *Value) Remove(key string) bool {
	if _, ok := v.M[key]; !ok {
		return false
	}
	delete(v.M, key)
	for i, k := range v.Keys {
		if k == key {
			v.Keys = append(v.Keys[:i], v.Keys[i+1:]...)
			break
		}
	}
	return true
}

// Clone returns a deep copy.
func (v *Value) Clone() *Value {
	c := *v
	switch v.Kind {
	case Object:
		c.Keys = append([]string(nil), v.Keys...)
		c.M = make(map[string]*Value, len(v.M))
		for k, m := range v.M {
			c.M[k] = m.Clone()
		}
	case Array:
		c.A = make([]*Value, len(v.A))
		for i, e := range v.A {
			c.A[i] = e.Clone()
		}
	}
	return &c
}

// Equal reports deep equality; numbers compare by source text.
func Equal(a, b *Value) bool {
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case Null:
		return true
	case Bool:
		return a.B == b.B
	case Number:
		return a.N == b.N
	case String:
		return a.S == b.S
	case Object:
		if len(a.M) != len(b.M) {
			return false
		}
		for k, av := range a.M {
			bv, ok := b.M[k]
			if !ok || !Equal(av, bv) {
				return false
			}
		}
		return true
	case Array:
		if len(a.A) != len(b.A) {
			return false
		}
		for i := range a.A {
			if !Equal(a.A[i], b.A[i]) {
				return false
			}
		}
		return true
	}
	return false
}

// ---- JSON Pointer (RFC 6901) ----

// ErrPointer is returned for malformed pointers.
var ErrPointer = errors.New("invalid JSON Pointer")

// ParsePointer splits an RFC 6901 pointer into unescaped tokens. "" is the root.
func ParsePointer(p string) ([]string, error) {
	if p == "" {
		return nil, nil
	}
	if p[0] != '/' {
		return nil, fmt.Errorf("%w: must start with '/'", ErrPointer)
	}
	parts := strings.Split(p[1:], "/")
	for i, t := range parts {
		for j := 0; j < len(t); j++ {
			if t[j] == '~' && (j+1 >= len(t) || (t[j+1] != '0' && t[j+1] != '1')) {
				return nil, fmt.Errorf("%w: bad '~' escape", ErrPointer)
			}
		}
		parts[i] = strings.ReplaceAll(strings.ReplaceAll(t, "~1", "/"), "~0", "~")
	}
	return parts, nil
}

// EscapeToken escapes one reference token.
func EscapeToken(t string) string {
	return strings.ReplaceAll(strings.ReplaceAll(t, "~", "~0"), "/", "~1")
}

// FormatPointer joins unescaped tokens into a pointer.
func FormatPointer(tokens []string) string {
	var b strings.Builder
	for _, t := range tokens {
		b.WriteByte('/')
		b.WriteString(EscapeToken(t))
	}
	return b.String()
}

// arrayIndex parses an array index token; "-" returns n (append position).
func arrayIndex(tok string, n int) (int, error) {
	if tok == "-" {
		return n, nil
	}
	if tok == "" || (len(tok) > 1 && tok[0] == '0') {
		return 0, fmt.Errorf("%w: bad array index", ErrPointer)
	}
	i, err := strconv.Atoi(tok)
	if err != nil || i < 0 {
		return 0, fmt.Errorf("%w: bad array index", ErrPointer)
	}
	return i, nil
}

// Lookup returns the value at tokens, or an error naming the failing pointer prefix.
func Lookup(root *Value, tokens []string) (*Value, error) {
	cur := root
	for i, t := range tokens {
		switch cur.Kind {
		case Object:
			next, ok := cur.M[t]
			if !ok {
				return nil, fmt.Errorf("%s: not found", FormatPointer(tokens[:i+1]))
			}
			cur = next
		case Array:
			idx, err := arrayIndex(t, len(cur.A))
			if err != nil {
				return nil, err
			}
			if idx >= len(cur.A) {
				return nil, fmt.Errorf("%s: not found", FormatPointer(tokens[:i+1]))
			}
			cur = cur.A[idx]
		default:
			return nil, fmt.Errorf("%s: is a %s, not a container", FormatPointer(tokens[:i]), cur.Kind)
		}
	}
	return cur, nil
}

// SetAt stores val at tokens. Missing intermediate objects are created;
// an array index may address an existing element or len (append), and "-" appends.
// Passing through an existing scalar is an error. tokens must be non-empty.
func SetAt(root *Value, tokens []string, val *Value) error {
	if len(tokens) == 0 {
		return fmt.Errorf("%w: cannot replace the root", ErrPointer)
	}
	cur := root
	for i, t := range tokens {
		last := i == len(tokens)-1
		switch cur.Kind {
		case Object:
			if last {
				cur.Put(t, val)
				return nil
			}
			next, ok := cur.M[t]
			if !ok {
				next = NewObject()
				cur.Put(t, next)
			}
			cur = next
		case Array:
			idx, err := arrayIndex(t, len(cur.A))
			if err != nil {
				return err
			}
			if idx > len(cur.A) {
				return fmt.Errorf("%s: index beyond array length %d", FormatPointer(tokens[:i+1]), len(cur.A))
			}
			if last {
				if idx == len(cur.A) {
					cur.A = append(cur.A, val)
				} else {
					cur.A[idx] = val
				}
				return nil
			}
			if idx == len(cur.A) {
				next := NewObject()
				cur.A = append(cur.A, next)
				cur = next
			} else {
				cur = cur.A[idx]
			}
		default:
			return fmt.Errorf("%s: is a %s, not a container", FormatPointer(tokens[:i]), cur.Kind)
		}
	}
	return nil
}

// DeleteAt removes the value at tokens. Array elements after it shift left.
func DeleteAt(root *Value, tokens []string) error {
	if len(tokens) == 0 {
		return fmt.Errorf("%w: cannot delete the root", ErrPointer)
	}
	parent, err := Lookup(root, tokens[:len(tokens)-1])
	if err != nil {
		return err
	}
	t := tokens[len(tokens)-1]
	switch parent.Kind {
	case Object:
		if !parent.Remove(t) {
			return fmt.Errorf("%s: not found", FormatPointer(tokens))
		}
	case Array:
		idx, err := arrayIndex(t, len(parent.A))
		if err != nil {
			return err
		}
		if idx >= len(parent.A) {
			return fmt.Errorf("%s: not found", FormatPointer(tokens))
		}
		parent.A = append(parent.A[:idx], parent.A[idx+1:]...)
	default:
		return fmt.Errorf("%s: is a %s, not a container", FormatPointer(tokens[:len(tokens)-1]), parent.Kind)
	}
	return nil
}

// Leaf is one scalar value with its location.
type Leaf struct {
	Pointer string
	Tokens  []string
	Value   *Value
}

// Leaves returns every scalar in document order. Empty containers yield nothing.
func Leaves(root *Value) []Leaf {
	var out []Leaf
	var walk func(v *Value, toks []string)
	walk = func(v *Value, toks []string) {
		switch v.Kind {
		case Object:
			for _, k := range v.Keys {
				walk(v.M[k], append(append([]string(nil), toks...), k))
			}
		case Array:
			for i, e := range v.A {
				walk(e, append(append([]string(nil), toks...), strconv.Itoa(i)))
			}
		default:
			out = append(out, Leaf{Pointer: FormatPointer(toks), Tokens: toks, Value: v})
		}
	}
	walk(root, nil)
	return out
}

// MapLeaves returns a copy of root with every scalar replaced by fn's result.
func MapLeaves(root *Value, fn func(pointer string, v *Value) (*Value, error)) (*Value, error) {
	var walk func(v *Value, toks []string) (*Value, error)
	walk = func(v *Value, toks []string) (*Value, error) {
		switch v.Kind {
		case Object:
			o := NewObject()
			for _, k := range v.Keys {
				c, err := walk(v.M[k], append(append([]string(nil), toks...), k))
				if err != nil {
					return nil, err
				}
				o.Put(k, c)
			}
			return o, nil
		case Array:
			a := NewArray()
			for i, e := range v.A {
				c, err := walk(e, append(append([]string(nil), toks...), strconv.Itoa(i)))
				if err != nil {
					return nil, err
				}
				a.A = append(a.A, c)
			}
			return a, nil
		default:
			return fn(FormatPointer(toks), v)
		}
	}
	return walk(root, nil)
}

// ---- Encoding ----

// Canonical returns the canonical JSON text of a scalar: numbers verbatim,
// strings with RFC 8785 escaping, literals as-is. It panics on containers.
func Canonical(v *Value) []byte {
	switch v.Kind {
	case Null:
		return []byte("null")
	case Bool:
		if v.B {
			return []byte("true")
		}
		return []byte("false")
	case Number:
		return []byte(v.N)
	case String:
		var b bytes.Buffer
		writeString(&b, v.S)
		return b.Bytes()
	}
	panic("doc.Canonical: container value")
}

// writeString writes s as a JSON string using RFC 8785 escaping rules.
func writeString(b *bytes.Buffer, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(b, `\u%04x`, r)
			} else if r == utf8.RuneError {
				b.WriteString(`�`)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
}

// Marshal serialises v with two-space indentation and a trailing newline,
// preserving object key order.
func Marshal(v *Value) []byte {
	var b bytes.Buffer
	marshal(&b, v, 0)
	b.WriteByte('\n')
	return b.Bytes()
}

// MarshalCompact serialises v without whitespace, preserving key order.
func MarshalCompact(v *Value) []byte {
	var b bytes.Buffer
	marshal(&b, v, -1)
	return b.Bytes()
}

func marshal(b *bytes.Buffer, v *Value, depth int) {
	nl := func(d int) {
		if depth >= 0 {
			b.WriteByte('\n')
			b.WriteString(strings.Repeat("  ", d))
		}
	}
	switch v.Kind {
	case Object:
		if len(v.Keys) == 0 {
			b.WriteString("{}")
			return
		}
		b.WriteByte('{')
		for i, k := range v.Keys {
			if i > 0 {
				b.WriteByte(',')
			}
			nl(depth + 1)
			writeString(b, k)
			b.WriteByte(':')
			if depth >= 0 {
				b.WriteByte(' ')
			}
			marshal(b, v.M[k], next(depth))
		}
		nl(depth)
		b.WriteByte('}')
	case Array:
		if len(v.A) == 0 {
			b.WriteString("[]")
			return
		}
		b.WriteByte('[')
		for i, e := range v.A {
			if i > 0 {
				b.WriteByte(',')
			}
			nl(depth + 1)
			marshal(b, e, next(depth))
		}
		nl(depth)
		b.WriteByte(']')
	default:
		b.Write(Canonical(v))
	}
}

func next(depth int) int {
	if depth < 0 {
		return -1
	}
	return depth + 1
}

// ToAny converts v into encoding/json-style Go values (map[string]any, []any,
// json.Number, string, bool, nil) for use with JSON Schema validators.
func ToAny(v *Value) any {
	switch v.Kind {
	case Null:
		return nil
	case Bool:
		return v.B
	case Number:
		return json.Number(v.N)
	case String:
		return v.S
	case Object:
		m := make(map[string]any, len(v.M))
		for k, e := range v.M {
			m[k] = ToAny(e)
		}
		return m
	case Array:
		a := make([]any, len(v.A))
		for i, e := range v.A {
			a[i] = ToAny(e)
		}
		return a
	}
	return nil
}

// FromAny converts encoding/json-style Go values into a Value. Map keys are
// sorted because Go maps have no order. float64 values are formatted with
// strconv 'g' -1; prefer json.Number input.
func FromAny(x any) (*Value, error) {
	switch t := x.(type) {
	case nil:
		return NewNull(), nil
	case bool:
		return NewBool(t), nil
	case json.Number:
		return NewNumber(string(t))
	case float64:
		return NewNumber(strconv.FormatFloat(t, 'g', -1, 64))
	case int:
		return NewNumber(strconv.Itoa(t))
	case string:
		return NewString(t), nil
	case map[string]any:
		o := NewObject()
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			c, err := FromAny(t[k])
			if err != nil {
				return nil, err
			}
			o.Put(k, c)
		}
		return o, nil
	case []any:
		a := NewArray()
		for _, e := range t {
			c, err := FromAny(e)
			if err != nil {
				return nil, err
			}
			a.A = append(a.A, c)
		}
		return a, nil
	}
	return nil, fmt.Errorf("unsupported Go type %T", x)
}
