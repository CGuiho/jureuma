---
name: ZeZé 0.2.0 — `inject` Alias for `run`
purpose: Record the release 0.2.0 train that adds `inject` as a command alias of `run`, its unit, brief and gates.
description: "Plan for the release 0.2.0 train: add `inject` as an alias of `run` (issue #9), validate it through the standard battery and release it as the 0.2.0 minor (CG, 2026-10-17)."
created: "2026-10-17"
owner: zeze-docs-plans-release-0.2.0
flags: []
tags:
  - "planning"
  - "release"
keywords:
  - "release 0.2.0"
  - "inject alias"
status: stable
---

#### &copy; 2026 [GUIHO](https://guiho.co) as represented by [Cristóvão GUIHO](https://guiho.co/cguiho) All Rights Reserved.

# ZeZé 0.2.0 — `inject` alias for `run`

## Context

Requested by CG on 2026-10-17 while testing the freshly installed 0.1.0 CLI:
`inject` must be available as a command alias of `run` so both spellings
behave identically.

## Units

### W1 — `inject` alias of `run` (issue #9)

Interactive single-unit change executed through a native background workhorse
(brief: `execution/handoffs/w1-inject-alias.md`): the Cobra alias, e2e
equivalence tests, and the documentation surfaces (help outputs, README,
design notes, bundled skill, CHANGELOG).

## Gates

- Unit gate: `runx run go-fmt-check`, `runx run go-vet-cross`,
  `runx run go-test`, `runx run test-e2e` and `runx run ci-check` all green
  (captured by the workhorse on `60a1ee3` and re-verified by the orchestrator);
  the orchestrator reviewed the actual diff and pushed the commit.
- Release gate: `mirror version plan/apply` and the release pipeline run only
  on CG's explicit authorization.

## Decisions

- Version (CG, 2026-10-17): **minor `0.2.0`** — the alias is a feature
  addition; this supersedes the earlier `0.1.1` patch assumption. Release
  explicitly authorized ("bump minor … release the minor"), so this train
  releases as `0.2.0` and the tree was renamed from `release-0.1.1` on
  2026-10-17.
- Issue #9 closed as completed on GitHub and set to Done on the bound Project
  after the orchestrator's re-verification of `60a1ee3` (2026-10-17).

## Outcome

Released 2026-10-17: `mirror version apply 0.2.0` tagged `v0.2.0` on
`85f3e3a`, `runx run release-prepare` and `runx run release-verify` passed
(all release checks green), and `runx run release-publish` published
<https://github.com/CGuiho/zeze/releases/tag/v0.2.0>. An independent
download spot-check verified `checksums.txt`, `guiho-s-zeze.zip`,
`zeze-darwin-arm64` and `zeze.schema.json`; the release carries the
same 23-asset layout as 0.1.0.
