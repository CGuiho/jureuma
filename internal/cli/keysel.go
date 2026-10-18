// Stored-key identity: labels, short-id / public-key-prefix rendering, the
// default label derivation, and the TTY-safe stored-key picker.
//
// Everything rendered here is non-secret metadata: labels, key IDs and the
// beginning of the public key. Private key material is never printed, logged
// or returned by any function in this file.
package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/CGuiho/jureuma/internal/crypt"
	"github.com/CGuiho/jureuma/internal/keystore"
)

const (
	// noLabel is shown where a stored key carries no label (legacy key files).
	noLabel = "—"
	// shortIDHex is how many hex characters of the key ID a listing shows
	// after the "k_" prefix.
	shortIDHex = 12
	// shortKeyRunes is how many base64url characters of the public key a
	// listing shows after the "jureuma-pk-1:" prefix.
	shortKeyRunes = 12
)

// ErrNoInteractiveSelection reports that no interactive selection is
// available because no human terminal is attached. Callers take an explicit
// selection (for example --key-id or --key-label) instead of prompting.
var ErrNoInteractiveSelection = errors.New("no interactive selection available")

// ErrNoKeySelected reports that the human cancelled a key selection.
var ErrNoKeySelected = errors.New("no key selected")

// ErrNoStoredKeys reports that there is nothing to choose from.
var ErrNoStoredKeys = errors.New("no stored keys")

// oneLine collapses whitespace so a label can never break the one-line-per-key
// listing format or inject extra output lines.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// keyLabel returns the label to display, or the no-label placeholder.
func keyLabel(label string) string {
	if l := oneLine(label); l != "" {
		return l
	}
	return noLabel
}

// shortKeyID renders the beginning of a key ID ("k_" plus the first
// shortIDHex hex characters). The key ID is public metadata.
func shortKeyID(id string) string {
	if len(id) <= 2+shortIDHex {
		return id
	}
	return id[:2+shortIDHex]
}

// shortPublicKey renders the beginning of a public key
// ("jureuma-pk-1:" plus the first shortKeyRunes base64url characters), or the
// no-label placeholder when the key file carried none.
func shortPublicKey(pk string) string {
	pk = strings.TrimSpace(pk)
	if pk == "" {
		return noLabel
	}
	if len(pk) > len(crypt.PublicKeyPrefix)+shortKeyRunes {
		return pk[:len(crypt.PublicKeyPrefix)+shortKeyRunes]
	}
	return pk
}

// renderKeyLines renders one line per stored key: label, short key ID,
// public-key prefix and state. The label column is padded to the widest label
// in the listing so the columns stay aligned. No entry is marked as the
// picker default.
func renderKeyLines(entries []keystore.Entry) []string {
	return renderKeyLinesWithDefault(entries, -1)
}

// renderKeyLinesWithDefault renders the picker listing like renderKeyLines
// and marks the entry at defaultIndex (a negative index marks none) as the
// picker default. A synthetic entry — an entry with no key ID, like "create a
// new key" — names an action, not a stored key: it renders as its label alone,
// without the empty key-ID and public-key columns or a key state.
func renderKeyLinesWithDefault(entries []keystore.Entry, defaultIndex int) []string {
	width := 0
	for _, en := range entries {
		if n := len(keyLabel(en.Label)); n > width {
			width = n
		}
	}
	out := make([]string, 0, len(entries))
	for i, en := range entries {
		line := ""
		switch {
		case en.KeyID == "":
			line = keyLabel(en.Label)
		default:
			state := "active"
			if en.Retired {
				state = "retired"
			}
			line = fmt.Sprintf("%-*s  %s  %s  %s", width, keyLabel(en.Label), shortKeyID(en.KeyID), shortPublicKey(en.PublicKey), state)
		}
		if i == defaultIndex {
			line += " (default)"
		}
		out = append(out, line)
	}
	return out
}

// stdinIsTTY reports whether a human terminal is attached to the process
// input. A character-device stdin is a terminal on Unix and Windows; no extra
// dependency is needed.
func stdinIsTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// pickKey asks a human to choose one of the stored keys and returns it. It
// prints the keys in the `key list` format and prompts "Which one do you want
// to use?"; the answer is a line number or a key ID. It has no default: an
// empty line re-prompts and a cancelled input ends the selection. It
// delegates to pickKeyWithDefault with no default, so its behavior is
// unchanged.
//
// in, out and isTTY are injected for tests; the CLI passes os.Stdin, the
// command's error stream and stdinIsTTY(). When isTTY is false it never reads
// in and returns ErrNoInteractiveSelection, so a non-interactive caller can
// never block on this helper.
func pickKey(entries []keystore.Entry, in io.Reader, out io.Writer, isTTY bool) (keystore.Entry, error) {
	return pickKeyWithDefault(entries, in, out, isTTY, -1)
}

