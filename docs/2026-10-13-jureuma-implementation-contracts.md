---
name: Jureuma Implementation Contracts
purpose: Fix the Go package layout and the public API of every internal package so they can be built in parallel and integrated without guesswork.
description: Package-by-package Go API contracts for the jureuma CLI, derived from the jureuma design document.
created: "2026-10-13"
owner: jureuma-docs
flags: []
tags:
  - docs
keywords:
  - contracts
  - packages
  - API
---

# Jureuma Implementation Contracts

The behaviour is defined in [the design](./2026-10-13-jureuma-design.md). This file fixes **where** it lives and the **exported API** each package must provide. Packages may add unexported helpers and extra exported helpers, but must not change these signatures.

Module: `github.com/CGuiho/jureuma`. Go 1.26. Dependencies allowed: `github.com/spf13/cobra`, `github.com/santhosh-tekuri/jsonschema/v6`, `go.yaml.in/yaml/v3`, and the standard library. Nothing else without a strong reason.

Every package has table-driven tests (`go test ./internal/<pkg>/...`) and must pass `go vet`. No package except `cli` and `runner` writes to stdout/stderr. **No error message anywhere may contain a configuration value, plaintext, or key material.**

## `internal/doc` (done)

Ordered JSON tree. See `internal/doc/doc.go`: `Value`, `Kind`, `ParsePointer`, `FormatPointer`, `EscapeToken`, `Lookup`, `SetAt`, `DeleteAt`, `Leaves`, `MapLeaves`, `Canonical`, `Marshal`, `MarshalCompact`, `ToAny`, `FromAny`, `Equal`, `ValidNumber`.

## `internal/jsonc`

```go
// Parse parses JSON (strict=true: RFC 8259 only) or JSONC (strict=false:
// // and /* */ comments, trailing commas in objects and arrays) into a doc.Value.
// Duplicate object keys, strings containing NUL (U+0000), invalid UTF-8,
// and trailing non-whitespace content are errors.
func Parse(data []byte, strict bool) (*doc.Value, error)

// HasComments reports whether JSONC data contains at least one comment.
func HasComments(data []byte) bool

// SyntaxError is the only error type Parse returns. Its Error() is
// "line L, column C: <Class>" and never includes source text.
type SyntaxError struct { Line, Column int; Class string }
```

Classes: `unexpected character`, `unexpected end of input`, `invalid escape`, `invalid number`, `duplicate key`, `NUL in string`, `invalid UTF-8`, `unterminated comment`, `trailing content`, `nesting too deep` (depth limit `MaxDepth`). A leading UTF-8 BOM is skipped.

## `internal/crypt`

Implements the design's Cryptography section exactly (X25519, HKDF-SHA256, AES-256-GCM, padding buckets, AAD layout, strict base64url).

```go
type PublicKey  [32]byte
type PrivateKey [32]byte

func GenerateKey() (PrivateKey, error)
func (k PrivateKey) Public() PublicKey
func (k PublicKey) String() string                 // "jureuma-pk-1:" + base64url
func (k PrivateKey) String() string                // "jureuma-sk-1:" + base64url
func ParsePublicKey(s string) (PublicKey, error)
func ParsePrivateKey(s string) (PrivateKey, error)
func KeyID(pk PublicKey) string                    // "k_" + 32 hex chars
func ValidKeyID(id string) bool                    // ^k_[0-9a-f]{32}$

// EncryptValue seals canonical scalar JSON for the leaf at pointer.
// Output: "jureuma:1:<keyId>:<base64url(eph_pk||nonce||sealed)>".
func EncryptValue(pk PublicKey, pointer string, canonical []byte) (string, error)
// DecryptValue reverses EncryptValue. Any failure returns ErrDecrypt (no detail).
func DecryptValue(sk PrivateKey, pointer string, ciphertext string) ([]byte, error)
// IsCiphertext reports whether s has the "jureuma:1:" envelope prefix and a valid key ID.
func IsCiphertext(s string) bool
// CiphertextKeyID extracts the key ID from an envelope.
func CiphertextKeyID(s string) (string, bool)

// SyncTag returns hex(first 16 bytes of SHA-256(ciphertext)).
func SyncTag(ciphertext string) string
// PlainTag returns hex(first 16 bytes of HMAC-SHA256(HKDF(sk,"jureuma/1/sync"), canonical)).
func PlainTag(sk PrivateKey, canonical []byte) string

var ErrDecrypt = errors.New("decryption failed")
```

