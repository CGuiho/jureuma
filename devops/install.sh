#!/bin/sh
# jureuma installer for macOS and Linux (POSIX sh).
#
#   curl -fsSL https://raw.githubusercontent.com/CGuiho/jureuma/main/devops/install.sh | sh
#   curl -fsSL .../install.sh | sh -s -- --version 0.1.0
#   curl -fsSL .../install.sh | sh -s -- --channel beta
#   curl -fsSL .../install.sh | sh -s -- --dry-run
#
# Installs the verified release payload into the immutable directory
# ~/.guiho/jureuma/versions/<version>/, the stable launcher at
# ~/.guiho/bin/jureuma, the active-version pointer ~/.guiho/jureuma/current.json,
# the installed artifact manifest, the guiho-s-jureuma agent skill copies and
# (inside a project) the jureuma block in AGENTS.md, and adds ~/.guiho/bin to
# PATH. All downloads and verification happen in a unique staging directory
# ~/.guiho/.temp/jureuma-install-<id>/. Any failure rolls back to the previous
# installation. Configuration, keys, state and the audit log are never touched.
#
# Environment overrides (testing and mirrors only):
#   JUREUMA_RELEASES_URL       release catalog API (default: GitHub releases API)
#   JUREUMA_DOWNLOAD_BASE_URL  asset base; assets are <base>/<tag>/<asset>
set -eu

REPO=CGuiho/jureuma
RELEASES_URL=${JUREUMA_RELEASES_URL:-https://api.github.com/repos/$REPO/releases}
DOWNLOAD_BASE=${JUREUMA_DOWNLOAD_BASE_URL:-https://github.com/$REPO/releases/download}
SEMVER_RE='^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$'
AGENT_ASSETS='guiho-s-jureuma.zip jureuma-instruction.md guiho-p-jureuma.md guiho-p-jureuma-install.md guiho-p-jureuma-uninstall.md'
SKILL_ID=guiho-s-jureuma

usage() {
  cat <<'EOF'
Install jureuma, the GUIHO agent-safe configuration CLI.

Usage: install.sh [--version <semver> | --channel <name>] [--dry-run] [--no-modify-path]

Options:
  --version <semver>  install this exact release (for example 0.1.0 or 0.2.0-beta.1)
  --channel <name>    install the newest release of a channel (stable, beta, canary, ...)
  --dry-run           resolve the release and print the plan; change nothing
  --no-modify-path    do not add ~/.guiho/bin to PATH in shell startup files
  --help              show this help

Without --version and --channel the latest stable release is installed.
Re-running the installer performs a complete, safe reinstall.
EOF
}

say() { printf '%s\n' "$*"; }
warn() { printf 'warning: %s\n' "$*" >&2; }
die() { printf 'error: %s\n' "$*" >&2; exit 1; }

want_version=
want_channel=
dry_run=false
modify_path=true
while [ "$#" -gt 0 ]; do
  case "$1" in
    --version) [ "$#" -ge 2 ] || die '--version needs a value'; want_version=$2; shift 2 ;;
    --version=*) want_version=${1#*=}; shift ;;
    --channel) [ "$#" -ge 2 ] || die '--channel needs a value'; want_channel=$2; shift 2 ;;
    --channel=*) want_channel=${1#*=}; shift ;;
    --dry-run) dry_run=true; shift ;;
    --no-modify-path) modify_path=false; shift ;;
    --help) usage; exit 0 ;;
    *) usage >&2; die "unknown argument: $1" ;;
  esac
done
if [ -n "$want_version" ] && [ -n "$want_channel" ]; then
  die '--version and --channel are mutually exclusive'
