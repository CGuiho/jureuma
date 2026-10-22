// Lossless JSONC editing. Edit parses the original file text into a span
// tree (every key, value, comma and comment keeps its byte span) and splices
// only the bytes a change needs, so comments, blank lines, indentation, key
// order, number text and string-escape spelling survive untouched outside the
// edited spans. When a change cannot be represented without touching bytes
// outside its spans, Edit refuses; it never falls back to rewriting.
//
// Errors never contain document text: they carry fixed descriptions and JSON
// Pointers of field names only, never values.
package jsonc

import (
	"bytes"
	"errors"
	"strings"

	"github.com/CGuiho/zeze/internal/doc"
)

// Edit errors: fixed strings, never document text.
var (
	// ErrMismatch means the given old document is not the file text.
	ErrMismatch = errors.New("the file text does not match the given document")
	// ErrUnrepresentable means a change needs bytes outside its own spans.
	ErrUnrepresentable = errors.New("the change cannot be applied without touching preserved text")
	// ErrVerify means the edited text does not reproduce the target document.
	ErrVerify = errors.New("the edited text does not reproduce the target document")
)

// Edit returns original with the changes from old to new spliced in, keeping
// every byte outside the edited spans exactly as it is. old must be the
// document parsed from original (Parse, lenient); the result parses to exactly
// new. Object key order in new may differ from the file: surviving keys keep
// their file order and new keys are appended before the closing brace.
func Edit(original []byte, old, new *doc.Value) ([]byte, error) {
	cur, err := Parse(original, false)
	if err != nil {
		return nil, err
	}
	if !equalDocs(cur, old) {
		return nil, ErrMismatch
	}
	tree, err := parseLossless(original)
	if err != nil {
		return nil, err
	}
	ed := &editor{data: original, nl: newlineOf(original)}
	if err := ed.value(tree.root, cur, new); err != nil {
		return nil, err
	}
	out, err := ed.apply(tree)
	if err != nil {
		return nil, err
	}
	got, err := Parse(out, false)
	if err != nil || !equalDocs(got, new) {
		return nil, ErrVerify
	}
	return out, nil
}

// newlineOf returns the file's newline spelling.
func newlineOf(data []byte) string {
	if bytes.Contains(data, []byte("\r\n")) {
		return "\r\n"
	}
	return "\n"
}

// equalDocs reports semantic equality: object key order does not matter;
// numbers and strings compare by their exact stored spelling.
func equalDocs(a, b *doc.Value) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case doc.Object:
		if len(a.Keys) != len(b.Keys) {
			return false
		}
		for _, k := range a.Keys {
			bv := b.Get(k)
			if bv == nil || !equalDocs(a.M[k], bv) {
				return false
			}
		}
		return true
	case doc.Array:
		if len(a.A) != len(b.A) {
			return false
		}
		for i := range a.A {
			if !equalDocs(a.A[i], b.A[i]) {
				return false
			}
		}
		return true
	case doc.Number:
		return a.N == b.N
	case doc.String:
		return a.S == b.S
	case doc.Bool:
		return a.B == b.B
	}
	return true // null
}

// ---- lossless tree ----

// lspan is a half-open byte range of the original text.
type lspan struct{ start, end int }

// lvalue is one JSON value with its exact byte span (strings include their
// quotes). Containers additionally record their delimiters and slots; leaves
// record their spelling in str.
type lvalue struct {
	kind  doc.Kind
	span  lspan
	str   string // string: decoded text; number: stored text
	open  int    // containers: offset of { or [
	close int    // containers: offset of } or ]
	slots []lslot
}

// lslot is one object member or array element. key is the member's key token
// (nil for array elements); colon is the member's ':' offset (-1 for array
// elements); comma is the separator that follows the slot's value — a trailing
// comma counts — or -1.
type lslot struct {
	key   *lvalue
	value *lvalue
	colon int
	comma int
}

// keyText returns the decoded key of an object member slot.
func (s lslot) keyText() string { return s.key.str }

// ltree is the lossless parse of one file: the root value plus the span of
// every comment, so edits can be proven not to cut preserved text.
type ltree struct {
	root     *lvalue
	comments []lspan
}

type lparser struct {
	data     []byte
	pos      int
	comments []lspan
}

