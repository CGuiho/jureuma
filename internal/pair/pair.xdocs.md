---
subject: jureuma-pair
description: "Configuration pairs: resolution of encrypted, unencrypted and schema files, encrypt/decrypt, sync state per path, the .gitignore block and git tracking checks."
parent: jureuma-internal
children: []
files:
  "pair.go": "Pair resolution."
  "files.go": "Encrypted-file format and atomic reads/writes."
  "header.go": "The $jureuma identity header: format generation, key reference, home, the $schema pointer and the embedded schema copy."
  "git.go": "Managed .gitignore block and git check-ignore/tracked checks."
  "crypto.go": "Encrypting and decrypting whole documents."
  "mutate.go": "set/unset mutations and pending paths."
  "pair_test.go": "Pair tests."
  "header_test.go": "Identity header, schema copy, full plaintext read/write and identity-consistency tests."
documents: {}
tags:
  - "go"
  - "configuration"
keywords:
  - "encrypted.json"
  - "unencrypted.json"
  - "status"
  - "gitignore"
flags: []
status: stable
---

#### &copy; 2026 [GUIHO](https://guiho.co) as represented by [Cristóvão GUIHO](https://guiho.co/cguiho) All Rights Reserved.

# jureuma-pair

Configuration pairs: resolution of encrypted, unencrypted and schema files, encrypt/decrypt, sync state per path, the .gitignore block and git tracking checks.

## Files

- `pair.go`: Pair resolution.
- `files.go`: Encrypted-file format and atomic reads/writes.
- `header.go`: The $jureuma identity header: format generation, key reference, home, the $schema pointer and the embedded schema copy.
- `git.go`: Managed .gitignore block and git check-ignore/tracked checks.
- `crypto.go`: Encrypting and decrypting whole documents.
- `mutate.go`: set/unset mutations and pending paths.
- `pair_test.go`: Pair tests.
- `header_test.go`: Identity header, schema copy, full plaintext read/write and identity-consistency tests.
