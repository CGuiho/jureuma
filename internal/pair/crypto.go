package pair

import (
	"sort"

	"github.com/CGuiho/jureuma/internal/crypt"
	"github.com/CGuiho/jureuma/internal/doc"
	"github.com/CGuiho/jureuma/internal/jsonc"
	"github.com/CGuiho/jureuma/internal/keystore"
	"github.com/CGuiho/jureuma/internal/xerr"
)

// KeyLookup returns the private key for a key ID.
type KeyLookup func(keyID string) (crypt.PrivateKey, error)

// SingleKey returns a KeyLookup that knows only sk.
func SingleKey(sk crypt.PrivateKey) KeyLookup {
	id := crypt.KeyID(sk.Public())
	return func(keyID string) (crypt.PrivateKey, error) {
		if keyID != id {
			return crypt.PrivateKey{}, keystore.ErrNoKey
		}
		return sk, nil
	}
}

// StoreKeys returns a KeyLookup that tries primary first and then the store
// (which includes retired keys).
func StoreKeys(primary *crypt.PrivateKey, st *keystore.Store) KeyLookup {
	cache := map[string]crypt.PrivateKey{}
	if primary != nil {
		cache[crypt.KeyID(primary.Public())] = *primary
	}
	return func(keyID string) (crypt.PrivateKey, error) {
		if k, ok := cache[keyID]; ok {
			return k, nil
		}
		if st == nil {
			return crypt.PrivateKey{}, keystore.ErrNoKey
		}
		k, _, err := st.Find(keyID)
		if err != nil {
			return crypt.PrivateKey{}, err
		}
		cache[keyID] = k
		return k, nil
	}
}

func join(base, rel string) string { return base + rel }

// EncryptAt encrypts every leaf of v, which is located at the escaped
// pointer base, to pk.
func EncryptAt(pk crypt.PublicKey, base string, v *doc.Value) (*doc.Value, error) {
	return doc.MapLeaves(v, func(p string, leaf *doc.Value) (*doc.Value, error) {
		ct, err := crypt.EncryptValue(pk, join(base, p), doc.Canonical(leaf))
		if err != nil {
			return nil, xerr.Failure("%s: encryption failed", displayPointer(join(base, p)))
		}
		return doc.NewString(ct), nil
	})
}

// DecryptAt decrypts every ciphertext leaf of v located at base.
func DecryptAt(keys KeyLookup, base string, v *doc.Value) (*doc.Value, error) {
	return doc.MapLeaves(v, func(p string, leaf *doc.Value) (*doc.Value, error) {
		ptr := join(base, p)
		return decryptLeaf(keys, ptr, leaf)
	})
}

func decryptLeaf(keys KeyLookup, ptr string, leaf *doc.Value) (*doc.Value, error) {
	if leaf.Kind != doc.String {
		return nil, xerr.Key("%s: decryption failed", displayPointer(ptr))
	}
	id, ok := crypt.CiphertextKeyID(leaf.S)
	if !ok {
		return nil, xerr.Key("%s: decryption failed", displayPointer(ptr))
	}
	sk, err := keys(id)
	if err != nil {
		return nil, xerr.Key("%s: private key %s not available", displayPointer(ptr), id)
	}
	plain, err := crypt.DecryptValue(sk, ptr, leaf.S)
	if err != nil {
		return nil, xerr.Key("%s: decryption failed", displayPointer(ptr))
	}
	v, err := jsonc.Parse(plain, true)
	clear(plain)
	if err != nil || !v.IsLeaf() {
		return nil, xerr.Key("%s: decryption failed", displayPointer(ptr))
	}
	return v, nil
}

// DecryptLeaf decrypts one ciphertext at pointer ptr.
func DecryptLeaf(keys KeyLookup, ptr string, leaf *doc.Value) (*doc.Value, error) {
	return decryptLeaf(keys, ptr, leaf)
}

func displayPointer(p string) string {
	if p == "" {
		return "(root)"
	}
	return p
}

// EncryptDoc encrypts plain to pk. When prev and sk are given, a leaf whose
// plaintext is unchanged keeps its previous ciphertext (only if that
// ciphertext is under pk's key ID), so diffs stay small.
func EncryptDoc(pk crypt.PublicKey, plain *doc.Value, prev *Encrypted, sk *crypt.PrivateKey) (*doc.Value, error) {
	if plain == nil || plain.Kind != doc.Object {
		return nil, xerr.Failure("the document must be an object")
	}
	keyID := crypt.KeyID(pk)
	return doc.MapLeaves(plain, func(p string, leaf *doc.Value) (*doc.Value, error) {
		canon := doc.Canonical(leaf)
		if prev != nil && sk != nil && prev.Body != nil {
			if toks, err := doc.ParsePointer(p); err == nil {
				if old, err := doc.Lookup(prev.Body, toks); err == nil && old.Kind == doc.String {
					if id, ok := crypt.CiphertextKeyID(old.S); ok && id == keyID {
						if dec, err := crypt.DecryptValue(*sk, p, old.S); err == nil {
							same := string(dec) == string(canon)
							clear(dec)
							if same {
								return doc.NewString(old.S), nil
							}
						}
					}
				}
			}
		}
		ct, err := crypt.EncryptValue(pk, p, canon)
		if err != nil {
			return nil, xerr.Failure("%s: encryption failed", displayPointer(p))
		}
		return doc.NewString(ct), nil
	})
}

// DecryptDoc decrypts every leaf of body with sk.
func DecryptDoc(sk crypt.PrivateKey, body *doc.Value) (*doc.Value, error) {
	return DecryptAt(SingleKey(sk), "", body)
}

