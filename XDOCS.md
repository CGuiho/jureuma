# jureuma -- XDocs Root

The single root index for this repository. It has no frontmatter. Every
project-owned directory has exactly one `*.xdocs.md` descriptor; the tree below
starts at the repository descriptor.

## Repository

- [jureuma](jureuma.xdocs.md): repository root, tooling (`mirror.yaml`, `runx.yaml`, `xdocs.yaml`), README, AGENTS.md, changelog and the governing convention.

## Executable

- [cmd](cmd/cmd.xdocs.md)
- [cmd/jureuma](cmd/jureuma/jureuma.xdocs.md): `main` package with the build-time version.

## Packages

- [internal](internal/internal.xdocs.md): container of every internal package.
- [agentres](internal/agentres/agentres.xdocs.md): embedded skill, prompts and AGENTS.md instruction (`assets/` is embedded verbatim and excluded from scanning).
- [cli](internal/cli/cli.xdocs.md): the Cobra command tree.
- [config](internal/config/config.xdocs.md), [examples](internal/config/examples/examples.xdocs.md), [schemas](internal/config/schemas/schemas.xdocs.md): configuration files, examples and their JSON Schemas.
- [crypt](internal/crypt/crypt.xdocs.md), [keystore](internal/keystore/keystore.xdocs.md): value encryption and private keys.
- [doc](internal/doc/doc.xdocs.md), [jsonc](internal/jsonc/jsonc.xdocs.md), [schema](internal/schema/schema.xdocs.md): JSON model, parsing and JSON Schema.
- [pair](internal/pair/pair.xdocs.md): configuration pairs and sync state.
- [runner](internal/runner/runner.xdocs.md), [redact](internal/redact/redact.xdocs.md): `run` injection and output redaction.
- [gate](internal/gate/gate.xdocs.md): human-only gate and audit log.
- [paths](internal/paths/paths.xdocs.md), [update](internal/update/update.xdocs.md), [xerr](internal/xerr/xerr.xdocs.md): locations, release catalog and exit codes.

## Operations

- [devops](devops/devops.xdocs.md): install/uninstall scripts and build, release, smoke and lint scripts.

## Tests

- [test](test/test.xdocs.md), [test/e2e](test/e2e/e2e.xdocs.md): end-to-end tests against the built binary.

## Documentation

- [docs](docs/docs.xdocs.md): design and implementation contracts.
