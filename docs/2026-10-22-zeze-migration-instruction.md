---
name: ZeZé Migration Instruction
purpose: Give the next agent (or CG) one complete, executable instruction for migrating a machine that still has the legacy jureuma installation to the published ZeZé rename, without ever losing the X25519 private keys.
description: A single ordered migration brief: back up and carry the private keys and the whole CLI home across, replace the launcher, project and global config files, managed blocks, agent-tool deny rules, environment variables and agent artifacts, then verify everything and roll back safely. Backward incompatible by design; the legacy names are recorded here only so they can be found and removed.
created: "2026-10-22"
owner: zeze-docs
flags: []
tags:
  - docs
  - migration
  - rename
keywords:
  - zeze
  - migration
  - rename
  - jureuma
  - juju
  - JUREUMA_
  - private keys
  - managed block
  - deny rules
  - agent artifacts
---

#### &copy; 2026 [GUIHO](https://guiho.co) as represented by [Cristóvão GUIHO](https://guiho.co/cguiho) All Rights Reserved.

# ZeZé Migration Instruction

> **This is one instruction.** Hand this whole document to the agent that performs the migration, or follow it yourself, on the machine that still has the legacy `jureuma` installation. It is self-contained: every path, file name, marker and environment variable it names is spelled out. The rename is **backward incompatible by design** — there is no compatibility shim, no fallback and no automatic detection of the old names anywhere. Do the steps in order. The one thing you must never lose is the private keys; everything else is recoverable.

## What migrates

| Concern | On the machine today | After the migration |
| --- | --- | --- |
| CLI, display name | `jureuma` ("GUIHO Jureuma") | `zeze` ("ZeZé") |
| CLI home | `~/.guiho/jureuma/` | `~/.guiho/zeze/` |
| Stable launcher | `~/.guiho/bin/jureuma` (`.exe` on Windows) | `~/.guiho/bin/zeze` (`.exe` on Windows) |
| Optional alias launcher | `~/.guiho/bin/juju` (`.exe`) — feature removed | gone |
| Project config | `jureuma.yaml` | `zeze.yaml` |
| Global config | `~/.guiho/jureuma/jureuma.global.yaml` | `~/.guiho/zeze/zeze.global.yaml` |
| Environment variables | `JUREUMA_*` | `ZEZE_*` |
| Error-tag prefix on stderr | `[JUREUMA:<KIND>]` | `[ZEZE:<KIND>]` |
| `AGENTS.md` managed block | `<!-- >>> jureuma managed block >>> -->` … `<!-- <<< jureuma managed block <<< -->` | `<!-- >>> zeze managed block >>> -->` … `<!-- <<< zeze managed block <<< -->` |
| `.gitignore` managed block | `# >>> jureuma managed block >>>` … `# <<< jureuma managed block <<<` | `# >>> zeze managed block >>>` … `# <<< zeze managed block <<<` |
| Agent skill | `guiho-s-jureuma` | `guiho-s-zeze` |
| Agent prompts | `guiho-p-jureuma`, `guiho-p-jureuma-install`, `guiho-p-jureuma-uninstall` | `guiho-p-zeze`, `guiho-p-zeze-install`, `guiho-p-zeze-uninstall` |
| Configuration pairs in projects | `config.unencrypted.json`, `config.encrypted.json`, `config.schema.json` (+ `.jsonc` variants) | **unchanged — never renamed** |

## Interface changes with no compatibility shim

These four changes break every caller that matches the old spelling. Nothing rewrites them, and nothing falls back:

1. **Structured error-tag prefix.** Failure lines on stderr are tagged `[ZEZE:<KIND>]` (kinds `ERROR`, `USAGE`, `PROTECTED`, `VALIDATION`, `KEY`, `EXEC`; `run` returns 125 for its own failures). CI and scripts that grep for `[JUREUMA:` must be updated.
2. **Managed-block markers.** The `AGENTS.md` and `.gitignore` markers changed. The old markers are unrecognized afterwards: the new CLI sees an ordinary file and **appends a second managed block** unless the old block is removed first.
3. **Environment variable prefix.** `JUREUMA_*` → `ZEZE_*`. **There is no automatic fallback**: an un-renamed `JUREUMA_CONFIG` silently stops being read, so the process starts with no configuration at all.
4. **CLI home path.** Nothing looks at `~/.guiho/jureuma/` any more. Keys, global configuration, state and the audit log are only found under `~/.guiho/zeze/`.

