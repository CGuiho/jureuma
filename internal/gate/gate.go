// Package gate implements zeze's human-only gate for protected operations
// and the append-only audit log.
//
// A protected operation proceeds only when standard input and standard error
// are terminals, no known agent marker is present in the environment, and the
// user types a randomly generated confirmation word. The gate is a guard
// against accidental exposure, not a security boundary against a same-user
// process.
package gate

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"strings"

	"golang.org/x/term"
)

// ErrRefused is returned (wrapped with a reason) when any gate check fails.
var ErrRefused = errors.New("protected operation refused")

// DefaultMarkers are the environment variable names that identify an AI agent
// or agent sandbox. A marker is present when its variable is set to a
// non-empty value.
var DefaultMarkers = []string{"CLAUDECODE", "CLAUDE_CODE_ENTRYPOINT", "CODEX_SANDBOX", "CODEX_HOME", "CURSOR_AGENT", "CURSOR_TRACE_ID", "GEMINI_CLI", "AIDER", "OPENCODE", "AMP_AGENT", "CLINE", "GOOSE_SESSION", "WINDSURF_AGENT", "COPILOT_AGENT", "ZEZE_AGENT"}

// Refusal reasons. They name the failed check only, never any input.
const (
	ReasonNotTerminal  = "not an interactive terminal"
	ReasonAgent        = "agent environment detected"
	ReasonNoAnswer     = "no confirmation received"
	ReasonWrongAnswer  = "confirmation word did not match"
	ReasonInvalidState = "gate is not configured"
)

// maxAnswerLen bounds how much of a single answer line is read.
const maxAnswerLen = 256

// Gate decides whether a protected operation may run.
type Gate struct {
	Stdin, Stderr *os.File
	Getenv        func(string) string
	ExtraMarkers  []string
	Prompt        io.Reader // defaults to Stdin
	Out           io.Writer // defaults to Stderr
	IsTerminal    func(*os.File) bool
	Word          func() string // random confirmation word; injectable for tests
}

// RefusedError carries the reason a gate check failed. It wraps ErrRefused.
type RefusedError struct {
	Reason string
	// Marker is the name (never the value) of the agent marker that was found,
	// when Reason is ReasonAgent.
	Marker string
}

func (e *RefusedError) Error() string { return ErrRefused.Error() + ": " + e.Reason }

// Unwrap lets errors.Is(err, ErrRefused) succeed.
func (e *RefusedError) Unwrap() error { return ErrRefused }

func refused(reason string) error { return &RefusedError{Reason: reason} }

// New returns a Gate wired to the process's stdin, stderr and environment.
func New(extraMarkers []string) *Gate {
	return &Gate{
		Stdin:        os.Stdin,
		Stderr:       os.Stderr,
		Getenv:       os.Getenv,
		ExtraMarkers: extraMarkers,
	}
}

// Check runs the three checks and asks the user to type the confirmation
// word describing action (e.g. "reveal 1 value"). Returns ErrRefused with a
// reason when any check fails.
func (g *Gate) Check(action string) error {
	if g == nil {
		return refused(ReasonInvalidState)
	}
	isTerm := g.IsTerminal
	if isTerm == nil {
		isTerm = IsTerminal
	}
	// 1. stdin and stderr must both be terminals.
	if g.Stdin == nil || g.Stderr == nil || !isTerm(g.Stdin) || !isTerm(g.Stderr) {
		return refused(ReasonNotTerminal)
	}
	// 2. no agent marker.
	getenv := g.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	if m, found := DetectAgent(getenv, g.ExtraMarkers); found {
		return &RefusedError{Reason: ReasonAgent, Marker: m}
	}
	// 3. confirmation word.
	in := g.Prompt
	if in == nil {
		in = g.Stdin
	}
	out := g.Out
	if out == nil {
		out = g.Stderr
	}
	wordFn := g.Word
	if wordFn == nil {
		wordFn = RandomWord
	}
	word := wordFn()
	if word == "" {
		return refused(ReasonInvalidState)
	}
	action = sanitizeAction(action)
	if _, err := fmt.Fprintf(out, "type %q to %s: ", word, action); err != nil {
		return refused(ReasonNotTerminal)
	}
	answer, ok := readLine(in)
	if !ok {
		return refused(ReasonNoAnswer)
	}
	if strings.TrimSpace(answer) != word {
		return refused(ReasonWrongAnswer)
	}
	return nil
}

