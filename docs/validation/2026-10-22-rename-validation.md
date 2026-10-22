---
name: Rename to ZeZe — Pre-Tag Validation
purpose: Record every pre-tag validation command run after the rename to the ZeZe identity, with its honest exit code, so the release unit starts from observed evidence instead of assumption.
description: "The ci-check battery, the legacy-token completeness gates, binary honesty, history and tag integrity, the pinned-schema transient, the release-notes cosmetic note and the zero-compatibility spot checks observed at HEAD 9ebfe6d before v0.4.0."
created: "2026-10-22"
owner: zeze-docs-validation
flags: []
tags:
  - "validation"
keywords:
  - "rename validation"
  - "pre-tag gate"
  - "history integrity"
  - "completeness gate"
  - "schema pin"
  - "zero compatibility"
  - "0.4.0"
status: stable
---

#### &copy; 2026 [GUIHO](https://guiho.co) as represented by [Cristóvão GUIHO](https://guiho.co/cguiho) All Rights Reserved.

# Rename to ZeZe — Pre-Tag Validation

Every command below was executed from the repository root on 2026-10-22
(UTC+2) against branch `main` at HEAD
`9ebfe6ddfbddff3d2bd81e1a63355bc3c7e4fd97`, four commits ahead of
`origin/main` (`dab9023`). Exit codes were captured from the process, not
reconstructed. Nothing here is a release: no tag was created, nothing was
pushed, and no GitHub release was touched.

## Elision of the legacy tokens

The run's single named exemption for the pre-rename name is
[`docs/2026-10-22-zeze-migration-instruction.md`](../2026-10-22-zeze-migration-instruction.md);
it is byte-identical to `HEAD` and was neither edited nor grep-excluded
silently. This record is **not** exempt, so the two legacy tokens are written
here as `<legacy-cli>` (the CLI's old name) and `<legacy-alias>` (its old
short alias). Their literal spelling is recorded verbatim in the exemption
file; substitute it there to reproduce any command below literally.

## A. Full gate

| Command | Exit | Key output |
| --- | --- | --- |
| `runx run ci-check` | 0 | `gofmt: ok` through `xdocs doctor` → `valid: true`, `errors: 0`, `warnings: 0` |
| `runx run go-test` | 0 | every package `ok`, including `test/e2e 12.324s` |
| `runx run build` | 0 | `built bin/zeze (0.0.0-dev)`, `built bin/zeze-launcher` |
| `runx run test-e2e` | 0 | `ok github.com/CGuiho/zeze/test/e2e 11.272s` |
| `runx run test-smoke` | 0 | checks 1–17 pass, `smoke: passed (0.0.0-dev)` |
| `runx run test-installers` | 0 | `e2e-installers: 24 checks passed`; `pwsh not found: install.ps1 and uninstall.ps1 were not exercised` |
| `runx run test-shell-lint` | 0 | `shellcheck not installed; skipped`, `pwsh not installed; PowerShell scripts not parsed`, `shell-lint: ok` |
| `runx run docs-check` | 0 | `valid: true`, `errors: 0`, `warnings: 0` |
| `runx run schema-check` | 0 | `ok internal/config 0.023s`, `ok internal/schema 0.014s` |
| `mirror config check` | 0 | `configuration file loaded: …/mirror.yaml`, `ok` |
| `runx check --format json` | 0 | `"valid": true`, `"commands": 25`, `"groups": 8` |

`ci-check` itself chains `go-fmt-check`, `go-vet-cross`, `go-test`,
`test-smoke`, `test-shell-lint`, `mirror config check`, `runx check` and
`docs-check`, so its exit 0 independently confirms gofmt and cross-platform
`go vet` for linux, darwin and windows.

**Shell lint is a syntax-level pass only.** `shellcheck` and `pwsh` are not
installed on this host. `sh -n`/`dash -n`/`bash -n` ran over every
`devops/*.sh`; the PowerShell scripts were not parsed at all, and no Windows
runtime behaviour was executed. This is not a clean shellcheck run and must
not be reported as one.

## B. Completeness gates

| Gate | Command | Result |
| --- | --- | --- |
| Legacy CLI name absent | `grep -ri "<legacy-cli>" --exclude-dir=.git --exclude=2026-10-22-zeze-migration-instruction.md .` | exit **1** — zero matches |
| Legacy alias absent | `grep -ri "<legacy-alias>" --exclude-dir=.git --exclude=2026-10-22-zeze-migration-instruction.md .` | exit **1** — zero matches |
| No legacy-named file or directory | `find . -path ./.git -prune -o -iname '*<legacy-cli>*' -print` | exit 0, **empty output** |
| Exemption file unchanged | `git diff --exit-code -- docs/2026-10-22-zeze-migration-instruction.md` and `sha256sum` against `git show HEAD:…` | exit **0**, both digests `58e2707…3d8a` — byte-identical |
| Exemption file still exempt by content | `grep -c -i` inside that file only | 54 lines carry the legacy CLI name, 6 carry the alias |

