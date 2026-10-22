package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/CGuiho/zeze/internal/config"
	"github.com/CGuiho/zeze/internal/crypt"
	"github.com/CGuiho/zeze/internal/doc"
	"github.com/CGuiho/zeze/internal/keystore"
	"github.com/CGuiho/zeze/internal/pair"
	"github.com/CGuiho/zeze/internal/xerr"
	"github.com/spf13/cobra"
)

func (a *app) cmdKey() *cobra.Command {
	c := &cobra.Command{
		Use:   "key",
		Short: "show, list, export, rotate and retire keys",
	}
	c.AddCommand(a.cmdKeyShow(), a.cmdKeyList(), a.cmdKeyExport(), a.cmdKeyRotate(), a.cmdKeyRetire())
	return c
}

func (a *app) cmdKeyShow() *cobra.Command {
	var pf pairFlags
	var private, bypass bool
	c := &cobra.Command{
		Use:   "show",
		Short: "show the pair's public key, key ID and CI variable name (--private is human-only)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			e, err := loadEnv()
			if err != nil {
				return err
			}
			p, err := e.resolve(&pf)
			if err != nil {
				return err
			}
			enc, err := pair.ReadEncrypted(p.EncryptedPath)
			if err != nil {
				return err
			}
			sk, src, err := e.findKey(enc.Meta.KeyID)
			if err != nil {
				return err
			}
			if private {
				if err := e.protect(cmd, "reveal the private key "+enc.Meta.KeyID, []string{p.Rel(p.EncryptedPath)}, bypass); err != nil {
					return err
				}
				k, err := e.requireKey(enc)
				if err != nil {
					return err
				}
				sk = &k
			}
			label := ""
			if entries, err := e.store.List(); err == nil {
				for _, en := range entries {
					if en.KeyID == enc.Meta.KeyID {
						label = en.Label
					}
				}
			}
			outf(cmd, "keyId:      %s\n", enc.Meta.KeyID)
			outf(cmd, "label:      %s\n", keyLabel(label))
			outf(cmd, "publicKey:  %s\n", enc.Meta.PublicKey)
			outf(cmd, "ciVariable: %s\n", keystore.EnvName(enc.Meta.KeyID))
			if sk != nil {
				outf(cmd, "privateKey: available (%s)\n", src)
			} else {
				outf(cmd, "privateKey: not available on this machine\n")
			}
			if private && sk != nil {
				outf(cmd, "secret:     %s\n", sk.String())
			}
			return nil
		},
	}
	addPairFlags(c, &pf)
	c.Flags().BoolVar(&private, "private", false, "[human-only] also print the private key")
	addBypassFlag(c, &bypass, "skip the human-only gate for --private (only when a human explicitly asked; audited)")
	return c
}

func (a *app) cmdKeyList() *cobra.Command {
	c := &cobra.Command{
		Use:   "list",
		Short: "list the stored keys by label, short key ID and public-key prefix",
		Long: `Lists every key stored in ~/.guiho/zeze/keys/ (active keys first, then retired
keys), one line each: label, short key ID, the beginning of the public key, and the
state. Labels, key IDs and public-key prefixes are non-secret metadata; private keys
are never listed. Keys without a label (created before labels existed) show a
placeholder.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			e, err := loadEnv()
			if err != nil {
				return err
			}
			entries, err := e.store.List()
			if err != nil {
				return xerr.Failure("cannot list keys")
			}
			for _, line := range renderKeyLines(entries) {
				outf(cmd, "%s\n", line)
			}
			return nil
		},
	}
	return c
}

func (a *app) cmdKeyExport() *cobra.Command {
	var pf pairFlags
	var bypass bool
	c := &cobra.Command{
		Use:   "export [keyId]",
		Short: "[human-only] print a private key for backup",
		Long: `Prints the private key (zeze-sk-1:...) of the pair's key, or of the given key ID
(active or retired), so a human can back it up or store it as a CI secret.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			e, err := loadEnv()
			if err != nil {
				return err
			}
			keyID := ""
			if len(args) == 1 {
				keyID = args[0]
				if !crypt.ValidKeyID(keyID) {
					return xerr.Usage("the argument is not a key ID (k_ followed by 32 hex characters); pass the key ID, never the private key")
				}
			} else {
				p, err := e.resolve(&pf)
				if err != nil {
					return err
				}
				enc, err := pair.ReadEncrypted(p.EncryptedPath)
				if err != nil {
					return err
				}
				keyID = enc.Meta.KeyID
			}
			if err := e.protect(cmd, "export the private key "+keyID, nil, bypass); err != nil {
				return err
			}
			sk, _, err := e.findKey(keyID)
			if err != nil {
				return err
			}
			if sk == nil {
				return xerr.Key("private key %s not available", keyID)
			}
			outf(cmd, "%s\n", sk.String())
			return nil
		},
	}
	addPairFlags(c, &pf)
	addBypassFlag(c, &bypass, "skip the human-only gate (only when a human explicitly asked; audited)")
	return c
}

