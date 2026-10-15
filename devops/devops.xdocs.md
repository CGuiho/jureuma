---
subject: jureuma-devops
description: "Lifecycle scripts (install.sh, install.ps1, uninstall.sh, uninstall.ps1) and the build, release, smoke and lint scripts cataloged in runx.yaml. Installers select releases by --version/--channel across every catalog page, verify SHA-256 checksums, stage under ~/.guiho/.temp/jureuma-install-<id>, install immutable payloads in ~/.guiho/jureuma/versions/<version>, write the launcher and current.json, update PATH and roll back on failure."
parent: jureuma
children: []
files:
  "install.sh": "POSIX sh installer for macOS and Linux (release selection, checksum verification, staging, immutable versions directory, stable launcher, current.json, installed-artifacts.json, skill projections, AGENTS.md block, PATH, rollback, --dry-run)."
  "install.ps1": "PowerShell installer with the same outcome as install.sh (Windows primary; also runs under pwsh on macOS and Linux)."
  "uninstall.sh": "POSIX sh uninstaller: delegates to the installed payload (jureuma uninstall) or applies the same REMOVE/PRESERVE plan itself; --preserve-config, --preserve-data, --dry-run, --yes."
  "uninstall.ps1": "PowerShell uninstaller with the same contract as uninstall.sh (-PreserveConfig, -PreserveData, -DryRun, -Yes)."
  "version.sh": "Resolves the build version: JUREUMA_VERSION, else the exact v<semver> tag at HEAD, else 0.0.0-dev (--release forbids the dev version)."
  "version-sync.sh": "Pins (or with --check verifies) the schema URLs in internal/config/examples to a version; the version-bearing files Mirror cannot write."
  "build.sh": "Builds bin/jureuma (version injected) and bin/jureuma-launcher for the host."
  "cross-build.sh": "Builds jureuma-<os>-<arch>[.exe] payloads and jureuma-launcher-<os>-<arch>[.exe] stable launchers for linux/darwin/windows on amd64/arm64 into dist/ (JUREUMA_TARGETS narrows the list)."
  "release-prepare.sh": "Recreates dist/ with every release artifact (payloads, stable launchers, agent assets), writes artifacts.json and checksums.txt and verifies the result."
  "release-verify.sh": "Verifies a release directory: mandatory assets, payload/launcher pairing, checksums.txt shape and digests, artifacts.json consistency, host payload version."
  "release-publish.sh": "Creates the GitHub release for the pushed v<version> tag from a verified dist/ with gh (explicit authorization required)."
  "schema-publish.sh": "Uploads the two configuration JSON Schemas to the GitHub release v<version> (explicit authorization required)."
  "smoke.sh": "Smoke-tests a built binary in a temporary JUREUMA_USER_HOME and git project and fails if planted values leak."
  "e2e-installers.sh": "End-to-end test of the four lifecycle scripts against a local release catalog served on 127.0.0.1, with a temporary HOME."
  "shell-lint.sh": "sh/dash/bash syntax checks, shellcheck when installed, and the PowerShell parser for the .ps1 scripts."
  "fmt-check.sh": "Fails when a Go file under cmd/ or internal/ is not gofmt-formatted."
documents: {}
tags:
  - "devops"
  - "installer"
  - "release"
keywords:
  - "install.sh"
  - "install.ps1"
  - "uninstall"
  - "checksums.txt"
  - "artifacts.json"
  - "current.json"
  - "launcher"
  - "dist"
flags: []
status: stable
---

# jureuma-devops

Lifecycle scripts (install.sh, install.ps1, uninstall.sh, uninstall.ps1) and the build, release, smoke and lint scripts cataloged in runx.yaml. Installers select releases by --version/--channel across every catalog page, verify SHA-256 checksums, stage under ~/.guiho/.temp/jureuma-install-<id>, install immutable payloads in ~/.guiho/jureuma/versions/<version>, write the launcher and current.json, update PATH and roll back on failure.

## Files

- `install.sh`: POSIX sh installer for macOS and Linux (release selection, checksum verification, staging, immutable versions directory, stable launcher, current.json, installed-artifacts.json, skill projections, AGENTS.md block, PATH, rollback, --dry-run).
- `install.ps1`: PowerShell installer with the same outcome as install.sh (Windows primary; also runs under pwsh on macOS and Linux).
- `uninstall.sh`: POSIX sh uninstaller: delegates to the installed payload (jureuma uninstall) or applies the same REMOVE/PRESERVE plan itself; --preserve-config, --preserve-data, --dry-run, --yes.
- `uninstall.ps1`: PowerShell uninstaller with the same contract as uninstall.sh (-PreserveConfig, -PreserveData, -DryRun, -Yes).
- `version.sh`: Resolves the build version: JUREUMA_VERSION, else the exact v<semver> tag at HEAD, else 0.0.0-dev (--release forbids the dev version).
- `version-sync.sh`: Pins (or with --check verifies) the schema URLs in internal/config/examples to a version; the version-bearing files Mirror cannot write.
- `build.sh`: Builds bin/jureuma (version injected) and bin/jureuma-launcher for the host.
- `cross-build.sh`: Builds jureuma-<os>-<arch>[.exe] payloads and jureuma-launcher-<os>-<arch>[.exe] stable launchers for linux/darwin/windows on amd64/arm64 into dist/ (JUREUMA_TARGETS narrows the list).
- `release-prepare.sh`: Recreates dist/ with every release artifact (payloads, stable launchers, agent assets), writes artifacts.json and checksums.txt and verifies the result.
- `release-verify.sh`: Verifies a release directory: mandatory assets, payload/launcher pairing, checksums.txt shape and digests, artifacts.json consistency, host payload version.
- `release-publish.sh`: Creates the GitHub release for the pushed v<version> tag from a verified dist/ with gh (explicit authorization required).
- `schema-publish.sh`: Uploads the two configuration JSON Schemas to the GitHub release v<version> (explicit authorization required).
- `smoke.sh`: Smoke-tests a built binary in a temporary JUREUMA_USER_HOME and git project and fails if planted values leak.
- `e2e-installers.sh`: End-to-end test of the four lifecycle scripts against a local release catalog served on 127.0.0.1, with a temporary HOME.
- `shell-lint.sh`: sh/dash/bash syntax checks, shellcheck when installed, and the PowerShell parser for the .ps1 scripts.
- `fmt-check.sh`: Fails when a Go file under cmd/ or internal/ is not gofmt-formatted.
