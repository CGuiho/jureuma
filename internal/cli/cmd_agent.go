package cli

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/CGuiho/zeze/internal/agentres"
	"github.com/CGuiho/zeze/internal/paths"
	"github.com/CGuiho/zeze/internal/xerr"
	"github.com/spf13/cobra"
)

func (a *app) cmdAgent() *cobra.Command {
	c := &cobra.Command{
		Use:   "agent",
		Short: "manage the bundled agent skills, AGENTS.md instruction and prompts",
	}
	skill := &cobra.Command{Use: "skill", Short: "manage the bundled agent skills in the global agent-skill directories"}
	skill.AddCommand(a.cmdSkillInstall(), a.cmdSkillUninstall(), a.cmdSkillUpgrade(), a.cmdSkillList(), a.cmdSkillShow())
	instr := &cobra.Command{Use: "instruction", Short: "manage the zeze block in the project's AGENTS.md"}
	instr.AddCommand(a.cmdInstrApply(), a.cmdInstrRemove(), a.cmdInstrUpgrade(), a.cmdInstrShow())
	prompt := &cobra.Command{Use: "prompt", Short: "inspect the bundled one-time agent prompts"}
	prompt.AddCommand(a.cmdPromptList(), a.cmdPromptShow())
	c.AddCommand(skill, instr, prompt)
	return c
}

func selectSkills(args []string) ([]agentres.Skill, error) {
	if len(args) == 0 {
		return agentres.Skills(), nil
	}
	var out []agentres.Skill
	for _, id := range args {
		s, ok := agentres.SkillByID(id)
		if !ok {
			return nil, xerr.Usage("unknown skill %q (see zeze agent skill list)", id)
		}
		out = append(out, s)
	}
	return out, nil
}

func skillDests() ([]string, error) {
	uh, err := paths.UserHome()
	if err != nil {
		return nil, xerr.Failure("cannot resolve the user home directory")
	}
	return agentres.SkillDestinations(uh), nil
}

func installSkills(cmd *cobra.Command, skills []agentres.Skill, onlyExisting bool) error {
	dests, err := skillDests()
	if err != nil {
		return err
	}
	for _, s := range skills {
		for _, d := range dests {
			target := filepath.Join(d, s.ID)
			if onlyExisting && !fileExists(target) {
				continue
			}
			if agentres.SkillInstalledCurrent(s, d) {
				outf(cmd, "unchanged %s\n", target)
				continue
			}
			if err := agentres.InstallSkill(s, d); err != nil {
				return xerr.Failure("%s: %v", target, err)
			}
			outf(cmd, "installed %s\n", target)
		}
	}
	return nil
}

func (a *app) cmdSkillInstall() *cobra.Command {
	return &cobra.Command{
		Use:   "install [skill-id...]",
		Short: "install bundled skills (default: all) into every supported destination",
		RunE: func(cmd *cobra.Command, args []string) error {
			skills, err := selectSkills(args)
			if err != nil {
				return err
			}
			return installSkills(cmd, skills, false)
		},
	}
}

func (a *app) cmdSkillUpgrade() *cobra.Command {
	return &cobra.Command{
		Use:   "upgrade [skill-id...]",
		Short: "replace installed skills (default: all) with the bundled version",
		RunE: func(cmd *cobra.Command, args []string) error {
			skills, err := selectSkills(args)
			if err != nil {
				return err
			}
			return installSkills(cmd, skills, true)
		},
	}
}

// uninstallSkills removes dest/<id> for bundled skill IDs only.
func uninstallSkills(skills []agentres.Skill, report func(string)) error {
	dests, err := skillDests()
	if err != nil {
		return err
	}
	for _, s := range skills {
		for _, d := range dests {
			target := filepath.Join(d, s.ID)
			fi, err := os.Lstat(target)
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil || !fi.IsDir() {
				return xerr.Failure("%s is not a directory installed by zeze; remove it by hand", target)
			}
			if err := os.RemoveAll(target); err != nil {
				return xerr.Failure("cannot remove %s", target)
			}
			report(target)
		}
	}
	return nil
}

func (a *app) cmdSkillUninstall() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall [skill-id...]",
		Short: "remove installed copies of bundled skills (default: all)",
		RunE: func(cmd *cobra.Command, args []string) error {
			skills, err := selectSkills(args)
			if err != nil {
				return err
			}
			n := 0
			err = uninstallSkills(skills, func(p string) { n++; outf(cmd, "removed %s\n", p) })
			if err == nil && n == 0 {
				outf(cmd, "no installed copies found\n")
			}
			return err
		},
	}
}

func (a *app) cmdSkillList() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "list the bundled skill IDs and where they are installed",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dests, err := skillDests()
			if err != nil {
				return err
			}
			for _, s := range agentres.Skills() {
				outf(cmd, "%s\n", s.ID)
				for _, d := range dests {
					state := "not installed"
					if agentres.SkillInstalledCurrent(s, d) {
						state = "current"
					} else if fileExists(filepath.Join(d, s.ID)) {
						state = "outdated"
					}
					outf(cmd, "  %s: %s\n", filepath.Join(d, s.ID), state)
				}
			}
			return nil
		},
	}
}

