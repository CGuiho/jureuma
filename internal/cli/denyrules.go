package cli

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/CGuiho/jureuma/internal/doc"
	"github.com/CGuiho/jureuma/internal/jsonc"
	"github.com/CGuiho/jureuma/internal/paths"
	"github.com/CGuiho/jureuma/internal/xerr"
)

// Agent-tool deny rules: entries added to the project's Claude Code settings
// (.claude/settings.json permissions.deny) and recorded in
// <home>/state/deny-rules.json so they can be removed exactly.

var denyRules = []string{
	"Read(**/*.unencrypted.json)",
	"Read(**/*.unencrypted.jsonc)",
	"Read(~/.guiho/jureuma/**)",
}

const denyStateFile = "deny-rules.json"

func claudeSettingsPath(root string) string {
	return filepath.Join(root, ".claude", "settings.json")
}

type denyState struct {
	Projects map[string][]string `json:"projects"`
}

func readDenyState(home string) denyState {
	st := denyState{Projects: map[string][]string{}}
	data, err := os.ReadFile(filepath.Join(home, "state", denyStateFile))
	if err != nil {
		return st
	}
	if json.Unmarshal(data, &st) != nil || st.Projects == nil {
		st.Projects = map[string][]string{}
	}
	return st
}

func writeDenyState(home string, st denyState) error {
	if err := os.MkdirAll(filepath.Join(home, "state"), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return paths.AtomicWrite(filepath.Join(home, "state", denyStateFile), append(data, '\n'), 0o600)
}

func readSettings(path string) (*doc.Value, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return doc.NewObject(), nil
	}
	if err != nil {
		return nil, xerr.Failure("%s: cannot read file", path)
	}
	if jsonc.HasComments(data) {
		return nil, xerr.Failure("%s contains comments; add the deny rules by hand", path)
	}
	v, err := jsonc.Parse(data, false)
	if err != nil {
		return nil, xerr.Failure("%s: %v", path, err)
	}
	if v.Kind != doc.Object {
		return nil, xerr.Failure("%s: top level must be an object", path)
	}
	return v, nil
}

func denyArray(settings *doc.Value, create bool) *doc.Value {
	perm := settings.Get("permissions")
	if perm == nil || perm.Kind != doc.Object {
		if !create {
			return nil
		}
		perm = doc.NewObject()
		settings.Put("permissions", perm)
	}
	deny := perm.Get("deny")
	if deny == nil || deny.Kind != doc.Array {
		if !create {
			return nil
		}
		deny = doc.NewArray()
		perm.Put("deny", deny)
	}
	return deny
}

func hasString(arr *doc.Value, s string) bool {
	if arr == nil {
		return false
	}
	for _, x := range arr.A {
		if x.Kind == doc.String && x.S == s {
			return true
		}
	}
	return false
}

// denyRulesPresent reports whether every deny rule is in the project settings.
func denyRulesPresent(root string) bool {
	settings, err := readSettings(claudeSettingsPath(root))
	if err != nil {
		return false
	}
	deny := denyArray(settings, false)
	for _, r := range denyRules {
		if !hasString(deny, r) {
			return false
		}
	}
	return true
}

// applyDenyRules adds the missing rules and records what was added.
func applyDenyRules(home, root string) (added int, err error) {
	path := claudeSettingsPath(root)
	settings, err := readSettings(path)
	if err != nil {
		return 0, err
	}
	deny := denyArray(settings, true)
	st := readDenyState(home)
	rec := st.Projects[root]
	for _, r := range denyRules {
		if !hasString(deny, r) {
			deny.A = append(deny.A, doc.NewString(r))
			added++
			rec = append(rec, r)
		}
	}
	if added == 0 {
		return 0, nil
	}
	if err := paths.AtomicWrite(path, doc.Marshal(settings), 0o644); err != nil {
		return 0, xerr.Failure("%s: cannot write file", path)
	}
	sort.Strings(rec)
	st.Projects[root] = rec
	if err := writeDenyState(home, st); err != nil {
		return added, xerr.Failure("cannot record the deny rules in the jureuma state")
	}
	return added, nil
}

// removeDenyRules removes exactly the rules jureuma recorded for root.
func removeDenyRules(home, root string) (removed int, err error) {
	st := readDenyState(home)
	rec := st.Projects[root]
	if len(rec) == 0 {
		return 0, nil
	}
	path := claudeSettingsPath(root)
	if fileExists(path) {
		settings, err := readSettings(path)
		if err != nil {
			return 0, err
		}
		if deny := denyArray(settings, false); deny != nil {
			var keep []*doc.Value
			for _, x := range deny.A {
				if x.Kind == doc.String && containsStr(rec, x.S) {
					removed++
					continue
				}
				keep = append(keep, x)
			}
			deny.A = keep
			if removed > 0 {
				if err := paths.AtomicWrite(path, doc.Marshal(settings), 0o644); err != nil {
					return 0, xerr.Failure("%s: cannot write file", path)
				}
			}
		}
	}
	delete(st.Projects, root)
	if err := writeDenyState(home, st); err != nil {
		return removed, xerr.Failure("cannot update the jureuma state")
	}
	return removed, nil
}
