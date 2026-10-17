package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/CGuiho/jureuma/internal/agentres"
	"github.com/CGuiho/jureuma/internal/config"
	"github.com/CGuiho/jureuma/internal/crypt"
	"github.com/CGuiho/jureuma/internal/doc"
	"github.com/CGuiho/jureuma/internal/gate"
	"github.com/CGuiho/jureuma/internal/keystore"
	"github.com/CGuiho/jureuma/internal/pair"
	"github.com/CGuiho/jureuma/internal/paths"
	"github.com/CGuiho/jureuma/internal/schema"
	"github.com/CGuiho/jureuma/internal/update"
	"github.com/CGuiho/jureuma/internal/xerr"
	"github.com/spf13/cobra"
)

type initReport struct {
	cmd     *cobra.Command
	created []string
	updated []string
	same    []string
	notes   []string
	fatal   []string
}

func (r *initReport) step(format string, a ...any) { outf(r.cmd, "  "+format+"\n", a...) }

func evolutionFields(e *config.Evolution) []struct {
	name string
	ptr  *string
} {
	return []struct {
		name string
		ptr  *string
	}{
		{"upgrade", &e.Upgrade},
		{"issues.bugs", &e.Issues.Bugs},
		{"issues.improvements", &e.Issues.Improvements},
		{"issues.reviews", &e.Issues.Reviews},
	}
}

const evolutionExplanation = `agent.evolution controls what AI agents may do to evolve jureuma (global configuration,
~/.guiho/jureuma/jureuma.global.yaml; a project's jureuma.yaml may override single fields):
  disabled        never upgrade jureuma or create issues on your behalf
  always-ask      ask you every time before upgrading or creating an issue (default)
  always-proceed  upgrade and file bug/improvement/review issues, then report the result
Recommended: always-proceed, so agents keep jureuma current and report what they find.
`

// askEvolution fills unset policies interactively. It reports whether anything changed.
func askEvolution(cmd *cobra.Command, ev *config.Evolution) bool {
	fields := evolutionFields(ev)
	missing := 0
	for _, f := range fields {
		if !config.ValidPolicy(*f.ptr) {
			missing++
		}
	}
	if missing == 0 {
		return false
	}
	errf(cmd, "\n%s\n", evolutionExplanation)
	if askYesNo(cmd, "[global] Apply always-proceed to all agent-evolution actions?", false) {
		for _, f := range fields {
			if !config.ValidPolicy(*f.ptr) {
				*f.ptr = config.PolicyAlwaysProceed
			}
		}
		return true
	}
	for _, f := range fields {
		if config.ValidPolicy(*f.ptr) {
			continue
		}
		ans, ok := ask(cmd, "[global] agent.evolution."+f.name+" (disabled / always-ask / always-proceed) [always-ask]: ")
		if !ok || !config.ValidPolicy(ans) {
			ans = config.PolicyAlwaysAsk
		}
		*f.ptr = ans
	}
	return true
}