// pickKeyWithDefault is pickKey with a default choice: the entry at
// defaultIndex (a 0-based index; any negative or out-of-range index means no
// default) that an empty line — Enter — selects immediately. The listing marks
// that entry as the default and the prompt names it ("Which one do you want to
// use? (Enter = create a new key) "). A line with no answer at all (EOF) still
// cancels with ErrNoKeySelected; any other line that names no entry re-prompts.
func pickKeyWithDefault(entries []keystore.Entry, in io.Reader, out io.Writer, isTTY bool, defaultIndex int) (keystore.Entry, error) {
	if len(entries) == 0 {
		return keystore.Entry{}, ErrNoStoredKeys
	}
	if !isTTY {
		return keystore.Entry{}, ErrNoInteractiveSelection
	}
	hasDefault := defaultIndex >= 0 && defaultIndex < len(entries)
	if !hasDefault {
		defaultIndex = -1
	}
	for _, line := range renderKeyLinesWithDefault(entries, defaultIndex) {
		fmt.Fprintln(out, line)
	}
	prompt := "Which one do you want to use? "
	if hasDefault {
		prompt = fmt.Sprintf("Which one do you want to use? (Enter = %s) ", keyLabel(entries[defaultIndex].Label))
	}
	r := bufio.NewReader(in)
	for {
		fmt.Fprint(out, prompt)
		line, err := r.ReadString('\n')
		if err != nil && line == "" {
			fmt.Fprintln(out)
			return keystore.Entry{}, ErrNoKeySelected
		}
		sel := strings.TrimSpace(line)
		if sel == "" && hasDefault {
			return entries[defaultIndex], nil
		}
		if en, ok := matchKeyChoice(entries, sel); ok {
			return en, nil
		}
		fmt.Fprintf(out, "enter a number between 1 and %d, or a key ID\n", len(entries))
	}
}

// matchKeyChoice resolves a typed selection: a 1-based line number, a full
// key ID or a short key ID as shown in the listing.
func matchKeyChoice(entries []keystore.Entry, sel string) (keystore.Entry, bool) {
	if sel == "" {
		return keystore.Entry{}, false
	}
	if n, err := strconv.Atoi(sel); err == nil {
		if n >= 1 && n <= len(entries) {
			return entries[n-1], true
		}
		return keystore.Entry{}, false
	}
	for _, en := range entries {
		if en.KeyID == sel || shortKeyID(en.KeyID) == sel {
			return en, true
		}
	}
	return keystore.Entry{}, false
}

// gitOriginURL returns the origin remote URL of the repository at root, or ""
// when git or the remote is unavailable. It only reads local git
// configuration: it never touches the network and never fails hard.
func gitOriginURL(root string) string {
	out, err := exec.Command("git", "-C", root, "remote", "get-url", "origin").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// defaultKeyLabel derives the label for a key created in the project at root:
// the repository name from the origin remote, falling back to the project
// directory name. origin is injectable for tests (the CLI passes
// gitOriginURL). It never fails hard; an empty result means no label.
func defaultKeyLabel(root string, origin func(root string) string) string {
	if origin != nil {
		if name := remoteRepoName(origin(root)); name != "" {
			return name
		}
	}
	return dirBaseName(root)
}

// remoteRepoName returns the repository name from a git remote URL: the last
// path (or scp-like "host:path") segment without the ".git" suffix. It
// returns "" when the remote carries no repository path.
func remoteRepoName(url string) string {
	s := strings.TrimSpace(url)
	if s == "" {
		return ""
	}
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimRight(s, "/\\")
	if i := strings.Index(s, "://"); i >= 0 {
		rest := s[i+3:]
		j := strings.IndexAny(rest, "/\\")
		if j < 0 {
			return ""
		}
		s = rest[j:]
	} else if i := strings.Index(s, ":"); i >= 0 && !strings.ContainsAny(s[:i], "/\\") {
		s = s[i+1:]
	}
	if i := strings.LastIndexAny(s, "/\\"); i >= 0 {
		s = s[i+1:]
	}
	s = strings.TrimSuffix(s, ".git")
	return oneLine(s)
}

// dirBaseName returns the base name of a directory, or "" for a filesystem
// root or an empty path.
func dirBaseName(dir string) string {
	dir = strings.TrimRight(dir, string(filepath.Separator))
	if dir == "" {
		return ""
	}
	base := filepath.Base(dir)
	if base == "." || base == string(filepath.Separator) {
		return ""
	}
	return oneLine(base)
}
