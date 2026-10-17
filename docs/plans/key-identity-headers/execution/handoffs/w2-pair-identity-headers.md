#### &copy; 2026 [GUIHO](https://guiho.co) as represented by [Cristóvão GUIHO](https://guiho.co/cguiho) All Rights Reserved.

# Handoff — W2: `$jureuma` identity header on both pair files (issue #13, part 2)

Mode: dnd · Role: workhorse (engineer) · Harness: OpenCode native background
subagent (`general`) · Model: `xiaomi/mimo-v2.6-pro` (provider-default
thinking, no variant) · Issue: <https://github.com/CGuiho/jureuma/issues/13>

**Dispatch gate:** the JSONC write-support unit (issue #12) is editing
`internal/pair/files.go` and `docs/2026-10-13-jureuma-implementation-contracts.md`
right now. Do not start until the parent confirms those commits have landed;
record the base commit when you start and build **on** the landed JSONC
lossless writer — header insert/repair on a `.jsonc` file goes through it,
never a rewrite.

## Mission

Put the pair identity on **both** files: a reserved `$jureuma` header in the
unencrypted file alongside the one in the encrypted file, so whichever single
file a holder has, jureuma knows the public key, the key reference (`keyId`)
and the global directory. Headerless/legacy files are detected and point at
`jureuma init` for repair. CG: "We need to have a `$jureuma` header for both
unencrypted and encrypted files ... on both ... we must specify the public key
and the key reference ... and the global directory."

## Current state (verify after the JSONC landing)

- `internal/pair/files.go` — `ReadPlain(path, jsonc)`, `WritePlain`,
  `WriteEncrypted`; the encrypted `$jureuma` header (format 1) carries
  `format`/`publicKey`/`keyId` plus sync/pending state; the plaintext file has
  no header and a top-level `$jureuma` key is currently reserved/rejected.
- `internal/pair/pair.go` — pair resolution (extensions, schema path).
- `internal/jsonc` — the landed lossless edit engine (`Edit`) and lenient
  parser.
- Design `docs/2026-10-13-jureuma-design.md` (File Model, Encrypted file
  format, key rules); contracts
  `docs/2026-10-13-jureuma-implementation-contracts.md` (`internal/pair`
  section); train decisions in `docs/plans/key-identity-headers/plan.md`.

## Required reading (before touching anything)

- `AGENTS.md` (Mandume section, JSONC format contract), the design document
  sections above, the contracts document.
- Skills: `guiho-s-0001-guiho`, `guiho-s-0004-working-with-cg`,
  `guiho-s-0040-explorer`, `guiho-s-0032-git-commit`, `guiho-s-0016-docs`,
  `guiho-s-runx`, `guiho-s-jureuma`.
- `guiho-convention-0001-cli.md`, `guiho-convention-0007-models.md`.

## Implementation requirements

1. **Header model** (`internal/pair`, new `header.go`): a typed identity
   struct `{format, publicKey, keyId, home}` for both sides. `home` is the
   global directory reference in conventional portable form
   (`~/.guiho/jureuma/`, as defined by `internal/paths`) — never a
   machine-specific absolute path. Parse and marshal for the plaintext
   header; keep encrypted-side parsing aligned with what #12 leaves.
2. **Format 2.** Both headers are written as `format: 2`. Reading: `format: 1`
   or a missing plaintext header is legacy — reads keep working; every
   mutating path refuses with a typed, value-free error whose message includes
   `run jureuma init to repair`. Unknown future formats keep old jureuma's
   explicit rejection behavior.
3. **Plaintext body exclusion.** `$jureuma` is reserved and excluded from the
   body everywhere the plaintext is read: `ReadPlain` returns the body without
   it, so `list`, `get`, `run` injection, the `set` mirror and `schema infer`
   see body-only data automatically. If any caller parses the plaintext
   directly instead of through `ReadPlain`, note it in your report for the
   parent — do not edit another unit's file.
4. **Consistency.** When both files exist: `publicKey` and `keyId` must match
   each other and the stored key where present — mismatch = exit 5 (the
   existing keyId↔publicKey rule); `home` mismatch is a warning only, never
   fatal.
5. **Writing.** New plaintext starters and header repairs write the header;
   `WritePlain` on `.jsonc` composes with the lossless engine (header insert
   or edit is a span edit). Every byte outside the changed spans is preserved.
6. **Tests.** Parse/marshal round-trips; legacy detection (`format: 1` and
   headerless) and the exact repair pointer; mismatch = exit 5 on load; `home`
   warning; body exclusion for read and mirror paths; a `.jsonc` round-trip
   where header insert/repair leaves comments and formatting intact.
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
- Manual proof in a throwaway `JUREUMA_USER_HOME` with a hand-made legacy pair
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
  `flock -w 120 /tmp/opencode/jureuma-git.lock -c 'cd /root/swe/jureuma && git add <paths> && git commit -m "..."'`.
  Stage only files you own; never `git add -A`; never push.
- Report: base commit, commit hashes, changed files, exact commands + exit
  codes, test names, deviations from the brief and why, residual uncertainty.
  Do not claim results you did not capture.

## Boundaries

- Never leak plaintext, ciphertext values or key material into logs, tests,
  fixtures or error messages; errors report file, line, column and class only.
- DND: never stop to ask a human. Undecided matters: safest reversible choice,
  recorded under `docs/questions/key-identity-headers/`.