AAD prefix string is `"jureuma/1/aad"`, HKDF info strings `"jureuma/1/value"` and `"jureuma/1/sync"`, key-ID domain `"jureuma/1/keyid"`.

## `internal/keystore`

```go
// Store resolves private keys. Home is the CLI home (~/.guiho/jureuma).
type Store struct { Home string; Getenv func(string) string }

func New(home string) *Store                        // Getenv = os.Getenv
// Find returns the private key for keyID from, in order: env
// JUREUMA_PRIVATE_KEY_<UPPER(keyID)>, env JUREUMA_PRIVATE_KEY (only if it
// matches), keys/<keyID>.json, keys/retired/<keyID>.json.
// Returns ErrNoKey when none exists, *MismatchError when a source holds a
// different key.
func (s *Store) Find(keyID string) (crypt.PrivateKey, Source, error)
func (s *Store) Save(sk crypt.PrivateKey) (path string, err error)   // keys/<id>.json, 0600, atomic
func (s *Store) Retire(keyID string) error                          // move to keys/retired/
func (s *Store) Delete(keyID string) error                          // retired only
func (s *Store) List() ([]Entry, error)
// Source names where a key came from, e.g. "env:JUREUMA_PRIVATE_KEY", "file:keys/k_….json".
type Source string
type Entry struct { KeyID string; Retired bool; Path string }
type MismatchError struct { Source Source }
var ErrNoKey = errors.New("private key not available")
// PrivateKeyEnvNames are stripped from child environments by runner.
func IsPrivateKeyEnv(name string) bool
```

Key file format: `{"keyId": "...", "publicKey": "jureuma-pk-1:...", "privateKey": "jureuma-sk-1:...", "created": "RFC3339"}`. Reject symlinks, files not owned by the current user, and mode wider than 0600 on Unix. `keyID` is validated before any path is built.

## `internal/gate`

```go
// Gate decides whether a protected operation may run.
type Gate struct {
    Stdin, Stderr *os.File
    Getenv func(string) string
    ExtraMarkers []string
    Prompt io.Reader            // defaults to Stdin
    Out    io.Writer            // defaults to Stderr
    IsTerminal func(*os.File) bool
    Word func() string          // random confirmation word; injectable for tests
}
// Check runs the three checks and asks the user to type the confirmation
// word describing action (e.g. "reveal 1 value"). Returns ErrRefused with a
// reason when any check fails.
func (g *Gate) Check(action string) error
func DetectAgent(getenv func(string) string, extra []string) (marker string, found bool)
var DefaultMarkers = []string{"CLAUDECODE", "CLAUDE_CODE_ENTRYPOINT", "CODEX_SANDBOX", "CODEX_HOME", "CURSOR_AGENT", "CURSOR_TRACE_ID", "GEMINI_CLI", "AIDER", "OPENCODE", "AMP_AGENT", "CLINE", "GOOSE_SESSION", "WINDSURF_AGENT", "COPILOT_AGENT", "JUREUMA_AGENT"}
var ErrRefused = errors.New("protected operation refused")

// Audit appends one JSON line to <home>/audit.log (0600). Never values.
func Audit(home string, ev AuditEvent) error
type AuditEvent struct { Time time.Time; Command string; Paths []string; Outcome string; Markers []string; Bypass bool }
```

`IsTerminal` uses `golang.org/x/term`-free detection: `(*os.File).Stat()` and `os.ModeCharDevice`.

## `internal/redact`

```go
// Writer wraps w and replaces every occurrence of any secret with "[REDACTED]",
// streaming and boundary-safe: it holds back at most maxLen-1 bytes until it
// can decide. Flush on Close.
func NewWriter(w io.Writer, secrets [][]byte) *Writer
func (r *Writer) Write(p []byte) (int, error)
func (r *Writer) Close() error
// Variants expands one secret into the forms to redact: raw, base64 std,
// base64 url (padded and raw), URL query-escaped, and JSON-escaped (without quotes).
func Variants(secret string) []string
```

