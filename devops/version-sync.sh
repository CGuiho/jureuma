#!/bin/sh
# Keep the version-bearing files that Mirror's adapters cannot write in sync.
#
# usage: devops/version-sync.sh [--check] <version>
#   <version>  the SemVer the files must pin (for example 0.1.0)
#   --check    only verify; exit 1 when a file pins another version
#
# Files: the yaml-language-server schema URLs in
#   internal/config/examples/jureuma.yaml
#   internal/config/examples/jureuma.global.yaml
set -eu
root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
check=false
version=
while [ "$#" -gt 0 ]; do
  case "$1" in
    --check) check=true; shift ;;
    --help) sed -n '2,10p' "$0"; exit 0 ;;
    -*) echo "error: unknown argument: $1" >&2; exit 2 ;;
    *) [ -z "$version" ] || { echo 'error: one version only' >&2; exit 2; }; version=${1#v}; shift ;;
  esac
done
[ -n "$version" ] || { echo 'error: a version is required' >&2; exit 2; }
JUREUMA_VERSION=$version "$root/devops/version.sh" >/dev/null

prefix='https://github.com/CGuiho/jureuma/releases/download/v'
status=0
for pair in "internal/config/examples/jureuma.yaml:jureuma.schema.json" \
            "internal/config/examples/jureuma.global.yaml:jureuma.global.schema.json"; do
  file="$root/${pair%%:*}"
  schema=${pair#*:}
  want="# yaml-language-server: \$schema=$prefix$version/$schema"
  first=$(sed -n '1p' "$file")
  if [ "$first" = "$want" ]; then
    printf 'ok       %s pins v%s\n' "${pair%%:*}" "$version"
    continue
  fi
  if [ "$check" = true ]; then
    printf 'mismatch %s: expected v%s\n' "${pair%%:*}" "$version" >&2
    status=1
    continue
  fi
  case "$first" in
    "# yaml-language-server: \$schema=$prefix"*) ;;
    *) printf 'error: %s does not start with a pinned schema comment\n' "${pair%%:*}" >&2; exit 1 ;;
  esac
  tmp="$file.version-sync.$$"
  { printf '%s\n' "$want"; sed '1d' "$file"; } > "$tmp"
  mv -- "$tmp" "$file"
  printf 'updated  %s -> v%s\n' "${pair%%:*}" "$version"
done
exit "$status"
