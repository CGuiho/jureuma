---
subject: jureuma-paths
description: "Platform-native locations (~/.guiho/jureuma, ~/.guiho/bin, ~/.guiho/.temp, JUREUMA_USER_HOME override), atomic writes, locks and process liveness."
parent: jureuma-internal
children: []
files:
  "paths.go": "Path resolution, atomic writes and locks."
  "alive_unix.go": "Process liveness on Unix."
  "alive_windows.go": "Process liveness on Windows."
  "paths_test.go": "Tests for marker-preserving temp names and home relocation detection."
documents: {}
tags:
  - "go"
  - "filesystem"
keywords:
  - "JUREUMA_USER_HOME"
  - "AtomicWrite"
flags: []
status: stable
---

# jureuma-paths

Platform-native locations (~/.guiho/jureuma, ~/.guiho/bin, ~/.guiho/.temp, JUREUMA_USER_HOME override), atomic writes, locks and process liveness.

## Files

- `paths.go`: Path resolution, atomic writes and locks.
- `alive_unix.go`: Process liveness on Unix.
- `alive_windows.go`: Process liveness on Windows.
- `paths_test.go`: Tests for marker-preserving temp names and home relocation detection.
