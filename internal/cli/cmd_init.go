package cli

import (
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
	var denyRulesFlag string
	c := &cobra.Command{
		Use:   "init",
		Short: "initialize or reconcile jureuma for this project (idempotent)",
		Long: `Installs the bundled agent skills, applies the jureuma block to AGENTS.md, ensures
~/.guiho/jureuma/jureuma.global.yaml and the project's jureuma.yaml exist and are valid, asks
the agent-evolution policy questions when an interactive terminal is present (defaults to
always-ask otherwise), and for every configured pair:
  - with an existing encrypted file: never generates keys; reports key availability;
  - without one: generates a key pair (~/.guiho/jureuma/keys/), an empty encrypted file and an
    empty, git-ignored unencrypted file;
  - ensures the .gitignore block and the schema (a permissive starter when missing);
  - offers deny rules for *.unencrypted.* in the agent tools' settings.
Fails unless the .gitignore block is present, no unencrypted file is tracked by git, every
schema is present and supported, and every keyId matches its publicKey.`,
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
				if err := initPair(r, e, p); err != nil {
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

func initPair(r *initReport, e *env, p *pair.Pair) error {
	outf(r.cmd, "pair %s\n", p.Name)
	if err := pair.CheckSchemaName(p.SchemaPath); err != nil {
		return err
	}
	if p.SchemaSource == pair.SchemaFromFlag && !pair.Within(p.Root, p.SchemaPath) {
		// The reference would be committed in $jureuma.schema, which may not
		// point outside the project.
		return xerr.Usage("--schema-file must stay inside the project root for init")
	}
	if fileExists(p.EncryptedPath) {
		enc, err := pair.ReadEncrypted(p.EncryptedPath)
		if err != nil {
			return err
		}
		r.step("exists    %s (%s)", p.EncryptedPath, enc.Meta.KeyID)
		sk, _, err := e.findKey(enc.Meta.KeyID)
		switch {
		case err != nil:
			r.step("key       %v", err)
		case sk == nil:
			r.step("key       private key not available (expected on agent and CI machines)")
		default:
			r.step("key       private key %s available", enc.Meta.KeyID)
		}
	} else {
		sk, err := crypt.GenerateKey()
		if err != nil {
			return xerr.Failure("cannot generate a key")
		}
		path, err := e.store.Save(sk)
		if err != nil {
			return xerr.Wrap(xerr.CodeKey, xerr.KindKey, err)
		}
		r.step("generated %s", path)
		ref := ""
		if p.SchemaSource != pair.SchemaFromDefault {
			ref = p.SchemaRef()
		}
		enc := pair.NewEncrypted(sk.Public(), ref)
		if err := pair.WriteEncrypted(p.EncryptedPath, enc); err != nil {
			return err
		}
		r.step("created   %s", p.EncryptedPath)
	}
	if !fileExists(p.UnencryptedPath) {
		if p.UnencryptedIsJSONC {
			r.step("missing   %s (jureuma never writes .jsonc files)", p.UnencryptedPath)
		} else {
			if err := pair.WritePlain(p.UnencryptedPath, doc.NewObject()); err != nil {
				return err
			}
			r.step("created   %s (empty, git-ignored)", p.UnencryptedPath)
		}
	} else {
		r.step("exists    %s", p.UnencryptedPath)
	}
	if !fileExists(p.SchemaPath) {
		if err := paths.AtomicWrite(p.SchemaPath, doc.Marshal(schema.Starter()), 0o644); err != nil {
			return xerr.Failure("%s: cannot write file", p.SchemaPath)
		}
		r.step("created   %s (permissive starter; run jureuma schema infer)", p.SchemaPath)
	} else {
		r.step("exists    %s", p.SchemaPath)
	}
	return nil
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
		rep := pair.Status(p, enc, plain, sk)
		outf(r.cmd, "  %s: sync state %s\n", p.Name, rep.Overall)
	} else {
		outf(r.cmd, "  %s: private key not available (expected on agent and CI machines)\n", p.Name)
	}
}
