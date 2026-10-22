#!/bin/sh
# End-to-end test of devops/install.sh, devops/uninstall.sh and (when pwsh is
# available) devops/install.ps1 and devops/uninstall.ps1 against a local
# release catalog. Never touches the real home: every run uses a temporary
# HOME and a local HTTP server (python3) on 127.0.0.1.
#
# usage: devops/e2e-installers.sh [--keep]
#   --keep  keep the temporary directory for inspection
# shellcheck disable=SC2015 # 'test && test || fail' assertions are intended
set -eu
root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
keep=false
[ "${1:-}" = --keep ] && keep=true
command -v python3 >/dev/null 2>&1 || { echo 'e2e-installers: python3 is required for the local release server' >&2; exit 1; }

work=$(mktemp -d "${TMPDIR:-/tmp}/jureuma-e2e-installers.XXXXXX")
server_pid=
finish() {
  status=$?
  [ -z "$server_pid" ] || kill "$server_pid" 2>/dev/null || true
  if [ "$keep" = true ] || [ "$status" -ne 0 ]; then echo "e2e-installers: work directory kept at $work" >&2
  else rm -rf -- "$work"; fi
  exit "$status"
}
trap finish EXIT

pass=0
ok() { pass=$((pass + 1)); printf 'ok %d - %s\n' "$pass" "$*"; }
fail() { printf 'not ok - %s\n' "$*" >&2; exit 1; }

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$(uname -m)" in x86_64|amd64) arch=amd64 ;; aarch64|arm64) arch=arm64 ;; *) fail "unsupported host"; esac
host_target="$os/$arch"

# Releases: 0.1.0 (stable), 0.2.0-beta.1 (beta), 0.3.0 (assets missing),
# 0.4.0 (corrupted payload), 9.9.9 (draft).
srv="$work/srv"
for v in 0.1.0 0.2.0-beta.1 0.4.0; do
  JUREUMA_VERSION=$v JUREUMA_TARGETS="$host_target windows/amd64" "$root/devops/release-prepare.sh" --allow-dev --out "$srv/download/v$v" >/dev/null
done
printf 'corrupted' >> "$srv/download/v0.4.0/jureuma-$os-$arch"

# The fixture asset set is what the installers consume: every fixture must
# carry its released launcher binary for every target it builds.
[ -f "$srv/download/v0.1.0/jureuma-launcher-$os-$arch" ] || fail 'the host fixture is missing the launcher asset'
[ -f "$srv/download/v0.1.0/jureuma-launcher-windows-amd64.exe" ] || fail 'the windows fixture is missing the launcher asset'
ok 'release fixtures carry the released launcher assets (host and windows)'
assets_json() {
  first=true
  printf '['
  for f in "$1"/*; do
    if [ "$first" = true ]; then first=false; else printf ','; fi
    printf '{"name":"%s","uploader":{"login":"x","name":"decoy-%s"},"label":null,"size":1}' "${f##*/}" "${f##*/}"
  done
  printf ']'
}
r999=$(printf '{"tag_name":"v9.9.9","draft":true,"author":{"name":"a"},"assets":%s}' "$(assets_json "$srv/download/v0.1.0")")
r040=$(printf '{"tag_name":"v0.4.0","draft":false,"body":"notes with \\"quotes\\" and [brackets] {braces}","assets":%s}' "$(assets_json "$srv/download/v0.4.0")")
r030='{"tag_name":"v0.3.0","draft":false,"assets":[{"name":"checksums.txt"}]}'
r020=$(printf '{"tag_name":"v0.2.0-beta.1","draft":false,"prerelease":true,"assets":%s}' "$(assets_json "$srv/download/v0.2.0-beta.1")")
r010=$(printf '{"tag_name":"v0.1.0","draft":false,"assets":%s}' "$(assets_json "$srv/download/v0.1.0")")
# releases.json has every case; releases-good.json leaves out the corrupted
# 0.4.0, which would otherwise be the latest stable release.
printf '[%s,%s,%s,%s,%s]' "$r999" "$r040" "$r030" "$r020" "$r010" > "$srv/releases.json"
printf '[%s,%s,%s,%s]' "$r999" "$r030" "$r020" "$r010" > "$srv/releases-good.json"