## The one data-loss risk: private keys first

`~/.guiho/jureuma/keys/` and `~/.guiho/jureuma/keys/retired/` hold the X25519 **private keys** (one `k_<32 hex>.json` per key). Every `*.encrypted.json` in every project is encrypted to one of these keys. If the key files are not carried across (or backed up), **every encrypted value in every project becomes permanently undecryptable** — there is no recovery, no key escrow and no reset.

Therefore the order below is fixed: **back up first → move the whole old home to the new one → verify key availability → only then reinstall or uninstall anything.**

The human-only safety path is `jureuma key export [keyId]`: it prints one private key for backup ("[human-only] print a private key for backup", active or retired keys), guarded by the human-only gate (interactive terminal, no agent markers, typed confirmation). `jureuma key list` shows labels and short key IDs and **never** private material. Agents must not run `key export`, `key show --private`, `decrypt` or `get`, and exported keys must never be pasted into logs, tickets or chats.

## Ordered procedure

Run it as the machine's human owner where a step is marked human-only. Stop and roll back (see below) if a verification fails.

### 0. Preconditions (read-only)

- The rename is **published** in the renamed repository `https://github.com/CGuiho/zeze`: the breaking release is `0.4.0` ("ZeZé 0.4.0") or newer, and its installer runs.
- You know every project on this machine that uses the CLI. Find them by their config file and pairs: `find ~ -name 'jureuma.yaml' -o -name '*.encrypted.json'` (skip `node_modules`, `.git`, caches).
- For each project, record the key ID each pair needs: the `keyId` field inside each `<stem>.encrypted.json`, and the `key:` line of `jureuma.yaml`. Retired keys count too.

### 1. Back up the private keys (human-only, first)

1. In a real terminal: `jureuma key list` — note every key (active and retired).
2. `jureuma key export <keyId>` for **every** key ID from step 0 — it prints the private key line to stdout. Store each one offline (password manager, sealed note) or as a CI secret. Never leave them in shell history or on disk in plaintext.
3. As a second copy, archive the key directories themselves to external storage: `tar czf zeze-keys-backup-$(date +%F).tar.gz -C ~/.guiho/jureuma keys` and move the archive somewhere off this machine.

### 2. Move the whole old home to the new one

Move the directory **whole** — it is one unit; do not cherry-pick files:

```sh
mv ~/.guiho/jureuma ~/.guiho/zeze
```

(Extra-safe variant: `cp -a ~/.guiho/jureuma ~/.guiho/zeze`, keep the old directory untouched until final verification passes, then delete it.)

This carries across everything that matters:

| Inside the home | Carried | Notes |
| --- | --- | --- |
| `keys/`, `keys/retired/` | yes | the private keys — the reason the whole home moves |
| `jureuma.global.yaml` | yes | rename it next: `mv ~/.guiho/zeze/jureuma.global.yaml ~/.guiho/zeze/zeze.global.yaml` |
| `state/` | yes | keep `state/deny-rules.json` (the record of old deny rules, see below), `state/upgrade-transaction.json`, `state/protections.key`, `state/protections.json`, `state/stash/` (ciphertext copies kept so `encrypt` reuses the key) |
| `state/alias.json` | **drop it** | dead: the `juju` alias feature was removed; the file only recorded the old alias copy |
| `audit.log` | yes | append-only audit log of human-gated operations |
| `cache.json`, `update-worker.lease`, `locks/`, `run/` | optional | disposable (update cache, worker lease, CLI locks, run work dirs); the installer deletes the first two anyway |
| `current.json`, `installed-artifacts.json`, `versions/<version>/` | yes | installation state; the new installer rewrites them, but keep `installed-artifacts.json` until then (step 4 uses it) |

### 3. Verify key availability (before touching the installation)

- `ls ~/.guiho/zeze/keys/ ~/.guiho/zeze/keys/retired/` lists one `k_<32 hex>.json` per key ID recorded in step 0.
- Every `keyId` referenced by every project's `<stem>.encrypted.json` and by every `key:` binding has a file here.
- If anything is missing, restore from the backup of step 1 **before continuing**. Do not open the key files; their contents are secret.

