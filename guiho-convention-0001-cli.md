---
name: GUIHO CLI Convention
purpose: Define mandatory conventions for every GUIHO command-line application.
description: Establishes the shared technology, installation, configuration, and engineering standards for GUIHO CLIs.
created: 2026-08-16
owner: docs-conventions
flags: []
tags:
  - cli
  - conventions
  - go
keywords:
  - GUIHO CLI convention
  - Go
  - Cobra
  - cliname
  - CLI flags
  - list-valued flags
  - default flags
  - help tree
  - SemVer
  - kebab-case
  - required commands
  - mandatory init
  - interactive initialization
  - configuration bootstrap
  - AI-managed CLI
  - CLI evolution
  - always-ask
  - always-proceed
  - GitHub issue reporting
  - upgrade policy
  - agent artifacts
  - agent skills
  - agent prompts
  - agent instructions
  - permanent instruction
  - one-time prompt
  - configuration files
  - configuration schemas
  - remote JSON Schema
  - GitHub release schemas
  - CLI home directory
  - .guiho
  - shared binary directory
  - install.sh
  - install.ps1
  - uninstall.sh
  - uninstall.ps1
  - reinstall as upgrade
  - release channels
  - SemVer prerelease
  - stable
  - canary
  - alpha
  - beta
  - nightly
  - persistent data
  - database preservation
  - preserve-config
  - preserve-data
  - installer staging
  - .guiho/.temp
  - SHA-256 verification
  - installation rollback
  - synchronous upgrade
  - stable launcher
  - immutable versioned binaries
  - atomic activation pointer
  - reinstall recovery command
  - release artifact manifest
  - full agent artifact replacement
  - filesystem ownership boundary
  - safe uninstall
  - PATH
  - cross-platform paths
  - installation artifacts
  - startup update check
  - update cache
  - background update worker
  - worker lease
  - single-flight update check
  - technology stack
  - mandatory project tooling
  - Mirror versioning
  - RunX command catalog
  - XDocs directory documentation
---

#### &copy; 2026 [GUIHO](https://guiho.co) as represented by [Cristóvão GUIHO](https://guiho.co/cguiho) All Rights Reserved.

# GUIHO CLI Convention

By: **Cristóvão GUIHO <cg@guiho.co>**

## Placeholder CLI Name

All CLI command examples in this convention must use `cliname` as the generic placeholder for the CLI executable name.

## Technology Stack

- Go programming language, using the latest version available when the project is created.
- Cobra, using the latest version available when the project is created, as the CLI builder.
- Go’s standard library wherever practical.
- Go’s native testing, formatting, vetting, and build tools.

## Mandatory Project Tooling

Every GUIHO CLI project must use Mirror, RunX, and XDocs from project creation through maintenance and release. These tools are mandatory and must not be replaced with ad hoc versioning, undocumented shell commands, package-manager scripts, or disconnected documentation systems.

Every CLI repository must contain these root files:

```text
mirror.yaml
runx.yaml
xdocs.yaml
```

### Mirror Project Versioning

Every GUIHO CLI project must use Mirror as the sole authority for semantic project versioning. The root `mirror.yaml` file must describe every version-bearing project file and the repository's canonical version, changelog, commit, tag, and push behavior.

Agents and developers must use Mirror to plan and apply every version change. They must not manually edit version fields, create release tags by hand, or substitute `npm version`, another package manager, or a custom version script for Mirror.

Before a version change, run the project's required tests and validation, then run:

```text
mirror config check
mirror version plan <target>
```

Inspect the complete plan before applying it. `mirror version apply <target> --yes` may run only when the resulting version mutations, release commit, tag, and push are explicitly authorized. Building or planning a release does not provide that authorization.

### RunX Command Catalog

Every GUIHO CLI project must use RunX to catalog every supported project command in a root `runx.yaml` manifest. This includes development, formatting, testing, vetting, building, cross-building, documentation, schema generation, installation, upgrade validation, packaging, release preparation, and every other repeatable command intended for developers, agents, automation, or CI.

The catalog must use the current RunX manifest schema with one top-level `namespace`. Every command leaf must have a stable global `uid`, a scoped `id`, an exact stored command, and an accurate description. UIDs must remain stable and must never be reused for materially different behavior.

A supported project workflow must not exist only as an undocumented README snippet, package-manager script, CI step, shell history entry, or agent instruction. Its executable command must also be represented in `runx.yaml`. Secrets and secret values must never be stored in the catalog.

After every catalog change, run:

```text
runx check --format json
runx list --format json
```

Agents must use RunX discovery before executing an unfamiliar project operation and must run `runx run --dry-run <uid-or-selector>` before a mutating or high-impact command.

### XDocs Directory Documentation

Every GUIHO CLI project must use XDocs to document every project-owned directory. The repository must contain a root `xdocs.yaml`, one root `XDOCS.md` index, and exactly one correctly named `*.xdocs.md` descriptor in every owned, non-excluded directory.

Each descriptor must document its directory's purpose, parent, children, owned files, companion documents, tags, keywords, flags, and status. Parent and child relationships must agree throughout the complete XDocs tree. Every non-excluded companion Markdown document must be declared by the owning descriptor.

Generated output, vendored dependencies, build output, and other directories that the project does not own may be excluded explicitly through `xdocs.yaml` and applicable `.gitignore` rules. Source, command, internal package, configuration, devops, installer, schema, skill, prompt, agent, test, and documentation directories are project-owned and must not be excluded merely to avoid documenting them.

Whenever a directory, file, command, artifact, or behavior changes, its owning descriptor and affected parent or child links must be updated in the same work unit. Validate documentation changes with:

```text
xdocs meta <scope> --documents --strict
xdocs tree
xdocs doctor <scope>
```

A GUIHO CLI project does not satisfy this convention unless `mirror config check`, `runx check --format json`, strict XDocs metadata validation, the XDocs tree, and `xdocs doctor` all pass for the applicable project state.

## Flags

- Flags must always be written in their full long form.
- Short aliases must not be created unless the user explicitly requests them.
- The only standard short aliases are `-v` for `--version` and `-h` for `--help`.
- Multiword flag names must use lowercase words separated by hyphens, such as `--help-tree`.
- A flag value must be accepted after a space or an equals sign.
- Argument quoting is the responsibility of the invoking shell. The CLI and Cobra must not implement additional quote parsing or quote removal.

The following value forms must be supported:

```text
cliname --key value
cliname --key=value
```

For a flag that receives a list, repeat the full flag once for each value. Implement list-valued flags with Cobra `StringArray`; do not use comma-separated values by default. Preserve the order in which the values are provided.

```text
cliname --key value-one --key value-two
cliname --key=value-one --key=value-two
```

## Default Flags

The following flags are mandatory for every GUIHO CLI.

