package keystore

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/CGuiho/zeze/internal/crypt"
)

func newKey(t *testing.T) (crypt.PrivateKey, string) {
	t.Helper()
	sk, err := crypt.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	return sk, crypt.KeyID(sk.Public())
}

func newStore(t *testing.T, env map[string]string) *Store {
	t.Helper()
	home := t.TempDir()
	t.Setenv("ZEZE_USER_HOME", home)
	return &Store{Home: filepath.Join(home, ".guiho", "zeze"), Getenv: func(k string) string { return env[k] }}
}

func assertNoSecret(t *testing.T, err error, sk crypt.PrivateKey) {
	t.Helper()
	if err == nil {
		return
	}
	enc := strings.TrimPrefix(sk.String(), crypt.PrivateKeyPrefix)
	if strings.Contains(err.Error(), enc) || strings.Contains(err.Error(), enc[:10]) {
		t.Fatalf("error leaks key material: %v", err)
	}
}

func TestFindPrecedence(t *testing.T) {
	sk, id := newKey(t)
	cases := []struct {
		name    string
		env     map[string]string
		file    bool
		retired bool
		want    Source
	}{
		{"dedicated env wins", map[string]string{EnvName(id): sk.String(), EnvPrivateKey: sk.String()}, true, false, Source("env:" + EnvName(id))},
		{"generic env next", map[string]string{EnvPrivateKey: sk.String()}, true, false, "env:ZEZE_PRIVATE_KEY"},
		{"active file", nil, true, false, Source("file:keys/" + id + ".json")},
		{"retired file", nil, true, true, Source("file:keys/retired/" + id + ".json")},
		{"env with whitespace", map[string]string{EnvName(id): " " + sk.String() + "\n"}, false, false, Source("env:" + EnvName(id))},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newStore(t, c.env)
			if c.file {
				if _, err := s.Save(sk); err != nil {
					t.Fatal(err)
				}
				if c.retired {
					if err := s.Retire(id); err != nil {
						t.Fatal(err)
					}
				}
			}
			got, src, err := s.Find(id)
			if err != nil {
				t.Fatal(err)
			}
			if src != c.want || !crypt.SameKey(got, sk) {
				t.Fatalf("source %q, want %q", src, c.want)
			}
		})
	}
}

func TestGenericEnvSkippedWhenOtherKey(t *testing.T) {
	sk, id := newKey(t)
	other, _ := newKey(t)
	s := newStore(t, map[string]string{EnvPrivateKey: other.String()})
	if _, _, err := s.Find(id); !errors.Is(err, ErrNoKey) {
		t.Fatalf("want ErrNoKey, got %v", err)
	}
	if _, err := s.Save(sk); err != nil {
		t.Fatal(err)
	}
	got, src, err := s.Find(id)
	if err != nil || !crypt.SameKey(got, sk) || !strings.HasPrefix(string(src), "file:") {
		t.Fatalf("expected file key, got %q %v", src, err)
	}
}

func TestMalformedGenericEnvDeferred(t *testing.T) {
	sk, id := newKey(t)
	s := newStore(t, map[string]string{EnvPrivateKey: "garbage-secret-value"})
	_, _, err := s.Find(id)
	var ie *InvalidKeyError
	if !errors.As(err, &ie) || ie.Source != "env:ZEZE_PRIVATE_KEY" {
		t.Fatalf("want InvalidKeyError, got %v", err)
	}
	if strings.Contains(err.Error(), "garbage") {
		t.Fatal("error echoes env value")
	}
	if _, err := s.Save(sk); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Find(id); err != nil {
		t.Fatalf("file key should still be found: %v", err)
	}
}

