// Package config loads, validates, merges, and saves zeze's YAML
// configuration: the project file zeze.yaml and the global file
// ~/.guiho/zeze/zeze.global.yaml.
//
// Both files are validated against embedded JSON Schemas (the same schemas
// published with every release) and then decoded strictly into Go structs.
// Error messages never contain configuration values: they name the file, a
// JSON Pointer built only from known schema keys and array indices, a YAML
// line number, and a fixed reason.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/CGuiho/zeze/internal/crypt"
	"github.com/CGuiho/zeze/internal/paths"
	"github.com/CGuiho/zeze/internal/xerr"
	"go.yaml.in/yaml/v3"
)

// File names.
const (
	ProjectFile = "zeze.yaml"
	GlobalFile  = "zeze.global.yaml"

	// ProjectSchemaFile and GlobalSchemaFile are the release asset names of
	// the published schemas.
	ProjectSchemaFile = "zeze.schema.json"
	GlobalSchemaFile  = "zeze.global.schema.json"
)

// Agent evolution policies.
const (
	PolicyDisabled      = "disabled"
	PolicyAlwaysAsk     = "always-ask"
	PolicyAlwaysProceed = "always-proceed"

	// DefaultPolicy applies to every policy field that neither file sets.
	DefaultPolicy = PolicyAlwaysAsk
)

// run formats and defaults.
const (
	FormatJSON    = "json"
	FormatFlatten = "flatten"
	FormatFile    = "file"

	DefaultFormat  = FormatJSON
	DefaultEnvName = "ZEZE_CONFIG"
)

// maxConfigSize bounds what is read from a configuration file.
const maxConfigSize = 1 << 20

// Project is zeze.yaml.
type Project struct {
	// Key is the full key ID of the one key pair every configured pair of
	// this project uses. "" means the project binds no key. A label is
	// display metadata and is never accepted here: a binding must be
	// verifiable on an agent machine without the keystore.
	Key   string       `yaml:"key"`
	Pairs []PairConfig `yaml:"pairs"`
	Run   RunConfig    `yaml:"run"`
	Agent AgentConfig  `yaml:"agent"`
}

// PairConfig declares one pair. Empty fields are unset.
type PairConfig struct {
	Name        string `yaml:"name"`
	Schema      string `yaml:"schema"`
	Encrypted   string `yaml:"encrypted"`
	Unencrypted string `yaml:"unencrypted"`
}

// RunConfig holds defaults for "zeze run". Empty Format and EnvName mean
// DefaultFormat and DefaultEnvName (see Effective).
type RunConfig struct {
	Format   string `yaml:"format"`
	EnvName  string `yaml:"envName"`
	Prefix   string `yaml:"prefix"`
	Override bool   `yaml:"override"`
}

// Global is zeze.global.yaml.
type Global struct {
	Protections Protections `yaml:"protections"`
	Agent       AgentConfig `yaml:"agent"`
}

// Protections exists only in the global file. A nil AllowBypass means true.
type Protections struct {
	AllowBypass     *bool    `yaml:"allowBypass"`
	AgentEnvMarkers []string `yaml:"agentEnvMarkers"`
}

// BypassAllowed reports whether --dangerously-bypass-protections may be used
// according to this block (nil AllowBypass defaults to true).
func (p Protections) BypassAllowed() bool {
	return p.AllowBypass == nil || *p.AllowBypass
}

// AgentConfig is the agent block shared by both files.
type AgentConfig struct {
	Evolution Evolution `yaml:"evolution"`
}

// Evolution is the agent-evolution policy. Empty fields are unset.
type Evolution struct {
	Upgrade string `yaml:"upgrade"`
	Issues  Issues `yaml:"issues"`
}

// Issues holds the GitHub issue policies. Empty fields are unset.
type Issues struct {
	Bugs         string `yaml:"bugs"`
	Improvements string `yaml:"improvements"`
	Reviews      string `yaml:"reviews"`
}

// EffectiveConfig is the merged configuration a command acts on. (The
// contract names both this type and the merge function "Effective", which Go
// does not allow; the function keeps the name.)
type EffectiveConfig struct {
	Evolution   Evolution
	Run         RunConfig
	Protections Protections
}

// ErrProtectionsInProject is returned (wrapped) when zeze.yaml contains a
// top-level protections key.
var ErrProtectionsInProject = errors.New("zeze.yaml: protections is only allowed in zeze.global.yaml")

