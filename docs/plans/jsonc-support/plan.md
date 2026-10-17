---
name: JSONC Write Support
purpose: Record the one-unit train that makes `.jsonc` fully two-way through a comment-preserving writer, JSON staying the default.
description: "Plan for the one-unit train: a comment-preserving JSONC writer with wire-in for set/unset mirrors, decrypt and schema writes (issue #12), JSON staying the default."
created: "2026-10-17"
owner: jureuma-docs-plans-jsonc-support
flags: []
tags:
  - "planning"
keywords:
  - "JSONC"
  - "comment preservation"
  - "config format"
status: stable
---

#### &copy; 2026 [GUIHO](https://guiho.co) as represented by [Cristóvão GUIHO](https://guiho.co/cguiho) All Rights Reserved.

# JSONC write support

## Context

CG directive (voice, 2026-10-17): "We should add support for JSONC, but by
default use JSON." JSONC was first floated as the default schema format and
explicitly corrected back to JSON in the same message. Today `jureuma` reads
JSONC leniently everywhere but v1 never writes a `.jsonc` file — the design
states there is no comment-preserving writer, so `set`/`unset` skip the
plaintext mirror, `decrypt` refuses, and the human reconciles by hand.

## Units

### W1 — comment-preserving JSONC writer (issue #12)

Lossless JSONC layer and minimal-edit engine in `internal/jsonc`; `WritePlain`,
the `set`/`unset` mirrors, `decrypt` and `schema infer` write `.jsonc` while
preserving comments and formatting byte-for-byte outside the changed spans,
refusing rather than destroying them. Brief:
`execution/handoffs/w1-jsonc-writer.md`.

## Sequencing

Runs after the run env name + OpenCode deny rules train lands
(`run-envname-and-opencode-deny-rules`): shared files
(`internal/cli/cmd_init.go`, the design document, `CHANGELOG.md`) and one
serialized Git index. One native background workhorse; the orchestrator
reviews actual commits, checks and readbacks before delivery.

## Decisions

- "Support JSONC" means full two-way support for the human-owned files with
  byte-level comment preservation; JSON remains the default and there are no
  new flags — choosing JSONC means naming the file `.jsonc`.
- Refuse rather than destroy: every write either preserves all bytes outside
  the changed spans or fails with a value-free error; only spans that the
  change itself replaces or removes may lose their own comments.
- The encrypted file stays `.json` and comments are never carried into it;
  `.json` write behavior (canonical) is unchanged.
- `schema infer` on an existing `.jsonc` schema (init starter or `--force`)
  merges onto the existing text so comments on surviving keys survive.

## Gates

- Unit gate: full battery (`go-fmt-check`, `go-vet-cross`, `go-test`,
  `test-e2e`, `test-smoke`, `docs-check`, `ci-check`) green; orchestrator
  reviews actual commits, diffs, checks and readbacks.
- Release gate: version bump and release only on CG's explicit word.
