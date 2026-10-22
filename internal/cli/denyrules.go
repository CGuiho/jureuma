package cli

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/CGuiho/zeze/internal/doc"
	"github.com/CGuiho/zeze/internal/jsonc"
	"github.com/CGuiho/zeze/internal/paths"
	"github.com/CGuiho/zeze/internal/xerr"
)

// Agent-tool deny rules: entries added to the project's Claude Code settings
// (.claude/settings.json permissions.deny) and OpenCode settings
// (opencode.json(c) top-level permissions, V2 ordered rules), recorded in
// <home>/state/deny-rules.json so they can be removed exactly.

var denyRules = []string{
	"Read(**/*.unencrypted.json)",
	"Read(**/*.unencrypted.jsonc)",
	"Read(~/.guiho/zeze/**)",
}

// opencodeDenyRule is one OpenCode V2 permissions entry; the rule fields are
// the live-docs names (action, resource, effect) and the last matching rule
// wins, so these deny rules are appended last at install time.
type opencodeDenyRule struct {
	Action   string `json:"action"`
	Resource string `json:"resource"`
	Effect   string `json:"effect"`
}

var opencodeDenyRules = []opencodeDenyRule{
	{Action: "read", Resource: "*.unencrypted.json", Effect: "deny"},
	{Action: "read", Resource: "*.unencrypted.jsonc", Effect: "deny"},
	{Action: "read", Resource: "$HOME/.guiho/zeze/*", Effect: "deny"},
}

const denyStateFile = "deny-rules.json"

func claudeSettingsPath(root string) string {
	return filepath.Join(root, ".claude", "settings.json")
}

// opencodeConfigPath resolves the project's OpenCode settings: opencode.json,
// or opencode.jsonc when only that exists; a missing file resolves to
// opencode.json (created on apply). zeze never writes .opencode/ or any
// global OpenCode configuration.
func opencodeConfigPath(root string) string {
	jsonPath := filepath.Join(root, "opencode.json")
	if fileExists(jsonPath) {
		return jsonPath
	}
	jsoncPath := filepath.Join(root, "opencode.jsonc")
	if fileExists(jsoncPath) {
		return jsoncPath
	}
	return jsonPath
}

type denyState struct {
	Projects map[string][]string           `json:"projects"`
	OpenCode map[string][]opencodeDenyRule `json:"opencode"`
}

func readDenyState(home string) denyState {
	st := denyState{Projects: map[string][]string{}, OpenCode: map[string][]opencodeDenyRule{}}
	data, err := os.ReadFile(filepath.Join(home, "state", denyStateFile))
	if err != nil {
		return st
	}
	// State files lacking a key keep loading; each map keeps its own meaning.
	if json.Unmarshal(data, &st) != nil {
		return denyState{Projects: map[string][]string{}, OpenCode: map[string][]opencodeDenyRule{}}
	}
	if st.Projects == nil {
		st.Projects = map[string][]string{}
	}
	if st.OpenCode == nil {
		st.OpenCode = map[string][]opencodeDenyRule{}
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
		return added, xerr.Failure("cannot record the deny rules in the zeze state")
	}
	return added, nil
}

// removeDenyRules removes exactly the rules zeze recorded for root.
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
		return removed, xerr.Failure("cannot update the zeze state")
	}
	return removed, nil
}

// ruleOf converts a permissions entry to its rule fields.
func ruleOf(v *doc.Value) (opencodeDenyRule, bool) {
	if v == nil || v.Kind != doc.Object {
		return opencodeDenyRule{}, false
	}
	a, r, e := v.Get("action"), v.Get("resource"), v.Get("effect")
	if a == nil || a.Kind != doc.String || r == nil || r.Kind != doc.String || e == nil || e.Kind != doc.String {
		return opencodeDenyRule{}, false
	}
	return opencodeDenyRule{Action: a.S, Resource: r.S, Effect: e.S}, true
}

func hasRule(arr *doc.Value, rule opencodeDenyRule) bool {
	if arr == nil {
		return false
	}
	for _, x := range arr.A {
		if r, ok := ruleOf(x); ok && r == rule {
			return true
		}
	}
	return false
}

