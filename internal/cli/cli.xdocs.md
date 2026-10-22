---
subject: jureuma-cli
description: "The Cobra command tree: every command, help flags (--help-docs, --help-tree), exit codes, the human-only gate and bypass handling, init, agent, upgrade, uninstall and the bare-invocation update check."
parent: jureuma-internal
children: []
files:
  "cli.go": "Root command, Execute, global flags and exit-code mapping."
  "helpdocs.go": "--help-docs and --help-tree rendering."
  "context.go": "Shared command environment (project root, configuration, pair resolution with the jureuma.yaml key-binding enforcement), the single plaintext reader with identity metadata, the embedded schema-copy helpers and the small file/executable helpers."
  "stash.go": "Saved copy of an encrypted file removed by decrypt, and its restore (saved copy, then git HEAD) for encrypt."
  "cmd_values.go": "encrypt, decrypt, set, unset and get, including the only-unencrypted sibling builds and the schema-copy restore on decrypt."
  "cmd_run.go": "run: inject configuration into a child process with redacted output."
  "cmd_info.go": "list, status and check: pair state across the four file states, identity headers and $schema/embedded-copy reporting; a missing key ID is reported as state, never a key error."
  "cmd_key.go": "key show, list, export, rotate (project-wide with a binding: every bound pair to one new key — values, embedded schema copies and recorded identities — plus the jureuma.yaml binding update) and retire."
  "keysel.go": "Key labels, short ID and public-key prefix rendering, default label derivation and the TTY-safe stored-key picker (Enter selects the default choice)."
  "cmd_schema.go": "schema infer (optional $schema property, $schema pointer and embedded copy refresh)."
  "cmd_agent.go": "agent skill, instruction and prompt commands."
  "cmd_upgrade.go": "upgrade, upgrade check and upgrade list with the reinstall recovery block."
  "cmd_uninstall.go": "uninstall with the REMOVE/PRESERVE plan and key protection."
  "cmd_init.go": "init: configuration files, keys (the bound key, stored-key selection), the jureuma.yaml key binding, pair creation and header repair, gitignore block, schema, agent resources and deny rules."
  "denyrules.go": "Agent-tool deny rules for *.unencrypted.* in .claude/settings.json."
  "detach_unix.go": "Detached background update worker on Unix (Setsid)."
  "detach_windows.go": "Detached background update worker on Windows (DETACHED_PROCESS)."
  "cli_test.go": "In-process CLI tests, including the marker-value leak test."
  "keysel_test.go": "Rendering, picker (including the Enter-default path) and default-label tests."
  "cmd_key_list_test.go": "key list, key show and key rotate label tests."
  "projectkey_test.go": "Project key-binding tests (disagreement exit 5, init binding writes and the bound key, flag contradiction exit 2, rotate update, malformed binding exit 4, never-overwritten binding)."
  "upgrade_tx.go": "Synchronous upgrade transaction (staging, verification, versions/<v>, skills, current.json, launcher check, journal rollback) and the hidden __self-test command."
  "upgrade_journal.go": "Upgrade transaction journal (phases, write-ahead rollback plan) and interrupted-transaction repair."
  "upgrade_journal_test.go": "Journal and repair tests."
  "selfdelete_unix.go": "No-op running-image release for uninstall on Unix."
  "selfdelete_windows.go": "Moves the running executable aside so uninstall can remove it on Windows."
documents: {}
tags:
  - "go"
  - "cobra"
  - "commands"
keywords:
  - "init"
  - "run"
  - "set"
  - "get"
  - "decrypt"
  - "uninstall"
  - "upgrade"
flags: []
status: stable
---

# jureuma-cli

The Cobra command tree: every command, help flags (--help-docs, --help-tree), exit codes, the human-only gate and bypass handling, init, agent, upgrade, uninstall and the bare-invocation update check.

## Files

- `cli.go`: Root command, Execute, global flags and exit-code mapping.
- `helpdocs.go`: --help-docs and --help-tree rendering.
- `context.go`: Shared command environment (project root, configuration, pair resolution with the jureuma.yaml key-binding enforcement), the single plaintext reader with identity metadata, the embedded schema-copy helpers and the small file/executable helpers.
- `stash.go`: Saved copy of an encrypted file removed by decrypt, and its restore (saved copy, then git HEAD) for encrypt.
- `cmd_values.go`: encrypt, decrypt, set, unset and get, including the only-unencrypted sibling builds and the schema-copy restore on decrypt.
- `cmd_run.go`: run: inject configuration into a child process with redacted output.
- `cmd_info.go`: list, status and check: pair state across the four file states, identity headers and $schema/embedded-copy reporting; a missing key ID is reported as state, never a key error.
- `cmd_key.go`: key show, list, export, rotate (project-wide with a binding: every bound pair to one new key — values, embedded schema copies and recorded identities — plus the jureuma.yaml binding update) and retire.
- `keysel.go`: Key labels, short ID and public-key prefix rendering, default label derivation and the TTY-safe stored-key picker (Enter selects the default choice).
- `cmd_schema.go`: schema infer (optional $schema property, $schema pointer and embedded copy refresh).
- `cmd_agent.go`: agent skill, instruction and prompt commands.
- `cmd_upgrade.go`: upgrade, upgrade check and upgrade list with the reinstall recovery block.
- `cmd_uninstall.go`: uninstall with the REMOVE/PRESERVE plan and key protection.
- `cmd_init.go`: init: configuration files, keys (the bound key, stored-key selection), the jureuma.yaml key binding, pair creation and header repair, gitignore block, schema, agent resources and deny rules.
- `denyrules.go`: Agent-tool deny rules for *.unencrypted.* in .claude/settings.json.
- `detach_unix.go`: Detached background update worker on Unix (Setsid).
- `detach_windows.go`: Detached background update worker on Windows (DETACHED_PROCESS).
- `cli_test.go`: In-process CLI tests, including the marker-value leak test.
- `keysel_test.go`: Rendering, picker (including the Enter-default path) and default-label tests.
- `cmd_key_list_test.go`: key list, key show and key rotate label tests.
- `projectkey_test.go`: Project key-binding tests (disagreement exit 5, init binding writes and the bound key, flag contradiction exit 2, rotate update, malformed binding exit 4, never-overwritten binding).
- `upgrade_tx.go`: Synchronous upgrade transaction (staging, verification, versions/<v>, skills, current.json, launcher check, journal rollback) and the hidden __self-test command.
- `upgrade_journal.go`: Upgrade transaction journal (phases, write-ahead rollback plan) and interrupted-transaction repair.
- `upgrade_journal_test.go`: Journal and repair tests.
- `selfdelete_unix.go`: No-op running-image release for uninstall on Unix.
- `selfdelete_windows.go`: Moves the running executable aside so uninstall can remove it on Windows.
