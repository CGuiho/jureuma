package agentres

import (
	"bytes"
	"strings"
)

// Markers bounding jureuma's managed block in AGENTS.md. A marker is
// recognised only when it is alone on its line (surrounding spaces allowed).
const (
	BeginMarker = "<!-- >>> jureuma managed block >>> -->"
	EndMarker   = "<!-- <<< jureuma managed block <<< -->"
)

// span locates one complete managed block: start is the first byte of the
// begin-marker line, contentEnd the byte after the end marker's text, and
// lineEnd the byte after the end marker's line terminator (if any).
type span struct{ start, contentEnd, lineEnd int }

// findBlocks returns every complete block in data, in order. A begin marker
// pairs with the nearest following end marker; an orphan begin marker (one
// followed by another begin marker before any end marker) and an orphan end
// marker are ignored and left untouched as ordinary content.
func findBlocks(data []byte) []span {
	var out []span
	begin := -1
	pos := 0
	for pos < len(data) {
		lineStart := pos
		nl := bytes.IndexByte(data[pos:], '\n')
		var contentEnd, lineEnd int
		if nl < 0 {
			contentEnd, lineEnd = len(data), len(data)
		} else {
			lineEnd = pos + nl + 1
			contentEnd = pos + nl
			if contentEnd > lineStart && data[contentEnd-1] == '\r' {
				contentEnd--
			}
		}
		line := bytes.TrimSpace(data[lineStart:contentEnd])
		switch {
		case bytes.Equal(line, []byte(BeginMarker)):
			begin = lineStart
		case bytes.Equal(line, []byte(EndMarker)) && begin >= 0:
			// Keep any trailing spaces after the marker as part of the block
			// so a replacement does not leave them dangling.
			out = append(out, span{start: begin, contentEnd: contentEnd, lineEnd: lineEnd})
			begin = -1
		}
		pos = lineEnd
	}
	return out
}

// detectEOL returns "\r\n" when the first line terminator in data is CRLF,
// otherwise "\n".
func detectEOL(data []byte) string {
	i := bytes.IndexByte(data, '\n')
	if i > 0 && data[i-1] == '\r' {
		return "\r\n"
	}
	return "\n"
}

// normalizeBody converts line endings to LF and strips leading and trailing
// blank lines.
func normalizeBody(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.TrimRight(body, "\n\r\t ")
	body = strings.TrimLeft(body, "\n\r")
	return body
}

// renderBlock builds the complete block (markers included, no trailing line
// terminator) using eol as line terminator.
func renderBlock(body, eol string) []byte {
	b := normalizeBody(body)
	var sb strings.Builder
	sb.WriteString(BeginMarker)
	sb.WriteString(eol)
	if b != "" {
		sb.WriteString(strings.ReplaceAll(b, "\n", eol))
		sb.WriteString(eol)
	}
	sb.WriteString(EndMarker)
	return []byte(sb.String())
}

func endsWithBlankLine(b []byte) bool {
	return bytes.HasSuffix(b, []byte("\n\n")) || bytes.HasSuffix(b, []byte("\n\r\n"))
}

// ApplyBlock inserts the managed block with body into agentsMD, or replaces
// the existing one. Every byte outside the block is preserved. When no block
// exists, the block is appended after a blank line. Duplicate managed blocks
// after the first are removed. The block uses the file's line ending (CRLF
// when the first line ends in CRLF). The input slice is never modified.
func ApplyBlock(agentsMD []byte, body string) []byte {
	eol := detectEOL(agentsMD)
	block := renderBlock(body, eol)
	blocks := findBlocks(agentsMD)
	if len(blocks) == 0 {
		out := make([]byte, 0, len(agentsMD)+len(block)+3*len(eol))
		out = append(out, agentsMD...)
		if len(out) > 0 {
			if out[len(out)-1] != '\n' {
				out = append(out, eol...)
			}
			if !endsWithBlankLine(out) {
				out = append(out, eol...)
			}
		}
		out = append(out, block...)
		out = append(out, eol...)
		return out
	}
	out := make([]byte, 0, len(agentsMD)+len(block))
	first := blocks[0]
	out = append(out, agentsMD[:first.start]...)
	out = append(out, block...)
	prev := first.contentEnd
	for _, b := range blocks[1:] {
		out = append(out, agentsMD[prev:b.start]...)
		prev = b.lineEnd
	}
	out = append(out, agentsMD[prev:]...)
	return out
}

// RemoveBlock removes every managed block (with its line terminator) and
// reports whether one was found. Bytes outside the blocks are preserved,
// except that when the last block ends the file, the single blank-line
// separator that ApplyBlock inserted before it is removed too. AGENTS.md is
// never deleted by this package; an empty result is returned as empty.
func RemoveBlock(agentsMD []byte) ([]byte, bool) {
	blocks := findBlocks(agentsMD)
	if len(blocks) == 0 {
		return append([]byte(nil), agentsMD...), false
	}
	out := make([]byte, 0, len(agentsMD))
	prev := 0
	for _, b := range blocks {
		out = append(out, agentsMD[prev:b.start]...)
		prev = b.lineEnd
	}
	out = append(out, agentsMD[prev:]...)
	if blocks[len(blocks)-1].lineEnd == len(agentsMD) && endsWithBlankLine(out) {
		out = out[:len(out)-1]
		if len(out) > 0 && out[len(out)-1] == '\r' {
			out = out[:len(out)-1]
		}
	}
	return out, true
}

// HasCurrentBlock reports whether agentsMD contains exactly one managed block
// and that block is byte-identical to what ApplyBlock would write for body.
func HasCurrentBlock(agentsMD []byte, body string) bool {
	blocks := findBlocks(agentsMD)
	if len(blocks) != 1 {
		return false
	}
	b := blocks[0]
	return bytes.Equal(agentsMD[b.start:b.contentEnd], renderBlock(body, detectEOL(agentsMD)))
}
