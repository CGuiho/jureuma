#### &copy; 2026 [GUIHO](https://guiho.co) as represented by [Cristóvão GUIHO](https://guiho.co/cguiho) All Rights Reserved.

# Handoff — W1: comment-preserving JSONC writer (issue #12)

Mode: dnd · Role: workhorse (engineer) · Harness: OpenCode native background
subagent (`general`) · Model: `xiaomi/mimo-v2.6-pro` (provider-default
thinking, no variant) · Issue: <https://github.com/CGuiho/jureuma/issues/12>

## Mission

Make `.jsonc` a fully supported, two-way format for the human-owned files.
Today `jureuma` reads JSONC but never writes a `.jsonc` file (the design says
so — "no comment-preserving writer in v1"): `set`/`unset` silently skip the
plaintext mirror, `decrypt` refuses, and the human reconciles by hand. Build
the comment-preserving writer and wire it in, so editing a `.jsonc` file
preserves every comment, blank line and formatting byte outside the regions
the edit itself changes — and refuses rather than destroys them when that
cannot be done. `.json` stays the default and its behavior is unchanged; the
encrypted file stays `.json`. This unit is dispatched after the run env name +
OpenCode deny rules train lands; record the base commit when you start.

## Current state (verify before changing)

- `internal/jsonc/jsonc.go` — strict JSON / lenient JSONC parser
  (`Parse(data, strict bool)`), value-free errors; no lossless mode.
- `internal/pair/files.go` — `WritePlain` refuses `.jsonc` targets
  (~line 292); `ReadPlain(path, jsonc bool)` parses either extension;
  `WriteEncrypted` always `.json`.
- `internal/cli/cmd_values.go` — `mirrorPlain` (~line 665) returns early when
  `p.UnencryptedIsJSONC`; `decrypt` (~line 250) refuses `.jsonc`;
  `set`/`unset` help texts say "existing .json unencrypted file" (~lines 460
  and 693).
- `internal/cli/cmd_init.go` — (~line 397) steps "missing ... (jureuma never
  writes .jsonc files)" for a missing `.jsonc` plaintext; (~line 409) writes
  the starter schema with `doc.Marshal` (canonical JSON — valid JSONC) even
  when the resolved schema path ends in `.schema.jsonc`.
- `internal/cli/cmd_schema.go` — `schema infer` refuses an existing
  non-starter schema without `--force`, then writes `doc.Marshal(inferred)`
  unconditionally (which would destroy comments in a `.jsonc` schema under
  `--force`).
- Tests assert the old behavior: `test/e2e/lifecycle_test.go` (~line 211)
  "jureuma never writes .jsonc: decrypt refuses, set leaves it alone" — flip
  it to the new behavior.
- Design `docs/2026-10-13-jureuma-design.md` (~lines 54-63) — the "Supported
  Formats" table and its notes state the old contract.

## Required reading (before touching anything)

- Repository `AGENTS.md` — the "Supported Format: JSONC" section is the
  contract: lenient JSONC; trailing commas accepted when reading and never
  emitted when writing; comments must not affect the data; "when rewriting a
  file, preserve existing comments and formatting wherever possible".
- `docs/2026-10-13-jureuma-design.md` — Supported Formats, Pairs and schema,
  Schema resolution, "The `set` mirror never replaces an unsaved human edit",
  the encrypt/decrypt workflows; `docs/2026-10-13-jureuma-implementation-contracts.md`.
- Skills: `guiho-s-0001-guiho`, `guiho-s-0004-working-with-cg`,
  `guiho-s-0040-explorer`, `guiho-s-0032-git-commit`, `guiho-s-0016-docs`,
  `guiho-s-runx`, `guiho-s-jureuma`.
- `guiho-convention-0001-cli.md` (no flag changes; help text must comply).
- The jsonc.org draft specification (reference implementation: Microsoft
  `jsonc-parser`, whose `modify`/`applyEdits` compute minimal edits) — this
  feature is modeled on that behavior.

## Implementation requirements

1. **Lossless JSONC layer** (`internal/jsonc`, new file, e.g.
   `lossless.go`): parse the file text into a token tree recording the byte
   span of every key, value, comma and comment, including trailing commas;
   the original bytes stay the source of truth. Never decode and re-encode
   numbers (`1e3`, `-0`), string escapes, or any unchanged region.
2. **Edit engine** — a function like
   `func Edit(original []byte, old, new *doc.Value) ([]byte, error)` (exact
   shape up to you; it lives in `internal/jsonc`, which already depends on
   `doc`):
   - compute the change set by comparing `old` and `new` (replaced leaves,
     added leaves, removed leaves);
   - map each change to a minimal byte-span splice on `original`: a replaced
     value replaces exactly the old value span; an insertion adds the new
     member immediately before the closing brace/bracket, matching the
     container's sibling indentation and separator style, and reusing an
     existing trailing comma instead of emitting a new one; a removal
     deletes the key-to-value span plus one separator comma;
   - arrays: match common prefixes/suffixes element-wise (including the
     shifted-suffix case of an interior removal) so a single removed element
     only edits that element's own span;
   - every byte outside the edited spans is preserved exactly: comments,
     blank lines, indentation, key order, number text, string escape
     spelling, trailing commas already present;
   - never emit a new trailing comma; never delete a comment that lies
     outside a removed span;
   - if a change cannot be represented without touching bytes outside its
     spans, fail with a value-free error — never fall back to rewriting;
   - the returned text must parse to exactly `new`.
