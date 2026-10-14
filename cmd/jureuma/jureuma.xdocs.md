---
subject: jureuma-cmd-jureuma
description: "The jureuma executable entry point: delegates to internal/cli and receives its version through -ldflags \"-X main.version=<semver>\" (default 0.0.0-dev)."
parent: jureuma-cmd
children: []
files:
  "main.go": "main package; holds the build-time version variable and exits with cli.Execute(version)."
documents: {}
tags:
  - "go"
  - "entrypoint"
keywords:
  - "main.version"
  - "ldflags"
flags: []
status: stable
---

# jureuma-cmd-jureuma

The jureuma executable entry point: delegates to internal/cli and receives its version through -ldflags "-X main.version=<semver>" (default 0.0.0-dev).

## Files

- `main.go`: main package; holds the build-time version variable and exits with cli.Execute(version).
