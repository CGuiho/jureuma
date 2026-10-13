---
subject: jureuma-schema
description: "JSON Schema loading, supported-subset checks, validation with value-free issues, and schema infer."
parent: jureuma-internal
children: []
files:
  "schema.go": "Schema loading and compilation."
  "check.go": "Supported-subset and secret-annotation checks."
  "infer.go": "Starter schema inference from structure and types."
  "issues.go": "Value-free validation issues."
  "structure.go": "Structural validation of encrypted documents."
  "walk.go": "Per-value provisional validation."
  "schema_test.go": "Schema tests."
documents: {}
tags:
  - "go"
  - "schema"
keywords:
  - "JSON Schema"
  - "x-jureuma-secret"
  - "schema infer"
flags: []
status: stable
---

# jureuma-schema

JSON Schema loading, supported-subset checks, validation with value-free issues, and schema infer.

## Files

- `schema.go`: Schema loading and compilation.
- `check.go`: Supported-subset and secret-annotation checks.
- `infer.go`: Starter schema inference from structure and types.
- `issues.go`: Value-free validation issues.
- `structure.go`: Structural validation of encrypted documents.
- `walk.go`: Per-value provisional validation.
- `schema_test.go`: Schema tests.