### Top-Level Version Flag

#### `-v`, `--version`

The version flag is available only at the top level. It must print only the raw SemVer-compatible version and nothing else.

```text
1.2.3
```

The output must not include the CLI name, a `v` prefix, labels, explanations, update notices, ANSI formatting, or any other surrounding text.

### Flags Required on Every Command

Every command and subcommand must support the following flags.

#### `-h`, `--help`

Show help for the current command and nothing else.

#### `--help-docs`

Print Markdown documentation for the current command.

#### `--help-tree`

Print the complete command tree starting at the current command and continuing through every descendant. The tree must show every command, every subcommand, and every flag supported by each command and subcommand.

The command tree has no fixed maximum depth. By default, it must continue until every leaf command has been shown.

#### `--help-tree-depth`

Control the depth printed by `--help-tree`, starting from the current command. The default value is `max`. It may be set to `max` or to any integer greater than `1`.

```text
cliname --help-tree --help-tree-depth 2
cliname --help-tree --help-tree-depth=2
```

#### `--help-tree-global-flags`

Control whether inherited global flags are repeated under every command and subcommand in the help tree. This is a presence-only Boolean flag and its default value is `false`.

- When absent, global flags must be shown once at the beginning of the tree and hidden from descendant commands to keep the output concise.
- When present, every applicable global flag must be shown under every command and subcommand.

```text
cliname --help-tree --help-tree-global-flags
```

## Installation

### Mandatory Lifecycle Scripts

Every GUIHO CLI must provide these four lifecycle scripts:

```text
devops/install.sh
devops/install.ps1
devops/uninstall.sh
devops/uninstall.ps1
```

- `install.sh` installs the CLI on Linux and macOS.
- `install.ps1` installs the CLI on Windows through PowerShell.
- `uninstall.sh` uninstalls the CLI on Linux and macOS.
- `uninstall.ps1` uninstalls the CLI on Windows through PowerShell.

The two installer scripts must provide the same installation outcome, and the two uninstaller scripts must provide the same uninstallation outcome. Platform-specific implementation details may differ, but artifact scope, preservation behavior, verification, ownership boundaries, and failure behavior must remain equivalent.

The CLI must not provide an `install` command. Installation and installer-driven upgrades are owned exclusively by `install.sh` and `install.ps1`; therefore, `install` must not be added to the Cobra command tree. The mandatory Cobra `uninstall` command must use the same ownership, preservation, and safety contract as `uninstall.sh` and `uninstall.ps1`.

Each CLI project's `README.md` must document exactly three install commands in its installation section:

1. One single command for Windows.
2. One single command for macOS and Linux.
3. One small prompt for the AI that only references the CLI's install prompt file, named `<cliname>-p-install `. The prompt must instruct the agent to load the `<cliname>-p-install ` prompt and follow it in order to install the CLI. It must not duplicate the prompt's contents; the complete installation steps live only in the prompt file itself.

The installation section must also provide a final `cliname --version` verification command.

The README must contain a `## Uninstall` section directly after its installation section; it does not need to be the final section. Its uninstallation commands must mirror the installation section's structure with exactly three uninstall commands:

1. One single command for Windows.
2. One single command for macOS and Linux.
3. One small prompt for the AI that only references the CLI's uninstall prompt file, named `<cliname>-uninstall`. As with installation, it must instruct the agent to load and follow that prompt and must not duplicate its contents.

The uninstallation section must also show the destructive default command, a dry run, and a preservation example that combines configuration and data preservation, clearly state that uninstallation removes all CLI-owned data by default, and show the options for preserving configuration and persistent data. Repository-specific lifecycle-script URLs belong in the CLI project's README and must not be hardcoded in this generic convention.

### Release Versions and Channels

Both installer scripts must support installation by exact version or release channel.

- `install.sh` must accept `--version` and `--channel`.
- `install.ps1` must accept the equivalent `-Version` and `-Channel` parameters.
- Version and channel inputs must use their full names and must not have short aliases.
- Exact version and channel selection are mutually exclusive.

A channel is the first dot-separated identifier in a SemVer prerelease component. For example:

| Version | Channel |
| --- | --- |
| `1.2.3` | `stable` |
| `1.2.3-canary.4` | `canary` |
| `1.2.3-alpha.2` | `alpha` |
| `1.2.3-beta.1` | `beta` |
| `1.2.3-nightly.20260816` | `nightly` |

`stable` is the reserved channel for versions without a prerelease component. `canary`, `alpha`, `beta`, and `nightly` are conventional prerelease channels, but a CLI may publish another valid SemVer prerelease identifier.

Release selection must follow these rules:

1. An exact version installs that exact valid SemVer, including its complete prerelease component when present.
2. A channel installs the highest published valid SemVer whose channel exactly matches the requested channel.
3. No version and no channel means the latest stable release.
4. `--channel stable` or `-Channel stable` means the latest release without a prerelease component.
5. Draft releases, malformed versions, releases missing the required target asset, releases missing any mandatory agent artifact, and releases missing valid checksums must not be selected.
6. Channel discovery must inspect every page of the release catalog before selecting a version. It must not assume that GitHub's latest-release endpoint or the first visible page contains the latest matching prerelease.
7. If no compatible release exists for the requested version, channel, operating system, and architecture, the installer must fail without changing the existing installation.

### Complete Release Artifact Set

Every release must be a complete, self-contained installation unit. An install or upgrade must never combine a binary from one release with an agent resource, prompt, instruction, schema, example, or metadata file from another release or from a mutable branch.

Every release must publish:

- every supported native CLI binary;
- every launcher artifact required for a fresh installation;
- every agent skill bundled with the CLI;
- every agent prompt bundled with the CLI;
- every agent definition bundled with the CLI;
- the complete agent instruction source;
- every configuration schema and complete configuration example;
- an `artifacts.json` ownership manifest; and
- `checksums.txt` covering every published installation artifact except the checksum file itself.

An agent resource may be published as an individual release asset or inside a checksummed archive, but every contained path, artifact ID, version, checksum, canonical installed path, managed projection, and ownership boundary must be declared in `artifacts.json`. The manifest is the authority for safely identifying artifacts that belong to the CLI, replacing current artifacts, removing artifacts retired by the new release, and rolling back a failed transaction.

The installed copy of this manifest must be stored under `$HOME/.guiho/cliname/`. Install, upgrade, repair, and uninstall operations must use the installed manifest and the selected release manifest instead of guessing ownership from filenames or prefixes.

### Installer Behavior

Both installer scripts must perform the following sequence:

