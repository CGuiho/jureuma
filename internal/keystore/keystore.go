// Package keystore resolves, stores, retires and lists zeze private keys.
//
// Errors name sources (environment variable names, key file paths relative to
// the CLI home) but never key material or file contents.
package keystore

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/CGuiho/zeze/internal/crypt"
	"github.com/CGuiho/zeze/internal/paths"
)

// Environment variable names.
const (
	EnvPrivateKey       = "ZEZE_PRIVATE_KEY"
	EnvPrivateKeyPrefix = "ZEZE_PRIVATE_KEY_"
)

const (
	keysDir    = "keys"
	retiredDir = "retired"
	// maxKeyFile bounds how much of a key file is read.
	maxKeyFile = 64 << 10
)

// Store resolves private keys. Home is the CLI home (~/.guiho/zeze).
type Store struct {
	Home   string
	Getenv func(string) string
}

// New returns a Store for home that reads the process environment.
func New(home string) *Store { return &Store{Home: home, Getenv: os.Getenv} }

// Source names where a key came from, e.g. "env:ZEZE_PRIVATE_KEY" or
// "file:keys/k_….json".
type Source string

// Entry is one stored key file. Label and PublicKey are non-secret metadata
// read from the file: both are empty when the file could not be parsed or the
// key file predates labels. No private material is ever carried here.
type Entry struct {
	KeyID     string
	Label     string
	PublicKey string
	Retired   bool
	Path      string
}

// MismatchError reports that a source holds a key whose derived key ID does
// not match the one requested.
type MismatchError struct{ Source Source }

func (e *MismatchError) Error() string { return fmt.Sprintf("key mismatch (source: %s)", e.Source) }

// InvalidKeyError reports that a source could not be used: malformed key,
// unreadable or unparsable key file, or insecure file ownership/permissions.
// Reason is a fixed description and never contains file contents.
type InvalidKeyError struct {
	Source Source
	Reason string
}

func (e *InvalidKeyError) Error() string {
	return fmt.Sprintf("unusable private key (source: %s): %s", e.Source, e.Reason)
}

// ErrNoKey is returned when no source holds the requested key.
var ErrNoKey = errors.New("private key not available")

// ErrInvalidKeyID is returned when a key ID does not match ^k_[0-9a-f]{32}$.
var ErrInvalidKeyID = errors.New("invalid key ID")

// ErrAlreadyRetired is returned by Retire when a retired copy already exists.
var ErrAlreadyRetired = errors.New("key is already retired")

// EnvName returns ZEZE_PRIVATE_KEY_<UPPER(keyID)>.
func EnvName(keyID string) string { return EnvPrivateKeyPrefix + strings.ToUpper(keyID) }

// IsPrivateKeyEnv reports whether name is a private-key variable that must be
// stripped from child environments. Matching is case-insensitive so that
// Windows-style spellings are also caught.
func IsPrivateKeyEnv(name string) bool {
	u := strings.ToUpper(name)
	return u == EnvPrivateKey || strings.HasPrefix(u, EnvPrivateKeyPrefix)
}

// keyFile is the on-disk format. Label is non-secret metadata and is omitted
// when empty; key files written before labels existed simply lack the field.
type keyFile struct {
	KeyID      string `json:"keyId"`
	PublicKey  string `json:"publicKey"`
	PrivateKey string `json:"privateKey"`
	Label      string `json:"label,omitempty"`
	Created    string `json:"created"`
}

// keyFileMeta is the non-secret part of a key file. Decoding into this shape
// never materialises the private key in memory.
type keyFileMeta struct {
	KeyID     string `json:"keyId"`
	PublicKey string `json:"publicKey"`
	Label     string `json:"label"`
	Created   string `json:"created"`
}

func (s *Store) getenv(name string) string {
	if s.Getenv == nil {
		return os.Getenv(name)
	}
	return s.Getenv(name)
}

func (s *Store) activePath(keyID string) string {
	return filepath.Join(s.Home, keysDir, keyID+".json")
}

func (s *Store) retiredPath(keyID string) string {
	return filepath.Join(s.Home, keysDir, retiredDir, keyID+".json")
}

func fileSource(rel ...string) Source {
	return Source("file:" + filepath.ToSlash(filepath.Join(rel...)))
}