fi
if [ -n "$want_version" ]; then
  want_version=${want_version#v}
  printf '%s\n' "$want_version" | grep -Eq "$SEMVER_RE" || die "not a valid SemVer version: $want_version"
fi
if [ -n "$want_channel" ]; then
  printf '%s\n' "$want_channel" | grep -Eq '^[0-9A-Za-z-]+$' || die "not a valid channel name: $want_channel"
fi
if [ -z "$want_version" ] && [ -z "$want_channel" ]; then want_channel=stable; fi

for tool in curl unzip awk sed grep mktemp uname chmod cp mv rm mkdir; do
  command -v "$tool" >/dev/null 2>&1 || die "required command not found: $tool"
done
if command -v sha256sum >/dev/null 2>&1; then
  sha256() { sha256sum "$1" | awk '{print $1}'; }
elif command -v shasum >/dev/null 2>&1; then
  sha256() { shasum -a 256 "$1" | awk '{print $1}'; }
else
  die 'required command not found: sha256sum or shasum'
fi

# 1. Home directory and CLI-owned paths.
[ -n "${HOME:-}" ] || die 'HOME is not set'
case "$HOME" in /*) ;; *) die "HOME is not an absolute path: $HOME" ;; esac
[ -d "$HOME" ] || die "HOME does not exist: $HOME"
USER_HOME=${HOME%/}
[ -n "$USER_HOME" ] || die 'HOME must not be /'
GUIHO="$USER_HOME/.guiho"
BIN="$GUIHO/bin"
TEMP_ROOT="$GUIHO/.temp"
JHOME="$GUIHO/jureuma"
VERSIONS="$JHOME/versions"
LAUNCHER="$BIN/jureuma"

# 3. Platform.
os_name=$(uname -s)
machine=$(uname -m)
case "$os_name" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) die "unsupported operating system: $os_name (use install.ps1 on Windows)" ;;
esac
case "$machine" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) die "unsupported architecture: $machine" ;;
esac
if [ "$os" = darwin ] && [ "$arch" = amd64 ] && [ "$(sysctl -n hw.optional.arm64 2>/dev/null || echo 0)" = 1 ]; then
  arch=arm64
fi
ASSET="jureuma-$os-$arch"

# 2. Release selection across every catalog page.
fetch() { curl -fsSL --retry 2 --connect-timeout 15 -H 'Accept: application/vnd.github+json' -H 'User-Agent: jureuma-installer' "$@"; }

# Tokenise GitHub's JSON (one token per line) and emit one line per release:
# <tag>\t<draft>\t<space-separated asset names>, then "#count <n>".
parse_releases() {
  grep -oE '"([^"\\]|\\.)*"|[][{}:,]|true|false|null|-?[0-9][0-9.eE+-]*' | awk '
    function unq(t) { return substr(t, 2, length(t) - 2) }
    BEGIN { sp = 0; n = 0 }
    {
      t = $0
      if (t == "{" || t == "[") {
        sp++; typ[sp] = (t == "{") ? "o" : "a"; ek[sp] = (t == "{"); key[sp] = ""
        if (sp == 2 && typ[sp] == "o") { tag = ""; draft = "false"; assets = "" }
        next
      }
      if (t == "}" || t == "]") {
        if (sp == 2 && typ[sp] == "o") { n++; printf "%s\t%s\t%s\n", tag, draft, assets }
        sp--; next
      }
      if (t == ":") next
      if (t == ",") { if (typ[sp] == "o") ek[sp] = 1; next }
      if (typ[sp] == "o" && ek[sp]) { key[sp] = unq(t); ek[sp] = 0; next }
      if (sp == 2 && key[2] == "tag_name") tag = unq(t)
      if (sp == 2 && key[2] == "draft") draft = t
      if (sp == 4 && key[2] == "assets" && typ[3] == "a" && key[4] == "name") assets = assets " " unq(t)
    }
    END { printf "#count %d\n", n }'
}

# Select the release: exact version, else the highest SemVer of the channel.
# Only releases that carry every required asset are candidates.
select_release() {
  awk -F '\t' -v want_version="$want_version" -v want_channel="$want_channel" -v required="$ASSET checksums.txt artifacts.json $AGENT_ASSETS" '
    function num_cmp(a, b) { if (length(a) != length(b)) return length(a) < length(b) ? -1 : 1; return a < b ? -1 : (a > b ? 1 : 0) }
    function isnum(x) { return x ~ /^[0-9]+$/ }
    function pre_cmp(a, b,   na, nb, i, x, y, c) {
      if (a == b) return 0
      if (a == "") return 1
      if (b == "") return -1
      na = split(a, x, "."); nb = split(b, y, ".")
      for (i = 1; i <= na && i <= nb; i++) {
        if (x[i] == y[i]) continue
        if (isnum(x[i]) && isnum(y[i])) return num_cmp(x[i], y[i])
        if (isnum(x[i])) return -1
        if (isnum(y[i])) return 1
        return x[i] < y[i] ? -1 : 1
      }
      return na < nb ? -1 : (na > nb ? 1 : 0)
    }
    function core(v) { sub(/\+.*/, "", v); return v }
    function vcmp(a, b,   ca, cb, pa, pb, xa, xb, i, c) {
      a = core(a); b = core(b)
      pa = ""; pb = ""
      if (index(a, "-")) { pa = substr(a, index(a, "-") + 1); a = substr(a, 1, index(a, "-") - 1) }
      if (index(b, "-")) { pb = substr(b, index(b, "-") + 1); b = substr(b, 1, index(b, "-") - 1) }
      split(a, xa, "."); split(b, xb, ".")
      for (i = 1; i <= 3; i++) { c = num_cmp(xa[i], xb[i]); if (c != 0) return c }
      return pre_cmp(pa, pb)
    }
    function channel(v,   p) { v = core(v); if (!index(v, "-")) return "stable"; p = substr(v, index(v, "-") + 1); sub(/\..*/, "", p); return p }
    BEGIN { nreq = split(required, req, " "); best = ""; besttag = "" }
    /^#/ { next }
    {
      tag = $1; draft = $2; v = tag; sub(/^v/, "", v)
      if (draft != "false") next
      if (v !~ /^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$/) next
      split("", have); na = split($3, as, " "); for (i = 1; i <= na; i++) have[as[i]] = 1
      ok = 1; for (i = 1; i <= nreq; i++) if (!(req[i] in have)) ok = 0
      if (!ok) next
      if (want_version != "") { if (core(v) == core(want_version)) { best = v; besttag = tag } ; next }
      if (channel(v) != want_channel) next
      if (best == "" || vcmp(v, best) > 0) { best = v; besttag = tag }
    }
    END { if (best != "") printf "%s\t%s\n", besttag, best }'
}

