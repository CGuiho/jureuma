#### &copy; 2026 [GUIHO](https://guiho.co) as represented by [Cristóvão GUIHO](https://guiho.co/cguiho) All Rights Reserved.

# Handoff — W3: single-file flows, `init` repair and `init` key selection (issue #13, parts 3–4)

Mode: dnd · Role: workhorse (engineer) · Harness: OpenCode native background
subagent (`general`) · Model: `xiaomi/mimo-v2.6-pro` (provider-default
thinking, no variant) · Issue: <https://github.com/CGuiho/jureuma/issues/13>

**Dispatch gate:** after W1 (labels + picker) and W2 (headers) have landed and
the parent confirms, and only after the JSONC write-support unit (issue #12)
has landed — this unit edits `internal/cli/cmd_init.go`,
`internal/cli/cmd_values.go`, `internal/cli/cmd_schema.go`,
`internal/schema/**` (API only if needed), `test/e2e/lifecycle_test.go`, the
design document and `CHANGELOG.md`, all shared with it. Record the base commit
when you start.

## Mission

Make the two single-file states first-class and wire the header and key work
into the commands. CG: "This is actually designed to only work with one of the
two files present: either we have only the unencrypted file or we have only
the encrypted file ... The agent can just open the file ... if that's not the
case, we need to run the `init` command again, so we will choose ... a key
that is already stored."

Additional directive (2026-10-17): the schema pointer and the encrypted copy
must survive every flow — `init` writes and repairs the `$schema` pointer,
`encrypt` and sibling builds embed the encrypted schema copy, `decrypt`
restores a missing local schema from it (never clobbering an existing file),
`schema infer` refreshes both, `set`/`unset`/mirror treat `$schema` as
reserved metadata, and `check`/`status` report schema presence, path match and
drift.

## Current state (verify after W1 + W2 land)

- `internal/cli/cmd_values.go` — `set`/`unset` write the encrypted file and
  the `mirrorPlain` plaintext mirror; `decrypt` (~250) reads encrypted, writes
  plaintext and the stash; `encrypt` writes encrypted and removes the
  plaintext unless `--keep-both`.
- `internal/cli/cmd_init.go` — pair creation (incl. key creation), starter
  schema, deny-rules blocks; no header repair today.
- `internal/cli/cmd_info.go` — `status`/`check`; `internal/cli/stash.go` —
  decrypt stash.
- After W2: format-2 headers on both sides; legacy files refuse mutations with
  the `run jureuma init to repair` pointer.
- After W1: `internal/cli/keysel.go` picker + short-id helpers; keystore
  labels and labeled saves.
- The schema path resolves today by convention (`<stem>.schema.json(c)`
  adjacent); `schema infer` writes canonical schemas and does not yet include
  the optional `$schema` property auth-ui's refined schema already declares.

## Implementation requirements

1. **Only-encrypted flows** stay as today: `set`, `unset`, `run`, `list`,
   `get` keep working when only the encrypted file is present.
2. **Only-unencrypted flows become first-class.** `set`/`unset` build or
   refresh the encrypted sibling from the plaintext header's `publicKey` —
   **no private key required**; reuse the existing crypto write path. Keep
   `mirrorBlocked` semantics exactly (an unsaved human edit still blocks the
   mirror).
3. **Transitions.** `encrypt`/`decrypt` keep working from every state;
   `decrypt` writes header+body into the plaintext (through the JSONC lossless
   engine for `.jsonc`); stash behavior stays (its recorded identity must
   survive); `--keep-both` remains the explicit opt-in; when both files are
   present, `status`/`check` warn that both exist (the riskiest state) while
   staying consistent.
4. **`init` repair.** Detect legacy/headerless files and add or repair the
   format-2 identity header plus the `$schema` pointer
   (`./<stem>.schema.json` or `.jsonc`); the encrypted side keeps its
   sync/pending state and gains the embedded schema copy when the local schema
   is present. Reads keep working before repair; mutations keep pointing at
   `init`.
5. **`init` key selection.** When no encrypted file exists, offer stored keys
   through the W1 picker (TTY only): label + short keyId, plus a "create a new
   key" choice. Non-TTY default stays create-new unless `--key-id` or
   `--key-label` is given. New keys get the W1 default label (repository
   name).
6. **Reporting.** `status`/`check` cover all four states (only-unencrypted,
   only-encrypted, both, legacy) with value-free output; never claim in-sync
   with a missing `h` (keyless rebuild baseline).
7. **Tests.** Extend `test/e2e/lifecycle_test.go` (already flipped by #12) for
   the four states across `set`/`unset`/`encrypt`/`decrypt`/`list`/`status`/
   `check`; keyless `set` from only-unencrypted; `--key-id`/`--key-label`
   non-TTY selection; `$schema` never listed or injected; pointer repair;
   encrypted-schema embed/restore round-trip and no-clobber on decrypt;
   `schema infer` output includes the optional `$schema` property; keep the
   full existing coverage green.
8. **Docs.** Design document sections (File Lifecycle, Pairs and schema,
   `init` domain steps, Command Tree, pair-identity description, `$schema`
   metadata and the embedded encrypted schema); `CHANGELOG.md` entries under
   the existing `## Unreleased` heading for this train (append-only; never
   rewrite other entries); help texts for changed commands.
9. **Schema lifecycle (2026-10-17 directive).** `encrypt` and sibling builds
   embed the schema copy (`$jureuma.schema = { path, c }`, via W2's API);
   `decrypt` restores the schema to its recorded path when missing — never
   clobbering an existing file, warning when it differs; `schema infer`
   includes the optional `$schema` string property in generated schemas and
   refreshes the embedded copy; `check`/`status` report pointer presence, path
   match and embedded-vs-local drift as warnings; `set` validates against the
   readable schema when present and warns explicitly when validation is
   skipped; reserved keys (`$schema`, `$jureuma`) are refused as value paths
   in `set`/`unset`.

## Do not touch

`internal/keystore/**` beyond W1's surface, `internal/pair/**` beyond W2's
surface, `internal/jsonc/**`, `README.md`, `internal/agentres/**` (a later
docs pass owns them), `todo.md`, `docs/plans/**`, `docs/questions/**` (except
the key-identity-headers ledger when an undecided matter forces it), the bound
GitHub Project, release machinery, version pins, other projects.

## Validation (capture exact commands and exit codes)

- `runx run go-fmt-check`, `runx run go-vet-cross`, `runx run go-test`,
  `runx run test-e2e`, `runx run test-smoke` — all exit 0; then
  `runx run docs-check` and `runx run ci-check`.
- Manual proof in a throwaway `JUREUMA_USER_HOME` (never the real home):
  walk a pair through only-unencrypted → `set` rebuilds the sibling → `decrypt`
  → only-unencrypted again; `init` repairs a hand-made legacy headerless pair.

## Deliverable and commit

- Two or three coherent commits, e.g. `Operate on single-file pairs from set
  and unset`, `Repair pair headers from init and select stored keys`, and any
  docs/tests follow-up. Imperative messages.
- Commit through the serialized index:
  `flock -w 120 /tmp/opencode/jureuma-git.lock -c 'cd /root/swe/jureuma && git add <paths> && git commit -m "..."'`.
  Stage only files you own; never `git add -A`; never push.
- Report: base commit, commit hashes, changed files, exact commands + exit
  codes, test names, deviations from the brief and why, residual uncertainty.
  Do not claim results you did not capture.

## Boundaries

- Never leak plaintext, ciphertext values or key material into logs, tests,
  fixtures or error messages; dummy fixture values are disclosed verbatim.
  Keep every human-only gate intact (`decrypt`, `get`, `key show --private`,
  `key export`).
- DND: never stop to ask a human. Undecided matters: safest reversible choice,
  recorded under `docs/questions/key-identity-headers/`.