// Find returns the private key for keyID from, in order: env
// ZEZE_PRIVATE_KEY_<UPPER(keyID)>, env ZEZE_PRIVATE_KEY (only if it
// matches), keys/<keyID>.json, keys/retired/<keyID>.json.
//
// It returns ErrNoKey when none exists, *MismatchError when a dedicated source
// holds a different key, and *InvalidKeyError when a source is malformed or
// insecure. An unparsable ZEZE_PRIVATE_KEY does not hide a later valid
// source; it is reported only when nothing else matches.
func (s *Store) Find(keyID string) (crypt.PrivateKey, Source, error) {
	var zero crypt.PrivateKey
	if !crypt.ValidKeyID(keyID) {
		return zero, "", ErrInvalidKeyID
	}

	// 1. Dedicated variable.
	name := EnvName(keyID)
	if v := s.getenv(name); v != "" {
		src := Source("env:" + name)
		sk, err := crypt.ParsePrivateKey(strings.TrimSpace(v))
		if err != nil {
			return zero, src, &InvalidKeyError{Source: src, Reason: "malformed private key"}
		}
		if crypt.KeyID(sk.Public()) != keyID {
			return zero, src, &MismatchError{Source: src}
		}
		return sk, src, nil
	}

	// 2. Generic variable, used only when it matches.
	var deferred error
	if v := s.getenv(EnvPrivateKey); v != "" {
		src := Source("env:" + EnvPrivateKey)
		sk, err := crypt.ParsePrivateKey(strings.TrimSpace(v))
		if err != nil {
			deferred = &InvalidKeyError{Source: src, Reason: "malformed private key"}
		} else if crypt.KeyID(sk.Public()) == keyID {
			return sk, src, nil
		}
	}

	// 3, 4. Key files.
	for _, f := range []struct {
		path string
		src  Source
	}{
		{s.activePath(keyID), fileSource(keysDir, keyID+".json")},
		{s.retiredPath(keyID), fileSource(keysDir, retiredDir, keyID+".json")},
	} {
		sk, err := loadKeyFile(f.path, f.src, keyID)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return zero, f.src, err
		}
		return sk, f.src, nil
	}

	if deferred != nil {
		return zero, "", deferred
	}
	return zero, "", ErrNoKey
}

// loadKeyFile opens path safely, checks it, and parses it. A missing file
// returns an error wrapping os.ErrNotExist.
func loadKeyFile(path string, src Source, keyID string) (crypt.PrivateKey, error) {
	var zero crypt.PrivateKey
	f, err := openKeyFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return zero, os.ErrNotExist
		}
		if errors.Is(err, errSymlink) {
			return zero, &InvalidKeyError{Source: src, Reason: "key file is a symbolic link"}
		}
		return zero, &InvalidKeyError{Source: src, Reason: "cannot open key file"}
	}
	defer f.Close()
	if reason := checkKeyFile(f); reason != "" {
		return zero, &InvalidKeyError{Source: src, Reason: reason}
	}
	data, err := io.ReadAll(io.LimitReader(f, maxKeyFile+1))
	if err != nil {
		return zero, &InvalidKeyError{Source: src, Reason: "cannot read key file"}
	}
	defer clear(data)
	if len(data) > maxKeyFile {
		return zero, &InvalidKeyError{Source: src, Reason: "key file too large"}
	}
	var kf keyFile
	if err := json.Unmarshal(data, &kf); err != nil {
		// Never wrap json errors: they can quote file contents.
		return zero, &InvalidKeyError{Source: src, Reason: "key file is not valid JSON"}
	}
	sk, err := crypt.ParsePrivateKey(kf.PrivateKey)
	kf.PrivateKey = ""
	if err != nil {
		return zero, &InvalidKeyError{Source: src, Reason: "malformed private key"}
	}
	pk := sk.Public()
	if crypt.KeyID(pk) != keyID || kf.KeyID != keyID {
		return zero, &MismatchError{Source: src}
	}
	if kf.PublicKey != "" && kf.PublicKey != pk.String() {
		return zero, &MismatchError{Source: src}
	}
	return sk, nil
}

var errSymlink = errors.New("symbolic link")

// readKeyMeta returns the non-secret metadata of one key file. It reports ok
// false when the file cannot be opened, checked or parsed, or when the key ID
// inside does not match the file name; callers listing keys stay best-effort
// in that case. The private key field is never decoded.
func readKeyMeta(path string, keyID string) (keyFileMeta, bool) {
	var zero keyFileMeta
	f, err := openKeyFile(path)
	if err != nil {
		return zero, false
	}
	defer f.Close()
	if checkKeyFile(f) != "" {
		return zero, false
	}
	data, err := io.ReadAll(io.LimitReader(f, maxKeyFile+1))
	if err != nil || len(data) > maxKeyFile {
		return zero, false
	}
	var meta keyFileMeta
	if err := json.Unmarshal(data, &meta); err != nil || meta.KeyID != keyID {
		return zero, false
	}
	return meta, true
}

// Save writes sk to keys/<id>.json without a label. See SaveLabeled.
func (s *Store) Save(sk crypt.PrivateKey) (string, error) {
	return s.SaveLabeled(sk, "")
}