cat > "$work/server.py" <<'EOF'
import http.server, os, sys, functools
root, portfile = sys.argv[1], sys.argv[2]
class H(http.server.SimpleHTTPRequestHandler):
    def log_message(self, *a): pass
    def translate_path(self, path):
        return super().translate_path(path.split('?', 1)[0])
s = http.server.ThreadingHTTPServer(('127.0.0.1', 0), functools.partial(H, directory=root))
open(portfile, 'w').write(str(s.server_address[1]))
s.serve_forever()
EOF
python3 -I "$work/server.py" "$srv" "$work/port" &
server_pid=$!
i=0
while [ ! -s "$work/port" ]; do i=$((i + 1)); [ "$i" -lt 100 ] || fail 'server did not start'; sleep 0.1; done
port=$(cat "$work/port")
base="http://127.0.0.1:$port"
export NO_PROXY=127.0.0.1 no_proxy=127.0.0.1
export JUREUMA_DOWNLOAD_BASE_URL="$base/download"
export JUREUMA_NO_UPDATE_CHECK=1
unset JUREUMA_USER_HOME || true
clean_path=$(printf '%s' "$PATH" | tr ':' '\n' | grep -v '\.guiho/bin' | paste -sd: -)

# Invocations that do not target a project run from a neutral directory:
# install.sh applies the jureuma AGENTS.md block when the current directory is
# a project root, and this test must never modify the directory it was invoked
# from. The guard at the end of the script verifies that.
neutral="$work/neutral"; mkdir -p "$neutral"
cwd_agents_sum=
[ ! -f "$PWD/AGENTS.md" ] || cwd_agents_sum=$(cksum < "$PWD/AGENTS.md")

run_install() { # home catalog args...
  h=$1; c=$2; shift 2
  env HOME="$h" PATH="$clean_path" JUREUMA_RELEASES_URL="$base/$c" sh "$root/devops/install.sh" "$@"
}
run_uninstall() {
  h=$1; shift
  env HOME="$h" PATH="$clean_path" sh "$root/devops/uninstall.sh" "$@"
}
active_of() { sed -n 's/^{"protocol":1,"active":"\([^"]*\)".*/\1/p' "$1/.guiho/jureuma/current.json"; }

# --- install.sh ---------------------------------------------------------------
H="$work/home-sh"; mkdir -p "$H/.claude"
sh "$root/devops/install.sh" --help | grep -q -- '--channel' || fail 'install.sh --help'
ok 'install.sh --help'
if sh "$root/devops/install.sh" --version 1.0.0 --channel beta >/dev/null 2>&1; then fail '--version with --channel accepted'; fi
ok 'install.sh rejects --version with --channel'
if sh "$root/devops/install.sh" --version not-a-version >/dev/null 2>&1; then fail 'invalid version accepted'; fi
ok 'install.sh rejects an invalid version'

out=$(cd "$neutral" && run_install "$H" releases-good.json --dry-run)
printf '%s\n' "$out" | grep -q 'Version:      0.1.0' || fail "dry run did not resolve 0.1.0: $out"
[ ! -e "$H/.guiho" ] || fail 'dry run created ~/.guiho'
ok 'install.sh --dry-run resolves latest stable and changes nothing'

