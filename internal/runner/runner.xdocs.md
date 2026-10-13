---
subject: jureuma-runner
description: "Child-process injection for run: environment building (json, flatten, file formats), private-key stripping, memfd or temp-file documents, output redaction, signal forwarding and exit-code pass-through."
parent: jureuma-internal
children: []
files:
  "runner.go": "Child environment and process supervision."
  "flatten.go": "Flatten names, collisions and the denylist."
  "platform_unix.go": "Unix signal forwarding."
  "platform_windows.go": "Windows console handling."
  "memfd_linux.go": "memfd_create document passing on Linux."
  "memfd_other.go": "Temporary-file fallback elsewhere."
  "memfdnr_linux_amd64.go": "memfd_create syscall number (amd64)."
  "memfdnr_linux_386.go": "memfd_create syscall number (386)."
  "memfdnr_linux_arm.go": "memfd_create syscall number (arm)."
  "memfdnr_linux_generic.go": "memfd_create syscall number (generic Linux architectures)."
  "memfdnr_linux_other.go": "Unknown architectures use the file fallback."
  "runner_test.go": "Runner tests."
documents: {}
tags:
  - "go"
  - "process"
keywords:
  - "JUREUMA_CONFIG"
  - "flatten"
  - "memfd"
  - "redaction"
flags: []
status: stable
---

# jureuma-runner

Child-process injection for run: environment building (json, flatten, file formats), private-key stripping, memfd or temp-file documents, output redaction, signal forwarding and exit-code pass-through.

## Files

- `runner.go`: Child environment and process supervision.
- `flatten.go`: Flatten names, collisions and the denylist.
- `platform_unix.go`: Unix signal forwarding.
- `platform_windows.go`: Windows console handling.
- `memfd_linux.go`: memfd_create document passing on Linux.
- `memfd_other.go`: Temporary-file fallback elsewhere.
- `memfdnr_linux_amd64.go`: memfd_create syscall number (amd64).
- `memfdnr_linux_386.go`: memfd_create syscall number (386).
- `memfdnr_linux_arm.go`: memfd_create syscall number (arm).
- `memfdnr_linux_generic.go`: memfd_create syscall number (generic Linux architectures).
- `memfdnr_linux_other.go`: Unknown architectures use the file fallback.
- `runner_test.go`: Runner tests.
