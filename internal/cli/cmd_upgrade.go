package cli

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"

	"github.com/CGuiho/jureuma/internal/paths"
	"github.com/CGuiho/jureuma/internal/runner"
	"github.com/CGuiho/jureuma/internal/update"
	"github.com/CGuiho/jureuma/internal/xerr"
	"github.com/spf13/cobra"
)

func printRecovery(cmd *cobra.Command, version, channel string) {
	outf(cmd, "If the upgrade fails, reinstall jureuma with this command:\n%s\n", update.ReinstallCommand(runtime.GOOS, version, channel))
}

func (a *app) cmdUpgrade() *cobra.Command {
	var version, channel string
	var force bool
	c := &cobra.Command{
		Use:   "upgrade",
		Short: "upgrade jureuma to the latest (or a selected) release",
		Long: `Resolves the target release from the GitHub release catalog and upgrades the installation.
The reinstall command is printed first and last; it is always a valid recovery. Refuses
while "jureuma run" instances are alive (they redact their child's output) unless --force,
which asks them to terminate first. After an upgrade, run jureuma init and verify
jureuma --version.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if version != "" && channel != "" {
				return xerr.Usage("--version and --channel are mutually exclusive")
			}
			if version != "" {
				if _, ok := update.Parse(version); !ok {
					return xerr.Usage("--version must be a SemVer version such as 1.2.3")
				}
			}
			printRecovery(cmd, trimV(version), channel)
			target := ""
			err := a.doUpgrade(cmd, version, channel, force, &target)
			if err != nil {
				errf(cmd, "jureuma: %s\n", err.Error())
			}
			printRecovery(cmd, target, channelIf(target, channel))
			if err != nil {
				return &silentError{code: xerr.CodeOf(err)}
			}
			return nil
		},
	}
	c.Flags().StringVar(&version, "version", "", "upgrade to this exact version")
	c.Flags().StringVar(&channel, "channel", "", "upgrade to the latest release of this channel (stable, beta, ...)")
	c.Flags().BoolVar(&force, "force", false, "ask running jureuma run instances to terminate first")
	c.AddCommand(a.cmdUpgradeCheck(), a.cmdUpgradeList())
	return c
}

func (a *app) doUpgrade(cmd *cobra.Command, version, channel string, force bool, targetOut *string) error {
	home, err := paths.Home()
	if err != nil {
		return xerr.Failure("cannot resolve the jureuma home")
	}
	// Refuse early (before any download) when run instances are alive and
	// --force was not given. With --force they are only asked to terminate
	// right before activation, never for a no-op or a failed download.
	if inst := runner.Instances(home); len(inst) > 0 && !force {
		for _, i := range inst {
			errf(cmd, "  jureuma run instance %s (pid %d)\n", i.ID, i.PID)
		}
		return xerr.Failure("%d jureuma run %s alive; stop them or pass --force", len(inst), plural(len(inst), "instance is", "instances are"))
	}
	rs, err := update.FetchReleases(context.Background())
	if err != nil {
		if errors.Is(err, update.ErrNoReleases) {
			return xerr.Failure("no published jureuma release was found in %s", update.RepoURL)
		}
		return xerr.Failure("cannot read the release catalog: %v", err)
	}
	rel, ok := update.Select(rs, trimV(version), channel)
	if !ok {
		return xerr.Failure("no compatible release (with every required asset for %s/%s) matches the requested version or channel", runtime.GOOS, runtime.GOARCH)
	}
	*targetOut = rel.Version
	cur, curOK := update.Parse(a.version)
	tv, _ := update.Parse(rel.Version)
	if curOK && update.Compare(tv, cur) == 0 {
		outf(cmd, "up-to-date: jureuma %s is installed\n", a.version)
		return nil
	}
	if version == "" && curOK && update.Compare(tv, cur) < 0 {
		outf(cmd, "up-to-date: jureuma %s is newer than the latest %s release %s\n", a.version, rel.Channel, rel.Version)
		return nil
	}
	outf(cmd, "upgrading jureuma %s -> %s\n", a.version, rel.Version)
	return a.runUpgrade(cmd, home, rel, force)
}

func trimV(v string) string {
	if len(v) > 0 && v[0] == 'v' {
		return v[1:]
	}
	return v
}

func channelIf(target, channel string) string {
	if target != "" {
		return ""
	}
	return channel
}

func (a *app) cmdUpgradeCheck() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "check whether a newer release is available (changes nothing)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := paths.Home()
			if err != nil {
				return xerr.Failure("cannot resolve the jureuma home")
			}
			c, err := update.Check(context.Background(), home, a.version)
			if err != nil {
				if errors.Is(err, update.ErrNoReleases) {
					outf(cmd, "current: %s\nlatest:  none published\nstatus:  up-to-date\n", a.version)
					return nil
				}
				return xerr.Failure("cannot check for upgrades: %v", err)
			}
			status := "up-to-date"
			if c.NewVersionAvailable {
				status = "upgrade available (run jureuma upgrade)"
			}
			outf(cmd, "current: %s\nlatest:  %s\nstatus:  %s\n", a.version, c.LatestVersion, status)
			return nil
		},
	}
}

func (a *app) cmdUpgradeList() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "list the releases available for installation",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rs, err := update.FetchReleases(context.Background())
			if err != nil {
				if errors.Is(err, update.ErrNoReleases) {
					outf(cmd, "no published releases\n")
					return nil
				}
				return xerr.Failure("cannot read the release catalog: %v", err)
			}
			for _, r := range update.Installable(rs) {
				mark := ""
				if r.Version == trimV(a.version) {
					mark = "  (installed)"
				}
				outf(cmd, "%s  %s%s\n", r.Version, r.Channel, mark)
			}
			return nil
		},
	}
}

// ---- bare invocation and background worker ----

func (a *app) runBare(cmd *cobra.Command, args []string) error {
	if err := cmd.Help(); err != nil {
		return err
	}
	// The startup notice and the background check belong to the bare name
	// only: no arguments and no flags at all.
	if len(args) > 0 || cmd.Flags().NFlag() > 0 {
		return nil
	}
	if os.Getenv(update.WorkerEnv) != "" || os.Getenv(update.DisableEnv) != "" {
		return nil
	}
	home, err := paths.Home()
	if err != nil {
		return nil
	}
	if n := update.Notice(home, a.version); n != "" {
		errf(cmd, "%s", n)
	}
	if !update.NeedsCheck(home) {
		return nil
	}
	token, ok := update.AcquireLease(home)
	if !ok {
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		update.ReleaseLease(home, token)
		return nil
	}
	w := exec.Command(exe, workerCommand)
	w.Env = append(os.Environ(), update.WorkerEnv+"=1", leaseTokenEnv+"="+token)
	w.Stdin, w.Stdout, w.Stderr = nil, nil, nil
	detach(w)
	if err := w.Start(); err != nil {
		update.ReleaseLease(home, token)
		return nil
	}
	_ = w.Process.Release()
	return nil
}

const (
	workerCommand = "__update-worker"
	leaseTokenEnv = "JUREUMA_UPDATE_LEASE"
)

func (a *app) cmdUpdateWorker() *cobra.Command {
	return &cobra.Command{
		Use:    workerCommand,
		Hidden: true,
		Args:   cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if os.Getenv(update.WorkerEnv) != "1" {
				return nil
			}
			home, err := paths.Home()
			if err != nil {
				return nil
			}
			defer update.ReleaseLease(home, os.Getenv(leaseTokenEnv))
			ctx, cancel := context.WithTimeout(context.Background(), update.FetchTimeout)
			defer cancel()
			_, _ = update.Check(ctx, home, a.version)
			return nil
		},
	}
}