func parseLossless(data []byte) (*ltree, error) {
	_, skip := stripBOM(data)
	p := &lparser{data: data, pos: skip}
	if err := p.ws(); err != nil {
		return nil, err
	}
	root, err := p.any(0)
	if err != nil {
		return nil, err
	}
	if err := p.ws(); err != nil {
		return nil, err
	}
	if p.pos < len(data) {
		return nil, p.at(p.pos, ClassTrailingContent)
	}
	return &ltree{root: root, comments: p.comments}, nil
}

// at builds a SyntaxError for a byte offset (a BOM is not counted as a column).
func (p *lparser) at(off int, class string) *SyntaxError {
	q := &parser{data: p.data}
	return q.errAt(off, class)
}

// ws skips whitespace and comments, recording every comment span.
func (p *lparser) ws() error {
	for p.pos < len(p.data) {
		switch p.data[p.pos] {
		case ' ', '\t', '\n', '\r':
			p.pos++
		case '/':
			if p.pos+1 >= len(p.data) {
				return p.at(p.pos, ClassUnexpectedChar)
			}
			start := p.pos
			switch p.data[p.pos+1] {
			case '/':
				p.pos += 2
				for p.pos < len(p.data) && p.data[p.pos] != '\n' && p.data[p.pos] != '\r' {
					p.pos++
				}
			case '*':
				end := bytes.Index(p.data[p.pos+2:], []byte("*/"))
				if end < 0 {
					return p.at(start, ClassUnterminatedComment)
				}
				p.pos += 2 + end + 2
			default:
				return p.at(p.pos, ClassUnexpectedChar)
			}
			p.comments = append(p.comments, lspan{start, p.pos})
		default:
			return nil
		}
	}
	return nil
}

// str parses a string token into a value node (span includes the quotes).
func (p *lparser) str() (*lvalue, error) {
	start := p.pos
	q := &parser{data: p.data, pos: p.pos}
	s, err := q.str()
	if err != nil {
		return nil, err
	}
	p.pos = q.pos
	return &lvalue{kind: doc.String, span: lspan{start, p.pos}, str: s}, nil
}

// any parses one value.
func (p *lparser) any(depth int) (*lvalue, error) {
	if p.pos >= len(p.data) {
		return nil, p.at(p.pos, ClassUnexpectedEOF)
	}
	start := p.pos
	switch c := p.data[p.pos]; {
	case c == '{':
		return p.object(depth + 1)
	case c == '[':
		return p.array(depth + 1)
	case c == '"':
		return p.str()
	case c == 't' || c == 'f' || c == 'n':
		word, kind := "true", doc.Bool
		switch {
		case c == 'f':
			word = "false"
		case c == 'n':
			word, kind = "null", doc.Null
		}
		q := &parser{data: p.data, pos: p.pos}
		if err := q.literal(word); err != nil {
			return nil, err
		}
		p.pos = q.pos
		return &lvalue{kind: kind, span: lspan{start, p.pos}}, nil
	case c == '-' || (c >= '0' && c <= '9'):
		q := &parser{data: p.data, pos: p.pos}
		nv, err := q.number()
		if err != nil {
			return nil, err
		}
		p.pos = q.pos
		return &lvalue{kind: doc.Number, span: lspan{start, p.pos}, str: nv.N}, nil
	}
	return nil, p.at(p.pos, ClassUnexpectedChar)
}

// object parses an object, recording each member's key, value, colon and comma.
func (p *lparser) object(depth int) (*lvalue, error) {
	if depth > MaxDepth {
		return nil, p.at(p.pos, ClassTooDeep)
	}
	open := p.pos
	p.pos++ // '{'
	v := &lvalue{kind: doc.Object, open: open}
	for {
		if err := p.ws(); err != nil {
			return nil, err
		}
		if p.pos >= len(p.data) {
			return nil, p.at(p.pos, ClassUnexpectedEOF)
		}
		if p.data[p.pos] == '}' {
			p.pos++
			v.close = p.pos - 1
			v.span = lspan{open, p.pos}
			return v, nil
		}
		key, err := p.str()
		if err != nil {
			return nil, err
		}
		if err := p.ws(); err != nil {
			return nil, err
		}
		if p.pos >= len(p.data) || p.data[p.pos] != ':' {
			return nil, p.at(p.pos, ClassUnexpectedChar)
		}
		colon := p.pos
		p.pos++
		if err := p.ws(); err != nil {
			return nil, err
		}
		val, err := p.any(depth)
		if err != nil {
			return nil, err
		}
		v.slots = append(v.slots, lslot{key: key, value: val, colon: colon, comma: -1})
		if err := p.ws(); err != nil {
			return nil, err
		}
		if p.pos >= len(p.data) {
			return nil, p.at(p.pos, ClassUnexpectedEOF)
		}
		switch p.data[p.pos] {
		case ',':
			v.slots[len(v.slots)-1].comma = p.pos
			p.pos++
		case '}':
			p.pos++
			v.close = p.pos - 1
			v.span = lspan{open, p.pos}
			return v, nil
		default:
			return nil, p.at(p.pos, ClassUnexpectedChar)
		}
	}
}

