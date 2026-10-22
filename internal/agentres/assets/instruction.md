---
name: guiho-i-zeze
purpose: Provide the bounded zeze managed block for a project's AGENTS.md.
description: Repository instructions that keep project configuration and secrets agent-safe under zeze; applied as the managed AGENTS.md block with its metadata frontmatter stripped.
created: 2026-10-13
version: "0.3.1"
owner: zeze
flags: []
tags:
  - repository-instructions
  - configuration
  - secrets
keywords:
  - zeze
  - agent-safe configuration
  - ZEZE_CONFIG
metadata:
  version: "0.3.1"
---

## zeze (agent-safe configuration and secrets)

This project's configuration and secrets are owned by **zeze** (https://github.com/CGuiho/zeze).

- Configuration lives in pairs: `<stem>.encrypted.json` (committed; field names readable, every value encrypted), `<stem>.unencrypted.json` or `.jsonc` (gitignored plaintext, human-owned), and `<stem>.schema.json` or `.schema.jsonc` (committed JSON Schema, no values). The default stem is `config`; pairs may be declared in `zeze.yaml`.
- Reach for zeze for any configuration value or secret instead of `.env` files, hard-coded values, or ad hoc scripts.

Rules that always apply:

- Never read, open, print, grep, diff, or copy any `*.unencrypted.*` file, or anything under `~/.guiho/zeze/` (except `zeze.global.yaml`, which holds policy only).
- Understand configuration with `zeze list` and `zeze status`; validate with `zeze check`.
- Change a value with `printf '%s' "$VALUE" | zeze set /json/pointer --value-stdin` (add `--json` for numbers, booleans, null, objects, arrays). Remove with `zeze unset /json/pointer`. Never echo values back.
- Run anything that needs configuration with `zeze run -- <command>`. The child receives the whole document as JSON in `ZEZE_CONFIG` (change with `--format json|flatten|file` and `--env-name`). Never try to extract values from a `run` child (no printing, writing to files, or transforming them).
- `decrypt`, `get`, `key show --private`, and `key export` are for humans only. Never pass `--dangerously-bypass-protections` unless the human explicitly asked for it in this conversation.
- If a value is missing or invalid, ask the human to set it. Never guess or invent secrets.
- Exit `3` means the operation is human-only. Exit `4` is a schema validation failure, `5` a key or decryption problem. From `zeze run`, exit `125` means zeze failed, not the child.
- Keep the schema current: `zeze schema infer`, then refine it (types, formats, `description`, `"x-zeze-secret": true`); never put real values in `default`, `examples`, `const`, or `enum` of secret fields.
- Follow the effective `agent.evolution` policy before upgrading zeze or filing issues.

Deeper guidance: the `guiho-s-zeze` skill, the `guiho-p-zeze` prompt (`zeze agent prompt show guiho-p-zeze`), and `zeze <command> --help-docs`.