// DecryptDocKeys decrypts body, resolving each ciphertext's key through keys
// (so ciphertexts under retired keys can be migrated).
func DecryptDocKeys(keys KeyLookup, body *doc.Value) (*doc.Value, error) {
	return DecryptAt(keys, "", body)
}

// ComputeSync returns the sync baseline for a body whose plaintext is plain.
// Leaves of body without a plaintext counterpart are skipped.
func ComputeSync(sk crypt.PrivateKey, plain, body *doc.Value) map[string]SyncEntry {
	out := map[string]SyncEntry{}
	pl := LeafMap(plain)
	for _, l := range doc.Leaves(body) {
		pv, ok := pl[l.Pointer]
		if !ok || l.Value.Kind != doc.String {
			continue
		}
		out[l.Pointer] = SyncEntry{C: crypt.SyncTag(l.Value.S), H: crypt.PlainTag(sk, doc.Canonical(pv))}
	}
	return out
}

// LeafMap indexes the leaves of v by pointer. A nil v yields an empty map.
func LeafMap(v *doc.Value) map[string]*doc.Value {
	out := map[string]*doc.Value{}
	if v == nil {
		return out
	}
	for _, l := range doc.Leaves(v) {
		out[l.Pointer] = l.Value
	}
	return out
}

// CiphertextKeyIDs returns how many leaves of body are under each key ID.
func CiphertextKeyIDs(body *doc.Value) map[string]int {
	out := map[string]int{}
	for _, l := range doc.Leaves(body) {
		if l.Value.Kind == doc.String {
			if id, ok := crypt.CiphertextKeyID(l.Value.S); ok {
				out[id]++
			}
		}
	}
	return out
}

// Status states.
const (
	StateInSync             = "in-sync"
	StateUnencryptedNewer   = "unencrypted-newer"
	StateEncryptedNewer     = "encrypted-newer"
	StateConflict           = "conflict"
	StateUnencryptedMissing = "unencrypted-missing"
	StateKeyUnavailable     = "key-unavailable"
)

// Report is the read-only state of a pair.
type Report struct {
	Overall string
	Paths   []PathState
}

// PathState is the state of one leaf pointer.
type PathState struct{ Pointer, State string }

// Count returns how many paths are in state.
func (r Report) Count(state string) int {
	n := 0
	for _, p := range r.Paths {
		if p.State == state {
			n++
		}
	}
	return n
}

// Pointers returns the pointers in state.
func (r Report) Pointers(state string) []string {
	var out []string
	for _, p := range r.Paths {
		if p.State == state {
			out = append(out, p.Pointer)
		}
	}
	return out
}

// Status compares both sides of a pair against the sync baseline. plain is
// nil when the unencrypted file is missing; sk is nil when no matching
// private key is available. It never modifies anything.
func Status(p *Pair, e *Encrypted, plain *doc.Value, sk *crypt.PrivateKey) Report {
	enc := LeafMap(e.Body)
	pl := LeafMap(plain)
	pending := map[string]bool{}
	for _, x := range e.Meta.Pending {
		pending[x] = true
	}
	all := map[string]bool{}
	for k := range enc {
		all[k] = true
	}
	for k := range pl {
		all[k] = true
	}
	for k := range e.Meta.Sync {
		all[k] = true
	}
	for k := range pending {
		// A pending pointer for a removed leaf still counts.
		all[k] = true
	}
	ptrs := make([]string, 0, len(all))
	for k := range all {
		ptrs = append(ptrs, k)
	}
	sort.Strings(ptrs)

	var keyID string
	if sk != nil {
		keyID = crypt.KeyID(sk.Public())
	}
	var r Report
	for _, ptr := range ptrs {
		base, inBase := e.Meta.Sync[ptr]
		ct, inEnc := enc[ptr]
		encChanged := pending[ptr] || inBase != inEnc || (inBase && inEnc && crypt.SyncTag(ct.S) != base.C)
		state := StateInSync
		switch {
		case plain == nil || sk == nil:
			if encChanged {
				state = StateEncryptedNewer
			} else if plain != nil {
				state = StateKeyUnavailable
			}
			if plain == nil && !inEnc {
				// The leaf only exists in the baseline/pending list and the
				// encrypted side removed it.
				state = StateEncryptedNewer
			}
		default:
			pv, inPlain := pl[ptr]
			plainChanged := inBase != inPlain || (inBase && inPlain && crypt.PlainTag(*sk, doc.Canonical(pv)) != base.H)
			switch {
			case !encChanged && !plainChanged:
				state = StateInSync
			case plainChanged && !encChanged:
				state = StateUnencryptedNewer
			case encChanged && !plainChanged:
				state = StateEncryptedNewer
			default:
				state = StateConflict
				if !inEnc && !inPlain {
					state = StateInSync
				} else if inEnc && inPlain {
					if id, ok := crypt.CiphertextKeyID(ct.S); ok && id == keyID {
						if dec, err := crypt.DecryptValue(*sk, ptr, ct.S); err == nil {
							if string(dec) == string(doc.Canonical(pv)) {
								state = StateInSync
							}
							clear(dec)
						}
					}
				}
			}
		}
		r.Paths = append(r.Paths, PathState{Pointer: ptr, State: state})
	}
	switch {
	case plain == nil:
		r.Overall = StateUnencryptedMissing
	case sk == nil:
		r.Overall = StateKeyUnavailable
	case r.Count(StateConflict) > 0 || (r.Count(StateUnencryptedNewer) > 0 && r.Count(StateEncryptedNewer) > 0):
		r.Overall = StateConflict
	case r.Count(StateUnencryptedNewer) > 0:
		r.Overall = StateUnencryptedNewer
	case r.Count(StateEncryptedNewer) > 0:
		r.Overall = StateEncryptedNewer
	default:
		r.Overall = StateInSync
	}
	return r
}