### 4. Replace the launcher and the installation

1. Remove the stale launchers: `~/.guiho/bin/jureuma`, `~/.guiho/bin/jureuma.exe`, and any stale `~/.guiho/bin/juju`, `~/.guiho/bin/juju.exe`.
2. Install the published rename with **its** installer from `https://github.com/CGuiho/zeze`:

   ```sh
   curl -fsSL https://raw.githubusercontent.com/CGuiho/zeze/main/devops/install.sh | sh
   ```

   ```powershell
   irm https://raw.githubusercontent.com/CGuiho/zeze/main/devops/install.ps1 | iex
   ```

   It installs the stable launcher `~/.guiho/bin/zeze` (`.exe` on Windows), the immutable payload `~/.guiho/zeze/versions/<version>/zeze` (`.exe`), the `current.json` active-version pointer, `installed-artifacts.json`, and the `guiho-s-zeze` skill copies.
3. Useful verified behaviour of the installer — you do not need to clean these by hand:
   - it never touches configuration, keys, state or the audit log;
   - because the old `installed-artifacts.json` was carried into the new home, it recognises the previously installed skill ID and **removes the old skill copies** (`guiho-s-jureuma`) from all skill directories as retired artifacts;
   - it prunes obsolete `versions/` payloads and deletes the disposable `cache.json` and `update-worker.lease`;
   - run it from a project root to have it apply the `AGENTS.md` block there; otherwise apply blocks in step 6.

### 5. Rename the project and global configuration files

- Per project: `git mv jureuma.yaml zeze.yaml` (or plain `mv` outside a repository). Only the CLI's own `*.yaml` files rename.
- Global: `~/.guiho/zeze/jureuma.global.yaml` → `~/.guiho/zeze/zeze.global.yaml` (step 2).
- The first line of these files is a pinned editor hint, `# yaml-language-server: $schema=…/jureuma.schema.json` (and `jureuma.global.schema.json`); the CLI does not read it. Replace it with the first line of the new release's example files, shipped at `~/.guiho/zeze/versions/<version>/artifacts/examples/`, so editors keep validating against the right schema.
- While the file is open, check `run.envName` and `run.prefix`: the default document variable is now `ZEZE_CONFIG`. Whatever name is configured is the name the child process must read — there is no fallback.
- The `key:` binding and key IDs (`k_<32 hex>`) are data, not names: **they never rename**.

### 6. Replace the managed blocks (remove the old ones first)

The old markers are unrecognized after the rename. Remove each old block **completely** (both marker lines and everything between) before the new CLI writes its own; otherwise it appends a second block.

1. **`AGENTS.md`** — old block to delete, markers exactly:

   ```text
   <!-- >>> jureuma managed block >>> -->
   … block body …
   <!-- <<< jureuma managed block <<< -->
   ```

   Then let the new CLI write the new block (`zeze agent instruction apply`, or `zeze init`, or the installer run from the project root). New markers: `<!-- >>> zeze managed block >>> -->` … `<!-- <<< zeze managed block <<< -->`. The rest of `AGENTS.md` is kept; only the managed block is owned by the CLI.
2. **`.gitignore`** — old block to delete, markers exactly:

   ```text
   # >>> jureuma managed block >>>
   *.unencrypted.json
   *.unencrypted.jsonc
   # <<< jureuma managed block <<<
   ```

   The body (`*.unencrypted.json`, `*.unencrypted.jsonc`) is unchanged; only the two marker comment lines differ. New markers: `# >>> zeze managed block >>>` … `# <<< zeze managed block <<<`. `zeze init` adds or repairs the new block.

### 7. Re-establish the agent-tool deny rules

The old CLI wrote deny rules into the project's agent-tool settings and recorded them in `~/.guiho/jureuma/state/deny-rules.json` (now at `~/.guiho/zeze/state/deny-rules.json`) so they could be removed exactly. Two of the three rules name the old CLI home and no longer protect anything:

- **Claude Code** — `.claude/settings.json`, `permissions.deny` array entries:
  `Read(**/*.unencrypted.json)`, `Read(**/*.unencrypted.jsonc)`, `Read(~/.guiho/jureuma/**)`.
