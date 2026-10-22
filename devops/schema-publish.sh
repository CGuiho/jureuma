#!/bin/sh
# Publish (or refresh) the configuration JSON Schemas on the existing GitHub
# release v<version>, the URLs pinned by zeze.yaml and zeze.global.yaml:
#   https://github.com/CGuiho/zeze/releases/download/v<version>/zeze.schema.json
#   https://github.com/CGuiho/zeze/releases/download/v<version>/zeze.global.schema.json
#
# usage: devops/schema-publish.sh
# Requires an authenticated gh CLI and explicit authorization to publish. The
# release's checksums.txt already covers both files when it was published with
# devops/release-publish.sh; this only repairs missing schema assets.
set -eu
root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
[ "${1:-}" != --help ] || { sed -n '2,10p' "$0"; exit 0; }
version=$("$root/devops/version.sh" --release)
"$root/devops/version-sync.sh" --check "$version"
gh release upload "v$version" --clobber \
  "$root/internal/config/schemas/zeze.schema.json" \
  "$root/internal/config/schemas/zeze.global.schema.json"