say 'jureuma installer'
say "Resolving the release catalog ($RELEASES_URL)..."
catalog=
page=1
while [ "$page" -le 50 ]; do
  case "$RELEASES_URL" in *\?*) sep='&' ;; *) sep='?' ;; esac
  body=$(fetch "$RELEASES_URL${sep}per_page=100&page=$page") || die "the release catalog request failed (page $page); nothing was changed"
  parsed=$(printf '%s' "$body" | parse_releases)
  count=$(printf '%s\n' "$parsed" | sed -n 's/^#count //p')
  catalog="$catalog
$(printf '%s\n' "$parsed" | grep -v '^#count ' || true)"
  [ "${count:-0}" -ge 100 ] || break
  page=$((page + 1))
done
selected=$(printf '%s\n' "$catalog" | select_release)
if [ -z "$selected" ]; then
  if [ -n "$want_version" ]; then
    die "no published release $want_version with every required asset for $os/$arch; nothing was changed"
  fi
  die "no published $want_channel release with every required asset for $os/$arch; nothing was changed"
fi
TAG=$(printf '%s' "$selected" | cut -f1)
VERSION=$(printf '%s' "$selected" | cut -f2)
printf '%s\n' "$TAG" | grep -Eq '^[0-9A-Za-z.+-]+$' || die 'the release tag contains unexpected characters'
BASE_URL="${DOWNLOAD_BASE%/}/$TAG"

current_active=
if [ -f "$JHOME/current.json" ]; then
  current_active=$(sed -n 's/^{"protocol":1,"active":"\([^"]*\)".*/\1/p' "$JHOME/current.json" | head -n 1)
fi