The exclusion is genuinely effective rather than accidentally broad: the same
`grep -ril` without `--exclude` returns exactly one path, the exemption file.
A second, independent sweep over the tree found **zero** occurrences of either
token (see section F).

## C. Binary honesty

| Check | Command | Result |
| --- | --- | --- |
| Version | `bin/zeze --version` | `0.0.0-dev` (exit 0) |
| Version source | `runx run version-show` | `0.0.0-dev` |
| Help surface | `bin/zeze --help`, `bin/zeze --help-tree` | 0 legacy-name matches in 133 lines of tree output |
| Binary strings | `strings bin/zeze \| grep -i` | 0 matches |
| Throw-away home | `ZEZE_USER_HOME=/tmp/…/home bin/zeze init --agent-evolution disabled --agent-deny-rules no` | exit 0, `zeze is initialized for /tmp/…/proj` |
| Injection | `bin/zeze set /db/host db.internal` then `bin/zeze run -- sh -c …` | exit 0; child saw the document in `ZEZE_CONFIG` (29 bytes, value match exit 0) with `{"db":{"host":"[REDACTED]"}}` on the parent's stdout |
| Real home untouched | `test -e /root/.guiho/zeze` | absent before and after the whole battery |

`0.0.0-dev` is the honest answer, **not** `0.4.0`. `devops/version.sh` resolves
`ZEZE_VERSION`, then an exact `v<semver>` tag at HEAD, then `0.0.0-dev`; HEAD
is four commits past `v0.3.1` and no `v0.4.0` tag exists yet. Reporting `0.4.0`
here would be a fabrication. `mirror version plan 0.4.0` (read-only, no
mutation) still reports `current: 0.3.1`, `next: 0.4.0`, `tag: v0.4.0`.

## D. History integrity

History was not rewritten. `git rev-list --count HEAD` is **105** commits and
the first commit `acc6c9e` still carries the pre-rename name in its subject,
exactly as it was written. Old content still holds old names by design:

```text
$ git show dab9023:go.mod | head -1
module github.com/CGuiho/<legacy-cli>
```

The four rename commits sit linearly on top of `dab9023` with no merge, amend,
rebase or force:

| Commit | Parent | Subject |
| --- | --- | --- |
| `a57a812` | `dab9023` | Track the rename-to-ZeZe run: component option, issue, ledger and task spec |
| `95b4847` | `a57a812` | Delete the obsolete alias feature and its state handling |
| `5e5746b` | `95b4847` | Add the zeze migration instruction and register it in the docs descriptor |
| `9ebfe6d` | `5e5746b` | Rename the project, CLI and every artifact to the ZeZe identity |

Author and committer timestamps are identical on all four, and
`git log --merges dab9023..HEAD` is empty.

### Tags

| Tag | Type | Commit | Expected |
| --- | --- | --- | --- |
| `v0.1.0` | annotated | `473d4225605d3f798d2a23a16a49ba62921e8051` | untouched |
| `v0.2.0` | annotated | `85f3e3ad9c13b9c318dcace0f616031ffb31d5bc` | untouched |
| `v0.3.0` | annotated | `e8cc7953d53ca5fd6a41ca0005087655ec86cb9b` | untouched |
| `v0.3.1` | annotated | `bfae325f846e1cc4e3ee813438722ac41185dc38` | **matches** |

`git rev-parse v0.3.1^{commit}` equals
`bfae325f846e1cc4e3ee813438722ac41185dc38` — verified by direct string
comparison, not by eye. `git ls-remote --tags origin` returns the same four
annotated tags with the same dereferenced commits, so the remote tags were not
moved either.

### Releases

`gh release list -R CGuiho/zeze --limit 100` returns exactly **four**
releases, all published, none draft, none prerelease, each still carrying its
historical legacy title:

| Tag | Title | Draft | Prerelease |
| --- | --- | --- | --- |
| `v0.3.1` | `<legacy-cli> 0.3.1` | no | no |
| `v0.3.0` | `<legacy-cli> 0.3.0` | no | no |
| `v0.2.0` | `<legacy-cli> 0.2.0` | no | no |
| `v0.1.0` | `<legacy-cli> 0.1.0` | no | no |

## E. Open items for the release unit

### 1. Pinned schema URL transient — unresolved by design

`internal/config/examples/zeze.yaml` and `zeze.global.yaml` pin
`https://github.com/CGuiho/zeze/releases/download/v0.3.1/zeze.schema.json`
and `…/zeze.global.schema.json`. The `v0.3.1` release predates the rename and
its assets still carry the legacy file names, so both pinned URLs do **not**
resolve today:

