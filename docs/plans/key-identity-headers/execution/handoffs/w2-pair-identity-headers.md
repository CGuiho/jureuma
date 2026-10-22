#### &copy; 2026 [GUIHO](https://guiho.co) as represented by [Cristóvão GUIHO](https://guiho.co/cguiho) All Rights Reserved.

# Handoff — W2: `$zeze` identity header on both pair files (issue #13, part 2)

Mode: dnd · Role: workhorse (engineer) · Harness: OpenCode native background
subagent (`general`) · Model: `xiaomi/mimo-v2.6-pro` (provider-default
thinking, no variant) · Issue: <https://github.com/CGuiho/zeze/issues/13>

**Dispatch gate:** the JSONC write-support unit (issue #12) is editing
`internal/pair/files.go` and `docs/2026-10-13-zeze-implementation-contracts.md`
right now. Do not start until the parent confirms those commits have landed;
record the base commit when you start and build **on** the landed JSONC
lossless writer — header insert/repair on a `.jsonc` file goes through it,
never a rewrite.

## Mission

Put the pair identity on **both** files: a reserved `$zeze` header in the
unencrypted file alongside the one in the encrypted file, so whichever single
file a holder has, zeze knows the public key, the key reference (`keyId`)
and the global directory. Headerless/legacy files are detected and point at
`zeze init` for repair. CG: "We need to have a `$zeze` header for both
unencrypted and encrypted files ... on both ... we must specify the public key
and the key reference ... and the global directory."

Additional directive (2026-10-17): "On the unencrypted file, we should always,
always, always specify the path to the local schema. On the encrypted file, we
should also specify a property which will be an encrypted schema." → the
unencrypted document always carries the top-level `"$schema"` pointer (reserved
metadata: never a value, listing, injection or encrypted body), and the
encrypted header carries `$zeze.schema` as `{ path, c }` — the readable path
plus the schema document encrypted to the pair key.

## Current state (verify after the JSONC landing)

- `internal/pair/files.go` — `ReadPlain(path, jsonc)`, `WritePlain`,
  `WriteEncrypted`; the encrypted `$zeze` header (format 1) carries
  `format`/`publicKey`/`keyId` plus sync/pending state; the plaintext file has
  no header and a top-level `$zeze` key is currently reserved/rejected.
- `internal/pair/pair.go` — pair resolution (extensions, schema path).
- `internal/jsonc` — the landed lossless edit engine (`Edit`) and lenient
  parser.
- Design `docs/2026-10-13-zeze-design.md` (File Model, Encrypted file
  format, key rules); contracts
  `docs/2026-10-13-zeze-implementation-contracts.md` (`internal/pair`
  section); train decisions in `docs/plans/key-identity-headers/plan.md`.

## Required reading (before touching anything)

- `AGENTS.md` (Mandume section, JSONC format contract), the design document
  sections above, the contracts document.
- Skills: `guiho-s-0001-guiho`, `guiho-s-0004-working-with-cg`,
  `guiho-s-0040-explorer`, `guiho-s-0032-git-commit`, `guiho-s-0016-docs`,
  `guiho-s-runx`, `guiho-s-zeze`.
- `guiho-convention-0001-cli.md`, `guiho-convention-0007-models.md`.

## Implementation requirements

1. **Header model** (`internal/pair`, new `header.go`): a typed identity
   struct `{format, publicKey, keyId, home}` for both sides. `home` is the
   global directory reference in conventional portable form
   (`~/.guiho/zeze/`, as defined by `internal/paths`) — never a
   machine-specific absolute path. The encrypted header additionally carries
   `schema` as `{ path, c }` — the readable local schema path plus the schema
   document encrypted to the pair key as a single envelope (reuse the
   `internal/crypt` representation; the envelope plaintext is the schema
   document text). Parse and marshal for the plaintext header; keep
   encrypted-side parsing aligned with what #12 leaves.
2. **Format 2.** Both headers are written as `format: 2`. Reading: `format: 1`
   or a missing plaintext header is legacy — reads keep working; every
   mutating path refuses with a typed, value-free error whose message includes
   `run zeze init to repair`. Unknown future formats keep old zeze's
   explicit rejection behavior. A format-2 plaintext is expected to carry the
   `$schema` pointer; when it is missing, writes restore it and `check`
   reports it — never a hard refusal.
3. **Reserved plaintext keys.** Two keys are reserved metadata, excluded from
   the body everywhere the plaintext is read: the `$zeze` header and the
   top-level `$schema` pointer (`"$schema": "./config.schema.json"` or
   `.jsonc` — always present in format 2). `ReadPlain` returns the body
   without them and exposes both for callers that need them, so `list`, `get`,
   `run` injection, the `set` mirror and `schema infer` see body-only data
   automatically. `WritePlain` writes header + `$schema` + body, preserving
   all bytes outside changed spans. If any caller parses the plaintext
   directly instead of through `ReadPlain`, note it in your report for the
   parent — do not edit another unit's file.
4. **Consistency.** When both files exist: `publicKey` and `keyId` must match
   each other and the stored key where present — mismatch = exit 5 (the
   existing keyId↔publicKey rule); `home` and `schema.path` mismatches across
   files are warnings only, never fatal.
5. **Writing.** New plaintext starters and repairs always write the header
   and the `$schema` pointer; the encrypted side writes `schema` whenever the
   caller supplies the schema text (and preserves an existing embedded copy
   otherwise). `WritePlain` on `.jsonc` composes with the lossless engine
   (insert or edit is a span edit). Every byte outside the changed spans is
   preserved.
6. **Tests.** Parse/marshal round-trips (incl. the `schema` envelope
   decrypting back to the schema text); legacy detection (`format: 1` and
   headerless) and the exact repair pointer; mismatch = exit 5 on load;
   `home`/`schema.path` warnings; body exclusion of `$zeze` and `$schema`
   for read, mirror and injection paths; a `.jsonc` round-trip where header
   and pointer insert/repair leave comments and formatting intact.
7. **Docs.** Update the `internal/pair` section of the implementation
   contracts and `internal/pair/pair.xdocs.md` where metadata requires it.

## Do not touch

`internal/cli/**` (W3 owns command wiring), `internal/keystore/**`,
`internal/jsonc/**` (use its API only), `test/e2e/**`, `CHANGELOG.md`, the
design document, `README.md`, `internal/agentres/**`, `todo.md`,
`docs/plans/**`, `docs/questions/**` (except the key-identity-headers ledger
when an undecided matter forces it), the bound GitHub Project, release
machinery, version pins, other projects.

## Validation (capture exact commands and exit codes)

- `runx run go-fmt-check`, `runx run go-vet-cross`, `runx run go-test`,
  `runx run test-e2e`, `runx run test-smoke` — all exit 0; then
  `runx run docs-check` and `runx run ci-check`.
- Manual proof in a throwaway `ZEZE_USER_HOME` with a hand-made legacy pair
  (`.json` and `.jsonc` variants): reads work; a mutating call refuses with the
  repair pointer; writing through the pair API produces the format-2 header on
  both files; a `publicKey` mismatch exits 5. (Command-level wiring lands in
  W3; prove at the package level and say so.)

## Deliverable and commit

- One or two coherent commits, e.g.
  `Write the pair identity header on both files` and
  `Refuse mutations on legacy pair files with an init repair pointer`.
  Imperative messages.
- Commit through the serialized index:
  `flock -w 120 /tmp/opencode/zeze-git.lock -c 'cd /root/swe/zeze && git add <paths> && git commit -m "..."'`.
  Stage only files you own; never `git add -A`; never push.
- Report: base commit, commit hashes, changed files, exact commands + exit
  codes, test names, deviations from the brief and why, residual uncertainty.
  Do not claim results you did not capture.

## Boundaries

- Never leak plaintext, ciphertext values or key material into logs, tests,
  fixtures or error messages; errors report file, line, column and class only.
- DND: never stop to ask a human. Undecided matters: safest reversible choice,
  recorded under `docs/questions/key-identity-headers/`.

## Dispatch amendments (UltraCode packet v2, 2026-10-17)

Added by the Mandume primary at dispatch time; they refine the brief and never
contradict it.

- **A1 — API compatibility (required).** `pair.ReadPlain`/`WritePlain` have
  live callers in `internal/cli/**` (this unit must not edit them). Keep the
  existing signatures compiling: add additive variants (a full read returning
  header + `$schema` + body; a full write) and keep `ReadPlain`/`WritePlain`
  as thin wrappers; implement legacy mutation-refusal inside the pair write
  paths so untouched CLI callers inherit it; call-site wiring stays W3's. The
  full battery must compile and pass without editing `internal/cli/**` or
  `test/e2e/**`.
- **A2 — ledger normalization.** Append the DND-required `confidence`, `action
  taken` and `human-review: pending` fields to Q1–Q8 in
  `docs/questions/key-identity-headers/2026-10-17-dnd-decisions.md`
  (additive only; W1's answers stay as written). This ledger is the unit's one
  permitted `docs/questions/**` edit.
- **A3 — wording.** `ParseEncrypted`'s `this zeze reads format 1` message
  moves to the accepted model: format 2 is written; format 1 reads keep
  working with the repair mutation pointer on present-but-legacy headers
  (a headerless plaintext is never a refusal trigger — see A4b); unknown
  future formats keep the explicit rejection. The `MetaKey` comment becomes
  "reserved top-level key of both pair files".

### A4 — suite-locked compatibility (UltraCode packet v2.1, 2026-10-17)

Validated against the untouched test suite at `69c1bea`. This block **refines
and, where noted, supersedes** Implementation requirements 2, 3 and 5 of this
brief; the untouched suite is decisive wherever they disagree.

- **A4a — schema-shape compatibility.** `$zeze.schema` accepts a plain
  string (path-only) on read in **any** format generation — path safety
  (inside-project) checks are identical for both shapes. `Meta.Schema string`
  and `NewEncrypted(pk, schemaRef)` stay exactly as they are (untouchable
  callers at `cmd_init.go:408`, `cmd_key_list_test.go:192/211/229`); add an
  additive field + setter for the embedded copy. Format-2 writes use
  `{ path, c }` **when an embedded copy is present**; without one, the plain
  string remains the written form and stays legal to read (required:
  `review_test.go:111` hand-writes the string form into a format-2 file and
  expects exit 2 on an outside-project path). Field name `schema` and the
  `ConfiguredSchema` path-only precedence are unchanged.
- **A4b — refusal scoping.** Legacy refusal keys on **format state, never on
  plaintext header absence**: a `$zeze` block with `format: 1`, or a
  present-but-malformed `$zeze` on either side → typed, value-free refusal
  including `run zeze init to repair` on mutating pair entry points;
  `format` outside {1, 2} keeps today's explicit "unsupported format" read
  rejection. A **headerless plaintext is never a refusal trigger** (encrypt,
  mirror, both-present, keyless `set` included) — `main_test.go:310`,
  `onefile_test.go:13-31/33-64` and `cli_test.go:79-146` mutate through
  headerless plaintexts and must stay green, and W3 §7 keeps them green at
  train end. The missing-plaintext-identity repair pointer is defined here as
  a typed error but fires only in W3's single-file identity paths (building/
  refreshing the sibling from a headerless plaintext) and in `check`/`init`
  repair detection. Enforce inside the pair mutating entry points (A1), so
  untouched CLI callers inherit without edits.
- **A4c — old wrappers are byte-compatible, not merely non-injecting.**
  `ReadPlain`/`WritePlain` keep **today's exact behavior**: `ReadPlain` keeps
  rejecting a top-level `$zeze` ("is reserved") and keeps passing `$schema`
  through as an ordinary body key; `WritePlain` keeps rejecting body
  `$zeze` and writes exactly what it is given — no header/pointer
  injection (`lifecycle_test.go:167-176`) and no deletion of anything already
  in the document. The new header/`$schema`/embedded-schema model lives
  exclusively in the full APIs; W3 wires its call sites and flips
  expectations. **Do not strip reserved keys in the old readers and do not
  pass `$zeze` through as a body key**: a stripped read + body-only write
  silently drops a human's `$schema` on every `set` mirror (data loss on the
  exact files this train protects), and a passed-through `$zeze` would be
  fed into `EncryptDoc` and silently dropped by `MarshalEncrypted` (body
  `$zeze` is skipped at `files.go:205-208`). `.jsonc` writes compose
  through the lossless engine everywhere.
