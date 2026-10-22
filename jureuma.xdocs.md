---
subject: jureuma
description: "Repository root of jureuma, the GUIHO agent-safe configuration CLI written in Go: AI agents can set configuration and secrets and inject them into processes without ever reading them, while humans keep plaintext in a gitignored file and commit an encrypted file plus a JSON Schema."
parent: null
children:
  - jureuma-cmd
  - jureuma-devops
  - jureuma-docs
  - jureuma-internal
  - jureuma-test
files:
  "go.mod": "Go module github.com/CGuiho/jureuma (Go 1.26; cobra, santhosh-tekuri/jsonschema/v6, go.yaml.in/yaml/v3)."
  "go.sum": "Module checksums."
  "mirror.yaml": "GUIHO Mirror versioning: Git tags v<semver> are the canonical version (first release 0.1.0), release commit/tag/push policy and agent instruction hooks."
  "runx.yaml": "GUIHO RunX command catalog (namespace jureuma): fmt, vet, test, build, cross-build, e2e, smoke, installer tests, release preparation/verification/publication, schema publication, version and docs checks."
  "xdocs.yaml": "XDocs configuration: descriptor extension, excluded build output and embedded assets, frontmatter opt-outs."
  ".gitignore": "Ignores build output (bin/, dist/)."
documents:
  "README.md": "User-facing overview: install, uninstall, quick start, agent-safety model and command reference pointer."
  "AGENTS.md": "Instructions for AI agents working in this repository (project scope, governing convention, tooling rules, Mandume coordination section)."
  "CHANGELOG.md": "Release notes maintained at release time under Mirror (agents.write_changelog)."
  "todo.md": "GitHub-first task index: the bound GUIHO Project (GitHub component jureuma) mirrored locally; every task is a real issue in CGuiho/jureuma."
  "XDOCS.md": "Root XDocs index (special setup file): root descriptor and top-level directories; browse with xdocs tree."
  "guiho-convention-0001-cli.md": "The governing GUIHO CLI Convention (copied verbatim; authoritative over AGENTS.md)."
tags:
  - "cli"
  - "go"
  - "secrets"
  - "configuration"
  - "agent-safety"
keywords:
  - "jureuma"
  - "encrypt"
  - "JUREUMA_CONFIG"
  - "JSON Schema"
  - "GUIHO"
flags: []
status: stable
---

# jureuma

Repository root of jureuma, the GUIHO agent-safe configuration CLI written in Go: AI agents can set configuration and secrets and inject them into processes without ever reading them, while humans keep plaintext in a gitignored file and commit an encrypted file plus a JSON Schema.

## Files

- `go.mod`: Go module github.com/CGuiho/jureuma (Go 1.26; cobra, santhosh-tekuri/jsonschema/v6, go.yaml.in/yaml/v3).
- `go.sum`: Module checksums.
- `mirror.yaml`: GUIHO Mirror versioning: Git tags v<semver> are the canonical version (first release 0.1.0), release commit/tag/push policy and agent instruction hooks.
- `runx.yaml`: GUIHO RunX command catalog (namespace jureuma): fmt, vet, test, build, cross-build, e2e, smoke, installer tests, release preparation/verification/publication, schema publication, version and docs checks.
- `xdocs.yaml`: XDocs configuration: descriptor extension, excluded build output and embedded assets, frontmatter opt-outs.
- `.gitignore`: Ignores build output (bin/, dist/).

## Documents

- [README.md](README.md): User-facing overview: install, uninstall, quick start, agent-safety model and command reference pointer.
- [AGENTS.md](AGENTS.md): Instructions for AI agents working in this repository (project scope, governing convention, tooling rules, Mandume coordination section).
- [CHANGELOG.md](CHANGELOG.md): Release notes maintained at release time under Mirror (agents.write_changelog).
- [todo.md](todo.md): GitHub-first task index: the bound GUIHO Project (GitHub component jureuma) mirrored locally; every task is a real issue in CGuiho/jureuma.
- [guiho-convention-0001-cli.md](guiho-convention-0001-cli.md): The governing GUIHO CLI Convention (copied verbatim; authoritative over AGENTS.md).

## Children

- `jureuma-cmd`
- `jureuma-devops`
- `jureuma-docs`
- `jureuma-internal`
- `jureuma-test`