Longest match wins; overlapping secrets are handled. Use an Aho-Corasick-style or simple multi-pattern scan; correctness over speed, but linear-ish for normal output.

## `internal/schema`

```go
// Load reads a .json or .jsonc schema file (draft 2020-12 assumed if $schema absent)
// and compiles it. Unknown keywords other than x-* are errors. format is asserted for:
// date-time, date, time, email, hostname, ipv4, ipv6, uri, uuid; other formats are errors.
func Load(path string) (*Schema, error)
func Compile(raw *doc.Value, location string) (*Schema, error)
type Schema struct { /* compiled validator + raw tree */ }
// Validate validates a whole document. Issues never contain instance values.
func (s *Schema) Validate(v *doc.Value) []Issue
// ValidateValue provisionally validates one value at pointer with siblings unknown:
// accept if any schema branch that could apply to the location accepts it.
func (s *Schema) ValidateValue(pointer string, v *doc.Value) []Issue
// ValidateValueUnknown also reports unknown=true when no subschema could be determined.
func (s *Schema) ValidateValueUnknown(pointer string, v *doc.Value) (issues []Issue, unknown bool)
// ValidateStructure checks only structural keywords against a document whose
// leaves are opaque (encrypted): required, additionalProperties, propertyNames,
// minItems/maxItems, minProperties/maxProperties. Leaf values are ignored.
func (s *Schema) ValidateStructure(v *doc.Value) []Issue
// ExpectedKind returns the JSON type(s) the schema declares at pointer, if determinable.
func (s *Schema) ExpectedTypes(pointer string) []string
// Secret reports whether the schema marks the pointer with "x-jureuma-secret": true.
func (s *Schema) Secret(pointer string) bool
type Issue struct { Pointer string; Keyword string; Message string } // Message from schema-side params only
// Infer builds a starter schema from structure and types (see design). If
// typesKnown is false (encrypted source), leaves get {} (any).
func Infer(v *doc.Value, typesKnown bool) *doc.Value
// Starter returns {"$schema": "...2020-12/schema", "type": "object"}.
func Starter() *doc.Value
func IsStarter(raw *doc.Value) bool
```

Use `santhosh-tekuri/jsonschema/v6` for full validation; translate its `ValidationError` tree into `Issue`s using only instance location and keyword location plus values read from the **schema** (e.g. `maximum 100`), never the library's message text.

## `internal/config`

```go
type Project struct {
    Pairs []PairConfig `yaml:"pairs"`
    Run   RunConfig    `yaml:"run"`
    Agent AgentConfig  `yaml:"agent"`
}
type PairConfig struct { Name, Schema, Encrypted, Unencrypted string }
type RunConfig  struct { Format string; EnvName string `yaml:"envName"`; Prefix string; Override bool }
type Global struct {
    Protections Protections `yaml:"protections"`
    Agent       AgentConfig `yaml:"agent"`
}
type Protections struct { AllowBypass *bool `yaml:"allowBypass"`; AgentEnvMarkers []string `yaml:"agentEnvMarkers"` }
type AgentConfig struct { Evolution Evolution `yaml:"evolution"` }
type Evolution struct { Upgrade string; Issues Issues }
type Issues struct { Bugs, Improvements, Reviews string }

const ProjectFile = "jureuma.yaml"
const GlobalFile  = "jureuma.global.yaml"
func LoadProject(root string) (*Project, bool, error)  // bool: existed. Strict decode; "protections" key rejected.
func LoadGlobal(home string) (*Global, bool, error)
func SaveProject(root string, p *Project, version string) error  // writes schema-reference header, atomic
func SaveGlobal(home string, g *Global, version string) error
func Effective(p *Project, g *Global) EffectiveConfig   // project overrides global for agent.evolution
type EffectiveConfig struct { Evolution Evolution; Run RunConfig; Protections Protections } // Go forbids a type and func both named Effective
func ValidPolicy(s string) bool                          // disabled | always-ask | always-proceed
// ProjectSchemaJSON / GlobalSchemaJSON return the embedded JSON Schemas (also published per release).
func ProjectSchemaJSON() []byte
func GlobalSchemaJSON() []byte
// ProtectionsHMAC helpers (design: Human-only gate / bypass).
func SealProtections(home string, g *Global) error
func ProtectionsIntact(home string, g *Global) (bool, error)
```