// array parses an array, recording each element and its comma.
func (p *lparser) array(depth int) (*lvalue, error) {
	if depth > MaxDepth {
		return nil, p.at(p.pos, ClassTooDeep)
	}
	open := p.pos
	p.pos++ // '['
	v := &lvalue{kind: doc.Array, open: open}
	for {
		if err := p.ws(); err != nil {
			return nil, err
		}
		if p.pos >= len(p.data) {
			return nil, p.at(p.pos, ClassUnexpectedEOF)
		}
		if p.data[p.pos] == ']' {
			p.pos++
			v.close = p.pos - 1
			v.span = lspan{open, p.pos}
			return v, nil
		}
		val, err := p.any(depth)
		if err != nil {
			return nil, err
		}
		v.slots = append(v.slots, lslot{value: val, colon: -1, comma: -1})
		if err := p.ws(); err != nil {
			return nil, err
		}
		if p.pos >= len(p.data) {
			return nil, p.at(p.pos, ClassUnexpectedEOF)
		}
		switch p.data[p.pos] {
		case ',':
			v.slots[len(v.slots)-1].comma = p.pos
			p.pos++
		case ']':
			p.pos++
			v.close = p.pos - 1
			v.span = lspan{open, p.pos}
			return v, nil
		default:
			return nil, p.at(p.pos, ClassUnexpectedChar)
		}
	}
}

// ---- edit engine ----

// splice replaces original[start:end] with text (an insertion has start==end).
type splice struct {
	start, end int
	text       string
	seq        int
}

type editor struct {
	data []byte
	nl   string
	sps  []splice
	seq  int
}

func (e *editor) replace(start, end int, text string) {
	e.seq++
	e.sps = append(e.sps, splice{start, end, text, e.seq})
}

func (e *editor) insert(at int, text string) { e.replace(at, at, text) }

// value walks the lossless value n (whose text parses to o) toward v.
func (e *editor) value(n *lvalue, o, v *doc.Value) error {
	if equalDocs(o, v) {
		return nil
	}
	if o.Kind != v.Kind {
		e.replace(n.span.start, n.span.end, render(v))
		return nil
	}
	switch o.Kind {
	case doc.Object:
		return e.object(n, o, v)
	case doc.Array:
		return e.array(n, o, v)
	}
	e.replace(n.span.start, n.span.end, render(v))
	return nil
}

// object edits one object container: member removals, value edits, and member
// additions appended before the closing brace.
func (e *editor) object(n *lvalue, o, v *doc.Value) error {
	removed := map[int]bool{}
	claimed := map[int]bool{}
	var removals []int
	for i, s := range n.slots {
		if v.Get(s.keyText()) == nil {
			removed[i] = true
			removals = append(removals, i)
		}
	}
	for _, i := range removals {
		e.removeSlot(n, i, claimed)
	}
	for i, s := range n.slots {
		if removed[i] {
			continue
		}
		if err := e.value(s.value, o.M[s.keyText()], v.Get(s.keyText())); err != nil {
			return err
		}
	}
	var items []string
	for _, k := range v.Keys {
		if o.Get(k) == nil {
			items = append(items, e.renderMember(k, v.M[k], n))
		}
	}
	if len(items) > 0 {
		e.insertEnd(n, items, removed, claimed)
	}
	return nil
}

