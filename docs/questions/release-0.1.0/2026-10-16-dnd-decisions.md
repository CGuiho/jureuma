---
name: 2026-10-16 DND Decisions
purpose: Record every question the unattended (DND) executor answered itself during the release 0.1.0 implementation-completion and release run.
description: "DND question ledger for the release 0.1.0 run: release authorization and version, the installer-test AGENTS.md pollution incident, push cadence, and boundaries left closed."
created: 2026-10-16
owner: jureuma-docs-questions-release-0.1.0
flags: []
tags:
  - "jureuma"
  - "questions"
  - "release"
  - "dnd"
keywords:
  - "release 0.1.0"
  - "dnd"
  - "installer hermeticity"
  - "question ledger"
---

#### &copy; 2026 [GUIHO](https://guiho.co) as represented by [Cristóvão GUIHO](https://guiho.co/cguiho) All Rights Reserved.

# 2026-10-16 DND Decisions

Context: CG authorized, right before sleeping, a full DND run: finish the
remaining jureuma implementation, validate it and release the artifact to his
level of quality. Rules of the run: never stop, never ask CG, answer every
question with the safest reversible choice and record it here.

## Q1 — Is the release itself authorized?

**Answer: yes.** CG asked to "release the artifact and everything". That is
explicit authorization for `mirror version apply 0.1.0` (commit, tag `v0.1.0`,
push) and for `release-publish` and `schema-publish`. Boundary kept: no deploy,
no promotion, no migration, no credential rotation.

## Q2 — Which version?

**Answer: 0.1.0.** `mirror.yaml`, `AGENTS.md` and the README already fix the
first release as 0.1.0, and `mirror version plan 0.1.0` confirms
`initial: true` with tag `v0.1.0`.

## Q3 — The installer e2e suite appended the jureuma managed block to this repository's own AGENTS.md. Keep it or revert it?

**Answer: revert, and fix the cause.** The block was an unintended artifact:
`devops/e2e-installers.sh` invoked `devops/install.sh` from the invoking
directory, and install.sh applies the managed AGENTS.md block when the current
directory is a project root (this repository qualifies). Fix (cc1645e): every
non-project test invocation now runs from a neutral directory, and the script
guards the invoking `AGENTS.md` against modification — the suite now reports
22 checks including that guard. This repository does not use jureuma for its
own configuration, so the block was reverted; whether to dogfood the managed
block here remains CG's decision.

## Q4 — Push cadence during the run?

**Answer: milestone pushes.** The standing delivery contract announced at
initialization (auto-commit each coherent change and push finished work unless
told otherwise) is unchanged. Pushes happen at safe milestones so verified
work survives any session loss.

## Open questions for CG (not blocking)

- Should this repository carry its own jureuma managed block on purpose
  (dogfooding), now that the installer test can no longer add it by accident?
- Anything found by the gap-analysis and convention-audit reconnaissance that
  cannot be answered with a reversible choice is appended here before the
  release.
