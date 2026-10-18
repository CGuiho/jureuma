#### &copy; 2026 [GUIHO](https://guiho.co) as represented by [Cristóvão GUIHO](https://guiho.co/cguiho) All Rights Reserved.

# Jureuma Done List

## GitHub Project

- GitHub repository: `CGuiho/jureuma`
- URL: [https://github.com/users/CGuiho/projects/2](https://github.com/users/CGuiho/projects/2)
- GitHub component: `jureuma`

## Completion Summary

| Status | Count |
| --- | ---: |
| completed | 6 |

## Completed Tasks

### 1. Add `inject` as an alias of `run`

- Status: completed
- Created: `2026-10-17`
- Updated: `2026-10-17`
- Completed: `2026-10-17`
- Archived: `2026-10-17`
- Outcome: `inject` is an accepted alias of `run` with identical flags, injected document, redaction and exit codes; delivered on `main` as commit `60a1ee3` (ci-check and e2e green) and released in 0.2.0 (tag `v0.2.0`, GitHub release published).
- GitHub project item: [Issue #9](https://github.com/CGuiho/jureuma/issues/9)
- GitHub component: `jureuma`
- Related files:
  - [plan.md](../../plans/release-0.2.0/plan.md) - Plan, gates and decisions of the release 0.2.0 train.
  - [w1-inject-alias.md](../../plans/release-0.2.0/execution/handoffs/w1-inject-alias.md) - Worker brief for the unit.
- Original index: [../../../todo.md](../../../todo.md)

### 2. Default run env name: `JUREUMA_CONFIG` (restore after the `JUREUMA_` experiment)

- Status: completed
- Created: `2026-10-17`
- Updated: `2026-10-18`
- Completed: `2026-10-18`
- Archived: `2026-10-18`
- Outcome: `JUREUMA_CONFIG` is the default run env name again; `JUREUMA_` is refused as an ordinary `JUREUMA_*` name; every live surface agrees. Delivered in `ca4a80b` and `9efb59d`, released in 0.3.0 (tag `v0.3.0` at `e8cc795`).
- GitHub project item: [Issue #10](https://github.com/CGuiho/jureuma/issues/10)
- GitHub component: `jureuma`
- Related files:
  - [w3-restore-jureuma-config.md](../../plans/run-envname-and-opencode-deny-rules/execution/handoffs/w3-restore-jureuma-config.md) - Worker brief for the restore unit.
  - [0.3.0-train-review.md](../../reviews/implementation/0.3.0-train-review.md) - Self-review acceptance and residual risk.
- Original index: [../../../todo.md](../../../todo.md)

### 3. Add OpenCode deny rules to `jureuma init` alongside the Claude Code ones

- Status: completed
- Created: `2026-10-17`
- Updated: `2026-10-18`
- Completed: `2026-10-18`
- Archived: `2026-10-18`
- Outcome: `init` appends read-deny rules for the plaintext and the jureuma home to `opencode.json`/`.jsonc`, `--agent-deny-rules` governs both tools, `check` reports both, and removal paths stay exact. Delivered in `17e10a1`, released in 0.3.0.
- GitHub project item: [Issue #11](https://github.com/CGuiho/jureuma/issues/11)
- GitHub component: `jureuma`
- Related files:
  - [plan.md](../../plans/run-envname-and-opencode-deny-rules/plan.md) - Plan and gates of the #10/#11 train.
  - [0.3.0-train-review.md](../../reviews/implementation/0.3.0-train-review.md) - Self-review acceptance.
- Original index: [../../../todo.md](../../../todo.md)

### 4. Support writing `.jsonc` files with preserved comments (JSON stays the default)

- Status: completed
- Created: `2026-10-17`
- Updated: `2026-10-18`
- Completed: `2026-10-18`
- Archived: `2026-10-18`
- Outcome: `.jsonc` is fully two-way for human-owned files; only minimal byte spans are spliced, comments/formatting/number text survive, and unrepresentable or unparseable content is refused with nothing written. Delivered in `9749e9e` and `67e6afa`, released in 0.3.0.
- GitHub project item: [Issue #12](https://github.com/CGuiho/jureuma/issues/12)
- GitHub component: `jureuma`
- Related files:
  - [plan.md](../../plans/jsonc-support/plan.md) - Plan and accepted deviations of the JSONC unit.
  - [0.3.0-train-review.md](../../reviews/implementation/0.3.0-train-review.md) - Self-review acceptance.
- Original index: [../../../todo.md](../../../todo.md)

### 5. Single-file pairs: `$jureuma` header on both files, key labels and key selection

- Status: completed
- Created: `2026-10-17`
- Updated: `2026-10-18`
- Completed: `2026-10-18`
- Archived: `2026-10-18`
- Outcome: Both pair files carry the format-2 `$jureuma` identity; the plaintext always carries the `$schema` pointer and the encrypted file the embedded schema copy; legacy state refuses mutations and `init` repairs it keylessly; single-file flows, key labels, the stored-key picker and four-state reports are delivered. Commits `7ece58f`, `09bfad3`, `d6e7c0c`, `4af44de`, `741d536`, `fd86e93`, released in 0.3.0.
- GitHub project item: [Issue #13](https://github.com/CGuiho/jureuma/issues/13)
- GitHub component: `jureuma`
- Related files:
  - [plan.md](../../plans/key-identity-headers/plan.md) - Plan, decisions and gates of the key-identity train.
  - [2026-10-17-dnd-decisions.md](../../questions/key-identity-headers/2026-10-17-dnd-decisions.md) - Question ledger Q1-Q24.
  - [0.3.0-train-review.md](../../reviews/implementation/0.3.0-train-review.md) - Self-review acceptance and residual risk.
- Original index: [../../../todo.md](../../../todo.md)

### 6. Bind the project key pair in `jureuma.yaml` (`key:`)

- Status: completed
- Created: `2026-10-17`
- Updated: `2026-10-18`
- Completed: `2026-10-18`
- Archived: `2026-10-18`
- Outcome: An optional top-level `key:` names the project's key pair (full key ID); one shared resolver enforces it against every pair header (exit 5, never a silent re-encrypt), `init` uses and splices it without touching comments, and `key rotate` is project-wide when bound with every recorded identity following. Commits `131d34c`, `cf61443`, `dbe0410`, `8e4a5f1`, `b144d5a`, released in 0.3.0.
- GitHub project item: [Issue #14](https://github.com/CGuiho/jureuma/issues/14)
- GitHub component: `jureuma`
- Related files:
  - [w4-project-key-binding.md](../../plans/key-identity-headers/execution/handoffs/w4-project-key-binding.md) - Worker brief for the binding unit.
  - [2026-10-17-dnd-decisions.md](../../questions/key-identity-headers/2026-10-17-dnd-decisions.md) - Question ledger Q25-Q39.
  - [0.3.0-train-review.md](../../reviews/implementation/0.3.0-train-review.md) - Self-review acceptance and residual risk.
- Original index: [../../../todo.md](../../../todo.md)
