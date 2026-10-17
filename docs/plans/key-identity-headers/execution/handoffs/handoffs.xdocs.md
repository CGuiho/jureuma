---
subject: jureuma-docs-plans-key-identity-headers-execution-handoffs
description: "Self-contained worker briefs for the key-identity train, one per executable unit (issue #13)."
parent: jureuma-docs-plans-key-identity-headers-execution
children: []
files: {}
documents:
  "w1-key-labels-and-selection.md": "Workhorse brief for unit W1 (issue #13, part 1): keystore labels, `key list`, label on `key show`, TTY-safe stored-key picker helper."
  "w2-pair-identity-headers.md": "Workhorse brief for unit W2 (issue #13, part 2): format-2 `$jureuma` identity header on both files, consistency checks and legacy-repair behavior."
  "w3-single-file-flows-and-init.md": "Workhorse brief for unit W3 (issue #13, parts 3-4): only-unencrypted/only-encrypted flows, `init` header repair and stored-key selection."
tags:
  - "handoffs"
  - "workers"
keywords:
  - "worker briefs"
flags: []
status: stable
---

# jureuma-docs-plans-key-identity-headers-execution-handoffs

Self-contained briefs handed to native background workers, one per executable
unit. Each brief names mode (dnd), role, model, ownership, acceptance and the
verification and commit contracts.
