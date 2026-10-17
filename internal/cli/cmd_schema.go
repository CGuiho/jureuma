package cli

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/CGuiho/jureuma/internal/doc"
	"github.com/CGuiho/jureuma/internal/jsonc"
	"github.com/CGuiho/jureuma/internal/pair"
	"github.com/CGuiho/jureuma/internal/paths"
	"github.com/CGuiho/jureuma/internal/schema"
	"github.com/CGuiho/jureuma/internal/xerr"
	"github.com/spf13/cobra"
)

func (a *app) cmdSchema() *cobra.Command {
	c := &cobra.Command{
		Use:   "schema",
		Short: "work with the pair's JSON Schema",
	}
	c.AddCommand(a.cmdSchemaInfer())
	return c
}

func (a *app) cmdSchemaInfer() *cobra.Command {
	var pf pairFlags
	var force bool
	c := &cobra.Command{
		Use:   "infer",
		Short: "generate a starter schema from the plaintext structure and JSON types (never values)",
		Long: `Writes a JSON Schema (draft 2020-12) at the pair's resolved schema path, derived from the
unencrypted file's structure and JSON types: objects get required for every key and
additionalProperties false, homogeneous arrays get items, mixed arrays prefixItems. Without
an unencrypted file the structure comes from the encrypted file and leaf types stay
unconstrained. Values are never read into the schema. Refuses to overwrite an existing
schema (other than the init starter) without --force. A .jsonc schema is merged in place,
keeping the comments and formatting of surviving keys. Refine the result by hand: formats,
ranges, descriptions and "x-jureuma-secret": true.`,
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
			if err := p.CheckSchemaFlag(); err != nil {
				return err
			}
			if err := pair.CheckSchemaName(p.SchemaPath); err != nil {
				return err
			}
			if fileExists(p.SchemaPath) && !force {
				existing, err := schema.Load(p.SchemaPath)
				if err != nil || !schema.IsStarter(existing.Raw()) {
					return xerr.Failure("%s already exists; pass --force to overwrite it", p.Rel(p.SchemaPath))
				}
			}
			var inferred *doc.Value
			source := ""
			if fileExists(p.UnencryptedPath) {
				plain, err := pair.ReadPlain(p.UnencryptedPath, p.UnencryptedIsJSONC)
				if err != nil {
					return err
				}
				inferred = schema.Infer(plain, true)
				source = p.Rel(p.UnencryptedPath)
			} else {
				enc, err := pair.ReadEncrypted(p.EncryptedPath)
				if err != nil {
					return err
				}
				inferred = schema.Infer(enc.Body, false)
				source = p.Rel(p.EncryptedPath) + " (structure only; leaf types unconstrained)"
			}
			if _, err := schema.Compile(inferred, p.SchemaPath); err != nil {
				return xerr.Wrap(xerr.CodeValidation, xerr.KindValidation, err)
			}
			data, err := schemaBytes(p.SchemaPath, inferred)
			if err != nil {
				return err
			}
			if err := paths.AtomicWrite(p.SchemaPath, data, 0o644); err != nil {
				return xerr.Failure("%s: cannot write file", p.Rel(p.SchemaPath))
			}
			outf(cmd, "wrote %s from %s\n", p.Rel(p.SchemaPath), source)
			outf(cmd, "next: refine it (formats, ranges, descriptions, \"x-jureuma-secret\": true), then run jureuma check\n")
			return nil
		},
	}
	addPairFlags(c, &pf)
	c.Flags().BoolVar(&force, "force", false, "overwrite an existing schema")
	return c
}

// schemaBytes renders the inferred schema for the resolved path: canonical
// JSON for a .json target, and for a .jsonc target with existing content a
// comment-preserving merge so comments on surviving keys survive. An
// unparseable or unrepresentable .jsonc file is refused, never rewritten.
func schemaBytes(path string, inferred *doc.Value) ([]byte, error) {
	name := filepath.Base(path)
	if !strings.EqualFold(filepath.Ext(path), ".jsonc") {
		return doc.Marshal(inferred), nil
	}
	orig, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return doc.Marshal(inferred), nil
		}
		return nil, xerr.Failure("%s: cannot read file", name)
	}
	current, err := jsonc.Parse(orig, false)
	if err != nil {
		return nil, xerr.Validation("%s: %v (fix it by hand; jureuma will not rewrite it)", name, err)
	}
	data, err := jsonc.Edit(orig, current, inferred)
	if err != nil {
		return nil, xerr.Failure("%s: %v; nothing was written", name, err)
	}
	return data, nil
}
