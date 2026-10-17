#### &copy; 2026 [GUIHO](https://guiho.co) as represented by [Cristóvão GUIHO](https://guiho.co/cguiho) All Rights Reserved.

# Handoff — W1: default run env name `JUREUMA_` (issue #10)

Mode: dnd · Role: workhorse (engineer) · Harness: OpenCode native background
subagent (`general`) · Model: `xiaomi/mimo-v2.6-pro` (provider-default
thinking, no variant) · Issue: <https://github.com/CGuiho/jureuma/issues/10>

## Mission

Change the default env name of `jureuma run` from `JUREUMA_CONFIG` to
`JUREUMA_` — the name `auth-ui` already configures — so the CLI default, the
denylist exception and every documentation surface agree. This unit runs
first; commit it before starting W2.

## Required reading (before touching anything)

- Repository `AGENTS.md`; `docs/2026-10-13-jureuma-design.md` (run/injection
  section); `docs/2026-10-13-jureuma-implementation-contracts.md`.
- Skills: `guiho-s-0001-guiho`, `guiho-s-0004-working-with-cg`,
  `guiho-s-0040-explorer`, `guiho-s-0032-git-commit`, `guiho-s-0016-docs`,
  `guiho-s-runx`.
- `guiho-convention-0001-cli.md` (the flag surface does not change).

## Implementation requirements

1. Constants: `DefaultEnvName = "JUREUMA_"` in `internal/runner/runner.go`
   (~line 42) and `internal/config/config.go` (~line 54); keep the two
   consistent and their comments accurate.
2. Denylist: `internal/config/envnames.go` — `DeniedRunEnvName` already
   excepts exactly `DefaultEnvName`; verify by test that `JUREUMA_` is
   accepted and `JUREUMA_CONFIG` is now denied as a `JUREUMA_*` name. Update
   any comment that names the old default. No other logic change.
3. CLI text: `internal/cli/cmd_run.go` — the `Long` help ("...in --env-name
   (default JUREUMA_CONFIG)") and every other mention, including the
   `--env-name` flag help if it hardcodes the name; check
   `internal/cli/helpdocs.go` payloads if they repeat it.
4. Schema and example: `internal/config/schemas/jureuma.schema.json`
   (run.envName description "Default JUREUMA_CONFIG.") and
   `internal/config/examples/jureuma.yaml` (`envName: JUREUMA_CONFIG`).
5. Docs: `README.md` (two mentions, including the code comment in the run
   example), root `AGENTS.md` (one), `docs/2026-10-13-jureuma-design.md`
   (five: injection prose, format table, the denylist parenthetical
   "(except the default ...)", the sample `jureuma.yaml`),
   `internal/runner/runner.xdocs.md` and `jureuma.xdocs.md` descriptors,
   `devops/smoke.sh` (line ~58).
6. Bundled agent artifacts:
   `internal/agentres/assets/skills/guiho-s-jureuma/SKILL.md` (eight
   mentions, description included), `internal/agentres/assets/instruction.md`
   (two), `internal/agentres/assets/prompts/guiho-p-jureuma.md` (one).
7. Tests: update every expectation that exercises the default
   (`test/e2e/run_test.go`, `inject_test.go`, `sentinel_test.go`,
   `main_test.go`, `lifecycle_test.go`, `onefile_test.go`;
   `internal/runner/runner_test.go`, `internal/config/config_test.go`,
   `internal/cli/cli_test.go`, `internal/agentres/agentres_test.go`; check
   `internal/keystore/keystore_test.go` — leave it if its mention is a
   distinct name). Add:
   - a config test: `run.envName: JUREUMA_` loads (denylist exception) and
     `run.envName: JUREUMA_CONFIG` is rejected as denylisted;
   - a runner or e2e test asserting the default injection is `JUREUMA_`.
8. CHANGELOG: `## Unreleased` → `### Changed` entry describing the new
   default and that `JUREUMA_CONFIG` is no longer excepted.

## Do not touch

Historical records: CHANGELOG 0.1.0/0.2.0 entries, `docs/reviews/**`,
`docs/plans/**`, `docs/questions/**`, `todo.md`. No version pins, no release
machinery, no other projects.

## Validation (capture exact commands and exit codes)

- `runx run go-fmt-check`, `runx run go-vet-cross`, `runx run go-test`,
  `runx run test-e2e`, `runx run test-smoke` — all exit 0; then
  `runx run docs-check` and `runx run ci-check`.
- Manual proof with `JUREUMA_USER_HOME` (and `HOME` where relevant) set to a
  throwaway temp directory: `jureuma run -- sh -c 'echo "$JUREUMA_"'` shows
  the injected JSON, and `envName: JUREUMA_CONFIG` in a project file is
  refused as denylisted.

## Deliverable and commit

- One coherent commit (code + tests + docs + CHANGELOG), message
  `Default the run env name to JUREUMA_`.
- Commit through the serialized index:
  `flock -w 120 /tmp/opencode/jureuma-git.lock -c 'cd /root/swe/jureuma && git add <paths> && git commit -m "..."'`.
  Never push.
- Report: commit hash, changed files, exact commands + exit codes, test
  names, residual uncertainty. Do not claim results you did not capture.

## Boundaries

- Never leak plaintext or key material into logs, tests, fixtures or error
  messages. If a convention conflict is found, stop and report it instead of
  inventing a rule.