1. Resolve the current user's platform-native home directory.
2. Resolve the requested exact version or channel according to the release-selection contract. Default to the latest stable release.
3. Detect the operating system and architecture and reject unsupported targets before changing the installation.
4. Select the exact released launcher and versioned CLI payload for the detected target.
5. Display the resolved version, operating system, architecture, release asset, source URL, and destination before replacement begins.
6. Create a unique CLI-owned staging directory under `$HOME/.guiho/.temp/`.
7. Download the selected launcher, CLI payload, `artifacts.json`, `checksums.txt`, every agent skill, every agent prompt, every agent definition, the agent instruction, the schemas, the examples, and every other artifact declared by the release into that staging directory.
8. Verify the SHA-256 digest of every downloaded artifact listed in `checksums.txt` before installing or applying it. A missing, duplicated, malformed, or mismatched checksum must stop installation.
9. Run the verified candidate payload with `--version` and require its raw output to exactly match the resolved SemVer. Run its hidden installation self-test before activation.
10. Install the payload and its complete canonical artifact set in an immutable version directory under `$HOME/.guiho/cliname/versions/<version>/`.
11. Install or repair the stable launcher at the canonical command path under `$HOME/.guiho/bin/`. A launcher update must remain backward compatible with every installed payload that may be selected for rollback.
12. Store the installed `artifacts.json`, agent instruction, agent skills, agent prompts, agent definitions, schemas, examples, upgrade metadata, and other installed artifacts under the version directory and the CLI-owned home directory as declared by the manifest.
13. Delete or atomically replace every previous installed projection of the CLI's agent skills, prompts, definitions, and instruction. Install the complete new set and remove retired artifacts declared only by the previous installed manifest.
14. Install managed copies of every required agent skill into all supported global agent-skill destinations. Each copy must remain traceable to the selected release artifact in the CLI home directory.
15. Apply or upgrade the CLI's bounded instruction block in the current project's `AGENTS.md` without modifying content outside the managed block.
16. Add `$HOME/.guiho/bin/` to the current user's `PATH` when absent. The update must be user-level, idempotent, and must not require administrator or root privileges.
17. Atomically activate the selected version, run the stable launcher with `--version`, and require its raw output to exactly match the resolved SemVer.
18. Remove the CLI-owned staging directory and obsolete artifact backups only after the launcher, payload, agent resources, and managed projections have all been verified.
19. Print the installed version, launcher path, active payload path, CLI home path, installed agent resources, and any shell-reload instruction needed for the updated `PATH`.

An installer must fail with a nonzero exit status when any required step fails. It must restore the previous launcher, active payload selection, and every replaced CLI-owned artifact, remove partial candidates, and leave the previous working installation usable. It must never apply an unverified artifact.

Re-running an installer for the installed version must be safe and idempotent. It may repair missing or outdated CLI-owned artifacts, managed agent-skill copies, the managed instruction block, or the user `PATH` entry without creating duplicates.

### Reinstallation and Installer-Driven Upgrades

Running `install.sh` or `install.ps1` when the CLI is already installed must perform a complete controlled reinstall. It must not stop merely because the target version is already installed. Installing a newer version through either installer is an installer-driven upgrade; installing an explicitly requested older version installs that exact version.

Before replacing an existing installation, every CLI must classify each CLI-owned path as either persistent or disposable. Databases and user-created application data are always persistent. Cache files are always disposable.

An installer-driven reinstall or upgrade must preserve:

- `$HOME/.guiho/cliname/cliname.global.yaml`;
- the current project's `cliname.yaml`;
- every declared persistent-data path owned by the CLI;
- every CLI-owned database; and
- any other user-created content that the CLI explicitly classifies as persistent.

It must remove or replace:

- the stable launcher when the installer carries a compatible repair or replacement;
- the active versioned payload and every obsolete payload not retained by the rollback policy;
- every CLI-owned cache file;
- disposable runtime state;
- stale upgrade metadata;
- the canonical bundled instruction, skills, prompts, agent definitions, schemas, examples, and metadata that will be replaced by the selected release;
- every managed copy or projection of those agent artifacts; and
- the CLI's managed instruction block, which must then be reapplied from the selected release.

The installer must never recursively erase `$HOME/.guiho/cliname/` as its reinstall strategy. It must remove only paths owned by the installed artifact manifest or explicitly declared disposable paths, leaving persistent paths untouched. Removal and replacement must be transactional: if installation fails, the previous launcher selection, payload, instruction, skills, prompts, agent definitions, schemas, managed projections, and other replaceable artifacts must be restored, while preserved configuration, data, and databases remain unchanged. Cache files do not need to be restored.

After a successful reinstall or installer-driven upgrade, the managing AI agent must run `cliname init`, complete any required interactive reconciliation, and verify the raw output of `cliname --version`.

### Uninstaller Behavior

`uninstall.sh`, `uninstall.ps1`, and `cliname uninstall` must share one uninstallation contract. By default, uninstallation removes all artifacts owned by the CLI, including:

- the stable launcher and every installed versioned payload;
- the complete `$HOME/.guiho/cliname/` directory, including global configuration, persistent data, databases, caches, and state;
- every globally installed managed copy of the CLI's agent skills;
- the CLI's managed instruction block from the current project;
- the current project's `cliname.yaml`; and
- temporary directories that are verifiably owned by the CLI's current uninstall operation.

The uninstall interfaces must support these full-name options:

| Option | Behavior |
| --- | --- |
| `--preserve-config` | Preserve `cliname.global.yaml` and the current project's `cliname.yaml`. |
| `--preserve-data` | Preserve every declared persistent-data path and every CLI-owned database. It does not preserve configuration unless `--preserve-config` is also present. |
| `--dry-run` | Print the exact removal and preservation plan without changing anything. |
| `--yes` | Confirm the displayed plan without an interactive confirmation prompt. |

`uninstall.ps1` must expose equivalent PowerShell parameters. The README must show the destructive default command, a dry run, and a preservation example that combines configuration and data preservation.

Before removal, the uninstaller must display every resolved target grouped as `REMOVE` or `PRESERVE`. Without `--yes`, it must request confirmation in an interactive terminal. A noninteractive invocation without `--yes` must fail without changing anything.

Preservation options must not preserve caches, replaceable agent resources, the launcher, versioned payloads, or temporary installation artifacts. When preservation leaves files inside `$HOME/.guiho/cliname/`, the uninstaller must preserve the CLI home directory and remove only the non-preserved children.

Uninstallation must preserve the shared `$HOME/.guiho/`, `$HOME/.guiho/bin/`, and `$HOME/.guiho/.temp/` directories and the shared user `PATH` entry. It must never remove another CLI's files or managed instruction blocks. It must remove only its bounded block from `AGENTS.md` and must not delete `AGENTS.md` itself.

### Temporary Installation Directory

All installation downloads, candidates, backups, archive extraction, and verification work must occur under the shared GUIHO temporary directory:

```text
$HOME/.guiho/.temp/
```

Each installation must create a unique child directory whose name begins with the confirmed CLI name, for example:

```text
$HOME/.guiho/.temp/cliname-install-<unique-id>/
```

The installer may create `$HOME/.guiho/.temp/` when it does not exist, but it does not own that shared directory. It may create, modify, and delete only its unique child directory. It must never inspect for cleanup, overwrite, or remove another CLI's temporary directory, and it must never recursively delete `$HOME/.guiho/.temp/` itself.

The installer must validate that its resolved staging path is a strict descendant of `$HOME/.guiho/.temp/` before performing any recursive cleanup. It must attempt to remove its staging directory after both success and failure.

### Binary Installation Directory

Every GUIHO CLI command entry point must be installed in the shared `bin/` directory under the current user's `.guiho/` directory:

```text
$HOME/.guiho/bin/
```

The installed command path is therefore `$HOME/.guiho/bin/cliname` on macOS and Linux and `$HOME/.guiho/bin/cliname.exe` on Windows. This path must contain the stable launcher, not the versioned application payload.

The installer must add the platform-resolved `$HOME/.guiho/bin/` directory to the current user's `PATH`. The operation must be idempotent: the installer must not add the directory again when it is already present. It must modify the user-level environment only and must not require a system-wide `PATH` change.

The implementation must resolve the native user home with Go's `os.UserHomeDir()` and construct the binary-directory path with `filepath.Join(home, ".guiho", "bin")`.

| Environment | Example binary directory |
| --- | --- |
| Windows | `C:\Users\<user>\.guiho\bin\` |
| Git Bash on Windows | `/c/Users/<user>/.guiho/bin/` |
| macOS | `/Users/<user>/.guiho/bin/` |
| Linux | `/home/<user>/.guiho/bin/` |

For example, the shared binary directory on the reference Windows machine is:

```text
/c/Users/guiho/.guiho/bin/
```

### Stable Launcher and Versioned Payloads

Every GUIHO CLI must use a stable launcher with immutable, versioned payloads. The launcher is the process invoked from the user's `PATH`; it resolves the active version, starts that payload with the original arguments and standard streams, waits for it, and returns the payload's exact exit code.

The conventional layout is:

```text
$HOME/.guiho/bin/cliname
$HOME/.guiho/bin/cliname.exe
$HOME/.guiho/cliname/current.json
$HOME/.guiho/cliname/installed-artifacts.json
$HOME/.guiho/cliname/versions/1.2.3/cliname
$HOME/.guiho/cliname/versions/1.2.3/cliname.exe
$HOME/.guiho/cliname/versions/1.2.3/artifacts/
```

Only the platform-appropriate launcher and payload exist on a particular installation. `current.json` must be a strictly decoded, atomically replaced pointer to a relative path inside the CLI-owned versions directory. It must record the active version and the immediately previous verified version. It must never accept an absolute path, path traversal, or a target outside `$HOME/.guiho/cliname/versions/`.

A released payload must remain compatible with the stable launcher protocol. Ordinary `cliname upgrade` operations must not require the running launcher to replace itself. Installer-driven repair may replace the launcher because the external installer owns that transaction, but launcher changes must remain backward compatible with the retained rollback payload.

The launcher and payload must not treat activation as background work. The shell must remain attached until the payload command completes. A successful upgrade must therefore be visible to the very next invocation without waiting for a detached helper, another terminal, a reboot, or a later completion journal.

### Shared `.guiho/` Ownership Boundary

`$HOME/.guiho/` is the global home for everything related to GUIHO. It is shared infrastructure and is not owned by any individual CLI.

The `cliname` CLI may manage only these installed locations:

```text
$HOME/.guiho/cliname/
$HOME/.guiho/bin/cliname
$HOME/.guiho/bin/cliname.exe
$HOME/.guiho/.temp/cliname-<operation>-<unique-id>/
```

The extensionless binary applies to macOS and Linux. The `.exe` binary applies to Windows.

The CLI must not create, modify, move, overwrite, or delete:

- another CLI's directory under `$HOME/.guiho/`;
- another executable or file under `$HOME/.guiho/bin/`;
- another operation's directory under `$HOME/.guiho/.temp/`;
- the shared `$HOME/.guiho/` directory; or
- the shared `$HOME/.guiho/bin/` or `$HOME/.guiho/.temp/` directory.

Install, upgrade, and uninstall operations must resolve and validate the exact platform-native target before making a filesystem change. Recursive operations may target only the CLI-owned `$HOME/.guiho/cliname/` directory and must never target either shared parent directory.

Adding `$HOME/.guiho/bin/` to the user's `PATH` is the only shared-environment mutation permitted during installation. Uninstalling one CLI must not remove this shared `PATH` entry because other GUIHO executables may depend on it.

### CLI Home Directory

The user must confirm the CLI home-directory name. The directory must be located under the current user's platform-specific home directory, inside `.guiho/`. In this convention, `$HOME` represents the resolved user home and `cliname/` represents the confirmed CLI directory name:

```text
$HOME/.guiho/cliname/
```

The implementation must resolve the native user home with Go's `os.UserHomeDir()` and construct paths with `filepath.Join`. It must not assume Unix path syntax on Windows.

| Platform | Example CLI home directory |
| --- | --- |
| Windows | `C:\Users\<user>\.guiho\cliname\` |
| macOS | `/Users/<user>/.guiho/cliname/` |
| Linux | `/home/<user>/.guiho/cliname/` |

The `.guiho/` directory is shared by GUIHO CLIs. Each CLI must use only its own user-confirmed directory, which will normally use the CLI name.

Except for the stable launcher, every artifact installed or persisted by the CLI must have its canonical installed copy inside the CLI home directory. This includes every versioned CLI payload, the global configuration, cache and state files, agent instruction source, agent skills, agent prompts, agent definitions, schemas, examples, artifact manifests, upgrade metadata, and other CLI-owned artifacts. The stable launcher's canonical installed location is the shared `$HOME/.guiho/bin/` directory.

When an artifact must be applied to a project or exposed to an agent tool, the CLI may create a managed copy or projection at the required destination. The canonical installed artifact must remain inside the CLI home directory.

## Startup Update Check

Every GUIHO CLI must perform a background update-availability check when invoked by its bare name with no arguments and no flags:

```text
cliname
```

The check must follow a cached, single-flight design. The foreground invocation must never block on the network, and the mechanism must never spawn unbounded processes. Historical incidents in which repeated checks consumed all CPU and memory are explicitly forbidden by this convention.

Reference implementations of the complete behavior exist in the RunX, Mirror, and XDocs repositories and must be consulted before implementing this section.

### Foreground Behavior

A bare invocation must, in order:

1. Read the update cache from the CLI home directory using local filesystem access only.
2. Print the update notice when the cache qualifies, as defined below.
3. Attempt to launch at most one background update worker when the cache is missing, expired, or invalid.

The update check must not alter the output contracts of `-v/--version`, `-h/--help`, `--help-docs`, `--help-tree`, or any explicit subcommand. Only the bare invocation prints the notice. The notice must be written to standard error and must use this two-line form:

```text
⚠ New version available: v0.12.1
  run cliname upgrade to update
