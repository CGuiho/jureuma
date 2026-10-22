# Handoff — W3: Convention 0002 agent-artifact metadata (issue #7)

- **Mode:** dnd — never stop to ask; answer every question with the safest
  reversible choice and report every decision.
- **Role:** workhorse (engineer). **Harness:** opencode native background
  subagent; this brief is your session.
- **Model:** `xiaomi/mimo-v2.6-pro` — provider-default thinking, no variant.
- **Working directory:** `/root/swe/zeze` — work on `main`; never create
  branches; never push; never tag.

## Mission

Convention 0002 requires metadata on agent artifacts. zeze ships:
`SKILL.md` with only `name`/`description` (missing `metadata.version`);
`assets/prompts/guiho-p-zeze*.md` and `assets/instruction.md` with no
frontmatter at all (the instruction also lacks its `guiho-i-zeze`
identifier). Siblings are the reference: read
`/root/swe/guiho/conventions/guiho-convention-0002-agent-artifacts.md`,
`/root/swe/buda/prompts/*.md`, `/root/swe/buda/skills/guiho-s-buda/SKILL.md`,
`/root/swe/mirror/embed/prompts/` and how sibling installers/CLIs strip
frontmatter when applying instruction blocks
(`/root/swe/mirror/devops/install.sh` `write_instruction_body`,
`/root/swe/runx/pkg/maintenance/maintenance.go`).

## Skills to load first

`guiho-s-0001-guiho`, `guiho-s-0004-working-with-cg`, `guiho-s-0032-git-commit`,
`guiho-s-0040-explorer`, `guiho-s-xdocs`.

## Requirements

1. `internal/agentres/assets/skills/guiho-s-zeze/SKILL.md`: add
   `metadata.version` (release version — set `0.1.0`).
2. `assets/prompts/guiho-p-zeze.md`, `guiho-p-zeze-install.md`,
   `guiho-p-zeze-uninstall.md`: frontmatter modeled on buda's prompts
   (`name`, `purpose`, `description`, `created`, `version`, `owner` —
   owner `zeze-prompts` unless sibling precedent says otherwise; keep
   existing content intact below).
3. `assets/instruction.md`: frontmatter with its `guiho-i-zeze` identity
   (same field pattern; owner `zeze`).
4. Block application: `internal/agentres/block.go` (and `cmd_agent.go` where
   needed) must strip a leading YAML frontmatter block so the applied
   AGENTS.md block body never contains it — both for `agent instruction
   apply` and for `show` follow sibling precedent exactly (check what mirror
   shows; keep `--file`/file output sensible). Installer step 15 uses
   `agent instruction apply`, so stripping there covers the installer.
5. Version alignment: extend `devops/version-sync.sh` (and `--check`
   behavior, which release-prepare already runs) so the artifact metadata
   versions you added are synced/checked against the release version.
   Only `devops/version-sync.sh` — do NOT touch `devops/release-prepare.sh`,
   `devops/install.sh`, `devops/install.ps1` (owned by another unit); report
   if packaging changes seem needed.
6. Tests: update/extend `internal/agentres` tests (frontmatter stripping,
   artifact metadata presence), prompt/skill listing tests, and any
   `internal/cli` agent tests. The `xdocs.yaml` exemption comments for these
   shipped files stay as they are (artifact frontmatter is not XDocs
   frontmatter).
7. Keep the shipped bytes release-safe: `zip` of the skill and prompt assets
   package these files verbatim.

## Verification

- `gofmt -l` clean; `runx run go-vet-cross`; `runx run go-test` green.
- Sanity: build the binary and exercise `agent instruction apply` →
  `remove`, `agent instruction show`, `agent skill install` in a temp
  `ZEZE_USER_HOME` — the applied block must have no frontmatter.
- Do NOT run `runx run ci-check` / `test-smoke` / `test-installers` (other
  workers active; the parent runs the central suite).

## Commit contract

- `flock -w 120 /tmp/opencode/zeze-git.lock -c 'cd /root/swe/zeze &&
  git add <paths> && git commit -m "<message>"'`; never `git add -A`; never
  push; unexpected changes elsewhere: leave untouched, report.

## Report (final message)

Commit hash(es); changed files; exact verification results; decisions
(especially version wiring and show/apply semantics); blockers.
