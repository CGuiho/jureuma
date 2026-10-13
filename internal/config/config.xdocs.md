---
subject: jureuma-config
description: "Loads, validates, merges and saves jureuma.yaml and ~/.guiho/jureuma/jureuma.global.yaml against embedded JSON Schemas, including the global-only protections block and its seal."
parent: jureuma-internal
children:
  - jureuma-config-examples
  - jureuma-config-schemas
files:
  "config.go": "Project and global configuration types, loading, merging and saving with version-pinned schema comments."
  "schema.go": "Embeds schemas/ and examples/."
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
  - "jureuma.yaml"
  - "jureuma.global.yaml"
  - "agent.evolution"
  - "protections"
flags: []
status: stable
---

# jureuma-config

Loads, validates, merges and saves jureuma.yaml and ~/.guiho/jureuma/jureuma.global.yaml against embedded JSON Schemas, including the global-only protections block and its seal.

## Files

- `config.go`: Project and global configuration types, loading, merging and saving with version-pinned schema comments.
- `schema.go`: Embeds schemas/ and examples/.
- `protections.go`: Protections block seal (HMAC) state.
- `config_test.go`: Configuration tests (schema validation, merge, examples equal embedded copies).
- `protections_test.go`: Protections seal tests.
- `envnames.go`: Shared denylist of environment variable names that load or run code (flatten, run.envName, --env-name).

## Children

- `jureuma-config-examples`
- `jureuma-config-schemas`
