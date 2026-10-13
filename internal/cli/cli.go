// Package cli is jureuma's Cobra command tree.
//
// Every message printed here is built from file names, JSON Pointers, key IDs
// and fixed phrases. Configuration values, plaintext and key material are only
// ever written to stdout by the human-only commands (decrypt writes a file;
// get, key show --private and key export print), after the gate passed or the
// caller explicitly bypassed it.
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/CGuiho/jureuma/internal/paths"
	"github.com/CGuiho/jureuma/internal/runner"
	"github.com/CGuiho/jureuma/internal/xerr"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// exitError carries an exit code without a message (e.g. a child's code).
type exitError struct{ code int }

func (e *exitError) Error() string { return fmt.Sprintf("exit %d", e.code) }

// silentError has already been reported; it only carries the exit code.
type silentError struct{ code int }

func (e *silentError) Error() string { return fmt.Sprintf("exit %d", e.code) }

// app holds process-wide settings.
type app struct {
	version string
	root    *cobra.Command
}

// Execute runs the CLI with os.Args and returns the process exit code.
func Execute(version string) int {
	return ExecuteArgs(version, os.Args[1:], os.Stdout, os.Stderr)
}

// ExecuteArgs runs the CLI with explicit arguments and output streams.
func ExecuteArgs(version string, args []string, stdout, stderr io.Writer) int {
	return ExecuteIO(version, args, os.Stdin, stdout, stderr)
}

// ExecuteIO runs the CLI with explicit arguments and streams. The human-only
// gate always reads the process's real terminal, never stdin given here.
func ExecuteIO(version string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	a := &app{version: version}
	root := a.newRoot()
	root.SetArgs(args)
	root.SetIn(stdin)
	root.SetOut(stdout)
	root.SetErr(stderr)
	cmd, err := root.ExecuteC()
	if err == nil {
		return 0
	}
	isRun := cmd != nil && cmd.Name() == "run" && cmd.Parent() == root
	var ee *exitError
	if errors.As(err, &ee) {
		return ee.code
	}
	var se *silentError
	if errors.As(err, &se) {
		if isRun {
			return xerr.CodeRun
		}
		return se.code
	}
	var xe *xerr.Error
	isX := errors.As(err, &xe)
	if isRun {
		kind := xerr.KindOf(err)
		msg := err.Error()
		if !isX {
			kind = xerr.KindUsage
			msg = parseErrorMessage(err, args)
		}
		fmt.Fprintf(stderr, "[JUREUMA:%s] %s\n", kind, msg)
		return xerr.CodeRun
	}
	if !isX {
		// Errors produced by Cobra itself (unknown command or flag, bad
		// argument count) are usage errors.
		fmt.Fprintf(stderr, "jureuma: %s\n", parseErrorMessage(err, args))
		fmt.Fprintf(stderr, "Run '%s --help' for usage.\n", commandPath(cmd))
		return xerr.CodeUsage
	}
	if xe.Code == xerr.CodeProtected && strings.HasPrefix(err.Error(), "[PROTECTED]") {
		fmt.Fprintln(stderr, err.Error())
	} else {
		fmt.Fprintf(stderr, "jureuma: %s\n", err.Error())
	}
	return xe.Code
}

// dashHint tells the user how to pass a value that begins with '-'.
const dashHint = "; a value that begins with '-' is read as a flag: put -- before it, or use --value-stdin or --value-file"

// parseErrorMessage turns a Cobra or pflag parse error into a message that
// never echoes a command-line token: a mistyped flag or argument may well be
// a secret (random passwords and base64 tokens often begin with '-'). Only
// names of flags that exist are kept.
func parseErrorMessage(err error, args []string) string {
	var notExist *pflag.NotExistError
	var valueReq *pflag.ValueRequiredError
	var invalid *pflag.InvalidValueError
	var syntax *pflag.InvalidSyntaxError
	switch {
	case errors.As(err, &notExist):
		return "unknown flag" + dashHint
	case errors.As(err, &valueReq):
		if f := valueReq.GetFlag(); f != nil {
			return "flag needs an argument: --" + f.Name
		}
		return "a flag needs an argument"
	case errors.As(err, &invalid):
		if f := invalid.GetFlag(); f != nil {
			return "invalid value for --" + f.Name
		}
		return "invalid flag value"
	case errors.As(err, &syntax):
		return "bad flag syntax" + dashHint
	}
	msg := err.Error()
	switch {
	case strings.HasPrefix(msg, "unknown command "):
		return "unknown command (see --help for the available commands)"
	case strings.HasPrefix(msg, "invalid argument "):
		return "invalid argument"
	case strings.HasPrefix(msg, "unknown flag") || strings.HasPrefix(msg, "unknown shorthand flag"):
		return "unknown flag" + dashHint
	case strings.HasPrefix(msg, "flag needs an argument"):
		return "a flag needs an argument"
	case strings.HasPrefix(msg, "bad flag syntax"):
		return "bad flag syntax" + dashHint
	case strings.Contains(msg, " flag: "):
		return "invalid flag value"
	}
	// Anything else from the parser: strip every argument it might quote.
	for _, a := range args {
		if len(a) >= 3 && strings.Contains(msg, a) {
			return "invalid command line"
		}
	}
	return msg
}

func commandPath(cmd *cobra.Command) string {
	if cmd == nil {
		return "jureuma"
	}
	return cmd.CommandPath()
}

