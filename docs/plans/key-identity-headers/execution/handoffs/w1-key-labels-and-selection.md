#### &copy; 2026 [GUIHO](https://guiho.co) as represented by [Cristóvão GUIHO](https://guiho.co/cguiho) All Rights Reserved.

# Handoff — W1: key labels, key listing and key selection (issue #13, part 1)

Mode: dnd · Role: workhorse (engineer) · Harness: OpenCode native background
subagent (`general`) · Model: `xiaomi/mimo-v2.6-pro` (provider-default
thinking, no variant) · Issue: <https://github.com/CGuiho/zeze/issues/13>

## Mission

Give every key pair a label and make stored keys listable and selectable, so a
human running `zeze init` (or any key-selection flow) can be asked "Which
one do you want to use?" with `label + short keyId + public-key prefix` —
never key material. CG directive: "For every key that we create, we can attach
a label. The label is usually the name of the project ... Just pick the name
of the repository ... When you list the keys ... You will list the label of
the project and then just the beginning of the public key or the key ID."

This unit is file-disjoint from the in-flight JSONC worker (issue #12). Do not
touch its files: `internal/jsonc/**`, `internal/pair/**`,
`internal/cli/cmd_init.go`, `internal/cli/cmd_values.go`,
`internal/cli/cmd_schema.go`, `test/e2e/lifecycle_test.go`, the design
document, `CHANGELOG.md`, `internal/agentres/**`.

## Current state (verify before changing)

- `internal/keystore/keystore.go` — on-disk `keyFile` JSON
  (`keyId`/`publicKey`/...), `Entry`, `Store.Save(sk)` (~line 234),
  `Store.List() ([]Entry, error)` (~line 313), `Find`, `Retire`, `Delete`.
  No label anywhere.
- `internal/cli/cmd_key.go` — subcommands `show` (~31), `export [keyId]`
  (~85), `rotate` (~134), `retire <keyId>` (~279). No `list` subcommand, no
  label output. `show --private` and `export` stay human-only.
- Key creation call sites today: `init` (`cmd_init.go` — **not yours**) and
  `key rotate` (yours).
- No picker helper exists anywhere.

## Required reading (before touching anything)

- `AGENTS.md` (Mandume section, CLI rules), `docs/2026-10-13-zeze-design.md`
  (key rules, keyId↔publicKey consistency, human-only commands),
  `docs/2026-10-13-zeze-implementation-contracts.md` (keystore and cli
  sections), `docs/plans/key-identity-headers/plan.md` (the train's decisions).
- Skills: `guiho-s-0001-guiho`, `guiho-s-0004-working-with-cg`,
  `guiho-s-0040-explorer`, `guiho-s-0032-git-commit`, `guiho-s-0016-docs`,
  `guiho-s-runx`, `guiho-s-zeze`.
- `guiho-convention-0001-cli.md` (flag rules; `--help` compliance),
  `guiho-convention-0007-models.md`.

## Implementation requirements

1. **Label in the keystore.** `keyFile` gains `label` (omit when empty);
   `Entry` gains `Label` plus whatever short public-key surface the listing
   needs. A legacy key file without `label` must keep loading (label shown as
   a placeholder, e.g. `—`). Do not change key material fields or formats.
2. **Save API.** Keep existing callers compiling: add a labeled save path
   (e.g. `SaveLabeled(sk, label)` with `Save(sk)` delegating to it) — `init`'s
   call site is wired by W3, never edit `cmd_init.go` here.
3. **Default label derivation.** A helper (e.g. in `internal/cli`) deriving
   the default label from the project: basename of `git remote get-url origin`
   minus `.git`, falling back to `filepath.Base(root)`; injectable for tests,
   never touches the network, never fails hard — empty result means no label.
   Wire it into `key rotate` when it creates a new key; add the label line to
   `key show` output.
4. **`key list`** (new subcommand under `key`). One line per stored key:
   `label  k_shortid  zeze-pk-1:prefix  active|retired` — readable, stable,
   includes retired keys with their state, never private material. Follow the
   existing `cmd_key.go` help-text and output style; no new flags beyond the
   convention's long-form rules.
5. **Picker helper** — `internal/cli/keysel.go` (+ tests). Given
   `[]keystore.Entry`: in a TTY, print the keys and prompt
   `Which one do you want to use?`; in a non-TTY it must never read stdin or
   block — callers supply explicit `--key-id`/`--key-label` selection and the
   helper returns a typed "no interactive selection available" result instead.
   Take injected reader/writer for tests; detect TTY portably (e.g.
   `os.Stdin.Stat()` `ModeCharDevice`), no new dependency. Short-id and
   short-public-key rendering helpers live here and are reused by `key list`.
6. **Tests.** keystore: label round-trip on save/load; legacy file without
   `label` loads; `List` surfaces labels. cli: `key list` output shape
   (including a retired key and a label-less key), picker non-TTY behavior,
   short-id rendering. Use new test files; do not edit
   `internal/cli/cli_test.go` (a later unit will touch it).
7. **Descriptors.** Update `internal/keystore/keystore.xdocs.md` and
   `internal/cli/cli.xdocs.md` only where their metadata requires the new
   file/subcommand.

## Do not touch

`internal/cli/cmd_init.go`, `internal/cli/cmd_values.go`,
`internal/cli/cmd_info.go`, `internal/cli/stash.go`, `internal/pair/**`,
`internal/jsonc/**`, `test/e2e/**`, `CHANGELOG.md`, the design document,
`README.md`, `internal/agentres/**`, `todo.md`, `docs/plans/**`,
`docs/questions/**` (except creating the ledger under
`docs/questions/key-identity-headers/` when an undecided matter forces it),
the bound GitHub Project, release machinery, version pins, other projects.

## Validation (capture exact commands and exit codes)

- `runx run go-fmt-check`, `runx run go-vet-cross`, `runx run go-test`,
  `runx run test-e2e`, `runx run test-smoke` — all exit 0; then
  `runx run docs-check` and `runx run ci-check` (`shellcheck` is not
  installed: shell-lint skips; Windows coverage is vet + review here).
- Manual proof in a throwaway `ZEZE_USER_HOME` (never the real home):
  `key list` shows label + short keyId + prefix; `key show` shows the label;
  a label-less legacy key still lists with the placeholder.

## Deliverable and commit

- One or two coherent commits, e.g. `Label stored keys and add key list` and
  `Add a TTY-safe stored-key picker helper`. Imperative messages.
- Commit through the serialized index (the JSONC worker commits in parallel):
  `flock -w 120 /tmp/opencode/zeze-git.lock -c 'cd /root/swe/zeze && git add <paths> && git commit -m "..."'`.
  Stage only files you own; never `git add -A`; never push.
- Report: base commit, commit hashes, changed files, exact commands + exit
  codes, test names, deviations from the brief and why, residual uncertainty.
  Do not claim results you did not capture.

## Boundaries

- Never print or log private key material; labels, short ids and public-key
  prefixes only. Error messages stay value-free.
- DND: never stop to ask a human. Undecided matters: safest reversible choice,
  recorded under `docs/questions/key-identity-headers/`.
