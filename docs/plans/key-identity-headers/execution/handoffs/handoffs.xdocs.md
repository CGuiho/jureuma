---
subject: zeze-docs-plans-key-identity-headers-execution-handoffs
description: "Self-contained worker briefs for the key-identity train, one per executable unit (issues #13 and #14)."
parent: zeze-docs-plans-key-identity-headers-execution
children: []
files: {}
documents:
  "w1-key-labels-and-selection.md": "Workhorse brief for unit W1 (issue #13, part 1): keystore labels, `key list`, label on `key show`, TTY-safe stored-key picker helper."
  "w2-pair-identity-headers.md": "Workhorse brief for unit W2 (issue #13, part 2): format-2 `$zeze` identity header on both files, the always-present `$schema` pointer and embedded encrypted schema, consistency checks and legacy-repair behavior."
  "w3-single-file-flows-and-init.md": "Workhorse brief for unit W3 (issue #13, parts 3-4): only-unencrypted/only-encrypted flows, `init` header and `$schema` repair, stored-key selection and the schema lifecycle (embed, restore, refresh)."
  "w4-project-key-binding.md": "Workhorse brief for unit W4 (issue #14): the optional top-level `key:` binding in `zeze.yaml`, its enforcement against pair headers, and the byte-preserving write used by `init` and `key rotate`."
  "w5-picker-enter-default.md": "Workhorse brief for the 0.3.1 fix (issue #15): make Enter select the picker's default `create a new key` entry and render that entry honestly."
tags:
  - "handoffs"
  - "workers"
keywords:
  - "worker briefs"
flags: []
status: stable
---

# zeze-docs-plans-key-identity-headers-execution-handoffs

Self-contained briefs handed to native background workers, one per executable
unit. Each brief names mode (dnd), role, model, ownership, acceptance and the
verification and commit contracts.
