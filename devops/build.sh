#!/bin/sh
# Build jureuma for the host into bin/jureuma (bin/jureuma.exe on Windows),
# with the version from devops/version.sh injected through -ldflags.
set -eu
root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
version=$("$root/devops/version.sh")
exe=jureuma
if [ "$(go env GOOS)" = windows ]; then exe=jureuma.exe; fi
mkdir -p "$root/bin"
cd "$root"
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$version" -o "bin/$exe" ./cmd/jureuma
printf 'built bin/%s (%s)\n' "$exe" "$version"