# 5. Show the plan.
say ''
say "Version:      $VERSION (tag $TAG)"
say "Platform:     $os/$arch"
say "Asset:        $ASSET"
say "Source URL:   $BASE_URL/$ASSET"
say "Payload:      $VERSIONS/$VERSION/jureuma"
say "Launcher:     $LAUNCHER"
say "CLI home:     $JHOME"
if [ -n "$current_active" ]; then say "Installed:    $current_active (reinstall/upgrade; configuration, keys, state and audit log are preserved)"; fi
case ":${PATH:-}:" in
  *":$BIN:"*) path_present=true ;;
  *) path_present=false ;;
esac
if [ "$dry_run" = true ]; then
  say ''
  say 'Dry run: the plan would'
  say "  download and verify $ASSET, artifacts.json, checksums.txt and the agent artifacts"
  say "  install $VERSIONS/$VERSION/ (immutable) and point $JHOME/current.json at it"
  say "  write the stable launcher $LAUNCHER"
  say "  install the $SKILL_ID skill into the supported global agent-skill directories"
  if [ -f "$PWD/AGENTS.md" ] && { [ -e "$PWD/.git" ] || [ -f "$PWD/jureuma.yaml" ]; } || [ -f "$PWD/jureuma.yaml" ]; then
    say "  apply the jureuma block in $PWD/AGENTS.md"
  fi
  if [ "$path_present" = false ] && [ "$modify_path" = true ]; then say "  add $BIN to PATH in your shell startup files"; fi
  say 'Dry run: nothing was changed.'
  exit 0
fi

# 6. Staging directory (unique, CLI-owned, strict child of ~/.guiho/.temp).
mkdir -p "$TEMP_ROOT"
STAGE=$(mktemp -d "$TEMP_ROOT/jureuma-install-XXXXXXXX") || die 'cannot create the staging directory'
case "$STAGE" in
  "$TEMP_ROOT"/jureuma-install-*) ;;
  *) die 'the staging directory is outside ~/.guiho/.temp' ;;
esac
case "$STAGE" in */../*|*/./*) die 'the staging directory path is not canonical' ;; esac
ID=${STAGE##*/jureuma-install-}

# Transaction state for rollback.
committed=false
created_version=false
version_backup=
launcher_backup=
launcher_written=false
current_backup=
current_written=false
manifest_backup=
manifest_written=false
skills_backed_up=
skills_written=
agents_backup=
agents_target=

rollback() {
  say 'Rolling back to the previous installation...' >&2
  if [ -n "$agents_target" ] && [ -n "$agents_backup" ] && [ -f "$agents_backup" ]; then cp -p -- "$agents_backup" "$agents_target" || true; fi
  if [ "$current_written" = true ]; then
    if [ -n "$current_backup" ]; then cp -p -- "$current_backup" "$JHOME/current.json" || true; else rm -f -- "$JHOME/current.json"; fi
  fi
  if [ "$manifest_written" = true ]; then
    if [ -n "$manifest_backup" ]; then cp -p -- "$manifest_backup" "$JHOME/installed-artifacts.json" || true; else rm -f -- "$JHOME/installed-artifacts.json"; fi
  fi
  if [ "$launcher_written" = true ]; then
    if [ -n "$launcher_backup" ]; then cp -p -- "$launcher_backup" "$LAUNCHER" || true; else rm -f -- "$LAUNCHER"; fi
  fi
  for t in $skills_written; do rm -rf -- "$t"; done
  for pair in $skills_backed_up; do
    src=${pair%%=*}; dst=${pair#*=}
    [ -e "$src" ] && { rm -rf -- "$dst"; mv -- "$src" "$dst" || true; }
  done
  if [ "$created_version" = true ]; then rm -rf -- "${VERSIONS:?}/$VERSION"; fi
  if [ -n "$version_backup" ] && [ -d "$version_backup" ]; then mv -- "$version_backup" "$VERSIONS/$VERSION" || true; fi
}

cleanup() {
  status=$?
  if [ "$committed" != true ] && [ "$status" -ne 0 ]; then rollback; fi
  rm -f -- "$JHOME/.current.json.$ID" "$JHOME/.installed-artifacts.json.$ID"
  rm -rf -- "${VERSIONS:?}/.candidate-$ID"
  case "$STAGE" in "$TEMP_ROOT"/jureuma-install-?*) rm -rf -- "$STAGE" ;; esac
  exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
trap 'exit 129' HUP

# 7. Download.
download() {
  case "$1" in */*|..|.) die "unsafe asset name: $1" ;; esac
  fetch -o "$STAGE/dl/$1" "$BASE_URL/$1" || die "download failed: $1"
}
mkdir -p "$STAGE/dl"
say ''
say 'Downloading...'
download checksums.txt
download artifacts.json

# 8. Verify checksums.txt shape, then every artifact.
awk '
  { name = $2; sub(/^\*/, "", name) }
  NF != 2 || tolower($1) !~ /^[0-9a-f]+$/ || length($1) != 64 || name == "" { bad = 1 }
  seen[name]++ { dup = 1 }
  END { if (NR == 0 || bad) exit 3; if (dup) exit 4 }
