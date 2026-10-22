---
subject: zeze-config
description: "Loads, validates, merges and saves zeze.yaml and ~/.guiho/zeze/zeze.global.yaml against embedded JSON Schemas, including the project key binding (key:) and the global-only protections block and its seal."
parent: zeze-internal
children:
  - zeze-config-examples
  - zeze-config-schemas
files:
  "config.go": "Project and global configuration types (including the zeze.yaml key binding), loading, merging and saving with version-pinned schema comments."
  "schema.go": "Embeds schemas/ and examples/."
  "keybind.go": "Byte-preserving one-line splice of the zeze.yaml key: binding (used by init and key rotate; never a whole-document rewrite)."
  "keybind_test.go": "Key-binding splice tests (append, in-place replace, already-equal no-op, refusals without writes)."
  "protections.go": "Protections block seal (HMAC) state."
  "config_test.go": "Configuration tests (schema validation, merge, examples equal embedded copies)."
  "protections_test.go": "Protections seal tests."
  "envnames.go": "Shared denylist of environment variable names that load or run code (flatten, run.envName, --env-name)."
documents: {}
tags:
  - "go"
  - "configuration"
  - "yaml"
keywords:
  - "zeze.yaml"
  - "zeze.global.yaml"
  - "agent.evolution"
  - "protections"
flags: []
status: stable
---

# zeze-config

Loads, validates, merges and saves zeze.yaml and ~/.guiho/zeze/zeze.global.yaml against embedded JSON Schemas, including the project key binding (`key:`) and the global-only protections block and its seal. Configuration documents that are present but invalid fail every load as validation errors (exit 4).

## Files

- `config.go`: Project and global configuration types (including the zeze.yaml key binding), loading, merging and saving with version-pinned schema comments.
- `schema.go`: Embeds schemas/ and examples/.
- `keybind.go`: Byte-preserving one-line splice of the zeze.yaml key: binding (used by init and key rotate; never a whole-document rewrite).
- `keybind_test.go`: Key-binding splice tests (append, in-place replace, already-equal no-op, refusals without writes).
- `protections.go`: Protections block seal (HMAC) state.
- `config_test.go`: Configuration tests (schema validation, merge, examples equal embedded copies).
- `protections_test.go`: Protections seal tests.
- `envnames.go`: Shared denylist of environment variable names that load or run code (flatten, run.envName, --env-name).

## Children

- `zeze-config-examples`
- `zeze-config-schemas`
