---
name: 2026-10-17 DND Decisions
purpose: Record every question the unattended (DND) workers answer themselves during the key-identity-headers train (issue #13).
description: "DND question ledger for the key-identity-headers train: W1 key-list display format, label storage and relabeling, best-effort key-file metadata, picker behavior and label derivation, the issue #10 env-name-restore DND calls, and the W4 project key-binding calls (issue #14)."
created: 2026-10-17
owner: jureuma-docs-questions-key-identity-headers
flags: []
tags:
  - "jureuma"
  - "questions"
  - "dnd"
keywords:
  - "key identity"
  - "key labels"
  - "key list"
  - "key selection"
  - "key binding"
  - "jureuma.yaml"
  - "issue 13"
  - "issue 14"
  - "question ledger"
---

#### &copy; 2026 [GUIHO](https://guiho.co) as represented by [Cristóvão GUIHO](https://guiho.co/cguiho) All Rights Reserved.

# 2026-10-17 DND Decisions

Context: the key-identity-headers train (issue #13) runs unattended (DND):
never stop, never ask CG, answer every question with the safest reversible
choice and record it here. Entries are grouped by unit.

## W1 — key labels, listing and selection

### Q1 — How short are the "short key ID" and the public-key prefix in listings?

**Answer: 12 characters each.** `key list` shows `k_` plus the first 12 hex
characters of the key ID and `jureuma-pk-1:` plus the first 12 base64url
characters of the public key. CG asked for "the beginning of the public key or
the key ID"; 12 characters are unique in practice for one machine's key set
and stay readable in one line. Display-only, so widening or narrowing later
touches no stored data.

- **confidence:** high
- **action taken:** implemented in W1 (labeled key storage, `key list`, `key show` and the TTY-safe picker).
- **human-review:** done

### Q2 — `key list` shows short key IDs only. How does a human get the full key ID?

**Answer: `key show` and the file name carry it.** The brief mandates the line
`label  k_shortid  jureuma-pk-1:prefix  active|retired`, so `export <keyId>`
and `retire <keyId>` cannot be fed straight from `list`. The full key ID stays
available from `jureuma key show`, from `check`/`status` output, and from the
key file name `~/.guiho/jureuma/keys/<keyId>.json`. Safest reading of the
mandated format; if CG wants copy-pasteable IDs in the listing, adding a
long-format option is additive.

- **confidence:** high
- **action taken:** implemented in W1 (labeled key storage, `key list`, `key show` and the TTY-safe picker).
- **human-review:** done

### Q3 — What happens when a key is saved again with a different label?

**Answer: the label is updated and `created` is preserved.** Silently ignoring
the new label would make `SaveLabeled` a lie; rewriting with a fresh timestamp
would lose history. Labels are non-secret metadata, so an update is cheap and
reversible. An identical key with an identical label leaves the file
untouched, as before.

- **confidence:** high
- **action taken:** implemented in W1 (labeled key storage, `key list`, `key show` and the TTY-safe picker).
- **human-review:** done

### Q4 — `key list` needs labels, but `List` used to "not read contents". What is the contract now?

**Answer: `List` reads each file's non-secret metadata (label, public key)
best-effort and never decodes the private key.** Files that cannot be opened,
checked or parsed — or whose inner key ID contradicts the file name — still
list by name with placeholder metadata, so `key retire` state detection keeps
working. The decode target struct simply has no `privateKey` field, so key
material cannot materialise during listing.

- **confidence:** high
- **action taken:** implemented in W1 (labeled key storage, `key list`, `key show` and the TTY-safe picker).
- **human-review:** done

### Q5 — What does `key list` print for an empty store?

**Answer: nothing, exit 0.** One line per stored key is the whole output
contract; an empty-state sentence would be indistinguishable from data for
parsers and for agents reading metadata. `key list` stays machine-readable.

- **confidence:** high
- **action taken:** implemented in W1 (labeled key storage, `key list`, `key show` and the TTY-safe picker).
- **human-review:** done

### Q6 — What do `key show` and `key list` display when a key has no label?

**Answer: the placeholder `—`.** Covers legacy key files created before
labels existed and env-only keys (`JUREUMA_PRIVATE_KEY*`) that have no file at
all. The placeholder is display-only.

- **confidence:** high
- **action taken:** implemented in W1 (labeled key storage, `key list`, `key show` and the TTY-safe picker).
- **human-review:** done

### Q7 — How does the stored-key picker decide it may prompt, and what may it read?

**Answer: character-device stdin means a TTY (`os.Stdin.Stat()`,
`ModeCharDevice`), no new dependency.** In a TTY it prints the `key list`
lines and prompts `Which one do you want to use? `; the answer is a line
number, a full key ID or a short key ID; an empty line or EOF cancels
(`ErrNoKeySelected`). Outside a TTY it never reads the injected reader and
returns the typed `ErrNoInteractiveSelection`, so callers must take an
explicit `--key-id` / `--key-label` (W3 wires `init` to that rule).

- **confidence:** high
- **action taken:** implemented in W1 (labeled key storage, `key list`, `key show` and the TTY-safe picker).
- **human-review:** done

### Q8 — Where does the default label come from, and what if git is absent?

**Answer: the `origin` remote's repository name (`.git` suffix stripped,
https/ssh/scp/path remotes all supported), falling back to the project
directory name; an empty result means no label.** The git lookup reads local
configuration only (`git remote get-url origin`), never the network, and never
fails hard: no git, no remote, or any error falls through to the directory
name. The lookup is injectable for tests.

- **confidence:** high
- **action taken:** implemented in W1 (labeled key storage, `key list`, `key show` and the TTY-safe picker).
- **human-review:** done

## W2 — `$jureuma` identity headers on both pair files

### Q9 — Where does the legacy mutation refusal live, and how can `init` repair write over legacy files?

**Answer: in every pair mutating entry point (`WritePlain`, `WritePlainFile`,
`WriteEncrypted`), checked on both sides of the pair, with dedicated repair
writers for `init`.** A4b says the refusal fires "on mutating pair entry
points" and can be triggered "on either side", while W3's brief requires
`init` repair to upgrade legacy files (keyless). Both hold only if the
ordinary write paths are gated and one explicit repair path
(`RepairPlainFile`, `RepairEncrypted`) may replace format-1 or malformed
blocks. The repair path still refuses an unknown future format, so jureuma
never overwrites a file it cannot read.

### Q10 — What does the refusal key on when the file cannot be parsed at all?

**Answer: nothing — such content keeps today's overwrite behaviour.** The
trigger is a *visible* `$jureuma` block (format 1 or present-but-malformed);
content with no parseable document has no header jureuma can see, and the old
`.jsonc` path already refuses unparseable text through the lossless writer.
Conservative for A4b's exact wording ("a `$jureuma` block with `format: 1`,
or a present-but-malformed `$jureuma`").

### Q11 — What are the exit codes and the types of the refusals?

**Answer: exit 1 (`xerr.CodeFailure`) with sentinels `ErrLegacyHeader`,
`ErrUnsupportedFormat` and `ErrMissingPlainIdentity`; key mismatches stay
exit 5.** The legacy refusal is a file-state failure, not a usage error and
not a key error; the existing keyId↔publicKey rule keeps its exit 5 even when
the mismatch is first seen by a mutating entry point. Every legacy refusal
message carries the exact `run jureuma init to repair` pointer and no value.

### Q12 — Does the marshaled `format` follow the parsed generation?

**Answer: no — every write renders `FormatVersion` (2), and `Header.Format` /
`Meta.Format` only record the generation that was read.** That makes the
marshal deterministic, keeps "this jureuma writes format 2" true on every
path (including `MarshalEncrypted` callers such as the decrypt stash), and
means no repair or mirror can accidentally re-emit `format: 1`.

### Q13 — Where do `home` and the embedded schema copy live?

**Answer: `home` is an identity field of both `$jureuma` blocks
(`~/.guiho/jureuma/` as `HomeRef`), and the embedded copy is
`$jureuma.schema = { path, c }` with `c` an `internal/crypt` envelope bound
to `/$jureuma/schema/c`.** The plaintext keeps the schema pointer at the
top-level `$schema` key instead. `Meta.Schema` stays the readable path, the
plain-string `$jureuma.schema` stays readable in any generation and stays the
written form without a copy, and `Meta.SchemaCopy` is the additive field with
`SetSchemaCopy`/`OpenSchemaCopy` as its setter and opener.

### Q14 — How is cross-file consistency enforced?

**Answer: `CheckIdentity` — publicKey/keyId disagreement is exit 5 (the
existing mismatch rule), `home` and schema-path differences are warnings
only.** Matching against the *stored* key stays with `check` (W3), which
already owns keystore lookups.

- **confidence:** high (Q9–Q14 follow the brief's A4 block and the untouched
  suite; each choice is exercised by `internal/pair/header_test.go`)
- **action taken:** implemented in W2 (`internal/pair/header.go`, the pair
  write paths, `header_test.go`); Q9–Q12 change no behaviour the untouched
  suite pins.
- **human-review:** done

## W3 — single-file flows, `init` repair and `init` key selection

### Q15 — How does a keyless sibling rebuild record the sync baseline when `h`
needs the private key?

**Answer: it writes no baseline at all — `Meta.Sync` stays empty and nothing is
pending (touched paths only when a skipped mirror leaves them there).** The
plan's "keyless rebuild baseline" wanted `c` per leaf with `h` absent, but the
frozen W2 parser (`internal/pair`, `parseMeta`) requires every `sync` entry to
carry both tags and rejects the file otherwise, and `crypt.PlainTag` derives `h`
from the private key. Writing `c`-only entries would make the pair unreadable.
The observable guarantee holds unchanged: jureuma never claims `in-sync`
without an `h` tag — with the key, `status` reports `in-sync` only after
proving equality by decryption (pair `Status`'s existing fallback); without it,
the overall state is `key-unavailable`. A key holder refreshing the sibling
writes full `c`/`h` pairs.

### Q16 — Where does `init` get the pair identity when no encrypted file exists?

**Answer: from the unencrypted file's `$jureuma` header when it has one, else
from a chosen stored key.** CG's flow is that the agent opens the single file;
when the file records the key, that key *is* "a key that is already stored" and
re-keying would orphan the pair. The picker (`--key-id` / `--key-label` /
TTY picker / create-new) therefore runs only when nothing records an identity.
`--key-id`/`--key-label` against a plaintext-recorded identity is a usage error
(they would disagree), and against an existing pair they are ignored with a
note (`key rotate` re-keys).

### Q17 — What does `init` write into the sibling when the plaintext holds
values?

**Answer: the sibling is built from the unencrypted document (values
included), not left empty.** An empty sibling beside a plaintext with values
would make `run` inject an empty document — a silent outage on agent machines,
where `encrypt` is impossible (no key). Building from the document is exactly
the sibling build `set`/`unset` use, so `run` works right after `init`. When
the plaintext is missing or empty, `init` keeps today's empty-file behaviour.

### Q18 — How is the "create a new key" choice offered through the W1 picker?

**Answer: `pickKey` is called with a synthetic trailing entry labeled
`create a new key`, whose selection (empty key ID) creates a key instead of
using one.** `internal/cli/keysel.go` is W1's surface and outside this unit's
ownership, so the picker is used as it stands; the synthetic line renders with
an empty key-ID column, a cosmetic imperfection accepted to keep the picker
untouched. EOF/cancel stays a hard `no key selected` failure — it never falls
through to creating a key.

### Q19 — When may `set`'s mirror write into the plaintext in the
only-unencrypted flow?

**Answer: exactly when today's `mirrorBlocked` allows it — an unsaved human
edit, or a value jureuma cannot prove unchanged, is never replaced.** The
baseline is the last saved state (the decrypt saved copy, then git `HEAD`),
falling back to "nothing is saved" when neither exists; without the private
key any existing value at or under the target blocks, as before. A skipped
mirror warns, keeps the path pending and leaves the value in the sibling
(`encrypt`/`decrypt` reconcile it). The alternative — treating the plaintext as
its own baseline so the mirror always lands — would silently overwrite a
human's post-decrypt edit at the set path, which the design document forbids.

### Q20 — What does `init` repair touch, and what about files it cannot parse?

**Answer: only metadata, only when it differs.** The encrypted side is
rewritten (format 2 identity, `home`, recorded schema path, embedded copy
added when absent) keeping values, `sync` and `pending`; the unencrypted side
keeps its body exactly and gains/renews the identity header and `$schema`
pointer. An already-correct file is not rewritten at all, so `init` is
byte-idempotent and keyless for legacy upgrades. A `$jureuma` block jureuma
cannot parse at all (present-but-malformed) cannot be repaired through the
frozen pair API — `ReadPlainFile`/`ReadEncrypted` refuse it and there is no
lenient reader — so `init` reports it (`fix or remove the $jureuma block by
hand, then re-run jureuma init`) and leaves the file untouched. **Pair-API need
recorded for review:** an additive lenient repair reader (parse the document,
split the body from a malformed `$jureuma` without validating it) would let
`RepairPlainFile`/`RepairEncrypted` heal malformed blocks too; it needs an
`internal/pair` addition, which this unit must not make.

### Q21 — Why is the schema path recorded in `$jureuma.schema`, and how does a
schema-file rename work?

**Answer: the path is always recorded (the `{ path, c }` form with the copy,
the plain string without) so both files agree on the schema and the copy can
be restored to its recorded path; a rename is an explicit choice through
`jureuma.yaml` (`pairs[].schema`) or `schema infer`.** `pair.CheckIdentity`
compares the plaintext `$schema` pointer against `$jureuma.schema` directly, so
an empty recorded path would warn "different schemas" on every ordinary pair;
recording the resolved path in both keeps that check meaningful and
`decrypt`'s restore faithful (a `.schema.jsonc` copy returns to a
`.schema.jsonc` path). The known cost: deleting the recorded schema file and
creating a differently-named one does not follow automatically — `check` fails
naming the missing schema until `pairs[].schema` or `schema infer` records the
new path. `test/e2e/cli_test.go`'s `.schema.jsonc` scenario was flipped to
declare the file in `jureuma.yaml` for exactly this reason (its coverage —
comments, refinements, maximum — is unchanged).

### Q22 — `key rotate` must keep passing with embedded copies. What had to
change?

**Answer: `internal/cli/cmd_key.go` got an additive 8-line migration of the
embedded schema copy during rotation (and the copy's key now counts as key
usage for `retire`/rotation sharing) — a recorded scope deviation.** Without
it, `TestKeyRotateAndRetire`'s security invariant ("no ciphertext under the
old key remains after rotate") fails as soon as a copy is embedded, because
the copy is a ciphertext like every value. Weakening that assertion instead
would have hidden a real leak of the old key ID; leaving the suite red would
break the unit gate. `cmd_key.go` is otherwise untouched.

### Q23 — What severity do `check` and `status` give the new states?

**Answer: warnings for anything repairable or cosmetic, failures only where a
mutation or an invariant is actually broken.** Legacy format state fails
(every mutation refuses, exit 1, with the repair pointer); a missing plaintext
identity fails only when the plaintext is the single file (the flows that must
rebuild the sibling refuse) and warns beside an encrypted file (mutations work
there); missing `$schema` pointer, pointer mismatch, missing/differing schema
copy and "both files exist" are warnings. `status` reports `encrypted-missing`
as the overall state of the only-unencrypted pair and lists its paths as
`unencrypted-only` — it never claims `in-sync` there.

### Q24 — What metadata does `decrypt` write into the plaintext?

**Answer: the identity header (the one already in the file, else the pair
identity from the encrypted file) and the `$schema` pointer (the recorded one,
else the resolved form), so the single-file state stays workable.** The body
and, in `.jsonc`, every comment and formatting detail outside the written
spans are preserved; a missing local schema is restored from the embedded
copy, an existing one is never overwritten. `set`/`unset`/mirror keep whatever
they read byte-identically and never add metadata.

- **confidence:** high for Q15–Q20 and Q23–Q24 (each is pinned by the new
  `test/e2e/lifecycle_test.go` coverage and by the untouched suite staying
  green); medium for Q21 (a real usability trade-off, chosen for cross-file
  consistency) and Q22 (a scope deviation forced by a security invariant).
- **action taken:** implemented in W3 (`internal/cli/cmd_values.go`,
  `cmd_init.go`, `cmd_info.go`, `cmd_schema.go`, `context.go`, the
  `cmd_key.go` migration of Q22, `test/e2e/lifecycle_test.go` plus three
  flipped assertions in `lifecycle_test.go`, `values_test.go` and
  `cli_test.go`); the Q20 pair-API need is recorded here, not implemented.
- **human-review:** done

## Issue #10 follow-up — restore `JUREUMA_CONFIG` as the default run env name
(W3 of the run-envname train, 2026-10-17)

CG reversed the `JUREUMA_` experiment (2026-10-17): "Make the environment
variable name this again. JUREUMA_CONFIG. Now I understand what it is, and it
should be the name!" The following judgment calls were made unattended while
flipping every surface back.

### Q25 — Two xdocs descriptor keyword lists still name `JUREUMA_`. Are they
"surfaces that name the default"?

**Answer: yes — `jureuma.xdocs.md` and `internal/runner/runner.xdocs.md` carry
`JUREUMA_` as a search keyword next to the run/injection description, so both
were flipped to `JUREUMA_CONFIG`.** The keywords index the document variable
that `run` injects, which is the default name; leaving them would keep the old
default searchable while every documented surface says `JUREUMA_CONFIG`. The
keyword is metadata only, so the choice is trivially reversible.

- **confidence:** medium-high (a keyword could also be read as the
  `JUREUMA_*` denylist namespace, but the default-name reading matches the
  descriptors' run-focused context).
- **action taken:** both keyword lists updated to `JUREUMA_CONFIG`; no other
  descriptor changed (no file lists changed).
- **human-review:** done

### Q26 — How is the `## Unreleased` CHANGELOG entry corrected?

**Answer: the entry is reworded to the final behaviour (default
`JUREUMA_CONFIG`, `JUREUMA_` denied like any other `JUREUMA_*` name), not
deleted.** The brief says "correct it to the final behaviour", and the entry
sits in the same unreleased train that briefly carried the `JUREUMA_` default;
released sections are untouched.

- **confidence:** high (explicit brief instruction).
- **action taken:** the `### Changed` entry under `## Unreleased` now states
  the restored default and the denylist exception rule.
- **human-review:** done

### Q27 — How is the new refusal coverage asserted in `test/e2e`?

**Answer: `--env-name JUREUMA_` is asserted as exit `125` with the
`[JUREUMA:USAGE]` prefix (the runner's `runErr` kind), while a project
`run.envName: JUREUMA_` is asserted as a refused load matching
`/run/envName` in stderr — `config.fieldErr` never prints the rejected value,
so the assertion keys on the field path instead.** `--env-name JUREUMA_CONFIG`
and `run.envName: JUREUMA_CONFIG` are asserted accepted. Redaction,
private-key-strip and exit-code assertions are unchanged.

- **confidence:** high (both behaviours were reproduced by the manual proof
  before the assertions were written).
- **action taken:** added to `TestRunDefaultJSON` in `test/e2e/run_test.go`;
  the config/runner unit tables were flipped in place.
- **human-review:** done

### Q28 — Does `internal/keystore/keystore_test.go`'s `"JUREUMA_CONFIG": false`
case change with the default?

**Answer: no — it classifies private-key variable names (`IsPrivateKeyEnv`) and
stays correct either way.** `JUREUMA_CONFIG` is not a private-key name in any
world; the run-env-name default is a different concern. The case was left
untouched on purpose.

- **confidence:** high.
- **action taken:** none for that test.
- **human-review:** done

## W4 — project key binding in `jureuma.yaml` (issue #14)

CG's directive (2026-10-17): "a project will use a key pair, and the key pair
that the project will use will be specified by the jureuma.yaml of the
project." The W4 brief (`w4-project-key-binding.md`) is the contract; the
judgment calls below were made unattended.

### Q29 — `init` binds the key it creates or chooses. What happens to the
multi-pair behaviour the W3 suite pins ("one key per new pair")?

**Answer: one project, one key. When `init` creates or chooses a key for a
pair with no recorded identity and no other configured pair records a
different key, that key is written as the `key:` binding and every pair
processed after it uses the bound key — so a fresh two-pair project now ends
with one key, not two.** The brief's mission sentence is explicit ("One
project, one key, every configured pair") and bullet 1 applies to a binding
that exists by the time the next pair is processed ("when `jureuma.yaml` names
a key … a pair with no recorded identity … uses the bound key"). The W3
assertion `TestNamedPairsAndFileFlags: "want one key per new pair"` was
flipped to "want one project key" accordingly, with the shared-key and
binding assertions added; the other pinned selection behaviour
(`TestInitKeySelection`: a pair with no recorded identity gets its own new key
when the project records other keys) is unchanged and stays green.

- **confidence:** medium-high (the mission sentence and the brief's bullet 1
  support it; the alternative — keeping per-pair keys and never binding —
  leaves CG's directive unrealised for multi-pair projects).
- **action taken:** `TestNamedPairsAndFileFlags` flipped in
  `test/e2e/lifecycle_test.go`; every other pinned assertion kept.
- **human-review:** done

### Q30 — `init` must not write a binding other pairs would immediately
contradict. Add it anyway (brief: "absent → add it"), refuse (exit 5), or
skip?

**Answer: skip with a note. When other configured pairs already record
different keys, `init` adds no binding and says so (`jureuma.yaml got no key
binding: pair X records … while pair Y uses …`); the pairs keep working
unbound.** Writing the binding would violate the brief's own agreement rule
("When `key` is set, every pair's recorded identity must match it") the
moment it exists — every later `run`/`set` on those pairs would exit 5 — and
refusing would break `init` for the ordinary mixed project (the pinned
`TestInitKeySelection` expects a successful run that gives the fresh pair its
own key). The exit-5 clause of the brief ("different → never overwrite") is
kept for the binding itself: a binding that differs from the key just created
or chosen is never overwritten and fails with exit 5.

- **confidence:** high on "never write a contradiction"; medium on skip vs
  refuse (skip keeps every pinned scenario working and loses nothing
  reversible — the binding can be added later).
- **action taken:** `ensureKeyBinding` in `internal/cli/cmd_init.go` with the
  conflict scan and the note; `TestEnsureKeyBindingNeverOverwrites` pins the
  exit-5 case.
- **human-review:** done

### Q31 — How is the "byte-preserving one-line splice" implemented?

**Answer: the top-level `key` entry is located through the parsed YAML node
(line/column), and only the value span of that one line is replaced — the
spacing before it and any trailing comment are preserved byte for byte; with
no key entry one `key: <id>` line is appended on its own line (matching the
file's line-ending style). The file must validate before the write, the
spliced result is re-parsed and re-validated before anything is written, and
the written file is re-read and validated again; an already-equal binding
never rewrites the file at all.** Locating the line through the parser (never
a blind `^key:` regex) keeps a line that merely looks like `key:` inside a
multi-line scalar untouched. A binding that cannot be represented as one line
— a quoted or flow-style key, a block or multi-line value, a duplicated
`key` entry, a document that is not a top-level mapping — is refused with
nothing written (the brief's "if the update cannot be represented" case).
Preserving the trailing comment is the safest reading of "keep every comment
byte-identical", and matches this repository's comment-preservation rule.

- **confidence:** high (every refusal path is covered by
  `internal/config/keybind_test.go`).
- **action taken:** `config.BindProjectKey` in `internal/config/keybind.go`,
  used by `init` and `key rotate`; `config.SaveProject` is never used for the
  binding (it rewrites the whole document) but now round-trips the field so a
  later whole-document save cannot drop it.
- **human-review:** done

### Q32 — The brief's proof (b) says a binding disagreement "fails `run`/`set`
with exit 5". `run` maps every jureuma-side failure to 125. Which wins?

**Answer: `set` (and every other command) exits 5; `run` exits 125 with
`[JUREUMA:KEY]` and the same message — the underlying error is the exit-5 key
error.** `run`'s contract ("run returns the child's code, or `125` for a
jureuma-side failure", `[JUREUMA:<KIND>]` prefix) is pinned by the design
document, the command help and the suite; special-casing one key error inside
`run` would contradict it. The manual proof records both numbers.

- **confidence:** high (the run contract is explicit and test-pinned).
- **action taken:** none in code; both exit codes captured in the manual
  proof and asserted (`run` 125 `[JUREUMA:KEY]`, `set` 5) in
  `internal/cli/projectkey_test.go` and `test/e2e/projectkey_test.go`.
- **human-review:** done

### Q33 — `key rotate` updates the binding even when other pairs still use the
old key. Is that not a fresh disagreement? *(superseded in the fix wave)*

**Answer (fix wave, 2026-10-17): it was — and the parent review rejected it
as a dead end. Rotation is now project-wide: with a binding, `key rotate`
rotates EVERY configured pair that records the bound key to ONE new key, so
no pair is ever left recording a key the binding no longer names.** One
generated key, every pair's values re-encrypted (`sync`/`pending` rebuilt per
pair, each embedded schema copy migrated), every recorded identity followed
to the new key, one binding update through `config.BindProjectKey` after the
pair writes, and one retirement of the old key. The original entry's
behaviour (single-pair rotation updating the binding while other bound pairs
kept the old key) is gone: it left those pairs at exit 5 — including for
`key rotate` itself — so the project could not be repaired from the CLI.

- **confidence:** high (the parent review named the old behaviour a blocking
  dead end; the project-wide rotation is pinned by
  `TestProjectKeyBindingRotateRotatesWholeProject`).
- **action taken:** `rotateProject` in `internal/cli/cmd_key.go` (bound
  projects with configured pairs); the single-pair path is untouched.
- **human-review:** done

### Q34 — Flag resolution order when `--key-id` both names no stored key and
contradicts the binding: exit 2 (usage) or exit 5 (key)?

**Answer: a full key ID that contradicts the binding is exit 2 (it is a usage
error detectable without the store); anything else is resolved first — an
unknown key stays exit 5 ("no stored key …"), an ambiguous label stays exit 2
("--key-id" hint) — and a resolved key that contradicts the binding is exit
2.** This keeps the pinned `TestInitKeySelection` outcomes (unknown
`--key-id` → 5, ambiguous label → 2) while satisfying "flags that contradict
the binding are a usage error".

- **confidence:** high (both pinned outcomes preserved).
- **action taken:** `checkInitKeyFlags` in `internal/cli/cmd_init.go`;
  `TestProjectKeyBindingFlagContradiction`.
- **human-review:** done

### Q35 — `status` on a plaintext-only headerless pair exits 5 with
"invalid key ID". *(fixed in the fix wave)*

**Answer (fix wave, 2026-10-17): fixed — with no recorded key ID both
read-only reports skip the keystore lookup entirely. `status` reports the
state (missing-identity note) and exits 0; `check` keeps its exit-1
repair-pointer failure and never raises a key error from an empty lookup.**
The original entry recorded it as pre-existing and out of scope; the parent
review then assigned the fix.
Verified at base commit `4027953` with the same scenario (no binding in the
project at all): `internal/cli/cmd_info.go` calls `findKey("")` when neither
file records an identity, and the keystore rejects the empty key ID. The
message is not a binding disagreement ("… does not match the key bound …"),
and `cmd_info.go` is not in this unit's ownership (`internal/cli` is scoped
to `context.go`, `cmd_init.go`, `cmd_key.go` and tests). A binding-proof
scenario avoids the state (`set` on a headerless plaintext alone returns the
single-file `run jureuma init to repair` pointer instead) and the
recommendation for a later unit is a `keyID != ""` guard before `findKey`.

- **confidence:** high (reproduced on the base binary and diff-verified
  unchanged code paths; the fix is pinned by
  `TestStatusAndCheckWithoutRecordedKey`).
- **action taken:** `internal/cli/cmd_info.go` (status and `checkPair`) now
  call `findKey` only with a non-empty key ID; both call sites keep every
  other report unchanged.
- **human-review:** done

### Q36 — "Empty or malformed values are validation errors (exit 4 on load)".
Only the `key:` field, or every configuration validation error?

**Answer: every schema and semantic validation error of a configuration
document now carries exit 4 (a typed `xerr` validation error), while files
that cannot be read, are too large, or do not parse as YAML keep failing as
general failures (exit 1).** The brief classifies the malformed binding as "a
validation error (exit 4 on load)" and jureuma's exit taxonomy maps
`4` to "validation failed"; giving only the `key:` field that code while a
malformed `run.format` or pair name kept exiting 1 would make the taxonomy
arbitrary. Blast radius checked against the untouched suite: no test pinned
exit 1 for a configuration-load validation error (the pinned exit-1 cases are
pair/git-state refusals, untouched), and `run` keeps mapping every
jureuma-side failure to 125.

- **confidence:** medium-high (consistent with the taxonomy and the brief;
  the alternative — key-only classification — was rejected as inconsistent).
- **action taken:** `fieldErr` and `validateSchema` return typed exit-4
  errors in `internal/config`; `loadEnv` and `init` preserve the carried code
  instead of forcing exit 1. Pinned by `TestConfigErrorsAreValidationErrors`.
- **human-review:** done

## W4 fix wave — project-wide rotation and the empty key lookup (2026-10-17)

Two parent-review findings were fixed: the multi-pair rotation dead end
(Finding 1) and the empty key-ID lookup in read-only reports (Finding 2,
Q35). The judgment calls below were made unattended.

### Q37 — Which pairs does "rotate ALL configured pairs that belong to the
binding" cover, and what is the rotation set?

**Answer: every configured pair (plus the explicitly selected pair when it is
not configured — an explicit selection is never silently ignored) is
verified; the pairs that RECORD an identity — which the verification proves
to be the bound key — are rotated, and pairs with no recorded identity (no
files, or a headerless plaintext alone) are skipped with a note.** "Belongs
to the binding" is exactly "records an identity" here, because the same
pre-check refuses any recorded identity that differs from the binding
(exit 5, naming the pair and both key IDs) before anything is written. The
reading of the brief's "and no selected role" is "plays no part in the
rotation": empty pairs are skipped with a note whichever way they were
selected, because there is nothing of theirs to re-encrypt.

- **confidence:** medium-high (the skip clause is unambiguous; "selected
  role" is the one phrase that could mean the flag-selected pair is treated
  differently — resolved as above since rotating a pair with no files is
  impossible).
- **action taken:** `rotateProject` phase 1 (`internal/cli/cmd_key.go`);
  `TestProjectKeyBindingRotateSkipsIdentitylessPair` and
  `TestProjectKeyBindingRotateRefusesMixedKeys`.
- **human-review:** done

### Q38 — What happens to recorded identities that are not the encrypted file
(headered unencrypted siblings, plaintext-only pairs)?

**Answer: every recorded identity follows the new key. A headered
unencrypted file — the sibling beside an encrypted file, or a pair whose
plaintext is its only file — is re-keyed (identity fields only; the body,
comments and `$schema` pointer are kept), and a headerless plaintext is left
byte-identical.** Without this, rotation would immediately create the very
dead end the finding forbids: `pair.CheckIdentity` and the shared resolver
both compare the plaintext header with the pair key, so a stale header makes
`set`, `check`, `status` and `init` exit 5 on that pair (verified on the
pre-fix build in the both-present state). Rotation therefore means "every
record of this pair's identity says the new key".

- **confidence:** high (the alternative leaves bound projects unrepairable
  after one rotation; the behaviour is pinned by the sibling-header
  assertions in `TestProjectKeyBindingRotateRotatesWholeProject`).
- **action taken:** the identity refresh is part of project-wide rotation
  only; it uses the ordinary pair writer (`pair.WritePlainFile`), so `.jsonc`
  keeps its comments and a `.json` plaintext is normalised the same way
  `init` repair normalises it.
- **human-review:** done

### Q39 — "Rotation is all-or-nothing" — what is guaranteed, and what about
the single-pair path?

**Answer: all-or-nothing holds for everything a pair can refuse — unreadable
files, old jureuma state, a missing key and a mismatched recorded key all
fail in the preparation phase, before the first write (and before the new key
is even generated). A mid-write I/O failure stops at the failing pair and
reports how many pairs were written and that the binding was not updated;
perfect atomicity across several files is not achievable without a
transaction.** ~~The single-pair/no-binding path is untouched byte-for-byte,
per the brief; its stale-sibling residual is left for a follow-up.~~

**Update (third fix wave, 2026-10-17): the residual is FIXED — the parent
review overrode the byte-compatibility instruction ("correctness wins over
byte-compatibility of the broken case").** The reproduction was exact: an
unbound project with both pair files present ended `key rotate` with the
plaintext `$jureuma.keyId`/`publicKey` on the old key and the encrypted file
on the new one, after which every `set`, `check` and `status` exited 5 ("the
two files disagree on the pair key") with no way back from the CLI. The
single-pair path now refreshes the recorded identity of a headered
unencrypted sibling right after the encrypted file is written — exactly the
mechanism project-wide rotation uses (`pair.WritePlainFile`: body, `$schema`
pointer, comments and formatting kept, `.jsonc` spliced byte-for-byte, a
headerless plaintext left byte-identical) — preflights the sibling the way
project-wide rotation does (an unreadable or legacy pair refuses before any
write, before the new key is generated), and reports a failed sibling write
truthfully ("the encrypted side was rotated but the plaintext identity was
not"), never claiming success. Output keeps every existing line and adds one
truthful, value-free line when the sibling identity is re-keyed.

- **confidence:** high on the guarantee boundary and on the fix (the
  reproduction and its repair are pinned by
  `TestKeyRotateRefreshesPlainSiblingIdentity` and
  `TestKeyRotateKeepsHeaderlessSiblingHeaderless`).
- **action taken:** the preparation/write split in `rotateProject` and the
  sibling identity refresh with its preflight in the single-pair path
  (`internal/cli/cmd_key.go`).
- **human-review:** done

## W5 — init key picker defaults to "create a new key" (issue #15, 2026-10-18)

CG reported that pressing Enter in `jureuma init`'s stored-key picker
re-prompts instead of selecting the synthetic "create a new key" entry. The
judgment calls below were made unattended (Mode: dnd).

### Q40 — The brief mandates `## Unreleased` with one `### Fixed` bullet in
`CHANGELOG.md`, but GUIHO convention 0009 (and this file's own sections) call
fixes `### Patches` under a `## [<version>] - <date>` title. Which shape?

**Answer: the brief's exact shape — `## Unreleased` with one `### Fixed`
bullet — because it is an explicit, twice-stated parent instruction (the task
brief and the handoff) and the parent renames the section at the 0.3.1
release; the convention conflict is flagged here rather than silently
resolved.** Convention 0009 governs released-version sections and does not
define an `Unreleased` section at all, so the unreleased placement is out of
its scope; only the section name (`### Fixed` vs `### Patches`) conflicts. A
heading rename at release is trivially reversible, whereas silently deviating
from the explicit brief would surprise the parent's release step.

- **confidence:** high on following the brief (explicit instruction), medium
  on the parent keeping `### Fixed` at release.
- **action taken:** `CHANGELOG.md` `## Unreleased` / `### Fixed` section as
  mandated. Flag to CG at review: convention 0009 would name it
  `### Patches` (its bullets already start with "Fixed").
- **human-review:** pending

### Q41 — Underspecified picker edges: whitespace-only line, out-of-range
default index, and the `(default)` marker on a non-synthetic default.

**Answer: (a) a whitespace-only line is Enter — it selects the default (it
carries no selection and `matchKeyChoice` already trims before matching);
(b) a negative or out-of-range default index means "no default", so the
generic `pickKey` four-argument behavior stays byte-identical, including the
empty-line number hint and the `ErrNoKeySelected` cancel at EOF; (c) the
`(default)` marker is appended to whichever listing line is the picker
default — for the synthetic entry that yields exactly `create a new key
(default)`, and a real default line would keep its columns and gain the
marker, though no current caller passes a real default.** The real-entry
rendering is unchanged in every existing and new usage: the only caller with
a default (`init`) always passes the synthetic entry.

- **confidence:** high (each choice is the conservative reading; the required
  strings and the unchanged paths are pinned by
  `TestPickKeyDefaultEnterSelectsDefault`,
  `TestPickKeyDefaultSelectionsUnchanged`, `TestPickKeyDefaultEOFCancels`,
  `TestPickKeyDefaultNonTTYNeverReads` and
  `TestPickKeyNoDefaultEmptyLineReprompts`).
- **action taken:** `pickKeyWithDefault` / `renderKeyLinesWithDefault` in
  `internal/cli/keysel.go` (the four-argument `pickKey` delegates with -1).
- **human-review:** pending