' "$STAGE/dl/checksums.txt" || die 'checksums.txt is empty, malformed or has duplicate entries'
expected_sum() { awk -v n="$1" '{ name = $2; sub(/^\*/, "", name) } name == n { print tolower($1) }' "$STAGE/dl/checksums.txt"; }
verify() {
  want=$(expected_sum "$1")
  [ -n "$want" ] || die "checksums.txt has no entry for $1"
  got=$(sha256 "$STAGE/dl/$1")
  [ "$want" = "$got" ] || die "SHA-256 mismatch for $1"
}
verify artifacts.json
MANIFEST="$STAGE/dl/artifacts.json"
manifest_version=$(sed -n 's/^  "version": "\([^"]*\)",$/\1/p' "$MANIFEST")
[ "$manifest_version" = "$VERSION" ] || die "artifacts.json declares version ${manifest_version:-none}, expected $VERSION"
# Assets to install: this platform's payload and every non-binary artifact.
install_assets=$(grep '^    {"id": ' "$MANIFEST" | while IFS= read -r line; do
  asset=$(printf '%s\n' "$line" | sed -n 's/.*"asset": "\([^"]*\)".*/\1/p')
  kind=$(printf '%s\n' "$line" | sed -n 's/.*"kind": "\([^"]*\)".*/\1/p')
  if [ "$kind" != binary ] || [ "$asset" = "$ASSET" ]; then printf '%s\n' "$asset"; fi
done)
printf '%s\n' "$install_assets" | grep -qx "$ASSET" || die "artifacts.json does not declare $ASSET"
for a in $AGENT_ASSETS; do printf '%s\n' "$install_assets" | grep -qx "$a" || die "artifacts.json does not declare $a"; done
for a in $install_assets; do
  download "$a"
  verify "$a"
  declared=$(grep -F "\"asset\": \"$a\"" "$MANIFEST" | sed -n 's/.*"sha256": "\([0-9a-f]*\)".*/\1/p')
  [ "$declared" = "$(expected_sum "$a")" ] || die "artifacts.json and checksums.txt disagree about $a"
done
say '[OK] SHA-256 verification complete.'

# Unpack the skill archive (entries must stay inside the archive root).
unzip -Z1 "$STAGE/dl/guiho-s-jureuma.zip" > "$STAGE/zip-list" || die 'cannot list guiho-s-jureuma.zip'
if grep -Eq '(^/|(^|/)\.\.(/|$))' "$STAGE/zip-list" || grep -vq "^$SKILL_ID/" "$STAGE/zip-list"; then
  die 'guiho-s-jureuma.zip contains unexpected paths'
fi
mkdir -p "$STAGE/skill"
unzip -q "$STAGE/dl/guiho-s-jureuma.zip" -d "$STAGE/skill" || die 'cannot unpack guiho-s-jureuma.zip'
[ -f "$STAGE/skill/$SKILL_ID/SKILL.md" ] || die "guiho-s-jureuma.zip is missing $SKILL_ID/SKILL.md"

