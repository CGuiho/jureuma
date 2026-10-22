package crypt

import (
	"bytes"
	"encoding/base64"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/CGuiho/zeze/internal/doc"
)

func mustKey(t *testing.T) PrivateKey {
	t.Helper()
	sk, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	return sk
}

func num(t *testing.T, s string) *doc.Value {
	v, err := doc.NewNumber(s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestRoundTripScalars(t *testing.T) {
	sk := mustKey(t)
	pk := sk.Public()
	cases := []struct {
		name string
		v    *doc.Value
	}{
		{"null", doc.NewNull()},
		{"true", doc.NewBool(true)},
		{"false", doc.NewBool(false)},
		{"int", num(t, "100")},
		{"float", num(t, "1.0")},
		{"exp", num(t, "-1e2")},
		{"empty string", doc.NewString("")},
		{"string", doc.NewString("postgres://u:p@h/db")},
		{"unicode", doc.NewString("é   \x01 \"q\"")},
		{"long string", doc.NewString(strings.Repeat("x", 5000))},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			canon := doc.Canonical(c.v)
			ct, err := EncryptValue(pk, "/a/b~1c", canon)
			if err != nil {
				t.Fatal(err)
			}
			if !IsCiphertext(ct) {
				t.Fatal("not recognised as ciphertext")
			}
			if id, ok := CiphertextKeyID(ct); !ok || id != KeyID(pk) {
				t.Fatal("key id mismatch")
			}
			if bytes.Contains([]byte(ct), canon) && len(canon) > 4 {
				t.Fatal("plaintext visible in ciphertext")
			}
			got, err := DecryptValue(sk, "/a/b~1c", ct)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, canon) {
				t.Fatal("round trip mismatch")
			}
			ct2, _ := EncryptValue(pk, "/a/b~1c", canon)
			if ct2 == ct {
				t.Fatal("encryption is deterministic")
			}
		})
	}
}

func TestWrongPointerFails(t *testing.T) {
	sk := mustKey(t)
	ct, err := EncryptValue(sk.Public(), "/db/password", []byte(`"secret"`))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"/db/user", "/db/password/", "", "/db~1password", "bad"} {
		if _, err := DecryptValue(sk, p, ct); !errors.Is(err, ErrDecrypt) {
			t.Fatalf("pointer %q: expected ErrDecrypt, got %v", p, err)
		}
	}
}

func TestWrongKeyFails(t *testing.T) {
	sk, other := mustKey(t), mustKey(t)
	ct, _ := EncryptValue(sk.Public(), "/x", []byte(`"secret"`))
	if _, err := DecryptValue(other, "/x", ct); err != ErrDecrypt {
		t.Fatalf("expected ErrDecrypt, got %v", err)
	}
	// Forge the envelope key ID to the other key: still fails on authentication.
	forged := strings.Replace(ct, KeyID(sk.Public()), KeyID(other.Public()), 1)
	if _, err := DecryptValue(other, "/x", forged); err != ErrDecrypt {
		t.Fatalf("expected ErrDecrypt, got %v", err)
	}
}

func TestTamperFails(t *testing.T) {
	sk := mustKey(t)
	ct, _ := EncryptValue(sk.Public(), "/x", []byte(`"secret"`))
	prefix := ct[:len(EnvelopePrefix)+35]
	raw, err := base64.RawURLEncoding.DecodeString(ct[len(prefix):])
	if err != nil {
		t.Fatal(err)
	}
	for i := range raw {
		mod := append([]byte(nil), raw...)
		mod[i] ^= 0x01
		tampered := prefix + base64.RawURLEncoding.EncodeToString(mod)
		if _, err := DecryptValue(sk, "/x", tampered); err != ErrDecrypt {
			t.Fatalf("byte %d: expected ErrDecrypt, got %v", i, err)
		}
	}
	// Truncation, extension, encoding variants.
	bad := []string{
		ct[:len(ct)-1],
		ct + "A",
		ct + "=",
		ct[:len(ct)-10] + "\n" + ct[len(ct)-10:],
		prefix + base64.RawURLEncoding.EncodeToString(raw[:91]),
		prefix + base64.URLEncoding.EncodeToString(raw),
		prefix + base64.RawStdEncoding.EncodeToString(raw),
		strings.Replace(ct, "zeze:1:", "zeze:2:", 1),
		"",
		"zeze:1:",
	}
	for i, b := range bad {
		if b == ct {
			continue
		}
		if _, err := DecryptValue(sk, "/x", b); err != ErrDecrypt {
			t.Fatalf("variant %d: expected ErrDecrypt, got %v", i, err)
		}
	}
}

