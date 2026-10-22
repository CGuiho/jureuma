package cli

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/CGuiho/zeze/internal/keystore"
)

const testKeyID = "k_0123456789abcdef0123456789abcdef"

// testEntry builds a stored-key entry for rendering and picker tests.
func testEntry(keyID, label, publicKey string, retired bool) keystore.Entry {
	return keystore.Entry{KeyID: keyID, Label: label, PublicKey: publicKey, Retired: retired}
}

func TestShortKeyID(t *testing.T) {
	cases := []struct {
		id   string
		want string
	}{
		{testKeyID, "k_0123456789ab"},
		{"k_0123", "k_0123"},
		{"", ""},
	}
	for _, c := range cases {
		if got := shortKeyID(c.id); got != c.want {
			t.Errorf("shortKeyID(%q) = %q, want %q", c.id, got, c.want)
		}
	}
}

func TestShortPublicKey(t *testing.T) {
	full := "zeze-pk-1:" + strings.Repeat("A", 43)
	if got := shortPublicKey(full); got != "zeze-pk-1:"+strings.Repeat("A", shortKeyRunes) {
		t.Errorf("shortPublicKey = %q", got)
	}
	if got := shortPublicKey("zeze-pk-1:short"); got != "zeze-pk-1:short" {
		t.Errorf("shortPublicKey(short) = %q", got)
	}
	if got := shortPublicKey(""); got != noLabel {
		t.Errorf("shortPublicKey(\"\") = %q, want placeholder", got)
	}
}

func TestRenderKeyLines(t *testing.T) {
	entries := []keystore.Entry{
		testEntry("k_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "zeze", "zeze-pk-1:"+strings.Repeat("A", 43), false),
		testEntry("k_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "", "zeze-pk-1:"+strings.Repeat("B", 43), true),
	}
	lines := renderKeyLines(entries)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	wantActive := "zeze  k_aaaaaaaaaaaa  zeze-pk-1:" + strings.Repeat("A", shortKeyRunes) + "  active"
	wantRetired := "—" + strings.Repeat(" ", len("zeze")-1) + "  k_bbbbbbbbbbbb  zeze-pk-1:" + strings.Repeat("B", shortKeyRunes) + "  retired"
	if lines[0] != wantActive {
		t.Errorf("active line = %q\nwant        %q", lines[0], wantActive)
	}
	if lines[1] != wantRetired {
		t.Errorf("retired line = %q\nwant         %q", lines[1], wantRetired)
	}
}

func TestRenderKeyLinesCollapsesLabelWhitespace(t *testing.T) {
	entries := []keystore.Entry{
		testEntry(testKeyID, "two\nwords\t here", "zeze-pk-1:"+strings.Repeat("A", 43), false),
	}
	lines := renderKeyLines(entries)
	if len(lines) != 1 || strings.ContainsAny(lines[0], "\n\t") {
		t.Fatalf("label breaks the one-line format: %q", lines)
	}
	if !strings.HasPrefix(lines[0], "two words here ") {
		t.Fatalf("label not collapsed: %q", lines[0])
	}
}

// panicReader fails the test if the picker reads it.
type panicReader struct{}

func (panicReader) Read([]byte) (int, error) { panic("picker read stdin outside a TTY") }

func TestPickKeyNonTTYNeverReads(t *testing.T) {
	entries := []keystore.Entry{testEntry(testKeyID, "zeze", "zeze-pk-1:"+strings.Repeat("A", 43), false)}
	var out bytes.Buffer
	got, err := pickKey(entries, panicReader{}, &out, false)
	if !errors.Is(err, ErrNoInteractiveSelection) {
		t.Fatalf("err = %v, want ErrNoInteractiveSelection", err)
	}
	if got.KeyID != "" {
		t.Fatalf("non-TTY pick returned %q", got.KeyID)
	}
	if out.Len() != 0 {
		t.Fatalf("non-TTY pick wrote output: %q", out.String())
	}
}

func TestPickKeyNoEntries(t *testing.T) {
	var out bytes.Buffer
	if _, err := pickKey(nil, panicReader{}, &out, true); !errors.Is(err, ErrNoStoredKeys) {
		t.Fatalf("err = %v, want ErrNoStoredKeys", err)
	}
}

func TestPickKeyTTYSelections(t *testing.T) {
	entries := []keystore.Entry{
		testEntry("k_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "one", "zeze-pk-1:"+strings.Repeat("A", 43), false),
		testEntry("k_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "two", "zeze-pk-1:"+strings.Repeat("B", 43), true),
	}
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"by number", "2\n", entries[1].KeyID},
		{"by key ID", "k_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n", entries[0].KeyID},
		{"by short key ID", "k_bbbbbbbbbbbb\n", entries[1].KeyID},
		{"reprompts on garbage", "x\n1\n", entries[0].KeyID},
		{"reprompts on out-of-range", "9\n1\n", entries[0].KeyID},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var out bytes.Buffer
			got, err := pickKey(entries, strings.NewReader(c.input), &out, true)
			if err != nil {
				t.Fatal(err)
			}
			if got.KeyID != c.want {
				t.Fatalf("picked %s, want %s", got.KeyID, c.want)
			}
			if !strings.Contains(out.String(), "Which one do you want to use?") {
				t.Fatalf("prompt missing: %q", out.String())
			}
		})
	}
}

