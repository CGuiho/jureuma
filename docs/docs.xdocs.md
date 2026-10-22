---
subject: jureuma-docs
description: "Design and implementation-contract documents for jureuma, plus the planning, question-ledger, review, validation and todo trees of its Mandume lifecycle."
parent: jureuma
children:
  - jureuma-docs-plans
  - jureuma-docs-questions
  - jureuma-docs-reviews
  - jureuma-docs-validation
  - jureuma-docs-todo
files: {}
documents:
  "2026-10-13-jureuma-design.md": "Behavioural design: file model, cryptography, agent-safety model, run injection, lifecycle, commands, configuration and owned paths."
  "2026-10-13-jureuma-implementation-contracts.md": "Go package layout and exported API contracts of every internal package."
  "2026-10-22-zeze-migration-instruction.md": "One-shot machine migration instruction for the zeze rename: back up and carry the private keys and the whole CLI home across, replace the launcher, config files, managed blocks, agent-tool deny rules, environment variables and agent artifacts, verify everything, and roll back safely."
tags:
  - "docs"
  - "design"
keywords:
  - "design"
  - "contracts"
flags: []
status: stable
---

# jureuma-docs

Design and implementation-contract documents for jureuma. The `plans/`,
`questions/`, `reviews/`, `validation/` and `todo/` trees record the Mandume
lifecycle and the completion archive of this repository.

## Documents

- [2026-10-13-jureuma-design.md](2026-10-13-jureuma-design.md): Behavioural design: file model, cryptography, agent-safety model, run injection, lifecycle, commands, configuration and owned paths.
- [2026-10-13-jureuma-implementation-contracts.md](2026-10-13-jureuma-implementation-contracts.md): Go package layout and exported API contracts of every internal package.
- [2026-10-22-zeze-migration-instruction.md](2026-10-22-zeze-migration-instruction.md): One-shot machine migration instruction for the zeze rename: back up and carry the private keys and the whole CLI home across, replace the launcher, config files, managed blocks, agent-tool deny rules, environment variables and agent artifacts, verify everything, and roll back safely.