Schema reference header: `# yaml-language-server: $schema=https://github.com/CGuiho/jureuma/releases/download/v<version>/jureuma.schema.json` (global: `jureuma.global.schema.json`).

## `internal/paths`

```go
func Home() (string, error)            // ~/.guiho/jureuma via os.UserHomeDir + filepath.Join
func BinDir() (string, error)          // ~/.guiho/bin
func TempDir() (string, error)         // ~/.guiho/.temp
func ProjectRoot(start string) (string, error) // nearest dir with jureuma.yaml, else .git, else start
func AtomicWrite(path string, data []byte, perm os.FileMode) error   // temp in same dir + rename
func Lock(home, key string) (unlock func(), err error)               // locks/<sha256(key)[:16]>.lock, PID+token, stale recovery
```

## `internal/pair`

Resolves and operates on a pair. Depends on doc, jsonc, crypt, keystore, schema, config, paths.

```go
type Options struct { Root, Name, EncryptedFile, UnencryptedFile, SchemaFile string; Config *config.Project }
type Pair struct { Name, EncryptedPath, UnencryptedPath, SchemaPath string; UnencryptedIsJSONC bool }
func Resolve(o Options) (*Pair, error)            // naming rules, .encrypted./.unencrypted. enforcement, schema precedence
type Encrypted struct { Meta Meta; Body *doc.Value }   // Body excludes $jureuma
type Meta struct { Format int; PublicKey, KeyID, Schema string; Sync map[string]SyncEntry; Pending []string }
type SyncEntry struct { C, H string }
func ReadEncrypted(path string) (*Encrypted, error)
func WriteEncrypted(path string, e *Encrypted) error   // $jureuma first, sorted sync/pending, atomic
func ReadPlain(path string, jsonc bool) (*doc.Value, error)
func WritePlain(path string, v *doc.Value) error        // refuses unless git-ignored (EnsureIgnored)
func EnsureIgnored(root, path string) error              // .gitignore managed block + git check-ignore
func TrackedByGit(root, path string) (bool, error)
func EncryptDoc(pk crypt.PublicKey, plain *doc.Value, prev *Encrypted, sk *crypt.PrivateKey) (*doc.Value, error) // reuses unchanged ciphertexts when sk given
func DecryptDoc(sk crypt.PrivateKey, body *doc.Value) (*doc.Value, error)
func Status(p *Pair, e *Encrypted, plain *doc.Value, sk *crypt.PrivateKey) Report
type Report struct { Overall string; Paths []PathState }
type PathState struct { Pointer, State string }
```

## `internal/runner`

```go
type Options struct {
    Doc *doc.Value; Schema *schema.Schema; Format string; EnvName, Prefix string
    Env []string /* NAME=/pointer */; Override bool; Redact bool; PrivateKeys []crypt.PrivateKey
    Args []string; Stdin io.Reader; Stdout, Stderr io.Writer; Home string
}
// Run builds the environment (strip JUREUMA_PRIVATE_KEY*, inject per Format), starts the
// child, redacts output, and returns the child's exit code. juju-side failures return
// (125, err).
func Run(ctx context.Context, o Options) (int, error)
func Flatten(v *doc.Value, prefix string) (map[string]string, error)   // rules + denylist + collisions
```

## `internal/xerr` and `internal/update` (added during integration)

- `xerr.Error{Code, Kind, Msg, Err}` carries the exit code (`CodeUsage` 2, `CodeProtected` 3, `CodeValidation` 4, `CodeKey` 5, `CodeRun` 125) and the kind printed by `run` as `[JUREUMA:<KIND>]`. `pair`, `runner` and `cli` return these; `xerr.CodeOf(err)` maps any error to its exit code.
- `update`: SemVer parsing/comparison, the GitHub release catalog (`JUREUMA_RELEASES_URL` overrides the API URL for tests), the `cache.json` update cache, the single-flight worker lease, and the platform reinstall commands.

