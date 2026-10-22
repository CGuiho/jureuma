#### &copy; 2026 [GUIHO](https://guiho.co) as represented by [Cristóvão GUIHO](https://guiho.co/cguiho) All Rights Reserved.

# Handoff — restore `ZEZE_CONFIG` as the default run env name (issue #10)

Mode: dnd · Role: workhorse (engineer) · Harness: OpenCode native background
subagent (`general`) · Model: `xiaomi/mimo-v2.6-pro` (provider-default
thinking, no variant) · Issue: <https://github.com/CGuiho/zeze/issues/10>

**Base commit:** the commit that adds this brief (verify with `git log -1`;
record it at start).

## Mission

CG reversed the 0.2.0 env-name experiment (2026-10-17, verbatim): "Make the
environment variable name this again. ZEZE_CONFIG. Now I understand what it
is, and it should be the name!"

The default variable that carries the document for `zeze run --format json`
and `--format file` must be **`ZEZE_CONFIG`** again, in the CLI default and
in every documented surface. `ZEZE_` becomes an ordinary denylisted
`ZEZE_*` name: an explicit `--env-name ZEZE_` / `run.envName: ZEZE_`
is refused like any other zeze-owned name.

`auth-ui`'s committed `zeze.yaml` already sets `run.envName: ZEZE_CONFIG`
again, so the real project works the moment this lands.

## Implementation requirements

1. **Defaults.** `DefaultEnvName = "ZEZE_CONFIG"` in `internal/config` and
   in `internal/runner`. `config.DeniedRunEnvName` keeps its shape (every
   denylisted name except the current default), so `ZEZE_CONFIG` is
   accepted as `--env-name` / `run.envName` and `ZEZE_` is denied. The
   flatten denylist itself does not change (`ZEZE_*` stays denied).
2. **Help and schema.** `internal/cli/cmd_run.go` long/flag help
   ("default ZEZE_"), the `run.envName` description in
   `internal/config/schemas/zeze.schema.json` ("Default ZEZE_."),
   `internal/config/examples/zeze.yaml`, the design document's run section
   and yaml example, and any `internal/agentres/assets/**` text that names the
   default. `README.md` and the repository `AGENTS.md` sentence "into
   `ZEZE_` by default" follow.
3. **Tests.** Flip the expectations, not the coverage: `internal/config`
   (`DefaultEnvName`, the denylist table, the explicit-name parse cases),
   `internal/runner` (default injection, inherited-env replacement, the
   former-default test becomes the new-default test), and `test/e2e`
   (`onefile_test.go`, `inject_test.go`, `lifecycle_test.go`,
   `run_test.go`, `sentinel_test.go`, `main_test.go` helper). Every test that
   asserted `$ZEZE_` now asserts `$ZEZE_CONFIG`; add explicit coverage
   that `--env-name ZEZE_` is refused as denylisted while
   `--env-name ZEZE_CONFIG` is accepted. Do not weaken any redaction,
   private-key-strip or exit-code assertion.
4. **Scripts.** `devops/smoke.sh` must read `$ZEZE_CONFIG` in its `run`
   step and still fail on any planted-value leak.
5. **Docs and changelog.** Update live documentation only (README, root
   `AGENTS.md`, design document, schema/example, agent-resource skill text).
   Do **not** rewrite historical records under `docs/plans/**`,
   `docs/questions/**`, `docs/reviews/**`, `dist/**` or `bin/**`.
   `CHANGELOG.md`: the `## Unreleased` entry that currently describes the
   `ZEZE_` default is part of this same unreleased train — correct it to
   the final behaviour (default `ZEZE_CONFIG`, `ZEZE_` denied); do not
   touch released sections.
6. **No unrelated edits.** Do not change flatten naming, the `run` formats,
   pair identity, schema-copy behaviour or any 0.3.0 train work. If you find a
   surface this brief missed, update it only when it names the default or the
   denylist exception; otherwise record it in the ledger.

## Do not touch

`internal/pair/**`, `internal/jsonc/**`, `internal/keystore/**` beyond test
fixtures, `todo.md`, `docs/plans/**` (except reading this brief),
`docs/questions/**` (except the ledger entry for an undecided matter),
release machinery, version pins, other projects.

## Validation (capture exact commands and exit codes)

- `runx run go-fmt-check`, `runx run go-vet-cross`, `runx run go-test`,
  `runx run test-e2e`, `runx run test-smoke`, `runx run docs-check`,
  `runx run ci-check` — all exit 0.
- Manual proof in a throwaway `ZEZE_USER_HOME` and git project (never the
  real home): (a) `zeze run -- sh -c 'printf %s "$ZEZE_CONFIG"'` prints
  the document; (b) `--env-name ZEZE_` exits with the denylist usage error;
  (c) a project `zeze.yaml` with `run.envName: ZEZE_CONFIG` loads and
  one with `run.envName: ZEZE_` is refused; (d) explicit
  `--env-name APP_CFG` still works.
- `xdocs doctor` stays 0 errors / 0 warnings; update descriptors only if you
  change files they list.

## Commits

One or two coherent commits (code + tests; docs if separable) with imperative
messages, through the serialized index:
`flock -w 120 /tmp/opencode/zeze-git.lock -c 'cd /root/swe/zeze && git add <paths> && git commit -m "<message>"'`.
Stage only owned paths; never `git add -A`; never push; leave the worktree
clean.

## Boundaries

- Never leak plaintext, ciphertext values or key material into logs, tests,
  fixtures or error messages; dummy fixture values are disclosed verbatim.
- Keep every human-only gate and every redaction rule intact.
- DND: never stop to ask a human. Undecided matters: safest reversible
  choice, recorded in `docs/questions/key-identity-headers/2026-10-17-dnd-decisions.md`
  with confidence, action taken and human-review pending.

## Report

Return: base commit verified; commit hashes and subjects; changed files;
exact validation commands with exit codes; new/updated test names; the
manual-proof transcript summary (value-free); any surface that still names the
old default and why; residual uncertainty. Do not claim a result you did not
capture.
