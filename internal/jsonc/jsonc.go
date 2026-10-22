// Package jsonc parses strict JSON (RFC 8259) and lenient JSONC (comments and
// trailing commas) into zeze's ordered doc.Value tree.
//
// Errors never contain source text: a SyntaxError carries only a position and
// a fixed error class, so configuration values cannot leak through messages.
package jsonc

import (
	"bytes"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/CGuiho/zeze/internal/doc"
)

// Error classes reported in SyntaxError.Class.
const (
	ClassUnexpectedChar      = "unexpected character"
	ClassUnexpectedEOF       = "unexpected end of input"
	ClassInvalidEscape       = "invalid escape"
	ClassInvalidNumber       = "invalid number"
	ClassDuplicateKey        = "duplicate key"
	ClassNULInString         = "NUL in string"
	ClassInvalidUTF8         = "invalid UTF-8"
	ClassUnterminatedComment = "unterminated comment"
	ClassTrailingContent     = "trailing content"
	ClassTooDeep             = "nesting too deep"
)

// MaxDepth is the maximum nesting depth of arrays and objects. Deeper input
// is rejected with ClassTooDeep instead of exhausting the stack.
const MaxDepth = 10000

// SyntaxError is the only error type Parse returns. Its Error() is
// "line L, column C: <Class>" and never includes source text.
// Line and Column are 1-based; Column counts characters (runes) on the line.
type SyntaxError struct {
	Line, Column int
	Class        string
}

func (e *SyntaxError) Error() string {
	return "line " + strconv.Itoa(e.Line) + ", column " + strconv.Itoa(e.Column) + ": " + e.Class
}

var bom = []byte{0xEF, 0xBB, 0xBF}

func stripBOM(data []byte) ([]byte, int) {
	if len(data) >= 3 && data[0] == bom[0] && data[1] == bom[1] && data[2] == bom[2] {
		return data[3:], 3
	}
	return data, 0
}

// Parse parses JSON (strict=true: RFC 8259 only) or JSONC (strict=false:
// // and /* */ comments, trailing commas in objects and arrays) into a doc.Value.
// Duplicate object keys, strings containing NUL (U+0000), invalid UTF-8,
// and trailing non-whitespace content are errors. A leading UTF-8 BOM is skipped.
func Parse(data []byte, strict bool) (*doc.Value, error) {
	body, skip := stripBOM(data)
	p := &parser{data: data, pos: skip, strict: strict}
	if !utf8.Valid(body) {
		off := skip
		for off < len(data) {
			r, n := utf8.DecodeRune(data[off:])
			if r == utf8.RuneError && n <= 1 {
				break
			}
			off += n
		}
		return nil, p.errAt(off, ClassInvalidUTF8)
	}
	if err := p.skipSpace(); err != nil {
		return nil, err
	}
	v, err := p.value(0)
	if err != nil {
		return nil, err
	}
	if err := p.skipSpace(); err != nil {
		return nil, err
	}
	if p.pos < len(p.data) {
		return nil, p.errAt(p.pos, ClassTrailingContent)
	}
	return v, nil
}

