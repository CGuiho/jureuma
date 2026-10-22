package cli

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/CGuiho/zeze/internal/config"
	"github.com/CGuiho/zeze/internal/crypt"
	"github.com/CGuiho/zeze/internal/gate"
	"github.com/CGuiho/zeze/internal/keystore"
	"github.com/CGuiho/zeze/internal/pair"
	"github.com/CGuiho/zeze/internal/paths"
	"github.com/CGuiho/zeze/internal/schema"
	"github.com/CGuiho/zeze/internal/xerr"
	"github.com/spf13/cobra"
)

// pairFlags are the common flags of pair-aware commands.
type pairFlags struct {
	name, enc, unenc, schema string
}

func addPairFlags(c *cobra.Command, pf *pairFlags) {
	f := c.Flags()
	f.StringVar(&pf.name, "name", "", "pair stem, e.g. production or apps/api/config (default: first pair in zeze.yaml, else config)")
	f.StringVar(&pf.enc, "encrypted-file", "", "encrypted file path (must contain .encrypted.)")
	f.StringVar(&pf.unenc, "unencrypted-file", "", "unencrypted file path (must contain .unencrypted.)")
	f.StringVar(&pf.schema, "schema-file", "", "schema file path (overrides zeze.yaml and $zeze.schema)")
}

func (pf *pairFlags) explicit() bool {
	return pf.name != "" || pf.enc != "" || pf.unenc != ""
}

// env is the loaded project/global context.
type env struct {
	root    string
	home    string
	proj    *config.Project
	glob    *config.Global
	globErr error
	store   *keystore.Store
}

func loadEnv() (*env, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, xerr.Failure("cannot determine the current directory")
	}
	root, err := paths.ProjectRoot(wd)
	if err != nil {
		return nil, xerr.Failure("cannot resolve the project root")
	}
	home, err := paths.Home()
	if err != nil {
		return nil, xerr.Failure("cannot resolve the user home directory")
	}
	proj, _, err := config.LoadProject(root)
	if err != nil {
		return nil, xerr.Wrap(xerr.CodeOf(err), xerr.KindOf(err), err)
	}
	e := &env{root: root, home: home, proj: proj, store: keystore.New(home)}
	e.glob, _, e.globErr = config.LoadGlobal(home)
	if e.glob == nil {
		e.glob = &config.Global{}
	}
	return e, nil
}

func (e *env) effective() config.EffectiveConfig {
	return config.Effective(e.proj, e.glob)
}

func (e *env) resolve(pf *pairFlags) (*pair.Pair, error) {
	return e.resolvePair(pair.Options{Root: e.root, Name: pf.name, EncryptedFile: pf.enc, UnencryptedFile: pf.unenc, SchemaFile: pf.schema, Config: e.proj})
}

// resolvePair resolves one pair and enforces the project key binding. Every
// command that resolves pairs goes through here (or through pairs, which uses
// it), so the agreement check exists in exactly one place.
func (e *env) resolvePair(o pair.Options) (*pair.Pair, error) {
	p, err := pair.Resolve(o)
	if err != nil {
		return nil, err
	}
	return p, e.checkProjectKey(p)
}

// checkProjectKey enforces zeze.yaml's key binding against the identity the
// pair's files record — the encrypted file's $zeze.keyId, or the
// unencrypted file's when the encrypted file is missing. A disagreement is a
// key error (exit 5) naming the binding and the file; it is never a silent
// re-encrypt. A pair with no recorded identity (both files absent, or a
// headerless plaintext) is not a disagreement.
func (e *env) checkProjectKey(p *pair.Pair) error {
	if e.proj == nil || e.proj.Key == "" {
		return nil
	}
	recorded := recordedKeyID(p)
	if recorded == "" || recorded == e.proj.Key {
		return nil
	}
	file := p.Rel(p.EncryptedPath)
	if !fileExists(p.EncryptedPath) {
		file = p.Rel(p.UnencryptedPath)
	}
	return xerr.Key("%s: %s/keyId %s does not match the key bound in %s (%s)", file, pair.MetaKey, recorded, config.ProjectFile, e.proj.Key)
}

// recordedKeyID returns the key ID the pair's files record — the encrypted
// file's $zeze.keyId, or the unencrypted file's when the encrypted file is
// missing ("" when neither records one). Files that cannot be read are
// skipped: their own read reports the problem, which keeps init able to
// repair them.
func recordedKeyID(p *pair.Pair) string {
	if fileExists(p.EncryptedPath) {
		if enc, err := pair.ReadEncrypted(p.EncryptedPath); err == nil {
			return enc.Meta.KeyID
		}
		return ""
	}
	if pf, err := readPlainIfExists(p); err == nil && pf != nil && pf.Header != nil {
		return pf.Header.KeyID
	}
	return ""
}