// array edits one array container: common prefixes and suffixes stay put,
// equal-length middles are edited element-wise, and unequal middles anchor
// equal elements in order, pair the rest in place, remove surplus old elements
// anywhere, and append surplus new elements at the end only.
func (e *editor) array(n *lvalue, o, v *doc.Value) error {
	lo, lv := len(o.A), len(v.A)
	p := 0
	for p < lo && p < lv && equalDocs(o.A[p], v.A[p]) {
		p++
	}
	s := 0
	for s < lo-p && s < lv-p && equalDocs(o.A[lo-1-s], v.A[lv-1-s]) {
		s++
	}
	om, vm := o.A[p:lo-s], v.A[p:lv-s]
	if len(om) == len(vm) {
		for i := range om {
			if err := e.value(n.slots[p+i].value, om[i], vm[i]); err != nil {
				return err
			}
		}
		return nil
	}
	type anchor struct{ o, v int }
	var anchors []anchor
	oi := 0
	for vi := 0; vi < len(vm); vi++ {
		found := -1
		for k := oi; k < len(om); k++ {
			if equalDocs(om[k], vm[vi]) {
				found = k
				break
			}
		}
		if found >= 0 {
			anchors = append(anchors, anchor{found, vi})
			oi = found + 1
		}
	}
	removed := map[int]bool{}
	claimed := map[int]bool{}
	var removals []int
	var appends []*doc.Value
	appendsFinal := true
	prevO, prevV := 0, 0
	for ai := 0; ai <= len(anchors); ai++ {
		endO, endV := len(om), len(vm)
		if ai < len(anchors) {
			endO, endV = anchors[ai].o, anchors[ai].v
		}
		segO, segV := om[prevO:endO], vm[prevV:endV]
		nSeg := len(segO)
		if len(segV) < nSeg {
			nSeg = len(segV)
		}
		for i := 0; i < nSeg; i++ {
			if err := e.value(n.slots[p+prevO+i].value, segO[i], segV[i]); err != nil {
				return err
			}
		}
		for i := nSeg; i < len(segO); i++ {
			abs := p + prevO + i
			removed[abs] = true
			removals = append(removals, abs)
		}
		for i := nSeg; i < len(segV); i++ {
			appends = append(appends, segV[i])
			if ai != len(anchors) {
				appendsFinal = false
			}
		}
		prevO, prevV = endO+1, endV+1
	}
	// Appends are representable only at the very end of the array.
	if len(appends) > 0 && (s != 0 || !appendsFinal) {
		return ErrUnrepresentable
	}
	for _, i := range removals {
		e.removeSlot(n, i, claimed)
	}
	if len(appends) > 0 {
		items := make([]string, len(appends))
		for i, a := range appends {
			items[i] = render(a)
		}
		e.insertEnd(n, items, removed, claimed)
	}
	return nil
}

// removeSlot deletes one member or element: the key-to-value span plus one
// separator comma (its own, or the previous slot's). A comma is deleted at
// most once across all removals in a container.
func (e *editor) removeSlot(n *lvalue, i int, claimed map[int]bool) {
	s := n.slots[i]
	start := s.value.span.start
	if s.key != nil {
		start = s.key.span.start
	}
	e.replace(start, s.value.span.end, "")
	comma := s.comma
	if comma >= 0 && claimed[comma] {
		comma = -1
	}
	if comma < 0 && i > 0 {
		if c := n.slots[i-1].comma; c >= 0 && !claimed[c] {
			comma = c
		}
	}
	if comma >= 0 {
		claimed[comma] = true
		e.replace(comma, comma+1, "")
	}
}

// insertEnd appends rendered members or elements before the closing
// delimiter, matching the container's indentation and separator style and
// reusing a surviving separator comma instead of emitting a new one.
func (e *editor) insertEnd(n *lvalue, items []string, removed map[int]bool, claimed map[int]bool) {
	last := -1
	for i := range n.slots {
		if !removed[i] {
			last = i
		}
	}
	if last >= 0 {
		commaAt := n.slots[last].value.span.end
		needComma := true
		for i := range n.slots {
			if c := n.slots[i].comma; c >= 0 && !claimed[c] && c >= commaAt && c < n.close {
				needComma = false
			}
		}
		if needComma {
			e.insert(commaAt, ",")
		}
	}
	e.insertClose(n, items)
}

