package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/CGuiho/jureuma/internal/keystore"
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
	full := "jureuma-pk-1:" + strings.Repeat("A", 43)
	if got := shortPublicKey(full); got != "jureuma-pk-1:"+strings.Repeat("A", shortKeyRunes) {
		t.Errorf("shortPublicKey = %q", got)
	}
	if got := shortPublicKey("jureuma-pk-1:short"); got != "jureuma-pk-1:short" {
		t.Errorf("shortPublicKey(short) = %q", got)
	}
	if got := shortPublicKey(""); got != noLabel {
		t.Errorf("shortPublicKey(\"\") = %q, want placeholder", got)
	}
}

func TestRenderKeyLines(t *testing.T) {
	entries := []keystore.Entry{
		testEntry("k_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "jureuma", "jureuma-pk-1:"+strings.Repeat("A", 43), false),
		testEntry("k_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "", "jureuma-pk-1:"+strings.Repeat("B", 43), true),
	}
	lines := renderKeyLines(entries)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	wantActive := "jureuma  k_aaaaaaaaaaaa  jureuma-pk-1:" + strings.Repeat("A", shortKeyRunes) + "  active"
	wantRetired := "—" + strings.Repeat(" ", len("jureuma")-1) + "  k_bbbbbbbbbbbb  jureuma-pk-1:" + strings.Repeat("B", shortKeyRunes) + "  retired"
	if lines[0] != wantActive {
		t.Errorf("active line = %q\nwant        %q", lines[0], wantActive)
	}
	if lines[1] != wantRetired {
		t.Errorf("retired line = %q\nwant         %q", lines[1], wantRetired)
	}
}

func TestRenderKeyLinesCollapsesLabelWhitespace(t *testing.T) {
	entries := []keystore.Entry{
		testEntry(testKeyID, "two\nwords\t here", "jureuma-pk-1:"+strings.Repeat("A", 43), false),
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
	entries := []keystore.Entry{testEntry(testKeyID, "jureuma", "jureuma-pk-1:"+strings.Repeat("A", 43), false)}
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
		testEntry("k_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "one", "jureuma-pk-1:"+strings.Repeat("A", 43), false),
		testEntry("k_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "two", "jureuma-pk-1:"+strings.Repeat("B", 43), true),
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
	entries := []keystore.Entry{testEntry(testKeyID, "one", "jureuma-pk-1:"+strings.Repeat("A", 43), false)}
	for name, input := range map[string]string{"empty line": "\n", "eof": ""} {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			if _, err := pickKey(entries, strings.NewReader(input), &out, true); !errors.Is(err, ErrNoKeySelected) {
				t.Fatalf("err = %v, want ErrNoKeySelected", err)
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
		{"https remote", "https://github.com/CGuiho/jureuma.git", "/work/jureuma", "jureuma"},
		{"https remote without .git", "https://github.com/CGuiho/jureuma", "/work/jureuma", "jureuma"},
		{"scp remote", "git@github.com:CGuiho/jureuma.git", "/work/jureuma", "jureuma"},
		{"scp remote without owner", "git@github.com:jureuma.git", "/work/jureuma", "jureuma"},
		{"ssh remote", "ssh://git@github.com/CGuiho/jureuma.git", "/work/jureuma", "jureuma"},
		{"trailing slash", "https://github.com/CGuiho/jureuma.git/", "/work/jureuma", "jureuma"},
		{"path remote", "/srv/git/jureuma.git", "/work/jureuma", "jureuma"},
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
