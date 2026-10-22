package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/CGuiho/zeze/internal/paths"
	"github.com/CGuiho/zeze/internal/xerr"
	"go.yaml.in/yaml/v3"
)

// isolate points the user home at a temp dir so nothing touches the real home.
func isolate(t *testing.T) string {
	t.Helper()
	h := t.TempDir()
	t.Setenv(paths.HomeOverrideEnv, h)
	home, err := paths.Home()
	if err != nil {
		t.Fatal(err)
	}
	return home
}

func boolp(b bool) *bool { return &b }

// secretish is a marker that must never appear in an error message.
const secretish = "S3cr3tV4lu3"

func TestValidPolicy(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want bool
	}{
		{"disabled", true}, {"always-ask", true}, {"always-proceed", true},
		{"", false}, {"Always-Ask", false}, {"true", false}, {"ask", false},
	} {
		if got := ValidPolicy(tc.in); got != tc.want {
			t.Errorf("ValidPolicy(%q)=%v", tc.in, got)
		}
	}
}

func TestParseProjectValid(t *testing.T) {
	p, err := ParseProject(ProjectExampleYAML())
	if err != nil {
		t.Fatalf("example: %v", err)
	}
	if len(p.Pairs) != 2 || p.Pairs[0].Name != "config" || p.Pairs[1].Encrypted != "./production.encrypted.json" {
		t.Fatalf("pairs: %+v", p.Pairs)
	}
	if p.Key != "k_5c1e0a9f3b7d2e4c8a6f1b0d9e3c7a25" {
		t.Fatalf("key binding: %q", p.Key)
	}
	if p.Run.Format != "json" || p.Run.EnvName != "ZEZE_CONFIG" || p.Agent.Evolution.Issues.Reviews != "always-ask" {
		t.Fatalf("decoded: %+v", p)
	}
	for _, s := range []string{"", "\n", "# only a comment\n", "~\n", "{}\n", "pairs: []\n", "run:\n  format: flatten\n  prefix: APP_\n  override: true\n"} {
		if _, err := ParseProject([]byte(s)); err != nil {
			t.Errorf("%q: %v", s, err)
		}
	}
}

func TestParseProjectInvalid(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"protections", "protections:\n  allowBypass: true\n", "protections is only allowed"},
		{"protections empty", "protections: {}\n", "protections is only allowed"},
		{"unknown top", secretish + ": 1\n", "unknown field"},
		{"unknown nested", "run:\n  " + secretish + ": x\n", "/run: unknown field"},
		{"bad format", "run:\n  format: " + secretish + "\n", "/run/format: value not allowed"},
		{"empty format", "run:\n  format: \"\"\n", "/run/format"},
		{"bad envName", "run:\n  envName: \"1" + secretish + "\"\n", "/run/envName: does not match"},
		{"bad prefix", "run:\n  prefix: \"a-" + secretish + "\"\n", "/run/prefix"},
		{"override type", "run:\n  override: " + secretish + "\n", "/run/override: wrong type"},
		{"policy bool", "agent:\n  evolution:\n    upgrade: true\n", "/agent/evolution/upgrade: value not allowed"},
		{"policy unknown", "agent:\n  evolution:\n    issues:\n      bugs: " + secretish + "\n", "/agent/evolution/issues/bugs"},
		{"pair no name", "pairs:\n  - schema: x.schema.json\n", "/pairs/0: missing required field"},
		{"pair traversal", "pairs:\n  - name: ../" + secretish + "\n", "/pairs/0/name: not allowed"},
		{"pair absolute", "pairs:\n  - name: /etc/" + secretish + "\n", "/pairs/0/name"},
		{"pair dup", "pairs:\n  - name: a\n  - name: a\n", "/pairs/1/name: duplicates /pairs/0/name"},
		{"pair encrypted", "pairs:\n  - name: a\n    encrypted: " + secretish + ".json\n", "/pairs/0/encrypted"},
		{"pair unencrypted", "pairs:\n  - name: a\n    unencrypted: a.encrypted.json\n", "/pairs/0/unencrypted"},
		{"dup key", "run:\n  format: json\n  format: " + secretish + "\n", "duplicate key"},
		{"key label", "key: " + secretish + "\n", "/key: does not match"},
		{"key empty", "key: \"\"\n", "/key: does not match"},
		{"key null", "key:\n", "/key: wrong type"},
		{"key wrong type", "key: [" + secretish + "]\n", "/key: wrong type"},
		{"key short", "key: k_" + secretish + "\n", "/key: does not match"},
		{"key uppercase", "key: k_" + strings.Repeat("A", 32) + "\n", "/key: does not match"},
		{"syntax", "run: [" + secretish + "\n", "invalid YAML syntax"},
		{"two docs", "run: {}\n---\nrun: {}\n", "exactly one YAML document"},
		{"not mapping", secretish + "\n", "(root): wrong type"},
		{"non-string key", "1: " + secretish + "\n", "not representable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseProject([]byte(tc.in))
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not contain %q", err, tc.want)
			}
			if strings.Contains(err.Error(), secretish) {
				t.Errorf("error leaks content: %q", err)
			}
		})
	}
	if _, err := ParseProject([]byte("protections: {}\n")); !errors.Is(err, ErrProtectionsInProject) {
		t.Errorf("want ErrProtectionsInProject, got %v", err)
	}
}

