---
name: Release 0.1.0 — Validation
purpose: Record every validation command run for the 0.1.0 delivery and its honest result, including skipped checks, so the human review starts from real evidence.
description: "Commands, captured results, skipped checks and readiness for the zeze 0.1.0 delivery: the central ci-check battery, e2e and installer suites, version sync, mirror apply and the release pipeline."
created: "2026-10-16"
owner: zeze-docs-validation
flags: []
tags:
  - "validation"
keywords:
  - "release 0.1.0"
  - "validation battery"
status: stable
---

# Release 0.1.0 — Validation

All commands were run through the RunX catalog from the repository root on
2026-10-16 (UTC+2). Exit codes were captured explicitly; nothing here is
reconstructed from memory.

## Validation battery (frozen tree `be15cd0`, after review fixes)

| Command | Target | Result |
| --- | --- | --- |
| `runx run ci-check` | Format, cross-vet, tests, smoke, docs, xdocs, config checks | **exit 0** |
| `runx run test-e2e` | End-to-end CLI tests against the built binary | **exit 0** |
| `runx run test-installers` | Installer/upgrade/lifecycle harness | **exit 0 — 24 checks passed** |

The installer suite includes the released-launcher equality checks, the
asset-dir hermetic install, the upgrade refusal/rollback/channel checks
(12–15 unchanged), and the hermeticity guard (24): the invoking directory's
`AGENTS.md` was not modified.

## Release pipeline (tag `v0.1.0` at `473d422`)

| Command | Result |
| --- | --- |
| `runx run version-sync -- 0.1.0` | All five artifact files and both example schema URLs already pin `v0.1.0` |
| `mirror config check` | ok |
| `mirror version plan 0.1.0` | initial release; tag `v0.1.0` |
| `mirror version apply 0.1.0 --yes` | `applied: true`; tag pushed (`refs/tags/v0.1.0` → `473d422`) |
| `runx run release-prepare` | **exit 0** — 6 payloads + 6 launchers + agent/schema/example artifacts |
| `runx run release-verify` | **exit 0** — `release-verify: 0.1.0 ok (22 files)` |
| `runx run release-publish` | **exit 0** — release published, 23 assets |

Post-publish readbacks: `gh api` lists 23 assets; `checksums.txt` covers
every other file and is (correctly) not self-listed; SHA-256 spot-checks for
`zeze-linux-amd64` match both `checksums.txt` and `artifacts.json`; the
release is neither draft nor prerelease.

## Checks not run here

- `install.ps1` / `uninstall.ps1` execution — `pwsh` is not installed on this
  host; the suite prints "pwsh not found: install.ps1 and uninstall.ps1 were
  not exercised". The scripts were hand-reviewed and mirrored by the tested
  Unix versions.
- Windows runtime paths (launcher resolution, upgrade transaction,
  termination) — no Windows host; covered by `GOOS=windows` vet plus code
  review and by unit tests of the pure logic.

## Readiness

Ready for human review: the release is live at
<https://github.com/CGuiho/zeze/releases/tag/v0.1.0>, the tagged tree is
validated, and no check was skipped silently.
