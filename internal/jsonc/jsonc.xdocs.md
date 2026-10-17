---
subject: jureuma-jsonc
description: "Strict JSON and lenient JSONC parsing with value-free error messages (file, line, column, class), plus the comment-preserving lossless JSONC editor."
parent: jureuma-internal
children: []
files:
  "jsonc.go": "Parser."
  "jsonc_test.go": "Parser tests."
  "lossless.go": "Comment-preserving JSONC editor: span tree of every key, value, comma and comment, and Edit which splices minimal byte spans (refusing rather than touching preserved text)."
  "lossless_test.go": "Editor golden tests."
documents: {}
tags:
  - "go"
  - "json"
keywords:
  - "JSONC"
  - "comments"
  - "trailing commas"
flags: []
status: stable
---

# jureuma-jsonc

Strict JSON and lenient JSONC parsing with value-free error messages (file, line, column, class), plus the comment-preserving lossless JSONC editor.

## Files

- `jsonc.go`: Parser.
- `jsonc_test.go`: Parser tests.
- `lossless.go`: Comment-preserving JSONC editor: span tree of every key, value, comma and comment, and Edit which splices minimal byte spans (refusing rather than touching preserved text).
- `lossless_test.go`: Editor golden tests.