// A configuration document that is present but invalid is a validation error
// (exit 4), never a plain failure: the empty or malformed key binding and
// every other schema/semantic refusal share that classification.
func TestConfigErrorsAreValidationErrors(t *testing.T) {
	for _, in := range []string{
		"key: " + secretish + "\n",
		"key: \"\"\n",
		"key:\n",
		"run:\n  format: " + secretish + "\n",
		"run:\n  envName: ZEZE_\n",
		"pairs:\n  - name: ../" + secretish + "\n",
	} {
		_, err := ParseProject([]byte(in))
		if err == nil || xerr.CodeOf(err) != xerr.CodeValidation {
			t.Errorf("%q: got %v, want exit %d", in, err, xerr.CodeValidation)
		}
	}
	if _, err := ParseGlobal([]byte("protections:\n  allowBypass: " + secretish + "\n")); err == nil || xerr.CodeOf(err) != xerr.CodeValidation {
		t.Errorf("global: got %v, want exit %d", err, xerr.CodeValidation)
	}
}

func TestParseGlobal(t *testing.T) {
	g, err := ParseGlobal(GlobalExampleYAML())
	if err != nil {
		t.Fatalf("example: %v", err)
	}
	if g.Protections.AllowBypass == nil || !*g.Protections.AllowBypass || g.Protections.AgentEnvMarkers == nil {
		t.Fatalf("protections: %+v", g.Protections)
	}
	bad := []struct{ in, want string }{
		{"pairs: []\n", "unknown field"},
		{"run: {}\n", "unknown field"},
		{"protections:\n  allowBypass: " + secretish + "\n", "/protections/allowBypass: wrong type"},
		{"protections:\n  agentEnvMarkers: [\"a b " + secretish + "\"]\n", "/protections/agentEnvMarkers/0"},
		{"protections:\n  " + secretish + ": 1\n", "/protections: unknown field"},
		{"agent:\n  evolution:\n    upgrade: never\n", "/agent/evolution/upgrade"},
	}
	for _, tc := range bad {
		_, err := ParseGlobal([]byte(tc.in))
		if err == nil || !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), secretish) {
			t.Errorf("%q: got %v, want %q", tc.in, err, tc.want)
		}
	}
}

func TestLoadMissing(t *testing.T) {
	home := isolate(t)
	p, ok, err := LoadProject(t.TempDir())
	if err != nil || ok || p == nil {
		t.Fatalf("project: %v %v %v", p, ok, err)
	}
	g, ok, err := LoadGlobal(home)
	if err != nil || ok || g == nil {
		t.Fatalf("global: %v %v %v", g, ok, err)
	}
}