// pairs returns the pairs a project-wide command (check) acts on.
func (e *env) pairs(pf *pairFlags) ([]*pair.Pair, error) {
	if pf.explicit() || e.proj == nil || len(e.proj.Pairs) == 0 {
		p, err := e.resolve(pf)
		if err != nil {
			return nil, err
		}
		return []*pair.Pair{p}, nil
	}
	var out []*pair.Pair
	for _, pc := range e.proj.Pairs {
		p, err := e.resolvePair(pair.Options{Root: e.root, Name: pc.Name, SchemaFile: pf.schema, Config: e.proj})
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

// findKey returns the private key for keyID, or nil when no source has it.
func (e *env) findKey(keyID string) (*crypt.PrivateKey, keystore.Source, error) {
	sk, src, err := e.store.Find(keyID)
	if err != nil {
		if errors.Is(err, keystore.ErrNoKey) {
			return nil, "", nil
		}
		return nil, "", xerr.Wrap(xerr.CodeKey, xerr.KindKey, err)
	}
	return &sk, src, nil
}

// requireKey returns the private key matching the encrypted file and checks
// that it derives $zeze.publicKey.
func (e *env) requireKey(enc *pair.Encrypted) (crypt.PrivateKey, error) {
	sk, _, err := e.findKey(enc.Meta.KeyID)
	if err != nil {
		return crypt.PrivateKey{}, err
	}
	if sk == nil {
		return crypt.PrivateKey{}, xerr.Key("private key %s not available (set %s or restore %s)", enc.Meta.KeyID, keystore.EnvName(enc.Meta.KeyID), filepath.Join("~", ".guiho", "zeze", "keys", enc.Meta.KeyID+".json"))
	}
	if sk.Public().String() != enc.Meta.PublicKey {
		return crypt.PrivateKey{}, xerr.Key("the private key does not match $zeze.publicKey")
	}
	return *sk, nil
}

func (e *env) lock(p *pair.Pair) (func(), error) {
	unlock, err := paths.Lock(e.home, p.EncryptedPath)
	if err != nil {
		if errors.Is(err, paths.ErrLocked) {
			return nil, xerr.Failure("%s is locked by another zeze process", p.Rel(p.EncryptedPath))
		}
		return nil, xerr.Failure("cannot take the pair lock")
	}
	return unlock, nil
}

// loadSchema loads the pair's schema; (nil, nil) when the file is missing.
func loadSchema(p *pair.Pair) (*schema.Schema, error) {
	if _, err := os.Stat(p.SchemaPath); errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	s, err := schema.Load(p.SchemaPath)
	if err != nil {
		return nil, xerr.Wrap(xerr.CodeValidation, xerr.KindValidation, err)
	}
	return s, nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// exeName appends the platform's executable suffix to a binary name.
func exeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// fileSHA256 returns the lowercase hex SHA-256 of a file.
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// readPlainIfExists returns nil when the unencrypted file is missing. The
// full pair reader is the only plaintext parser in the CLI: it keeps the
// $zeze identity header and the top-level $schema pointer apart from the
// body, so no flow can turn reserved metadata into a value or drop it on
// write.
func readPlainIfExists(p *pair.Pair) (*pair.PlainFile, error) {
	if !fileExists(p.UnencryptedPath) {
		return nil, nil
	}
	return pair.ReadPlainFile(p.UnencryptedPath, p.UnencryptedIsJSONC)
}

// checkIdentity runs the cross-file identity check (one pair belongs to one
// key) and prints its warnings. Either side may be absent or carry no
// plaintext header; the check is then vacuous.
func checkIdentity(cmd *cobra.Command, plain *pair.PlainFile, enc *pair.Encrypted) error {
	warnings, err := pair.CheckIdentity(plain, enc)
	if err != nil {
		return err
	}
	for _, w := range warnings {
		errf(cmd, "warning: %s\n", w)
	}
	return nil
}

// schemaCopyState reports the state of the schema copy embedded in the
// encrypted file: "none" when no copy is stored, "match" or "differs" when a
// key holder compared it with the local schema file, and "unchecked" when no
// available key can open it. It never prints schema content.
func schemaCopyState(e *env, p *pair.Pair, enc *pair.Encrypted, sk *crypt.PrivateKey) string {
	if enc == nil || enc.Meta.SchemaCopy == "" {
		return "none"
	}
	if sk == nil {
		return "unchecked"
	}
	text, err := enc.OpenSchemaCopy(pair.StoreKeys(sk, e.store))
	if err != nil {
		return "unchecked"
	}
	defer clear(text)
	local, err := os.ReadFile(p.SchemaPath)
	if err != nil {
		return "unchecked"
	}
	if bytes.Equal(local, text) {
		return "match"
	}
	return "differs"
}

// setSchemaCopy embeds the local schema document in the encrypted metadata
// (as $zeze.schema { path, c }). When a copy is already stored and want is
// true, it is replaced only when a key holder can prove it differs from the
// local file, or when the stored copy is encrypted under a key other than the
// file's (a retired key), so unchanged writes stay byte-identical while
// retired-key copies migrate.
func setSchemaCopy(e *env, enc *pair.Encrypted, p *pair.Pair, want bool) error {
	if !fileExists(p.SchemaPath) {
		return nil
	}
	if enc.Meta.SchemaCopy != "" && !want {
		return nil
	}
	text, err := os.ReadFile(p.SchemaPath)
	if err != nil {
		return xerr.Failure("%s: cannot read file", p.Rel(p.SchemaPath))
	}
	if enc.Meta.SchemaCopy != "" && want {
		if id, ok := crypt.CiphertextKeyID(enc.Meta.SchemaCopy); ok && id != enc.Meta.KeyID {
			// Under another key (usually retired): re-encrypt it.
		} else if sk, _, err := e.findKey(enc.Meta.KeyID); err == nil && sk != nil {
			if cur, err := enc.OpenSchemaCopy(pair.StoreKeys(sk, e.store)); err == nil {
				same := bytes.Equal(cur, text)
				clear(cur)
				if same {
					return nil
				}
			}
		} else {
			return nil
		}
	}
	return enc.SetSchemaCopy(text)
}

func issuesErr(cmd *cobra.Command, file string, issues []schema.Issue) error {
	for _, is := range issues {
		errf(cmd, "  %s\n", is.String())
	}
	return xerr.Validation("%s: %d validation issue(s)", file, len(issues))
}

// ---- protections ----

const protectedMsg = "[PROTECTED] %s is human-only. A human must run it in an interactive terminal."

func (e *env) audit(cmd *cobra.Command, files []string, outcome string, bypass bool) error {
	eff := e.effective()
	return gate.Audit(e.home, gate.AuditEvent{
		Time:    time.Now().UTC(),
		Command: cmd.CommandPath(),
		Paths:   files,
		Outcome: outcome,
		Markers: gate.DetectAgents(os.Getenv, eff.Protections.AgentEnvMarkers),
		Bypass:  bypass,
	})
}

// checkBypass validates --dangerously-bypass-protections: allowed by the
// global configuration, protections block intact, warning printed, audited.
func (e *env) checkBypass(cmd *cobra.Command, files []string) error {
	if paths.HomeRelocated() {
		return xerr.Protected("[PROTECTED] --dangerously-bypass-protections is refused while %s relocates the home away from the user's home directory", paths.HomeOverrideEnv)
	}
	if e.globErr != nil {
		_ = e.audit(cmd, files, "bypass-refused", true)
		return xerr.Protected("[PROTECTED] --dangerously-bypass-protections is refused: %s is invalid", config.GlobalFile)
	}
	if !e.glob.Protections.BypassAllowed() {
		_ = e.audit(cmd, files, "bypass-refused", true)
		return xerr.Protected("[PROTECTED] --dangerously-bypass-protections is disabled by protections.allowBypass")
	}
	intact, err := config.ProtectionsIntact(e.home, e.glob)
	if err != nil || !intact {
		_ = e.audit(cmd, files, "bypass-refused", true)
		return xerr.Protected("[PROTECTED] --dangerously-bypass-protections is refused: the protections block in %s does not match its seal (a human must re-run zeze init interactively)", config.GlobalFile)
	}
	if err := e.audit(cmd, files, "bypassed", true); err != nil {
		return xerr.Protected("[PROTECTED] --dangerously-bypass-protections is refused: the audit log cannot be written")
	}
	errf(cmd, "warning: --dangerously-bypass-protections: %s runs without the human-only gate; this was recorded in the audit log\n", cmd.CommandPath())
	return nil
}

// protect runs the human-only gate (or validates the bypass).
func (e *env) protect(cmd *cobra.Command, action string, files []string, bypass bool) error {
	if bypass {
		return e.checkBypass(cmd, files)
	}
	g := gate.New(e.effective().Protections.AgentEnvMarkers)
	if err := g.Check(action); err != nil {
		_ = e.audit(cmd, files, "refused", false)
		return xerr.Protected(protectedMsg, cmd.CommandPath())
	}
	_ = e.audit(cmd, files, "allowed", false)
	return nil
}

func addBypassFlag(c *cobra.Command, p *bool, usage string) {
	c.Flags().BoolVar(p, "dangerously-bypass-protections", false, usage)
}

// ---- interactive prompts ----

func interactive() bool {
	return gate.IsTerminal(os.Stdin) && gate.IsTerminal(os.Stderr)
}

var stdinReader *bufio.Reader

func ask(cmd *cobra.Command, question string) (string, bool) {
	errf(cmd, "%s", question)
	if stdinReader == nil {
		stdinReader = bufio.NewReader(os.Stdin)
	}
	line, err := stdinReader.ReadString('\n')
	if err != nil && line == "" {
		errf(cmd, "\n")
		return "", false
	}
	return strings.TrimSpace(line), true
}

func askYesNo(cmd *cobra.Command, question string, def bool) bool {
	suffix := " [y/N] "
	if def {
		suffix = " [Y/n] "
	}
	ans, ok := ask(cmd, question+suffix)
	if !ok || ans == "" {
		return def
	}
	switch strings.ToLower(ans) {
	case "y", "yes":
		return true
	case "n", "no":
		return false
	}
	return def
}
