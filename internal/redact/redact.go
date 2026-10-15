// Package redact replaces secrets in a byte stream with "[REDACTED]".
//
// The Writer is streaming and boundary-safe: a secret split across any number
// of Write calls is still replaced. At each position the longest secret that
// matches wins; matches never overlap and are found left to right, so the
// output equals a leftmost-longest non-overlapping replacement of the whole
// stream. At most maxLen-1 bytes (maxLen being the longest secret) are held
// back between writes; Close flushes them.
package redact

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strings"
	"sync"
)

// Placeholder replaces every secret occurrence.
const Placeholder = "[REDACTED]"

// ErrClosed is returned by Write after Close.
var ErrClosed = errors.New("redact: write after close")

// node is a byte trie node.
type node struct {
	next     map[byte]*node
	terminal bool
}

// Writer wraps an io.Writer and redacts secrets from everything written.
// It is safe for concurrent use. Close does not close the wrapped writer.
type Writer struct {
	mu     sync.Mutex
	w      io.Writer
	root   *node
	first  [256]bool
	maxLen int
	buf    []byte // held-back bytes, always shorter than maxLen after a Write
	err    error  // sticky error from w
	closed bool
}

// NewWriter wraps w and replaces every occurrence of any secret with
// "[REDACTED]". Empty secrets are ignored; duplicates are harmless.
func NewWriter(w io.Writer, secrets [][]byte) *Writer {
	r := &Writer{w: w, root: &node{}}
	for _, s := range secrets {
		if len(s) == 0 {
			continue
		}
		n := r.root
		for _, b := range s {
			if n.next == nil {
				n.next = make(map[byte]*node)
			}
			c := n.next[b]
			if c == nil {
				c = &node{}
				n.next[b] = c
			}
			n = c
		}
		n.terminal = true
		r.first[s[0]] = true
		if len(s) > r.maxLen {
			r.maxLen = len(s)
		}
	}
	return r
}

// Write redacts p and forwards what can already be decided. It reports
// len(p) on success, including bytes that are held back.
func (r *Writer) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return 0, ErrClosed
	}
	if r.err != nil {
		return 0, r.err
	}
	if len(p) == 0 {
		return 0, nil
	}
	if r.maxLen == 0 {
		if _, err := r.w.Write(p); err != nil {
			r.err = err
			return 0, err
		}
		return len(p), nil
	}
	r.buf = append(r.buf, p...)
	if err := r.process(false); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Close flushes held-back bytes (redacting any complete secret among them).
// It does not close the wrapped writer. Calling Close again is a no-op.
func (r *Writer) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return r.err
	}
	r.closed = true
	if r.err != nil {
		r.buf = nil
		return r.err
	}
	if len(r.buf) == 0 {
		return nil
	}
	return r.process(true)
}

// process scans r.buf, writes everything decidable, and keeps the undecided
// tail in r.buf. With final set, nothing is held back.
func (r *Writer) process(final bool) error {
	buf := r.buf
	out := make([]byte, 0, len(buf)+len(Placeholder))
	i := 0
	for i < len(buf) {
		if !r.first[buf[i]] {
			// Fast path: copy the run of bytes that cannot start a secret.
			j := i + 1
			for j < len(buf) && !r.first[buf[j]] {
				j++
			}
			out = append(out, buf[i:j]...)
			i = j
			continue
		}
		matchLen, undecided := r.matchAt(buf[i:], final)
		if undecided {
			break
		}
		if matchLen > 0 {
			out = append(out, Placeholder...)
			i += matchLen
		} else {
			out = append(out, buf[i])
			i++
		}
	}
	// Keep the undecided tail in a fresh slice so the held-back bytes do not
	// pin a large buffer and are not aliased by out.
	rest := len(buf) - i
	if rest > 0 {
		tail := make([]byte, rest)
		copy(tail, buf[i:])
		clear(buf)
		r.buf = tail
	} else {
		clear(buf)
		r.buf = r.buf[:0]
	}
	if len(out) > 0 {
		_, err := r.w.Write(out)
		clear(out)
		if err != nil {
			r.err = err
			return err
		}
	}
	return nil
}

// matchAt returns the length of the longest secret that is a prefix of s.
// undecided is true when s ends while a longer secret could still match and
// more input may arrive (final is false).
func (r *Writer) matchAt(s []byte, final bool) (matchLen int, undecided bool) {
	n := r.root
	for k := 0; ; k++ {
		if n.terminal {
			matchLen = k
		}
		if len(n.next) == 0 {
			return matchLen, false
		}
		if k == len(s) {
			if final {
				return matchLen, false
			}
			return 0, true
		}
		c := n.next[s[k]]
		if c == nil {
			return matchLen, false
		}
		n = c
	}
}

// Variants expands one secret into the forms to redact: raw, base64 std,
// base64 url (padded and raw), URL query-escaped, and JSON-escaped (without
// quotes). It also includes unpadded standard base64, URL path escaping, the
// JSON escape without HTML escaping, and the \ufffd-escaped form of both JSON
// escapes when they differ. The result has no
// duplicates and no empty strings; an empty secret yields nil.
func Variants(secret string) []string {
	if secret == "" {
		return nil
	}
	b := []byte(secret)
	escHTML := jsonEscape(secret, true)
	escPlain := jsonEscape(secret, false)
	cands := []string{
		secret,
		base64.StdEncoding.EncodeToString(b),
		base64.RawStdEncoding.EncodeToString(b),
		base64.URLEncoding.EncodeToString(b),
		base64.RawURLEncoding.EncodeToString(b),
		url.QueryEscape(secret),
		url.PathEscape(secret),
		escHTML,
		escPlain,
		// encoding/json writes a literal U+FFFD for invalid UTF-8 while other
		// encoders write the \ufffd escape; redact both representations.
		strings.ReplaceAll(escHTML, "\uFFFD", `\ufffd`),
		strings.ReplaceAll(escPlain, "\uFFFD", `\ufffd`),
	}
	seen := make(map[string]bool, len(cands))
	out := make([]string, 0, len(cands))
	for _, c := range cands {
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	return out
}

// jsonEscape returns the JSON string encoding of s without the surrounding
// quotes. Invalid UTF-8 is encoded as encoding/json does (U+FFFD).
func jsonEscape(s string, escapeHTML bool) string {
	var sb strings.Builder
	enc := json.NewEncoder(&sb)
	enc.SetEscapeHTML(escapeHTML)
	if err := enc.Encode(s); err != nil {
		return ""
	}
	e := strings.TrimSuffix(sb.String(), "\n")
	if len(e) < 2 {
		return ""
	}
	return e[1 : len(e)-1]
}
