package config

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/CGuiho/jureuma/internal/paths"
)

// State file names under <home>/state.
const (
	ProtectionsKeyFile    = "protections.key"
	ProtectionsDigestFile = "protections.json"
)

const (
	protectionsKeySize = 32
	protectionsDomain  = "jureuma-protections-v1\n"
	protectionsAlg     = "HMAC-SHA256"
)

type protectionsDigest struct {
	Version   int    `json:"version"`
	Algorithm string `json:"algorithm"`
	Digest    string `json:"digest"`
}

// canonicalProtections encodes the effective meaning of the block so that a
// semantically identical rewrite (nil vs true, nil vs empty list) keeps the
// seal intact, while any change of meaning breaks it.
func canonicalProtections(g *Global) []byte {
	var p Protections
	if g != nil {
		p = g.Protections
	}
	markers := p.AgentEnvMarkers
	if markers == nil {
		markers = []string{}
	}
	b, _ := json.Marshal(struct {
		AllowBypass     bool     `json:"allowBypass"`
		AgentEnvMarkers []string `json:"agentEnvMarkers"`
	}{p.BypassAllowed(), markers})
	return append([]byte(protectionsDomain), b...)
}

func protectionsMAC(key []byte, g *Global) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(canonicalProtections(g))
	return m.Sum(nil)
}

func stateDir(home string) string { return filepath.Join(home, "state") }

// readProtectionsKey returns the local secret, or nil and no error when it
// does not exist.
func readProtectionsKey(home string) ([]byte, error) {
	b, err := os.ReadFile(filepath.Join(stateDir(home), ProtectionsKeyFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("protections key: cannot read: %w", osCause(err))
	}
	if len(b) != protectionsKeySize {
		return nil, errors.New("protections key: invalid length")
	}
	return b, nil
}

// ensureProtectionsKey returns the local secret, creating it if needed. A
// concurrently created key wins over ours (link-without-clobber).
func ensureProtectionsKey(home string) ([]byte, error) {
	if k, err := readProtectionsKey(home); err != nil || k != nil {
		return k, err
	}
	dir := stateDir(home)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("protections key: cannot create state directory: %w", osCause(err))
	}
	key := make([]byte, protectionsKeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, errors.New("protections key: random source failed")
	}
	f, err := os.CreateTemp(dir, "."+ProtectionsKeyFile+".tmp-*")
	if err != nil {
		return nil, fmt.Errorf("protections key: cannot write: %w", osCause(err))
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	_, werr := f.Write(key)
	serr := f.Sync()
	cerr := f.Close()
	if werr != nil || serr != nil || cerr != nil {
		return nil, errors.New("protections key: cannot write")
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		return nil, fmt.Errorf("protections key: cannot write: %w", osCause(err))
	}
	if err := os.Link(tmp, filepath.Join(dir, ProtectionsKeyFile)); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("protections key: cannot write: %w", osCause(err))
		}
	}
	k, err := readProtectionsKey(home)
	if err == nil && k == nil {
		err = errors.New("protections key: missing after creation")
	}
	return k, err
}

// SealProtections records an HMAC-SHA256 of g's protections block in
// <home>/state/protections.json, keyed by a random 32-byte local secret in
// <home>/state/protections.key (created on first use, mode 0600).
func SealProtections(home string, g *Global) error {
	key, err := ensureProtectionsKey(home)
	if err != nil {
		return err
	}
	d := protectionsDigest{Version: 1, Algorithm: protectionsAlg, Digest: hex.EncodeToString(protectionsMAC(key, g))}
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return errors.New("protections digest: cannot encode")
	}
	b = append(b, '\n')
	if err := paths.AtomicWrite(filepath.Join(stateDir(home), ProtectionsDigestFile), b, 0o600); err != nil {
		return fmt.Errorf("protections digest: cannot write: %w", osCause(err))
	}
	return nil
}

// ProtectionsIntact reports whether g's protections block still matches the
// sealed digest. With no seal (no key or no digest file), a block equal to
// the defaults (bypass allowed, no extra markers) is intact and any other
// block is not, because it was written by hand. A malformed digest file is
// reported as not intact; I/O failures are errors.
func ProtectionsIntact(home string, g *Global) (bool, error) {
	key, err := readProtectionsKey(home)
	if err != nil {
		return false, err
	}
	b, err := os.ReadFile(filepath.Join(stateDir(home), ProtectionsDigestFile))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("protections digest: cannot read: %w", osCause(err))
	}
	if key == nil || b == nil {
		return isDefaultProtections(g), nil
	}
	var d protectionsDigest
	if err := json.Unmarshal(b, &d); err != nil || d.Version != 1 || d.Algorithm != protectionsAlg {
		return false, nil
	}
	want, err := hex.DecodeString(d.Digest)
	if err != nil || len(want) != sha256.Size {
		return false, nil
	}
	return hmac.Equal(want, protectionsMAC(key, g)), nil
}

func isDefaultProtections(g *Global) bool {
	if g == nil {
		return true
	}
	return g.Protections.BypassAllowed() && len(g.Protections.AgentEnvMarkers) == 0
}