func containsRule(list []opencodeDenyRule, rule opencodeDenyRule) bool {
	for _, r := range list {
		if r == rule {
			return true
		}
	}
	return false
}

// opencodePermArray returns the top-level permissions array of an OpenCode
// settings document, creating it only when create is set. A permissions key of
// another kind is refused: the file must be fixed by hand.
func opencodePermArray(path string, settings *doc.Value, create bool) (*doc.Value, error) {
	perm := settings.Get("permissions")
	if perm == nil {
		if !create {
			return nil, nil
		}
		perm = doc.NewArray()
		settings.Put("permissions", perm)
		return perm, nil
	}
	if perm.Kind != doc.Array {
		return nil, xerr.Failure("%s: permissions must be an array; add the deny rules by hand", path)
	}
	return perm, nil
}

// opencodeDenyRulesPresent reports whether every OpenCode deny rule is in the
// project settings.
func opencodeDenyRulesPresent(root string) bool {
	path := opencodeConfigPath(root)
	settings, err := readSettings(path)
	if err != nil {
		return false
	}
	perm, err := opencodePermArray(path, settings, false)
	if err != nil {
		return false
	}
	for _, r := range opencodeDenyRules {
		if !hasRule(perm, r) {
			return false
		}
	}
	return true
}

// applyOpencodeDenyRules appends the missing rules to the project's
// permissions array (last word at install time) and records what was added.
func applyOpencodeDenyRules(home, root string) (added int, err error) {
	path := opencodeConfigPath(root)
	created := !fileExists(path)
	settings, err := readSettings(path)
	if err != nil {
		return 0, err
	}
	perm, err := opencodePermArray(path, settings, true)
	if err != nil {
		return 0, err
	}
	st := readDenyState(home)
	rec := st.OpenCode[root]
	for _, r := range opencodeDenyRules {
		if hasRule(perm, r) {
			continue
		}
		perm.A = append(perm.A, ruleValue(r))
		added++
		rec = append(rec, r)
	}
	if added == 0 {
		return 0, nil
	}
	if created {
		// A newly created opencode.json carries the schema reference first.
		with := doc.NewObject()
		with.Put("$schema", doc.NewString("https://opencode.ai/config.json"))
		for _, k := range settings.Keys {
			with.Put(k, settings.Get(k))
		}
		settings = with
	}
	if err := paths.AtomicWrite(path, doc.Marshal(settings), 0o644); err != nil {
		return 0, xerr.Failure("%s: cannot write file", path)
	}
	st.OpenCode[root] = rec
	if err := writeDenyState(home, st); err != nil {
		return added, xerr.Failure("cannot record the deny rules in the zeze state")
	}
	return added, nil
}

// removeOpencodeDenyRules removes exactly the rules zeze recorded for root.
func removeOpencodeDenyRules(home, root string) (removed int, err error) {
	st := readDenyState(home)
	rec := st.OpenCode[root]
	if len(rec) == 0 {
		return 0, nil
	}
	path := opencodeConfigPath(root)
	if fileExists(path) {
		settings, err := readSettings(path)
		if err != nil {
			return 0, err
		}
		perm, err := opencodePermArray(path, settings, false)
		if err != nil {
			return 0, err
		}
		if perm != nil {
			var keep []*doc.Value
			for _, x := range perm.A {
				if r, ok := ruleOf(x); ok && containsRule(rec, r) {
					removed++
					continue
				}
				keep = append(keep, x)
			}
			perm.A = keep
			if removed > 0 {
				if err := paths.AtomicWrite(path, doc.Marshal(settings), 0o644); err != nil {
					return 0, xerr.Failure("%s: cannot write file", path)
				}
			}
		}
	}
	delete(st.OpenCode, root)
	if err := writeDenyState(home, st); err != nil {
		return removed, xerr.Failure("cannot update the zeze state")
	}
	return removed, nil
}

// ruleValue renders a rule as a permissions entry with the live-docs field
// names (action, resource, effect).
func ruleValue(r opencodeDenyRule) *doc.Value {
	v := doc.NewObject()
	v.Put("action", doc.NewString(r.Action))
	v.Put("resource", doc.NewString(r.Resource))
	v.Put("effect", doc.NewString(r.Effect))
	return v
}
