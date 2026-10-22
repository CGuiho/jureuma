#!/bin/sh
# Cross-build zeze and its stable launcher for every supported release
# target into an output directory (default dist/), named zeze-<os>-<arch>[.exe]
# and zeze-launcher-<os>-<arch>[.exe].
#
# usage: devops/cross-build.sh [--release] [--out <dir>]
#   --release  require an exact release version (see devops/version.sh)
# ZEZE_TARGETS overrides the target list (space-separated os/arch pairs).
set -eu
root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
out="$root/dist"
vflag=
while [ "$#" -gt 0 ]; do
  case "$1" in
    --release) vflag=--release; shift ;;
    --out) [ "$#" -ge 2 ] || { echo 'error: --out needs a value' >&2; exit 2; }; out=$2; shift 2 ;;
    --out=*) out=${1#*=}; shift ;;
    --help) sed -n '2,8p' "$0"; exit 0 ;;
    *) echo "error: unknown argument: $1" >&2; exit 2 ;;
  esac
done
version=$("$root/devops/version.sh" $vflag)
targets=${ZEZE_TARGETS:-"linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64"}
mkdir -p "$out"
cd "$root"
for target in $targets; do
  goos=${target%/*}
  goarch=${target#*/}
  name="zeze-$goos-$goarch"
  launcher="zeze-launcher-$goos-$goarch"
  if [ "$goos" = windows ]; then name="$name.exe"; launcher="$launcher.exe"; fi
  CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch go build -trimpath \
    -ldflags "-s -w -X main.version=$version" -o "$out/$name" ./cmd/zeze
  printf 'built %s (%s)\n' "$name" "$version"
  # The stable launcher is release-independent: no version is injected.
  CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch go build -trimpath \
    -ldflags "-s -w" -o "$out/$launcher" ./cmd/zeze-launcher
  printf 'built %s\n' "$launcher"
done