func (a *app) newRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "jureuma",
		Short: "Agent-safe configuration CLI: encrypt, set and inject configuration without revealing it",
		Long: `jureuma keeps a gitignored plaintext JSON file, a committed encrypted JSON file and a
committed JSON Schema per configuration. AI agents can set values and inject them into
processes but cannot read them: decrypt and get are human-only.

Files of the default pair:
  config.unencrypted.json   plaintext, human-owned, always gitignored
  config.encrypted.json     ciphertext + public key, committed
  config.schema.json        JSON Schema, committed

Exit codes: 0 ok, 1 failure, 2 usage, 3 protected operation refused, 4 validation failed,
5 key not found / key mismatch / decryption failed. run returns the child's code, or 125.`,
		Version:       a.version,
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.NoArgs,
		RunE:          a.runBare,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if cmd.Name() == selfTestCmd {
				return // the self-test never touches the filesystem
			}
			if home, err := paths.Home(); err == nil {
				runner.SweepStale(home)
			}
		},
	}
	a.root = root
	root.SetVersionTemplate("{{.Version}}\n")
	root.InitDefaultVersionFlag()
	if f := root.Flags().Lookup("version"); f != nil {
		f.Usage = "print the raw version and nothing else"
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.SetHelpCommand(&cobra.Command{
		Use:    "help [command]",
		Short:  "show help for a command",
		Hidden: true,
		RunE: func(c *cobra.Command, args []string) error {
			target, _, err := root.Find(args)
			if err != nil || target == nil {
				return xerr.Usage("unknown command (run 'jureuma --help' for the command list)")
			}
			return target.Help()
		},
	})
	pf := root.PersistentFlags()
	pf.Bool("help-docs", false, "print Markdown documentation for the command")
	pf.Bool("help-tree", false, "print the command tree from this command, with every flag")
	pf.String("help-tree-depth", "max", "depth of --help-tree: max or an integer greater than 1")
	pf.Bool("help-tree-global-flags", false, "repeat global flags under every command in --help-tree")
	root.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		return xerr.Usage("%s", parseErrorMessage(err, nil))
	})

	root.AddCommand(
		a.cmdInit(),
		a.cmdEncrypt(),
		a.cmdDecrypt(),
		a.cmdSet(),
		a.cmdUnset(),
		a.cmdGet(),
		a.cmdRun(),
		a.cmdList(),
		a.cmdStatus(),
		a.cmdCheck(),
		a.cmdKey(),
		a.cmdSchema(),
		a.cmdEnableJuju(),
		a.cmdDisableJuju(),
		a.cmdAgent(),
		a.cmdUpgrade(),
		a.cmdUninstall(),
		a.cmdUpdateWorker(),
		a.cmdSelfTest(),
	)
	instrument(root)
	return root
}

// helpRequested reports whether a help-output flag was given.
func helpRequested(cmd *cobra.Command) bool {
	for _, n := range []string{"help-docs", "help-tree"} {
		if f := cmd.Flags().Lookup(n); f != nil && f.Changed {
			return true
		}
	}
	return false
}

// instrument wraps every command so the help flags work everywhere (even
// when positional arguments are missing), argument errors are usage errors,
// group commands print help, and unclassified errors become exit 1.
func instrument(c *cobra.Command) {
	for _, sub := range c.Commands() {
		instrument(sub)
	}
	origArgs := c.Args
	if origArgs == nil && c.HasSubCommands() {
		origArgs = cobra.NoArgs
	}
	c.Args = func(cmd *cobra.Command, args []string) error {
		if helpRequested(cmd) {
			return nil
		}
		if origArgs != nil {
			if err := origArgs(cmd, args); err != nil {
				return xerr.Usage("%s", parseErrorMessage(err, args))
			}
		}
		return nil
	}
	origRun := c.RunE
	c.RunE = func(cmd *cobra.Command, args []string) error {
		if handled, err := handleHelpFlags(cmd); handled {
			return err
		}
		if origRun == nil {
			return cmd.Help()
		}
		err := origRun(cmd, args)
		if err == nil {
			return nil
		}
		var xe *xerr.Error
		var ee *exitError
		var se *silentError
		if errors.As(err, &xe) || errors.As(err, &ee) || errors.As(err, &se) {
			return err
		}
		return xerr.Wrap(xerr.CodeFailure, xerr.KindFailure, err)
	}
	c.Run = nil
}

func handleHelpFlags(cmd *cobra.Command) (bool, error) {
	f := cmd.Flags()
	if hd, _ := f.GetBool("help-docs"); hd {
		return true, writeHelpDocs(cmd, cmd.OutOrStdout())
	}
	if ht, _ := f.GetBool("help-tree"); ht {
		depthStr, _ := f.GetString("help-tree-depth")
		depth, err := parseDepth(depthStr)
		if err != nil {
			return true, err
		}
		global, _ := f.GetBool("help-tree-global-flags")
		writeHelpTree(cmd, cmd.OutOrStdout(), depth, global)
		return true, nil
	}
	return false, nil
}

// outf and errf write to the command's streams.
func outf(cmd *cobra.Command, format string, a ...any) {
	fmt.Fprintf(cmd.OutOrStdout(), format, a...)
}

func errf(cmd *cobra.Command, format string, a ...any) {
	fmt.Fprintf(cmd.ErrOrStderr(), format, a...)
}