// insertClose places the rendered items immediately before the closing
// delimiter, matching the container's layout.
func (e *editor) insertClose(n *lvalue, items []string) {
	data := e.data
	multi := bytes.IndexByte(data[n.open+1:n.close], '\n') >= 0
	if multi {
		ins := n.close
		j := n.close
		for j > n.open+1 && (data[j-1] == ' ' || data[j-1] == '\t') {
			j--
		}
		if j > 0 && data[j-1] == '\n' {
			ins = j
		}
		var b strings.Builder
		if ins > 0 && data[ins-1] != '\n' {
			b.WriteString(e.nl)
		}
		indent := memberIndent(data, n)
		for i, it := range items {
			b.WriteString(indent)
			b.WriteString(it)
			if i < len(items)-1 {
				b.WriteString(",")
			}
			b.WriteString(e.nl)
		}
		e.insert(ins, b.String())
		return
	}
	ins := n.close
	spaced := ins > 0 && (data[ins-1] == ' ' || data[ins-1] == '\t')
	if n.open+1 < n.close && (data[n.open+1] == ' ' || data[n.open+1] == '\t') {
		spaced = true
	}
	for _, s := range n.slots {
		if s.comma >= 0 && s.comma+1 < len(data) && (data[s.comma+1] == ' ' || data[s.comma+1] == '\t') {
			spaced = true
		}
	}
	lead, tail := "", ""
	if spaced {
		prev := byte(' ')
		if ins > 0 {
			prev = data[ins-1]
		}
		if prev != ' ' && prev != '\t' && prev != '\n' && prev != ',' {
			lead = " "
		}
		if prev == ' ' || prev == '\t' {
			tail = " "
		}
	}
	sep := ","
	if spaced {
		sep = ", "
	}
	e.insert(ins, lead+strings.Join(items, sep)+tail)
}

// memberIndent returns the indentation the container's siblings use.
func memberIndent(data []byte, n *lvalue) string {
	closeIndent := ""
	j := n.close
	for j > n.open+1 && (data[j-1] == ' ' || data[j-1] == '\t') {
		j--
	}
	if j > 0 && data[j-1] == '\n' {
		closeIndent = string(data[j:n.close])
	}
	if len(n.slots) > 0 {
		s := n.slots[len(n.slots)-1]
		k := s.value.span.start
		if s.key != nil {
			k = s.key.span.start
		}
		j := k
		for j > 0 && data[j-1] != '\n' {
			j--
		}
		if ind := data[j:k]; len(ind) > 0 && bytes.IndexFunc(ind, func(r rune) bool { return r != ' ' && r != '\t' }) < 0 {
			return string(ind)
		}
	}
	return closeIndent + "  "
}

// renderMember renders one object member in the container's colon style.
func (e *editor) renderMember(key string, v *doc.Value, n *lvalue) string {
	colon := ": "
	if len(n.slots) > 0 {
		if c := n.slots[len(n.slots)-1].colon; c >= 0 && c+1 < len(e.data) && e.data[c+1] != ' ' && e.data[c+1] != '\t' {
			colon = ":"
		}
	}
	return string(doc.Canonical(doc.NewString(key))) + colon + render(v)
}

// render renders a value for insertion: leaves keep their stored spelling,
// containers are compact canonical JSON.
func render(v *doc.Value) string {
	if v.IsLeaf() {
		return string(doc.Canonical(v))
	}
	return string(doc.MarshalCompact(v))
}

// apply splices the collected edits into the original text, verifying that no
// splice overlaps another or cuts a comment in half.
func (e *editor) apply(tree *ltree) ([]byte, error) {
	sps := append([]splice(nil), e.sps...)
	sortSplices(sps)
	for i := 1; i < len(sps); i++ {
		a, b := sps[i-1], sps[i]
		if a.end > a.start && b.end > b.start && b.start < a.end {
			return nil, ErrUnrepresentable
		}
	}
	for _, c := range tree.comments {
		for _, s := range sps {
			if s.end <= s.start {
				continue
			}
			if (c.start < s.start && s.start < c.end) || (c.start < s.end && s.end < c.end) {
				return nil, ErrUnrepresentable
			}
		}
	}
	out := make([]byte, 0, len(e.data)+64)
	pos := 0
	for _, s := range sps {
		if s.start < pos {
			return nil, ErrUnrepresentable
		}
		out = append(out, e.data[pos:s.start]...)
		out = append(out, s.text...)
		pos = s.end
	}
	out = append(out, e.data[pos:]...)
	return out, nil
}

// sortSplices orders splices by start ascending, insertions before deletions
// at the same start, keeping the scheduled order for equal splices.
func sortSplices(sps []splice) {
	less := func(a, b splice) bool {
		if a.start != b.start {
			return a.start < b.start
		}
		ai, bi := a.end <= a.start, b.end <= b.start
		if ai != bi {
			return ai
		}
		return a.seq < b.seq
	}
	for i := 1; i < len(sps); i++ {
		for j := i; j > 0 && less(sps[j], sps[j-1]); j-- {
			sps[j-1], sps[j] = sps[j], sps[j-1]
		}
	}
}
