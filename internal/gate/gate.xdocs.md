---
subject: jureuma-gate
description: "Human-only gate for protected operations (terminal checks, agent markers, typed confirmation word) and the audit log."
parent: jureuma-internal
children: []
files:
  "gate.go": "Gate checks and agent-marker detection."
  "audit.go": "Audit log lines without values."
  "gate_test.go": "Gate tests."
documents: {}
tags:
  - "go"
  - "security"
keywords:
  - "human-only"
  - "audit.log"
  - "agent markers"
flags: []
status: stable
---

# jureuma-gate

Human-only gate for protected operations (terminal checks, agent markers, typed confirmation word) and the audit log.

## Files

- `gate.go`: Gate checks and agent-marker detection.
- `audit.go`: Audit log lines without values.
- `gate_test.go`: Gate tests.
