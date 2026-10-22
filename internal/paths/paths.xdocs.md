---
subject: zeze-paths
description: "Platform-native locations (~/.guiho/zeze, ~/.guiho/bin, ~/.guiho/.temp, ZEZE_USER_HOME override), atomic writes, locks and process liveness."
parent: zeze-internal
children: []
files:
  "paths.go": "Path resolution, atomic writes and locks."
  "alive_unix.go": "Process liveness on Unix."
  "alive_windows.go": "Process liveness on Windows."
  "paths_test.go": "Tests for marker-preserving temp names, home relocation detection, locks and project-root detection."
documents: {}
tags:
  - "go"
  - "filesystem"
keywords:
  - "ZEZE_USER_HOME"
  - "AtomicWrite"
flags: []
status: stable
---

# zeze-paths

Platform-native locations (~/.guiho/zeze, ~/.guiho/bin, ~/.guiho/.temp, ZEZE_USER_HOME override), atomic writes, locks and process liveness.

## Files

- `paths.go`: Path resolution, atomic writes and locks.
- `alive_unix.go`: Process liveness on Unix.
- `alive_windows.go`: Process liveness on Windows.
- `paths_test.go`: Tests for marker-preserving temp names, home relocation detection, locks and project-root detection.