(cd "$neutral" && run_install "$H" releases-good.json >/dev/null)
[ "$(active_of "$H")" = 0.1.0 ] || fail 'active is not 0.1.0'
[ "$(env HOME="$H" "$H/.guiho/bin/jureuma" --version)" = 0.1.0 ] || fail 'launcher version'
[ -x "$H/.guiho/jureuma/versions/0.1.0/jureuma" ] || fail 'payload missing'
[ "$(cksum < "$H/.guiho/bin/jureuma")" = "$(cksum < "$srv/download/v0.1.0/jureuma-launcher-$os-$arch")" ] || fail 'the installed launcher is not the released launcher binary'
[ -f "$H/.guiho/jureuma/installed-artifacts.json" ] || fail 'installed-artifacts.json missing'
[ -f "$H/.guiho/jureuma/versions/0.1.0/artifacts/skills/guiho-s-jureuma/SKILL.md" ] || fail 'canonical skill missing'
[ -f "$H/.agents/skills/guiho-s-jureuma/SKILL.md" ] && [ -f "$H/.claude/skills/guiho-s-jureuma/SKILL.md" ] || fail 'skill projections missing'
[ ! -e "$H/.codex/skills/guiho-s-jureuma" ] || fail 'codex projection without ~/.codex'
grep -q '\.guiho/bin' "$H/.profile" || fail 'PATH not added to .profile'
[ -z "$(ls -A "$H/.guiho/.temp")" ] || fail 'staging directory left behind'
ok 'install.sh installs 0.1.0 (payload, launcher, current.json, manifest, skills, PATH, staging cleaned)'

(cd "$neutral" && run_install "$H" releases-good.json >/dev/null)
[ "$(grep -c '\.guiho/bin' "$H/.profile")" = 1 ] || fail 'PATH line duplicated'
[ "$(active_of "$H")" = 0.1.0 ] || fail 'reinstall changed the version'
ok 'install.sh reinstall is idempotent'

(cd "$neutral" && run_install "$H" releases.json --channel beta >/dev/null)
[ "$(active_of "$H")" = 0.2.0-beta.1 ] || fail 'beta channel not selected'
grep -q '"previous":"0.1.0"' "$H/.guiho/jureuma/current.json" || fail 'previous not recorded'
ok 'install.sh --channel beta selects 0.2.0-beta.1 and records the previous version'

(cd "$neutral" && run_install "$H" releases.json --version 0.1.0 >/dev/null)
[ "$(active_of "$H")" = 0.1.0 ] || fail 'exact version not installed'
ok 'install.sh --version 0.1.0 installs the exact older version'

before=$(cat "$H/.guiho/jureuma/current.json")
if (cd "$neutral" && run_install "$H" releases.json --version 0.3.0 >/dev/null 2>&1); then fail '0.3.0 (missing assets) installed'; fi
if (cd "$neutral" && run_install "$H" releases.json --version 9.9.9 >/dev/null 2>&1); then fail 'draft installed'; fi
if (cd "$neutral" && run_install "$H" releases.json --channel nightly >/dev/null 2>&1); then fail 'empty channel installed'; fi
if (cd "$neutral" && run_install "$H" releases.json --version 0.4.0 >/dev/null 2>&1); then fail 'corrupted payload installed'; fi
[ "$(cat "$H/.guiho/jureuma/current.json")" = "$before" ] || fail 'failed installs changed current.json'
[ ! -e "$H/.guiho/jureuma/versions/0.4.0" ] || fail 'corrupted version directory left behind'
[ "$(env HOME="$H" "$H/.guiho/bin/jureuma" --version)" = 0.1.0 ] || fail 'launcher broken after failed installs'
[ -z "$(ls -A "$H/.guiho/.temp")" ] || fail 'staging directory left behind after failure'
ok 'install.sh refuses incomplete, draft, unknown-channel and corrupted releases without changing anything'

