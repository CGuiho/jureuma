#### &copy; 2026 [GUIHO](https://guiho.co) as represented by [Cristóvão GUIHO](https://guiho.co/cguiho) All Rights Reserved.

# Handoff — W1: `inject` as an alias of `run` (issue #9)

Mode: dnd · Role: workhorse (engineer) · Harness: OpenCode native background
subagent (`general`) · Model: `xiaomi/mimo-v2.6-pro` (provider-default
thinking, no variant) · Issue: <https://github.com/CGuiho/zeze/issues/9>

## Mission

Make `inject` a command alias of `run` in the zeze CLI, so
`zeze inject -- <command> [args...]` behaves exactly like
`zeze run -- <command> [args...]`, and make the alias discoverable in the
documentation surfaces.

## Required reading (before touching anything)

- Repository `AGENTS.md` (this repo); `docs/2026-10-13-zeze-design.md`
  (command-surface sections for `run`);
  `docs/2026-10-13-zeze-implementation-contracts.md` (`internal/cli`).
- Skills: `guiho-s-0001-guiho`, `guiho-s-0004-working-with-cg`,
  `guiho-s-0040-explorer`, `guiho-s-0032-git-commit`, `guiho-s-0016-docs`,
  `guiho-s-runx`.
- `guiho-convention-0001-cli.md` — the command-alias request is explicit from
  the user and allowed; no convention change is needed.

## Implementation requirements

1. Alias: add `inject` as a Cobra alias of the `run` command in
   `internal/cli/cmd_run.go` (`Aliases: []string{"inject"}`). Verify the
   exit-code path still works: `internal/cli/cli.go` decides `isRun` via
   `cmd.Name() == "run"` — Cobra keeps `Name()` as `run` under alias
   invocation; confirm by reading and by test, and adjust only if actually
   broken.
2. Behavior equivalence: `inject` must accept every `run` flag, inject
   `ZEZE_CONFIG` identically, redact child output identically, and return
   the child's exit code. `zeze inject --help` shows the run help;
   `zeze run --help` makes the alias discoverable (Cobra's default
   template does; verify).
3. Help surfaces: check `--help-docs` for `run` (`internal/cli/helpdocs.go`)
   — if the rendered Markdown does not mention the alias, add a minimal,
   style-consistent line (for example a short note shown only when aliases
   exist). Check `--help-tree` — the tree lists command names only; do not
   add a tree node; make sure the existing e2e help-tree tests still pass.
4. Tests (`test/e2e`, mirror the helpers in `test/e2e/run_test.go`):
   - a new e2e test proving `inject` ≡ `run`: the child sees the injected
     document (`ZEZE_CONFIG`), the child exit code passes through, and
     redaction still applies;
   - assert the alias is discoverable (for example `run --help` mentions
     `inject`).
5. Documentation:
   - `README.md`: where `run` is documented, note the `inject` alias (keep it
     concise; do not restructure the README);
   - `docs/2026-10-13-zeze-design.md`: the `run` command description gets
     the alias mention;
   - `internal/agentres/assets/guiho-s-zeze/SKILL.md`: where `run` is
     documented, note the alias; check the prompts under
     `internal/agentres/assets/prompts/` and add the mention only where they
     enumerate `run` (keep edits minimal);
   - `CHANGELOG.md`: add under `## Unreleased` an `### Added` entry for the
     `inject` alias.
6. Do not touch: `todo.md`, `docs/plans/**`, `docs/questions/**`, the GitHub
   Project, version pins (`version-sync` needs no run), or any behavior of
   other commands.

## Validation (capture exact commands and exit codes)

- While iterating: `runx run go-fmt-check`, `runx run go-vet-cross`,
  `runx run go-test`, `runx run test-e2e`.
- Final: `runx run ci-check` and `runx run test-e2e` — both must exit 0.
- Manual proof: with `ZEZE_USER_HOME` (and `HOME` where relevant) set to a
  throwaway temp directory, show the built binary (or `go run`) running
  `inject -- sh -c 'echo $ZEZE_CONFIG'` and injecting the document.

## Deliverable and commit

- One coherent commit with the code, tests, docs and CHANGELOG entry; message
  in imperative mood, e.g. `Add inject as an alias of run`.
- Commit through the serialized index:
  `flock -w 120 /tmp/opencode/zeze-git.lock -c 'cd /root/swe/zeze && git add <owned paths> && git commit -m "..."'`.
  Never push.
- Report: commit hash, changed files, exact commands + captured exit codes,
  test names, and any residual uncertainty. Do not claim results you did not
  capture.

## Boundaries

- Never leak plaintext or key material into logs, tests, fixtures or error
  messages.
- If a convention conflict is found, stop and report it instead of inventing
  a rule.
