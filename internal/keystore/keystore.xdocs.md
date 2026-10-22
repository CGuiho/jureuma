---
subject: zeze-keystore
description: "Resolves, stores, retires and lists private keys under ~/.guiho/zeze/keys and from ZEZE_PRIVATE_KEY variables, with file-permission checks."
parent: zeze-internal
children: []
files:
  "keystore.go": "Key storage and lookup, with non-secret key labels."
  "file_unix.go": "Unix permission checks."
  "file_windows.go": "Windows file handling."
  "keystore_test.go": "Keystore tests."
  "keystore_label_test.go": "Label round-trip, legacy key files and label listing tests."
  "keystore_unix_test.go": "Unix permission tests."
documents: {}
tags:
  - "go"
  - "crypto"
  - "keys"
keywords:
  - "keys"
  - "ZEZE_PRIVATE_KEY"
flags: []
status: stable
---

# zeze-keystore

Resolves, stores, retires and lists private keys under ~/.guiho/zeze/keys and from ZEZE_PRIVATE_KEY variables, with file-permission checks.

## Files

- `keystore.go`: Key storage and lookup, with non-secret key labels.
- `file_unix.go`: Unix permission checks.
- `file_windows.go`: Windows file handling.
- `keystore_test.go`: Keystore tests.
- `keystore_label_test.go`: Label round-trip, legacy key files and label listing tests.
- `keystore_unix_test.go`: Unix permission tests.