func (a *app) cmdSkillShow() *cobra.Command {
	var file string
	c := &cobra.Command{
		Use:   "show [skill-id]",
		Short: "print a bundled skill (default: the main skill) without installing it",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := agentres.MainSkillID
			if len(args) == 1 {
				id = args[0]
			}
			s, ok := agentres.SkillByID(id)
			if !ok {
				return xerr.Usage("unknown skill %q", id)
			}
			data, ok := s.Files[file]
			if !ok {
				names := make([]string, 0, len(s.Files))
				for n := range s.Files {
					names = append(names, n)
				}
				sort.Strings(names)
				return xerr.Usage("skill %s has no file %q (files: %v)", id, file, names)
			}
			_, err := cmd.OutOrStdout().Write(data)
			return err
		},
	}
	c.Flags().StringVar(&file, "file", "SKILL.md", "file of the skill to print")
	return c
}

// ---- instruction ----

func agentsPath() (string, string, error) {
	e, err := loadEnv()
	if err != nil {
		return "", "", err
	}
	return e.root, filepath.Join(e.root, "AGENTS.md"), nil
}

// applyInstruction ensures the current block; returns "created", "updated" or "unchanged".
func applyInstruction(path string, onlyIfPresent bool) (string, error) {
	data, err := os.ReadFile(path)
	exists := err == nil
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", xerr.Failure("cannot read %s", path)
	}
	body := agentres.Instruction()
	if exists && agentres.HasCurrentBlock(data, body) {
		return "unchanged", nil
	}
	if onlyIfPresent {
		if _, had := agentres.RemoveBlock(data); !exists || !had {
			return "", xerr.Failure("%s has no zeze block; use zeze agent instruction apply", path)
		}
	}
	out := agentres.ApplyBlock(data, body)
	if err := paths.AtomicWrite(path, out, 0o644); err != nil {
		return "", xerr.Failure("cannot write %s", path)
	}
	if !exists {
		return "created", nil
	}
	return "updated", nil
}

func (a *app) cmdInstrApply() *cobra.Command {
	return &cobra.Command{
		Use:   "apply",
		Short: "add the zeze block to AGENTS.md (created if missing); other content is kept",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, path, err := agentsPath()
			if err != nil {
				return err
			}
			res, err := applyInstruction(path, false)
			if err != nil {
				return err
			}
			outf(cmd, "%s %s\n", res, path)
			return nil
		},
	}
}

func (a *app) cmdInstrUpgrade() *cobra.Command {
	return &cobra.Command{
		Use:   "upgrade",
		Short: "replace an existing zeze block in AGENTS.md with the bundled version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, path, err := agentsPath()
			if err != nil {
				return err
			}
			res, err := applyInstruction(path, true)
			if err != nil {
				return err
			}
			outf(cmd, "%s %s\n", res, path)
			return nil
		},
	}
}

func removeInstruction(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, xerr.Failure("cannot read %s", path)
	}
	out, had := agentres.RemoveBlock(data)
	if !had {
		return false, nil
	}
	if err := paths.AtomicWrite(path, out, 0o644); err != nil {
		return false, xerr.Failure("cannot write %s", path)
	}
	return true, nil
}

func (a *app) cmdInstrRemove() *cobra.Command {
	return &cobra.Command{
		Use:   "remove",
		Short: "remove only the zeze block from AGENTS.md (and zeze's agent-tool deny rules)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, path, err := agentsPath()
			if err != nil {
				return err
			}
			had, err := removeInstruction(path)
			if err != nil {
				return err
			}
			if had {
				outf(cmd, "removed the zeze block from %s\n", path)
			} else {
				outf(cmd, "%s has no zeze block\n", path)
			}
			if home, err := paths.Home(); err == nil {
				if n, err := removeDenyRules(home, root); err != nil {
					return err
				} else if n > 0 {
					outf(cmd, "removed %d agent-tool deny %s from %s\n", n, plural(n, "rule", "rules"), claudeSettingsPath(root))
				}
				if n, err := removeOpencodeDenyRules(home, root); err != nil {
					return err
				} else if n > 0 {
					outf(cmd, "removed %d agent-tool deny %s from %s\n", n, plural(n, "rule", "rules"), opencodeConfigPath(root))
				}
			}
			return nil
		},
	}
}

func (a *app) cmdInstrShow() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "print the bundled instruction without modifying AGENTS.md",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			outf(cmd, "%s", agentres.Instruction())
			return nil
		},
	}
}

// ---- prompt ----

func (a *app) cmdPromptList() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "list the bundled prompt IDs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, p := range agentres.Prompts() {
				mark := ""
				if p.ID == agentres.MainPromptID {
					mark = " (default)"
				}
				outf(cmd, "%s%s\n", p.ID, mark)
			}
			return nil
		},
	}
}

func (a *app) cmdPromptShow() *cobra.Command {
	return &cobra.Command{
		Use:   "show [prompt-id]",
		Short: "print a bundled prompt (default: " + agentres.MainPromptID + ")",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := agentres.MainPromptID
			if len(args) == 1 {
				id = args[0]
			}
			p, ok := agentres.PromptByID(id)
			if !ok {
				return xerr.Usage("unknown prompt %q (see zeze agent prompt list)", id)
			}
			outf(cmd, "%s", p.Body)
			return nil
		},
	}
}
