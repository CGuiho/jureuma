package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/CGuiho/jureuma/internal/crypt"
	"github.com/CGuiho/jureuma/internal/paths"
	"go.yaml.in/yaml/v3"
)

// BindProjectKey writes the top-level key binding of <root>/jureuma.yaml as
// exactly one spliced line: the value of the existing top-level key line is
// replaced (its spacing and any trailing comment kept byte for byte), or one
// line is appended when the document has no key entry. Every comment, blank
// line, ordering and quoting outside that one line stays byte-identical —
// SaveProject is never used here because it rewrites the whole document.
//
// The spliced result is re-parsed and re-validated before anything is
// written, and the written file is re-read and validated again. A file that
// is missing, does not parse, no longer validates, or whose binding cannot be
// represented as one line (a multi-line or block-scalar value, a quoted or
// aliased key) is refused with nothing written. A binding that already equals
// keyID leaves the file untouched. The result reports whether the file
// changed; errors carry key IDs and file names only, never configuration
// values.
func BindProjectKey(root string, keyID string) (bool, error) {
	if !crypt.ValidKeyID(keyID) {
		return false, fmt.Errorf("%s: invalid key ID; nothing was written", ProjectFile)
	}
	path := filepath.Join(root, ProjectFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, fmt.Errorf("%s: not found; nothing was written", ProjectFile)
		}
		return false, fmt.Errorf("%s: cannot read; nothing was written", ProjectFile)
	}
	p, err := ParseProject(data)
	if err != nil {
		return false, fmt.Errorf("%w; nothing was written", err)
	}
	if p.Key == keyID {
		// Already equal: never rewrite the file.
		return false, nil
	}
	out, err := spliceKeyLine(data, keyID)
	if err != nil {
		return false, err
	}
	if _, err := ParseProject(out); err != nil {
		return false, fmt.Errorf("%s: the key binding cannot be written without breaking the file (%v); nothing was written", ProjectFile, err)
	}
	perm := os.FileMode(0o644)
	if st, err := os.Stat(path); err == nil {
		perm = st.Mode().Perm()
	}
	if err := paths.AtomicWrite(path, out, perm); err != nil {
		return false, fmt.Errorf("%s: cannot write; nothing was written", ProjectFile)
	}
	if check, err := os.ReadFile(path); err != nil {
		return true, fmt.Errorf("%s: written but cannot be re-read", ProjectFile)
	} else if _, err := ParseProject(check); err != nil {
		return true, fmt.Errorf("%s: written but no longer validates (%v)", ProjectFile, err)
	}
	return true, nil
}

// srcLine is one line of a file with its exact terminator.
type srcLine struct{ text, term string }

// splitLines splits data into lines, keeping each line's "\n" or "\r\n"
// terminator so a splice never changes the file's line endings.
func splitLines(data []byte) []srcLine {
	var out []srcLine
	start := 0
	for i := 0; i < len(data); i++ {
		if data[i] != '\n' {
			continue
		}
		text, term := string(data[start:i]), "\n"
		if strings.HasSuffix(text, "\r") {
			text, term = strings.TrimSuffix(text, "\r"), "\r\n"
		}
		out = append(out, srcLine{text, term})
		start = i + 1
	}
	if start < len(data) {
		out = append(out, srcLine{text: string(data[start:])})
	}
	return out
}

func joinLines(lines []srcLine) []byte {
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(l.text)
		b.WriteString(l.term)
	}
	return []byte(b.String())
}

// keyLineRe matches the spelling of a top-level key line this writer can
// splice: the plain scalar key "key" at column 1. A quoted key or one that is
// not at the start of its line cannot be represented as one line and is
// refused.
var keyLineRe = regexp.MustCompile(`^key[ \t]*:`)