# A failure after activation (AGENTS.md cannot be written) rolls everything back.
B="$work/broken-project"; mkdir -p "$B/AGENTS.md"; git -C "$B" init -q; printf "pairs: 5\n" > "$B/jureuma.yaml"
launcher_sum=$(cksum < "$H/.guiho/bin/jureuma")
manifest_sum=$(cksum < "$H/.guiho/jureuma/installed-artifacts.json")
if (cd "$B" && env HOME="$H" PATH="$clean_path" JUREUMA_RELEASES_URL="$base/releases.json" sh "$root/devops/install.sh" --channel beta >/dev/null 2>&1); then
  fail 'install into a broken project succeeded'
fi
[ "$(cat "$H/.guiho/jureuma/current.json")" = "$before" ] || fail 'rollback did not restore current.json'
[ "$(cksum < "$H/.guiho/bin/jureuma")" = "$launcher_sum" ] || fail 'rollback did not restore the launcher'
[ "$(cksum < "$H/.guiho/bin/jureuma")" = "$(cksum < "$srv/download/v0.1.0/jureuma-launcher-$os-$arch")" ] || fail 'rollback left a launcher that is not the released 0.1.0 launcher'
[ "$(cksum < "$H/.guiho/jureuma/installed-artifacts.json")" = "$manifest_sum" ] || fail 'rollback did not restore the manifest'
[ -x "$H/.guiho/jureuma/versions/0.2.0-beta.1/jureuma" ] || fail 'rollback lost the retained 0.2.0-beta.1 payload'
[ -f "$H/.agents/skills/guiho-s-jureuma/SKILL.md" ] && [ -f "$H/.claude/skills/guiho-s-jureuma/SKILL.md" ] || fail 'rollback lost the skill copies'
[ "$(env HOME="$H" "$H/.guiho/bin/jureuma" --version)" = 0.1.0 ] || fail 'launcher broken after rollback'
[ -z "$(ls -A "$H/.guiho/.temp")" ] || fail 'staging directory left behind after rollback'
ok 'install.sh rolls back launcher, pointer, manifest, payloads and skills when a late step fails'

# jureuma upgrade: the synchronous transaction in the Go payload.
run_upgrade() { # catalog args...
  c=$1; shift
  (cd "$work" && env HOME="$H" PATH="$clean_path" JUREUMA_RELEASES_URL="$base/$c" "$H/.guiho/bin/jureuma" upgrade "$@")
}
before=$(cat "$H/.guiho/jureuma/current.json")
if run_upgrade releases.json --version 0.4.0 >/dev/null 2>&1; then fail 'upgrade to the corrupted 0.4.0 succeeded'; fi
[ "$(cat "$H/.guiho/jureuma/current.json")" = "$before" ] || fail 'failed upgrade changed current.json'
[ ! -e "$H/.guiho/jureuma/versions/0.4.0" ] || fail 'failed upgrade left a version directory'
[ -z "$(ls -A "$H/.guiho/.temp")" ] || fail 'failed upgrade left staging data'
ok 'jureuma upgrade refuses a corrupted release without changing anything'
if run_upgrade releases.json --version 0.3.0 >/dev/null 2>&1; then fail 'upgrade to 0.3.0 (missing assets) succeeded'; fi
ok 'jureuma upgrade refuses a release without every required asset'
launcher_sum=$(cksum < "$H/.guiho/bin/jureuma")
if (cd "$B" && env HOME="$H" PATH="$clean_path" JUREUMA_RELEASES_URL="$base/releases.json" "$H/.guiho/bin/jureuma" upgrade --channel beta >/dev/null 2>&1); then
  fail 'upgrade from a broken project succeeded'