func TestNonCanonicalBase64Rejected(t *testing.T) {
	// Find a ciphertext whose last char carries unused bits and flip them.
	sk := mustKey(t)
	ct, _ := EncryptValue(sk.Public(), "/x", []byte("1"))
	payload := ct[len(EnvelopePrefix)+35:]
	if len(payload)%4 == 0 {
		t.Skip("payload has no trailing bits")
	}
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	last := strings.IndexByte(alphabet, payload[len(payload)-1])
	alt := alphabet[last^1]
	mod := ct[:len(ct)-1] + string(alt)
	if _, err := DecryptValue(sk, "/x", mod); err != ErrDecrypt {
		t.Fatal("non-canonical base64 accepted")
	}
}

func TestPaddingHidesLength(t *testing.T) {
	sk := mustKey(t)
	pk := sk.Public()
	length := func(p []byte) int {
		ct, err := EncryptValue(pk, "/flag", p)
		if err != nil {
			t.Fatal(err)
		}
		return len(ct)
	}
	base := length([]byte("true"))
	for _, p := range []string{"false", "null", "1", "12345", `"abc"`, strings.Repeat("9", 31)} {
		if l := length([]byte(p)); l != base {
			t.Fatalf("length differs for %d-byte value", len(p))
		}
	}
	if length([]byte(strings.Repeat("9", 31))) == length([]byte(strings.Repeat("9", 32))) {
		t.Fatal("expected next bucket at 32 bytes")
	}
}

func TestFrame(t *testing.T) {
	for _, n := range []int{0, 1, 30, 31, 32, 126, 127, 128, 1000} {
		p := bytes.Repeat([]byte{'a'}, n)
		f := frame(p)
		if len(f)%32 != 0 || len(f) < 32 {
			t.Fatalf("n=%d: bad frame size %d", n, len(f))
		}
		got, ok := unframe(f)
		if !ok || !bytes.Equal(got, p) {
			t.Fatalf("n=%d: unframe failed", n)
		}
		bad := append([]byte(nil), f...)
		bad[len(bad)-1] = 1
		if n < len(f)-2 {
			if _, ok := unframe(bad); ok {
				t.Fatalf("n=%d: non-zero padding accepted", n)
			}
		}
		if _, ok := unframe(append(f, make([]byte, 32)...)); ok {
			t.Fatalf("n=%d: oversized bucket accepted", n)
		}
	}
	if _, ok := unframe(make([]byte, 31)); ok {
		t.Fatal("short frame accepted")
	}
	long := make([]byte, 32)
	long[0] = 40 // claims 40 bytes in a 32-byte frame
	if _, ok := unframe(long); ok {
		t.Fatal("overlong length accepted")
	}
}

var keyIDRe = regexp.MustCompile(`^k_[0-9a-f]{32}$`)

func TestKeyID(t *testing.T) {
	for i := 0; i < 20; i++ {
		sk := mustKey(t)
		id := KeyID(sk.Public())
		if !keyIDRe.MatchString(id) || !ValidKeyID(id) {
			t.Fatalf("bad key id %q", id)
		}
		if KeyID(sk.Public()) != id {
			t.Fatal("key id not deterministic")
		}
	}
	for _, bad := range []string{"", "k_", "k_5C1E0A9F3B7D2E4C8A6F1B0D9E3C7A25", "K_5c1e0a9f3b7d2e4c8a6f1b0d9e3c7a25",
		"k_5c1e0a9f3b7d2e4c8a6f1b0d9e3c7a2", "k_5c1e0a9f3b7d2e4c8a6f1b0d9e3c7a255", "k_5c1e0a9f3b7d2e4c8a6f1b0d9e3c7a2g",
		"k_../../../../../../../../etc/pas", "x_5c1e0a9f3b7d2e4c8a6f1b0d9e3c7a25"} {
		if ValidKeyID(bad) {
			t.Fatalf("accepted %q", bad)
		}
	}
	if !ValidKeyID("k_5c1e0a9f3b7d2e4c8a6f1b0d9e3c7a25") {
		t.Fatal("rejected valid id")
	}
	// Known-answer: all-zero public key.
	var zero PublicKey
	if KeyID(zero) != KeyID(PublicKey{}) || !ValidKeyID(KeyID(zero)) {
		t.Fatal("zero key id")
	}
}

