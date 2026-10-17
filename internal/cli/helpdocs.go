package cli

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/CGuiho/jureuma/internal/xerr"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// globalFlagNames are shown once at the top of --help-tree.
var globalFlagNames = []string{"help", "help-docs", "help-tree", "help-tree-depth", "help-tree-global-flags"}

func isGlobalFlag(name string) bool {
	for _, n := range globalFlagNames {
		if n == name {
			return true
		}
	}
	return false
}

func parseDepth(s string) (int, error) {
	if s == "" || s == "max" {
		return -1, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 2 {
		return 0, xerr.Usage("--help-tree-depth must be max or an integer greater than 1")
	}
	return n, nil
}

func flagSig(f *pflag.Flag) string {
	s := ""
	if f.Shorthand != "" {
		s = "-" + f.Shorthand + ", "
	}
	s += "--" + f.Name
	t := f.Value.Type()
	switch t {
	case "bool":
	case "stringArray":
		s += " <value> (repeatable)"
	default:
		s += " <" + t + ">"
	}
	return s
}

func flagDefault(f *pflag.Flag) string {
	switch f.Value.Type() {
	case "bool":
		if f.DefValue == "false" {
			return ""
		}
	case "stringArray":
		if f.DefValue == "[]" {
			return ""
		}
	}
	if f.DefValue == "" {
		return ""
	}
	return f.DefValue
}

// ownFlags returns the flags specific to cmd (local and its own persistent
// flags), excluding global ones, sorted by name.
func ownFlags(cmd *cobra.Command) []*pflag.Flag {
	var out []*pflag.Flag
	seen := map[string]bool{}
	add := func(f *pflag.Flag) {
		if f.Hidden || isGlobalFlag(f.Name) || seen[f.Name] {
			return
		}
		seen[f.Name] = true
		out = append(out, f)
	}
	cmd.LocalNonPersistentFlags().VisitAll(add)
	cmd.PersistentFlags().VisitAll(add)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func globalFlags(cmd *cobra.Command) []*pflag.Flag {
	root := cmd.Root()
	var out []*pflag.Flag
	help := &pflag.Flag{Name: "help", Shorthand: "h", Usage: "show help for the command", Value: boolValue{}, DefValue: "false"}
	out = append(out, help)
	for _, n := range globalFlagNames[1:] {
		if f := root.PersistentFlags().Lookup(n); f != nil {
			out = append(out, f)
		}
	}
	return out
}

type boolValue struct{}

func (boolValue) String() string   { return "false" }
func (boolValue) Set(string) error { return nil }
func (boolValue) Type() string     { return "bool" }

func visibleChildren(cmd *cobra.Command) []*cobra.Command {
	var out []*cobra.Command
	for _, c := range cmd.Commands() {
		if !c.Hidden {
			out = append(out, c)
		}
	}
	return out
}

func writeHelpTree(cmd *cobra.Command, w io.Writer, depth int, repeatGlobal bool) {
	fmt.Fprintf(w, "%s  %s\n", cmd.CommandPath(), cmd.Short)
	if !repeatGlobal {
		fmt.Fprintln(w, "  global flags (apply to every command):")
		for _, f := range globalFlags(cmd) {
			fmt.Fprintf(w, "    %s  %s\n", flagSig(f), f.Usage)
		}
	}
	printFlags(w, cmd, "  ", repeatGlobal)
	var walk func(c *cobra.Command, prefix string, level int)
	walk = func(c *cobra.Command, prefix string, level int) {
		if depth > 0 && level >= depth {
			return
		}
		kids := visibleChildren(c)
		for i, k := range kids {
			last := i == len(kids)-1
			branch, cont := "├── ", "│   "
			if last {
				branch, cont = "└── ", "    "
			}
			fmt.Fprintf(w, "%s%s%s  %s\n", prefix, branch, k.Name(), k.Short)
			printFlags(w, k, prefix+cont+"  ", repeatGlobal)
			walk(k, prefix+cont, level+1)
		}
	}
	walk(cmd, "", 1)
}

func printFlags(w io.Writer, c *cobra.Command, indent string, repeatGlobal bool) {
	for _, f := range ownFlags(c) {
		fmt.Fprintf(w, "%s%s  %s\n", indent, flagSig(f), f.Usage)
	}
	if repeatGlobal {
		for _, f := range globalFlags(c) {
			fmt.Fprintf(w, "%s%s  %s\n", indent, flagSig(f), f.Usage)
		}
	}
}

func writeHelpDocs(cmd *cobra.Command, w io.Writer) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", cmd.CommandPath())
	if cmd.Short != "" {
		fmt.Fprintf(&b, "%s\n\n", cmd.Short)
	}
	if cmd.Long != "" {
		fmt.Fprintf(&b, "%s\n\n", strings.TrimSpace(cmd.Long))
	}
	fmt.Fprintf(&b, "## Usage\n\n```text\n%s\n```\n\n", cmd.UseLine())
	if len(cmd.Aliases) > 0 {
		names := make([]string, len(cmd.Aliases))
		for i, al := range cmd.Aliases {
			names[i] = "`" + al + "`"
		}
		fmt.Fprintf(&b, "Alias: %s (the same command).\n\n", strings.Join(names, ", "))
	}
	if cmd.Example != "" {
		fmt.Fprintf(&b, "## Examples\n\n```text\n%s\n```\n\n", strings.Trim(cmd.Example, "\n"))
	}
	table := func(title string, fl []*pflag.Flag) {
		if len(fl) == 0 {
			return
		}
		fmt.Fprintf(&b, "## %s\n\n| Flag | Default | Description |\n| --- | --- | --- |\n", title)
		for _, f := range fl {
			fmt.Fprintf(&b, "| `%s` | %s | %s |\n", flagSig(f), mdCell(flagDefault(f)), mdCell(f.Usage))
		}
		b.WriteString("\n")
	}
	table("Flags", ownFlags(cmd))
	table("Global flags", globalFlags(cmd))
	if kids := visibleChildren(cmd); len(kids) > 0 {
		b.WriteString("## Subcommands\n\n")
		for _, k := range kids {
			fmt.Fprintf(&b, "- `%s`: %s\n", k.CommandPath(), k.Short)
		}
		b.WriteString("\n")
	}
	if cmd.HasParent() {
		fmt.Fprintf(&b, "## See also\n\n- `%s --help-docs`\n", cmd.Parent().CommandPath())
	}
	_, err := io.WriteString(w, strings.TrimRight(b.String(), "\n")+"\n")
	return err
}

func mdCell(s string) string {
	if s == "" {
		return ""
	}
	return strings.ReplaceAll(strings.ReplaceAll(s, "|", `\|`), "\n", " ")
}