fi
[ "$(cat "$H/.guiho/jureuma/current.json")" = "$before" ] || fail 'upgrade rollback did not restore current.json'
[ "$(cksum < "$H/.guiho/bin/jureuma")" = "$launcher_sum" ] || fail 'upgrade rollback changed the launcher'
[ "$(cksum < "$H/.guiho/jureuma/installed-artifacts.json")" = "$manifest_sum" ] || fail 'upgrade rollback did not restore the manifest'
[ -f "$H/.agents/skills/guiho-s-jureuma/SKILL.md" ] && [ -f "$H/.claude/skills/guiho-s-jureuma/SKILL.md" ] || fail 'upgrade rollback lost the skill copies'
[ "$(env HOME="$H" "$H/.guiho/bin/jureuma" --version)" = 0.1.0 ] || fail 'launcher broken after upgrade rollback'
[ -z "$(ls -A "$H/.guiho/.temp")" ] || fail 'upgrade rollback left staging data'
ok 'jureuma upgrade rolls back pointer, manifest and skills when a late step fails'
out=$(run_upgrade releases.json --channel beta 2>&1) || fail "jureuma upgrade --channel beta failed: $out"
[ "$(active_of "$H")" = 0.2.0-beta.1 ] || fail 'upgrade did not activate 0.2.0-beta.1'
grep -q '"previous":"0.1.0"' "$H/.guiho/jureuma/current.json" || fail 'upgrade did not record the previous version'
[ "$(env HOME="$H" "$H/.guiho/bin/jureuma" --version)" = 0.2.0-beta.1 ] || fail 'launcher does not run the upgraded payload'
[ -f "$H/.agents/skills/guiho-s-jureuma/SKILL.md" ] && [ -f "$H/.claude/skills/guiho-s-jureuma/SKILL.md" ] || fail 'upgrade lost the skill copies'
grep -q '"version": "0.2.0-beta.1"' "$H/.guiho/jureuma/installed-artifacts.json" || fail 'upgrade did not install the manifest'
[ -z "$(ls -A "$H/.guiho/.temp")" ] || fail 'upgrade left staging data'
printf '%s\n' "$out" | tail -n 1 | grep -q -- '--version 0.2.0-beta.1' || fail 'final recovery block is not pinned to the target'
ok 'jureuma upgrade --channel beta installs, activates and verifies 0.2.0-beta.1'

# The launcher falls back to the previous verified payload.
mv "$H/.guiho/jureuma/versions/0.2.0-beta.1" "$work/moved-payload"
[ "$(env HOME="$H" "$H/.guiho/bin/jureuma" --version 2>/dev/null)" = 0.1.0 ] || fail 'launcher did not fall back to the previous payload'
mv "$work/moved-payload" "$H/.guiho/jureuma/versions/0.2.0-beta.1"
ok 'the launcher falls back to the previous payload when the active one is missing'
(cd "$neutral" && run_install "$H" releases.json --version 0.1.0 >/dev/null)

# JUREUMA_RELEASE_ASSET_DIR: a hermetic install from a local release directory
# (no network): the released launcher and payload are taken from the directory
# and still SHA-256 verified.
A="$work/home-assets"; mkdir -p "$A"
(cd "$neutral" && env HOME="$A" PATH="$clean_path" JUREUMA_RELEASE_ASSET_DIR="$srv/download/v0.1.0" JUREUMA_RELEASE_VERSION=0.1.0 sh "$root/devops/install.sh" >/dev/null)
[ "$(active_of "$A")" = 0.1.0 ] || fail 'asset-dir install did not activate 0.1.0'
[ "$(cksum < "$A/.guiho/bin/jureuma")" = "$(cksum < "$srv/download/v0.1.0/jureuma-launcher-$os-$arch")" ] || fail 'asset-dir install did not install the released launcher'
[ "$(env HOME="$A" "$A/.guiho/bin/jureuma" --version)" = 0.1.0 ] || fail 'asset-dir launcher version'
[ -x "$A/.guiho/jureuma/versions/0.1.0/jureuma" ] || fail 'asset-dir install did not install the payload'
if (cd "$neutral" && env HOME="$A" PATH="$clean_path" JUREUMA_RELEASE_ASSET_DIR="$srv/download/v0.1.0" sh "$root/devops/install.sh" >/dev/null 2>&1); then
  fail 'asset-dir install without an exact version succeeded'
