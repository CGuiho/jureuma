#!/bin/sh
# Build jureuma and its stable launcher for the host into bin/jureuma and
# bin/jureuma-launcher (bin/jureuma.exe and bin/jureuma-launcher.exe on
# Windows), with the version from devops/version.sh injected into the payload
# through -ldflags.
set -eu
root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
version=$("$root/devops/version.sh")
exe=jureuma
launcher=jureuma-launcher
if [ "$(go env GOOS)" = windows ]; then exe=jureuma.exe; launcher=jureuma-launcher.exe; fi
mkdir -p "$root/bin"
cd "$root"
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$version" -o "bin/$exe" ./cmd/jureuma
printf 'built bin/%s (%s)\n' "$exe" "$version"
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "bin/$launcher" ./cmd/jureuma-launcher
printf 'built bin/%s\n' "$launcher"