# 9. Candidate checks.
chmod 0755 "$STAGE/dl/$ASSET"
got=$(JUREUMA_NO_UPDATE_CHECK=1 "$STAGE/dl/$ASSET" --version 2>/dev/null) || die 'the downloaded payload failed to run'
[ "$got" = "$VERSION" ] || die "the downloaded payload reports version $got, expected $VERSION"
JUREUMA_NO_UPDATE_CHECK=1 "$STAGE/dl/$ASSET" __self-test >/dev/null 2>&1 || die 'the downloaded payload failed its self-test'
say "[OK] Payload self-test passed ($VERSION)."

# 10. Immutable version directory (built beside its final place, then renamed).
mkdir -p "$VERSIONS"
CAND="$VERSIONS/.candidate-$ID"
mkdir -p "$CAND/artifacts/skills" "$CAND/artifacts/prompts" "$CAND/artifacts/instruction" "$CAND/artifacts/schemas" "$CAND/artifacts/examples"
cp -- "$STAGE/dl/$ASSET" "$CAND/jureuma"
cp -R -- "$STAGE/skill/$SKILL_ID" "$CAND/artifacts/skills/$SKILL_ID"
for p in guiho-p-jureuma guiho-p-jureuma-install guiho-p-jureuma-uninstall; do cp -- "$STAGE/dl/$p.md" "$CAND/artifacts/prompts/$p.md"; done
cp -- "$STAGE/dl/jureuma-instruction.md" "$CAND/artifacts/instruction/jureuma-instruction.md"
for f in jureuma.schema.json jureuma.global.schema.json; do [ ! -f "$STAGE/dl/$f" ] || cp -- "$STAGE/dl/$f" "$CAND/artifacts/schemas/$f"; done
for f in jureuma.example.yaml jureuma.global.example.yaml; do [ ! -f "$STAGE/dl/$f" ] || cp -- "$STAGE/dl/$f" "$CAND/artifacts/examples/$f"; done
cp -- "$MANIFEST" "$CAND/artifacts/artifacts.json"
cp -- "$STAGE/dl/checksums.txt" "$CAND/artifacts/checksums.txt"
find "$CAND" -type f -exec chmod a-w {} +
chmod 0555 "$CAND/jureuma"
if [ -e "$VERSIONS/$VERSION" ]; then
  version_backup="$STAGE/backup-version"
  mv -- "$VERSIONS/$VERSION" "$version_backup"
fi
mv -- "$CAND" "$VERSIONS/$VERSION"
created_version=true
PAYLOAD="$VERSIONS/$VERSION/jureuma"

# 11. Stable launcher.
cat > "$STAGE/launcher" <<'LAUNCHER_EOF'
#!/bin/sh
# jureuma stable launcher (managed by the jureuma installer; do not edit).
# It runs the payload selected by ~/.guiho/jureuma/current.json with the
# original arguments and standard streams, and returns its exact exit code.
self=$0
case "$self" in */*) ;; *) self=$(command -v -- "$self" 2>/dev/null || printf '%s' "$self") ;; esac
bin_dir=$(CDPATH='' cd -- "$(dirname -- "$self")" 2>/dev/null && pwd) || bin_dir=
home_dir=
if [ -n "$bin_dir" ]; then home_dir="$(dirname -- "$bin_dir")/jureuma"; fi
if [ -z "$home_dir" ] || [ ! -f "$home_dir/current.json" ]; then home_dir="${HOME:-}/.guiho/jureuma"; fi
pointer=$(cat -- "$home_dir/current.json" 2>/dev/null) || pointer=
semver='(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?'
if ! printf '%s\n' "$pointer" | grep -Eqx "\{\"protocol\":1,\"active\":\"$semver\",\"previous\":(null|\"$semver\")\}"; then
  echo 'jureuma: no valid active installation (current.json); reinstall: curl -fsSL https://raw.githubusercontent.com/CGuiho/jureuma/main/devops/install.sh | sh' >&2
  exit 1
fi
active=$(printf '%s\n' "$pointer" | sed 's/^{"protocol":1,"active":"\([^"]*\)".*/\1/')
payload="$home_dir/versions/$active/jureuma"
if [ ! -x "$payload" ]; then
  # Fall back to the previous verified payload when the active one is gone.
  previous=$(printf '%s\n' "$pointer" | sed -n 's/.*"previous":"\([^"]*\)".*/\1/p')
  if [ -n "$previous" ] && [ -x "$home_dir/versions/$previous/jureuma" ]; then
    echo "jureuma: warning: the active payload $active is missing; running the previous version $previous (reinstall: curl -fsSL https://raw.githubusercontent.com/CGuiho/jureuma/main/devops/install.sh | sh)" >&2
    exec "$home_dir/versions/$previous/jureuma" "$@"
  fi
  echo "jureuma: the active payload $active is missing; reinstall: curl -fsSL https://raw.githubusercontent.com/CGuiho/jureuma/main/devops/install.sh | sh" >&2
  exit 1