func TestLoadErrors(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ProjectFile), []byte("protections: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, ok, err := LoadProject(root)
	if !ok || !errors.Is(err, ErrProtectionsInProject) {
		t.Fatalf("got %v %v", ok, err)
	}
	if strings.Contains(err.Error(), root) {
		t.Errorf("error contains path: %v", err)
	}
	big := bytes.Repeat([]byte("#"), maxConfigSize+1)
	if err := os.WriteFile(filepath.Join(root, ProjectFile), big, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadProject(root); err == nil {
		t.Fatal("expected size error")
	}
	dirRoot := t.TempDir()
	os.Mkdir(filepath.Join(dirRoot, ProjectFile), 0o755)
	if _, _, err := LoadProject(dirRoot); err == nil {
		t.Fatal("expected directory error")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	home := isolate(t)
	root := t.TempDir()
	p := &Project{
		Key:   "k_5c1e0a9f3b7d2e4c8a6f1b0d9e3c7a25",
		Pairs: []PairConfig{{Name: "config", Schema: "./config.schema.json"}, {Name: "apps/api/production", Encrypted: "apps/api/p.encrypted.json", Unencrypted: "apps/api/p.unencrypted.jsonc"}},
		Run:   RunConfig{Format: "flatten", EnvName: "APP_CONFIG", Prefix: "APP_", Override: true},
		Agent: AgentConfig{Evolution: Evolution{Upgrade: "disabled", Issues: Issues{Bugs: "always-proceed"}}},
	}
	if err := SaveProject(root, p, "v1.2.3"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, ProjectFile))
	wantHeader := "# yaml-language-server: $schema=https://github.com/CGuiho/zeze/releases/download/v1.2.3/zeze.schema.json\n"
	if !strings.HasPrefix(string(data), wantHeader) {
		t.Fatalf("header:\n%s", data)
	}
	if strings.Contains(string(data), "improvements") {
		t.Errorf("unset field written:\n%s", data)
	}
	got, ok, err := LoadProject(root)
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	if !reflect.DeepEqual(got, p) {
		t.Fatalf("round trip:\n%+v\n%+v", got, p)
	}

	g := DefaultGlobal()
	g.Protections.AgentEnvMarkers = []string{"MY_AGENT"}
	if err := SaveGlobal(home, g, "0.1.0"); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(filepath.Join(home, GlobalFile))
	if !strings.HasPrefix(string(data), "# yaml-language-server: $schema=https://github.com/CGuiho/zeze/releases/download/v0.1.0/zeze.global.schema.json\n") {
		t.Fatalf("global header:\n%s", data)
	}
	gg, ok, err := LoadGlobal(home)
	if err != nil || !ok || !reflect.DeepEqual(gg, g) {
		t.Fatalf("global round trip: %v %v\n%+v\n%+v", ok, err, gg, g)
	}

	// Defaults and empty configs also round-trip.
	if err := SaveProject(root, DefaultProject(), "0.1.0"); err != nil {
		t.Fatal(err)
	}
	if got, _, err := LoadProject(root); err != nil || !reflect.DeepEqual(got, DefaultProject()) {
		t.Fatalf("default project: %v %+v", err, got)
	}
	if err := SaveProject(root, &Project{}, "0.1.0"); err != nil {
		t.Fatal(err)
	}
	if err := SaveGlobal(home, &Global{}, "0.1.0"); err != nil {
		t.Fatal(err)
	}
	// No temp files left behind.
	entries, _ := os.ReadDir(root)
	if len(entries) != 1 {
		t.Errorf("unexpected files: %v", entries)
	}
}

func TestSaveRejects(t *testing.T) {
	root := t.TempDir()
	cases := []struct {
		p       *Project
		version string
	}{
		{&Project{Run: RunConfig{Format: secretish}}, "0.1.0"},
		{&Project{Agent: AgentConfig{Evolution: Evolution{Upgrade: secretish}}}, "0.1.0"},
		{&Project{Pairs: []PairConfig{{Name: ""}}}, "0.1.0"},
		{&Project{}, ""},
		{&Project{}, "latest"},
		{&Project{}, "main"},
	}
	for i, tc := range cases {
		err := SaveProject(root, tc.p, tc.version)
		if err == nil || strings.Contains(err.Error(), secretish) {
			t.Errorf("case %d: %v", i, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ProjectFile)); !os.IsNotExist(err) {
		t.Error("invalid config was written")
	}
	g := &Global{Protections: Protections{AgentEnvMarkers: []string{"bad name"}}}
	if err := SaveGlobal(t.TempDir(), g, "0.1.0"); err == nil {
		t.Error("bad marker accepted")
	}
}

func TestEffective(t *testing.T) {
	f := false
	g := &Global{
		Protections: Protections{AllowBypass: &f, AgentEnvMarkers: []string{"X_AGENT"}},
		Agent:       AgentConfig{Evolution: Evolution{Upgrade: "disabled", Issues: Issues{Bugs: "always-proceed", Reviews: "disabled"}}},
	}
	p := &Project{
		Run:   RunConfig{Prefix: "APP_"},
		Agent: AgentConfig{Evolution: Evolution{Issues: Issues{Bugs: "always-ask", Improvements: "always-proceed"}}},
	}
	tests := []struct {
		name string
		p    *Project
		g    *Global
		want EffectiveConfig
	}{
		{"nil both", nil, nil, EffectiveConfig{
			Evolution:   DefaultEvolution(),
			Run:         RunConfig{Format: "json", EnvName: "ZEZE_CONFIG"},
			Protections: Protections{AllowBypass: boolp(true)},
		}},
		{"merge", p, g, EffectiveConfig{
			Evolution:   Evolution{Upgrade: "disabled", Issues: Issues{Bugs: "always-ask", Improvements: "always-proceed", Reviews: "disabled"}},
			Run:         RunConfig{Format: "json", EnvName: "ZEZE_CONFIG", Prefix: "APP_"},
			Protections: Protections{AllowBypass: boolp(false), AgentEnvMarkers: []string{"X_AGENT"}},
		}},
		{"project only", p, nil, EffectiveConfig{
			Evolution:   Evolution{Upgrade: "always-ask", Issues: Issues{Bugs: "always-ask", Improvements: "always-proceed", Reviews: "always-ask"}},
			Run:         RunConfig{Format: "json", EnvName: "ZEZE_CONFIG", Prefix: "APP_"},
			Protections: Protections{AllowBypass: boolp(true)},
		}},
		{"run explicit", &Project{Run: RunConfig{Format: "file", EnvName: "CFG"}}, g, EffectiveConfig{
			Evolution:   Evolution{Upgrade: "disabled", Issues: Issues{Bugs: "always-proceed", Improvements: "always-ask", Reviews: "disabled"}},
			Run:         RunConfig{Format: "file", EnvName: "CFG"},
			Protections: Protections{AllowBypass: boolp(false), AgentEnvMarkers: []string{"X_AGENT"}},
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Effective(tc.p, tc.g)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got  %+v\nwant %+v", got, tc.want)
			}
		})
	}
	// Mutating the result must not alias the global.
	e := Effective(p, g)
	e.Protections.AgentEnvMarkers[0] = "Y"
	if g.Protections.AgentEnvMarkers[0] != "X_AGENT" {
		t.Error("Effective aliases global markers")
	}
}

func TestProjectNeverLoosensProtections(t *testing.T) {
	// Even if a project struct were built by hand, Effective reads
	// protections only from the global.
	g := &Global{Protections: Protections{AllowBypass: boolp(false)}}
	if Effective(&Project{}, g).Protections.BypassAllowed() {
		t.Fatal("bypass allowed")
	}
}

func TestSchemasAndExamples(t *testing.T) {
	for _, b := range [][]byte{ProjectSchemaJSON(), GlobalSchemaJSON()} {
		var v map[string]any
		if err := json.Unmarshal(b, &v); err != nil {
			t.Fatal(err)
		}
		if v["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
			t.Errorf("$schema: %v", v["$schema"])
		}
	}
	// Returned slices are copies.
	s := ProjectSchemaJSON()
	s[0] = 'X'
	if ProjectSchemaJSON()[0] == 'X' {
		t.Error("schema bytes are shared")
	}
	// Examples validate and carry pinned headers.
	if !strings.HasPrefix(string(ProjectExampleYAML()), "# yaml-language-server: $schema=https://github.com/CGuiho/zeze/releases/download/v") {
		t.Error("project example header")
	}
	if !strings.HasPrefix(string(GlobalExampleYAML()), "# yaml-language-server: $schema=https://github.com/CGuiho/zeze/releases/download/v") {
		t.Error("global example header")
	}
	// The design's sample global block is valid against the global schema
	// but rejected by the project schema.
	var raw any
	if err := yaml.Unmarshal(GlobalExampleYAML(), &raw); err != nil {
		t.Fatal(err)
	}
	if err := validateSchema(projectSchema, raw, ProjectFile); err == nil {
		t.Error("project schema accepts protections")
	}
	// The example files on disk equal the embedded ones.
	for name, emb := range map[string][]byte{"examples/" + ProjectFile: ProjectExampleYAML(), "examples/" + GlobalFile: GlobalExampleYAML()} {
		disk, err := os.ReadFile(name)
		if err != nil || !bytes.Equal(disk, emb) {
			t.Errorf("%s differs from embedded copy", name)
		}
	}
}

// Go-side Validate and the JSON Schema must agree on every struct the
// program can save.
func TestValidateAgreesWithSchema(t *testing.T) {
	projects := []*Project{
		DefaultProject(),
		{Run: RunConfig{Format: "nope"}},
		{Run: RunConfig{EnvName: "9X"}},
		{Run: RunConfig{Prefix: "a.b"}},
		{Pairs: []PairConfig{{Name: `C:\x`}}},
		{Pairs: []PairConfig{{Name: `a\..\b`}}},
		{Pairs: []PairConfig{{Name: "a/b..c"}}},
		{Pairs: []PairConfig{{Name: "a", Encrypted: "a.json"}}},
		{Agent: AgentConfig{Evolution: Evolution{Issues: Issues{Reviews: "maybe"}}}},
	}
	for i, p := range projects {
		goErr := p.Validate()
		data, err := marshalProject(p, "0.1.0")
		if err != nil {
			t.Fatal(err)
		}
		var raw any
		if err := yaml.Unmarshal(data, &raw); err != nil {
			t.Fatal(err)
		}
		if raw == nil {
			raw = map[string]any{}
		}
		schemaErr := validateSchema(projectSchema, raw, ProjectFile)
		if (goErr == nil) != (schemaErr == nil) {
			t.Errorf("case %d: go=%v schema=%v", i, goErr, schemaErr)
		}
	}
}

func TestSchemaURL(t *testing.T) {
	for _, tc := range []struct {
		v, want string
		ok      bool
	}{
		{"0.1.0", "https://github.com/CGuiho/zeze/releases/download/v0.1.0/zeze.schema.json", true},
		{"v2.0.0-rc.1", "https://github.com/CGuiho/zeze/releases/download/v2.0.0-rc.1/zeze.schema.json", true},
		{"latest", "", false},
		{"1.2", "", false},
		{"1.2.3/../x", "", false},
	} {
		got, err := SchemaURL(tc.v, ProjectSchemaFile)
		if (err == nil) != tc.ok || got != tc.want {
			t.Errorf("%q: %q %v", tc.v, got, err)
		}
	}
}

func TestValidateRejectsDenylistedRunEnvName(t *testing.T) {
	for _, name := range []string{"BASH_ENV", "PAGER", "GIT_SSH_COMMAND", "LD_PRELOAD", "ZEZE_USER_HOME", "ZEZE_"} {
		p := &Project{Run: RunConfig{EnvName: name}}
		if err := p.Validate(); err == nil {
			t.Errorf("run.envName %s accepted", name)
		}
	}
	for _, name := range []string{"ZEZE_CONFIG", "APP_CONFIG"} {
		p := &Project{Run: RunConfig{EnvName: name}}
		if err := p.Validate(); err != nil {
			t.Errorf("run.envName %s refused: %v", name, err)
		}
	}
}

// The default run env name is the denylist's only exception; the abandoned
// ZEZE_ name is an ordinary denylisted ZEZE_* name.
func TestRunEnvNameDefaultIsTheDenylistException(t *testing.T) {
	if DefaultEnvName != "ZEZE_CONFIG" {
		t.Fatalf("DefaultEnvName = %q", DefaultEnvName)
	}
	p, err := ParseProject([]byte("pairs:\n  - name: config\nrun:\n  envName: ZEZE_CONFIG\n"))
	if err != nil {
		t.Fatalf("run.envName ZEZE_CONFIG must load: %v", err)
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("run.envName ZEZE_CONFIG refused: %v", err)
	}
	if _, err := ParseProject([]byte("pairs:\n  - name: config\nrun:\n  envName: ZEZE_\n")); err == nil {
		t.Fatal("run.envName ZEZE_ must be rejected as denylisted")
	}
}
