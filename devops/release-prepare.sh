#!/bin/sh
# Prepare the complete zeze release artifact set in dist/ (or --out <dir>).
#
# usage: devops/release-prepare.sh [--out <dir>] [--allow-dev]
#   --out <dir>   output directory (default dist/; it is recreated)
#   --allow-dev   accept a 0.0.0-dev or untagged version (local testing only)
#
# The version comes from devops/version.sh --release (exact v<semver> tag at
# HEAD, or ZEZE_VERSION). ZEZE_TARGETS narrows the binary targets for
# local testing; a published release must contain all six.
#
# Output (all listed in artifacts.json and checksums.txt):
#   zeze-<os>-<arch>[.exe]      native CLI payloads
#   zeze-launcher-<os>-<arch>[.exe]      stable launcher binaries
#   guiho-s-zeze.zip              main agent skill (directory guiho-s-zeze/)
#   guiho-p-zeze.md, guiho-p-zeze-install.md, guiho-p-zeze-uninstall.md  prompts
#   zeze-instruction.md         agent instruction source (AGENTS.md block)
#   zeze.schema.json, zeze.global.schema.json                  schemas
#   zeze.example.yaml, zeze.global.example.yaml                examples
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
  ZEZE_VERSION=$version "$root/devops/cross-build.sh" --out "$out"
else
  ZEZE_VERSION=$version "$root/devops/cross-build.sh" --release --out "$out"
fi

assets="$root/internal/agentres/assets"
(cd "$assets/skills" && zip -qrX "$out/guiho-s-zeze.zip" guiho-s-zeze -x '*.xdocs.md')
for p in guiho-p-zeze guiho-p-zeze-install guiho-p-zeze-uninstall; do
  cp -- "$assets/prompts/$p.md" "$out/$p.md"
done
cp -- "$assets/instruction.md" "$out/zeze-instruction.md"
cp -- "$root/internal/config/schemas/zeze.schema.json" "$out/zeze.schema.json"
cp -- "$root/internal/config/schemas/zeze.global.schema.json" "$out/zeze.global.schema.json"
cp -- "$root/internal/config/examples/zeze.yaml" "$out/zeze.example.yaml"
cp -- "$root/internal/config/examples/zeze.global.yaml" "$out/zeze.global.example.yaml"

# artifacts.json: installed paths are relative to ~/.guiho/zeze, except
# launcher artifacts, which install to the shared ~/.guiho/bin command path;
# projections are the managed copies the installers and the CLI maintain.
v="versions/$version"
{
  printf '{\n'
  printf '  "schema": 1,\n'
  printf '  "name": "zeze",\n'
  printf '  "version": "%s",\n' "$version"
  printf '  "tag": "v%s",\n' "$version"
  printf '  "repository": "https://github.com/CGuiho/zeze",\n'
  printf '  "launcher": {"unix": "released-binary", "windows": "released-binary", "assetPattern": "zeze-launcher-<os>-<arch>[.exe]", "path": "$HOME/.guiho/bin/zeze", "pathWindows": "$HOME/.guiho/bin/zeze.exe"},\n'
  printf '  "persistent": ["zeze.global.yaml", "keys/", "state/", "audit.log"],\n'
  printf '  "disposable": ["cache.json", "update-worker.lease", "locks/", "run/"],\n'
  printf '  "artifacts": [\n'
  first=true
  emit() { # id kind asset os arch installedPath projections-json
    if [ "$first" = true ]; then first=false; else printf ',\n'; fi
    printf '    {"id": "%s", "kind": "%s", "asset": "%s", "os": "%s", "arch": "%s", "sha256": "%s", "installedPath": "%s", "projections": %s}' \
      "$1" "$2" "$3" "$4" "$5" "$(sha256 "$out/$3")" "$6" "$7"
  }
  for f in "$out"/zeze-linux-* "$out"/zeze-darwin-* "$out"/zeze-windows-*; do
    [ -f "$f" ] || continue
    name=$(basename -- "$f")
    plat=${name#zeze-}; plat=${plat%.exe}
    os=${plat%-*}; arch=${plat#*-}
    exe=zeze; [ "$os" = windows ] && exe=zeze.exe
    emit "payload-$os-$arch" binary "$name" "$os" "$arch" "$v/$exe" '[]'
  done
  for f in "$out"/zeze-launcher-linux-* "$out"/zeze-launcher-darwin-* "$out"/zeze-launcher-windows-*; do
    [ -f "$f" ] || continue
    name=$(basename -- "$f")
    plat=${name#zeze-launcher-}; plat=${plat%.exe}
    os=${plat%-*}; arch=${plat#*-}
    lpath='$HOME/.guiho/bin/zeze'; [ "$os" = windows ] && lpath="$lpath.exe"
    emit "launcher-$os-$arch" launcher "$name" "$os" "$arch" "$lpath" '[]'
  done
  emit skill-guiho-s-zeze skill guiho-s-zeze.zip any any "$v/artifacts/skills/guiho-s-zeze" \
    '["~/.claude/skills/guiho-s-zeze", "~/.agents/skills/guiho-s-zeze", "~/.codex/skills/guiho-s-zeze"]'
  for p in guiho-p-zeze guiho-p-zeze-install guiho-p-zeze-uninstall; do
    emit "prompt-$p" prompt "$p.md" any any "$v/artifacts/prompts/$p.md" '[]'
  done
  emit instruction instruction zeze-instruction.md any any "$v/artifacts/instruction/zeze-instruction.md" '["<project>/AGENTS.md (zeze managed block)"]'
  emit schema-project schema zeze.schema.json any any "$v/artifacts/schemas/zeze.schema.json" '[]'
  emit schema-global schema zeze.global.schema.json any any "$v/artifacts/schemas/zeze.global.schema.json" '[]'
  emit example-project example zeze.example.yaml any any "$v/artifacts/examples/zeze.example.yaml" '[]'
  emit example-global example zeze.global.example.yaml any any "$v/artifacts/examples/zeze.global.example.yaml" '[]'
  printf '\n  ]\n}\n'
} > "$out/artifacts.json"

(
  cd "$out"
  for f in *; do
    [ "$f" = checksums.txt ] && continue
    printf '%s  %s\n' "$(sha256 "$f")" "$f"
  done
) | LC_ALL=C sort -k2 > "$out/checksums.txt"

"$root/devops/release-verify.sh" --dir "$out" --version "$version" ${ZEZE_TARGETS:+--partial}
printf 'release %s prepared in %s\n' "$version" "$out"