func TestDedicatedEnvMismatch(t *testing.T) {
	_, id := newKey(t)
	other, _ := newKey(t)
	s := newStore(t, map[string]string{EnvName(id): other.String()})
	_, _, err := s.Find(id)
	var me *MismatchError
	if !errors.As(err, &me) || me.Source != Source("env:"+EnvName(id)) {
		t.Fatalf("want MismatchError, got %v", err)
	}
	if !strings.Contains(err.Error(), "key mismatch (source: env:"+EnvName(id)+")") {
		t.Fatalf("message %q", err)
	}
	assertNoSecret(t, err, other)

	s2 := newStore(t, map[string]string{EnvName(id): "nonsense"})
	if _, _, err := s2.Find(id); !errors.As(err, new(*InvalidKeyError)) || strings.Contains(err.Error(), "nonsense") {
		t.Fatalf("want InvalidKeyError, got %v", err)
	}
}

func TestFileMismatch(t *testing.T) {
	sk, id := newKey(t)
	other, otherID := newKey(t)
	s := newStore(t, nil)
	if _, err := s.Save(other); err != nil {
		t.Fatal(err)
	}
	// Place other's key file under sk's id.
	if err := os.Rename(filepath.Join(s.Home, "keys", otherID+".json"), filepath.Join(s.Home, "keys", id+".json")); err != nil {
		t.Fatal(err)
	}
	_, _, err := s.Find(id)
	var me *MismatchError
	if !errors.As(err, &me) || me.Source != Source("file:keys/"+id+".json") {
		t.Fatalf("want MismatchError, got %v", err)
	}
	assertNoSecret(t, err, other)
	assertNoSecret(t, err, sk)
}

func TestFilePublicKeyMismatch(t *testing.T) {
	sk, id := newKey(t)
	other, _ := newKey(t)
	s := newStore(t, nil)
	p, err := s.Save(sk)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	var kf keyFile
	json.Unmarshal(b, &kf)
	kf.PublicKey = other.Public().String()
	b, _ = json.Marshal(kf)
	os.WriteFile(p, b, 0o600)
	if _, _, err := s.Find(id); !errors.As(err, new(*MismatchError)) {
		t.Fatalf("want MismatchError, got %v", err)
	}
}

func TestInvalidKeyID(t *testing.T) {
	s := newStore(t, nil)
	for _, id := range []string{"", "../x", "k_../../../etc/passwd", "K_5C1E0A9F3B7D2E4C8A6F1B0D9E3C7A25"} {
		if _, _, err := s.Find(id); !errors.Is(err, ErrInvalidKeyID) {
			t.Fatalf("Find(%q): %v", id, err)
		}
		if err := s.Retire(id); !errors.Is(err, ErrInvalidKeyID) {
			t.Fatalf("Retire(%q): %v", id, err)
		}
		if err := s.Delete(id); !errors.Is(err, ErrInvalidKeyID) {
			t.Fatalf("Delete(%q): %v", id, err)
		}
	}
}

