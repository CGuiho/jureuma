---
name: ZeZé 0.1.0 — Implementation Completion and First Release
purpose: Sequence the work that completes the zeze implementation and delivers the first public release (v0.1.0) during the 2026-10-16 unattended (DND) run.
description: Units of work for finishing remaining implementation gaps, validating the full suite, self-reviewing, and releasing zeze 0.1.0 with Mirror, RunX and the devops release pipeline.
created: "2026-10-16T00:45:00Z"
flags:
  - draft
tags:
  - plan
  - release
keywords:
  - zeze
  - 0.1.0
  - release
  - dnd
---

#### &copy; 2026 [GUIHO](https://guiho.co) as represented by [Cristóvão GUIHO](https://guiho.co/cguiho) All Rights Reserved.

# ZeZé 0.1.0 — Implementation Completion and First Release

## Mandate

CG, 2026-10-16 (voice, before sleeping): read the GUIHO CLI convention,
take inspiration from Mirror, XDocs, RunX and Buda, **fully implement
zeze — implement what is missing — and release the artifact and
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
- Bindings: GitHub `CGuiho/zeze`, Project #2 `GUIHO`, component
  `zeze`. Every unit below becomes a real issue on the bound Project.

## Units

Defined from the reconnaissance reports (gap analysis, Convention 0001
compliance audit, sibling-CLI study — all 2026-10-16). Every unit is a real
GitHub issue on the bound Project with the `zeze` component; disjoint
units run in parallel on `main` with serialized Git indexes, and briefs live
under `execution/handoffs/`.

1. Baseline repair (done, in-loop): config + redact test failures
   (`c80be4b`, `0af1e01`); installer-test hermeticity fix (`cc1645e`);
   stale RunX selectors fixed (`8895a7f`).
2. W1 — released launcher artifacts and installer download (issue #5).
3. W2 — contract test coverage and full catalog pagination (issue #6).
4. W3 — Convention 0002 agent-artifact metadata (issue #7).
5. W4 — upgrade transaction journal and instance termination (issue #8;
   queued behind W1).
6. Full validation (`ci-check`, e2e, smoke, installers) at the release
   commit.
7. Technical self-review (`guiho-s-0445-review`) and the human-review
   handoff packet under `docs/reviews/handoff/`.
8. Release: CHANGELOG entry, `version-sync -- 0.1.0`, Mirror apply 0.1.0
   (commit, tag `v0.1.0`, push), `release-prepare`, `release-verify`,
   `release-publish` (+ `schema-publish` only if repair is needed), push.

## Constraints

- No new branches; work on `main`.
- Children commit owned units; no child push without parent authorization.
- Secrets boundary and production boundary per `guiho-s-mandume`.
- Questions ledger: `docs/questions/release-0.1.0/`.
