#!/bin/sh
# Print the zeze version for builds.
#
# Order: $ZEZE_VERSION, then the exact v<semver> Git tag at HEAD (created
# only by Mirror), then 0.0.0-dev. With --release, a development version is an
# error, because release artifacts must carry the exact tagged SemVer.
set -eu

SEMVER_RE='^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$'

release=false
case "${1:-}" in
  --release) release=true ;;
  '') ;;
  --help) echo 'usage: devops/version.sh [--release]'; exit 0 ;;
  *) echo "error: unknown argument: $1" >&2; exit 2 ;;
esac

root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
version=${ZEZE_VERSION:-}
if [ -z "$version" ]; then
  tag=$(git -C "$root" describe --tags --exact-match --match 'v[0-9]*' HEAD 2>/dev/null || true)
  version=${tag#v}
fi
if [ -z "$version" ]; then
  if [ "$release" = true ]; then
    echo 'error: no release version: HEAD has no exact v<semver> tag and ZEZE_VERSION is unset' >&2
    exit 1
  fi
  version=0.0.0-dev
fi
version=${version#v}
if ! printf '%s\n' "$version" | grep -Eq "$SEMVER_RE"; then
  echo "error: not a valid SemVer: $version" >&2
  exit 1
fi
if [ "$release" = true ] && [ "$version" = 0.0.0-dev ]; then
  echo 'error: 0.0.0-dev is not a release version' >&2
  exit 1
fi
printf '%s\n' "$version"
