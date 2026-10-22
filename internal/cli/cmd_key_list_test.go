package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/CGuiho/zeze/internal/crypt"
	"github.com/CGuiho/zeze/internal/keystore"
	"github.com/CGuiho/zeze/internal/pair"
)

// keyHarness runs the CLI in-process in a throw-away user home and project.
// It is self-contained on purpose: the shared harness in cli_test.go belongs
// to a later unit.
type keyHarness struct {
	t     *testing.T
	home  string
	proj  string
	store *keystore.Store
}

func newKeyHarness(t *testing.T) *keyHarness {
	t.Helper()
	h := &keyHarness{t: t, home: t.TempDir(), proj: t.TempDir()}
	t.Setenv("ZEZE_USER_HOME", h.home)
	t.Setenv("HOME", h.home)
	t.Setenv("USERPROFILE", h.home)
	t.Setenv("ZEZE_NO_UPDATE_CHECK", "1")
	t.Setenv("ZEZE_PRIVATE_KEY", "")
	h.store = keystore.New(filepath.Join(h.home, ".guiho", "zeze"))
	t.Chdir(h.proj)
	return h
}

// git runs git in the project and skips the test when git is unavailable.
func (h *keyHarness) git(args ...string) {
	h.t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		h.t.Skip("git is required")
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = h.proj
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+filepath.Join(h.home, ".gitconfig"))
	if out, err := cmd.CombinedOutput(); err != nil {
		h.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func (h *keyHarness) run(args ...string) (int, string) {
	h.t.Helper()
	var out bytes.Buffer
	code := ExecuteIO("1.2.3", args, strings.NewReader(""), &out, &out)
	return code, out.String()
}

func (h *keyHarness) expect(code int, args ...string) string {
	h.t.Helper()
	got, out := h.run(args...)
	if got != code {
		h.t.Fatalf("zeze %s: exit %d, want %d\n%s", strings.Join(args, " "), got, code, out)
	}
	return out
}

// mustKey generates a key pair and returns it with its key ID.
func (h *keyHarness) mustKey() (crypt.PrivateKey, string) {
	h.t.Helper()
	sk, err := crypt.GenerateKey()
	if err != nil {
		h.t.Fatal(err)
	}
	return sk, crypt.KeyID(sk.Public())
}

// keyLine matches one `key list` line: label, short key ID, public-key
// prefix and state.
var keyLine = regexp.MustCompile(`^\S.*  k_[0-9a-f]{12}  zeze-pk-1:[A-Za-z0-9_-]{12}  (active|retired)$`)

func assertNoKeyMaterial(t *testing.T, where, data string) {
	t.Helper()
	if strings.Contains(data, crypt.PrivateKeyPrefix) {
		t.Fatalf("%s leaks private key material:\n%s", where, data)
	}
}

func TestKeyListOutputShape(t *testing.T) {
	h := newKeyHarness(t)
	labeled, labeledID := h.mustKey()
	legacy, legacyID := h.mustKey()
	retired, retiredID := h.mustKey()
	if _, err := h.store.SaveLabeled(labeled, "zeze"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.store.Save(legacy); err != nil {
		t.Fatal(err)
	}
	if _, err := h.store.SaveLabeled(retired, "old-project"); err != nil {
		t.Fatal(err)
	}
	if err := h.store.Retire(retiredID); err != nil {
		t.Fatal(err)
	}
	out := h.expect(0, "key", "list")
	assertNoKeyMaterial(t, "key list", out)
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3:\n%s", len(lines), out)
	}
	for _, line := range lines {
		if !keyLine.MatchString(line) {
			t.Fatalf("line does not match the key-list shape: %q", line)
		}
	}
	lineFor := func(keyID string) string {
		for _, line := range lines {
			if strings.Contains(line, shortKeyID(keyID)) {
				return line
			}
		}
		t.Fatalf("no line for key %s:\n%s", keyID, out)
		return ""
	}
	// Active keys first (sorted by key ID), retired key last.
	active := []string{labeledID, legacyID}
	if active[0] > active[1] {
		active[0], active[1] = active[1], active[0]
	}
	if !strings.Contains(lines[0], shortKeyID(active[0])) || !strings.Contains(lines[1], shortKeyID(active[1])) {
		t.Fatalf("active keys are not sorted by key ID:\n%s", out)
	}
	if !strings.Contains(lines[2], shortKeyID(retiredID)) {
		t.Fatalf("retired key not listed last:\n%s", out)
	}
	// Label, placeholder and state per row.
	rows := []struct {
		keyID, label, state string
	}{
		{labeledID, "zeze", "active"},
		{legacyID, "—", "active"},
		{retiredID, "old-project", "retired"},
	}
	for _, r := range rows {
		line := lineFor(r.keyID)
		if !strings.HasPrefix(line, r.label) {
			t.Fatalf("row for %s lacks label %q:\n%s", r.keyID, r.label, line)
		}
		if !strings.HasSuffix(line, r.state) {
			t.Fatalf("row for %s lacks state %q:\n%s", r.keyID, r.state, line)
		}
	}
	// The public-key prefix of every key is shown.
	for _, pk := range []crypt.PublicKey{labeled.Public(), legacy.Public(), retired.Public()} {
		prefix := pk.String()
		if len(prefix) > len(crypt.PublicKeyPrefix)+shortKeyRunes {
			prefix = prefix[:len(crypt.PublicKeyPrefix)+shortKeyRunes]
		}
		if !strings.Contains(out, prefix) {
			t.Fatalf("key list lacks public-key prefix %s:\n%s", prefix, out)
		}
	}
}

func TestKeyListEmptyStore(t *testing.T) {
	h := newKeyHarness(t)
	if out := h.expect(0, "key", "list"); out != "" {
		t.Fatalf("empty store printed %q", out)
	}
}

func TestKeyListHelp(t *testing.T) {
	h := newKeyHarness(t)
	out := h.expect(0, "key", "list", "--help")
	if !strings.Contains(out, "key list") {
		t.Fatalf("help does not name the command:\n%s", out)
	}
	if code, _ := h.run("key", "list", "extra-arg"); code != 2 {
		t.Fatal("key list must take no arguments")
	}
}

func TestKeyShowShowsLabel(t *testing.T) {
	h := newKeyHarness(t)
	sk, id := h.mustKey()
	if _, err := h.store.SaveLabeled(sk, "zeze"); err != nil {
		t.Fatal(err)
	}
	if err := pair.WriteEncrypted(filepath.Join(h.proj, "config.encrypted.json"), pair.NewEncrypted(sk.Public(), "")); err != nil {
		t.Fatal(err)
	}
	out := h.expect(0, "key", "show")
	assertNoKeyMaterial(t, "key show", out)
	if !strings.Contains(out, "keyId:      "+id) {
		t.Fatalf("key show lacks the key ID:\n%s", out)
	}
	if !strings.Contains(out, "label:      zeze") {
		t.Fatalf("key show lacks the label:\n%s", out)
	}
}

func TestKeyShowLabelPlaceholder(t *testing.T) {
	h := newKeyHarness(t)
	sk, _ := h.mustKey()
	if _, err := h.store.Save(sk); err != nil {
		t.Fatal(err)
	}
	if err := pair.WriteEncrypted(filepath.Join(h.proj, "config.encrypted.json"), pair.NewEncrypted(sk.Public(), "")); err != nil {
		t.Fatal(err)
	}
	out := h.expect(0, "key", "show")
	assertNoKeyMaterial(t, "key show", out)
	if !strings.Contains(out, "label:      "+noLabel) {
		t.Fatalf("key show lacks the label placeholder:\n%s", out)
	}
}

func TestKeyRotateDerivesLabel(t *testing.T) {
	h := newKeyHarness(t)
	h.git("init", "-q", ".")
	h.git("remote", "add", "origin", "https://github.com/CGuiho/w1-label-demo.git")
	sk, oldID := h.mustKey()
	if _, err := h.store.Save(sk); err != nil {
		t.Fatal(err)
	}
	if err := pair.WriteEncrypted(filepath.Join(h.proj, "config.encrypted.json"), pair.NewEncrypted(sk.Public(), "")); err != nil {
		t.Fatal(err)
	}
	out := h.expect(0, "key", "rotate")
	assertNoKeyMaterial(t, "key rotate", out)
	if !strings.Contains(out, "the new key is labeled w1-label-demo") {
		t.Fatalf("rotate output lacks the derived label:\n%s", out)
	}
	entries, err := h.store.List()
	if err != nil {
		t.Fatal(err)
	}
	var active, retired int
	for _, en := range entries {
		switch {
		case en.Retired:
			retired++
		default:
			active++
			if en.Label != "w1-label-demo" {
				t.Fatalf("new key label = %q, want w1-label-demo", en.Label)
			}
		}
	}
	if active != 1 || retired != 1 {
		t.Fatalf("after rotate: %d active, %d retired keys", active, retired)
	}
	if _, err := os.Stat(filepath.Join(h.home, ".guiho", "zeze", "keys", "retired", oldID+".json")); err != nil {
		t.Fatalf("old key not retired: %v", err)
	}
}
