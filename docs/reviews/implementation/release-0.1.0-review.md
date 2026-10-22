---
name: Release 0.1.0 — Technical Self-Review
purpose: Verify every executed unit and the published release against the validated plan, the specification, the architecture and the GUIHO conventions before the human review begins.
description: "Phase 2.1 self-review of the jureuma 0.1.0 run: unit-by-unit acceptance from actual commits and captured checks, the review fixes applied in-loop, task-state verification against the GitHub Project, and residual risk."
created: "2026-10-16"
owner: jureuma-docs-reviews-implementation
flags: []
tags:
  - "reviews"
keywords:
  - "self-review"
  - "release 0.1.0"
status: stable
---

# Release 0.1.0 — Technical Self-Review

## Scope and method

Self-review (Mandume phase 2.1) of the unattended 2026-10-16 run that closed
every implementation gap and delivered the first public release. Every unit
was judged from its actual commits, diffs, captured exit codes, GitHub
readbacks and owned artifacts — never from a worker summary alone. Workers ran
as OpenCode native background `general` children on `xiaomi/mimo-v2.6-pro`
(provider-default thinking, no variant): W1 `ses_ebe307ee2ffeOKyDhcDx7Lmh7y`,
W2 `ses_ebe307edeffey5xJMjfgIGgjIb`, W3 `ses_ebe307ea8ffea713RdeHld2eGo`,
W4 `ses_ebdf4e448ffeiMNcNtrjnxJAPS`.

## Unit acceptance

### W1 — Launcher artifacts and installed launcher (#5)

- Commits: `1e0a201`, `1471200`, `75ff3ab`, `1d446c3`, `53b8363`.
- Reviewed: launcher sources (`cmd/jureuma-launcher/**` — strict pointer
  decode, containment, exec semantics, no version embedded), the released
  launcher asset in `artifacts.json` (`kind: launcher`), installers
  (`install.sh`, `install.ps1`) downloading and SHA-256-verifying the released
  launcher, transaction activation reduced to the atomic `current.json` swap,
  packaging (`build.sh`, `cross-build.sh`, `release-prepare.sh`,
  `release-verify.sh`), `runx.yaml`, design notes. Accepted.
- Evidence: `test-installers` 24/24 including the launcher equality and
  asset-dir hermetic checks; `release-verify` enforces the 12-binary set.

### W2 — Contract tests and catalog pagination (#6)

- Commits: `306d495`, `fb79370`, `dad10a0`, `3dbdf59`, `43cf142`.
- Reviewed: doc/xerr/update/paths contract coverage; release-catalog
  pagination (`per_page=100`, 50-page bound); the two GAP-documenting test
  cases for `ParseManifest` were resolved by the parent's in-loop fix below.
  Accepted.
- Evidence: `go-test` green on the unit tree and on the final battery.

### W3 — Agent artifact metadata to Convention 0002 (#7)

- Commits: `623bae8`, `d971122`.
- Reviewed: frontmatter added to the bundled instruction, prompts and skill
  with `metadata.version`; `Instruction()` strips frontmatter for
  apply/show (prompt `show` and skill `show --file` stay raw);
  `version-sync.sh` checks and syncs the five artifact files plus the two
  example schema URLs. Accepted.
- Evidence: `version-sync` round-trip verified by W3; final battery green.

### W4 — Upgrade journal and instance termination (#8)

- Commits: `504487b`, `2578d16`, `9ed5910`.
- Reviewed: the journal distinguishes exactly the convention phases
  (`staging`, `artifact-replacement`, `activation`, `verification`,
  `rollback`, `completion`), is write-ahead, strictly validated (a tampered or
  unparseable journal is kept and never acted on), repaired in
  `PersistentPreRun` (skipped by `__self-test`) and under the upgrade lock
  before a new transaction may overwrite it; complete rollback for every
  non-completion phase, roll-forward when staged backups are gone. Instance
  termination inspects real processes (Linux `/proc`, `ps` for macOS/BSD,
  PowerShell/CIM + `taskkill` for Windows), never matches by filename, never
  targets the calling process or descendants of it or of registered `run`
  instances, and never terminates a process whose path or owner is
  unverifiable; only a fully verified old-payload survivor fails the upgrade
  before activation. Accepted.
- Evidence: journal/repair and selection unit tests, Linux `/proc`
  termination tests, `go-vet-cross` for all three OS families, final battery.

### Release umbrella (#4)

- `mirror config check` ok; `mirror version plan 0.1.0` (initial release);
  `mirror version apply 0.1.0 --yes` → `applied: true`, tag `v0.1.0` at
  `473d422` pushed.
- `release-prepare` exit 0; `release-verify` — `0.1.0 ok (22 files)`;
  spot-checks match; the manifest validates against the hardened
  `ParseManifest`.
- `release-publish` exit 0 → release published with 23 assets (the 22
  checksum-covered files plus `checksums.txt` itself, which cannot hash
  itself), read back via `gh api`: not a draft, not a prerelease.

## Convention compliance and deliberate deviations

Help flags, exit codes, flag style, command tree and configuration model
follow `guiho-convention-0001-cli.md`; `ci-check` (format, vet, tests, smoke,
docs, xdocs, config checks) exits 0. Deliberate deviations are recorded in
the question ledger (`docs/questions/release-0.1.0/2026-10-16-dnd-decisions.md`,
Q6): the human-only gate extended to deleting the only private key, the
installed-binary replacement mechanics, the root `XDOCS.md` index, and the
two convention-text conflicts where 0001 wins (B-6 prompt naming escalated
to the human; managed-block placement).

## Fixes applied during self-review

- `e0efbdd` — `ParseManifest` now rejects an unsupported `artifacts.json`
  `schema` version and two artifacts that could be installed together onto
  the same path (overlapping platforms; the same path across distinct
  platforms stays legal, as the real manifest requires). Closes the gap W2
  reported with two GAP-documenting tests; those tests now assert rejection.
- `9d988c4` — launcher-copy help corrected (the copy tracks the launcher,
  so ordinary upgrades do not invalidate it); the W4 brief refreshed.
- `16ad31a` — the two stale process-termination design-note bullets updated
  to the shipped behavior (W4 residue).
- `be15cd0` — the 0.1.0 CHANGELOG entry; `version-sync` confirmed every pin
  already at `v0.1.0`.

## Task state

All five items (`#4`–`#8`) were read back from the bound Project as
`Testing` with component `jureuma` after the green battery; the local
`todo.md` mirrors that state. Per the review discipline nothing is archived
as completed before the human verdict; the handoff packet lists the state.

## Residual risk and limitations

- **Windows:** no Windows host or `pwsh` here. `install.ps1`/`uninstall.ps1`
  were hand-reviewed but not executed (`test-installers` prints "pwsh not
  found"); Windows runtime behavior is covered by `GOOS=windows` vet plus
  review, and the Windows termination path additionally by unit tests of the
  pure selection logic.
- **B-6 prompt naming:** Conventional prompt-file naming conflict between
  Convention 0001 and 0002 remains an open decision for the human (ledger).
- **Dogfooding:** whether this repository should carry the jureuma managed
  block in its own `AGENTS.md` is an open question for the human.
- No material defects found; no requirement was left unverified.