fi
exec "$payload" "$@"
LAUNCHER_EOF
chmod 0755 "$STAGE/launcher"
mkdir -p "$BIN"
if [ -e "$LAUNCHER" ] || [ -L "$LAUNCHER" ]; then
  launcher_backup="$STAGE/backup-launcher"
  cp -p -- "$LAUNCHER" "$launcher_backup"
fi
launcher_written=true
mv -f -- "$STAGE/launcher" "$LAUNCHER"

# 12. Installed artifact manifest (previous one drives retired-artifact removal).
previous_manifest=
if [ -f "$JHOME/installed-artifacts.json" ]; then
  manifest_backup="$STAGE/backup-installed-artifacts.json"
  cp -p -- "$JHOME/installed-artifacts.json" "$manifest_backup"
  previous_manifest=$manifest_backup
fi
cp -- "$MANIFEST" "$JHOME/.installed-artifacts.json.$ID"
manifest_written=true
mv -f -- "$JHOME/.installed-artifacts.json.$ID" "$JHOME/installed-artifacts.json"

# 13-14. Agent skill projections: replace ours, remove retired ones.
skill_roots="$USER_HOME/.agents/skills"
[ -d "$USER_HOME/.claude" ] && skill_roots="$USER_HOME/.claude/skills $skill_roots"
[ -d "$USER_HOME/.codex" ] && skill_roots="$skill_roots $USER_HOME/.codex/skills"
skill_ids=$SKILL_ID
retired=
if [ -n "$previous_manifest" ]; then
  # shellcheck disable=SC2013 # skill IDs are single words ([A-Za-z0-9._-])
  for old in $(sed -n 's/.*"kind": "skill".*"installedPath": "[^"]*\/skills\/\([A-Za-z0-9._-]*\)".*/\1/p' "$previous_manifest"); do
    [ "$old" = "$SKILL_ID" ] || retired="$retired $old"
  done
fi
n=0
for root in $skill_roots; do
  for id in $skill_ids $retired; do
    target="$root/$id"
    if [ -e "$target" ] || [ -L "$target" ]; then
      n=$((n + 1))
      mv -- "$target" "$STAGE/backup-skill-$n"
      skills_backed_up="$skills_backed_up $STAGE/backup-skill-$n=$target"
    fi
  done
  mkdir -p "$root"
  skills_written="$skills_written $root/$SKILL_ID"
  cp -R -- "$VERSIONS/$VERSION/artifacts/skills/$SKILL_ID" "$root/$SKILL_ID"
  chmod -R u+w "$root/$SKILL_ID"
done

# 17. Activate atomically and verify through the launcher.
previous_active=
if [ -f "$JHOME/current.json" ]; then
  current_backup="$STAGE/backup-current.json"
  cp -p -- "$JHOME/current.json" "$current_backup"
  previous_active=$current_active
  if [ "$previous_active" = "$VERSION" ]; then
    previous_active=$(sed -n 's/.*"previous":"\([^"]*\)".*/\1/p' "$JHOME/current.json")
  fi
