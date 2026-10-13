#!/bin/sh
# Publish (or refresh) the configuration JSON Schemas on the existing GitHub
# release v<version>, the URLs pinned by jureuma.yaml and jureuma.global.yaml:
#   https://github.com/CGuiho/jureuma/releases/download/v<version>/jureuma.schema.json
#   https://github.com/CGuiho/jureuma/releases/download/v<version>/jureuma.global.schema.json
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
  "$root/internal/config/schemas/jureuma.schema.json" \
  "$root/internal/config/schemas/jureuma.global.schema.json"
