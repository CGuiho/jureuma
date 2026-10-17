---
name: guiho-p-jureuma
purpose: Set up jureuma in a project so its configuration and secrets are encrypted in git and safe for agents to use.
description: Use when an agent must install, initialize and verify jureuma for a project end to end.
created: 2026-10-13
version: "0.2.0"
owner: jureuma-prompts
flags: []
tags:
  - setup
  - lifecycle
keywords:
  - jureuma
  - install
  - init
  - encrypted configuration
metadata:
  version: "0.2.0"
---

#### &copy; 2026 [GUIHO](https://guiho.co) as represented by [Cristóvão GUIHO](https://guiho.co/cguiho) All Rights Reserved.

Set up jureuma, the GUIHO agent-safe configuration CLI (https://github.com/CGuiho/jureuma), for
this project, so configuration and secrets are encrypted in git and agents can use them without
ever reading them.

1. Install. If `jureuma --version` fails, install jureuma by loading the `guiho-p-jureuma-install`
   prompt (referenced in the README: https://github.com/CGuiho/jureuma#install) and following it.
   In short: `irm https://raw.githubusercontent.com/CGuiho/jureuma/main/devops/install.ps1 | iex`
   on Windows, `curl -fsSL https://raw.githubusercontent.com/CGuiho/jureuma/main/devops/install.sh | sh`
   on macOS and Linux. Never use a package manager.
2. Initialize. Run `jureuma init` in the project root. It installs the `guiho-s-jureuma` skill,
   applies the permanent jureuma block to `AGENTS.md`, creates `jureuma.yaml` and the global
   configuration, asks for the agent-evolution policy (explain `disabled`, `always-ask`, and
   `always-proceed`; `always-proceed` lets the agent keep jureuma upgraded and report issues),
   creates the default `config` pair and its key when none exists, adds the `.gitignore` block,
   and offers agent-tool deny rules for `*.unencrypted.*` in Claude Code and OpenCode settings
   (recommend accepting). If it needs a terminal you cannot provide, ask the user to run it.
3. Load the `guiho-s-jureuma` skill and follow its agent-safe rules for the rest of the task:
   never read `*.unencrypted.*` files or anything under `~/.guiho/jureuma/`; never pass
   `--dangerously-bypass-protections` unless the user asked for it in this conversation;
   `decrypt`, `get`, `key show --private`, and `key export` are for the user only.
4. Find the project's existing configuration (for example `.env` files, hard-coded settings, or
   CI variables). Do not open files that contain secret values. Instead, list the settings the
   application needs by reading the code that consumes them, and ask the user to move the values
   into `config.unencrypted.json` themselves and run `jureuma encrypt`, or to give you each
   value to set with `printf '%s' "$VALUE" | jureuma set /path --value-stdin`.
5. Build the schema: run `jureuma schema infer` (add `--force` only to replace the starter schema
   `init` created), then refine `config.schema.json` (or `.schema.jsonc`) following the
   "Writing the schema" guide in the `guiho-s-jureuma` skill: precise types, constraints, the
   supported formats, a `description` on every property, and `"x-jureuma-secret": true` on every
   secret. Never put real values in `default`, `examples`, `const`, or `enum`.
6. Wire the application: make it read the whole configuration from the `JUREUMA_`
   environment variable (JSON), and start it through `jureuma run -- <command>` (`inject` is an
   alias of `run`) in scripts,
   task runners, and documentation. Use `--format flatten` (with `--prefix`) or `--format file`
   only when the application needs individual variables or a file path; `--env-name` renames the
   variable.
7. Verify: `jureuma check`, `jureuma status`, and `jureuma list` must succeed (on a machine
   without the private key, `key-unavailable` and `unencrypted-missing` are normal). Confirm
   with `git status` that no `*.unencrypted.*` file is staged or tracked, and that
   `config.encrypted.json` and `config.schema.json` are ready to commit.
8. For CI and production, tell the user to store the private key as the secret environment
   variable `JUREUMA_PRIVATE_KEY_<KEYID>` (the name is shown by `jureuma key show`); the user
   obtains the key with `jureuma key export` themselves.
9. Report what you set up, the files created, and anything the user still has to do.
   Optionally offer `jureuma juju-enable` for the short `juju` alias.

Follow the effective `agent.evolution` policy for upgrades and GitHub issues as described in the
skill's "CLI Evolution and Feedback" section, and give the user the URL of any issue you create.