fi
ok 'install.sh JUREUMA_RELEASE_ASSET_DIR installs the released launcher from a local release directory'

# Project integration and uninstall.sh.
P="$work/project"; mkdir -p "$P"; git -C "$P" init -q
printf '# Project\n\nKeep this line.\n' > "$P/AGENTS.md"
(cd "$P" && env HOME="$H" PATH="$clean_path" JUREUMA_RELEASES_URL="$base/releases-good.json" sh "$root/devops/install.sh" >/dev/null)
grep -qF '<!-- >>> jureuma managed block >>> -->' "$P/AGENTS.md" || fail 'AGENTS.md block not applied'
ok 'install.sh applies the AGENTS.md block from a project root'
(cd "$P" && env HOME="$H" PATH="$H/.guiho/bin:$clean_path" jureuma init --agent-evolution always-ask --agent-deny-rules no >/dev/null)
ok 'jureuma init works through the launcher'

out=$(cd "$P" && run_uninstall "$H" --dry-run)
printf '%s\n' "$out" | grep -q 'REMOVE' || fail 'uninstall dry run printed no plan'
[ -x "$H/.guiho/bin/jureuma" ] && [ -f "$P/jureuma.yaml" ] || fail 'uninstall dry run changed something'
ok 'uninstall.sh --dry-run prints the plan and changes nothing'
if (cd "$P" && run_uninstall "$H" </dev/null >/dev/null 2>&1); then fail 'non-interactive uninstall without --yes succeeded'; fi
[ -x "$H/.guiho/bin/jureuma" ] || fail 'refused uninstall changed something'
ok 'uninstall.sh without --yes refuses non-interactively'

(cd "$P" && run_uninstall "$H" --preserve-config --preserve-data --yes </dev/null >/dev/null)
[ ! -e "$H/.guiho/bin/jureuma" ] || fail 'launcher not removed'
[ -f "$P/jureuma.yaml" ] && [ -f "$H/.guiho/jureuma/jureuma.global.yaml" ] || fail 'configuration not preserved'
[ -d "$H/.guiho/jureuma/keys" ] && [ -d "$H/.guiho/jureuma/state" ] || fail 'data not preserved'
[ ! -e "$H/.guiho/jureuma/versions" ] && [ ! -e "$H/.guiho/jureuma/current.json" ] || fail 'payloads not removed'
[ ! -e "$H/.agents/skills/guiho-s-jureuma" ] || fail 'skill copy not removed'
grep -q 'Keep this line.' "$P/AGENTS.md" && ! grep -qF 'jureuma managed block' "$P/AGENTS.md" || fail 'AGENTS.md block handling'
[ -d "$H/.guiho/bin" ] && [ -d "$H/.guiho/.temp" ] || fail 'shared directories removed'
ok 'uninstall.sh (delegated) --preserve-config --preserve-data --yes'

# Native fallback: reinstall, break the payload pointer, uninstall everything.
(cd "$P" && run_install "$H" releases-good.json >/dev/null)
(cd "$P" && JUREUMA_UNINSTALL_NATIVE=1 run_uninstall "$H" --dry-run >/dev/null)
[ -x "$H/.guiho/bin/jureuma" ] || fail 'native dry run changed something'
(cd "$P" && JUREUMA_UNINSTALL_NATIVE=1 run_uninstall "$H" --yes </dev/null >/dev/null)
[ ! -e "$H/.guiho/bin/jureuma" ] && [ ! -e "$P/jureuma.yaml" ] && [ ! -e "$H/.guiho/jureuma/jureuma.global.yaml" ] || fail 'native uninstall left owned files'
[ -d "$H/.guiho/jureuma/keys" ] || fail 'native non-interactive uninstall removed keys'
[ ! -e "$H/.claude/skills/guiho-s-jureuma" ] || fail 'native uninstall left the skill'
grep -q '\.guiho/bin' "$H/.profile" || fail 'uninstall touched PATH'
[ -d "$H/.guiho/bin" ] && [ -d "$H/.guiho/.temp" ] || fail 'native uninstall removed shared directories'
ok 'uninstall.sh native fallback removes everything except keys (non-interactive) and shared directories'

