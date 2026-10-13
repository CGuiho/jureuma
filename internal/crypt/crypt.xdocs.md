---
subject: jureuma-crypt
description: "Value encryption: X25519 ECIES with key IDs derived from public keys; ciphertext envelopes never contain plaintext."
parent: jureuma-internal
children: []
files:
  "crypt.go": "Key generation, key IDs, encrypt and decrypt of single values."
  "crypt_test.go": "Cryptography tests."
documents: {}
tags:
  - "go"
  - "crypto"
keywords:
  - "X25519"
  - "ECIES"
  - "keyId"
flags: []
status: stable
---

# jureuma-crypt

Value encryption: X25519 ECIES with key IDs derived from public keys; ciphertext envelopes never contain plaintext.

## Files

- `crypt.go`: Key generation, key IDs, encrypt and decrypt of single values.
- `crypt_test.go`: Cryptography tests.