// SaveLabeled writes sk with its non-secret label to keys/<id>.json with mode
// 0600, atomically. An empty label is omitted from the file. If an identical,
// valid key file with the same label already exists it is left untouched;
// otherwise the file is (re)written, keeping its original created timestamp
// when it already existed.
func (s *Store) SaveLabeled(sk crypt.PrivateKey, label string) (string, error) {
	pk := sk.Public()
	id := crypt.KeyID(pk)
	path := s.activePath(id)
	if err := ensureDir(filepath.Join(s.Home, keysDir)); err != nil {
		return "", err
	}
	created := time.Now().UTC().Format(time.RFC3339)
	if existing, err := loadKeyFile(path, fileSource(keysDir, id+".json"), id); err == nil && crypt.SameKey(existing, sk) {
		if meta, ok := readKeyMeta(path, id); ok {
			if meta.Label == label {
				return path, nil
			}
			if meta.Created != "" {
				created = meta.Created
			}
		}
	}
	if fi, err := os.Lstat(path); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		return "", &InvalidKeyError{Source: fileSource(keysDir, id+".json"), Reason: "key file is a symbolic link"}
	}
	data, err := json.MarshalIndent(keyFile{
		KeyID:      id,
		PublicKey:  pk.String(),
		PrivateKey: sk.String(),
		Label:      label,
		Created:    created,
	}, "", "  ")
	if err != nil {
		return "", errors.New("cannot encode key file")
	}
	data = append(data, '\n')
	defer clear(data)
	if err := paths.AtomicWrite(path, data, 0o600); err != nil {
		return "", fmt.Errorf("write %s: %w", fileSource(keysDir, id+".json"), stripPathErr(err))
	}
	return path, nil
}

// Retire moves keys/<id>.json to keys/retired/<id>.json.
func (s *Store) Retire(keyID string) error {
	if !crypt.ValidKeyID(keyID) {
		return ErrInvalidKeyID
	}
	src := s.activePath(keyID)
	fi, err := os.Lstat(src)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrNoKey
		}
		return fmt.Errorf("retire %s: %w", keyID, stripPathErr(err))
	}
	if fi.Mode()&os.ModeSymlink != 0 || !fi.Mode().IsRegular() {
		return &InvalidKeyError{Source: fileSource(keysDir, keyID+".json"), Reason: "key file is not a regular file"}
	}
	dst := s.retiredPath(keyID)
	if err := ensureDir(filepath.Dir(dst)); err != nil {
		return err
	}
	if _, err := os.Lstat(dst); err == nil {
		return ErrAlreadyRetired
	}
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("retire %s: %w", keyID, stripPathErr(err))
	}
	return nil
}

// Delete removes a retired key file. Active keys must be retired first.
func (s *Store) Delete(keyID string) error {
	if !crypt.ValidKeyID(keyID) {
		return ErrInvalidKeyID
	}
	p := s.retiredPath(keyID)
	if _, err := os.Lstat(p); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrNoKey
		}
		return fmt.Errorf("delete %s: %w", keyID, stripPathErr(err))
	}
	if err := os.Remove(p); err != nil {
		return fmt.Errorf("delete %s: %w", keyID, stripPathErr(err))
	}
	return nil
}

// List returns active keys followed by retired keys, each sorted by key ID.
// Only files named <valid keyId>.json are listed; each file's non-secret
// metadata (label, public key) is read best-effort and left empty when the
// file cannot be parsed. No private material is ever returned.
func (s *Store) List() ([]Entry, error) {
	var out []Entry
	for _, d := range []struct {
		dir     string
		retired bool
	}{
		{filepath.Join(s.Home, keysDir), false},
		{filepath.Join(s.Home, keysDir, retiredDir), true},
	} {
		ents, err := os.ReadDir(d.dir)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("list keys: %w", stripPathErr(err))
		}
		var part []Entry
		for _, e := range ents {
			name := e.Name()
			id, ok := strings.CutSuffix(name, ".json")
			if !ok || !crypt.ValidKeyID(id) || e.IsDir() {
				continue
			}
			path := filepath.Join(d.dir, name)
			en := Entry{KeyID: id, Retired: d.retired, Path: path}
			if meta, ok := readKeyMeta(path, id); ok {
				en.Label = meta.Label
				en.PublicKey = meta.PublicKey
			}
			part = append(part, en)
		}
		sort.Slice(part, func(i, j int) bool { return part[i].KeyID < part[j].KeyID })
		out = append(out, part...)
	}
	return out, nil
}

func ensureDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create key directory: %w", stripPathErr(err))
	}
	return nil
}

// stripPathErr unwraps *os.PathError / *os.LinkError to the underlying
// errno so messages stay short; paths are not secret but are redundant.
func stripPathErr(err error) error {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}
	var le *os.LinkError
	if errors.As(err, &le) {
		return le.Err
	}
	return err
}