// HasComments reports whether JSONC data contains at least one comment
// ("//" or "/*" outside a string). It does not require data to be valid.
func HasComments(data []byte) bool {
	data, _ = stripBOM(data)
	inStr := false
	for i := 0; i < len(data); i++ {
		c := data[i]
		if inStr {
			switch c {
			case '\\':
				i++
			case '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '/':
			if i+1 < len(data) && (data[i+1] == '/' || data[i+1] == '*') {
				return true
			}
		}
	}
	return false
}

type parser struct {
	data   []byte
	pos    int
	strict bool
}

// errAt builds a SyntaxError for byte offset off. A BOM is not counted as a column.
func (p *parser) errAt(off int, class string) *SyntaxError {
	if off > len(p.data) {
		off = len(p.data)
	}
	start := 0
	if _, skip := stripBOM(p.data); skip > 0 && off >= skip {
		start = skip
	}
	line, col := 1, 1
	for i := start; i < off; {
		c := p.data[i]
		switch {
		case c == '\n':
			line, col = line+1, 1
			i++
		case c == '\r':
			line, col = line+1, 1
			i++
			if i < off && p.data[i] == '\n' {
				i++
			}
		default:
			_, n := utf8.DecodeRune(p.data[i:off])
			col++
			i += n
		}
	}
	return &SyntaxError{Line: line, Column: col, Class: class}
}

func (p *parser) eof() *SyntaxError { return p.errAt(len(p.data), ClassUnexpectedEOF) }

// unexpected reports the current position: end of input or an unexpected character.
func (p *parser) unexpected() *SyntaxError {
	if p.pos >= len(p.data) {
		return p.eof()
	}
	return p.errAt(p.pos, ClassUnexpectedChar)
}

// skipSpace skips JSON whitespace and, in lenient mode, comments.
func (p *parser) skipSpace() error {
	for p.pos < len(p.data) {
		switch p.data[p.pos] {
		case ' ', '\t', '\n', '\r':
			p.pos++
		case '/':
			if p.strict || p.pos+1 >= len(p.data) {
				return p.errAt(p.pos, ClassUnexpectedChar)
			}
			switch p.data[p.pos+1] {
			case '/':
				p.pos += 2
				for p.pos < len(p.data) && p.data[p.pos] != '\n' && p.data[p.pos] != '\r' {
					p.pos++
				}
			case '*':
				start := p.pos
				end := bytes.Index(p.data[p.pos+2:], []byte("*/"))
				if end < 0 {
					return p.errAt(start, ClassUnterminatedComment)
				}
				p.pos += 2 + end + 2
			default:
				return p.errAt(p.pos, ClassUnexpectedChar)
			}
		default:
			return nil
		}
	}
	return nil
}

func (p *parser) value(depth int) (*doc.Value, error) {
	if p.pos >= len(p.data) {
		return nil, p.eof()
	}
	switch c := p.data[p.pos]; {
	case c == '{':
		return p.object(depth + 1)
	case c == '[':
		return p.array(depth + 1)
	case c == '"':
		s, err := p.str()
		if err != nil {
			return nil, err
		}
		return doc.NewString(s), nil
	case c == 't':
		return doc.NewBool(true), p.literal("true")
	case c == 'f':
		return doc.NewBool(false), p.literal("false")
	case c == 'n':
		return doc.NewNull(), p.literal("null")
	case c == '-' || (c >= '0' && c <= '9'):
		return p.number()
	}
	return nil, p.errAt(p.pos, ClassUnexpectedChar)
}

func (p *parser) literal(word string) error {
	for i := 0; i < len(word); i++ {
		if p.pos >= len(p.data) {
			return p.eof()
		}
		if p.data[p.pos] != word[i] {
			return p.errAt(p.pos, ClassUnexpectedChar)
		}
		p.pos++
	}
	return nil
}

func isNumByte(c byte) bool {
	return (c >= '0' && c <= '9') || c == '-' || c == '+' || c == '.' || c == 'e' || c == 'E'
}

func (p *parser) number() (*doc.Value, error) {
	start := p.pos
	for p.pos < len(p.data) && isNumByte(p.data[p.pos]) {
		p.pos++
	}
	text := string(p.data[start:p.pos])
	if !doc.ValidNumber(text) {
		return nil, p.errAt(start, ClassInvalidNumber)
	}
	return &doc.Value{Kind: doc.Number, N: text}, nil
}

func hexVal(c byte) rune {
	switch {
	case c >= '0' && c <= '9':
		return rune(c - '0')
	case c >= 'a' && c <= 'f':
		return rune(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return rune(c-'A') + 10
	}
	return -1
}

// hex4 reads 4 hex digits at off. ok is false on a bad digit; eof is true when
// input ends first.
func (p *parser) hex4(off int) (r rune, ok, eof bool) {
	if off+4 > len(p.data) {
		for i := off; i < len(p.data); i++ {
			if hexVal(p.data[i]) < 0 {
				return 0, false, false
			}
		}
		return 0, false, true
	}
	for i := off; i < off+4; i++ {
		h := hexVal(p.data[i])
		if h < 0 {
			return 0, false, false
		}
		r = r<<4 | h
	}
	return r, true, false
}

// str parses a string starting at the opening quote.
func (p *parser) str() (string, error) {
	p.pos++ // opening quote
	start := p.pos
	// Fast path: no escapes.
	for p.pos < len(p.data) {
		c := p.data[p.pos]
		if c == '"' {
			s := string(p.data[start:p.pos])
			p.pos++
			return s, nil
		}
		if c == '\\' || c < 0x20 {
			break
		}
		p.pos++
	}
	var b strings.Builder
	b.Write(p.data[start:p.pos])
	for {
		if p.pos >= len(p.data) {
			return "", p.eof()
		}
		c := p.data[p.pos]
		switch {
		case c == '"':
			p.pos++
			return b.String(), nil
		case c == 0:
			return "", p.errAt(p.pos, ClassNULInString)
		case c < 0x20:
			return "", p.errAt(p.pos, ClassUnexpectedChar)
		case c != '\\':
			b.WriteByte(c)
			p.pos++
			continue
		}
		esc := p.pos
		if p.pos+1 >= len(p.data) {
			return "", p.eof()
		}
		switch p.data[p.pos+1] {
		case '"':
			b.WriteByte('"')
		case '\\':
			b.WriteByte('\\')
		case '/':
			b.WriteByte('/')
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case 'u':
			r, ok, eof := p.hex4(p.pos + 2)
			if eof {
				return "", p.eof()
			}
			if !ok {
				return "", p.errAt(esc, ClassInvalidEscape)
			}
			p.pos += 6
			switch {
			case r == 0:
				return "", p.errAt(esc, ClassNULInString)
			case utf16.IsSurrogate(r):
				if r >= 0xDC00 { // lone low surrogate
					return "", p.errAt(esc, ClassInvalidEscape)
				}
				// A high surrogate must be followed by \u and a low surrogate.
				if p.pos >= len(p.data) || (p.data[p.pos] == '\\' && p.pos+1 >= len(p.data)) {
					return "", p.eof()
				}
				if p.data[p.pos] != '\\' || p.data[p.pos+1] != 'u' {
					return "", p.errAt(esc, ClassInvalidEscape)
				}
				r2, ok2, eof2 := p.hex4(p.pos + 2)
				if eof2 {
					return "", p.eof()
				}
				if !ok2 || r2 < 0xDC00 || r2 > 0xDFFF {
					return "", p.errAt(esc, ClassInvalidEscape)
				}
				p.pos += 6
				b.WriteRune(utf16.DecodeRune(r, r2))
			default:
				b.WriteRune(r)
			}
			continue
		default:
			return "", p.errAt(esc, ClassInvalidEscape)
		}
		p.pos += 2
	}
}

func (p *parser) object(depth int) (*doc.Value, error) {
	if depth > MaxDepth {
		return nil, p.errAt(p.pos, ClassTooDeep)
	}
	p.pos++ // '{'
	obj := doc.NewObject()
	for {
		if err := p.skipSpace(); err != nil {
			return nil, err
		}
		if p.pos >= len(p.data) {
			return nil, p.eof()
		}
		if p.data[p.pos] == '}' && (len(obj.Keys) == 0 || !p.strict) {
			p.pos++
			return obj, nil
		}
		if p.data[p.pos] != '"' {
			return nil, p.errAt(p.pos, ClassUnexpectedChar)
		}
		keyPos := p.pos
		key, err := p.str()
		if err != nil {
			return nil, err
		}
		if _, dup := obj.M[key]; dup {
			return nil, p.errAt(keyPos, ClassDuplicateKey)
		}
		if err := p.skipSpace(); err != nil {
			return nil, err
		}
		if p.pos >= len(p.data) || p.data[p.pos] != ':' {
			return nil, p.unexpected()
		}
		p.pos++
		if err := p.skipSpace(); err != nil {
			return nil, err
		}
		v, err := p.value(depth)
		if err != nil {
			return nil, err
		}
		obj.Put(key, v)
		if err := p.skipSpace(); err != nil {
			return nil, err
		}
		if p.pos >= len(p.data) {
			return nil, p.eof()
		}
		switch p.data[p.pos] {
		case ',':
			p.pos++
		case '}':
			p.pos++
			return obj, nil
		default:
			return nil, p.errAt(p.pos, ClassUnexpectedChar)
		}
	}
}

func (p *parser) array(depth int) (*doc.Value, error) {
	if depth > MaxDepth {
		return nil, p.errAt(p.pos, ClassTooDeep)
	}
	p.pos++ // '['
	arr := doc.NewArray()
	for {
		if err := p.skipSpace(); err != nil {
			return nil, err
		}
		if p.pos >= len(p.data) {
			return nil, p.eof()
		}
		if p.data[p.pos] == ']' && (len(arr.A) == 0 || !p.strict) {
			p.pos++
			return arr, nil
		}
		v, err := p.value(depth)
		if err != nil {
			return nil, err
		}
		arr.A = append(arr.A, v)
		if err := p.skipSpace(); err != nil {
			return nil, err
		}
		if p.pos >= len(p.data) {
			return nil, p.eof()
		}
		switch p.data[p.pos] {
		case ',':
			p.pos++
		case ']':
			p.pos++
			return arr, nil
		default:
			return nil, p.errAt(p.pos, ClassUnexpectedChar)
		}
	}
}
