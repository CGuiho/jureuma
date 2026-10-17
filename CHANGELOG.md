#### &copy; 2026 [GUIHO](https://guiho.co) as represented by [Cristóvão GUIHO](https://guiho.co/cguiho) All Rights Reserved.

# Changelog

All notable changes to jureuma are recorded here. Versions are planned and applied with GUIHO Mirror (`mirror.yaml`); entries are written at release time.

## Unreleased

### Added

- `jureuma init` also protects OpenCode sessions in the project: with the agent-tool deny rules it appends read-deny rules for `*.unencrypted.json`, `*.unencrypted.jsonc` and `$HOME/.guiho/jureuma/*` to the project's `opencode.json` (or `opencode.jsonc`), OpenCode V2's ordered `permissions` array (last matching rule wins), creating the file with its `$schema` when absent. `--agent-deny-rules` governs both tools, `check` reports both, and `agent instruction remove` / `uninstall` remove exactly what jureuma recorded for each tool.
- `.jsonc` is now fully two-way for the human-owned files: a comment-preserving writer updates an existing `.jsonc` plaintext or schema in place from `set`, `unset`, `decrypt` and `schema infer` (`WritePlain` and the schema write). Only the minimal byte spans a change needs are spliced; comments, blank lines, formatting, number text (`1e3`, `-0`) and string-escape spelling outside them survive byte for byte, existing trailing commas are preserved (and reused as separators) and no new trailing comma is emitted. A change that cannot be represented — or a file that no longer parses — is refused with nothing written. `init` also creates a missing `.jsonc` plaintext side. `.json` stays the default and the encrypted file stays `.json`.

### Changed

- `jureuma run` injects the document into `JUREUMA_` by default (with `--format json|file`) instead of `JUREUMA_CONFIG`: the CLI default, the denylist exception and every documented surface now agree on the name `auth-ui` already configures. `JUREUMA_CONFIG` is no longer excepted from the denylist and is refused as `--env-name` / `run.envName` like any other `JUREUMA_*` name.

## 0.2.0 - 2026-10-17

### Added

- `inject` is an accepted alias of `run`: the same flags, injected document, redacted child output and exit codes (`jureuma inject -- <command>`).

## 0.1.0 - 2026-10-16

### Added

- The agent-safe configuration CLI: `init`, `encrypt`, `decrypt`, `set`, `unset`, `get`, `run`, `list`, `status`, `check`, `key show|export|rotate|retire`, `schema infer`, `juju-enable`, `juju-disable`, `agent skill|instruction|prompt`, `upgrade check|list` and `uninstall`, with the convention help flags (`--help-docs`, `--help-tree`, `--help-tree-depth`, `--help-tree-global-flags`).
- Configuration pairs plus a schema: the gitignored `<stem>.unencrypted.json` plaintext, the committed `<stem>.encrypted.json` whose every value is X25519-encrypted with readable field names, and a committed JSON or JSONC JSON Schema draft 2020-12 (`schema infer` generates it); dedicated locks keep the pair in sync.
- Agent-safe injection: `jureuma run -- <cmd>` injects the decrypted document (JSON by default; `--format flatten|file`, `--env-name`) into `JUREUMA_CONFIG`, private keys stripped, the document hardened through memfd or a 0600 temp file, and child output redacted.
- The human-only gate: `decrypt`, `get`, `key show --private` and `key export` require a real terminal and a typed confirmation word, refuse agent markers, and audit `--dangerously-bypass-protections`.
- Lifecycle automation: `devops/install.sh`/`install.ps1` and `uninstall.sh`/`uninstall.ps1` with version and channel selection over every catalog page, SHA-256 verification, immutable payloads under `~/.guiho/jureuma/versions/<version>/`, the released stable launcher in `~/.guiho/bin`, transactional rollback, PATH integration and the managed `AGENTS.md` block.
- Upgrades: `jureuma upgrade` runs a synchronous journaled transaction (staging, verification against `checksums.txt` and `artifacts.json`, `--version` and `__self-test` of the candidate, immutable version directory, skill projections, atomic `current.json`, launcher verification, complete rollback) that repairs an interrupted transaction on the next invocation and terminates every verified old-payload process before activation; the stable launcher resolves the active payload and falls back to the retained previous version.
- The release artifact pipeline: six cross-compiled payloads, six launcher binaries, the `guiho-s-jureuma` skill, the three prompts, the instruction source, both configuration schemas, example configurations, `artifacts.json` and `checksums.txt`.