// ValidPolicy reports whether s is disabled, always-ask or always-proceed.
func ValidPolicy(s string) bool {
	switch s {
	case PolicyDisabled, PolicyAlwaysAsk, PolicyAlwaysProceed:
		return true
	}
	return false
}

// ValidFormat reports whether s is json, flatten or file.
func ValidFormat(s string) bool {
	switch s {
	case FormatJSON, FormatFlatten, FormatFile:
		return true
	}
	return false
}

var envNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ValidEnvName reports whether s is a portable environment variable name.
func ValidEnvName(s string) bool { return envNameRe.MatchString(s) }

// DefaultEvolution returns every policy set to DefaultPolicy.
func DefaultEvolution() Evolution {
	return Evolution{Upgrade: DefaultPolicy, Issues: Issues{Bugs: DefaultPolicy, Improvements: DefaultPolicy, Reviews: DefaultPolicy}}
}

// DefaultProject returns the configuration "init" writes for a new project.
func DefaultProject() *Project {
	return &Project{
		Pairs: []PairConfig{{Name: "config"}},
		Run:   RunConfig{Format: DefaultFormat, EnvName: DefaultEnvName},
		Agent: AgentConfig{Evolution: DefaultEvolution()},
	}
}

// DefaultGlobal returns the configuration "init" writes for a new user.
func DefaultGlobal() *Global {
	t := true
	return &Global{
		Protections: Protections{AllowBypass: &t, AgentEnvMarkers: []string{}},
		Agent:       AgentConfig{Evolution: DefaultEvolution()},
	}
}

// Validate checks p without the schema (empty strings are unset). It also
// enforces what the schema cannot: unique pair names.
func (p *Project) Validate() error {
	if p == nil {
		return nil
	}
	seen := map[string]int{}
	if p.Key != "" && !crypt.ValidKeyID(p.Key) {
		return fieldErr(ProjectFile, "/key", "must be a key ID (k_ followed by 32 lowercase hex characters); labels are display metadata and are not accepted here")
	}
	for i, pc := range p.Pairs {
		at := fmt.Sprintf("/pairs/%d", i)
		if pc.Name == "" {
			return fieldErr(ProjectFile, at+"/name", "is required")
		}
		if !validStem(pc.Name) {
			return fieldErr(ProjectFile, at+"/name", "must be a relative path without .. segments")
		}
		if j, dup := seen[pc.Name]; dup {
			return fieldErr(ProjectFile, at+"/name", fmt.Sprintf("duplicates /pairs/%d/name", j))
		}
		seen[pc.Name] = i
		if pc.Encrypted != "" && !strings.Contains(pc.Encrypted, ".encrypted.") {
			return fieldErr(ProjectFile, at+"/encrypted", "must contain .encrypted.")
		}
		if pc.Unencrypted != "" && !strings.Contains(pc.Unencrypted, ".unencrypted.") {
			return fieldErr(ProjectFile, at+"/unencrypted", "must contain .unencrypted.")
		}
	}
	if p.Run.Format != "" && !ValidFormat(p.Run.Format) {
		return fieldErr(ProjectFile, "/run/format", "must be json, flatten or file")
	}
	if p.Run.EnvName != "" && !ValidEnvName(p.Run.EnvName) {
		return fieldErr(ProjectFile, "/run/envName", "must match ^[A-Za-z_][A-Za-z0-9_]*$")
	}
	if p.Run.EnvName != "" && DeniedRunEnvName(p.Run.EnvName) {
		return fieldErr(ProjectFile, "/run/envName", "names a denylisted variable (one that loads or runs code, or belongs to zeze)")
	}
	if p.Run.Prefix != "" && !ValidEnvName(p.Run.Prefix) {
		return fieldErr(ProjectFile, "/run/prefix", "must match ^[A-Za-z_][A-Za-z0-9_]*$")
	}
	return validateEvolution(ProjectFile, p.Agent.Evolution)
}

// Validate checks g without the schema (empty strings are unset).
func (g *Global) Validate() error {
	if g == nil {
		return nil
	}
	for i, m := range g.Protections.AgentEnvMarkers {
		if !ValidEnvName(m) {
			return fieldErr(GlobalFile, fmt.Sprintf("/protections/agentEnvMarkers/%d", i), "must match ^[A-Za-z_][A-Za-z0-9_]*$")
		}
	}
	return validateEvolution(GlobalFile, g.Agent.Evolution)
}

