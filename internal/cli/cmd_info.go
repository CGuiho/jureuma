package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/CGuiho/jureuma/internal/doc"
	"github.com/CGuiho/jureuma/internal/pair"
	"github.com/CGuiho/jureuma/internal/paths"
	"github.com/CGuiho/jureuma/internal/runner"
	"github.com/CGuiho/jureuma/internal/schema"
	"github.com/CGuiho/jureuma/internal/xerr"
	"github.com/spf13/cobra"
)

func checkOutput(s string) error {
	if s != "text" && s != "json" {
		return xerr.Usage("--output must be text or json")
	}
	return nil
}

func writeJSON(cmd *cobra.Command, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	outf(cmd, "%s\n", data)
	return nil
}

// ---- list ----

func (a *app) cmdList() *cobra.Command {
	var pf pairFlags
	var output string
	c := &cobra.Command{
		Use:   "list",
		Short: "list pointers, flattened env names and types (never values)",
		Long: `Lists every configuration path of the encrypted file with its flattened environment
variable name and its type. Without the private key the type comes from the schema; with
the key it is the actual JSON type. Values are never printed.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := checkOutput(output); err != nil {
				return err
			}
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
			sch, serr := loadSchema(p)
			if serr != nil {
				errf(cmd, "warning: %v\n", serr)
			}
			var plain *doc.Value
			sk, _, kerr := e.findKey(enc.Meta.KeyID)
			if kerr != nil {
				errf(cmd, "warning: %v\n", kerr)
			} else if sk != nil && sk.Public().String() == enc.Meta.PublicKey {
				if d, err := pair.DecryptDocKeys(pair.StoreKeys(sk, e.store), enc.Body); err == nil {
					plain = d
				} else {
					errf(cmd, "warning: %v\n", err)
				}
			}
			eff := e.effective()
			type row struct {
				Pointer string `json:"pointer"`
				Env     string `json:"env"`
				Type    string `json:"type"`
			}
			var rows []row
			var ptrs []string
			actual := pair.LeafMap(plain)
			for _, l := range doc.Leaves(enc.Body) {
				ptrs = append(ptrs, l.Pointer)
				name, err := runner.EnvName(l.Pointer, eff.Run.Prefix)
				if err != nil {
					name = "-"
				}
				typ := "?"
				if v, ok := actual[l.Pointer]; ok {
					typ = v.Kind.String()
				} else if sch != nil {
					if ts := sch.ExpectedTypes(l.Pointer); len(ts) > 0 {
						typ = strings.Join(ts, "|")
					}
				}
				rows = append(rows, row{l.Pointer, name, typ})
			}
			problems := runner.NameProblems(ptrs, eff.Run.Prefix)
			if output == "json" {
				return writeJSON(cmd, map[string]any{
					"pair": p.Name, "encryptedFile": p.Rel(p.EncryptedPath), "keyId": enc.Meta.KeyID,
					"keyAvailable": plain != nil, "paths": rows, "envNameProblems": problems,
				})
			}
			keyState := "private key not available: types from the schema"
			if plain != nil {
				keyState = "private key available: actual types"
			}
			outf(cmd, "%s (%s, %s)\n", p.Rel(p.EncryptedPath), enc.Meta.KeyID, keyState)
			if len(rows) == 0 {
				outf(cmd, "  (no values)\n")
			} else {
				tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
				fmt.Fprintln(tw, "POINTER\tENV\tTYPE")
				for _, r := range rows {
					fmt.Fprintf(tw, "%s\t%s\t%s\n", r.Pointer, r.Env, r.Type)
				}
				tw.Flush()
			}
			for _, pr := range problems {
				errf(cmd, "warning: %s\n", pr)
			}
			return nil
		},
	}
	addPairFlags(c, &pf)
	c.Flags().StringVar(&output, "output", "text", "output format: text or json")
	return c
}

// ---- status ----

var stateHints = map[string]string{
	pair.StateUnencryptedNewer:   "run jureuma encrypt",
	pair.StateEncryptedNewer:     "a human runs jureuma decrypt",
	pair.StateConflict:           "a human resolves it, then encrypt --force or decrypt --force",
	pair.StateUnencryptedMissing: "normal on agent and CI machines",
	pair.StateKeyUnavailable:     "normal on agent machines; only encrypted-side changes are detected",
}

func (a *app) cmdStatus() *cobra.Command {
	var pf pairFlags
	var output string
	c := &cobra.Command{
		Use:   "status",
		Short: "show the sync state of the pair, per path (read-only)",
		Long: `Compares the unencrypted and encrypted files with the last sync and reports, per path and
overall: in-sync, unencrypted-newer, encrypted-newer, conflict, unencrypted-missing or
key-unavailable. Never modifies anything and never prints values.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := checkOutput(output); err != nil {
				return err
			}
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
			sk, _, err := e.findKey(enc.Meta.KeyID)
			if err != nil {
				return err
			}
			if sk != nil && sk.Public().String() != enc.Meta.PublicKey {
				return xerr.Key("the private key does not match $jureuma.publicKey")
			}
			plain, err := readPlainIfExists(p)
			if err != nil {
				return err
			}
			rep := pair.Status(p, enc, plain, sk)
			headWarn := ""
			if hp, ok := pair.HeadPublicKey(p.EncryptedPath); ok && hp != enc.Meta.PublicKey {
				headWarn = "$jureuma.publicKey differs from the version at git HEAD; verify the recipient key before encrypting new values"
			}
			if output == "json" {
				type ps struct {
					Pointer string `json:"pointer"`
					State   string `json:"state"`
				}
				var list []ps
				for _, x := range rep.Paths {
					list = append(list, ps{x.Pointer, x.State})
				}
				out := map[string]any{"pair": p.Name, "overall": rep.Overall, "paths": list, "pending": pair.SortedUnique(enc.Meta.Pending), "keyId": enc.Meta.KeyID}
				if headWarn != "" {
					out["warnings"] = []string{headWarn}
				}
				return writeJSON(cmd, out)
			}
			outf(cmd, "%s: %s\n", p.Name, rep.Overall)
			if h, ok := stateHints[rep.Overall]; ok {
				outf(cmd, "  (%s)\n", h)
			}
			outf(cmd, "  encrypted:   %s\n", p.Rel(p.EncryptedPath))
			outf(cmd, "  unencrypted: %s", p.Rel(p.UnencryptedPath))
			if plain == nil {
				outf(cmd, " (missing)")
			}
			outf(cmd, "\n  key:         %s (%s)\n", enc.Meta.KeyID, map[bool]string{true: "available", false: "not available"}[sk != nil])
			outf(cmd, "  paths:       %d in-sync", rep.Count(pair.StateInSync))
			for _, st := range []string{pair.StateUnencryptedNewer, pair.StateEncryptedNewer, pair.StateConflict, pair.StateKeyUnavailable} {
				if n := rep.Count(st); n > 0 {
					outf(cmd, ", %d %s", n, st)
				}
			}
			outf(cmd, "\n")
			for _, x := range rep.Paths {
				if x.State != pair.StateInSync && x.State != pair.StateKeyUnavailable {
					outf(cmd, "  %-18s %s\n", x.State, x.Pointer)
				}
			}
			if headWarn != "" {
				errf(cmd, "warning: %s\n", headWarn)
			}
			return nil
		},
	}
	addPairFlags(c, &pf)
	c.Flags().StringVar(&output, "output", "text", "output format: text or json")
	return c
}