func TestKeyEncoding(t *testing.T) {
	sk := mustKey(t)
	pk := sk.Public()
	if !strings.HasPrefix(pk.String(), "zeze-pk-1:") || !strings.HasPrefix(sk.String(), "zeze-sk-1:") {
		t.Fatal("bad prefix")
	}
	pk2, err := ParsePublicKey(pk.String())
	if err != nil || pk2 != pk {
		t.Fatal("public key round trip")
	}
	sk2, err := ParsePrivateKey(sk.String())
	if err != nil || sk2 != sk || !SameKey(sk, sk2) {
		t.Fatal("private key round trip")
	}
	enc := sk.String()[len(PrivateKeyPrefix):]
	bad := []string{
		"", enc, "zeze-pk-1:" + enc, PrivateKeyPrefix + enc + "=", PrivateKeyPrefix + enc[:42],
		PrivateKeyPrefix + enc + "AA", PrivateKeyPrefix + " " + enc,
		PrivateKeyPrefix + base64.RawStdEncoding.EncodeToString(bytes.Repeat([]byte{0xff}, 32)),
	}
	for _, b := range bad {
		_, err := ParsePrivateKey(b)
		if err == nil {
			t.Fatalf("accepted bad private key")
		}
		if enc != "" && strings.Contains(err.Error(), enc[:8]) {
			t.Fatal("error echoes key material")
		}
	}
	if _, err := ParsePublicKey(sk.String()); err == nil {
		t.Fatal("private key accepted as public")
	}
	if strings.Contains(sk.GoString(), enc[:8]) {
		t.Fatal("GoString leaks key")
	}
}

func TestLowOrderRecipientRejected(t *testing.T) {
	var zero PublicKey
	if _, err := EncryptValue(zero, "/x", []byte("1")); err == nil {
		t.Fatal("encrypted to all-zero public key")
	}
}

func TestInvalidPointerRejected(t *testing.T) {
	sk := mustKey(t)
	if _, err := EncryptValue(sk.Public(), "nope", []byte("1")); err == nil {
		t.Fatal("accepted invalid pointer")
	}
}

func TestIsCiphertext(t *testing.T) {
	id := "k_5c1e0a9f3b7d2e4c8a6f1b0d9e3c7a25"
	cases := map[string]bool{
		"zeze:1:" + id + ":AAAA":                      true,
		"zeze:1:" + id + ":":                          true,
		"zeze:1:" + id:                                false,
		"zeze:1:k_5C1E0A9F3B7D2E4C8A6F1B0D9E3C7A25:x": false,
		"zeze:2:" + id + ":AAAA":                      false,
		"hello":                                       false,
	}
	for s, want := range cases {
		if IsCiphertext(s) != want {
			t.Fatalf("IsCiphertext(%q) != %v", s, want)
		}
		got, ok := CiphertextKeyID(s)
		if ok != want || (ok && got != id) {
			t.Fatalf("CiphertextKeyID(%q)", s)
		}
	}
}

func TestTags(t *testing.T) {
	sk, other := mustKey(t), mustKey(t)
	hexRe := regexp.MustCompile(`^[0-9a-f]{32}$`)
	if s := SyncTag("abc"); !hexRe.MatchString(s) || s != SyncTag("abc") || s == SyncTag("abd") {
		t.Fatal("SyncTag")
	}
	// Known answer: sha256("abc") prefix.
	if SyncTag("abc") != "ba7816bf8f01cfea414140de5dae2223" {
		t.Fatal("SyncTag known answer")
	}
	a := PlainTag(sk, []byte(`"v"`))
	if !hexRe.MatchString(a) || a != PlainTag(sk, []byte(`"v"`)) {
		t.Fatal("PlainTag not deterministic")
	}
	if a == PlainTag(sk, []byte(`"w"`)) || a == PlainTag(other, []byte(`"v"`)) {
		t.Fatal("PlainTag collision")
	}
}

func TestErrorsCarryNoDetail(t *testing.T) {
	if ErrDecrypt.Error() != "decryption failed" {
		t.Fatal("ErrDecrypt text changed")
	}
}
