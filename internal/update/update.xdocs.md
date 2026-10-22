---
subject: zeze-update
description: "SemVer comparison, the paginated GitHub release catalog (15-second timeout), the update cache, the single-flight worker lease and the reinstall commands printed by upgrade."
parent: zeze-internal
children: []
files:
  "update.go": "Release catalog, selection, cache and lease."
  "update_test.go": "Update tests, including release-catalog pagination."
  "release.go": "Release asset downloads, checksums.txt and artifacts.json parsing for the upgrade transaction."
  "release_test.go": "Tests for asset downloads, checksums.txt and artifacts.json parsing."
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

# zeze-update

SemVer comparison, the paginated GitHub release catalog (15-second timeout), the update cache, the single-flight worker lease and the reinstall commands printed by upgrade.

## Files

- `update.go`: Release catalog, selection, cache and lease.
- `update_test.go`: Update tests, including release-catalog pagination.
- `release.go`: Release asset downloads, checksums.txt and artifacts.json parsing for the upgrade transaction.
- `release_test.go`: Tests for asset downloads, checksums.txt and artifacts.json parsing.
