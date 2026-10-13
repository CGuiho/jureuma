---
subject: jureuma-keystore
description: "Resolves, stores, retires and lists private keys under ~/.guiho/jureuma/keys and from JUREUMA_PRIVATE_KEY variables, with file-permission checks."
parent: jureuma-internal
children: []
files:
  "keystore.go": "Key storage and lookup."
  "file_unix.go": "Unix permission checks."
  "file_windows.go": "Windows file handling."
  "keystore_test.go": "Keystore tests."
  "keystore_unix_test.go": "Unix permission tests."
documents: {}
tags:
  - "go"
  - "crypto"
  - "keys"
keywords:
  - "keys"
  - "JUREUMA_PRIVATE_KEY"
flags: []
status: stable
---

# jureuma-keystore

Resolves, stores, retires and lists private keys under ~/.guiho/jureuma/keys and from JUREUMA_PRIVATE_KEY variables, with file-permission checks.

## Files

- `keystore.go`: Key storage and lookup.
- `file_unix.go`: Unix permission checks.
- `file_windows.go`: Windows file handling.
- `keystore_test.go`: Keystore tests.
- `keystore_unix_test.go`: Unix permission tests.
