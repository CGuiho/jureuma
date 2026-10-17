package cli

import (
	"errors"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/CGuiho/jureuma/internal/crypt"
	"github.com/CGuiho/jureuma/internal/doc"
	"github.com/CGuiho/jureuma/internal/keystore"
	"github.com/CGuiho/jureuma/internal/pair"
	"github.com/CGuiho/jureuma/internal/xerr"
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
		Long: `Lists every key stored in ~/.guiho/jureuma/keys/ (active keys first, then retired
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
		Long: `Prints the private key (jureuma-sk-1:...) of the pair's key, or of the given key ID
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
		Long: `Generates a new key pair (saved to ~/.guiho/jureuma/keys/), decrypts every value with the
old key (and retired keys for older ciphertexts), re-encrypts it to the new key, writes the
encrypted file atomically, and then moves the old key to keys/retired/. Rotation does not
revoke values already in git history: rotate the underlying secrets after a key leak.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			e, err := loadEnv()
			if err != nil {
				return err
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
			if label != "" {
				outf(cmd, "the new key is labeled %s\n", label)
			}
			if retired {
				outf(cmd, "the old key was moved to ~/.guiho/jureuma/keys/retired/\n")
			}
			outf(cmd, "update CI: set %s (the old %s is no longer used by this file)\n", keystore.EnvName(next.Meta.KeyID), keystore.EnvName(enc.Meta.KeyID))
			outf(cmd, "reminder: rotation does not revoke values already in git history; rotate leaked secrets themselves\n")
			return nil
		},
	}
	addPairFlags(c, &pf)
	return c
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
		Long: `Permanently deletes ~/.guiho/jureuma/keys/retired/<keyId>.json. Refuses while any
*.encrypted.json file under the project root still references the key (run jureuma encrypt
there to migrate). Back the key up first with jureuma key export <keyId>.`,
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
				return xerr.Key("key %s not found in ~/.guiho/jureuma/keys/retired/", keyID)
			case "active":
				return xerr.Failure("key %s is active; run jureuma key rotate first", keyID)
			}
			usage := projectKeyUsage(e.root, keyID)
			if len(usage) > 0 {
				files := make([]string, 0, len(usage))
				for f, n := range usage {
					files = append(files, f+" ("+itoa(n)+")")
				}
				sort.Strings(files)
				return xerr.Failure("key %s is still used by %s; run jureuma encrypt there first", keyID, strings.Join(files, ", "))
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
