# Handoff — W1: released launcher artifacts (issue #5)

- **Mode:** dnd — never stop to ask; answer every question with the safest
  reversible choice and report every decision.
- **Role:** workhorse (engineer). **Harness:** opencode native background
  subagent; this brief is your session.
- **Model:** `xiaomi/mimo-v2.6-pro` — provider-default thinking, no variant.
  Do not change it.
- **Working directory:** `/root/swe/zeze` — work on `main`; never create
  branches; never push; never tag.

## Mission

Convention 0001 (audit 2026-10-16) requires every release to publish "every
launcher artifact required for a fresh installation", and installer step 4 to
"select the exact released launcher and versioned CLI payload for the detected
target". Today zeze generates a shell launcher at install time
(`"launcher": {"unix": "generated-sh", "windows": "payload-copy"}`) and on
Windows the entry point is a payload copy that replaces itself during upgrade.

Deliver per-target launcher binaries, publish them as release assets, and make
the installers download the released launcher.

## Skills to load first

`guiho-s-0001-guiho`, `guiho-s-0004-working-with-cg`, `guiho-s-0032-git-commit`,
`guiho-s-0040-explorer`, `guiho-s-runx`, `guiho-s-xdocs`.

## Read before acting

- `AGENTS.md`, `guiho-convention-0001-cli.md` (sections: Complete Release
  Artifact Set; Stable Launcher and Versioned Payloads; installer step 4/7);
- `docs/2026-10-13-zeze-design.md`, `docs/2026-10-13-zeze-implementation-contracts.md`;
- `devops/install.sh` (launcher generation ~lines 401-440, activation
  ~481-502), `devops/install.ps1` (launcher ~396-437), `devops/release-prepare.sh`,
  `devops/release-verify.sh`, `devops/cross-build.sh`, `devops/build.sh`,
  `devops/e2e-installers.sh`;
- `internal/update/release.go` (manifest parsing), `internal/cli/upgrade_tx.go`
  (activation/verification),
  `internal/paths/paths.go` (home resolution, `ZEZE_USER_HOME`);
- Reference implementations to port from:
  `/root/swe/runx/cmd/runx-launcher/{main.go,delegate_unix.go,delegate_windows.go}`,
  `/root/swe/buda/internal/launcher/launcher.go`, `/root/swe/buda/cmd/buda-launcher/main.go`,
  `/root/swe/buda/devops/install.sh` and `/root/swe/runx/devops/install.sh`
  launcher handling.

## Requirements

1. New `cmd/zeze-launcher` producing `zeze-launcher-<os>-<arch>[.exe]`
   for linux/darwin/windows x amd64/arm64 (6 assets, symmetric with payloads).
   - Resolve the CLI home with the same semantics as `internal/paths`
     (`os.UserHomeDir()`, `ZEZE_USER_HOME` override honored).
   - Read `current.json` strictly (`{protocol:1, active, previous}`); validate
     the target is a relative path inside `versions/`; exec/start
     `versions/<active>/zeze[.exe]` with the original args and streams;
     return the exact exit code; fall back to `previous` when the active
     payload is missing or cannot start; clear error and non-zero exit when
     both fail.
   - Unix: process replacement (exec) semantics. Windows: spawn child, wire
     stdio, wait, propagate exit code (port runx's delegate approach).
2. `devops/cross-build.sh` / `devops/build.sh` / `devops/release-prepare.sh`:
   build and package the launcher assets; add `artifacts.json` entries (kind
   `launcher`, `installedPath` `$HOME/.guiho/bin/zeze[.exe]`, checksums);
   update the `"launcher"` descriptor accordingly; `release-verify.sh` must
   require the complete new asset set.
3. `devops/install.sh` / `devops/install.ps1`: download + SHA-256-verify the
   launcher for the detected target from the selected release (honor
   `ZEZE_RELEASE_ASSET_DIR` in tests) instead of generating/copying;
   transactional rollback restores the previous launcher; idempotent reinstall.
4. Windows: stable launcher is the entry point; ordinary `zeze upgrade`
   must not replace the running launcher (adjust `internal/cli/upgrade_tx.go`
   activation accordingly; payloads install only into `versions/<v>/` with the
   atomic `current.json` swap). First release — no legacy migration needed.
5. The launcher copy in `~/.guiho/bin`: it must still work when the launcher is
   a binary (executable permissions, checksum ownership).
6. `devops/e2e-installers.sh`: adapt all checks to the new asset set (launcher
   downloaded and installed, fallback-to-previous still proven, Windows
   fixture includes the launcher) — keep equal or stronger coverage. The
   hermeticity guard at the end must keep passing.
7. New descriptor `cmd/zeze-launcher/zeze-launcher.xdocs.md`; update
   `cmd/cmd.xdocs.md` children.

## Verification (all must pass; report exact results)

- `gofmt -l` clean on changed Go files.
- `runx run go-vet-cross` and `runx run go-test` green.
- `runx run test-installers` green (builds local fixtures in temp; you may run
  it — it does not touch shared outputs).
- Do NOT run `runx run ci-check` or `runx run test-smoke` (shared build
  outputs; the parent runs the central suite after the wave).

## Commit contract

- Commit only files you changed for this unit, using a concise imperative
  message. Serialize the Git index with the shared commit lock:
  `flock -w 120 /tmp/opencode/zeze-git.lock -c 'cd /root/swe/zeze && git add <paths> && git commit -m "<message>"'`
- Never `git add -A`; never commit paths you do not own; never push.
- If you see unexpected modifications outside your files, leave them
  untouched and report them.

## Report (final message)

Commit hash(es); changed files; exact verification commands with results;
every decision taken with its rationale; any blocker with evidence.
