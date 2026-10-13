#!/bin/sh
# Prepare the complete jureuma release artifact set in dist/ (or --out <dir>).
#
# usage: devops/release-prepare.sh [--out <dir>] [--allow-dev]
#   --out <dir>   output directory (default dist/; it is recreated)
#   --allow-dev   accept a 0.0.0-dev or untagged version (local testing only)
#
# The version comes from devops/version.sh --release (exact v<semver> tag at
# HEAD, or JUREUMA_VERSION). JUREUMA_TARGETS narrows the binary targets for
# local testing; a published release must contain all six.
#
# Output (all listed in artifacts.json and checksums.txt):
#   jureuma-<os>-<arch>[.exe]      native CLI payloads
#   guiho-s-jureuma.zip              main agent skill (directory guiho-s-jureuma/)
#   guiho-p-jureuma.md, guiho-p-jureuma-install.md, guiho-p-jureuma-uninstall.md  prompts
#   jureuma-instruction.md         agent instruction source (AGENTS.md block)
#   jureuma.schema.json, jureuma.global.schema.json                  schemas
#   jureuma.example.yaml, jureuma.global.example.yaml                examples
#   artifacts.json                 ownership manifest (one artifact per line)
#   checksums.txt                  SHA-256 of every other file
set -eu
root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
out="$root/dist"
allow_dev=false
while [ "$#" -gt 0 ]; do
  case "$1" in
    --out) [ "$#" -ge 2 ] || { echo 'error: --out needs a value' >&2; exit 2; }; out=$2; shift 2 ;;
    --out=*) out=${1#*=}; shift ;;
    --allow-dev) allow_dev=true; shift ;;
    --help) sed -n '2,21p' "$0"; exit 0 ;;
    *) echo "error: unknown argument: $1" >&2; exit 2 ;;
  esac
done
if [ "$allow_dev" = true ]; then
  version=$("$root/devops/version.sh")
else
  version=$("$root/devops/version.sh" --release)
fi

case "$out" in
  /*) ;;
  *) out="$(pwd)/$out" ;;
esac
case "$out" in
  /|"$root"|"$HOME"|"$HOME/") echo "error: refusing to use $out as the output directory" >&2; exit 1 ;;
esac

sha256() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}'
  else shasum -a 256 "$1" | awk '{print $1}'; fi
}
command -v zip >/dev/null 2>&1 || { echo 'error: zip is required' >&2; exit 1; }

if [ "$allow_dev" != true ]; then
  "$root/devops/version-sync.sh" --check "$version"
fi

rm -rf -- "$out"
mkdir -p "$out"
if [ "$allow_dev" = true ]; then
  JUREUMA_VERSION=$version "$root/devops/cross-build.sh" --out "$out"
else
  JUREUMA_VERSION=$version "$root/devops/cross-build.sh" --release --out "$out"
fi

assets="$root/internal/agentres/assets"
(cd "$assets/skills" && zip -qrX "$out/guiho-s-jureuma.zip" guiho-s-jureuma -x '*.xdocs.md')
for p in guiho-p-jureuma guiho-p-jureuma-install guiho-p-jureuma-uninstall; do
  cp -- "$assets/prompts/$p.md" "$out/$p.md"
done
cp -- "$assets/instruction.md" "$out/jureuma-instruction.md"
cp -- "$root/internal/config/schemas/jureuma.schema.json" "$out/jureuma.schema.json"
cp -- "$root/internal/config/schemas/jureuma.global.schema.json" "$out/jureuma.global.schema.json"
cp -- "$root/internal/config/examples/jureuma.yaml" "$out/jureuma.example.yaml"
cp -- "$root/internal/config/examples/jureuma.global.yaml" "$out/jureuma.global.example.yaml"

# artifacts.json: installed paths are relative to ~/.guiho/jureuma; projections
# are the managed copies the installers and the CLI maintain.
v="versions/$version"
{
  printf '{\n'
  printf '  "schema": 1,\n'
  printf '  "name": "jureuma",\n'
  printf '  "version": "%s",\n' "$version"
  printf '  "tag": "v%s",\n' "$version"
  printf '  "repository": "https://github.com/CGuiho/jureuma",\n'
  printf '  "launcher": {"unix": "generated-sh", "windows": "payload-copy", "path": "~/.guiho/bin/jureuma"},\n'
  printf '  "persistent": ["jureuma.global.yaml", "keys/", "state/", "audit.log"],\n'
  printf '  "disposable": ["cache.json", "update-worker.lease", "locks/", "run/"],\n'
  printf '  "artifacts": [\n'
  first=true
  emit() { # id kind asset os arch installedPath projections-json
    if [ "$first" = true ]; then first=false; else printf ',\n'; fi
    printf '    {"id": "%s", "kind": "%s", "asset": "%s", "os": "%s", "arch": "%s", "sha256": "%s", "installedPath": "%s", "projections": %s}' \
      "$1" "$2" "$3" "$4" "$5" "$(sha256 "$out/$3")" "$6" "$7"
  }
  for f in "$out"/jureuma-linux-* "$out"/jureuma-darwin-* "$out"/jureuma-windows-*; do
    [ -f "$f" ] || continue
    name=$(basename -- "$f")
    plat=${name#jureuma-}; plat=${plat%.exe}
    os=${plat%-*}; arch=${plat#*-}
    exe=jureuma; [ "$os" = windows ] && exe=jureuma.exe
    emit "payload-$os-$arch" binary "$name" "$os" "$arch" "$v/$exe" '["~/.guiho/bin/jureuma"]'
  done
  emit skill-guiho-s-jureuma skill guiho-s-jureuma.zip any any "$v/artifacts/skills/guiho-s-jureuma" \
    '["~/.claude/skills/guiho-s-jureuma", "~/.agents/skills/guiho-s-jureuma", "~/.codex/skills/guiho-s-jureuma"]'
  for p in guiho-p-jureuma guiho-p-jureuma-install guiho-p-jureuma-uninstall; do
    emit "prompt-$p" prompt "$p.md" any any "$v/artifacts/prompts/$p.md" '[]'
  done
  emit instruction instruction jureuma-instruction.md any any "$v/artifacts/instruction/jureuma-instruction.md" '["<project>/AGENTS.md (jureuma managed block)"]'
  emit schema-project schema jureuma.schema.json any any "$v/artifacts/schemas/jureuma.schema.json" '[]'
  emit schema-global schema jureuma.global.schema.json any any "$v/artifacts/schemas/jureuma.global.schema.json" '[]'
  emit example-project example jureuma.example.yaml any any "$v/artifacts/examples/jureuma.example.yaml" '[]'
  emit example-global example jureuma.global.example.yaml any any "$v/artifacts/examples/jureuma.global.example.yaml" '[]'
  printf '\n  ]\n}\n'
} > "$out/artifacts.json"

(
  cd "$out"
  for f in *; do
    [ "$f" = checksums.txt ] && continue
    printf '%s  %s\n' "$(sha256 "$f")" "$f"
  done
) | LC_ALL=C sort -k2 > "$out/checksums.txt"

"$root/devops/release-verify.sh" --dir "$out" --version "$version" ${JUREUMA_TARGETS:+--partial}
printf 'release %s prepared in %s\n' "$version" "$out"
