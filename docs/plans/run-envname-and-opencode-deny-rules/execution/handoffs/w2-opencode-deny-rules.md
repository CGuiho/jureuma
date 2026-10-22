#### &copy; 2026 [GUIHO](https://guiho.co) as represented by [Cristóvão GUIHO](https://guiho.co/cguiho) All Rights Reserved.

# Handoff — W2: OpenCode deny rules in `zeze init` (issue #11)

Mode: dnd · Role: workhorse (engineer) · Harness: OpenCode native background
subagent (`general`) · Model: `xiaomi/mimo-v2.6-pro` (provider-default
thinking, no variant) · Issue: <https://github.com/CGuiho/zeze/issues/11>

## Mission

Extend the agent-tool deny rules so `zeze init` also protects OpenCode
sessions inside the project — not only Claude Code — using OpenCode V2's
ordered `permissions` array in the project's `opencode.json`. This unit runs
after W1's commit.

## Required reading (before touching anything)

- Repository `AGENTS.md`; `docs/2026-10-13-zeze-design.md` (deny-rules
  passages); `docs/2026-10-13-zeze-implementation-contracts.md`.
- Skills: `guiho-s-0001-guiho`, `guiho-s-0004-working-with-cg`,
  `guiho-s-0040-explorer`, `guiho-s-0032-git-commit`, `guiho-s-0016-docs`,
  `guiho-s-runx`.
- `guiho-convention-0001-cli.md`.
- **OpenCode V2 documentation — the live source of truth for the exact
  shape**: fetch <https://opencode.ai/v2/docs/permissions> and
  <https://opencode.ai/v2/docs/config>. V2 only — V1 names (`permission`,
  `bash`, `task`) are wrong. Confirm the rule fields (`action`, `resource`,
  `effect`), that `read` is the action for file reads, that `*` matches any
  characters including `/`, that `$HOME`/`~` expands for `read` resources,
  and that the last matching rule wins before writing any code.

## Implementation requirements

1. Extend `internal/cli/denyrules.go` (the Claude path must keep working
   byte-for-byte):
   - Target resolution: the project's `opencode.json`; if absent but
     `opencode.jsonc` exists, use that; if neither exists, create
     `opencode.json`. Never write into `.opencode/` or any global config.
   - Rules to add, appended to the project's top-level `permissions` array
     (append = last word at install time), as objects with the live-docs
     field names:
     - read-deny `*.unencrypted.json`
     - read-deny `*.unencrypted.jsonc`
     - read-deny `$HOME/.guiho/zeze/*`
     Adjust exact resource strings only if the live V2 docs contradict the
     matching semantics above; say so in the report.
   - Preserve every other key and array entry. A file with comments is
     refused with an "add the deny rules by hand" failure — same policy as
     `readSettings` for Claude settings. A newly created `opencode.json`
     carries `"$schema": "https://opencode.ai/config.json"` first (the shape
     GUIHO projects already use — see `mandume/opencode.json`) followed by
     `permissions`.
   - State: extend `denyState` with an OpenCode map (for example
     `"opencode"`) recording exactly what was added per project root; state
     files lacking the key must keep loading, and the existing Claude
     `"projects"` map keeps its meaning.
   - Functions mirroring the Claude trio: presence check, apply (records
     additions), exact removal via the recording.
2. `internal/cli/cmd_init.go`: after the Claude block, a second block — when
   the OpenCode rules are already present, print the existing "already
   present" style confirmation with the file path; otherwise prompt
   `[project] Add deny rules for *.unencrypted.* and ~/.guiho/zeze to <path> (recommended)?`
   in TTY mode. `--agent-deny-rules yes|no` governs both tools. Report the
   added count like the Claude block does.
3. `internal/cli/cmd_info.go` (`check`): add OpenCode presence/absence
   reporting parallel to the Claude lines.
4. Removal paths: `internal/cli/cmd_agent.go` (`agent remove`) and
   `internal/cli/cmd_uninstall.go` (`uninstall`, including its PRESERVE list
   entry) remove both tools' recorded rules exactly.
5. Installer scripts: `devops/uninstall.sh` and `devops/uninstall.ps1` — add
   the `opencode.json` counterpart to their `.claude/settings.json` PRESERVE
   entry (mirror the existing wording; the deny entries zeze added must be
   removed by hand when the payload is gone).
6. Docs: `docs/2026-10-13-zeze-design.md` (the deny-rules passages, which
   today say "for example Claude Code `permissions.deny`"), `README.md`
   (deny-rule mentions), `internal/agentres/assets/skills/guiho-s-zeze/SKILL.md`
   and the prompts that mention deny rules
   (`internal/agentres/assets/prompts/guiho-p-zeze.md`,
   `guiho-p-zeze-uninstall.md`) — mention both tools.
7. Tests:
   - unit tests for: created-file shape, merge preserving unrelated
     settings, comment refusal, state recording + exact removal,
     backward-compatible old state file, presence check;
   - `internal/cli/cli_test.go`: extend the init deny-rules coverage (there
     is an existing test around `--agent-deny-rules`) to assert
     `opencode.json` contents;
   - e2e `test/e2e/sentinel_test.go`: mirror the deny-rule coverage for
     `opencode.json`; `agent remove` and `uninstall` remove the rules.
8. CHANGELOG: `## Unreleased` → `### Added` entry.

## Do not touch

`todo.md`, `docs/plans/**`, `docs/questions/**`, historical release notes and
reviews, version pins, release machinery, global OpenCode config
(`~/.config/opencode/**`), other projects.

## Validation (capture exact commands and exit codes)

- `runx run go-fmt-check`, `runx run go-vet-cross`, `runx run go-test`,
  `runx run test-e2e`, `runx run test-smoke` — all exit 0; then
  `runx run docs-check` and `runx run ci-check`.
- Manual proof with `ZEZE_USER_HOME` set to a throwaway temp directory:
  run `init --agent-deny-rules yes` in a temp project; show the resulting
  `opencode.json`; run `agent remove` and show the rules are gone and the
  rest of the file survives.

## Deliverable and commit

- One coherent commit after W1, message
  `Add OpenCode deny rules to init alongside Claude Code`.
- Commit through the serialized index:
  `flock -w 120 /tmp/opencode/zeze-git.lock -c 'cd /root/swe/zeze && git add <paths> && git commit -m "..."'`.
  Never push.
- Report per unit: commit hash, changed files, exact commands + exit codes,
  test names, residual uncertainty. Do not claim results you did not capture.

## Boundaries

- Never leak plaintext or key material into logs, tests, fixtures or error
  messages. If a convention conflict is found, stop and report it instead of
  inventing a rule.
