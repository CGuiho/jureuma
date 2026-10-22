#!/bin/sh
# Keep the version-bearing files that Mirror's adapters cannot write in sync.
#
# usage: devops/version-sync.sh [--check] <version>
#   <version>  the SemVer the files must pin (for example 0.1.0)
#   --check    only verify; exit 1 when a file pins another version
#
# Files:
#   internal/config/examples/zeze.yaml          pinned schema URL comment
#   internal/config/examples/zeze.global.yaml   pinned schema URL comment
#   internal/agentres/assets/instruction.md        frontmatter version and metadata.version
#   internal/agentres/assets/prompts/guiho-p-zeze.md           (same)
#   internal/agentres/assets/prompts/guiho-p-zeze-install.md   (same)
#   internal/agentres/assets/prompts/guiho-p-zeze-uninstall.md (same)
#   internal/agentres/assets/skills/guiho-s-zeze/SKILL.md      (same)
set -eu
root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
check=false
version=
while [ "$#" -gt 0 ]; do
  case "$1" in
    --check) check=true; shift ;;
    --help) sed -n '2,15p' "$0"; exit 0 ;;
    -*) echo "error: unknown argument: $1" >&2; exit 2 ;;
    *) [ -z "$version" ] || { echo 'error: one version only' >&2; exit 2; }; version=${1#v}; shift ;;
  esac
done
[ -n "$version" ] || { echo 'error: a version is required' >&2; exit 2; }
ZEZE_VERSION=$version "$root/devops/version.sh" >/dev/null

prefix='https://github.com/CGuiho/zeze/releases/download/v'
status=0
for pair in "internal/config/examples/zeze.yaml:zeze.schema.json" \
            "internal/config/examples/zeze.global.yaml:zeze.global.schema.json"; do
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

# Shipped agent artifacts pin the release version in their YAML frontmatter,
# both as the top-level version field and as metadata.version (GUIHO agent
# artifacts convention 0002). Only version fields inside the frontmatter are
# rewritten; the rest of the file is passed through untouched.
for rel in internal/agentres/assets/instruction.md \
           internal/agentres/assets/prompts/guiho-p-zeze.md \
           internal/agentres/assets/prompts/guiho-p-zeze-install.md \
           internal/agentres/assets/prompts/guiho-p-zeze-uninstall.md \
           internal/agentres/assets/skills/guiho-s-zeze/SKILL.md; do
  file="$root/$rel"
  tmp="$file.version-sync.$$"
  rc=0
  awk -v want="$version" '
    NR == 1 {
      if ($0 != "---") exit 4
      print
      fm = 1
      next
    }
    fm == 1 && $0 == "---" { print; fm = 2; next }
    fm == 1 {
      if ($0 ~ /^[[:blank:]]*version: /) {
        found++
        indent = $0
        sub(/version: .*$/, "", indent)
        line = indent "version: \"" want "\""
        if (line != $0) { changed++; $0 = line }
      }
      print
      next
    }
    { print }
    END {
      if (fm != 2) exit 4
      if (found == 0) exit 5
      if (changed > 0) exit 3
    }' "$file" > "$tmp" || rc=$?
  case "$rc" in
    0)
      rm -f "$tmp"
      printf 'ok       %s pins v%s\n' "$rel" "$version"
      ;;
    3)
      if [ "$check" = true ]; then
        rm -f "$tmp"
        printf 'mismatch %s: expected v%s\n' "$rel" "$version" >&2
        status=1
        continue
      fi
      mv -- "$tmp" "$file"
      printf 'updated  %s -> v%s\n' "$rel" "$version"
      ;;
    4)
      rm -f "$tmp"
      printf 'error: %s has no closed YAML frontmatter\n' "$rel" >&2
      exit 1
      ;;
    5)
      rm -f "$tmp"
      printf 'error: %s has no version field in its frontmatter\n' "$rel" >&2
      exit 1
      ;;
    *)
      rm -f "$tmp"
      printf 'error: %s could not be inspected\n' "$rel" >&2
      exit 1
      ;;
  esac
done
exit "$status"
