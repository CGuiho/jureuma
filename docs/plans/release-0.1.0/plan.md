---
name: Jureuma 0.1.0 — Implementation Completion and First Release
purpose: Sequence the work that completes the jureuma implementation and delivers the first public release (v0.1.0) during the 2026-10-16 unattended (DND) run.
description: Units of work for finishing remaining implementation gaps, validating the full suite, self-reviewing, and releasing jureuma 0.1.0 with Mirror, RunX and the devops release pipeline.
created: "2026-10-16T00:45:00Z"
flags:
  - draft
tags:
  - plan
  - release
keywords:
  - jureuma
  - 0.1.0
  - release
  - dnd
---

#### &copy; 2026 [GUIHO](https://guiho.co) as represented by [Cristóvão GUIHO](https://guiho.co/cguiho) All Rights Reserved.

# Jureuma 0.1.0 — Implementation Completion and First Release

## Mandate

CG, 2026-10-16 (voice, before sleeping): read the GUIHO CLI convention,
take inspiration from Mirror, XDocs, RunX and Buda, **fully implement
jureuma — implement what is missing — and release the artifact and
everything to CG's level of quality.** No questions; DND applies.

This is explicit authorization for: completing the implementation, the
full validation suite, the Mirror version apply for `0.1.0`
(commit, tag `v0.1.0`, push) and publishing the release artifacts
(`release-publish`, `schema-publish`).

## Context

- Baseline before this run: `runx run ci-check` failed on `main`
  (`internal/config` non-string-key test, `internal/redact` U+FFFD
  variant test). Both fixed in-loop and committed on 2026-10-16.
- The implementation is largely complete (16 internal packages, devops
  release pipeline, e2e suite). Recon workers produced a gap report
  used to define the units below.
- Bindings: GitHub `CGuiho/jureuma`, Project #2 `GUIHO`, component
  `jureuma`. Every unit below becomes a real issue on the bound Project.

## Units

Defined from the reconnaissance reports; see the execution section of
this plan and `todo.md`. Units are ordered by dependency; disjoint units
may run in parallel on `main` with serialized Git indexes.

1. Baseline repair (done, in-loop): config + redact test failures.
2. Remaining implementation gaps (from recon).
3. Full validation (`ci-check`, e2e, smoke, installers).
4. Technical self-review (`guiho-s-0445-review`).
5. Release: Mirror apply 0.1.0, version-sync, release-prepare, verify,
   publish, schema-publish, push.

## Constraints

- No new branches; work on `main`.
- Children commit owned units; no child push without parent authorization.
- Secrets boundary and production boundary per `guiho-s-mandume`.
- Questions ledger: `docs/questions/release-0.1.0/`.
