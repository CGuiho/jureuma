#!/bin/sh
# Build zeze and its stable launcher for the host into bin/zeze and
# bin/zeze-launcher (bin/zeze.exe and bin/zeze-launcher.exe on
# Windows), with the version from devops/version.sh injected into the payload
# through -ldflags.
set -eu
root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
version=$("$root/devops/version.sh")
exe=zeze
launcher=zeze-launcher
if [ "$(go env GOOS)" = windows ]; then exe=zeze.exe; launcher=zeze-launcher.exe; fi
mkdir -p "$root/bin"
cd "$root"
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$version" -o "bin/$exe" ./cmd/zeze
printf 'built bin/%s (%s)\n' "$exe" "$version"
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "bin/$launcher" ./cmd/zeze-launcher
printf 'built bin/%s\n' "$launcher"
