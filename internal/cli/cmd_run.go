package cli

import (
	"context"
	"strings"

	"github.com/CGuiho/jureuma/internal/config"
	"github.com/CGuiho/jureuma/internal/crypt"
	"github.com/CGuiho/jureuma/internal/pair"
	"github.com/CGuiho/jureuma/internal/runner"
	"github.com/CGuiho/jureuma/internal/xerr"
	"github.com/spf13/cobra"
)

func (a *app) cmdRun() *cobra.Command {
	var pf pairFlags
	var format, envName, prefix string
	var envs []string
	var override, bypass bool
	c := &cobra.Command{
		Use:     "run [flags] -- <command> [args...]",
		Aliases: []string{"inject"},
		Short:   "run a command with the decrypted configuration injected, output redacted",
		Long: `Decrypts the configuration in memory, validates it, and starts the command with it injected.
JUREUMA_PRIVATE_KEY and every JUREUMA_PRIVATE_KEY_* variable are always removed from the
child's environment.

--format json (default): the whole document, compact JSON, in --env-name (default
  JUREUMA_CONFIG).
--format flatten: one variable per leaf (/database/url -> DATABASE_URL), with --prefix.
  Code-loading names (LD_*, DYLD_*, NODE_OPTIONS, PYTHONPATH, ...) and JUREUMA_* are refused.
--format file: --env-name holds a path to the document (an in-memory /dev/fd file on Linux,
  otherwise a 0600 file removed when the child exits).
--env NAME=/pointer injects selected values in addition (repeatable).
Existing environment variables win unless --override is given.

The child's stdout and stderr are redacted: every string value of 4+ characters, every
value marked x-jureuma-secret, and every private key (always) become [REDACTED].
run returns the child's exit code; a jureuma-side failure exits 125 with a
[JUREUMA:<KIND>] message on stderr.`,
		Example: `  jureuma run -- node server.js
  jureuma run --format flatten --prefix APP_ -- ./start.sh
  jureuma run --env DATABASE_URL=/database/url -- psql "$DATABASE_URL"`,
		Args: cobra.MinimumNArgs(1),
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
			sk, err := e.requireKey(enc)
			if err != nil {
				return err
			}
			keys := pair.StoreKeys(&sk, e.store)
			plain, err := pair.DecryptDocKeys(keys, enc.Body)
			if err != nil {
				return err
			}
			used := []crypt.PrivateKey{sk}
			for id := range pair.CiphertextKeyIDs(enc.Body) {
				if id == enc.Meta.KeyID {
					continue
				}
				if k, err := keys(id); err == nil {
					used = append(used, k)
				}
			}
			sch, err := loadSchema(p)
			if err != nil {
				return err
			}
			if sch != nil {
				if issues := sch.Validate(plain); len(issues) > 0 {
					// The issues follow the [JUREUMA:VALIDATION] line so that
					// every jureuma-side failure starts with the fixed prefix.
					lines := make([]string, len(issues))
					for i, is := range issues {
						lines[i] = "\n  " + is.String()
					}
					return xerr.Validation("%d validation issue(s) in %s:%s", len(issues), p.Rel(p.EncryptedPath), strings.Join(lines, ""))
				}
			}
			eff := e.effective()
			fl := cmd.Flags()
			if !fl.Changed("format") {
				format = eff.Run.Format
			}
			if !fl.Changed("env-name") {
				envName = eff.Run.EnvName
			}
			if !fl.Changed("prefix") {
				prefix = eff.Run.Prefix
			}
			if !fl.Changed("override") {
				override = eff.Run.Override
			}
			if !config.ValidFormat(format) {
				return xerr.Usage("--format must be json, flatten or file")
			}
			redactValues := true
			if bypass {
				if err := e.checkBypass(cmd, []string{p.Rel(p.EncryptedPath)}); err != nil {
					return err
				}
				redactValues = false
			}
			code, err := runner.Run(context.Background(), runner.Options{
				Doc: plain, Schema: sch, Format: format, EnvName: envName, Prefix: prefix,
				Env: envs, Override: override, Redact: redactValues, PrivateKeys: used,
				Args: args, Stdin: cmd.InOrStdin(), Stdout: cmd.OutOrStdout(), Stderr: cmd.ErrOrStderr(), Home: e.home,
			})
			if err != nil {
				return err
			}
			if code != 0 {
				return &exitError{code: code}
			}
			return nil
		},
	}
	addPairFlags(c, &pf)
	f := c.Flags()
	f.SetInterspersed(false)
	f.StringVar(&format, "format", config.DefaultFormat, "injection format: json, flatten or file (default from jureuma.yaml run.format)")
	f.StringVar(&envName, "env-name", config.DefaultEnvName, "variable for --format json/file (default from run.envName)")
	f.StringVar(&prefix, "prefix", "", "prefix for --format flatten variable names")
	f.StringArrayVar(&envs, "env", nil, "inject one value as NAME=/json/pointer (repeatable)")
	f.BoolVar(&override, "override", false, "let injected values replace existing environment variables")
	addBypassFlag(c, &bypass, "disable redaction of configuration values (private keys stay redacted; audited)")
	return c
}