func (a *app) cmdKeyRotate() *cobra.Command {
	var pf pairFlags
	c := &cobra.Command{
		Use:   "rotate",
		Short: "generate a new key pair and re-encrypt every value to it",
		Long: `Generates a new key pair (saved to ~/.guiho/zeze/keys/), decrypts every value with the
old key (and retired keys for older ciphertexts), re-encrypts it to the new key, writes the
encrypted file atomically, and then moves the old key to keys/retired/. Every recorded
identity of the pair follows the new key: a headered unencrypted sibling is re-keyed (body,
$schema pointer, comments and formatting kept; a headerless plaintext stays headerless).
When zeze.yaml binds a key (key:), rotation is project-wide: every configured pair that
records the bound key is rotated to ONE new key (a pair recording another key refuses before
anything is written), and the binding is updated once with the same one-line splice. Rotation
does not revoke values already in git history: rotate the underlying secrets after a key leak.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			e, err := loadEnv()
			if err != nil {
				return err
			}
			// A bound project rotates as a whole: one new key for every
			// configured pair, one binding update, one retirement. The
			// single-pair path below is for projects without a binding (and
			// for a bound project that declares no pairs).
			if e.proj.Key != "" && len(e.proj.Pairs) > 0 {
				return rotateProject(cmd, e, &pf)
			}
			p, err := e.resolve(&pf)
			if err != nil {
				return err
			}
			unlock, err := e.lock(p)
			if err != nil {
				return err
			}
			defer unlock()
			enc, err := pair.ReadEncrypted(p.EncryptedPath)
			if err != nil {
				return err
			}
			// Preflight the sibling write before anything is written: a pair
			// zeze cannot read or repair refuses the rotation first, like
			// project-wide rotation does.
			plainFile, err := readPlainIfExists(p)
			if err != nil {
				return xerr.WrapMsg(xerr.CodeOf(err), xerr.KindOf(err), fmt.Sprintf("pair %s cannot be rotated", p.Name), err)
			}
			if enc.Meta.Format != pair.FormatVersion {
				return pair.LegacyHeader(filepath.Base(p.EncryptedPath))
			}
			if plainFile != nil && plainFile.Header != nil && plainFile.Header.Format != pair.FormatVersion {
				return pair.LegacyHeader(filepath.Base(p.UnencryptedPath))
			}
			oldSK, err := e.requireKey(enc)
			if err != nil {
				return err
			}
			plain, err := pair.DecryptDocKeys(pair.StoreKeys(&oldSK, e.store), enc.Body)
			if err != nil {
				return err
			}
			newSK, err := crypt.GenerateKey()
			if err != nil {
				return xerr.Failure("cannot generate a key")
			}
			newPK := newSK.Public()
			label := defaultKeyLabel(e.root, gitOriginURL)
			if _, err := e.store.SaveLabeled(newSK, label); err != nil {
				return xerr.Wrap(xerr.CodeKey, xerr.KindKey, err)
			}
			body, err := pair.EncryptDoc(newPK, plain, nil, nil)
			if err != nil {
				return err
			}
			next := &pair.Encrypted{Meta: enc.Meta, Body: body}
			next.Meta.PublicKey = newPK.String()
			next.Meta.KeyID = crypt.KeyID(newPK)
			next.Meta.Sync = map[string]pair.SyncEntry{}
			pending := append([]string{}, enc.Meta.Pending...)
			pl := pair.LeafMap(plain)
			for _, l := range doc.Leaves(body) {
				old, ok := enc.Meta.Sync[l.Pointer]
				canon := doc.Canonical(pl[l.Pointer])
				if ok && crypt.PlainTag(oldSK, canon) == old.H {
					next.Meta.Sync[l.Pointer] = pair.SyncEntry{C: crypt.SyncTag(l.Value.S), H: crypt.PlainTag(newSK, canon)}
				} else {
					pending = append(pending, l.Pointer)
				}
			}
			next.Meta.Pending = pair.SortedUnique(pending)
			// The embedded schema copy is a ciphertext like every value: it
			// follows the pair key.
			if next.Meta.SchemaCopy != "" {
				if text, err := enc.OpenSchemaCopy(pair.StoreKeys(&oldSK, e.store)); err == nil {
					serr := next.SetSchemaCopy(text)
					clear(text)
					if serr != nil {
						return serr
					}
				}
			}
			if err := pair.WriteEncrypted(p.EncryptedPath, next); err != nil {
				return err
			}
			// The unencrypted sibling's recorded identity follows the
			// rotation: a headered plaintext is re-keyed (body, $schema
			// pointer, comments and formatting kept; a headerless plaintext
			// stays headerless). Left on the old key it would disagree with
			// the new pair key and refuse every later set/check/status.
			updatedPlain := false
			if plainFile != nil && plainFile.Header != nil {
				h := *plainFile.Header
				h.Format = pair.FormatVersion
				h.PublicKey = newPK.String()
				h.KeyID = next.Meta.KeyID
				if h.Home == "" {
					h.Home = pair.HomeRef
				}
				write := &pair.PlainFile{Header: &h, Schema: plainFile.Schema, Body: plainFile.Body}
				if err := pair.WritePlainFile(p.UnencryptedPath, write); err != nil {
					return xerr.WrapMsg(xerr.CodeOf(err), xerr.KindOf(err),
						fmt.Sprintf("%s was rotated to %s but the %s identity in %s was not updated; the two files now disagree on the pair key and zeze refuses the pair until %s is fixed by hand", p.Rel(p.EncryptedPath), next.Meta.KeyID, pair.MetaKey, p.Rel(p.UnencryptedPath), p.Rel(p.UnencryptedPath)), err)
				}
				updatedPlain = true
			}
			retired := false
			if sharedBy := otherPairsUsing(e, p, enc.Meta.KeyID); len(sharedBy) > 0 {
				errf(cmd, "note: %s is still used by %s; it was not retired\n", enc.Meta.KeyID, strings.Join(sharedBy, ", "))
			} else if err := e.store.Retire(enc.Meta.KeyID); err != nil {
				if !errors.Is(err, keystore.ErrNoKey) {
					errf(cmd, "warning: the old key was not retired: %v\n", err)
				}
			} else {
				retired = true
			}
			outf(cmd, "rotated %s: %s -> %s (%d %s re-encrypted)\n", p.Rel(p.EncryptedPath), enc.Meta.KeyID, next.Meta.KeyID, len(doc.Leaves(body)), plural(len(doc.Leaves(body)), "value", "values"))
			if updatedPlain {
				outf(cmd, "updated the %s identity in %s: %s -> %s\n", pair.MetaKey, p.Rel(p.UnencryptedPath), enc.Meta.KeyID, next.Meta.KeyID)
			}
			if label != "" {
				outf(cmd, "the new key is labeled %s\n", label)
			}
			if retired {
				outf(cmd, "the old key was moved to ~/.guiho/zeze/keys/retired/\n")
			}
			outf(cmd, "update CI: set %s (the old %s is no longer used by this file)\n", keystore.EnvName(next.Meta.KeyID), keystore.EnvName(enc.Meta.KeyID))
			outf(cmd, "reminder: rotation does not revoke values already in git history; rotate leaked secrets themselves\n")
			// The zeze.yaml key binding follows the pair: an existing
			// binding is updated through the same one-line splice, an absent
			// one stays absent. A binding that cannot be updated is never
			// left silently stale: the rotation was reported above and the
			// command now fails.
			if e.proj.Key != "" {
				changed, err := config.BindProjectKey(e.root, next.Meta.KeyID)
				if err != nil {
					return xerr.WrapMsg(xerr.CodeFailure, xerr.KindFailure, fmt.Sprintf("the rotation succeeded but the %s key binding still names %s", config.ProjectFile, e.proj.Key), err)
				}
				if changed {
					outf(cmd, "updated the %s key binding: %s -> %s\n", config.ProjectFile, e.proj.Key, next.Meta.KeyID)
					e.proj.Key = next.Meta.KeyID
				}
			}
			return nil
		},
	}
	addPairFlags(c, &pf)
	return c
}

// rotationPair is one pair prepared for project-wide rotation. Every file is
// read, verified and rebuilt in memory before the first write.
type rotationPair struct {
	p         *pair.Pair
	enc       *pair.Encrypted // nil when the pair has no encrypted file
	plain     *pair.PlainFile // nil when the pair has no unencrypted file
	oldSK     crypt.PrivateKey
	values    *doc.Value // the decrypted values (enc pairs only)
	next      *pair.Encrypted
	nextPlain *pair.PlainFile // nil: the unencrypted file is left untouched
	leaves    int
}

// rotateProject rotates every configured pair that records the key bound in
// zeze.yaml to ONE new key: one generated key, every value re-encrypted to
// it (retired keys still open older ciphertexts), each pair's sync/pending
// rebuilt with the same rules as single-pair rotation, each embedded schema
// copy migrated, every recorded identity followed to the new key, the
// zeze.yaml binding updated once, and the old key retired when no pair
// still uses it. Pairs with no recorded identity are skipped with a note.
//
// Rotation is all-or-nothing for everything a pair can refuse: unreadable
// files, legacy zeze state, a missing key and a pair that records another
// key all fail before anything is written (a mismatched pair is exit 5 naming
// the pair and both key IDs — never a silent re-encrypt).
func rotateProject(cmd *cobra.Command, e *env, pf *pairFlags) error {
	bound := e.proj.Key
	// The rotation set: every configured pair, plus the explicitly selected
	// one when it is not among them (an explicit selection is never ignored).
	seen := map[string]bool{}
	var set []*pair.Pair
	add := func(q *pair.Pair) {
		if !seen[q.EncryptedPath] {
			seen[q.EncryptedPath] = true
			set = append(set, q)
		}
	}
	if pf.explicit() {
		q, err := e.resolve(pf)
		if err != nil {
			return err
		}
		add(q)
	}
	for _, pc := range e.proj.Pairs {
		q, err := pair.Resolve(pair.Options{Root: e.root, Name: pc.Name, SchemaFile: pf.schema, Config: e.proj})
		if err != nil {
			return xerr.WrapMsg(xerr.CodeOf(err), xerr.KindOf(err), "pair "+pc.Name, err)
		}
		add(q)
	}
	unlock, err := lockPairs(e, set)
	if err != nil {
		return err
	}
	defer unlock()

	// Phase 1: verify and prepare every pair in memory; no writes happen
	// until the whole set is ready.
	var preps []*rotationPair
	var skipped []string
	for _, q := range set {
		rp := &rotationPair{p: q}
		if fileExists(q.EncryptedPath) {
			if rp.enc, err = pair.ReadEncrypted(q.EncryptedPath); err != nil {
				return xerr.WrapMsg(xerr.CodeOf(err), xerr.KindOf(err), "pair "+q.Name+" cannot be rotated", err)
			}
		}
		if rp.plain, err = readPlainIfExists(q); err != nil {
			return xerr.WrapMsg(xerr.CodeOf(err), xerr.KindOf(err), "pair "+q.Name+" cannot be rotated", err)
		}
		recorded := ""
		if rp.enc != nil {
			recorded = rp.enc.Meta.KeyID
		} else if rp.plain != nil && rp.plain.Header != nil {
			recorded = rp.plain.Header.KeyID
		}
		if recorded == "" {
			skipped = append(skipped, q.Name)
			continue
		}
		if recorded != bound {
			return xerr.Key("pair %s records %s/keyId %s, but %s binds %s; nothing was rotated", q.Name, pair.MetaKey, recorded, config.ProjectFile, bound)
		}
		if rp.enc != nil && rp.enc.Meta.Format != pair.FormatVersion {
			return pair.LegacyHeader(filepath.Base(q.EncryptedPath))
		}
		if rp.plain != nil && rp.plain.Header != nil && rp.plain.Header.Format != pair.FormatVersion {
			return pair.LegacyHeader(filepath.Base(q.UnencryptedPath))
		}
		if rp.enc != nil {
			if rp.oldSK, err = e.requireKey(rp.enc); err != nil {
				return xerr.WrapMsg(xerr.CodeOf(err), xerr.KindOf(err), "pair "+q.Name, err)
			}
			if rp.values, err = pair.DecryptDocKeys(pair.StoreKeys(&rp.oldSK, e.store), rp.enc.Body); err != nil {
				return xerr.WrapMsg(xerr.CodeOf(err), xerr.KindOf(err), "pair "+q.Name, err)
			}
		}
		preps = append(preps, rp)
	}
	if len(preps) == 0 {
		return xerr.Failure("no configured pair records the key bound in %s (%s); nothing to rotate", config.ProjectFile, bound)
	}

	// Phase 2: one new key for the whole project, every pair rebuilt for it.
	newSK, err := crypt.GenerateKey()
	if err != nil {
		return xerr.Failure("cannot generate a key")
	}
	newPK := newSK.Public()
	newID := crypt.KeyID(newPK)
	for _, rp := range preps {
		if rp.enc != nil {
			body, err := pair.EncryptDoc(newPK, rp.values, nil, nil)
			if err != nil {
				return xerr.WrapMsg(xerr.CodeOf(err), xerr.KindOf(err), "pair "+rp.p.Name, err)
			}
			next := &pair.Encrypted{Meta: rp.enc.Meta, Body: body}
			next.Meta.PublicKey = newPK.String()
			next.Meta.KeyID = newID
			next.Meta.Sync = map[string]pair.SyncEntry{}
			pending := append([]string{}, rp.enc.Meta.Pending...)
			pl := pair.LeafMap(rp.values)
			for _, l := range doc.Leaves(body) {
				old, ok := rp.enc.Meta.Sync[l.Pointer]
				canon := doc.Canonical(pl[l.Pointer])
				if ok && crypt.PlainTag(rp.oldSK, canon) == old.H {
					next.Meta.Sync[l.Pointer] = pair.SyncEntry{C: crypt.SyncTag(l.Value.S), H: crypt.PlainTag(newSK, canon)}
				} else {
					pending = append(pending, l.Pointer)
				}
			}
			next.Meta.Pending = pair.SortedUnique(pending)
			// The embedded schema copy is a ciphertext like every value: it
			// follows the pair key.
			if next.Meta.SchemaCopy != "" {
				if text, err := rp.enc.OpenSchemaCopy(pair.StoreKeys(&rp.oldSK, e.store)); err == nil {
					serr := next.SetSchemaCopy(text)
					clear(text)
					if serr != nil {
						return xerr.WrapMsg(xerr.CodeOf(serr), xerr.KindOf(serr), "pair "+rp.p.Name, serr)
					}
				}
			}
			rp.next = next
			rp.leaves = len(doc.Leaves(body))
		}
		// Every recorded identity follows the new key: a headered unencrypted
		// file (the sibling, or a pair whose plaintext is its only file) is
		// re-keyed, or the pair would immediately disagree with the binding.
		if rp.plain != nil && rp.plain.Header != nil {
			h := *rp.plain.Header
			h.Format = pair.FormatVersion
			h.PublicKey = newPK.String()
			h.KeyID = newID
			if h.Home == "" {
				h.Home = pair.HomeRef
			}
			rp.nextPlain = &pair.PlainFile{Header: &h, Schema: rp.plain.Schema, Body: rp.plain.Body}
		}
	}
	label := defaultKeyLabel(e.root, gitOriginURL)
	if _, err := e.store.SaveLabeled(newSK, label); err != nil {
		return xerr.Wrap(xerr.CodeKey, xerr.KindKey, err)
	}

	// Phase 3: the writes — every pair file first, then the binding once.
	written := 0
	for _, rp := range preps {
		if rp.next != nil {
			if err := pair.WriteEncrypted(rp.p.EncryptedPath, rp.next); err != nil {
				return xerr.WrapMsg(xerr.CodeOf(err), xerr.KindOf(err),
					fmt.Sprintf("pair %s could not be written after %d of %d %s were rotated; the %s key binding still names %s", rp.p.Name, written, len(preps), plural(len(preps), "pair", "pairs"), config.ProjectFile, bound), err)
			}
			written++
		}
		if rp.nextPlain != nil {
			if err := pair.WritePlainFile(rp.p.UnencryptedPath, rp.nextPlain); err != nil {
				return xerr.WrapMsg(xerr.CodeOf(err), xerr.KindOf(err),
					fmt.Sprintf("pair %s could not be written after %d of %d %s were rotated; the %s key binding still names %s", rp.p.Name, written, len(preps), plural(len(preps), "pair", "pairs"), config.ProjectFile, bound), err)
			}
		}
	}
	if _, err := config.BindProjectKey(e.root, newID); err != nil {
		return xerr.WrapMsg(xerr.CodeFailure, xerr.KindFailure, fmt.Sprintf("the pairs were rotated to %s but the %s key binding still names %s", newID, config.ProjectFile, bound), err)
	}

	// Phase 4: retirement and the report.
	retired := false
	if sharedBy := otherPairsUsing(e, preps[0].p, bound); len(sharedBy) > 0 {
		errf(cmd, "note: %s is still used by %s; it was not retired\n", bound, strings.Join(sharedBy, ", "))
	} else if err := e.store.Retire(bound); err != nil {
		if !errors.Is(err, keystore.ErrNoKey) {
			errf(cmd, "warning: the old key was not retired: %v\n", err)
		}
	} else {
		retired = true
	}
	for _, rp := range preps {
		if rp.next != nil {
			outf(cmd, "rotated %s: %s -> %s (%d %s re-encrypted)\n", rp.p.Rel(rp.p.EncryptedPath), bound, newID, rp.leaves, plural(rp.leaves, "value", "values"))
		} else {
			outf(cmd, "re-keyed %s: %s -> %s (%s identity only; no encrypted file)\n", rp.p.Rel(rp.p.UnencryptedPath), bound, newID, pair.MetaKey)
		}
	}
	for _, name := range skipped {
		errf(cmd, "note: pair %s records no key; skipped\n", name)
	}
	if label != "" {
		outf(cmd, "the new key is labeled %s\n", label)
	}
	if retired {
		outf(cmd, "the old key was moved to ~/.guiho/zeze/keys/retired/\n")
	}
	outf(cmd, "updated the %s key binding: %s -> %s\n", config.ProjectFile, bound, newID)
	outf(cmd, "project key rotated: %s -> %s (%d %s)\n", bound, newID, len(preps), plural(len(preps), "pair", "pairs"))
	outf(cmd, "update CI: set %s (the old %s is no longer used by this project)\n", keystore.EnvName(newID), keystore.EnvName(bound))
	outf(cmd, "reminder: rotation does not revoke values already in git history; rotate leaked secrets themselves\n")
	return nil
}

// lockPairs takes every pair lock in a stable path order (so two concurrent
// project rotations cannot deadlock) and releases them in reverse order.
func lockPairs(e *env, set []*pair.Pair) (func(), error) {
	ordered := append([]*pair.Pair(nil), set...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].EncryptedPath < ordered[j].EncryptedPath })
	var unlocks []func()
	for _, q := range ordered {
		unlock, err := e.lock(q)
		if err != nil {
			for i := len(unlocks) - 1; i >= 0; i-- {
				unlocks[i]()
			}
			return nil, err
		}
		unlocks = append(unlocks, unlock)
	}
	return func() {
		for i := len(unlocks) - 1; i >= 0; i-- {
			unlocks[i]()
		}
	}, nil
}

// otherPairsUsing lists configured pairs (other than p) whose file uses keyID.
func otherPairsUsing(e *env, p *pair.Pair, keyID string) []string {
	var out []string
	if e.proj == nil {
		return nil
	}
	for _, pc := range e.proj.Pairs {
		q, err := pair.Resolve(pair.Options{Root: e.root, Name: pc.Name, Config: e.proj})
		if err != nil || q.EncryptedPath == p.EncryptedPath {
			continue
		}
		if enc, err := pair.ReadEncrypted(q.EncryptedPath); err == nil {
			copyKey, _ := crypt.CiphertextKeyID(enc.Meta.SchemaCopy)
			if enc.Meta.KeyID == keyID || copyKey == keyID || pair.CiphertextKeyIDs(enc.Body)[keyID] > 0 {
				out = append(out, q.Name)
			}
		}
	}
	return out
}

// projectKeyUsage counts, per encrypted file under root, the uses of keyID.
func projectKeyUsage(root, keyID string) map[string]int {
	out := map[string]int{}
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor", ".venv", "dist", "build":
				if path != root {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".json") || !strings.Contains(d.Name(), pair.EncryptedMarker) {
			return nil
		}
		enc, err := pair.ReadEncrypted(path)
		if err != nil {
			return nil
		}
		n := pair.CiphertextKeyIDs(enc.Body)[keyID]
		if id, ok := crypt.CiphertextKeyID(enc.Meta.SchemaCopy); ok && id == keyID {
			n++
		}
		if enc.Meta.KeyID == keyID && n == 0 {
			n = 1
		}
		if n > 0 {
			rel, _ := filepath.Rel(root, path)
			out[filepath.ToSlash(rel)] = n
		}
		return nil
	})
	return out
}

func (a *app) cmdKeyRetire() *cobra.Command {
	c := &cobra.Command{
		Use:   "retire <keyId>",
		Short: "delete a retired key once no ciphertext in this project uses it",
		Long: `Permanently deletes ~/.guiho/zeze/keys/retired/<keyId>.json. Refuses while any
*.encrypted.json file under the project root still references the key (run zeze encrypt
there to migrate). Back the key up first with zeze key export <keyId>.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			keyID := args[0]
			if !crypt.ValidKeyID(keyID) {
				return xerr.Usage("the argument is not a key ID (k_ followed by 32 hex characters); pass the key ID, never the private key")
			}
			e, err := loadEnv()
			if err != nil {
				return err
			}
			entries, err := e.store.List()
			if err != nil {
				return xerr.Failure("cannot list keys")
			}
			state := ""
			for _, en := range entries {
				if en.KeyID == keyID {
					if en.Retired {
						state = "retired"
					} else if state == "" {
						state = "active"
					}
				}
			}
			switch state {
			case "":
				return xerr.Key("key %s not found in ~/.guiho/zeze/keys/retired/", keyID)
			case "active":
				return xerr.Failure("key %s is active; run zeze key rotate first", keyID)
			}
			usage := projectKeyUsage(e.root, keyID)
			if len(usage) > 0 {
				files := make([]string, 0, len(usage))
				for f, n := range usage {
					files = append(files, f+" ("+itoa(n)+")")
				}
				sort.Strings(files)
				return xerr.Failure("key %s is still used by %s; run zeze encrypt there first", keyID, strings.Join(files, ", "))
			}
			if err := e.store.Delete(keyID); err != nil {
				return xerr.Wrap(xerr.CodeKey, xerr.KindKey, err)
			}
			outf(cmd, "deleted retired key %s\n", keyID)
			return nil
		},
	}
	return c
}
