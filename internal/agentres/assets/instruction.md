---
name: guiho-i-jureuma
purpose: Provide the bounded jureuma managed block for a project's AGENTS.md.
description: Repository instructions that keep project configuration and secrets agent-safe under jureuma; applied as the managed AGENTS.md block with its metadata frontmatter stripped.
created: 2026-10-13
version: "0.3.1"
owner: jureuma
flags: []
tags:
  - repository-instructions
  - configuration
  - secrets
keywords:
  - jureuma
  - agent-safe configuration
  - JUREUMA_CONFIG
metadata:
  version: "0.3.1"
---

## jureuma (agent-safe configuration and secrets)

This project's configuration and secrets are owned by **jureuma** (https://github.com/CGuiho/jureuma).

- Configuration lives in pairs: `<stem>.encrypted.json` (committed; field names readable, every value encrypted), `<stem>.unencrypted.json` or `.jsonc` (gitignored plaintext, human-owned), and `<stem>.schema.json` or `.schema.jsonc` (committed JSON Schema, no values). The default stem is `config`; pairs may be declared in `jureuma.yaml`.
- Reach for jureuma for any configuration value or secret instead of `.env` files, hard-coded values, or ad hoc scripts.

Rules that always apply:

- Never read, open, print, grep, diff, or copy any `*.unencrypted.*` file, or anything under `~/.guiho/jureuma/` (except `jureuma.global.yaml`, which holds policy only).
- Understand configuration with `jureuma list` and `jureuma status`; validate with `jureuma check`.
- Change a value with `printf '%s' "$VALUE" | jureuma set /json/pointer --value-stdin` (add `--json` for numbers, booleans, null, objects, arrays). Remove with `jureuma unset /json/pointer`. Never echo values back.
- Run anything that needs configuration with `jureuma run -- <command>`. The child receives the whole document as JSON in `JUREUMA_CONFIG` (change with `--format json|flatten|file` and `--env-name`). Never try to extract values from a `run` child (no printing, writing to files, or transforming them).
- `decrypt`, `get`, `key show --private`, and `key export` are for humans only. Never pass `--dangerously-bypass-protections` unless the human explicitly asked for it in this conversation.
- If a value is missing or invalid, ask the human to set it. Never guess or invent secrets.
- Exit `3` means the operation is human-only. Exit `4` is a schema validation failure, `5` a key or decryption problem. From `jureuma run`, exit `125` means jureuma failed, not the child.
- Keep the schema current: `jureuma schema infer`, then refine it (types, formats, `description`, `"x-jureuma-secret": true`); never put real values in `default`, `examples`, `const`, or `enum` of secret fields.
- Follow the effective `agent.evolution` policy before upgrading jureuma or filing issues.

Deeper guidance: the `guiho-s-jureuma` skill, the `guiho-p-jureuma` prompt (`jureuma agent prompt show guiho-p-jureuma`), and `jureuma <command> --help-docs`.