func validateEvolution(file string, e Evolution) error {
	fields := []struct{ ptr, v string }{
		{"/agent/evolution/upgrade", e.Upgrade},
		{"/agent/evolution/issues/bugs", e.Issues.Bugs},
		{"/agent/evolution/issues/improvements", e.Issues.Improvements},
		{"/agent/evolution/issues/reviews", e.Issues.Reviews},
	}
	for _, f := range fields {
		if f.v != "" && !ValidPolicy(f.v) {
			return fieldErr(file, f.ptr, "must be disabled, always-ask or always-proceed")
		}
	}
	return nil
}

var driveRe = regexp.MustCompile(`^[A-Za-z]:`)

func validStem(s string) bool {
	if s == "" || strings.HasPrefix(s, "/") || strings.HasPrefix(s, `\`) || driveRe.MatchString(s) {
		return false
	}
	for _, seg := range strings.FieldsFunc(s, func(r rune) bool { return r == '/' || r == '\\' }) {
		if seg == ".." {
			return false
		}
	}
	return true
}

// fieldErr builds the value-free validation error of one configuration field:
// the file, a JSON Pointer built only from known keys and indices, and a fixed
// reason. It carries exit code 4 (validation), so a document that is present
// but invalid fails every load as a validation error.
func fieldErr(file, ptr, reason string) error {
	return xerr.Validation("%s: %s: %s", file, ptr, reason)
}

// Effective merges the two files. For agent.evolution each field set in the
// project overrides the global one, and unset fields default to always-ask.
// Run comes only from the project (with defaults filled in). Protections come
// only from the global file; AllowBypass is always non-nil in the result.
// Either argument may be nil.
func Effective(p *Project, g *Global) EffectiveConfig {
	var pe, ge Evolution
	var run RunConfig
	var prot Protections
	if p != nil {
		pe = p.Agent.Evolution
		run = p.Run
	}
	if g != nil {
		ge = g.Agent.Evolution
		prot.AllowBypass = g.Protections.AllowBypass
		prot.AgentEnvMarkers = append([]string(nil), g.Protections.AgentEnvMarkers...)
	}
	if run.Format == "" {
		run.Format = DefaultFormat
	}
	if run.EnvName == "" {
		run.EnvName = DefaultEnvName
	}
	allow := prot.BypassAllowed()
	prot.AllowBypass = &allow
	return EffectiveConfig{
		Evolution: Evolution{
			Upgrade: mergePolicy(pe.Upgrade, ge.Upgrade),
			Issues: Issues{
				Bugs:         mergePolicy(pe.Issues.Bugs, ge.Issues.Bugs),
				Improvements: mergePolicy(pe.Issues.Improvements, ge.Issues.Improvements),
				Reviews:      mergePolicy(pe.Issues.Reviews, ge.Issues.Reviews),
			},
		},
		Run:         run,
		Protections: prot,
	}
}

func mergePolicy(project, global string) string {
	if ValidPolicy(project) {
		return project
	}
	if ValidPolicy(global) {
		return global
	}
	return DefaultPolicy
}

// LoadProject reads <root>/zeze.yaml. A missing file returns an empty
// Project and false. The file is validated against the embedded schema and
// decoded strictly; a top-level protections key is rejected with
// ErrProtectionsInProject.
func LoadProject(root string) (*Project, bool, error) {
	data, ok, err := readConfig(filepath.Join(root, ProjectFile), ProjectFile)
	if err != nil || !ok {
		return &Project{}, ok, err
	}
	p, err := ParseProject(data)
	if err != nil {
		return &Project{}, true, err
	}
	return p, true, nil
}

// LoadGlobal reads <home>/zeze.global.yaml, where home is the zeze home
// (~/.guiho/zeze). A missing file returns an empty Global and false.
func LoadGlobal(home string) (*Global, bool, error) {
	data, ok, err := readConfig(filepath.Join(home, GlobalFile), GlobalFile)
	if err != nil || !ok {
		return &Global{}, ok, err
	}
	g, err := ParseGlobal(data)
	if err != nil {
		return &Global{}, true, err
	}
	return g, true, nil
}

// ParseProject validates and decodes zeze.yaml content.
func ParseProject(data []byte) (*Project, error) {
	raw, err := decodeRaw(data, ProjectFile)
	if err != nil {
		return nil, err
	}
	if m, ok := raw.(map[string]any); ok {
		if _, has := m["protections"]; has {
			return nil, ErrProtectionsInProject
		}
	}
	if err := validateSchema(projectSchema, raw, ProjectFile); err != nil {
		return nil, err
	}
	p := &Project{}
	if err := strictDecode(data, p, ProjectFile); err != nil {
		return nil, err
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return p, nil
}

// ParseGlobal validates and decodes zeze.global.yaml content.
func ParseGlobal(data []byte) (*Global, error) {
	raw, err := decodeRaw(data, GlobalFile)
	if err != nil {
		return nil, err
	}
	if err := validateSchema(globalSchema, raw, GlobalFile); err != nil {
		return nil, err
	}
	g := &Global{}
	if err := strictDecode(data, g, GlobalFile); err != nil {
		return nil, err
	}
	if err := g.Validate(); err != nil {
		return nil, err
	}
	return g, nil
}

func readConfig(path, name string) ([]byte, bool, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("%s: cannot open: %w", name, osCause(err))
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, false, fmt.Errorf("%s: cannot stat", name)
	}
	if st.IsDir() {
		return nil, false, fmt.Errorf("%s: is a directory", name)
	}
	data, err := io.ReadAll(io.LimitReader(f, maxConfigSize+1))
	if err != nil {
		return nil, false, fmt.Errorf("%s: cannot read", name)
	}
	if len(data) > maxConfigSize {
		return nil, false, fmt.Errorf("%s: larger than %d bytes", name, maxConfigSize)
	}
	return data, true, nil
}

// decodeRaw parses exactly one YAML document into generic values. An empty
// document yields an empty mapping.
func decodeRaw(data []byte, file string) (any, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var raw any
	if err := dec.Decode(&raw); err != nil {
		if errors.Is(err, io.EOF) {
			return map[string]any{}, nil
		}
		return nil, yamlErr(file, err)
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%s: must contain exactly one YAML document", file)
	}
	if raw == nil {
		return map[string]any{}, nil
	}
	return raw, nil
}

func strictDecode(data []byte, out any, file string) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(out); err != nil && !errors.Is(err, io.EOF) {
		return yamlErr(file, err)
	}
	return nil
}

var lineRe = regexp.MustCompile(`line (\d+)`)

// yamlErr converts a yaml.v3 error into a message without source text. The
// library's own messages can quote values, so only line numbers and a fixed
// classification survive.
func yamlErr(file string, err error) error {
	var te *yaml.TypeError
	if errors.As(err, &te) {
		parts := make([]string, 0, len(te.Errors))
		for _, e := range te.Errors {
			parts = append(parts, classifyYAML(e))
		}
		return fmt.Errorf("%s: %s", file, strings.Join(parts, "; "))
	}
	return fmt.Errorf("%s: %s", file, classifyYAML(err.Error()))
}

func classifyYAML(msg string) string {
	reason := "invalid YAML syntax"
	switch {
	case strings.Contains(msg, "not found in type"):
		reason = "unknown field"
	case strings.Contains(msg, "already defined"):
		reason = "duplicate key"
	case strings.Contains(msg, "cannot unmarshal"):
		reason = "wrong value type"
	case strings.Contains(msg, "alias"):
		reason = "invalid alias"
	}
	if m := lineRe.FindStringSubmatch(msg); m != nil {
		return "line " + m[1] + ": " + reason
	}
	return reason
}

// SaveProject validates p and atomically writes <root>/zeze.yaml with a
// schema reference pinned to version (for example "0.1.0" or "v0.1.0").
func SaveProject(root string, p *Project, version string) error {
	if p == nil {
		p = &Project{}
	}
	if err := p.Validate(); err != nil {
		return err
	}
	data, err := marshalProject(p, version)
	if err != nil {
		return err
	}
	if _, err := ParseProject(data); err != nil {
		return err
	}
	if err := paths.AtomicWrite(filepath.Join(root, ProjectFile), data, 0o644); err != nil {
		return fmt.Errorf("%s: cannot write: %w", ProjectFile, osCause(err))
	}
	return nil
}

// SaveGlobal validates g and atomically writes <home>/zeze.global.yaml with
// a schema reference pinned to version.
func SaveGlobal(home string, g *Global, version string) error {
	if g == nil {
		g = &Global{}
	}
	if err := g.Validate(); err != nil {
		return err
	}
	data, err := marshalGlobal(g, version)
	if err != nil {
		return err
	}
	if _, err := ParseGlobal(data); err != nil {
		return err
	}
	if err := os.MkdirAll(home, 0o700); err != nil {
		return fmt.Errorf("%s: cannot create directory", GlobalFile)
	}
	if err := paths.AtomicWrite(filepath.Join(home, GlobalFile), data, 0o600); err != nil {
		return fmt.Errorf("%s: cannot write: %w", GlobalFile, osCause(err))
	}
	return nil
}

var versionRe = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$`)

func normalizeVersion(v string) (string, error) {
	v = strings.TrimPrefix(v, "v")
	if !versionRe.MatchString(v) {
		return "", errors.New("config: version must be a semantic version such as 0.1.0")
	}
	return v, nil
}

// SchemaURL returns the version-pinned release URL of a schema asset.
func SchemaURL(version, schemaFile string) (string, error) {
	v, err := normalizeVersion(version)
	if err != nil {
		return "", err
	}
	return "https://github.com/CGuiho/zeze/releases/download/v" + v + "/" + schemaFile, nil
}

// SchemaHeader returns the yaml-language-server comment line (with newline).
func SchemaHeader(version, schemaFile string) (string, error) {
	u, err := SchemaURL(version, schemaFile)
	if err != nil {
		return "", err
	}
	return "# yaml-language-server: $schema=" + u + "\n", nil
}

// Output mirrors with omitempty so unset fields are not written.
type outPair struct {
	Name        string `yaml:"name"`
	Schema      string `yaml:"schema,omitempty"`
	Encrypted   string `yaml:"encrypted,omitempty"`
	Unencrypted string `yaml:"unencrypted,omitempty"`
}

type outRun struct {
	Format   string `yaml:"format,omitempty"`
	EnvName  string `yaml:"envName,omitempty"`
	Prefix   string `yaml:"prefix,omitempty"`
	Override bool   `yaml:"override,omitempty"`
}

type outIssues struct {
	Bugs         string `yaml:"bugs,omitempty"`
	Improvements string `yaml:"improvements,omitempty"`
	Reviews      string `yaml:"reviews,omitempty"`
}

type outEvolution struct {
	Upgrade string     `yaml:"upgrade,omitempty"`
	Issues  *outIssues `yaml:"issues,omitempty"`
}

type outAgent struct {
	Evolution *outEvolution `yaml:"evolution,omitempty"`
}

type outProject struct {
	Key   string    `yaml:"key,omitempty"`
	Pairs []outPair `yaml:"pairs,omitempty"`
	Run   *outRun   `yaml:"run,omitempty"`
	Agent *outAgent `yaml:"agent,omitempty"`
}

type outProtections struct {
	AllowBypass     *bool    `yaml:"allowBypass,omitempty"`
	AgentEnvMarkers []string `yaml:"agentEnvMarkers"`
}

type outGlobal struct {
	Protections *outProtections `yaml:"protections,omitempty"`
	Agent       *outAgent       `yaml:"agent,omitempty"`
}

func outAgentOf(a AgentConfig) *outAgent {
	e := a.Evolution
	var iss *outIssues
	if e.Issues != (Issues{}) {
		iss = &outIssues{Bugs: e.Issues.Bugs, Improvements: e.Issues.Improvements, Reviews: e.Issues.Reviews}
	}
	if e.Upgrade == "" && iss == nil {
		return nil
	}
	return &outAgent{Evolution: &outEvolution{Upgrade: e.Upgrade, Issues: iss}}
}

func marshalProject(p *Project, version string) ([]byte, error) {
	header, err := SchemaHeader(version, ProjectSchemaFile)
	if err != nil {
		return nil, err
	}
	o := outProject{Key: p.Key, Agent: outAgentOf(p.Agent)}
	for _, pc := range p.Pairs {
		o.Pairs = append(o.Pairs, outPair(pc))
	}
	if p.Run != (RunConfig{}) {
		r := outRun(p.Run)
		o.Run = &r
	}
	return encodeYAML(header, o, ProjectFile)
}

func marshalGlobal(g *Global, version string) ([]byte, error) {
	header, err := SchemaHeader(version, GlobalSchemaFile)
	if err != nil {
		return nil, err
	}
	o := outGlobal{Agent: outAgentOf(g.Agent)}
	if g.Protections.AllowBypass != nil || g.Protections.AgentEnvMarkers != nil {
		markers := g.Protections.AgentEnvMarkers
		if markers == nil {
			markers = []string{}
		}
		o.Protections = &outProtections{AllowBypass: g.Protections.AllowBypass, AgentEnvMarkers: markers}
	}
	return encodeYAML(header, o, GlobalFile)
}

func encodeYAML(header string, v any, file string) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(header)
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("%s: cannot encode", file)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("%s: cannot encode", file)
	}
	return buf.Bytes(), nil
}

// osCause strips path context from filesystem errors, keeping the cause
// (for example "permission denied") so errors.Is still works.
func osCause(err error) error {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}
	var le *os.LinkError
	if errors.As(err, &le) {
		return le.Err
	}
	return errors.New("filesystem error")
}