func TestSaveFormatAndMode(t *testing.T) {
	sk, id := newKey(t)
	s := newStore(t, nil)
	p, err := s.Save(sk)
	if err != nil {
		t.Fatal(err)
	}
	if p != filepath.Join(s.Home, "keys", id+".json") {
		t.Fatalf("path %s", p)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m["keyId"] != id || m["publicKey"] != sk.Public().String() || m["privateKey"] != sk.String() || m["created"] == "" {
		t.Fatal("bad key file fields")
	}
	if runtime.GOOS != "windows" {
		fi, _ := os.Stat(p)
		if fi.Mode().Perm() != 0o600 {
			t.Fatalf("mode %v", fi.Mode().Perm())
		}
		di, _ := os.Stat(filepath.Dir(p))
		if di.Mode().Perm() != 0o700 {
			t.Fatalf("dir mode %v", di.Mode().Perm())
		}
	}
	// Saving again is idempotent.
	if p2, err := s.Save(sk); err != nil || p2 != p {
		t.Fatal("second save")
	}
}

func TestRetireDeleteList(t *testing.T) {
	a, aid := newKey(t)
	b, bid := newKey(t)
	s := newStore(t, nil)
	if l, err := s.List(); err != nil || len(l) != 0 {
		t.Fatalf("empty list: %v %v", l, err)
	}
	s.Save(a)
	s.Save(b)
	// Noise that must be ignored.
	os.WriteFile(filepath.Join(s.Home, "keys", ".tmp-x.json"), nil, 0o600)
	os.WriteFile(filepath.Join(s.Home, "keys", "notes.txt"), nil, 0o600)

	if err := s.Delete(aid); !errors.Is(err, ErrNoKey) {
		t.Fatalf("delete active key must fail: %v", err)
	}
	if err := s.Retire(aid); err != nil {
		t.Fatal(err)
	}
	if err := s.Retire(aid); !errors.Is(err, ErrNoKey) {
		t.Fatalf("second retire: %v", err)
	}
	l, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(l) != 2 || l[0].KeyID != bid || l[0].Retired || l[1].KeyID != aid || !l[1].Retired {
		t.Fatalf("list %+v", l)
	}
	if _, src, err := s.Find(aid); err != nil || src != Source("file:keys/retired/"+aid+".json") {
		t.Fatalf("find retired: %v", err)
	}
	// Retire when a retired copy already exists.
	s.Save(a)
	if err := s.Retire(aid); !errors.Is(err, ErrAlreadyRetired) {
		t.Fatalf("want ErrAlreadyRetired, got %v", err)
	}
	if err := s.Delete(aid); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(aid); !errors.Is(err, ErrNoKey) {
		t.Fatalf("second delete: %v", err)
	}
	if err := s.Retire("k_00000000000000000000000000000000"); !errors.Is(err, ErrNoKey) {
		t.Fatalf("retire missing: %v", err)
	}
}

func TestNoKey(t *testing.T) {
	_, id := newKey(t)
	s := newStore(t, nil)
	if _, _, err := s.Find(id); !errors.Is(err, ErrNoKey) {
		t.Fatalf("want ErrNoKey, got %v", err)
	}
}

func TestCorruptKeyFile(t *testing.T) {
	_, id := newKey(t)
	s := newStore(t, nil)
	os.MkdirAll(filepath.Join(s.Home, "keys"), 0o700)
	secretish := `{"privateKey": "zeze-sk-1:SUPERSECRETVALUE", oops`
	os.WriteFile(filepath.Join(s.Home, "keys", id+".json"), []byte(secretish), 0o600)
	_, _, err := s.Find(id)
	if !errors.As(err, new(*InvalidKeyError)) {
		t.Fatalf("want InvalidKeyError, got %v", err)
	}
	if strings.Contains(err.Error(), "SUPERSECRET") || strings.Contains(err.Error(), "oops") {
		t.Fatal("error echoes file contents")
	}
}

func TestIsPrivateKeyEnv(t *testing.T) {
	cases := map[string]bool{
		"ZEZE_PRIVATE_KEY": true,
		"ZEZE_PRIVATE_KEY_K_5C1E0A9F3B7D2E4C8A6F1B0D9E3C7A25": true,
		"ZEZE_PRIVATE_KEY_ANYTHING":                           true,
		"zeze_private_key":                                    true,
		"ZEZE_PRIVATE_KEYS":                                   false,
		"ZEZE_CONFIG":                                         false,
		"PATH":                                                false,
		"":                                                    false,
	}
	for n, want := range cases {
		if IsPrivateKeyEnv(n) != want {
			t.Fatalf("IsPrivateKeyEnv(%q) != %v", n, want)
		}
	}
	if EnvName("k_5c1e0a9f3b7d2e4c8a6f1b0d9e3c7a25") != "ZEZE_PRIVATE_KEY_K_5C1E0A9F3B7D2E4C8A6F1B0D9E3C7A25" {
		t.Fatal("EnvName")
	}
}

func TestNewUsesOSGetenv(t *testing.T) {
	sk, id := newKey(t)
	home := t.TempDir()
	t.Setenv("ZEZE_USER_HOME", home)
	t.Setenv(EnvName(id), sk.String())
	s := New(filepath.Join(home, ".guiho", "zeze"))
	if _, src, err := s.Find(id); err != nil || src != Source("env:"+EnvName(id)) {
		t.Fatalf("%q %v", src, err)
	}
}
