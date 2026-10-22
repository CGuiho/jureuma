---
name: guiho-p-jureuma-install
purpose: Install the jureuma CLI without initializing a project.
description: Use when an agent must install and verify jureuma while keeping project initialization as a separate explicit task.
created: 2026-10-13
version: "0.3.1"
owner: jureuma-prompts
flags: []
tags:
  - installation
  - lifecycle
keywords:
  - jureuma
  - install
  - checksums
  - stable launcher
metadata:
  version: "0.3.1"
---

Install jureuma, the GUIHO agent-safe configuration CLI (https://github.com/CGuiho/jureuma).
jureuma keeps configuration and secrets in a committed encrypted JSON file, a gitignored
plaintext JSON file, and a committed JSON Schema, and lets agents set values and inject them
into processes without ever reading them.

Install it by running exactly one command, chosen by the host operating system:

- Windows (PowerShell):
  irm https://raw.githubusercontent.com/CGuiho/jureuma/main/devops/install.ps1 | iex
- macOS or Linux (POSIX sh):
  curl -fsSL https://raw.githubusercontent.com/CGuiho/jureuma/main/devops/install.sh | sh

To install an exact version or a release channel, add --version 'X.Y.Z' or --channel '<name>'
to the install.sh command, or -Version 'X.Y.Z' or -Channel '<name>' to the install.ps1 command:

- macOS or Linux:
  curl -fsSL https://raw.githubusercontent.com/CGuiho/jureuma/main/devops/install.sh | sh -s -- --version 'X.Y.Z'
  curl -fsSL https://raw.githubusercontent.com/CGuiho/jureuma/main/devops/install.sh | sh -s -- --channel 'canary'
- Windows (PowerShell):
  & ([scriptblock]::Create((irm https://raw.githubusercontent.com/CGuiho/jureuma/main/devops/install.ps1))) -Version 'X.Y.Z'
  & ([scriptblock]::Create((irm https://raw.githubusercontent.com/CGuiho/jureuma/main/devops/install.ps1))) -Channel 'canary'

Never install jureuma through a package manager (npm, Bun, pip, Homebrew, or similar); no such
package exists or is supported.

When the installer finishes, verify with `jureuma --version` (it prints a raw SemVer such as
`0.3.1`). If the command is not found, open a new shell or follow the PATH instruction the
installer printed.

If the installer fails, fall back to a manual binary installation:
1. Download the asset matching the host platform from
   https://github.com/CGuiho/jureuma/releases/latest - for example
   `jureuma-windows-amd64.exe` or `jureuma-linux-amd64` - and verify its SHA-256
   digest against the release's `checksums.txt`.
2. Place the binary at `$HOME/.guiho/bin/jureuma` (`jureuma.exe` on Windows),
   creating the directory when needed.
3. On macOS and Linux, mark it executable with `chmod +x`. Ensure `$HOME/.guiho/bin`
   is on the `PATH`, then verify with `jureuma --version`.

If installation fails, explain the failure to the user. Following the user's jureuma
`agent.evolution.issues.bugs` policy (default `always-ask`: ask first and wait for approval),
create an issue at https://github.com/CGuiho/jureuma/issues/new describing the operating
system, architecture, the exact install command, the requested version or channel, and the
error output. Include no secrets, tokens, or configuration values. After creating the issue,
give the user its URL.

Finish by running `jureuma init` in the project root. It installs the `guiho-s-jureuma` agent
skill, applies the permanent jureuma instruction to `AGENTS.md` so that every future agent
knows how to use jureuma safely, creates the configuration files, and asks the user for the
agent-evolution policy. If it needs answers and no interactive terminal is available, ask the
user to run `jureuma init` themselves. Then verify again with `jureuma --version`.

From now on, never read any `*.unencrypted.*` file or anything under `~/.guiho/jureuma/`, and
never pass `--dangerously-bypass-protections` unless the user explicitly asks for it.
