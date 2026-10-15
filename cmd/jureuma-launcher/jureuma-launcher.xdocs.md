---
subject: jureuma-cmd-jureuma-launcher
description: "The stable launcher entry point installed at ~/.guiho/bin/jureuma (jureuma.exe on Windows): it reads the active version from current.json and starts versions/<active>/jureuma with the original arguments and exact exit code, falling back to the retained previous payload. Built as jureuma-launcher-<os>-<arch>[.exe] release assets and installed by the release installers."
parent: jureuma-cmd
children: []
files:
  "main.go": "Launcher core: strict current.json decoding ({protocol:1, active, previous}), version-name containment inside versions/, payload resolution, fallback to the previous payload and the reinstall hint on every failure."
  "delegate_unix.go": "Unix dispatch: syscall.Exec process replacement with the original arguments, environment and standard streams (exact exit code)."
  "delegate_windows.go": "Windows dispatch: spawn the payload attached to this console, wire stdio, wait and propagate the exact exit code."
  "main_test.go": "Pointer strictness, path-containment and fallback tests with a recorded starter in a temporary JUREUMA_USER_HOME."
documents: {}
tags:
  - "go"
  - "launcher"
keywords:
  - "stable launcher"
  - "current.json"
  - "fallback"
flags: []
status: stable
---

# jureuma-cmd-jureuma-launcher

The stable launcher entry point installed at `~/.guiho/bin/jureuma`
(`jureuma.exe` on Windows): it reads the active version from `current.json`
and starts `versions/<active>/jureuma` with the original arguments and exact
exit code, falling back to the retained previous payload. Built as
`jureuma-launcher-<os>-<arch>[.exe]` release assets and installed by the
release installers.

## Files

- `main.go`: Launcher core: strict current.json decoding ({protocol:1, active, previous}), version-name containment inside versions/, payload resolution, fallback to the previous payload and the reinstall hint on every failure.
- `delegate_unix.go`: Unix dispatch: syscall.Exec process replacement with the original arguments, environment and standard streams (exact exit code).
- `delegate_windows.go`: Windows dispatch: spawn the payload attached to this console, wire stdio, wait and propagate the exact exit code.
- `main_test.go`: Pointer strictness, path-containment and fallback tests with a recorded starter in a temporary JUREUMA_USER_HOME.
