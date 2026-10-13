#!/bin/sh
# Publish a prepared and verified dist/ as the GitHub release for its tag.
#
# usage: devops/release-publish.sh [--dir <dir>]
#
# Requires: the v<version> tag created by `mirror version apply` and pushed, an
# authenticated gh CLI, and explicit authorization to publish. Prerelease
# versions (any SemVer prerelease) are published as GitHub prereleases.
set -eu
root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
dir="$root/dist"
case "${1:-}" in
  --dir) dir=$2 ;;
  --help) sed -n '2,8p' "$0"; exit 0 ;;
  '') ;;
  *) echo "error: unknown argument: $1" >&2; exit 2 ;;
esac
version=$("$root/devops/version.sh" --release)
"$root/devops/release-verify.sh" --dir "$dir" --version "$version"
pre=
case "$version" in *-*) pre=--prerelease ;; esac
# shellcheck disable=SC2086 # $pre is empty or one flag
gh release create "v$version" --verify-tag --title "jureuma $version" --notes "jureuma $version. Install: see README.md#install." $pre "$dir"/*
