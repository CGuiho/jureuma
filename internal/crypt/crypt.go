// Package crypt implements zeze's value encryption: X25519 ECIES with
// HKDF-SHA256 and AES-256-GCM, length-hiding padding buckets, pointer-bound
// AAD, strict unpadded base64url encodings, derived key IDs, and sync tags.
//
// No error returned by this package contains plaintext or key material.
package crypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/CGuiho/zeze/internal/doc"
)

// Encoding and domain-separation constants.
const (
	PublicKeyPrefix  = "zeze-pk-1:"
	PrivateKeyPrefix = "zeze-sk-1:"
	EnvelopePrefix   = "zeze:1:"

	aadPrefix   = "zeze/1/aad"
	infoValue   = "zeze/1/value"
	infoSync    = "zeze/1/sync"
	keyIDDomain = "zeze/1/keyid"

	keyIDLen   = 34 // "k_" + 32 hex
	keySize    = 32
	nonceSize  = 12
	tagSize    = 16
	bucketSize = 32
	// minDecoded is eph_pk || nonce || GCM tag || smallest padded frame.
	minDecoded = keySize + nonceSize + tagSize + bucketSize
)

// PublicKey is an X25519 public key.
type PublicKey [32]byte

// PrivateKey is an X25519 private key (scalar).
type PrivateKey [32]byte

// ErrDecrypt is the only error DecryptValue returns.
var ErrDecrypt = errors.New("decryption failed")

// ErrInvalidPublicKey and ErrInvalidPrivateKey are returned by the parsers.
// They never echo the input.
var (
	ErrInvalidPublicKey  = errors.New("invalid public key")
	ErrInvalidPrivateKey = errors.New("invalid private key")
	ErrInvalidPointer    = errors.New("invalid JSON Pointer")
	ErrEncrypt           = errors.New("encryption failed")
)

var b64 = base64.RawURLEncoding.Strict()

// encode returns unpadded base64url.
func encode(b []byte) string { return b64.EncodeToString(b) }

// decodeStrict decodes unpadded base64url, rejecting padding, line breaks,
// non-zero trailing bits, and any other non-canonical form.
func decodeStrict(s string) ([]byte, bool) {
	if strings.ContainsAny(s, "=\r\n") {
		return nil, false
	}
	b, err := b64.DecodeString(s)
	if err != nil {
		return nil, false
	}
	if b64.EncodeToString(b) != s {
		return nil, false
	}
	return b, true
}

// GenerateKey creates a new random X25519 private key.
func GenerateKey() (PrivateKey, error) {
	k, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return PrivateKey{}, errors.New("key generation failed")
	}
	var sk PrivateKey
	copy(sk[:], k.Bytes())
	return sk, nil
}

// Public derives the public key.
func (k PrivateKey) Public() PublicKey {
	priv, err := ecdh.X25519().NewPrivateKey(k[:])
	if err != nil {
		// Unreachable: X25519 accepts any 32-byte scalar.
		panic("crypt: invalid X25519 private key length")
	}
	var pk PublicKey
	copy(pk[:], priv.PublicKey().Bytes())
	return pk
}

// String returns "zeze-pk-1:" + unpadded base64url.
func (k PublicKey) String() string { return PublicKeyPrefix + encode(k[:]) }

// String returns "zeze-sk-1:" + unpadded base64url. Callers must treat the
// result as secret.
func (k PrivateKey) String() string { return PrivateKeyPrefix + encode(k[:]) }

// GoString prevents accidental key disclosure through %#v.
func (k PrivateKey) GoString() string { return "crypt.PrivateKey{REDACTED}" }

// ParsePublicKey parses "zeze-pk-1:<base64url>".
func ParsePublicKey(s string) (PublicKey, error) {
	var pk PublicKey
	rest, ok := strings.CutPrefix(s, PublicKeyPrefix)
	if !ok {
		return pk, ErrInvalidPublicKey
	}
	b, ok := decodeStrict(rest)
	if !ok || len(b) != keySize {
		return pk, ErrInvalidPublicKey
	}
	copy(pk[:], b)
	return pk, nil
}