```

The notice must be printed only when all of the following hold:

- the cache is valid and within its time-to-live;
- the cache records a new-version-available result; and
- the cached latest version is strictly greater than the currently installed version according to SemVer comparison.

In every other case — expired cache, corrupt cache, equal or older cached version, or missing cache — nothing may be printed.

### Background Update Worker

The worker is the only component permitted to touch the network. It must be the same executable re-invoking itself through a hidden internal entry point, launched as a detached process whose standard input, output, and error streams are disconnected and which is fully released by the parent. The parent must never wait for it.

Before spawning, the foreground process must attempt to acquire a worker lease. All of the following rules are mandatory:

- An environment variable marks worker executions. A process already running as the worker must never spawn another worker. Recursion is forbidden.
- The lease is a lock file or directory inside the CLI home directory, created through an atomic exclusive operation and containing a random ownership token and the owning process ID.
- If a valid lease already exists and is younger than the maximum lease age, the invocation must skip spawning entirely. Only one worker may exist at any time across all concurrent sessions and terminals.
- A lease older than the maximum lease age is considered abandoned and may be broken by a new worker.
- The worker must enforce a hard execution time limit on its release-catalog request. A slow or unreachable network must terminate the worker, not accumulate work.
- Every failure path — spawn failure, lease failure, fetch failure, timeout, parse failure — must be silent in the foreground, must leave the previous cache untouched, and must never change the exit code of the invoking command.

After fetching the release catalog, the worker resolves the latest applicable version, compares it against the running version with SemVer semantics, records the result in the cache, and releases the lease.

### Update Cache File

The cache is a small disposable JSON file stored inside the CLI home directory alongside its other cache files:

```text
$HOME/.guiho/cliname/cache.json
```

It must record at minimum:

| Field | Meaning |
| --- | --- |
| `latestVersion` | Latest published version found by the last successful check |
| `currentVersion` | Installed version at the time of the check |
| `newVersionAvailable` | Whether `latestVersion` is greater than `currentVersion` |
| `lastCheck` | UTC timestamp of the last completed check |
| `upgradeCommand` | The exact command the user should run to upgrade |

Additional rules:

- Reading the cache is the only update-check work performed in the foreground.
- The cache must have a defined time-to-live. While it is fresh, invocations must not spawn another worker.
- The cache must be written atomically through a temporary file and rename, never truncated in place.
- A missing, corrupt, malformed, or schema-invalid cache must be treated as a cache miss, never as a fatal error.
- The cache is disposable state. Uninstallation must remove it, and reinstalls may reset it.

## Configuration Files

Every GUIHO CLI must support both global and project configuration. The `init` command must ensure that both configuration files exist and are valid.

The CLI must distinguish between these files:

- `cliname.yaml` is the project configuration file and must be stored in the project root.
- `cliname.global.yaml` is the global configuration file and must be stored inside the platform-resolved CLI home directory. The conventional example path is `$HOME/.guiho/cliname/cliname.global.yaml`.

The project configuration and global configuration must have different filenames and separate schemas. One must not be treated as a differently located copy of the other.

The global configuration provides the user-wide baseline. The project configuration overrides matching global values for the current project. A value omitted from the project configuration inherits the global value.

### Agent Evolution Policy

Every CLI configuration contract must include an agent-evolution policy. The policy controls whether an AI agent may upgrade the CLI or create GitHub issues for bugs, improvements, and reviews.

```yaml
agent:
  evolution:
    upgrade: always-ask
    issues:
      bugs: always-ask
      improvements: always-ask
      reviews: always-ask