fi
if [ -n "$previous_active" ] && printf '%s\n' "$previous_active" | grep -Eq "$SEMVER_RE" && [ -x "$VERSIONS/$previous_active/jureuma" ]; then
  prev_json="\"$previous_active\""
else
  previous_active=
  prev_json=null
fi
printf '{"protocol":1,"active":"%s","previous":%s}\n' "$VERSION" "$prev_json" > "$JHOME/.current.json.$ID"
current_written=true
mv -f -- "$JHOME/.current.json.$ID" "$JHOME/current.json"
got=$(JUREUMA_NO_UPDATE_CHECK=1 "$LAUNCHER" --version 2>/dev/null) || die 'the launcher failed to run the new payload'
[ "$got" = "$VERSION" ] || die "the launcher reports version $got, expected $VERSION"
say "[OK] Activated $VERSION through $LAUNCHER."

# 15. Managed AGENTS.md block, only when run from a project root.
if [ -f "$PWD/jureuma.yaml" ] || { [ -f "$PWD/AGENTS.md" ] && [ -e "$PWD/.git" ]; }; then
  agents_target="$PWD/AGENTS.md"
  if [ -f "$agents_target" ]; then
    agents_backup="$STAGE/backup-AGENTS.md"
    cp -p -- "$agents_target" "$agents_backup"
  fi
  JUREUMA_NO_UPDATE_CHECK=1 "$LAUNCHER" agent instruction apply || die 'could not apply the jureuma block to AGENTS.md'
fi

committed=true

# 16. PATH (user level, idempotent, never removes anything).
path_note=
if [ "$modify_path" = true ]; then
  # shellcheck disable=SC2016 # $HOME and $PATH must expand when the profile runs
  line='export PATH="$HOME/.guiho/bin:$PATH"'
  files="$USER_HOME/.profile"
  for f in .bashrc .bash_profile .zshrc .zprofile; do [ -f "$USER_HOME/$f" ] && files="$files $USER_HOME/$f"; done
  [ "$os" = darwin ] && [ ! -f "$USER_HOME/.zshrc" ] && files="$files $USER_HOME/.zshrc"
  for f in $files; do
    if [ -f "$f" ] && grep -q '\.guiho/bin' "$f"; then continue; fi
    { printf '\n# Added by the jureuma installer (GUIHO shared bin directory).\n'; printf '%s\n' "$line"; } >> "$f" || warn "could not update $f"
    say "[OK] Added ~/.guiho/bin to PATH in $f"
  done
  if [ "$path_present" = false ]; then path_note="Open a new shell, or run: export PATH=\"\$HOME/.guiho/bin:\$PATH\""; fi
elif [ "$path_present" = false ]; then
  path_note="Add ~/.guiho/bin to PATH: export PATH=\"\$HOME/.guiho/bin:\$PATH\""
fi

# Disposable state and obsolete payloads (keep the active and previous ones).
rm -f -- "$JHOME/cache.json" "$JHOME/update-worker.lease"
for d in "$VERSIONS"/*; do
  [ -d "$d" ] || continue
  v=${d##*/}
  printf '%s\n' "$v" | grep -Eq "$SEMVER_RE" || continue
  [ "$v" = "$VERSION" ] && continue
  [ -n "$previous_active" ] && [ "$v" = "$previous_active" ] && continue
  case "$d" in "$VERSIONS"/?*) rm -rf -- "$d" && say "Removed obsolete payload $v" ;; esac
done

# 19. Summary.
say ''
say "[OK] Installed jureuma $VERSION"
say "  launcher:  $LAUNCHER"
say "  payload:   $PAYLOAD"
say "  CLI home:  $JHOME"
for root in $skill_roots; do say "  skill:     $root/$SKILL_ID"; done
[ -z "$agents_target" ] || say "  AGENTS.md: $agents_target (jureuma block)"
[ -z "$path_note" ] || say "$path_note"
say 'Next: run "jureuma init" in your project root, then "jureuma --version".'