| URL | HTTP status |
| --- | --- |
| `…/download/v0.3.1/zeze.schema.json` | **404** |
| `…/download/v0.3.1/zeze.global.schema.json` | **404** |
| `…/download/v0.3.1/<legacy-cli>.schema.json` (control) | **200** |

This is a known transient, not a regression introduced here. `schema-check`
passes because it validates the example files against the embedded schemas and
never fetches the pinned URL; `release-prepare` does require the pinned URLs to
match, so it cannot succeed until the pin moves. The release unit resolves it
by re-pinning to `v0.4.0` and publishing the schema assets for that tag. This
validation deliberately did **not** re-pin anything.

### 2. Release-notes cosmetic inconsistency

`devops/release-publish.sh` publishes with
`--title "ZeZé $version"` but `--notes "zeze $version. Install: see
README.md#install."`. The title uses the accented display name, the notes use
the unaccented CLI name. Harmless, but a visible inconsistency in every
release; the release unit should settle which spelling the notes use. Not
changed here.

### 3. `/root/.agents` (read-only observation)

Listed read-only, never modified: 133 skill directories plus
`.skill-lock.json`. The human's global install still contains the legacy-named
skill directory `/root/.agents/skills/<legacy-cli-skill>` with mtime
2026-10-18 04:19 — that directory belongs to the migration on the human's
machine, not to this repository, and was left untouched.

Three files under `/root/.agents/skills/guiho-s-runx/` carry today's mtimes
(08:07–08:09), i.e. before this unit's first command; **this unit did not
write them**. They are recorded here rather than silently ignored.

### 4. Build artifacts

`bin/zeze` and `bin/zeze-launcher` exist from `runx run build`/`test-smoke`
and are ignored by `.gitignore:1:bin/`; `git ls-files bin dist` returns
nothing. `dist/` does not exist — no release preparation ran. No artifact is
tracked, and the battery itself left no worktree changes (this record is the
only path this unit adds).

## F. Zero-compatibility spot check

No shim, fallback or dual-read was introduced anywhere:

| Pattern | Command | Result |
| --- | --- | --- |
| Any legacy token in the tree | `grep -rIn --exclude-dir=.git --exclude=2026-10-22-zeze-migration-instruction.md -i -e "<legacy-cli>" -e "<legacy-alias>" .` | exit 1, zero matches |
| Legacy home path or config stems | same grep for `.guiho/<legacy-cli>`, `<legacy-cli>.yaml`, `<legacy-cli>.global.yaml`, `<legacy-cli>.schema.json`, `.<legacy-cli>.` | exit 1, zero matches |
| Environment variables | `grep -rhn '"ZEZE_[A-Z_]*"' cmd internal` | only `ZEZE_AGENT`, `ZEZE_CONFIG`, `ZEZE_DOWNLOAD_BASE_URL`, `ZEZE_NO_UPDATE_CHECK`, `ZEZE_PRIVATE_KEY*`, `ZEZE_RELEASES_URL`, `ZEZE_UPDATE_LEASE`, `ZEZE_UPDATE_WORKER`, `ZEZE_USER_HOME` — no second prefix is read |
| Config stems | `grep -rn 'zeze\.global\.yaml\|zeze\.yaml' internal/config internal/paths` | `ProjectFile = "zeze.yaml"`, `GlobalFile = "zeze.global.yaml"` only |
| Managed block markers | `grep -rn 'managed block' internal/agentres` | `<!-- >>> zeze managed block >>> -->` … `<!-- <<< zeze managed block <<< -->` only |
| Alias binary | `ls bin`, `grep -n 'launcher' devops/install.sh`, `cross-build.sh` | `bin/` holds `zeze` and `zeze-launcher`; installer targets `$HOME/.guiho/bin/zeze` and asset `zeze-launcher-<os>-<arch>` only |
| Alias subcommand | `bin/zeze --help` | the former alias command is gone; `inject` survives only as prose in command descriptions |

The deletion of the alias feature (`95b4847`) removed
`internal/cli/cmd_<legacy-alias>.go` and its state handling; no replacement
reader, no legacy branch and no dual-read path exists.

## Model routing

This unit ran on `opencode/deepseek-v4.1-flash-free#max` under a
**session-scoped override authorized by CG for this session only** ("Not
permanently, just on this session"). It is not a project decision: the
Mastermind/Workhorse tables in `AGENTS.md` and the model registry convention
were not edited by this unit, and the project's declared default remains
MiMo-V2.6-Pro.

## Verification limits

- `shellcheck` and `pwsh` are absent; shell lint is syntax-level only and the
  PowerShell installers were not parsed or exercised.
- No Windows runtime behaviour was executed (cross-platform `go vet` passed).
- The two pinned `v0.3.1` schema URLs return 404; this is the recorded
  transient above, not a passing check.
- Nothing was pushed, tagged or released by this unit, so `origin/main` still
  sits at `dab9023`.