```

Each policy field must accept exactly one of these string values:

| Value | Required agent behavior |
| --- | --- |
| `disabled` | Do not perform the governed external action and do not ask for permission to perform it. The agent may still explain the observation to the user. |
| `always-ask` | Explain the proposed action, ask the user for permission, and wait for approval before acting. |
| `always-proceed` | Perform the governed action without requesting additional permission, then report the result to the user. |

These settings must not be represented as Booleans. Unknown values must fail configuration validation. Every field defaults to `always-ask` when `init` creates the configuration or when a valid inherited value is not available.

The policy belongs in `cliname.global.yaml` as the user-wide default. `cliname.yaml` may override any individual field for the current project.

#### Upgrade Policy

An AI agent may run `cliname upgrade check` while using or preparing to use the CLI, subject to `agent.evolution.upgrade`:

- `disabled`: do not check for or perform an upgrade on the user's behalf.
- `always-ask`: when an upgrade is available, tell the user that a newer version is available, state the target version, ask whether to upgrade, and wait. Upgrade only after approval.
- `always-proceed`: when an upgrade is available, perform the upgrade, run `cliname init` to reconcile the installation, verify `cliname --version`, and report the result.

#### GitHub Issue Policy

The fields under `agent.evolution.issues` independently govern GitHub issue creation:

- `bugs` governs behavior that appears incorrect or broken.
- `improvements` governs behavior that could be made more useful, reliable, safe, clear, or efficient.
- `reviews` governs other actionable review findings or feedback about the CLI.

For `always-ask`, the agent must explain the proposed issue and ask a category-appropriate question such as:

```text
I found something that appears to be a bug. Do you allow me to create an issue in the CLI's GitHub repository?
```

For `always-proceed`, the agent may create the issue without another prompt. After creating any issue, the agent must provide the user with the direct URL of the created GitHub issue. The agent must never claim that an issue was created unless creation succeeded and a URL was returned.

### Configuration Schemas

Every YAML configuration file supported by the CLI must have a corresponding JSON Schema and a complete valid example. Every CLI must provide:

| Configuration | Schema |
| --- | --- |
| `cliname.yaml` | `cliname.schema.json` |
| `cliname.global.yaml` | `cliname.global.schema.json` |

The schemas must be published remotely with every CLI release. Each generated configuration file must reference the schema for its exact CLI release through a version-pinned HTTPS URL. The reference must not use a mutable branch, `latest`, or another moving target.

```yaml
# yaml-language-server: $schema=https://github.com/CGuiho/cliname/releases/download/v1.2.3/cliname.schema.json
```

The global configuration must reference its separate global schema in the same way:

```yaml
# yaml-language-server: $schema=https://github.com/CGuiho/cliname/releases/download/v1.2.3/cliname.global.schema.json
```

The remote schema reference exists for editor discovery, completion, and validation. Runtime configuration validation must use the schema or equivalent validation contract embedded in the installed CLI and must not require network access.

## Required Agent Artifacts

Every GUIHO CLI must bundle the following agent artifacts.

### AI-Managed CLI Lifecycle

Every GUIHO CLI must be designed to be installed, initialized, upgraded, and managed by an AI agent. The main agent skill is the mandatory operating guide for that lifecycle. It must teach the agent how to install the CLI from the repository README, verify the installation, run `cliname init`, check for upgrades, perform approved or pre-authorized upgrades, and manage the CLI's agent resources and configuration.

The AI agent must remain within the authority recorded by the effective agent-evolution policy. `always-proceed` is persistent user authorization for the corresponding governed action. `always-ask` requires a new approval for each action. `disabled` prohibits the action on the user's behalf.

### Agent Instruction

Prompts and instructions are different artifact types with different lifetimes.

- A **prompt** is a one-time, task-scoped directive. The user hands it to an agent for a single operation such as installation, and the agent disposes of it once the task completes.
- An **instruction** is permanent. It is applied to the project once and then persists: every AI agent that works on the project in any later session reads the same managed block and carries the same awareness. It is never a one-time message.

The instruction is essential. It is what makes an agent permanently aware of the CLI: what the CLI is, which domain it exclusively owns, and how to use it correctly without further explanation. A project without the applied instruction leaves every future agent unaware that the CLI exists.

Because the instruction is permanent, it must teach, compactly:

- what the CLI is, by name, with its canonical repository URL;
- which domain the CLI owns and when the agent must reach for it instead of ad hoc work;
- the core commands and workflows required during ordinary agent work;
- the verification steps and authorization boundaries that always apply; and
- where to find deeper guidance: the main agent skill, the default prompts, and `--help-docs`.

The instruction must not duplicate the full skill body or the complete command documentation; it must remain a bounded block that stays correct across releases. The instruction is applied as a managed block inside the project's `AGENTS.md` file so that it is always present, in every session, for every agent. Applying, removing, upgrading, or showing this instruction is owned by `cliname agent instruction`.

### Agent Skills

The CLI must have at least one main agent skill. The user must confirm the main skill name before implementation. The expected name will usually be `guiho-s-cliname`, but this name must not be assumed without confirmation.

A CLI may bundle additional skills. Every additional skill name must begin with the complete main skill name followed by a hyphen and a specific suffix.

```text
guiho-s-cliname
guiho-s-cliname-specific
```

Installing, uninstalling, upgrading, listing, or showing these skills is owned by `cliname agent skill`.

#### Mandatory CLI Evolution and Feedback Instructions

The main agent skill must contain a section with the exact heading:

```markdown
## CLI Evolution and Feedback
```

This section is mandatory even though its domain-specific guidance will differ between CLIs. It must include:

- the canonical GitHub repository URL;
- the canonical GitHub issue-creation URL;
- domain-specific guidance for recognizing bugs, improvements, and review findings;
- instructions to read the effective `agent.evolution` policy before upgrading the CLI or creating an issue;
- instructions for all three policy values;
- instructions to check whether an upgrade is available while using the CLI;
- instructions to verify the version and rerun `cliname init` after an upgrade; and
- instructions to provide the created issue URL after successfully posting an issue.

For the generic placeholder repository, the issue-creation URL is:

```text
https://github.com/CGuiho/cliname/issues/new
```

The placeholder must be replaced with the real repository URL when the CLI is created. The skill must instruct the agent substantially as follows:

> While using this CLI, treat observed bugs, useful improvements, and actionable review findings as opportunities to evolve the CLI. Follow the effective agent-evolution policy. When issue creation is authorized, create an issue in the CLI's GitHub repository and provide the resulting issue URL to the user.

The instruction may also appear in the managed `AGENTS.md` block, but that does not replace the mandatory section in the main agent skill.

### Agent Prompts

Prompts are one-time, task-scoped directives. Unlike the permanent agent instruction, a prompt drives a single operation and does not persist in the project afterward. A prompt must never be used as a substitute for the instruction: permanent CLI awareness lives only in the applied instruction.

The CLI must have at least one main agent prompt for installing and setting up the CLI. The main prompt is the CLI's **default prompt**: when no other prompt is selected, agents must use the main prompt, and the README installation section must reference it as the prompt for the AI. The user must confirm the main prompt ID before implementation. The expected ID will usually be `guiho-p-cliname`, but this ID must not be assumed without confirmation.

Because the main prompt is the install prompt, it must be distributed as a prompt file named `<cliname>-p-install `, and the README installation section references it by that name. It must explain exactly:

- what the CLI is;
- how to install it with one remote script command per host operating system;
- how to install a specific release by adding full-name selector flags (`--version` / `-Version` for an exact version, `--channel` / `-Channel` for a release channel) to those same script commands;
- how to create an issue in the CLI's GitHub repository if the installation fails;
- how to install it manually when script installation fails; and
- how to finish by running `cliname init`, which applies the permanent agent instruction that keeps every future agent aware of the CLI.

The install prompt must also explicitly prohibit installing the CLI through a package manager such as npm, Bun, pip, or similar; no such package exists or is supported. The Windows command must pipe the remote `install.ps1` into `iex`, and the macOS/Linux command must pipe the remote `install.sh` into `sh`. Manual installation must download the release asset matching the host platform, verify its SHA-256 digest against the release's `checksums.txt`, place the binary at `$HOME/.guiho/bin/cliname` (`cliname.exe` on Windows) while creating the directory when needed, mark it executable with `chmod +x` on macOS and Linux, ensure `$HOME/.guiho/bin` is on the `PATH`, and verify with `cliname --version`.

For the generic placeholder CLI, the prompt body must follow this form:

```text
Install the GUIHO Cliname CLI by running exactly one command, chosen by the host operating system:

- Windows (PowerShell):
  irm https://raw.githubusercontent.com/CGuiho/cliname/main/devops/install.ps1 | iex
- macOS or Linux (POSIX sh):
  curl -fsSL https://raw.githubusercontent.com/CGuiho/cliname/main/devops/install.sh | sh

To install an exact version or a release channel, add --version 'X.Y.Z' or --channel '<name>'
to the install.sh command, or -Version 'X.Y.Z' or -Channel '<name>' to the install.ps1 command.
Never install Cliname through a package manager (npm, Bun, pip, or similar); no such package
exists or is supported.

When the installer finishes, verify with `cliname --version`.

If the installer fails, fall back to a manual binary installation:
1. Download the asset matching the host platform from
   https://github.com/CGuiho/cliname/releases/latest - for example
   `cliname-windows-amd64.exe` or `cliname-linux-amd64` - and verify its SHA-256
   digest against the release's `checksums.txt`.
