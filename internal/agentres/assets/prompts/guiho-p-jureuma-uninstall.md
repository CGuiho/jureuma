---
name: guiho-p-jureuma-uninstall
purpose: Uninstall jureuma through its ownership-safe lifecycle interface.
description: Use when an agent must uninstall jureuma while preserving keys, configuration and data according to the user's choices.
created: 2026-10-13
version: "0.2.0"
owner: jureuma-prompts
flags: []
tags:
  - uninstallation
  - lifecycle
keywords:
  - jureuma
  - uninstall
  - preserve-config
  - preserve-data
  - keys
metadata:
  version: "0.2.0"
---

Uninstall jureuma, the GUIHO agent-safe configuration CLI (https://github.com/CGuiho/jureuma).

Warning to relay to the user before doing anything: by default, uninstalling removes every
jureuma-owned file, including `~/.guiho/jureuma/` (global configuration, state, audit log)
and the project's `jureuma.yaml`. Private keys in `~/.guiho/jureuma/keys/` are the only way to
decrypt the committed `*.encrypted.json` files; deleting the only copy makes every committed
secret permanently unreadable. jureuma therefore deletes `keys/` only after a human confirms
in an interactive terminal, and a non-interactive uninstall preserves `keys/`.

Steps:

1. Ask the user whether they have backed up their private keys. If not, tell them to run
   `jureuma key export` themselves (it is human-only; never run it for them and never read its
   output or anything under `~/.guiho/jureuma/`).
2. Ask the user what to keep:
   - `--preserve-config` keeps `jureuma.global.yaml` and the project's `jureuma.yaml`;
   - `--preserve-data` keeps persistent data (keys, state, audit log).
3. Show the plan first with a dry run, and present it to the user:
   jureuma uninstall --dry-run [--preserve-config] [--preserve-data]
4. After the user approves the displayed plan, run exactly one of:
   - with jureuma available:
     jureuma uninstall [--preserve-config] [--preserve-data]
   - Windows (PowerShell):
     irm https://raw.githubusercontent.com/CGuiho/jureuma/main/devops/uninstall.ps1 | iex
     (with options: & ([scriptblock]::Create((irm https://raw.githubusercontent.com/CGuiho/jureuma/main/devops/uninstall.ps1))) -PreserveConfig -PreserveData)
   - macOS or Linux (POSIX sh):
     curl -fsSL https://raw.githubusercontent.com/CGuiho/jureuma/main/devops/uninstall.sh | sh
     (with options: ... | sh -s -- --preserve-config --preserve-data)
   Pass `--yes` only when the user approved the plan and no interactive terminal is available;
   even then, `keys/` is preserved unless a human confirms in a terminal.
   Never pass `--dangerously-bypass-protections`.

What uninstall does not touch: the jureuma block in the project `.gitignore` (it keeps
`*.unencrypted.json` files out of git and stays on purpose), the `*.encrypted.json`,
`*.unencrypted.json`, and `*.schema.json` files, `AGENTS.md` content outside jureuma's managed
block (only the block is removed), the shared `~/.guiho/`, `~/.guiho/bin/`, and
`~/.guiho/.temp/` directories, and the `PATH` entry. The `juju` alias, the `guiho-s-jureuma`
skill copies, and agent-tool deny rules added by jureuma (Claude Code and OpenCode) are removed.

Finally, verify that `jureuma` is no longer found on the `PATH` (open a new shell if needed)
and report what was removed and what was preserved. Never read `*.unencrypted.*` files at any
point. If uninstallation fails, tell the user and, following their
`agent.evolution.issues.bugs` policy, offer to create an issue at
https://github.com/CGuiho/jureuma/issues/new without any secrets or configuration values;
give the user the issue URL after creating it.