- **OpenCode** — `opencode.json` (or `opencode.jsonc`) top-level `permissions` array of rule objects `{"action": "read", "resource": …, "effect": "deny"}` with resources `*.unencrypted.json`, `*.unencrypted.jsonc`, `$HOME/.guiho/jureuma/*`.

Migration per project:

1. Read `~/.guiho/zeze/state/deny-rules.json` — its `projects` and `opencode` maps list exactly what the old CLI added for each project root. Use it as the checklist (and as a cross-check that you removed nothing that was not the CLI's).
2. Delete those entries from `.claude/settings.json` and `opencode.json(c)`. If either settings file contains comments, the CLI refuses to rewrite it — edit it by hand.
3. Run `zeze init` in the project root (option `--agent-deny-rules yes` to answer non-interactively). It offers and records the new rules, which are identical except for the home path: `Read(~/.guiho/zeze/**)` and `$HOME/.guiho/zeze/*`, plus the two unchanged `*.unencrypted.*` rules.
4. The new rules are recorded in the new `state/deny-rules.json` for exact removal later.

### 8. Rename the environment variables (shell profiles and CI)

Search every shell startup file, systemd unit, container/Dockerfile, CI secret store and agent harness config for `JUREUMA_` and rename each hit to `ZEZE_`. At minimum:

| Old | New | Where it matters |
| --- | --- | --- |
| `JUREUMA_CONFIG` | `ZEZE_CONFIG` | the variable `run` injects the document into; **every application and script reading `$JUREUMA_CONFIG` must read `$ZEZE_CONFIG` instead — an un-renamed one silently stops being read, so the process starts with no configuration at all** |
| `JUREUMA_USER_HOME` | `ZEZE_USER_HOME` | home override used by tests/CI sandboxes; the old name is ignored |
| `JUREUMA_PRIVATE_KEY` | `ZEZE_PRIVATE_KEY` | CI private key; the old name is ignored and decryption fails with a key error |
| `JUREUMA_PRIVATE_KEY_<KEYID>` | `ZEZE_PRIVATE_KEY_<KEYID>` | per-key CI secret; `zeze key show` prints the exact expected name as `ciVariable:` |
| `JUREUMA_NO_UPDATE_CHECK` | `ZEZE_NO_UPDATE_CHECK` | disables the background update check; the old name no longer disables it |
| `JUREUMA_AGENT` | `ZEZE_AGENT` | agent marker for the human-only gate; an un-renamed one is no longer detected, so human-only commands may be allowed in that session |

Also present and renamed the same way (test/mirror/ops overrides): `JUREUMA_RELEASES_URL`, `JUREUMA_DOWNLOAD_BASE_URL`, `JUREUMA_UPDATE_WORKER`, `JUREUMA_UPDATE_LEASE`, `JUREUMA_RELEASE_ASSET_DIR`, `JUREUMA_RELEASE_VERSION`, `JUREUMA_UNINSTALL_NATIVE`.

### 9. Rename the agent artifacts

- **Skills.** Old copies live at `~/.agents/skills/guiho-s-jureuma`, plus `~/.claude/skills/guiho-s-jureuma` and `~/.codex/skills/guiho-s-jureuma` when those directories exist. Step 4's installer run removes them; if any survive, delete the directory by hand. Then install the new bundled skill with the new binary's own command: `zeze agent skill install` (and `zeze agent skill list` to see where it landed — `~/.agents/skills/guiho-s-zeze` always, `~/.claude/skills` and `~/.codex/skills` only when those roots exist).
- **Prompts.** `guiho-p-jureuma`, `guiho-p-jureuma-install`, `guiho-p-jureuma-uninstall` → `guiho-p-zeze`, `guiho-p-zeze-install`, `guiho-p-zeze-uninstall`. Prompts ship inside the binary; inspect them with `zeze agent prompt list` and `zeze agent prompt show <id>`. Update any stored agent instruction, snippet or bookmark that tells an agent to load a prompt by the old ID or URL.
- **Instruction.** The `AGENTS.md` block body is the bundled instruction (installed via `zeze agent instruction apply`); it is covered by step 6.

### 10. Final verification

| Command | Expected |
| --- | --- |
| `command -v jureuma; command -v juju` | nothing found |
| `command -v zeze` | `~/.guiho/bin/zeze` |
| `zeze --version` | the raw SemVer of the new release and nothing else |
| `zeze key list` | one line per key (label, short key ID, public-key prefix, state), active first, no private material |
| `zeze key show` | `keyId:` matches the project pair, `ciVariable: ZEZE_PRIVATE_KEY_…`, `privateKey: available (file:keys/…)` |
| `zeze check` (in each project) | every pair reported valid; `… is git-ignored and untracked`; `private key <id> available (…)`; `.gitignore contains the zeze managed block`; `agent-tool deny rules for *.unencrypted.* are present`; `OpenCode deny rules for *.unencrypted.* are present` |
| `zeze run -- sh -c 'test -n "$ZEZE_CONFIG"'` | exit 0 — the document reaches the child under the new name |
| `zeze run -- sh -c 'printf %s "$ZEZE_CONFIG"'` | a redacted document (values replaced with `[REDACTED]`), never plaintext |
| `grep -ri 'jureuma\|juju' ~/.bashrc ~/.zshrc ~/.profile ~/.bash_profile ~/.zprofile <project>/.claude/settings.json <project>/opencode.json* <project>/AGENTS.md <project>/.gitignore` | only expected leftovers (see below) |

**Where the legacy names are still expected after the migration** — and must not be "cleaned up":

- this document;
- git history, the published releases `v0.1.0`–`v0.3.1` and their tags;
- the contents of configuration pairs (`config.unencrypted.json`, `config.encrypted.json`, `config.schema.json` and `.jsonc` variants), key files under `keys/`, and anything those files record internally about the CLI. **Never grep-replace, rename or decrypt these by hand.** Their file names and contents are data. If a pair is flagged with a legacy or malformed identity header, the repair path the CLI names is `zeze init` (it repairs the identity metadata in place, keeping every value, sync and pending state) — never a hand edit.

## What does NOT rename

- Configuration stems inside projects: `config.unencrypted.json`, `config.encrypted.json`, `config.schema.json` (and the `.jsonc` variants), and any named pair such as `production.encrypted.json`. Only the CLI's own `*.yaml` files (`jureuma.yaml`, `jureuma.global.yaml`) rename. **Never rename or decrypt the encrypted pairs by hand.**
- The key IDs (`k_<32 hex>`), public keys, labels and the `key:` binding in the project config.
- `~/.guiho/` itself, `~/.guiho/bin/`, `~/.guiho/.temp/` and any other GUIHO CLI's files.
- The deny-rule bodies for `*.unencrypted.json` / `*.unencrypted.jsonc`; only the home-path rules change.
- The `.gitignore` block body (`*.unencrypted.json`, `*.unencrypted.jsonc`).
- Published releases `v0.1.0`–`v0.3.1`, the git tags and the commit history: **intentionally untouched**. The four published releases stay in the renamed repository under their original titles and assets. This document is the only place the legacy names survive — it exists precisely so they can be found and removed on each machine.

## Rollback

Nothing below destroys data; stop at the first failed verification and roll back.

- **Keys / home not yet moved:** nothing has changed; abort freely.
- **Home moved but not yet reinstalled:** `mv ~/.guiho/zeze ~/.guiho/jureuma` restores the old state exactly (or copy it back if you used the copy variant). The old launcher and payload were only removed in step 4, so restore them from your own copy if needed.
- **Old launchers deleted, new install failed:** the installers roll back to the previous installation on any failure; if the machine is left without a launcher, re-run the new installer (it performs a complete, safe reinstall) and keep the key backup from step 1 regardless.
- **Managed blocks or deny rules half-replaced:** `git checkout -- AGENTS.md .gitignore .claude/settings.json opencode.json` in a git-tracked project, or re-add the removed lines by hand from your diff; re-run `zeze init` afterwards.
- **Worst case (keys backed up in step 1):** delete `~/.guiho/zeze/`, install the new release fresh, copy `keys/` back from the backup, run `zeze init` in each project. Every encrypted value decrypts again as long as the key files are intact.

After the migration, and only after `zeze check` passes in every project, delete the leftover old home (if you kept the copy variant) and the key archives you no longer need — keeping at least one offline key backup is recommended.