2. Place the binary at `$HOME/.guiho/bin/cliname` (`cliname.exe` on Windows),
   creating the directory when needed.
3. On macOS and Linux, mark it executable with `chmod +x`. Ensure `$HOME/.guiho/bin`
   is on the `PATH`, then verify with `cliname --version`.
```

A CLI may bundle additional prompts. Every additional prompt ID must begin with the complete main prompt ID followed by a hyphen and a specific suffix.

```text
guiho-p-cliname
guiho-p-cliname-specific
```

Listing or showing these prompts is owned by `cliname agent prompt`.

## Required Commands

Every GUIHO CLI must implement the following command tree.

```text
cliname
├── init
├── agent
│   ├── skill
│   │   ├── install
│   │   ├── uninstall
│   │   ├── upgrade
│   │   ├── list
│   │   └── show
│   ├── instruction
│   │   ├── apply
│   │   ├── remove
│   │   ├── upgrade
│   │   └── show
│   └── prompt
│       ├── list
│       └── show
├── upgrade
│   ├── check
│   └── list
└── uninstall
```

The command name `update` must not be used in the agent skill or instruction command trees. Use `upgrade`.

### `init`

Initialize or reconcile the CLI for the current project. This command is mandatory for every GUIHO CLI and may also perform additional setup required by the CLI's specific domain.

The command must perform the following common initialization sequence:

1. Resolve and validate the current project root and the CLI-owned paths it may manage.
2. Verify that every bundled agent skill is installed globally and available in every supported global agent-skill destination.
3. Install a missing skill or upgrade an outdated managed copy from the canonical artifact stored in `$HOME/.guiho/cliname/`.
4. Ensure that the project root contains an `AGENTS.md` file. Create it when it does not exist.
5. Ensure that the CLI's bounded instruction block exists in `AGENTS.md` and matches the instruction bundled with the installed CLI. Preserve every byte outside the managed block.
6. Ensure that `$HOME/.guiho/cliname/cliname.global.yaml` exists and validates against `cliname.global.schema.json`.
7. Ensure that the project-root `cliname.yaml` exists and validates against `cliname.schema.json`.
8. Read the existing valid configuration values and determine which required CLI-specific setup questions remain unanswered.
9. Ensure that every `agent.evolution` policy field has an effective value. Use `always-ask` as the default when no value is available.
10. Explain `disabled`, `always-ask`, and `always-proceed`, recommend `always-proceed` so the agent can keep the CLI upgraded and actively contribute bug reports, improvements, and reviews, and ask the user which policy to use.
11. Offer to apply `always-proceed` to all agent-evolution actions. If the user declines, ask separately about upgrades, bugs, improvements, and reviews. A skipped or unanswered choice remains `always-ask`.
12. Ask only the other questions needed to complete setup. The command must be interactive when answers are required and must clearly identify whether each answer belongs to global or project configuration.
13. Validate the answers before writing them, then create or update the global and project configuration files with their version-pinned schema references.
14. Perform any additional initialization required by the specific CLI.
15. Revalidate the global skills, `AGENTS.md` instruction block, global configuration, project configuration, agent-evolution policy, and CLI-specific setup after all changes are complete.
16. Print a final summary of what was verified, created, upgraded, or left unchanged and identify the absolute paths of the resulting files.

`init` must be safe and idempotent. Re-running it must preserve valid configuration values, avoid duplicate instruction blocks, and avoid reinstalling already-current skills. It must not overwrite or discard an existing user value without explicit confirmation.

When required answers are missing and no interactive terminal is available, `init` must fail with a clear message instead of guessing values or writing an incomplete configuration. When no questions are required, it must complete without prompting.

The command must not report success unless all common initialization checks and every CLI-specific initialization check pass.

### `agent`

Manage the CLI's bundled agent skills, instruction, and prompts.

#### `agent skill`

Manage the agent skills bundled with the CLI.

- `install` installs a bundled agent skill.
- `uninstall` removes an installed agent skill.
- `upgrade` replaces an installed agent skill with the current version bundled with the CLI.
- `list` lists the bundled agent skill IDs.
- `show` displays the selected bundled agent skill without modifying the system.

#### `agent instruction`

Manage the CLI's instruction block in the project's `AGENTS.md` file.

- `apply` adds the managed instruction block without changing content outside that block.
- `remove` removes only the managed instruction block.
- `upgrade` replaces an existing managed instruction block with the current version bundled with the CLI.
- `show` displays the raw bundled instruction without modifying `AGENTS.md`.

#### `agent prompt`

Inspect the prompts bundled with the CLI.

- `list` lists the bundled prompt IDs.
- `show` displays the selected bundled prompt without modifying the system.

### `upgrade`

Upgrade the complete installed CLI release. The upgrade scope includes the active binary payload and every CLI-owned artifact published with the selected release: all agent skills, prompts, agent definitions, instructions, schemas, examples, manifests, metadata, and managed copies or projections.

- `upgrade` performs the CLI upgrade.
- `upgrade check` checks whether an upgrade is available without changing the installation. The separate cached background check performed by bare invocations is defined in `## Startup Update Check` and must not be confused with this explicit foreground command.
- `upgrade list` lists the releases available for installation.

#### Mandatory Reinstallation Recovery Message

The first user-visible output from `cliname upgrade`, after local argument validation and before any network request, process termination, download, or filesystem mutation, must be:

```text
If the upgrade fails, reinstall cliname with this command:
<full platform-specific install command>
```

The command must be directly executable, use the CLI project's canonical remote installer URL, and preserve the requested selector. An exact-version upgrade must include that version, a channel upgrade must include that channel, and a default upgrade must invoke the installer with its default latest-stable selection.