// sanitizeAction removes control characters so the prompt stays one line.
func sanitizeAction(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if s == "" {
		s = "continue"
	}
	return s
}

// readLine reads one line from r a byte at a time so it never consumes input
// beyond the newline. It reports false on EOF before any newline-terminated or
// partial content, or on a read error.
func readLine(r io.Reader) (string, bool) {
	var sb strings.Builder
	buf := make([]byte, 1)
	for sb.Len() < maxAnswerLen {
		n, err := r.Read(buf)
		if n == 1 {
			if buf[0] == '\n' {
				return sb.String(), true
			}
			sb.WriteByte(buf[0])
			continue
		}
		if err != nil {
			if errors.Is(err, io.EOF) && sb.Len() > 0 {
				return sb.String(), true
			}
			return "", false
		}
	}
	// Overlong answer: it cannot match a confirmation word.
	return sb.String(), true
}

// IsTerminal reports whether f is a real terminal. It uses the terminal
// ioctl (TCGETS/TIOCGETA) on Unix and GetConsoleMode on Windows, so other
// character devices such as /dev/null do not count.
func IsTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// DetectAgent returns the first agent marker (from DefaultMarkers, then extra)
// whose environment variable is set to a non-empty value. Only the marker
// name is returned, never its value.
func DetectAgent(getenv func(string) string, extra []string) (marker string, found bool) {
	all := DetectAgents(getenv, extra)
	if len(all) == 0 {
		return "", false
	}
	return all[0], true
}

// DetectAgents returns the names of every agent marker present, in
// DefaultMarkers order followed by extra, without duplicates. It is suitable
// for AuditEvent.Markers.
func DetectAgents(getenv func(string) string, extra []string) []string {
	if getenv == nil {
		getenv = os.Getenv
	}
	var out []string
	seen := make(map[string]bool)
	check := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		if getenv(name) != "" {
			out = append(out, name)
		}
	}
	for _, m := range DefaultMarkers {
		check(m)
	}
	for _, m := range extra {
		check(m)
	}
	return out
}

// words is the confirmation word list: short, unambiguous, lowercase.
var words = []string{
	"amber", "anchor", "apple", "aspen", "badger", "basil", "beacon", "birch",
	"bison", "bramble", "breeze", "brook", "cactus", "candle", "canyon", "cedar",
	"cherry", "cinder", "clover", "cobalt", "comet", "copper", "coral", "cotton",
	"crane", "crystal", "daisy", "delta", "desert", "dolphin", "dune", "eagle",
	"ember", "falcon", "fern", "fjord", "flint", "forest", "garnet", "glacier",
	"granite", "harbor", "hazel", "heron", "hollow", "indigo", "island", "ivory",
	"jasmine", "juniper", "kettle", "lagoon", "lantern", "lemon", "lilac", "linen",
	"lotus", "maple", "marble", "meadow", "mango", "meteor", "mint", "nectar",
	"nutmeg", "oasis", "ocean", "olive", "onyx", "orchid", "otter", "pebble",
	"pepper", "pine", "plum", "prairie", "quartz", "raven", "reef", "river",
	"robin", "saffron", "salmon", "sierra", "silver", "spruce", "summit", "sunset",
	"thistle", "thunder", "timber", "topaz", "tulip", "tundra", "valley", "velvet",
	"violet", "walnut", "willow", "winter", "zephyr",
}

// RandomWord returns a confirmation word such as "maple-17": a word from the
// list followed by two random digits, chosen with crypto/rand.
func RandomWord() string {
	w := words[randInt(len(words))]
	return fmt.Sprintf("%s-%02d", w, randInt(100))
}

func randInt(n int) int {
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		// Since Go 1.24 crypto/rand never fails on supported platforms.
		panic("gate: crypto/rand unavailable")
	}
	return int(v.Int64())
}
