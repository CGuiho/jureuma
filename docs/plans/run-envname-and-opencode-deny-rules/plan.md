---
name: Run Env Name Default + OpenCode Deny Rules
purpose: Record the two-unit train that defaults `zeze run` to `ZEZE_` and adds OpenCode deny rules to `zeze init`.
description: "Plan for the two-unit train: default the run env name to `ZEZE_` (issue #10) and add OpenCode deny rules to `zeze init` (issue #11), validated through the standard battery and released on CG's word."
created: "2026-10-17"
owner: zeze-docs-plans-run-envname-and-opencode-deny-rules
flags: []
tags:
  - "planning"
keywords:
  - "run env name"
  - "opencode deny rules"
status: stable
---

#### &copy; 2026 [GUIHO](https://guiho.co) as represented by [Cristóvão GUIHO](https://guiho.co/cguiho) All Rights Reserved.

# Run env name default + OpenCode deny rules

## Context

Two requests from CG on 2026-10-17 while dogfooding the 0.2.0 CLI:

- the default run env name must be `ZEZE_` instead of `ZEZE_CONFIG`
  ("Default env name should be this. ZEZE_. Instead of the previous one,
  which is ZEZE_CONFIG"); `auth-ui`'s committed `zeze.yaml` already
  carries `run.envName: ZEZE_` and the current CLI rejects it as
  denylisted;
- `zeze init` adds deny rules only for Claude Code, which the project does
  not use; OpenCode V2 has the equivalent ordered `permissions` mechanism and
  should be covered.

## Units

### W1 — default run env name `ZEZE_` (issue #10)

`DefaultEnvName` becomes `ZEZE_`; `DeniedRunEnvName` excepts exactly the
current default, so `ZEZE_CONFIG` becomes denied like any other
`ZEZE_*` name. Brief: `execution/handoffs/w1-envname-default.md`.

### W2 — OpenCode deny rules in `zeze init` (issue #11)

`init` offers and can add read-deny rules to the project's `opencode.json`
(appended to `permissions` so they win for matching reads at install time);
presence checks, exact removal and state recording mirror the Claude Code
path. Brief: `execution/handoffs/w2-opencode-deny-rules.md`.

## Gates

- Unit gate: full battery (`go-fmt-check`, `go-vet-cross`, `go-test`,
  `test-e2e`, `test-smoke`, `docs-check`, `ci-check`) green per unit;
  orchestrator reviews actual commits, diffs, checks and readbacks.
- Release gate: version bump and release only on CG's explicit word.

## Decisions

- OpenCode target file: project root `opencode.json` — the practice already
  in place in `kiluanji`, `mandume` and `njinga`; an existing
  `opencode.jsonc` is used only when `opencode.json` is absent, and a file
  with comments is left untouched with an "add the deny rules by hand"
  failure (same policy as the Claude path).
- OpenCode rules mirror the Claude set: deny `read` of `*.unencrypted.json`,
  `*.unencrypted.jsonc` and `$HOME/.guiho/zeze/*` (OpenCode `*` matches any
  characters including `/`; `$HOME` expands for `read`), appended to
  `permissions` because the last matching rule wins.
- `ZEZE_CONFIG` is not kept as an allowed override: the directive replaces
  the default, and the denylist exception follows the default only.
- Both units run in one native background workhorse, W1 then W2, one commit
  each (disjoint docs surfaces, serialized Git index).

## Outcome

Delivered 2026-10-17 in one native background workhorse (W1 `c0f9dcd`,
W2 `17e10a1`), both reviewed by the orchestrator with the full battery
green; the units sit at Testing pending CG's release word.