func (a *app) cmdInit() *cobra.Command {
	var evolution string
	var denyRulesFlag, keyID, keyLabel string
	c := &cobra.Command{
		Use:   "init",
		Short: "initialize or reconcile jureuma for this project (idempotent)",
		Long: `Installs the bundled agent skills, applies the jureuma block to AGENTS.md, ensures
~/.guiho/jureuma/jureuma.global.yaml and the project's jureuma.yaml exist and are valid, asks
the agent-evolution policy questions when an interactive terminal is present (defaults to
always-ask otherwise), and for every configured pair:
  - with an existing encrypted file: never generates keys; reports key availability; repairs
    the $jureuma identity header (format 2), the schema metadata and the embedded schema copy
    in place, keeping every value, sync and pending state;
  - without one, with the identity recorded in the unencrypted file's $jureuma header: builds
    the encrypted sibling from that document under that key (no private key needed);
  - without one and with no recorded identity: chooses the pair key — a stored key picked in
    a terminal (or named with --key-id / --key-label), a new key otherwise — then creates an
    empty encrypted file and an empty, git-ignored unencrypted file;
  - adds or repairs the unencrypted file's $jureuma identity header and $schema pointer,
    keeping its body exactly;
  - ensures the .gitignore block and the schema (a permissive starter when missing);
  - offers deny rules for *.unencrypted.* in the agent tools' settings (Claude Code, OpenCode).
Init is idempotent: an already-repaired pair is left byte-identical. Fails unless the
.gitignore block is present, no unencrypted file is tracked by git, every schema is present
and supported, and every keyId matches its publicKey.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if evolution != "" && !config.ValidPolicy(evolution) {
				return xerr.Usage("--agent-evolution must be disabled, always-ask or always-proceed")
			}
			switch denyRulesFlag {
			case "", "yes", "no":
			default:
				return xerr.Usage("--agent-deny-rules must be yes or no")
			}
			r := &initReport{cmd: cmd}
			wd, err := os.Getwd()
			if err != nil {
				return xerr.Failure("cannot determine the current directory")
			}
			root, err := paths.ProjectRoot(wd)
			if err != nil {
				return xerr.Failure("cannot resolve the project root")
			}
			home, err := paths.Home()
			if err != nil {
				return xerr.Failure("cannot resolve the jureuma home")
			}
			if err := os.MkdirAll(home, 0o700); err != nil {
				return xerr.Failure("cannot create %s", home)
			}
			tty := interactive()
			outf(cmd, "jureuma init\n  project root: %s\n  jureuma home: %s\n", root, home)

			// Skills.
			outf(cmd, "agent skills\n")
			dests, err := skillDests()
			if err != nil {
				return err
			}
			for _, s := range agentres.Skills() {
				for _, d := range dests {
					t := filepath.Join(d, s.ID)
					if agentres.SkillInstalledCurrent(s, d) {
						r.step("current   %s", t)
						continue
					}
					existed := fileExists(t)
					if err := agentres.InstallSkill(s, d); err != nil {
						return xerr.Failure("%s: %v", t, err)
					}
					if existed {
						r.step("upgraded  %s", t)
					} else {
						r.step("installed %s", t)
					}
				}
			}

			// AGENTS.md.
			outf(cmd, "AGENTS.md\n")
			agents := filepath.Join(root, "AGENTS.md")
			res, err := applyInstruction(agents, false)
			if err != nil {
				return err
			}
			r.step("%-9s %s", res, agents)

			// Global configuration.
			outf(cmd, "configuration\n")
			glob, gExisted, err := config.LoadGlobal(home)
			if err != nil {
				return xerr.Wrap(xerr.CodeFailure, xerr.KindFailure, err)
			}
			gChanged := !gExisted
			if !gExisted {
				glob = config.DefaultGlobal()
				glob.Agent.Evolution = config.Evolution{}
			}
			if glob.Protections.AllowBypass == nil && !gExisted {
				t := true
				glob.Protections.AllowBypass = &t
			}
			ev := &glob.Agent.Evolution
			if evolution != "" {
				for _, f := range evolutionFields(ev) {
					if !config.ValidPolicy(*f.ptr) {
						*f.ptr = evolution
						gChanged = true
					}
				}
			}
			if tty && askEvolution(cmd, ev) {
				gChanged = true
			}
			var defaulted []string
			for _, f := range evolutionFields(ev) {
				if !config.ValidPolicy(*f.ptr) {
					*f.ptr = config.DefaultPolicy
					gChanged = true
					defaulted = append(defaulted, f.name)
				}
			}
			if len(defaulted) > 0 {
				r.notes = append(r.notes, "agent.evolution "+strings.Join(defaulted, ", ")+" defaulted to always-ask (global; rerun jureuma init in a terminal or pass --agent-evolution to choose)")
			}
			globalPath := filepath.Join(home, config.GlobalFile)
			if gChanged {
				if err := config.SaveGlobal(home, glob, a.configVersion()); err != nil {
					return xerr.Wrap(xerr.CodeFailure, xerr.KindFailure, err)
				}
				if gExisted {
					r.step("updated   %s", globalPath)
				} else {
					r.step("created   %s", globalPath)
				}
			} else {
				r.step("valid     %s", globalPath)
			}
			if !gExisted {
				if err := config.SealProtections(home, glob); err != nil {
					return xerr.Failure("cannot seal the protections block")
				}
			} else if intact, _ := config.ProtectionsIntact(home, glob); !intact {
				if tty {
					errf(cmd, "The protections block in %s does not match its seal (it was edited by hand).\n", globalPath)
					if err := gate.New(glob.Protections.AgentEnvMarkers).Check("re-seal the protections block"); err == nil {
						if err := config.SealProtections(home, glob); err != nil {
							return xerr.Failure("cannot seal the protections block")
						}
						r.step("re-sealed the protections block")
					} else {
						r.notes = append(r.notes, "protections block not re-sealed: --dangerously-bypass-protections stays refused")
					}
				} else {
					r.notes = append(r.notes, "the protections block does not match its seal: --dangerously-bypass-protections is refused until a human re-runs jureuma init in a terminal")
				}
			}

			// Project configuration.
			proj, pExisted, err := config.LoadProject(root)
			if err != nil {
				return xerr.Wrap(xerr.CodeFailure, xerr.KindFailure, err)
			}
			projPath := filepath.Join(root, config.ProjectFile)
			if !pExisted {
				proj = config.DefaultProject()
				proj.Agent = config.AgentConfig{} // inherit agent.evolution from the global file
				if err := config.SaveProject(root, proj, a.configVersion()); err != nil {
					return xerr.Wrap(xerr.CodeFailure, xerr.KindFailure, err)
				}
				r.step("created   %s", projPath)
			} else {
				r.step("valid     %s", projPath)
			}
			eff := config.Effective(proj, glob)
			outf(cmd, "  agent.evolution: upgrade=%s bugs=%s improvements=%s reviews=%s\n", eff.Evolution.Upgrade, eff.Evolution.Issues.Bugs, eff.Evolution.Issues.Improvements, eff.Evolution.Issues.Reviews)

			// Domain steps.
			e := &env{root: root, home: home, proj: proj, glob: glob}
			e.store = keystore.New(home)
			pairs, err := e.pairs(&pairFlags{})
			if err != nil {
				return err
			}
			if changed, err := pair.EnsureGitignoreBlock(root); err != nil {
				return err
			} else if changed {
				r.step("updated   %s (jureuma managed block)", filepath.Join(root, ".gitignore"))
			}
			for _, p := range pairs {
				if err := initPair(r, e, p, keyID, keyLabel); err != nil {
					return err
				}
			}

			// Deny rules.
			outf(cmd, "agent-tool deny rules\n")
			settings := claudeSettingsPath(root)
			if denyRulesPresent(root) {
				r.step("present   %s", settings)
			} else {
				apply := denyRulesFlag == "yes"
				if denyRulesFlag == "" && tty {
					apply = askYesNo(cmd, "[project] Add deny rules for *.unencrypted.* and ~/.guiho/jureuma to "+settings+" (recommended)?", true)
				}
				if apply {
					n, err := applyDenyRules(home, root)
					if err != nil {
						return err
					}
					r.step("added %d %s to %s", n, plural(n, "rule", "rules"), settings)
				} else {
					r.step("skipped   (rerun with --agent-deny-rules yes to add them)")
				}
			}
			ocfg := opencodeConfigPath(root)
			if opencodeDenyRulesPresent(root) {
				r.step("present   %s", ocfg)
			} else {
				apply := denyRulesFlag == "yes"
				if denyRulesFlag == "" && tty {
					apply = askYesNo(cmd, "[project] Add deny rules for *.unencrypted.* and ~/.guiho/jureuma to "+ocfg+" (recommended)?", true)
				}
				if apply {
					n, err := applyOpencodeDenyRules(home, root)
					if err != nil {
						return err
					}
					r.step("added %d %s to %s", n, plural(n, "rule", "rules"), ocfg)
				} else {
					r.step("skipped   (rerun with --agent-deny-rules yes to add them)")
				}
			}

			// Revalidation.
			outf(cmd, "verification\n")
			for _, s := range agentres.Skills() {
				for _, d := range dests {
					if !agentres.SkillInstalledCurrent(s, d) {
						r.fatal = append(r.fatal, "skill "+filepath.Join(d, s.ID)+" is not current")
					}
				}
			}
			if data, err := os.ReadFile(agents); err != nil || !agentres.HasCurrentBlock(data, agentres.Instruction()) {
				r.fatal = append(r.fatal, "AGENTS.md block is not current")
			}
			if _, _, err := config.LoadGlobal(home); err != nil {
				r.fatal = append(r.fatal, err.Error())
			}
			if _, _, err := config.LoadProject(root); err != nil {
				r.fatal = append(r.fatal, err.Error())
			}
			if !pair.HasGitignoreBlock(root) {
				r.fatal = append(r.fatal, ".gitignore block is missing")
			}
			for _, p := range pairs {
				verifyPair(r, e, p)
			}
			for _, n := range r.notes {
				outf(cmd, "  note: %s\n", n)
			}
			if len(r.fatal) > 0 {
				for _, f := range r.fatal {
					errf(cmd, "  fail: %s\n", f)
				}
				return xerr.Failure("init found %d %s", len(r.fatal), plural(len(r.fatal), "problem", "problems"))
			}
			outf(cmd, "jureuma is initialized for %s\n", root)
			return nil
		},
	}
	c.Flags().StringVar(&evolution, "agent-evolution", "", "set every unset agent.evolution policy (disabled, always-ask, always-proceed) without asking")
	c.Flags().StringVar(&denyRulesFlag, "agent-deny-rules", "", "add (yes) or skip (no) the agent-tool deny rules without asking")
	c.Flags().StringVar(&keyID, "key-id", "", "use this stored key (full or short key ID) for a pair with no recorded identity")
	c.Flags().StringVar(&keyLabel, "key-label", "", "use the stored key carrying this label for a pair with no recorded identity")
	return c
}

// configVersion is the release version used in the schema reference header;
// development builds reference 0.0.0-dev's nearest release form.
func (a *app) configVersion() string {
	v := trimV(a.version)
	if _, ok := update.Parse(v); ok {
		return v
	}
	return "0.0.0"
}

func initPair(r *initReport, e *env, p *pair.Pair, keyID, keyLabel string) error {
	outf(r.cmd, "pair %s\n", p.Name)
	if err := pair.CheckSchemaName(p.SchemaPath); err != nil {
		return err
	}
	if p.SchemaSource == pair.SchemaFromFlag && !pair.Within(p.Root, p.SchemaPath) {
		// The reference would be committed in $jureuma.schema, which may not
		// point outside the project.
		return xerr.Usage("--schema-file must stay inside the project root for init")
	}
	// Both sides are read before anything is written: a file jureuma cannot
	// parse is reported and left untouched, never overwritten blindly.
	pf, err := readPlainIfExists(p)
	if err != nil {
		return xerr.Failure("%v; fix or remove the %s block by hand, then re-run jureuma init", err, pair.MetaKey)
	}
	var enc *pair.Encrypted
	if fileExists(p.EncryptedPath) {
		if enc, err = pair.ReadEncrypted(p.EncryptedPath); err != nil {
			return xerr.Failure("%v; fix or remove the %s block by hand, then re-run jureuma init", err, pair.MetaKey)
		}
	}
	warns, err := pair.CheckIdentity(pf, enc)
	if err != nil {
		return err
	}
	r.notes = append(r.notes, warns...)

	var h *pair.Header // the identity the pair keeps
	var sk *crypt.PrivateKey
	if enc != nil {
		// An existing pair never gets a new key (key rotate does that).
		h1 := enc.Meta.Identity()
		h = &h1
		r.step("exists    %s (%s)", p.EncryptedPath, enc.Meta.KeyID)
		sk, _, err = e.findKey(enc.Meta.KeyID)
		switch {
		case err != nil:
			r.step("key       %v", err)
		case sk == nil:
			r.step("key       private key not available (expected on agent and CI machines)")
		default:
			r.step("key       private key %s available", enc.Meta.KeyID)
		}
		if keyID != "" || keyLabel != "" {
			r.notes = append(r.notes, "--key-id/--key-label were not applied: the pair already has a key (jureuma key rotate changes it)")
		}
	} else {
		// No encrypted file: the identity comes from the unencrypted file's
		// $jureuma header when it has one, else from a stored key chosen for
		// this pair (picker in a terminal, --key-id / --key-label, or a new
		// key by default).
		if pf != nil && pf.Header != nil && (keyID != "" || keyLabel != "") {
			return xerr.Usage("the pair key is recorded in %s; drop --key-id/--key-label (jureuma key rotate changes the key)", p.Rel(p.UnencryptedPath))
		}
		if pf != nil && pf.Header != nil {
			h1 := *pf.Header
			h = &h1
			r.step("identity  %s (from %s)", h.KeyID, p.Rel(p.UnencryptedPath))
		} else {
			chosen, created, err := chooseInitKey(r, e, p, keyID, keyLabel)
			if err != nil {
				return err
			}
			sk = &chosen
			pk := chosen.Public()
			h = &pair.Header{Format: pair.FormatVersion, PublicKey: pk.String(), KeyID: crypt.KeyID(pk), Home: pair.HomeRef}
			if created != "" {
				r.step("generated %s", created)
			}
			r.step("identity  %s", h.KeyID)
		}
		if h.Home == "" {
			h.Home = pair.HomeRef
		}
		if sk == nil {
			if k, _, err := e.findKey(h.KeyID); err != nil {
				r.notes = append(r.notes, err.Error())
			} else {
				sk = k
			}
		}
	}
	if h.Format == 0 {
		h.Format = pair.FormatVersion
	}

	// The schema is ensured once the pair identity is resolved: the pair files
	// written below point at it and embed its document.
	if !fileExists(p.SchemaPath) {
		if err := paths.AtomicWrite(p.SchemaPath, doc.Marshal(schema.Starter()), 0o644); err != nil {
			return xerr.Failure("%s: cannot write file", p.SchemaPath)
		}
		r.step("created   %s (permissive starter; run jureuma schema infer)", p.SchemaPath)
	} else {
		r.step("exists    %s", p.SchemaPath)
	}

	// The encrypted side: created (empty, or built from the unencrypted
	// document when there is one) or repaired in place, never re-keyed.
	if enc == nil {
		enc = plainIdentity(h, p)
		body := doc.NewObject()
		if pf != nil && len(pf.Body.Keys) > 0 {
			body = pf.Body.Clone()
		}
		next, err := buildSibling(e, h, p, body, nil, sk)
		if err != nil {
			return err
		}
		if err := pair.RepairEncrypted(p.EncryptedPath, next); err != nil {
			return err
		}
		if len(body.Keys) > 0 {
			r.step("created   %s (built from %s)", p.EncryptedPath, p.Rel(p.UnencryptedPath))
		} else {
			r.step("created   %s", p.EncryptedPath)
		}
	} else {
		next := &pair.Encrypted{Meta: enc.Meta, Body: enc.Body}
		changed := enc.Meta.Format != pair.FormatVersion
		if next.Meta.Home == "" {
			next.Meta.Home = pair.HomeRef
			changed = true
		}
		if next.Meta.Schema != p.SchemaRef() {
			next.Meta.Schema = p.SchemaRef()
			changed = true
		}
		if err := setSchemaCopy(e, next, p, false); err != nil {
			return err
		}
		if next.Meta.SchemaCopy != enc.Meta.SchemaCopy {
			changed = true
		}
		if changed {
			if err := pair.RepairEncrypted(p.EncryptedPath, next); err != nil {
				return err
			}
			r.step("repaired  %s (%s identity header and schema metadata)", p.EncryptedPath, pair.MetaKey)
		}
	}

	// The unencrypted side: the body is kept exactly; the $jureuma identity
	// header and the $schema pointer are added or repaired.
	if pf == nil {
		pf = &pair.PlainFile{Body: doc.NewObject()}
	}
	hadPlain := fileExists(p.UnencryptedPath)
	schemaRef := p.SchemaRef()
	need := !hadPlain || pf.Header == nil || pf.Header.Format != pair.FormatVersion || pf.Header.Home == "" || pf.Schema != schemaRef
	if need {
		write := &pair.PlainFile{Body: pf.Body}
		if pf.Header != nil {
			hh := *pf.Header
			write.Header = &hh
		} else {
			hh := *h
			write.Header = &hh
		}
		write.Header.Format = pair.FormatVersion
		if write.Header.Home == "" {
			write.Header.Home = pair.HomeRef
		}
		write.Schema = schemaRef
		if err := pair.RepairPlainFile(p.UnencryptedPath, write); err != nil {
			return err
		}
		if !hadPlain {
			r.step("created   %s (empty, git-ignored)", p.UnencryptedPath)
		} else {
			r.step("repaired  %s (%s identity header and %s pointer)", p.UnencryptedPath, pair.MetaKey, pair.SchemaKey)
		}
	} else {
		r.step("exists    %s", p.UnencryptedPath)
	}
	return nil
}

// chooseInitKey resolves the key for a pair with no recorded identity. An
// explicit --key-id or --key-label names a stored key; a terminal offers the
// stored keys through the picker plus a "create a new key" choice; without a
// terminal a new key is created. New keys carry the default repository-name
// label. The path of a created key is returned so init can report it.
func chooseInitKey(r *initReport, e *env, p *pair.Pair, keyID, keyLabel string) (crypt.PrivateKey, string, error) {
	entries, err := e.store.List()
	if err != nil {
		return crypt.PrivateKey{}, "", xerr.Wrap(xerr.CodeKey, xerr.KindKey, err)
	}
	if keyID != "" && keyLabel != "" {
		return crypt.PrivateKey{}, "", xerr.Usage("--key-id and --key-label name one key; give only one of them")
	}
	if keyID != "" || keyLabel != "" {
		var match *keystore.Entry
		matches := 0
		for i := range entries {
			en := &entries[i]
			if keyID != "" {
				if en.KeyID == keyID || shortKeyID(en.KeyID) == keyID {
					match = en
					break
				}
				continue
			}
			if oneLine(en.Label) == oneLine(keyLabel) {
				matches++
				match = en
			}
		}
		if match == nil {
			if keyID != "" {
				return crypt.PrivateKey{}, "", xerr.Key("no stored key %s", keyID)
			}
			return crypt.PrivateKey{}, "", xerr.Key("no stored key carries the label %q", keyLabel)
		}
		if matches > 1 {
			return crypt.PrivateKey{}, "", xerr.Usage("%d stored keys carry the label %q; select one with --key-id", matches, keyLabel)
		}
		sk, _, err := e.findKey(match.KeyID)
		if err != nil {
			return crypt.PrivateKey{}, "", err
		}
		if sk == nil {
			return crypt.PrivateKey{}, "", xerr.Key("private key %s is not available", match.KeyID)
		}
		return *sk, "", nil
	}
	const newKeyChoice = "create a new key"
	if interactive() {
		listed := append(append([]keystore.Entry{}, entries...), keystore.Entry{Label: newKeyChoice})
		en, perr := pickKey(listed, os.Stdin, r.cmd.ErrOrStderr(), true)
		switch {
		case perr == nil && en.KeyID != "":
			sk, _, err := e.findKey(en.KeyID)
			if err != nil {
				return crypt.PrivateKey{}, "", err
			}
			if sk == nil {
				return crypt.PrivateKey{}, "", xerr.Key("private key %s is not available", en.KeyID)
			}
			return *sk, "", nil
		case perr == nil:
			// "create a new key" — fall through to creation.
		case errors.Is(perr, ErrNoKeySelected):
			return crypt.PrivateKey{}, "", xerr.Failure("no key selected; nothing was written for %s", p.Name)
		case errors.Is(perr, ErrNoStoredKeys):
			// Nothing to choose from: a new key.
		default:
			return crypt.PrivateKey{}, "", perr
		}
	}
	sk, err := crypt.GenerateKey()
	if err != nil {
		return crypt.PrivateKey{}, "", xerr.Failure("cannot generate a key")
	}
	path, err := e.store.SaveLabeled(sk, defaultKeyLabel(e.root, gitOriginURL))
	if err != nil {
		return crypt.PrivateKey{}, "", xerr.Wrap(xerr.CodeKey, xerr.KindKey, err)
	}
	return sk, path, nil
}

func verifyPair(r *initReport, e *env, p *pair.Pair) {
	enc, err := pair.ReadEncrypted(p.EncryptedPath)
	if err != nil {
		r.fatal = append(r.fatal, p.Name+": "+err.Error())
		return
	}
	if fileExists(p.UnencryptedPath) {
		if tracked, _ := pair.TrackedByGit(e.root, p.UnencryptedPath); tracked {
			r.fatal = append(r.fatal, p.Rel(p.UnencryptedPath)+" is tracked by git")
		}
	}
	s, err := loadSchema(p)
	switch {
	case err != nil:
		r.fatal = append(r.fatal, p.Name+": "+err.Error())
	case s == nil:
		r.fatal = append(r.fatal, p.Name+": schema "+p.Rel(p.SchemaPath)+" is missing")
	case schema.IsStarter(s.Raw()):
		r.notes = append(r.notes, p.Name+": "+p.Rel(p.SchemaPath)+" is still the starter schema (run jureuma schema infer, then refine it)")
	}
	if sk, _, err := e.findKey(enc.Meta.KeyID); err == nil && sk != nil {
		plain, _ := readPlainIfExists(p)
		var body *doc.Value
		if plain != nil {
			body = plain.Body
		}
		rep := pair.Status(p, enc, body, sk)
		outf(r.cmd, "  %s: sync state %s\n", p.Name, rep.Overall)
	} else {
		outf(r.cmd, "  %s: private key not available (expected on agent and CI machines)\n", p.Name)
	}
}
