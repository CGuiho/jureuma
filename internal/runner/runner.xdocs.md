---
subject: zeze-runner
description: "Child-process injection for run: environment building (json, flatten, file formats), private-key stripping, memfd or temp-file documents, output redaction, signal forwarding and exit-code pass-through; plus the run-instance registry and OS process inspection that terminate previous-payload processes before an upgrade activation."
parent: zeze-internal
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
  "proc.go": "Old-payload process selection and termination (verified executable paths, never the CLI's own children)."
  "proc_unix.go": "Unix SIGTERM/SIGKILL termination."
  "proc_linux.go": "/proc process inspection on Linux."
  "proc_ps.go": "ps-based process inspection on macOS and the BSDs."
  "proc_windows.go": "PowerShell/CIM process inspection and taskkill on Windows."
  "runner_test.go": "Runner tests."
  "proc_test.go": "Process selection tests."
  "proc_linux_test.go": "Linux /proc inspection and termination tests."
documents: {}
tags:
  - "go"
  - "process"
keywords:
  - "ZEZE_CONFIG"
  - "flatten"
  - "memfd"
  - "redaction"
flags: []
status: stable
---

# zeze-runner

Child-process injection for run: environment building (json, flatten, file formats), private-key stripping, memfd or temp-file documents, output redaction, signal forwarding and exit-code pass-through; plus the run-instance registry and OS process inspection that terminate previous-payload processes before an upgrade activation.

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
- `proc.go`: Old-payload process selection and termination (verified executable paths, never the CLI's own children).
- `proc_unix.go`: Unix SIGTERM/SIGKILL termination.
- `proc_linux.go`: /proc process inspection on Linux.
- `proc_ps.go`: ps-based process inspection on macOS and the BSDs.
- `proc_windows.go`: PowerShell/CIM process inspection and taskkill on Windows.
- `runner_test.go`: Runner tests.
- `proc_test.go`: Process selection tests.
- `proc_linux_test.go`: Linux /proc inspection and termination tests.
