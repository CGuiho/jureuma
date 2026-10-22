#!/bin/sh
# Verify a prepared zeze release directory before publication.
#
# usage: devops/release-verify.sh [--dir <dir>] [--version <semver>] [--partial]
#   --dir      release directory (default dist/)
#   --version  expected version (default: artifacts.json version)
#   --partial  allow a subset of the six binary targets (local testing)
#
# Checks: every mandatory asset exists; every payload has its paired launcher
# asset; checksums.txt is well formed, has no duplicates, covers every file
# except itself and matches every digest; artifacts.json lists every asset with
# the same digest; the host payload (when present) prints exactly the expected
# version.
set -eu
root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
dir="$root/dist"
version=
partial=false
while [ "$#" -gt 0 ]; do
  case "$1" in
    --dir) dir=$2; shift 2 ;;
    --dir=*) dir=${1#*=}; shift ;;
    --version) version=${2#v}; shift 2 ;;
    --version=*) version=${1#*=}; version=${version#v}; shift ;;
    --partial) partial=true; shift ;;
    --help) sed -n '2,13p' "$0"; exit 0 ;;
    *) echo "error: unknown argument: $1" >&2; exit 2 ;;
  esac
done
fail() { printf 'release-verify: %s\n' "$*" >&2; exit 1; }
sha256() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}'
  else shasum -a 256 "$1" | awk '{print $1}'; fi
}
[ -d "$dir" ] || fail "missing release directory $dir"
[ -f "$dir/artifacts.json" ] || fail 'missing artifacts.json'
[ -f "$dir/checksums.txt" ] || fail 'missing checksums.txt'
mv_version=$(sed -n 's/^  "version": "\([^"]*\)",$/\1/p' "$dir/artifacts.json")
[ -n "$mv_version" ] || fail 'artifacts.json has no version'
[ -z "$version" ] || [ "$version" = "$mv_version" ] || fail "artifacts.json version $mv_version != $version"
version=$mv_version

mandatory="guiho-s-zeze.zip guiho-p-zeze.md guiho-p-zeze-install.md guiho-p-zeze-uninstall.md zeze-instruction.md zeze.schema.json zeze.global.schema.json zeze.example.yaml zeze.global.example.yaml artifacts.json"
if [ "$partial" != true ]; then
  mandatory="$mandatory zeze-linux-amd64 zeze-linux-arm64 zeze-darwin-amd64 zeze-darwin-arm64 zeze-windows-amd64.exe zeze-windows-arm64.exe"
  mandatory="$mandatory zeze-launcher-linux-amd64 zeze-launcher-linux-arm64 zeze-launcher-darwin-amd64 zeze-launcher-darwin-arm64 zeze-launcher-windows-amd64.exe zeze-launcher-windows-arm64.exe"
fi
for a in $mandatory; do [ -f "$dir/$a" ] || fail "missing mandatory asset $a"; done

# Every payload present must ship its paired launcher (and the other way).
for f in "$dir"/zeze-*; do
  [ -f "$f" ] || continue
  name=$(basename -- "$f")
  case "$name" in
    zeze-launcher-linux-*|zeze-launcher-darwin-*|zeze-launcher-windows-*)
      twin="zeze-${name#zeze-launcher-}" ;;
    zeze-linux-*|zeze-darwin-*|zeze-windows-*)
      twin="zeze-launcher-${name#zeze-}" ;;
    *) continue ;;
  esac
  [ -f "$dir/$twin" ] || fail "$name has no paired asset $twin"
done

awk '
  NF != 2 || $1 !~ /^[0-9a-f]+$/ || length($1) != 64 { bad = 1 }
  seen[$2]++ { dup = 1 }
  END { if (bad) exit 3; if (dup) exit 4 }
' "$dir/checksums.txt" || fail 'checksums.txt is malformed or has duplicate entries'
count=0
for f in "$dir"/*; do
  name=$(basename -- "$f")
  [ "$name" = checksums.txt ] && continue
  count=$((count + 1))
  want=$(awk -v n="$name" '$2 == n {print $1}' "$dir/checksums.txt")
  [ -n "$want" ] || fail "checksums.txt does not cover $name"
  [ "$want" = "$(sha256 "$f")" ] || fail "checksum mismatch for $name"
  if [ "$name" != artifacts.json ]; then
    line=$(grep -F "\"asset\": \"$name\"" "$dir/artifacts.json" || true)
    [ -n "$line" ] || fail "artifacts.json does not declare $name"
    case "$line" in *"\"sha256\": \"$want\""*) ;; *) fail "artifacts.json digest differs for $name" ;; esac
  fi
done
lines=$(wc -l < "$dir/checksums.txt" | tr -d ' ')
[ "$lines" = "$count" ] || fail 'checksums.txt lists files that are not in the release'
grep -o '"asset": "[^"]*"' "$dir/artifacts.json" | sed 's/"asset": "\(.*\)"/\1/' | while read -r a; do
  [ -f "$dir/$a" ] || fail "artifacts.json declares missing asset $a"
done

os=$(uname -s | tr '[:upper:]' '[:lower:]'); arch=$(uname -m)
case "$arch" in x86_64|amd64) arch=amd64 ;; aarch64|arm64) arch=arm64 ;; esac
host="$dir/zeze-$os-$arch"
if [ -f "$host" ]; then
  got=$(ZEZE_NO_UPDATE_CHECK=1 "$host" --version)
  [ "$got" = "$version" ] || fail "host payload reports $got, expected $version"
fi
printf 'release-verify: %s ok (%s files)\n' "$version" "$count"
