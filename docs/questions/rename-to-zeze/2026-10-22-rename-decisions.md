---
name: 2026-10-22 Rename Decisions
purpose: Record every question the unattended (DND) executor answered itself during the rename-to-ZeZé run.
description: "DND question ledger for the 2026-10-22 rename run: the token mapping registers, zero backward compatibility, history that stays untouched, the preserved legacy Project component option, the untouched global install, the single-exemption token policy, the alias feature deletion, interface changes, issue lifecycle, the historical-doc scrub, and the dispatch record."
created: 2026-10-22
owner: zeze-docs-questions-rename-to-zeze
flags: []
tags:
  - "questions"
  - "rename"
  - "dnd"
keywords:
  - "release 0.4.0"
  - "rename to zeze"
  - "zero backward compatibility"
  - "question ledger"
---

#### &copy; 2026 [GUIHO](https://guiho.co) as represented by [Cristóvão GUIHO](https://guiho.co/cguiho) All Rights Reserved.

# 2026-10-22 Rename Decisions

Context: CG ordered the project, the CLI and every artifact moved to the ZeZé
identity in one breaking release. The work runs as a six-unit UltraCode run
under DND rules: never stop, never ask CG, answer every question with the
safest reversible choice and record it here. Unit U-S owns task tracking — the
Project Component option, the issue, this ledger and the task spec.

## Q1 — Is the rename a breaking release with zero backward compatibility?

**Answer: yes — zero backward compatibility, confirmed verbatim by CG.** No
alias, no shim, no dual-name support, no deprecation window: every surface
changes at once and anything pinned to the old identity breaks deliberately.
The breaking version is `0.4.0` and the release title is `ZeZé 0.4.0`.

## Q2 — What is the token mapping?

**Answer: one-to-one, in three registers plus filenames.** Machine identifiers
map to `zeze`; prose and titles map to `ZeZé`; the environment prefix maps to
`ZEZE_`; filenames stay ASCII-only (`zeze-...`, no accented characters). The
mapping is strictly one-to-one, so naming the target forms names the source
forms as well.

Deviation from the dispatch brief, recorded deliberately: the brief spelled the
mapping out with the legacy tokens on the left-hand side. This ledger does not
repeat them. The single-exemption token policy (Q6) makes
`docs/2026-10-22-zeze-migration-instruction.md` the only file allowed to carry
either legacy name, and a literal legacy token written here would either create
a second file carrying it or be mechanically rewritten by the rename sweep into
a tautology (`target` → `target`). The exact legacy spellings stay verbatim in
that exemption file and in git history, which is exactly where the policy keeps
them.

## Q3 — Which history is rewritten and which stays untouched?

**Answer: in-tree documents are renamed and minimally scrubbed; git tags,
existing commits and the four published GitHub releases stay untouched (CG's
own words).** Git history is never rewritten: no tag moves, no commit is
amended or rebased, and the four already-published releases keep their tags,
assets and titles. The record inside the working tree is renamed and scrubbed
only as far as Q10 allows.

## Q4 — What happens to the legacy Project Component option?

**Answer: preserved byte-identically.** Other project items still reference it,
so its name, colour and position are left exactly as they are; the run only
adds the new option. Evidence captured before and after the mutation on Project
`GUIHO` (number 2, field `Component`, id `PVTSSF_lAHOBUk1ds4AULOgzhi1B5U`):

- BEFORE: 80 options; the legacy option id `9c9e8db0`, colour `BLUE`; no
  `zeze` option.
- AFTER: 81 options; `zeze` id `f15e2d37`, colour `BLUE`; the 80 pre-existing
  options byte-identical (ids, names, colours, descriptions and order verified
  equal by diff).

Colour source: `BLUE` is mirrored from the preserved legacy option itself, which
is the precedent for this component's role (the same repository, renamed). No
palette fallback was needed because the legacy option's colour is set and
readable. Every new task uses component `zeze`.

## Q5 — What about the human's global install?

**Answer: deliberately untouched.** The human's global `/root/.agents` install
is left alone by this run; moving it is the job of the migration instruction
(`docs/2026-10-22-zeze-migration-instruction.md`), not of the sweep.

## Q6 — Which file may carry the legacy names?

**Answer: exactly one, by name.** The single-exemption token policy: the only
file allowed to carry either legacy token is
`docs/2026-10-22-zeze-migration-instruction.md`. Everywhere else the record says
"the legacy name", "the old CLI", "the removed alias" or "git history" instead
of spelling the old names out — including the GitHub issue, whose body and
title were written token-free before creation because remote objects cannot be
swept afterwards.

## Q7 — Is the alias removal a rename?

**Answer: no — it is a feature deletion.** The removed alias's command, its
launcher, its installer and uninstaller surface and its documentation are
deleted outright rather than renamed to anything. Nothing replaces them; the
only trace kept is the historical record, scrubbed minimally per Q10.

## Q8 — Which interface changes ride along with the rename?

**Answer: all of them, in the same breaking release.** Error tags become
`[ZEZE:<KIND>]`; the managed-block markers take the new identity; the injected
environment prefix becomes `ZEZE_`; the CLI home becomes `~/.guiho/zeze/`.
None of these keep a compatibility alias (Q1).

## Q9 — What is the issue lifecycle?

**Answer: created `in progress`, moved to `testing` at handoff, closed with the
human verdict.** The tracking issue is
[#16](https://github.com/CGuiho/zeze/issues/16) in `CGuiho/zeze`, added to
Project `GUIHO` (item `PVTI_lAHOBUk1ds4AULOgzhCnKBo`) with `Component = zeze`
(option `f15e2d37`) and `Status = In Progress` (option `47fc9ee4`), both read
back after the write. It is a real issue, not a draft project item.

## Q10 — How much do historical documents change?

**Answer: a minimal scrub of the removed alias only, so the records stay
coherent — no fabricated history.** Historical ledgers and reviews keep their
chronology, their decisions and their evidence; the only edit is the removal of
the deleted alias where leaving it would claim a surface that no longer exists.
Anything the scrub cannot state honestly is left to git history rather than
rewritten.

## Q11 — How is this ledger kept inside the token policy?

**Answer: by describing registers and target forms instead of repeating the
legacy spellings (see Q2).** The mapping, the policy and the exemption file
together make the rename fully recoverable without a second file carrying the
legacy names.

## Dispatch record

- The run has six units: `U-S`, `U-J`, `U-M`, `U-R`, `U-V`, `U-P`. `U-S` is
  this unit — task tracking: the Project Component option, the tracking issue,
  this question ledger and the task spec under `docs/todo/`.
- Wave 1 ran three workers concurrently on disjoint paths of the same working
  tree, with the Git index serialized by `flock` so only one worker staged and
  committed at a time.
- The rename sweep is deliberately a single mechanical pass across the tree,
  not a sequence of partial renames: it produces the final token state in one
  go and is the only unit allowed to touch tokens outside its own paths.

## References

- [docs/todo/rename-to-zeze.md](../../todo/rename-to-zeze.md) - Task spec of the rename run.
- [docs/2026-10-22-zeze-migration-instruction.md](../../2026-10-22-zeze-migration-instruction.md) - The single named exemption file (created by the migration unit).
- [Issue #16](https://github.com/CGuiho/zeze/issues/16) - Tracking issue of the rename run.