// ParsePrivateKey parses "zeze-sk-1:<base64url>".
func ParsePrivateKey(s string) (PrivateKey, error) {
	var sk PrivateKey
	rest, ok := strings.CutPrefix(s, PrivateKeyPrefix)
	if !ok {
		return sk, ErrInvalidPrivateKey
	}
	b, ok := decodeStrict(rest)
	if !ok || len(b) != keySize {
		return sk, ErrInvalidPrivateKey
	}
	copy(sk[:], b)
	clear(b)
	return sk, nil
}

// KeyID returns "k_" + the first 32 lowercase hex chars of
// SHA-256("zeze/1/keyid" || pk).
func KeyID(pk PublicKey) string {
	h := sha256.New()
	h.Write([]byte(keyIDDomain))
	h.Write(pk[:])
	return "k_" + hex.EncodeToString(h.Sum(nil))[:32]
}

// ValidKeyID reports whether id matches ^k_[0-9a-f]{32}$.
func ValidKeyID(id string) bool {
	if len(id) != keyIDLen || id[0] != 'k' || id[1] != '_' {
		return false
	}
	for i := 2; i < len(id); i++ {
		c := id[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// frame returns uvarint(len(p)) || p || zero padding to a multiple of 32
// bytes, at least 32.
func frame(p []byte) []byte {
	var hdr [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(hdr[:], uint64(len(p)))
	total := n + len(p)
	size := (total + bucketSize - 1) / bucketSize * bucketSize
	if size < bucketSize {
		size = bucketSize
	}
	out := make([]byte, size)
	copy(out, hdr[:n])
	copy(out[n:], p)
	return out
}

// unframe reverses frame, verifying the bucket size and all-zero padding.
func unframe(f []byte) ([]byte, bool) {
	if len(f) < bucketSize || len(f)%bucketSize != 0 {
		return nil, false
	}
	l, n := binary.Uvarint(f)
	if n <= 0 || l > uint64(len(f)-n) {
		return nil, false
	}
	end := n + int(l)
	var nz byte
	for _, b := range f[end:] {
		nz |= b
	}
	if nz != 0 {
		return nil, false
	}
	// Reject frames that are larger than the canonical bucket for this length.
	if len(frame(f[n:end])) != len(f) {
		return nil, false
	}
	return append([]byte(nil), f[n:end]...), true
}

// aad builds "zeze/1/aad" || keyId || uint32_be(len(p)) || p.
func aad(keyID, pointer string) []byte {
	b := make([]byte, 0, len(aadPrefix)+keyIDLen+4+len(pointer))
	b = append(b, aadPrefix...)
	b = append(b, keyID...)
	b = binary.BigEndian.AppendUint32(b, uint32(len(pointer)))
	b = append(b, pointer...)
	return b
}

func valueKey(shared, ephPK, recipientPK []byte) ([]byte, error) {
	salt := make([]byte, 0, 2*keySize)
	salt = append(salt, ephPK...)
	salt = append(salt, recipientPK...)
	return hkdf.Key(sha256.New, shared, salt, infoValue, 32)
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func validPointer(p string) bool {
	if _, err := doc.ParsePointer(p); err != nil {
		return false
	}
	return uint64(len(p)) <= 0xFFFFFFFF
}

// EncryptValue seals canonical scalar JSON for the leaf at pointer.
// Output: "zeze:1:<keyId>:<base64url(eph_pk||nonce||sealed)>".
func EncryptValue(pk PublicKey, pointer string, canonical []byte) (string, error) {
	if !validPointer(pointer) {
		return "", ErrInvalidPointer
	}
	curve := ecdh.X25519()
	recipient, err := curve.NewPublicKey(pk[:])
	if err != nil {
		return "", ErrInvalidPublicKey
	}
	eph, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return "", ErrEncrypt
	}
	shared, err := eph.ECDH(recipient)
	if err != nil {
		// Low-order or otherwise unusable recipient key.
		return "", ErrInvalidPublicKey
	}
	defer clear(shared)
	ephPK := eph.PublicKey().Bytes()
	key, err := valueKey(shared, ephPK, pk[:])
	if err != nil {
		return "", ErrEncrypt
	}
	defer clear(key)
	gcm, err := newGCM(key)
	if err != nil {
		return "", ErrEncrypt
	}
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return "", ErrEncrypt
	}
	framed := frame(canonical)
	defer clear(framed)
	id := KeyID(pk)
	out := make([]byte, 0, keySize+nonceSize+len(framed)+tagSize)
	out = append(out, ephPK...)
	out = append(out, nonce...)
	out = gcm.Seal(out, nonce, framed, aad(id, pointer))
	return EnvelopePrefix + id + ":" + encode(out), nil
}

// splitEnvelope returns the key ID and payload of an envelope.
func splitEnvelope(s string) (keyID, payload string, ok bool) {
	rest, ok := strings.CutPrefix(s, EnvelopePrefix)
	if !ok || len(rest) < keyIDLen+1 || rest[keyIDLen] != ':' {
		return "", "", false
	}
	keyID = rest[:keyIDLen]
	if !ValidKeyID(keyID) {
		return "", "", false
	}
	return keyID, rest[keyIDLen+1:], true
}

// DecryptValue reverses EncryptValue. Any failure returns ErrDecrypt.
func DecryptValue(sk PrivateKey, pointer string, ciphertext string) ([]byte, error) {
	keyID, payload, ok := splitEnvelope(ciphertext)
	if !ok || !validPointer(pointer) {
		return nil, ErrDecrypt
	}
	pk := sk.Public()
	if subtle.ConstantTimeCompare([]byte(KeyID(pk)), []byte(keyID)) != 1 {
		return nil, ErrDecrypt
	}
	raw, ok := decodeStrict(payload)
	if !ok || len(raw) < minDecoded || (len(raw)-keySize-nonceSize-tagSize)%bucketSize != 0 {
		return nil, ErrDecrypt
	}
	curve := ecdh.X25519()
	priv, err := curve.NewPrivateKey(sk[:])
	if err != nil {
		return nil, ErrDecrypt
	}
	ephPK := raw[:keySize]
	eph, err := curve.NewPublicKey(ephPK)
	if err != nil {
		return nil, ErrDecrypt
	}
	shared, err := priv.ECDH(eph)
	if err != nil {
		return nil, ErrDecrypt
	}
	defer clear(shared)
	key, err := valueKey(shared, ephPK, pk[:])
	if err != nil {
		return nil, ErrDecrypt
	}
	defer clear(key)
	gcm, err := newGCM(key)
	if err != nil {
		return nil, ErrDecrypt
	}
	nonce := raw[keySize : keySize+nonceSize]
	framed, err := gcm.Open(nil, nonce, raw[keySize+nonceSize:], aad(keyID, pointer))
	if err != nil {
		return nil, ErrDecrypt
	}
	defer clear(framed)
	plain, ok := unframe(framed)
	if !ok {
		return nil, ErrDecrypt
	}
	return plain, nil
}

// IsCiphertext reports whether s has the "zeze:1:" envelope prefix and a
// valid key ID followed by ':'.
func IsCiphertext(s string) bool {
	_, _, ok := splitEnvelope(s)
	return ok
}

// CiphertextKeyID extracts the key ID from an envelope.
func CiphertextKeyID(s string) (string, bool) {
	id, _, ok := splitEnvelope(s)
	return id, ok
}

// SyncTag returns hex(first 16 bytes of SHA-256(ciphertext)).
func SyncTag(ciphertext string) string {
	sum := sha256.Sum256([]byte(ciphertext))
	return hex.EncodeToString(sum[:16])
}

// PlainTag returns hex(first 16 bytes of HMAC-SHA256(HKDF(sk,
// "zeze/1/sync"), canonical)).
func PlainTag(sk PrivateKey, canonical []byte) string {
	key, err := hkdf.Key(sha256.New, sk[:], nil, infoSync, 32)
	if err != nil {
		// Unreachable: 32 bytes is far below the HKDF-SHA256 output limit.
		panic("crypt: hkdf failure")
	}
	defer clear(key)
	m := hmac.New(sha256.New, key)
	m.Write(canonical)
	return hex.EncodeToString(m.Sum(nil)[:16])
}

// SameKey reports whether a and b are equal, in constant time.
func SameKey(a, b PrivateKey) bool { return subtle.ConstantTimeCompare(a[:], b[:]) == 1 }