- **A4d — plan alignment.** The plan's "Format 2" decision bullet ("format: 1
  and headerless files are legacy: … mutations refuse") is superseded by A4b:
  refusal keys on format state; headerless plaintexts stay fully mutable; the
  repair pointer for missing plaintext identity belongs to the single-file
  identity paths, `check` and `init` repair. The primary amends
  `docs/plans/key-identity-headers/plan.md` accordingly (plans are
  worker-forbidden).
- **A4e — suite-shape invariants (marshal/parse contract).** Field spellings
  `format`, `publicKey`, `keyId`, `schema`, `sync` {`c`,`h`}, `pending` keep
  their names and `doc.Marshal` rendering (incl. the `"keyId": "` spacing
  `values_test.go:263` matches); `$zeze` stays the first top-level key with
  body keys in source order (`lifecycle_test.go:142-143`); `FormatVersion`
  becomes 2 while parse accepts 1 (legacy) and 2, and `NewEncrypted` writes 2;
  marshal is deterministic (second `init` diff-empty,
  `lifecycle_test.go:87-95`; keyless `init` leaves the encrypted file
  byte-identical, `:113-118` — constrains W3's repair to be a no-op on
  already-format-2 files and keyless for legacy upgrades); `set /$zeze/x`
  stays refused. W2-ownable `internal/pair/pair_test.go` expectations (e.g.
  `:218` "$zeze must be reserved") update with the model.
