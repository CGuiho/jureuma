---
name: Rename the project, CLI and all artifacts to ZeZé (release 0.4.0)
purpose: Define the expected outcome, constraints, and completion signals for the rename-to-ZeZé task in todo.md
description: What must be true for the 0.4.0 rename of the project, the CLI and every artifact to the ZeZé identity to be complete, and what to watch before, during and after the work.
created: 2026-10-22T05:48:44Z
---

#### &copy; 2026 [GUIHO](https://guiho.co) as represented by [Cristóvão GUIHO](https://guiho.co/cguiho) All Rights Reserved.

# Rename the project, CLI and all artifacts to ZeZé (release 0.4.0)

## Summary

The project, the CLI and every artifact move to the ZeZé identity in one
breaking release, `0.4.0`, published as `ZeZé 0.4.0`. The legacy name
disappears from the working tree entirely except for one named exemption file,
the obsolete alias command surface is deleted outright, and nothing keeps a
backward-compatibility path.

## Todo Index

- Task: `Rename the project, CLI and all artifacts to ZeZé (release 0.4.0)`
- Status: in progress
- Index: [todo.md](../../todo.md)
- GitHub project item: [Issue #16](https://github.com/CGuiho/zeze/issues/16)
- GitHub component: `zeze`
- Question ledger: [2026-10-22-rename-decisions.md](../questions/rename-to-zeze/2026-10-22-rename-decisions.md)

## Outcome

Release `0.4.0` exists as `ZeZé 0.4.0`, and the whole working tree speaks with
one identity: machine identifiers `zeze`, prose and titles `ZeZé`, environment
prefix `ZEZE_`, ASCII-only filenames. Exactly one file —
`docs/2026-10-22-zeze-migration-instruction.md` — still carries the legacy
names, and it is the document that tells a human how to migrate an existing
installation. There is no alias, no shim and no compatibility window anywhere.

## Scope

### In scope

- One global rename sweep across code, docs, scripts, configuration and
  metadata, done as a single mechanical pass.
- Removal of the obsolete alias command surface as a feature deletion: command,
  launcher, installer and uninstaller surface and documentation.
- The single migration instruction document for existing installations,
  including the human's global install.
- The `0.4.0` release train ending in `ZeZé 0.4.0`.
- Interface changes that come with the rename: error tags `[ZEZE:<KIND>]`,
  managed-block markers, the `ZEZE_` environment prefix, the CLI home
  `~/.guiho/zeze/`.

### Out of scope

- Rewriting git history: tags, existing commits and the four published GitHub
  releases are history and stay untouched.
- The legacy Project `Component` option, preserved byte-identically because
  other items reference it; new tasks use the `zeze` option.
- Any backward-compatibility alias, shim or deprecation period — explicitly
  rejected (zero backward compatibility, confirmed verbatim by CG).

## Acceptance Signals

- `0.4.0` is released under the title `ZeZé 0.4.0` and the version surfaces
  agree.
- No legacy-name token remains anywhere in the working tree except inside
  `docs/2026-10-22-zeze-migration-instruction.md` (the single named exemption).
- The alias command surface is gone: no command, no launcher, no installer or
  uninstaller path, no documentation for it.
- `runx`, `xdocs` and `mirror` validation all pass on the renamed tree, and the
  test suites are green.
- The migration instruction is complete enough to move an existing
  installation, including the human's global install.
- Historical documents remain coherent after the minimal alias scrub, with no
  fabricated history.

## Watch-outs

- The rename sweep is one deliberate mechanical pass; partial renames leave the
  tree in a mixed-identity state that is worse than either end state.
- The legacy Project `Component` option must survive byte-identically — do not
  rename, recolour, delete or recreate it to "clean up" the board.
- The issue body and title are token-free because remote objects cannot be
  swept later; keep every new remote artifact the same way.
- Git tags, commits and published releases are history: never move, amend or
  re-edit them to make the rename look complete.
- Two sibling workers edit disjoint paths of the same working tree during
  wave 1; the Git index is serialized with `flock`, and nothing outside the
  owned paths may be staged.

## Before Starting

- Confirm the tracking issue exists, is on Project `GUIHO` with
  `Component = zeze` and `Status = In Progress`, and read both back.
- Confirm the `zeze` Component option exists (id `f15e2d37`, colour `BLUE`
  mirrored from the preserved legacy option) and that the legacy option is
  untouched.
- Read the question ledger for the accepted decisions before changing anything.

## While Working

- Keep the single-exemption token policy: the migration instruction is the only
  file allowed to carry either legacy name.
- Answer new questions in the question ledger instead of stopping; DND mode is
  in force.
- Reference the legacy identity as "the legacy name", "the old CLI", "the
  removed alias" or "git history" in every new or rewritten record.

## After Finishing

- Move the issue to `testing` at handoff and close it with the human verdict.
- Record the release evidence (`0.4.0` / `ZeZé 0.4.0`) in the ledger and the
  changelog through their owning units.
- Keep the human's global install migration as the one remaining human action,
  owned by the migration instruction.

## Related Files

- [../questions/rename-to-zeze/2026-10-22-rename-decisions.md](../questions/rename-to-zeze/2026-10-22-rename-decisions.md) - Question ledger: every decision taken unattended, with rationale.
- [../2026-10-22-zeze-migration-instruction.md](../2026-10-22-zeze-migration-instruction.md) - The single named exemption file and the human migration guide (created by the migration unit).

## References

- [todo.md](../../todo.md)
- [Issue #16](https://github.com/CGuiho/zeze/issues/16)
