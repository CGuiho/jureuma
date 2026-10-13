---
subject: jureuma-update
description: "SemVer comparison, the paginated GitHub release catalog (15-second timeout), the update cache, the single-flight worker lease and the reinstall commands printed by upgrade."
parent: jureuma-internal
children: []
files:
  "update.go": "Release catalog, selection, cache and lease."
  "update_test.go": "Update tests."
  "release.go": "Release asset downloads, checksums.txt and artifacts.json parsing for the upgrade transaction."
documents: {}
tags:
  - "go"
  - "upgrade"
keywords:
  - "SemVer"
  - "channel"
  - "cache.json"
  - "reinstall"
flags: []
status: stable
---

# jureuma-update

SemVer comparison, the paginated GitHub release catalog (15-second timeout), the update cache, the single-flight worker lease and the reinstall commands printed by upgrade.

## Files

- `update.go`: Release catalog, selection, cache and lease.
- `update_test.go`: Update tests.
- `release.go`: Release asset downloads, checksums.txt and artifacts.json parsing for the upgrade transaction.