// spliceKeyLine returns data with its top-level key line (or one appended
// line) carrying keyID. It locates the key entry through the parsed document,
// so a line that merely looks like `key:` inside a multi-line scalar is never
// touched.
func spliceKeyLine(data []byte, keyID string) ([]byte, error) {
	var docNode yaml.Node
	if err := yaml.Unmarshal(data, &docNode); err != nil {
		return nil, fmt.Errorf("%s: does not parse; nothing was written", ProjectFile)
	}
	root := &docNode
	if docNode.Kind == yaml.DocumentNode {
		if len(docNode.Content) == 0 {
			root = nil // empty document: only an appended line can follow
		} else {
			root = docNode.Content[0]
		}
	}
	if root != nil && root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s: has no top-level mapping; nothing was written", ProjectFile)
	}
	if root == nil {
		return appendKeyLine(data, keyID), nil
	}
	var k, v *yaml.Node
	found := 0
	for i := 0; i+1 < len(root.Content); i += 2 {
		if kk := root.Content[i]; kk.Kind == yaml.ScalarNode && kk.Value == "key" {
			found++
			k, v = kk, root.Content[i+1]
		}
	}
	switch {
	case found > 1:
		return nil, fmt.Errorf("%s: has more than one top-level key line; nothing was written", ProjectFile)
	case found == 0:
		return appendKeyLine(data, keyID), nil
	}
	lines := splitLines(data)
	if k.Column != 1 || k.Line < 1 || k.Line > len(lines) {
		return nil, notOneLine()
	}
	l := lines[k.Line-1]
	if !keyLineRe.MatchString(l.text) || v.Kind != yaml.ScalarNode || v.Tag != "!!str" || v.Line != k.Line {
		return nil, notOneLine()
	}
	vStart := v.Column - 1
	if vStart < len("key:") || vStart > len(l.text) {
		return nil, notOneLine()
	}
	vEnd, ok := valueEnd(l.text, vStart)
	if !ok {
		return nil, notOneLine()
	}
	var got string
	if err := yaml.Unmarshal([]byte(l.text[vStart:vEnd]), &got); err != nil || got != v.Value {
		// The value does not end on this line (a folded continuation, a
		// block scalar): replacing one line would leave the rest behind.
		return nil, notOneLine()
	}
	lines[k.Line-1] = srcLine{text: l.text[:vStart] + keyID + l.text[vEnd:], term: l.term}
	return joinLines(lines), nil
}

func notOneLine() error {
	return fmt.Errorf("%s: the key binding is not one plain line and cannot be updated in place; nothing was written", ProjectFile)
}

// valueEnd returns the end of the single-line scalar starting at vStart:
// a quoted scalar runs to its closing quote, a plain one to the trailing
// comment (kept) or the end of the line. ok is false when the value has no
// closing quote on this line.
func valueEnd(text string, vStart int) (int, bool) {
	if vStart >= len(text) {
		return vStart, true
	}
	if q := text[vStart]; q == '\'' || q == '"' {
		for i := vStart + 1; i < len(text); i++ {
			switch {
			case q == '"' && text[i] == '\\':
				i++
			case text[i] == q:
				if q == '\'' && i+1 < len(text) && text[i+1] == '\'' {
					i++ // '' is one escaped quote
					continue
				}
				return i + 1, true
			}
		}
		return 0, false
	}
	for i := vStart; i < len(text); i++ {
		if text[i] == '#' && (i == vStart || text[i-1] == ' ' || text[i-1] == '\t') {
			return trimEnd(text, vStart, i), true
		}
	}
	return trimEnd(text, vStart, len(text)), true
}

func trimEnd(text string, lo, hi int) int {
	for hi > lo && (text[hi-1] == ' ' || text[hi-1] == '\t') {
		hi--
	}
	return hi
}

// appendKeyLine returns data with one key line appended on its own line,
// matching the file's line-ending style.
func appendKeyLine(data []byte, keyID string) []byte {
	out := append([]byte(nil), data...)
	term := "\n"
	if len(out) > 0 && out[len(out)-1] != '\n' {
		out = append(out, '\n')
	} else if len(out) >= 2 && out[len(out)-2] == '\r' {
		term = "\r\n"
	}
	return append(out, ("key: " + keyID + term)...)
}