# --- install.ps1 / uninstall.ps1 (pwsh) -------------------------------------
if command -v pwsh >/dev/null 2>&1; then
  W="$work/home-ps"; mkdir -p "$W"
  pw() { h=$1; c=$2; shift 2; (cd "$neutral" && env HOME="$h" PATH="$clean_path" JUREUMA_RELEASES_URL="$base/$c" pwsh -NoProfile -NonInteractive -File "$@"); }
  pw "$W" releases-good.json "$root/devops/install.ps1" -Help | grep -q -- '-Channel' || fail 'install.ps1 -Help'
  pw "$W" releases-good.json "$root/devops/install.ps1" -DryRun | grep -q 'Version:      0.1.0' || fail 'install.ps1 dry run'
  [ ! -e "$W/.guiho" ] || fail 'install.ps1 dry run created ~/.guiho'
  ok 'install.ps1 -DryRun resolves latest stable and changes nothing'
  pw "$W" releases-good.json "$root/devops/install.ps1" -NoModifyPath >/dev/null
  [ "$(env HOME="$W" "$W/.guiho/bin/jureuma" --version)" = 0.1.0 ] || fail 'install.ps1 launcher'
  [ "$(cksum < "$W/.guiho/bin/jureuma")" = "$(cksum < "$srv/download/v0.1.0/jureuma-launcher-$os-$arch")" ] || fail 'install.ps1 launcher is not the released launcher binary'
  pw "$W" releases.json "$root/devops/install.ps1" -Channel beta -NoModifyPath >/dev/null
  [ "$(active_of "$W")" = 0.2.0-beta.1 ] || fail 'install.ps1 channel'
  if pw "$W" releases.json "$root/devops/install.ps1" -Version 0.4.0 -NoModifyPath >/dev/null 2>&1; then fail 'install.ps1 installed a corrupted payload'; fi
  [ "$(active_of "$W")" = 0.2.0-beta.1 ] || fail 'install.ps1 failure changed current.json'
  [ -z "$(ls -A "$W/.guiho/.temp")" ] || fail 'install.ps1 left staging behind'
  ok 'install.ps1 installs, selects channels and rolls back a corrupted release'
  pw "$W" releases.json "$root/devops/uninstall.ps1" -DryRun | grep -q 'REMOVE' || fail 'uninstall.ps1 dry run'
  [ -x "$W/.guiho/bin/jureuma" ] || fail 'uninstall.ps1 dry run changed something'
  (cd "$neutral" && env HOME="$W" PATH="$clean_path" JUREUMA_UNINSTALL_NATIVE=1 pwsh -NoProfile -NonInteractive -File "$root/devops/uninstall.ps1" -Yes) >/dev/null
  [ ! -e "$W/.guiho/bin/jureuma" ] && [ ! -e "$W/.guiho/jureuma/versions" ] || fail 'uninstall.ps1 native'
  [ -d "$W/.guiho/bin" ] || fail 'uninstall.ps1 removed the shared bin directory'
  ok 'uninstall.ps1 dry run and native uninstall'
else
  echo '# pwsh not found: install.ps1 and uninstall.ps1 were not exercised'
fi

if [ -n "$cwd_agents_sum" ]; then
  [ "$(cksum < "$PWD/AGENTS.md")" = "$cwd_agents_sum" ] || fail 'the invoking directory AGENTS.md was modified'
  ok 'the invoking directory AGENTS.md was not modified'
elif [ -e "$PWD/AGENTS.md" ]; then
  fail 'the invoking directory AGENTS.md was created by this test'
fi

echo "e2e-installers: $pass checks passed"
