package cli

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/CGuiho/jureuma/internal/agentres"
	"github.com/CGuiho/jureuma/internal/config"
	"github.com/CGuiho/jureuma/internal/gate"
	"github.com/CGuiho/jureuma/internal/pair"
	"github.com/CGuiho/jureuma/internal/paths"
	"github.com/CGuiho/jureuma/internal/xerr"
	"github.com/spf13/cobra"
)

type planItem struct {
	remove bool
	path   string
	reason string
	do     func() error
}

func (a *app) cmdUninstall() *cobra.Command {
	var preserveConfig, preserveData, dryRun, yes bool
	c := &cobra.Command{
		Use:   "uninstall",
		Short: "remove jureuma: launcher, home directory, skills, AGENTS.md block and jureuma.yaml",
		Long: `Removes every jureuma-owned artifact by default: the launcher in ~/.guiho/bin,
~/.guiho/jureuma (configuration, state, audit log, caches), the installed skill copies, the
jureuma block in the project's AGENTS.md, the agent-tool deny rules jureuma added, and the
project's jureuma.yaml. The project .gitignore block is always kept (plaintext files may
outlive jureuma). Private keys (~/.guiho/jureuma/keys) are removed only after the human-only
confirmation, even with --yes; a non-interactive uninstall preserves them. Back keys up
first with jureuma key export. Shared ~/.guiho directories and PATH are never touched.`,
		Example: `  jureuma uninstall --dry-run
  jureuma uninstall --preserve-config --preserve-data --yes
  jureuma uninstall`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := paths.Home()
			if err != nil {
				return xerr.Failure("cannot resolve the jureuma home")
			}
			bin, err := paths.BinDir()
			if err != nil {
				return xerr.Failure("cannot resolve ~/.guiho/bin")
			}
			wd, _ := os.Getwd()
			root, err := paths.ProjectRoot(wd)
			if err != nil {
				return xerr.Failure("cannot resolve the project root")
			}
			var plan []planItem
			add := func(remove bool, path, reason string, do func() error) {
				plan = append(plan, planItem{remove, path, reason, do})
			}
			// Projections first (they need state that is removed later).
			if st := readDenyState(home); len(st.Projects[root]) > 0 || len(st.OpenCode[root]) > 0 {
				if len(st.Projects[root]) > 0 {
					add(true, claudeSettingsPath(root), "agent-tool deny rules added by jureuma (only those entries)", func() error {
						_, err := removeDenyRules(home, root)
						return err
					})
				}
				if len(st.OpenCode[root]) > 0 {
					add(true, opencodeConfigPath(root), "agent-tool deny rules added by jureuma (only those entries)", func() error {
						_, err := removeOpencodeDenyRules(home, root)
						return err
					})
				}
			}
			if dests, err := skillDests(); err == nil {
				for _, s := range agentres.Skills() {
					for _, d := range dests {
						t := filepath.Join(d, s.ID)
						if fi, err := os.Lstat(t); err == nil && fi.IsDir() {
							add(true, t, "managed skill copy", func() error { return os.RemoveAll(t) })
						}
					}
				}
			}
			agents := filepath.Join(root, "AGENTS.md")
			if data, err := os.ReadFile(agents); err == nil {
				if _, had := agentres.RemoveBlock(data); had {
					add(true, agents, "jureuma managed block only; the rest of AGENTS.md is kept", func() error {
						_, err := removeInstruction(agents)
						return err
					})
				}
			}
			projCfg := filepath.Join(root, config.ProjectFile)
			if fileExists(projCfg) {
				if preserveConfig {
					add(false, projCfg, "--preserve-config", nil)
				} else {
					add(true, projCfg, "project configuration", func() error { return os.Remove(projCfg) })
				}
			}
			if pair.HasGitignoreBlock(root) {
				add(false, filepath.Join(root, ".gitignore"), "jureuma block is persistent project data: plaintext files may outlive jureuma", nil)
			}
			launcher := filepath.Join(bin, exeName("jureuma"))
			if fileExists(launcher) {
				add(true, launcher, "stable launcher", func() error { return os.Remove(launcher) })
			}
			// Home children.
			keysDir := filepath.Join(home, "keys")
			removeKeys := false
			ents, _ := os.ReadDir(home)
			preservedInHome := false
			for _, en := range ents {
				name := en.Name()
				p := filepath.Join(home, name)
				switch {
				case name == "keys":
					continue
				case name == config.GlobalFile:
					if preserveConfig {
						add(false, p, "--preserve-config", nil)
						preservedInHome = true
						continue
					}
					add(true, p, "global configuration", func() error { return os.RemoveAll(p) })
				case name == "state" || name == gate.AuditFileName:
					if preserveData {
						add(false, p, "--preserve-data", nil)
						preservedInHome = true
						continue
					}
					add(true, p, "persistent data", func() error { return os.RemoveAll(p) })
				default:
					add(true, p, "installation, cache or disposable state", func() error { return os.RemoveAll(p) })
				}
			}
			glob, _, globErr := config.LoadGlobal(home)
			var markers []string
			if globErr == nil && glob != nil {
				markers = glob.Protections.AgentEnvMarkers
			}
			keyIDs := listKeyIDs(keysDir)
			if len(keyIDs) > 0 || fileExists(keysDir) {
				switch {
				case preserveData:
					add(false, keysDir, "--preserve-data", nil)
					preservedInHome = true
				case !interactive() || isAgent(markers):
					add(false, keysDir, "private keys are removed only through the human-only confirmation (non-interactive run)", nil)
					preservedInHome = true
				default:
					removeKeys = true
					add(true, keysDir, "private keys; requires the human-only confirmation", nil)
				}
			}

			outf(cmd, "jureuma uninstall plan\n")
			for _, label := range []string{"REMOVE", "PRESERVE"} {
				outf(cmd, "%s\n", label)
				n := 0
				for _, it := range plan {
					if it.remove == (label == "REMOVE") {
						outf(cmd, "  %s  (%s)\n", it.path, it.reason)
						n++
					}
				}
				if n == 0 {
					outf(cmd, "  (nothing)\n")
				}
			}
			if len(keyIDs) > 0 {
				outf(cmd, "private keys in %s:\n", keysDir)
				for _, id := range keyIDs {
					files := projectKeyUsage(root, strings.TrimSuffix(id, " (retired)"))
					outf(cmd, "  %s  referenced by %d encrypted %s in this project\n", id, len(files), plural(len(files), "file", "files"))
				}
				outf(cmd, "back them up first with: jureuma key export <keyId>\n")
			}
			if dryRun {
				outf(cmd, "dry run: nothing was changed\n")
				return nil
			}
			if !yes {
				if !interactive() {
					return xerr.Failure("uninstall needs confirmation: run it in an interactive terminal or pass --yes (nothing was changed)")
				}
				ans, _ := ask(cmd, "Type \"uninstall\" to remove the items listed under REMOVE: ")
				if ans != "uninstall" {
					return xerr.Failure("uninstall cancelled; nothing was changed")
				}
			}
			// The human-only confirmation for the keys runs before anything is
			// removed, so its outcome is audited while the audit log exists.
			keysConfirmed := false
			if removeKeys {
				g := gate.New(markers)
				ev := gate.AuditEvent{Time: time.Now().UTC(), Command: cmd.CommandPath(), Paths: []string{keysDir}, Markers: gate.DetectAgents(os.Getenv, markers), Outcome: "allowed"}
				if err := g.Check("permanently delete " + itoa(len(keyIDs)) + " private " + plural(len(keyIDs), "key", "keys")); err != nil {
					ev.Outcome = "refused"
				} else {
					keysConfirmed = true
				}
				_ = gate.Audit(home, ev)
			}
			cleanupImage := releaseRunningImage(home, launcher)
			defer cleanupImage()
			var failed []string
			for _, it := range plan {
				if !it.remove || it.do == nil {
					continue
				}
				if err := it.do(); err != nil && !errors.Is(err, fs.ErrNotExist) {
					failed = append(failed, it.path)
					errf(cmd, "error: could not remove %s\n", it.path)
					continue
				}
				outf(cmd, "removed %s\n", it.path)
			}
			if removeKeys {
				if !keysConfirmed {
					outf(cmd, "preserved %s (the human-only confirmation was not given)\n", keysDir)
					preservedInHome = true
				} else if err := os.RemoveAll(keysDir); err != nil {
					failed = append(failed, keysDir)
				} else {
					outf(cmd, "removed %s\n", keysDir)
				}
			}
			if !preservedInHome {
				// The key confirmation may have created the audit log after
				// the plan was built.
				_ = os.Remove(filepath.Join(home, gate.AuditFileName))
				if err := os.Remove(home); err == nil {
					outf(cmd, "removed %s\n", home)
				}
			}
			if len(failed) > 0 {
				return xerr.Failure("%d %s could not be removed", len(failed), plural(len(failed), "item", "items"))
			}
			outf(cmd, "jureuma was uninstalled; ~/.guiho, ~/.guiho/bin, ~/.guiho/.temp and PATH were left untouched\n")
			return nil
		},
	}
	f := c.Flags()
	f.BoolVar(&preserveConfig, "preserve-config", false, "keep jureuma.global.yaml and the project's jureuma.yaml")
	f.BoolVar(&preserveData, "preserve-data", false, "keep keys/, state/ and audit.log")
	f.BoolVar(&dryRun, "dry-run", false, "print the plan without changing anything")
	f.BoolVar(&yes, "yes", false, "confirm the plan without a prompt (keys still need the human-only confirmation)")
	return c
}

func isAgent(extra []string) bool {
	_, found := gate.DetectAgent(os.Getenv, extra)
	return found
}

// listKeyIDs lists key IDs stored under keysDir (retired ones marked).
func listKeyIDs(keysDir string) []string {
	var out []string
	add := func(dir, suffix string) {
		ents, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range ents {
			n := e.Name()
			if strings.HasSuffix(n, ".json") && strings.HasPrefix(n, "k_") {
				out = append(out, strings.TrimSuffix(n, ".json")+suffix)
			}
		}
	}
	add(keysDir, "")
	add(filepath.Join(keysDir, "retired"), " (retired)")
	sort.Strings(out)
	return out
}