func TestPickKeyCancel(t *testing.T) {
	entries := []keystore.Entry{testEntry(testKeyID, "one", "zeze-pk-1:"+strings.Repeat("A", 43), false)}
	for name, input := range map[string]string{"empty line": "\n", "eof": ""} {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			if _, err := pickKey(entries, strings.NewReader(input), &out, true); !errors.Is(err, ErrNoKeySelected) {
				t.Fatalf("err = %v, want ErrNoKeySelected", err)
			}
		})
	}
}

// syntheticEntry builds the picker's synthetic "create a new key" entry: an
// action, not a stored key, so it carries a label and no key ID.
func syntheticEntry(label string) keystore.Entry {
	return keystore.Entry{Label: label}
}

// defaultEntries lists two stored keys plus the synthetic default entry.
func defaultEntries() []keystore.Entry {
	return []keystore.Entry{
		testEntry("k_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "one", "zeze-pk-1:"+strings.Repeat("A", 43), false),
		testEntry("k_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "two", "zeze-pk-1:"+strings.Repeat("B", 43), true),
		syntheticEntry("create a new key"),
	}
}

func TestRenderKeyLinesWithDefaultMarksSyntheticDefault(t *testing.T) {
	entries := defaultEntries()
	lines := renderKeyLinesWithDefault(entries, 2)
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}
	def := lines[2]
	if def != "create a new key (default)" {
		t.Fatalf("default line = %q, want %q", def, "create a new key (default)")
	}
	for _, unwanted := range []string{"—", "active", "retired", "k_", "zeze-pk-1:"} {
		if strings.Contains(def, unwanted) {
			t.Errorf("default line renders %q as a key column: %q", unwanted, def)
		}
	}
	// Real entries keep the current columns (label column padded to the
	// widest label in the listing) and gain no marker.
	width := len("create a new key")
	wantOne := "one" + strings.Repeat(" ", width-len("one")) + "  k_aaaaaaaaaaaa  zeze-pk-1:" + strings.Repeat("A", shortKeyRunes) + "  active"
	wantTwo := "two" + strings.Repeat(" ", width-len("two")) + "  k_bbbbbbbbbbbb  zeze-pk-1:" + strings.Repeat("B", shortKeyRunes) + "  retired"
	if lines[0] != wantOne {
		t.Errorf("real line = %q\nwant     %q", lines[0], wantOne)
	}
	if lines[1] != wantTwo {
		t.Errorf("real line = %q\nwant     %q", lines[1], wantTwo)
	}
}

func TestRenderKeyLinesWithDefaultNoDefaultMarksNothing(t *testing.T) {
	lines := renderKeyLinesWithDefault([]keystore.Entry{syntheticEntry("create a new key")}, -1)
	if len(lines) != 1 || lines[0] != "create a new key" {
		t.Fatalf("lines = %q, want the plain label", lines)
	}
}

func TestPickKeyDefaultEnterSelectsDefault(t *testing.T) {
	entries := defaultEntries()
	for name, input := range map[string]string{"enter": "\n", "whitespace-only line": " \t\n"} {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			got, err := pickKeyWithDefault(entries, strings.NewReader(input), &out, true, 2)
			if err != nil {
				t.Fatal(err)
			}
			if got.KeyID != "" || got.Label != "create a new key" {
				t.Fatalf("picked %#v, want the synthetic default entry", got)
			}
			if strings.Contains(out.String(), "enter a number") {
				t.Fatalf("Enter printed the number hint: %q", out.String())
			}
			if !strings.Contains(out.String(), "Which one do you want to use? (Enter = create a new key) ") {
				t.Fatalf("prompt does not name the default: %q", out.String())
			}
			if !strings.Contains(out.String(), "create a new key (default)") {
				t.Fatalf("listing does not mark the default: %q", out.String())
			}
		})
	}
}

func TestPickKeyDefaultEOFCancels(t *testing.T) {
	var out bytes.Buffer
	if _, err := pickKeyWithDefault(defaultEntries(), strings.NewReader(""), &out, true, 2); !errors.Is(err, ErrNoKeySelected) {
		t.Fatalf("err = %v, want ErrNoKeySelected", err)
	}
}