`pair` additionally exports `NewEncrypted`, `MarshalEncrypted`, `EncryptAt`/`DecryptAt` (subtree at a pointer), `DecryptDocKeys` with a `KeyLookup` (retired keys), `ComputeSync`, `ConcreteTokens` (`-` → index), `EnsureGitignoreBlock`, `HasGitignoreBlock`, `IgnoredByGit`, `HeadPublicKey`, and `Pair.CheckSchemaFlag`. `runner` additionally exports `EnvName`, `NameProblems`, `Denied`, `FlattenExcept` (flatten skipping denylisted leaves that `--env` maps explicitly), `BuildEnv`, `Secrets`, `Instances` and `SweepStale` (the `run/<id>/owner` instance registry).

Added after the security and correctness review: `pair` exports `Within` (root containment after resolving symbolic links), `CheckSchemaName`, `AppendWithoutArray`, `EmptyContainersOnlyIn`, `MaxPointerDepth` and `Pair.DefaultSchema`; `config` exports `DeniedEnvName` and `DeniedRunEnvName` (the shared env-name denylist that `runner.Denied` delegates to); `paths` exports `HomeRelocated`, `TempMarker` and `IsStaleTemp`; `update` exports `Release.Tag`, `AgentAssets`, `PayloadAsset`, `RequiredAssets`, `Release.Compatible`, `Installable` (used by `Select`), `AssetURL` (`JUREUMA_DOWNLOAD_BASE_URL` overrides the base), `Download`, `ParseChecksums`, `Manifest`/`Artifact`/`ParseManifest`. `cli` implements the upgrade transaction (`upgrade_tx.go`) and the hidden `__self-test` command.

## `internal/agentres`

Embedded agent artifacts (`//go:embed`): the main skill `guiho-s-jureuma` (`SKILL.md`, including the mandatory `## CLI Evolution and Feedback` section and a JSON Schema authoring guide), the instruction block source, the install prompt `guiho-p-jureuma-install`, the main prompt `guiho-p-jureuma`, and the uninstall prompt `guiho-p-jureuma-uninstall`.

```go
func Skills() []Skill        // ID, Files map[string][]byte
func Prompts() []Prompt      // ID, Body
func Instruction() string    // block body, without markers
// AGENTS.md managed block, markers "<!-- >>> jureuma managed block >>> -->" / "<!-- <<< jureuma managed block <<< -->".
func ApplyBlock(agentsMD []byte, body string) []byte   // insert or replace; bytes outside untouched
func RemoveBlock(agentsMD []byte) ([]byte, bool)
func HasCurrentBlock(agentsMD []byte, body string) bool
// SkillDestinations returns the supported global agent-skill directories:
// ~/.claude/skills, ~/.agents/skills, ~/.codex/skills (only those whose parent tool dir exists, plus ~/.agents/skills always).
func SkillDestinations(userHome string) []string
func InstallSkill(s Skill, dest string) error          // dest/<id>/..., replaces atomically
func SkillInstalledCurrent(s Skill, dest string) bool
```

## `internal/cli`

Cobra command tree from the design plus the convention: `init`, `encrypt`, `decrypt`, `set`, `unset`, `get`, `run`, `list`, `status`, `check`, `key {show, export, rotate, retire}`, `schema {infer}`, `juju-enable`, `juju-disable`, `agent {skill {install, uninstall, upgrade, list, show}, instruction {apply, remove, upgrade, show}, prompt {list, show}}`, `upgrade {check, list}`, `uninstall`. Global: `-v/--version` (top-level, raw SemVer), and on every command `-h/--help`, `--help-docs`, `--help-tree`, `--help-tree-depth`, `--help-tree-global-flags`. `cmd/jureuma/main.go` calls `cli.Execute(version)` and exits with its code. `Version` is set by `-ldflags "-X main.version=…"`, default `0.0.0-dev`.

## Exit codes

`0` ok, `1` failure, `2` usage, `3` protected refused, `4` validation, `5` key/decryption. `run`: child code or `125`.
