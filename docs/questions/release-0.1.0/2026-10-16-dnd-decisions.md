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

## Q5 — Which reconnaissance gaps became work units?

**Answer: four units, each a real issue on the bound Project (component
`jureuma`), plus the umbrella release issue.** The convention audit
(2026-10-16) and the gap analysis (2026-10-16) drove the selection:

| Unit | Issue | Gap | Dispatch |
| --- | --- | --- | --- |
| W1 | [#5](https://github.com/CGuiho/jureuma/issues/5) | Released per-target launcher artifacts; installers download the launcher; Windows stable launcher (Convention 0001 release-artifact set and installer step 4) | session `ses_ebe307ee2ffeOKyDhcDx7Lmh7y` |
| W2 | [#6](https://github.com/CGuiho/jureuma/issues/6) | Contract test coverage for `doc`, `xerr`, `update`, `paths`; full release-catalog pagination | session `ses_ebe307edeffey5xJMjfgIGgjIb` |
| W3 | [#7](https://github.com/CGuiho/jureuma/issues/7) | Convention 0002 metadata on the bundled agent artifacts | session `ses_ebe307ea8ffea713RdeHld2eGo` |
| W4 | [#8](https://github.com/CGuiho/jureuma/issues/8) | Upgrade transaction journal with repair; verified old-payload instance termination (Convention 0001, lines 962 and 1002) | session `ses_ebdf4e448ffeiMNcNtrjnxJAPS` |

One claimed gap was retracted after cross-check: the gap analysis said
`jureuma upgrade` had no automated coverage, but `devops/e2e-installers.sh`
already exercises it (corruption refusal, missing-asset refusal, rollback,
channel install, launcher fallback — checks 11–15). No unit created for it.

## Q6 — Convention deviations kept deliberately

**Answer: keep each one, with the rationale below** (all recorded for CG's
review; largely deliberate safety or already-confirmed design decisions):

- **Prompt file naming** (`guiho-p-jureuma-install` instead of the literal
  `<cliname>-p-install`): Convention 0001 is internally inconsistent here —
  0002 mandates the `guiho-p-` grammar and the AGENTS.md confirmed decisions
  fix `guiho-p-jureuma-`. Kept; escalated as a convention fix for CG.
- **Uninstall key protection**: `uninstall` demands a human-only confirmation
  for `keys/` even with `--yes`, and preserves keys on non-interactive runs.
  Deleting keys destroys the ability to decrypt committed ciphertext — an
  irreversible data-loss risk — so safety wins over the literal `--yes`
  contract. Kept; documented in the README.
- **Binary-replacement mechanics** (`.old` pre-clean list, Windows detached
  helper): jureuma meets the mandated goal (stale-backup-proof replacement,
  verification-gated activation, idempotent reruns) through staging-only
  backups and rename replacement rather than the literal mechanics. Kept.
- **Root `XDOCS.md` index**: Convention 0001 requires it while the project
  text (and some sibling repos) operated without one. Resolved toward the
  convention: the root index now exists as the special setup file, is listed
  in the root descriptor, and the AGENTS.md sentence was updated.

## Q7 — What state did the managed-block incident end in?

**Answer: clean, with the systematic cause fixed.** The audit report
recorded the jureuma managed block present in this repository's AGENTS.md
during its run. The systematic cause — the installer e2e suite invoking
`install.sh` from the repository root — was already fixed (cc1645e) with a
guard check that now passes; the tree is clean at every commit since. The
dogfooding question stays open above.

## Repairs in-loop (defects, not questions)

- Stale RunX selectors (`runx run jureuma-<uid>`) in the `mirror.yaml` hook
  text and the AGENTS.md "Everyday commands" block were corrected to the live
  bare uids (8895a7f); all 15 selectors verified to resolve first.
- The design document's stale status line ("nothing here is implemented") and
  its Q7 "(repo not created yet)" note were corrected (c90d635).

## Unit review and dispatch (record)

- **W1–W3 were reviewed through their actual commits, not summaries**
  (2026-10-16, later): the launcher sources (`cmd/jureuma-launcher/**`), the
  release manifest and upgrade-transaction changes (`internal/update`,
  `internal/cli`), the installer hunks (`install.sh`, `install.ps1`), the
  packaging changes (`build.sh`, `cross-build.sh`, `release-prepare.sh`,
  `release-verify.sh`, `runx.yaml`), the artifact metadata and stripping
  (`internal/agentres`), `version-sync.sh`, the e2e additions, and the
  design-note update. All accepted.
- **Central acceptance:** `runx run ci-check` exits 0 on the 12 unit commits
  (captured exit code); `runx run test-installers` passes 24/24, including
  the released-launcher checks and the hermeticity guard.
- **In-loop review fixes:** `e0efbdd` closes the W2-reported gap —
  `ParseManifest` now refuses an unsupported artifacts.json `schema` version
  and two artifacts that can be installed together onto the same path
  (overlapping platforms; the same path across distinct platforms stays
  legal, as the real manifest requires for payloads and launchers);
  `9d988c4` refreshes the launcher-copy help (the copy tracks the launcher,
  so ordinary upgrades do not invalidate it) and the W4 brief.
- **W4 dispatched** once W1 had landed: issue #8 flipped to In Progress on
  the Project (read back), session `ses_ebdf4e448ffeiMNcNtrjnxJAPS`, brief
  `docs/plans/release-0.1.0/execution/handoffs/w4-upgrade-hardening.md`
  (refreshed with the post-W1 state, ownership, and the one design-doc
  bullet it must update).
- **Known limitation (recorded for the handoff):** `pwsh` is absent on this
  host, so the `install.ps1`/`uninstall.ps1` changes were hand-reviewed but
  not executed here, and Windows runtime behavior (launcher, upgrade
  transaction) is covered by `GOOS=windows` vet plus code review only.

## W4 review and release preparation (record)

- **W4 reviewed through its commits** (`504487b`, `2578d16`, `9ed5910`): the
  journal distinguishes exactly the convention phases (`staging`,
  `artifact-replacement`, `activation`, `verification`, `rollback`,
  `completion`), is written ahead of every change, strictly validated (a
  tampered or unparseable journal is never acted on and never deleted), and is
  repaired in `PersistentPreRun` (which `__self-test` skips) and again under
  the upgrade lock before a new transaction may overwrite it. Instance
  termination inspects real processes (Linux `/proc`, `ps` for macOS/BSD,
  PowerShell/CIM + `taskkill` for Windows), never matches by filename, never
  targets the calling process or descendants of it or of registered `run`
  instances, excludes the current user's other processes' children, and fails
  closed only for a fully verified old-payload survivor; a process whose path
  or owner cannot be verified is never terminated. Accepted.
- **W4 residue closed in-loop** (`16ad31a`): the two stale design-note bullets
  ("terminates only registered `run` instances" and "no other process is
  inspected or terminated") now describe the shipped behavior.
- **Known limitation (recorded):** there is no Windows host or `pwsh` here, so
  the Windows termination path and `install.ps1`/`uninstall.ps1` are covered
  by `GOOS=windows` vet plus code review only.
- **Release preparation committed** (`be15cd0`): the 0.1.0 CHANGELOG entry and
  `version-sync` (every artifact and example already pins v0.1.0; no changes
  needed). The final validation battery runs on this tree.

## Release (record)

- **Validation battery on the frozen tree:** `runx run ci-check` exit 0,
  `runx run test-e2e` exit 0, `runx run test-installers` exit 0 (24 checks).
- **Mirror:** `mirror config check` ok; `mirror version apply 0.1.0 --yes` →
  `applied: true`; tag `v0.1.0` at `473d422` pushed.
- **Artifacts:** `release-prepare` and `release-verify` exit 0 —
  `release-verify: 0.1.0 ok (22 files)`; the released set carries 23 assets
  because `checksums.txt` cannot hash itself; SHA-256 spot-checks match both
  `checksums.txt` and `artifacts.json`.
- **Publish:** `release-publish` exit 0 →
  <https://github.com/CGuiho/jureuma/releases/tag/v0.1.0> (not a draft, not a
  prerelease), 23 assets read back via `gh api`.
- **Task state:** all five items read back from the Project as `Testing` with
  component `jureuma`; completion and archival wait for the human verdict per
  the review discipline.
- **Records written:** `docs/reviews/implementation/release-0.1.0-review.md`,
  `docs/validation/release-0.1.0-validation.md`,
  `docs/reviews/handoff/2026-10-16-release-0.1.0.md`.

## Open questions for CG (not blocking)

- Should this repository carry its own jureuma managed block on purpose
  (dogfooding), now that the installer test can no longer add it by accident?
- Convention 0001 vs 0002 prompt-file naming inconsistency (see Q6) — the
  convention text should be reconciled.
- Anything found later in the run that cannot be answered with a reversible
  choice is appended here before the release.