func TestPickKeyDefaultSelectionsUnchanged(t *testing.T) {
	entries := defaultEntries()
	cases := []struct {
		name  string
		input string
		want  keystore.Entry
	}{
		{"by number", "1\n", entries[0]},
		{"number selects the default entry", "3\n", entries[2]},
		{"by key ID", "k_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\n", entries[1]},
		{"by short key ID", "k_aaaaaaaaaaaa\n", entries[0]},
		{"reprompts on garbage", "x\n2\n", entries[1]},
		{"reprompts on out-of-range", "9\n1\n", entries[0]},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var out bytes.Buffer
			got, err := pickKeyWithDefault(entries, strings.NewReader(c.input), &out, true, 2)
			if err != nil {
				t.Fatal(err)
			}
			if got.KeyID != c.want.KeyID || got.Label != c.want.Label {
				t.Fatalf("picked %q (%s), want %q (%s)", got.Label, got.KeyID, c.want.Label, c.want.KeyID)
			}
		})
	}
}

func TestPickKeyDefaultNonTTYNeverReads(t *testing.T) {
	var out bytes.Buffer
	got, err := pickKeyWithDefault(defaultEntries(), panicReader{}, &out, false, 2)
	if !errors.Is(err, ErrNoInteractiveSelection) {
		t.Fatalf("err = %v, want ErrNoInteractiveSelection", err)
	}
	if got.KeyID != "" || out.Len() != 0 {
		t.Fatalf("non-TTY pick returned %q and wrote %q", got.KeyID, out.String())
	}
}

func TestPickKeyDefaultNoEntries(t *testing.T) {
	var out bytes.Buffer
	if _, err := pickKeyWithDefault(nil, panicReader{}, &out, true, 2); !errors.Is(err, ErrNoStoredKeys) {
		t.Fatalf("err = %v, want ErrNoStoredKeys", err)
	}
}

func TestPickKeyNoDefaultEmptyLineReprompts(t *testing.T) {
	// Without a default the empty line keeps today's behavior: the number
	// hint, a re-prompt, and cancellation only at EOF.
	entries := []keystore.Entry{testEntry(testKeyID, "one", "zeze-pk-1:"+strings.Repeat("A", 43), false)}
	for name, pick := range map[string]func([]keystore.Entry, io.Reader, io.Writer, bool) (keystore.Entry, error){
		"pickKey": pickKey,
		"out-of-range index": func(en []keystore.Entry, in io.Reader, out io.Writer, tty bool) (keystore.Entry, error) {
			return pickKeyWithDefault(en, in, out, tty, 99)
		},
	} {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			if _, err := pick(entries, strings.NewReader("\n"), &out, true); !errors.Is(err, ErrNoKeySelected) {
				t.Fatalf("err = %v, want ErrNoKeySelected", err)
			}
			s := out.String()
			if !strings.Contains(s, "enter a number between 1 and 1, or a key ID") {
				t.Fatalf("number hint missing: %q", s)
			}
			if strings.Contains(s, "(Enter =") {
				t.Fatalf("prompt names a default without one: %q", s)
			}
		})
	}
}

func TestDefaultKeyLabel(t *testing.T) {
	cases := []struct {
		name   string
		origin string
		root   string
		want   string
	}{
		{"https remote", "https://github.com/CGuiho/zeze.git", "/work/zeze", "zeze"},
		{"https remote without .git", "https://github.com/CGuiho/zeze", "/work/zeze", "zeze"},
		{"scp remote", "git@github.com:CGuiho/zeze.git", "/work/zeze", "zeze"},
		{"scp remote without owner", "git@github.com:zeze.git", "/work/zeze", "zeze"},
		{"ssh remote", "ssh://git@github.com/CGuiho/zeze.git", "/work/zeze", "zeze"},
		{"trailing slash", "https://github.com/CGuiho/zeze.git/", "/work/zeze", "zeze"},
		{"path remote", "/srv/git/zeze.git", "/work/zeze", "zeze"},
		{"remote without repository path", "https://github.com", "/work/my-project", "my-project"},
		{"no remote", "", "/work/my-project", "my-project"},
		{"project root name", "", "/", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := defaultKeyLabel(c.root, func(string) string { return c.origin })
			if got != c.want {
				t.Fatalf("defaultKeyLabel = %q, want %q", got, c.want)
			}
		})
	}
	// A nil origin function must fall back, never panic.
	if got := defaultKeyLabel("/work/my-project", nil); got != "my-project" {
		t.Fatalf("defaultKeyLabel(nil origin) = %q", got)
	}
}

func TestGitOriginURLNeverFails(t *testing.T) {
	// Outside any repository this returns "" instead of an error or a panic.
	if got := gitOriginURL(t.TempDir()); got != "" && strings.ContainsAny(got, "\n\r") {
		t.Fatalf("gitOriginURL returned a multi-line value: %q", got)
	}
}
