---
name: 2026-10-17 DND Decisions
purpose: Record every question the unattended (DND) workers answer themselves during the key-identity-headers train (issue #13).
description: "DND question ledger for the key-identity-headers train: W1 key-list display format, label storage and relabeling, best-effort key-file metadata, picker behavior and label derivation."
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
  - "issue 13"
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
- **human-review:** pending

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
- **human-review:** pending

### Q3 — What happens when a key is saved again with a different label?

**Answer: the label is updated and `created` is preserved.** Silently ignoring
the new label would make `SaveLabeled` a lie; rewriting with a fresh timestamp
would lose history. Labels are non-secret metadata, so an update is cheap and
reversible. An identical key with an identical label leaves the file
untouched, as before.

- **confidence:** high
- **action taken:** implemented in W1 (labeled key storage, `key list`, `key show` and the TTY-safe picker).
- **human-review:** pending

### Q4 — `key list` needs labels, but `List` used to "not read contents". What is the contract now?

**Answer: `List` reads each file's non-secret metadata (label, public key)
best-effort and never decodes the private key.** Files that cannot be opened,
checked or parsed — or whose inner key ID contradicts the file name — still
list by name with placeholder metadata, so `key retire` state detection keeps
working. The decode target struct simply has no `privateKey` field, so key
material cannot materialise during listing.

- **confidence:** high
- **action taken:** implemented in W1 (labeled key storage, `key list`, `key show` and the TTY-safe picker).
- **human-review:** pending

### Q5 — What does `key list` print for an empty store?

**Answer: nothing, exit 0.** One line per stored key is the whole output
contract; an empty-state sentence would be indistinguishable from data for
parsers and for agents reading metadata. `key list` stays machine-readable.

- **confidence:** high
- **action taken:** implemented in W1 (labeled key storage, `key list`, `key show` and the TTY-safe picker).
- **human-review:** pending

### Q6 — What do `key show` and `key list` display when a key has no label?

**Answer: the placeholder `—`.** Covers legacy key files created before
labels existed and env-only keys (`JUREUMA_PRIVATE_KEY*`) that have no file at
all. The placeholder is display-only.

- **confidence:** high
- **action taken:** implemented in W1 (labeled key storage, `key list`, `key show` and the TTY-safe picker).
- **human-review:** pending

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
- **human-review:** pending

### Q8 — Where does the default label come from, and what if git is absent?

**Answer: the `origin` remote's repository name (`.git` suffix stripped,
https/ssh/scp/path remotes all supported), falling back to the project
directory name; an empty result means no label.** The git lookup reads local
configuration only (`git remote get-url origin`), never the network, and never
fails hard: no git, no remote, or any error falls through to the directory
name. The lookup is injectable for tests.

- **confidence:** high
- **action taken:** implemented in W1 (labeled key storage, `key list`, `key show` and the TTY-safe picker).
- **human-review:** pending

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
- **human-review:** pending

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
- **human-review:** pending
