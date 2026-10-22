# Handoff — W2: contract test coverage + pagination (issue #6)

- **Mode:** dnd — never stop to ask; answer every question with the safest
  reversible choice and report every decision.
- **Role:** workhorse (engineer). **Harness:** opencode native background
  subagent; this brief is your session.
- **Model:** `xiaomi/mimo-v2.6-pro` — provider-default thinking, no variant.
- **Working directory:** `/root/swe/zeze` — work on `main`; never create
  branches; never push; never tag.

## Mission

The implementation contracts require table-driven tests for every package.
`internal/doc` and `internal/xerr` have none; `internal/update` and
`internal/paths` lack direct coverage of their network/manifest and
lock/project-root code. Also Convention 0001 requires channel discovery to
inspect every release-catalog page while `update.FetchReleases` caps at
10 pages.

## Skills to load first

`guiho-s-0001-guiho`, `guiho-s-0004-working-with-cg`, `guiho-s-0032-git-commit`,
`guiho-s-0040-explorer`, `guiho-s-runx`.

## Read before acting

- `docs/2026-10-13-zeze-implementation-contracts.md` (packages `doc`, `xerr`,
  `update`, `paths`);
- `internal/doc/doc.go`, `internal/xerr/xerr.go`, `internal/update/update.go`,
  `internal/update/release.go`, `internal/paths/paths.go`;
- existing test style: `internal/jsonc/jsonc_test.go`, `internal/crypt/*_test.go`.

## Requirements

1. `internal/doc/doc_test.go` (new): table-driven coverage of `ParsePointer`
   (escapes `~0`/`~1`), `SetAt`/`DeleteAt` (objects, arrays, append rules),
   `Canonical` (RFC 8785 string escaping, verbatim number form, the
   container-value panic guard via deferred recover), `Marshal`, `FromAny`.
2. `internal/xerr/xerr_test.go` (new): exit-code taxonomy and classification
   (success/failure/usage/etc. exactly as `xerr` defines them).
3. `internal/update/release_test.go` (new): `Download` (httptest server),
   `ParseChecksums`, `ParseManifest` (valid; protocol/cli/version mismatches;
   unsafe or duplicate paths rejected), `ForPlatform` (os/arch matching).
4. Extend `internal/update/update_test.go`: nothing needed beyond pagination
   coverage if cheap.
5. `internal/update/update.go`: paginate until a page returns fewer items
   than `per_page`; keep a documented safety bound no smaller than the
   installer's 50 pages; drafts/malformed still skipped; no other behavior
   change.
6. Extend `internal/paths/paths_test.go`: `Lock` contention, stale-process
   recovery, token-checked unlock; `ProjectRoot` behavior.
7. All tests set `ZEZE_USER_HOME` (or temp dirs) and never touch the real
   home; no external network (httptest only); dummy values only — never
   real-looking secrets.

## Verification

- `gofmt -l` clean on changed Go files; `runx run go-vet-cross`;
  `runx run go-test` green.
- Do NOT run `runx run ci-check` / `test-smoke` / `test-installers` (other
  workers active; the parent runs the central suite).

## Commit contract

- Commit only your files with `flock -w 120 /tmp/opencode/zeze-git.lock -c
  'cd /root/swe/zeze && git add <paths> && git commit -m "<message>"'`.
- Never `git add -A`; never commit paths you do not own; never push.
- A sibling worker owns `internal/update` touch-ups for launcher assets; keep
  to your files (`release_test.go`, `update.go` pagination, `update_test.go`).
  Unexpected modifications elsewhere: leave untouched, report.

## Report (final message)

Commit hash(es); changed files; exact verification results; decisions; blockers.
