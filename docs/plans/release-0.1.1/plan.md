---
name: Jureuma 0.1.1 — `inject` Alias for `run`
purpose: Record the release 0.1.1 train that adds `inject` as a command alias of `run`, its unit, brief and gates.
description: "Plan for the release 0.1.1 train: add `inject` as an alias of `run` (issue #9), validate it through the standard battery and hand it to the human review."
created: "2026-10-17"
owner: jureuma-docs-plans-release-0.1.1
flags: []
tags:
  - "planning"
  - "release"
keywords:
  - "release 0.1.1"
  - "inject alias"
status: stable
---

# Jureuma 0.1.1 — `inject` alias for `run`

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
  `runx run go-test`, `runx run test-e2e` and `runx run ci-check` all green;
  the orchestrator reviews the actual diff and push the commit.
- Release gate: `mirror version plan/apply` and the release pipeline run only
  on CG's explicit authorization; until then the change stays on `main`
  unreleased.

## Open

- Release timing and version (expected: patch `0.1.1`) — requires explicit CG
  authorization at release time.