3. **Wire-in**:
   - `internal/pair/files.go` `WritePlain`: when the target is `.jsonc` — if
     the file exists, read it, parse it (lenient) and write
     `jsonc.Edit(original, current, v)`; when it does not exist, write
     `doc.Marshal(v)` (valid JSON is valid JSONC). Keep every existing guard:
     name check, gitignore, tracked-by-git, size limit, mode 0600,
     `clear(data)` of the written bytes.
   - `internal/cli/cmd_values.go`: remove the `p.UnencryptedIsJSONC` skip in
     `mirrorPlain` and the decrypt refusal; mirror and decrypt write `.jsonc`
     through `WritePlain`. Keep `mirrorBlocked` semantics exactly — an
     unsaved human edit still blocks the mirror.
   - `internal/cli/cmd_init.go`: a missing plaintext side is now created for
     `.jsonc` too (empty object, same as `.json`); the "never writes .jsonc"
     step message goes away. The starter schema write is unchanged.
   - `internal/cli/cmd_schema.go` `schema infer`: when overwriting an
     existing `.jsonc` schema (init starter or `--force`), merge the new
     schema onto the existing text with the same engine so comments on
     surviving keys survive. `--force` semantics stay otherwise unchanged.
   - `set`/`unset` help texts and any status output that says the mirror is
     skipped for `.jsonc` or tells the human to reconcile by hand — update
     to the new behavior; `check`/`status` must stay consistent and stop
     treating a `.jsonc` pair as unreconcilable.
4. **Defaults stay JSON**: no new flags; a user chooses JSONC by naming the
   file `.jsonc`. New pairs and the starter schema stay `.json`; the
   encrypted file is always `.json`; comments are never carried into it. Do
   not change `schema.Starter()` or the canonical `.json` write path.
5. **Docs**: update the design "Supported Formats" table and its notes; the
   root `AGENTS.md` "Supported Format: JSONC" section (add the write policy:
   full two-way support, comment preservation, refuse-rather-than-destroy,
   JSON default, encrypted always `.json`); `README.md` and the bundled
   skill/instruction/prompt wherever they state or imply the old rule
   (`grep -rin jsonc` across `README.md AGENTS.md docs/ internal/ test/`);
   CHANGELOG `## Unreleased` → an `### Added` (or `### Changed`) entry.
   Historical records (past CHANGELOG releases, reviews, `docs/todo/done/`,
   old plans) are never rewritten.
6. **Tests**:
   - `internal/jsonc`: table/golden tests for the engine — set a nested
     value, insert into an object with and without a trailing comma, remove
     members, interior array removal, unicode/escape spelling preserved,
     comments and blank lines preserved byte-for-byte, refusal on mismatch;
   - `internal/pair`: `WritePlain` on `.jsonc` — merge, create-missing,
     refusal cases (unparseable or invalid current file left untouched),
     guards unchanged;
   - `test/e2e`: flip `lifecycle_test.go` to assert the new behavior; add
     coverage for a `.jsonc` pair — `set` mirrors preserving comments,
     `decrypt` preserves comments of surviving keys, `check`/`status` stay
     consistent;
   - then the full battery.

## Do not touch

`todo.md`, `docs/plans/**`, `docs/questions/**` (except creating the ledger
under `docs/questions/jsonc-support/` when an undecided matter forces it),
the bound GitHub Project, historical records, version pins, release
machinery, the encrypted-file format, `.json` canonical output, global
OpenCode config, other projects.

## Validation (capture exact commands and exit codes)

- `runx run go-fmt-check`, `runx run go-vet-cross`, `runx run go-test`,
  `runx run test-e2e`, `runx run test-smoke` — all exit 0; then
  `runx run docs-check` and `runx run ci-check`.
- Manual proof in a throwaway temp project (`JUREUMA_USER_HOME` outside the
  repo): hand-write a `.jsonc` plaintext with comments and a trailing comma,
  `jureuma set` a nested value and show the comments survive; repeat with
  `decrypt`.

## Deliverable and commit

- Up to two coherent commits: (a) the lossless layer + engine + its unit
  tests, (b) wiring + integration tests + docs. Imperative messages, e.g.
  `Add a comment-preserving JSONC writer` and
  `Write .jsonc files from set, decrypt and schema writes`.
- Commit through the serialized index:
  `flock -w 120 /tmp/opencode/jureuma-git.lock -c 'cd /root/swe/jureuma && git add <paths> && git commit -m "..."'`.
  Never push.
- Report: base commit, commit hashes, changed files, exact commands + exit
  codes, test names, deviations from the brief and why, residual
  uncertainty. Do not claim results you did not capture.

## Boundaries

- Never leak plaintext or key material into logs, tests, fixtures or error
  messages; error messages report file, line, column and error class only.
- DND: never stop to ask a human. If something is genuinely undecided, take
  the safest reversible choice (preserve over destroy) and record it under
  `docs/questions/jsonc-support/`.
