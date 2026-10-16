# Handoff — W4: upgrade journal + instance termination (issue #8)

- **Mode:** dnd — never stop to ask; answer every question with the safest
  reversible choice and report every decision.
- **Role:** workhorse (engineer). **Harness:** opencode native background
  subagent; this brief is your session.
- **Model:** `xiaomi/mimo-v2.6-pro` — provider-default thinking, no variant.
- **Working directory:** `/root/swe/jureuma` — work on `main`; never create
  branches; never push; never tag.
- **Dependency satisfied:** W1 (issue #5, launcher artifacts) landed on `main`
  (`1e0a201`…`53b8363`) and passed the parent's review; the central `ci-check`
  exits 0 and `test-installers` passes 24/24. Activation in `runUpgrade` is
  now only the atomic `current.json` swap (`activate()` in
  `internal/cli/upgrade_tx.go`); the launcher is a released binary that
  ordinary upgrades never replace. The instance-termination block sits just
  before `activate` (search `runner.Instances(home)`); keep `--force`
  semantics. A verified old-payload process executes
  `versions/<previous>/jureuma[.exe]` (symlink-resolved).
- **Ownership:** your files are `internal/cli/upgrade_tx.go`,
  `internal/cli/cli.go`, `internal/runner/**`, any new journal files, the
  tests for these packages, and the one design-doc bullet named in
  requirement 5. Do not modify `internal/update/**`,
  `internal/agentres/**` or `devops/**` (landed units; the parent owns
  in-loop fixes). `devops/e2e-installers.sh` checks 12–15 must stay green.

## Mission

Convention 0001 line 1002: "The upgrade transaction journal must distinguish
staging, artifact replacement, activation, verification, rollback, and
completion so a later invocation can repair an interrupted transaction before
dispatching an ordinary command." Line 962: "Identify other running instances
through a CLI-owned instance registry and operating-system process inspection.
Before activation, forcefully terminate every other process owned by the
current user whose verified executable path is the previous `cliname` payload.
Never terminate a process based only on its filename, and never terminate an
unrelated child command launched by the CLI. If a verified old CLI process
cannot be terminated, fail without activating the new release."

## Skills to load first

`guiho-s-0001-guiho`, `guiho-s-0004-working-with-cg`, `guiho-s-0032-git-commit`,
`guiho-s-0040-explorer`, `guiho-s-runx`.

## Read before acting

- `internal/cli/upgrade_tx.go` (the transaction), `internal/cli/cli.go`
  (`PersistentPreRun` — currently only `runner.SweepStale`),
  `internal/runner/` (run-instance registry), `internal/paths/paths.go`;
- References: `/root/swe/buda/internal/upgrade/transaction.go` (journal
  phases + repair), `/root/swe/buda/internal/lifecycle/instances.go`
  (instance enumeration/kill), `/root/swe/mirror/pkg/updater/journal*`.

## Requirements

1. Journal: record transaction phases (staging, artifact replacement,
   activation, verification, rollback, completion) under the CLI home
   (`state/` preferred; follow buda's shape where sensible). On a later
   invocation, before dispatching an ordinary command, detect an interrupted
   transaction and repair it (complete rollback to a consistent state) —
   wired in `PersistentPreRun` next to the staging sweep. Idempotent,
   best-effort, never destructive beyond the transaction's own staging.
2. Instances: keep the registry behavior (refuse live `run` instances unless
   `--force`) and add OS inspection of current-user processes whose
   **verified executable path** is the previous payload
   (`versions/<previous>/jureuma[.exe]`, symlink-resolved) — Linux `/proc`,
   macOS equivalent, Windows equivalent (port buda's approach). Never match
   by filename; never terminate the CLI's own children; fail without
   activation when a verified old-payload process cannot be terminated.
3. Tests: journal write/repair phases (interrupted states), instance
   selection logic (pure selection function unit tests; OS-level bits
   guarded by build/platform where needed).
4. Do not regress: `--force` semantics, rollback, `__self-test`, recovery
   message first-and-last behavior.
5. Documentation: update the `docs/2026-10-13-jureuma-design.md` "Upgrade
   transaction scope" bullet — its "it keeps no transaction journal, so an
   interrupted upgrade is repaired by re-running `jureuma upgrade` or the
   reinstall command rather than automatically on the next invocation"
   sentence becomes false once the journal lands. Replace it with the journal
   and repair behavior as implemented; keep the rest of the bullet and the
   surrounding decisions intact (this is the only design-doc change allowed).

## Verification

- `gofmt -l` clean; `runx run go-vet-cross`; `runx run go-test` green;
  `runx run test-installers` green.
- Do NOT run `runx run ci-check` / `test-smoke` (parent runs the central
  suite).

## Commit contract

- `flock -w 120 /tmp/opencode/jureuma-git.lock -c 'cd /root/swe/jureuma &&
  git add <paths> && git commit -m "<message>"'`; never `git add -A`; never
  push; unexpected changes elsewhere: leave untouched, report.

## Report (final message)

Commit hash(es); changed files; exact verification results; decisions taken;
blockers with evidence.