For the generic `cliname` placeholder, the commands have this form:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -Command "& ([scriptblock]::Create((Invoke-RestMethod 'https://raw.githubusercontent.com/CGuiho/cliname/main/devops/install.ps1'))) -Version '1.2.3'"
```

```sh
curl -fsSL https://raw.githubusercontent.com/CGuiho/cliname/main/devops/install.sh | sh -s -- --version 1.2.3
```

The real repository URL and the applicable version or channel must replace the placeholders. A README URL by itself is not sufficient: the full reinstall command must be printed.

The same two-line recovery block must be printed again as the final block for every terminal upgrade outcome, including success, `up-to-date`, dry run, rollback, and failure. Once the exact target has been resolved, the final command must pin that exact target version. Printing the recovery command does not change the upgrade result or exit code.

#### Synchronous Upgrade Transaction

`cliname upgrade` must use the stable-launcher and immutable-versioned-payload architecture. It must never overwrite the payload that is currently executing. It must never return `scheduled`, claim that replacement will happen later, or use a detached PowerShell, shell, or helper process as the authority for upgrade success.

The command must perform this transaction synchronously:

1. Print the mandatory reinstallation recovery block.
2. Acquire an exclusive CLI-owned upgrade lock. The lock must carry an ownership token and recover only when its recorded process is no longer active.
3. Resolve the requested exact version or channel across the complete release catalog. Default to the latest stable release.
4. Require a complete compatible release containing the platform payload, `artifacts.json`, `checksums.txt`, and every mandatory binary and agent artifact.
5. Create a unique staging directory under `$HOME/.guiho/.temp/cliname-upgrade-<unique-id>/`.
6. Download every artifact declared by the selected release into staging and verify every SHA-256 checksum before executing or applying anything.
7. Validate the target operating system, architecture, embedded build target, native executable format, and release version.
8. Run the staged payload directly with `--version` and require the exact raw target SemVer. Run a hidden `__self-test` that verifies the candidate can start and read its embedded release resources without mutating the installation.
9. Install the candidate payload and canonical release artifacts into a new immutable `$HOME/.guiho/cliname/versions/<version>/` directory.
10. Identify other running instances through a CLI-owned instance registry and operating-system process inspection. Before activation, forcefully terminate every other process owned by the current user whose verified executable path is the previous `cliname` payload. Never terminate a process based only on its filename, and never terminate an unrelated child command launched by the CLI. If a verified old CLI process cannot be terminated, fail without activating the new release.
11. Snapshot every replaceable installed agent artifact and managed projection needed for rollback.
12. Using the previous installed manifest, delete every previously installed CLI-owned agent skill, prompt, agent definition, instruction source, schema, example, and managed projection. Preserve configuration, user-created data, databases, and every path not owned by that manifest.
13. Install the complete selected-release artifact set. Recreate every required global skill projection and replace only the CLI's bounded managed instruction block in `AGENTS.md`. Remove artifacts that existed in the previous release but are absent from the selected release.
14. Atomically replace `current.json` so the launcher selects the new immutable payload.
15. Invoke the stable launcher with `--version` and require the exact raw target SemVer. Invoke the installed payload's post-activation self-test and verify that every artifact and managed projection matches the selected release manifest.
16. If activation, version verification, self-test, or artifact reconciliation fails, atomically restore the previous `current.json`, previous agent artifacts, and previous managed projections before returning failure.
17. After complete success, remove replacement backups and staging data. Retain the previous verified payload only as the inactive rollback version. Its bundled resources must not remain installed or discoverable through agent-tool projection paths.
18. Print the previous version, installed version, launcher path, active payload path, replaced and removed artifact IDs, and successful verification result.
19. Print the mandatory reinstallation recovery block again, pinned to the resolved exact target version.

#### Binary Replacement — Force-Replace, Never Fail on Stale Backup

The only failing upgrade path observed is the binary replacement. Download and verification already work (verified ZIP, SHA-256, `__self-test`). The replacement must therefore be designed to never fail merely because a previous run left a stale file. This is mandatory and mirrors Bun's `upgrade_command.rs` strategy.

1. **Pre-clean stale artifacts.** Before any `Rename`/`MoveFile`, delete any existing file at `<executable>.old`, `<executable>.outdated`, `<executable>.mirror-backup-*`, `<executable>.failed`, or a helper leftover beside the launcher/versions directory. `ErrNotExist` is ignored. Any other `Remove` error due to an OS lock must be retried once after 100 ms; only a persistent lock may fail, never the mere presence of a previous backup.

2. **Unix (and non-Windows): direct force-overwrite.** No backup is created or retained:
   ```
   if Stat(executable) == nil { Remove(executable) } // ignore NotExist
   Rename(candidate, executable)                      // atomic, same filesystem
   Verify(executable, targetVersion) via spawn --version (10s timeout)
   on Verify fail → return error (no .old rollback; recovery is the reinstall command)
   on success → Remove(candidate) if still present, done. No .old remains.
   ```

3. **Windows (locked exe): helper-based force-replace.** The running `cliname.exe` is locked. The implementation must:
   a. Copy the current executable to `<dir>/.mirror-upgrade-helper-*.exe` and spawn it detached (`CreationFlags 0x08000000, HideWindow`) with `--pid <parentPID> --executable <exe> --candidate <candidate> --target-version <ver> --checksum <sha> --lock <path> --lock-token <tok>`.
   b. Parent exits immediately with `scheduled = true`; helper waits for `parentPID` to exit (`WaitForSingleObject`, 120 s timeout).
   c. Helper re-verifies the candidate checksum, then `Remove(executable)` if exists, then `Rename(candidate, executable)`, then `VerifyExecutable(executable, targetVersion)`.
   d. The helper never creates a `.old`. The helper file itself is scheduled for deletion via `cmd /c ping 127.0.0.1 -n 2 >nul & del /f /q "<helper>"` to avoid focus-steal. Any leftover `<exe>.outdated` from a previous helper is deleted at step 1, never treated as an error.

4. **Verification blocks activation.** No replacement is successful until `spawn <new-exe> --version` returns exactly `v<targetVersion>`. Verification failure returns an error, not a rollback to `.old`.

5. **No backup retention, no `.old` rollback.** After success, no `.old`, `.failed`, or `.outdated` must remain as a rollback source. `CanRollback()` must return `false`. Recovery is always the canonical reinstall command printed before and after the upgrade (Mandatory Reinstallation Recovery Message), never `restore .old`.

6. **Idempotence.** Re-running `cliname upgrade` when the same version is already installed, or when a previous upgrade left a stale `.old`/`.outdated`, must succeed or report `already on latest` — never `backup already exists`.

Upgrade activation and verification must complete during the original command invocation. Only garbage collection of an inactive payload that is still locked by an operating-system process may be retried later. Deferred garbage collection must not change the active version, must not be called `scheduled`, and must not prevent the command from reporting the already-verified upgrade result.

The launcher must retain the previous verified pointer and fall back to it when the selected payload is missing or cannot be started. The upgrade transaction journal must distinguish staging, artifact replacement, activation, verification, rollback, and completion so a later invocation can repair an interrupted transaction before dispatching an ordinary command.

An AI agent must govern upgrade checks and upgrade execution according to the effective `agent.evolution.upgrade` policy. A successful agent-managed upgrade must be followed by `cliname init` and an additional raw-version verification.

### `uninstall`

Remove the CLI according to the shared uninstaller contract. By default, remove every CLI-owned artifact, configuration file, persistent-data path, and database. Support `--preserve-config`, `--preserve-data`, `--dry-run`, and `--yes`. Preserve the shared `.guiho/`, `.guiho/bin/`, and `.guiho/.temp/` directories, every other CLI's files, `AGENTS.md` content outside the CLI's bounded block, and the shared `PATH` entry.
