package runner

import (
	"fmt"
	"sort"
	"strings"

	"github.com/CGuiho/zeze/internal/config"
	"github.com/CGuiho/zeze/internal/doc"
	"github.com/CGuiho/zeze/internal/xerr"
)

// Denied reports whether flatten mode refuses to produce name: names that
// load or run code (LD_*, DYLD_*, GIT_*, JAVA_*, BASH_ENV, PAGER, EDITOR,
// PATH, ...) or belong to zeze (ZEZE_*); see config.DeniedEnvName.
// --env may still map them explicitly.
func Denied(name string) bool {
	return config.DeniedEnvName(name)
}

// EnvName returns the flattened variable name for an escaped JSON Pointer:
// tokens joined with "_", uppercased, every character outside [A-Z0-9_]
// mapped to "_", "_" prepended when the result starts with a digit, then
// prefix prepended.
func EnvName(pointer, prefix string) (string, error) {
	toks, err := doc.ParsePointer(pointer)
	if err != nil {
		return "", xerr.Validation("%s: invalid JSON Pointer", pointer)
	}
	joined := strings.ToUpper(strings.Join(toks, "_"))
	var b strings.Builder
	for _, r := range joined {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	name := b.String()
	if name != "" && name[0] >= '0' && name[0] <= '9' {
		name = "_" + name
	}
	name = prefix + name
	if name == "" {
		return "", xerr.Validation("%s: produces an empty variable name", displayPointer(pointer))
	}
	return name, nil
}

func displayPointer(p string) string {
	if p == "" {
		return "(root)"
	}
	return p
}

// NameProblems checks the flattened names of pointers for empty names,
// denylisted names and collisions (after normalisation). Messages contain
// pointers and names only.
func NameProblems(pointers []string, prefix string) []string {
	var problems []string
	seen := map[string]string{}
	sorted := append([]string(nil), pointers...)
	sort.Strings(sorted)
	for _, p := range sorted {
		name, err := EnvName(p, prefix)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		if Denied(name) {
			problems = append(problems, fmt.Sprintf("%s: flattens to %s, which is denylisted (map it explicitly with --env)", displayPointer(p), name))
			continue
		}
		if other, ok := seen[name]; ok {
			problems = append(problems, fmt.Sprintf("%s and %s both flatten to %s", displayPointer(other), displayPointer(p), name))
			continue
		}
		seen[name] = p
	}
	return problems
}

// scalarEnv renders a leaf as an environment value. ok is false for null.
func scalarEnv(v *doc.Value) (string, bool) {
	switch v.Kind {
	case doc.Null:
		return "", false
	case doc.Bool:
		if v.B {
			return "true", true
		}
		return "false", true
	case doc.Number:
		return v.N, true
	case doc.String:
		return v.S, true
	}
	return string(doc.MarshalCompact(v)), true
}

// Flatten maps every leaf of v to one variable (see EnvName). null leaves and
// empty containers produce no variable. Collisions and denylisted names are
// errors.
func Flatten(v *doc.Value, prefix string) (map[string]string, error) {
	return FlattenExcept(v, prefix, nil)
}

// FlattenExcept is Flatten, except that a leaf whose pointer is in mapped
// (escaped JSON Pointers mapped explicitly with --env) and whose flattened
// name is denylisted is skipped instead of being an error: the explicit
// --env mapping is the only way it reaches the child.
func FlattenExcept(v *doc.Value, prefix string, mapped map[string]bool) (map[string]string, error) {
	if prefix != "" && !validEnvName(prefix) {
		return nil, xerr.Usage("--prefix must be a valid environment variable name")
	}
	var leaves []doc.Leaf
	for _, l := range doc.Leaves(v) {
		if mapped[l.Pointer] {
			if name, err := EnvName(l.Pointer, prefix); err == nil && Denied(name) {
				continue
			}
		}
		leaves = append(leaves, l)
	}
	ptrs := make([]string, len(leaves))
	for i, l := range leaves {
		ptrs[i] = l.Pointer
	}
	if probs := NameProblems(ptrs, prefix); len(probs) > 0 {
		return nil, xerr.Validation("flatten: %s", strings.Join(probs, "; "))
	}
	out := map[string]string{}
	for _, l := range leaves {
		name, _ := EnvName(l.Pointer, prefix)
		if s, ok := scalarEnv(l.Value); ok {
			if strings.IndexByte(s, 0) >= 0 {
				return nil, xerr.Validation("%s: contains NUL", displayPointer(l.Pointer))
			}
			out[name] = s
		}
	}
	return out, nil
}

func validEnvName(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		ok := r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (i > 0 && r >= '0' && r <= '9')
		if !ok {
			return false
		}
	}
	return true
}