// ---- check ----

type checker struct {
	cmd      *cobra.Command
	failCode int
	fails    int
	warns    int
}

func (c *checker) ok(format string, a ...any) { outf(c.cmd, "ok    "+format+"\n", a...) }
func (c *checker) info(format string, a ...any) {
	outf(c.cmd, "info  "+format+"\n", a...)
}
func (c *checker) warn(format string, a ...any) {
	c.warns++
	outf(c.cmd, "warn  "+format+"\n", a...)
}
func (c *checker) fail(code int, format string, a ...any) {
	c.fails++
	outf(c.cmd, "fail  "+format+"\n", a...)
	rank := map[int]int{xerr.CodeKey: 3, xerr.CodeValidation: 2, xerr.CodeFailure: 1}
	if rank[code] > rank[c.failCode] {
		c.failCode = code
	}
}

func (a *app) cmdCheck() *cobra.Command {
	var pf pairFlags
	c := &cobra.Command{
		Use:   "check",
		Short: "check schema, gitignore, tracked plaintext, keys, env names and deny rules",
		Long: `Checks every configured pair (or the selected one): the encrypted file and its keyId,
the .gitignore block, that no unencrypted file is tracked by git, the schema (present,
supported, still the starter, secret paths carrying default/examples/const/enum), the
structure of the encrypted file, and with the private key the whole decrypted document.
Also warns when $jureuma.publicKey differs from git HEAD, when values are encrypted under
other keys, when flattened env names collide, and when the agent-tool deny rules are
absent. Exit 5 for key problems, 4 for validation problems, 1 for anything else.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			e, err := loadEnv()
			if err != nil {
				return err
			}
			ch := &checker{cmd: cmd}
			if e.globErr != nil {
				ch.fail(xerr.CodeFailure, "%v", e.globErr)
			}
			if pair.HasGitignoreBlock(e.root) {
				ch.ok(".gitignore contains the jureuma managed block")
			} else {
				ch.fail(xerr.CodeFailure, ".gitignore lacks the jureuma managed block (run jureuma init)")
			}
			pairs, err := e.pairs(&pf)
			if err != nil {
				return err
			}
			for _, p := range pairs {
				a.checkPair(ch, e, p)
			}
			if denyRulesPresent(e.root) {
				ch.ok("agent-tool deny rules for *.unencrypted.* are present")
			} else {
				ch.warn("agent-tool deny rules for *.unencrypted.* are absent (jureuma init offers to add them)")
			}
			if opencodeDenyRulesPresent(e.root) {
				ch.ok("OpenCode deny rules for *.unencrypted.* are present")
			} else {
				ch.warn("OpenCode deny rules for *.unencrypted.* are absent (jureuma init offers to add them)")
			}
			outf(cmd, "%d %s, %d %s\n", ch.fails, plural(ch.fails, "failure", "failures"), ch.warns, plural(ch.warns, "warning", "warnings"))
			if ch.fails > 0 {
				return &silentError{code: ch.failCode}
			}
			return nil
		},
	}
	addPairFlags(c, &pf)
	return c
}

func (a *app) checkPair(ch *checker, e *env, p *pair.Pair) {
	label := p.Name
	enc, err := pair.ReadEncrypted(p.EncryptedPath)
	if err != nil {
		ch.fail(xerr.CodeOf(err), "%s: %v", label, err)
		return
	}
	ch.ok("%s: %s is valid (keyId matches publicKey)", label, p.Rel(p.EncryptedPath))
	if fileExists(p.UnencryptedPath) {
		tracked, _ := pair.TrackedByGit(e.root, p.UnencryptedPath)
		if tracked {
			ch.fail(xerr.CodeFailure, "%s: %s is tracked by git (run git rm --cached on it)", label, p.Rel(p.UnencryptedPath))
		} else if ign, ok := pair.IgnoredByGit(p.UnencryptedPath); ok && !ign {
			ch.fail(xerr.CodeFailure, "%s: %s is not ignored by git", label, p.Rel(p.UnencryptedPath))
		} else {
			ch.ok("%s: %s is git-ignored and untracked", label, p.Rel(p.UnencryptedPath))
		}
	} else {
		ch.info("%s: %s is missing (normal on agent and CI machines)", label, p.Rel(p.UnencryptedPath))
	}
	for _, stale := range staleTemps(p.UnencryptedPath) {
		ch.warn("%s: %s is a temporary file left by an interrupted write and may hold plaintext; delete it", label, p.Rel(stale))
	}
	var sch *schema.Schema
	if !fileExists(p.SchemaPath) {
		ch.fail(xerr.CodeValidation, "%s: schema %s is missing (run jureuma schema infer)", label, p.Rel(p.SchemaPath))
	} else if s, err := loadSchema(p); err != nil {
		ch.fail(xerr.CodeValidation, "%s: %v", label, err)
	} else {
		sch = s
		ch.ok("%s: schema %s is supported", label, p.Rel(p.SchemaPath))
		if schema.IsStarter(s.Raw()) {
			ch.warn("%s: %s is still the permissive starter schema (run jureuma schema infer, then refine it)", label, p.Rel(p.SchemaPath))
		}
		for _, is := range s.SecretValueKeywords() {
			ch.warn("%s: schema %s", label, is.String())
		}
		if issues := s.ValidateStructure(enc.Body); len(issues) > 0 {
			for _, is := range issues {
				ch.fail(xerr.CodeValidation, "%s: %s", label, is.String())
			}
		} else {
			ch.ok("%s: the encrypted structure matches the schema", label)
		}
	}
	if hp, ok := pair.HeadPublicKey(p.EncryptedPath); ok && hp != enc.Meta.PublicKey {
		ch.warn("%s: $jureuma.publicKey differs from the version at git HEAD; verify the recipient key", label)
	}
	ids := pair.CiphertextKeyIDs(enc.Body)
	others := make([]string, 0)
	for id := range ids {
		if id != enc.Meta.KeyID {
			others = append(others, id)
		}
	}
	sort.Strings(others)
	for _, id := range others {
		ch.warn("%s: %d %s encrypted under %s, not the file's key (run jureuma encrypt to migrate)", label, ids[id], plural(ids[id], "value is", "values are"), id)
	}
	eff := e.effective()
	var ptrs []string
	for _, l := range doc.Leaves(enc.Body) {
		ptrs = append(ptrs, l.Pointer)
	}
	for _, pr := range runner.NameProblems(ptrs, eff.Run.Prefix) {
		if eff.Run.Format == runner.FormatFlatten {
			ch.fail(xerr.CodeValidation, "%s: %s", label, pr)
		} else {
			ch.warn("%s: %s (only matters for --format flatten)", label, pr)
		}
	}
	sk, src, err := e.findKey(enc.Meta.KeyID)
	if err != nil {
		ch.fail(xerr.CodeKey, "%s: %v", label, err)
		return
	}
	if sk == nil {
		ch.info("%s: private key %s not available (expected on agent and CI machines)", label, enc.Meta.KeyID)
		return
	}
	if sk.Public().String() != enc.Meta.PublicKey {
		ch.fail(xerr.CodeKey, "%s: the private key does not match $jureuma.publicKey", label)
		return
	}
	ch.ok("%s: private key %s available (%s)", label, enc.Meta.KeyID, src)
	plain, err := pair.DecryptDocKeys(pair.StoreKeys(sk, e.store), enc.Body)
	if err != nil {
		ch.fail(xerr.CodeKey, "%s: %v", label, err)
		return
	}
	if sch != nil {
		if issues := sch.Validate(plain); len(issues) > 0 {
			for _, is := range issues {
				ch.fail(xerr.CodeValidation, "%s: %s", label, is.String())
			}
		} else {
			ch.ok("%s: the decrypted document is valid", label)
		}
	}
	if pl, err := readPlainIfExists(p); err != nil {
		ch.fail(xerr.CodeOf(err), "%s: %v", label, err)
	} else if pl != nil {
		rep := pair.Status(p, enc, pl, sk)
		ch.info("%s: sync state %s", label, rep.Overall)
	}
}

// staleTemps returns temporary files an interrupted atomic write left beside
// the unencrypted file (".<stem>.tmp-*.unencrypted.*" and the older
// ".<name>.tmp-*" form).
func staleTemps(unencrypted string) []string {
	dir, base := filepath.Split(unencrypted)
	stem := base
	if i := strings.Index(base, pair.UnencryptedMarker); i > 0 {
		stem = base[:i]
	}
	var out []string
	for _, pat := range []string{"." + stem + paths.TempMarker + "*", "." + base + paths.TempMarker + "*"} {
		m, _ := filepath.Glob(filepath.Join(dir, pat))
		out = append(out, m...)
	}
	sort.Strings(out)
	uniq := out[:0]
	for i, x := range out {
		if i == 0 || x != out[i-1] {
			uniq = append(uniq, x)
		}
	}
	return uniq
}
