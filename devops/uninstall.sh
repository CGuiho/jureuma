#!/bin/sh
# jureuma uninstaller for macOS and Linux (POSIX sh).
#
#   curl -fsSL https://raw.githubusercontent.com/CGuiho/jureuma/main/devops/uninstall.sh | sh -s -- --dry-run
#   curl -fsSL .../uninstall.sh | sh -s -- --yes
#   curl -fsSL .../uninstall.sh | sh -s -- --preserve-config --preserve-data --yes
#
# Same contract as `jureuma uninstall`: by default every jureuma-owned artifact
# is removed (launcher, versioned payloads, ~/.guiho/jureuma with
# configuration, state, audit log and caches, the guiho-s-jureuma skill copies,
# the jureuma block in the project's AGENTS.md, the deny rules jureuma added
# and the project's jureuma.yaml). Private keys (~/.guiho/jureuma/keys) are
# removed only after the human-only confirmation in an interactive terminal; a
# non-interactive run preserves them. The project .gitignore block, the shared
# ~/.guiho, ~/.guiho/bin and ~/.guiho/.temp directories and PATH are never
# touched. Run it from the project root so the project items are found.
#
# When the installed payload works, this script delegates to it
# (`jureuma uninstall`); otherwise it applies the same plan itself.
set -eu

SKILL_ID=guiho-s-jureuma
BEGIN_MARKER='<!-- >>> jureuma managed block >>> -->'
END_MARKER='<!-- <<< jureuma managed block <<< -->'
AGENT_MARKERS='CLAUDECODE CLAUDE_CODE_ENTRYPOINT CODEX_SANDBOX CODEX_HOME CURSOR_AGENT CURSOR_TRACE_ID GEMINI_CLI AIDER OPENCODE AMP_AGENT CLINE GOOSE_SESSION WINDSURF_AGENT COPILOT_AGENT JUREUMA_AGENT'
SEMVER_RE='^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$'

usage() {
  cat <<'EOF'
Uninstall jureuma, the GUIHO agent-safe configuration CLI.

Usage: uninstall.sh [--preserve-config] [--preserve-data] [--dry-run] [--yes]

By default uninstallation removes ALL jureuma-owned data: the launcher, every
installed version, ~/.guiho/jureuma (global configuration, state, audit log,
caches), the guiho-s-jureuma skill copies, the jureuma block in AGENTS.md and the
project's jureuma.yaml. Private keys are removed only after a human confirms
in an interactive terminal; back them up first with `jureuma key export`.

Options:
  --preserve-config  keep jureuma.global.yaml and the project's jureuma.yaml
  --preserve-data    keep keys/, state/ and audit.log
  --dry-run          print the REMOVE / PRESERVE plan; change nothing
  --yes              confirm the plan without a prompt (keys still need a human)
  --help             show this help
EOF
}

say() { printf '%s\n' "$*"; }
die() { printf 'error: %s\n' "$*" >&2; exit 1; }

preserve_config=false
preserve_data=false
dry_run=false
yes=false
while [ "$#" -gt 0 ]; do
  case "$1" in
    --preserve-config) preserve_config=true ;;
    --preserve-data) preserve_data=true ;;
    --dry-run) dry_run=true ;;
    --yes) yes=true ;;
    --help) usage; exit 0 ;;
    *) usage >&2; printf 'error: unknown argument: %s\n' "$1" >&2; exit 2 ;;
  esac
  shift
done

[ -n "${HOME:-}" ] || die 'HOME is not set'
case "$HOME" in /*) ;; *) die "HOME is not an absolute path: $HOME" ;; esac
USER_HOME=${HOME%/}
[ -n "$USER_HOME" ] || die 'HOME must not be /'
GUIHO="$USER_HOME/.guiho"
BIN="$GUIHO/bin"
JHOME="$GUIHO/jureuma"
LAUNCHER="$BIN/jureuma"

# Use a terminal for confirmations when the script itself arrives on stdin
# (curl ... | sh) and a terminal is attached.
use_tty=false
if [ ! -t 0 ] && [ -t 2 ] && (exec </dev/tty) 2>/dev/null; then use_tty=true; fi

# Delegate to the installed payload when it runs.
payload=
if [ -z "${JUREUMA_UNINSTALL_NATIVE:-}" ] && [ -f "$JHOME/current.json" ]; then
  active=$(sed -n 's/^{"protocol":1,"active":"\([^"]*\)".*/\1/p' "$JHOME/current.json" | head -n 1)
  if [ -n "$active" ] && printf '%s\n' "$active" | grep -Eq "$SEMVER_RE" && [ -x "$JHOME/versions/$active/jureuma" ]; then
    payload="$JHOME/versions/$active/jureuma"
    JUREUMA_NO_UPDATE_CHECK=1 "$payload" --version >/dev/null 2>&1 || payload=
  fi
fi
if [ -n "$payload" ]; then
  set --
  [ "$preserve_config" = false ] || set -- "$@" --preserve-config
  [ "$preserve_data" = false ] || set -- "$@" --preserve-data
  [ "$dry_run" = false ] || set -- "$@" --dry-run
  [ "$yes" = false ] || set -- "$@" --yes
  say "Using the installed jureuma payload ($payload)."
  status=0
  if [ "$use_tty" = true ]; then
    JUREUMA_NO_UPDATE_CHECK=1 "$payload" uninstall "$@" </dev/tty || status=$?
  else
    JUREUMA_NO_UPDATE_CHECK=1 "$payload" uninstall "$@" || status=$?
  fi
  exit "$status"
fi

say 'The installed jureuma payload is unavailable; applying the uninstall plan directly.'

# Project root: nearest ancestor with jureuma.yaml, else with .git, else $PWD.
find_up() {
  d=$PWD
  while :; do
    if [ -e "$d/$1" ]; then printf '%s\n' "$d"; return 0; fi
    [ "$d" = / ] && return 1
    d=$(dirname -- "$d")
  done
}
ROOT=$(find_up jureuma.yaml || find_up .git || printf '%s\n' "$PWD")

is_agent() {
  for m in $AGENT_MARKERS; do
    eval "v=\${$m:-}"
    [ -z "$v" ] || return 0
  done
  return 1
}
interactive() {
  if [ "$use_tty" = true ]; then return 0; fi
  [ -t 0 ] && [ -t 2 ]
}

# Plan: lines of "R|P<TAB>kind<TAB>path<TAB>reason".
TAB=$(printf '\t')
plan=
add() { plan="$plan$1$TAB$2$TAB$3$TAB$4
"; }

deny_state="$JHOME/state/deny-rules.json"
if [ -f "$deny_state" ] && grep -q "\"$ROOT\"" "$deny_state" 2>/dev/null; then
  add P deny "$ROOT/.claude/settings.json" 'jureuma deny rules need the jureuma payload to be removed exactly; remove the *.unencrypted.* deny entries jureuma added by hand'
  add P deny "$ROOT/opencode.json" 'jureuma deny rules need the jureuma payload to be removed exactly; remove the *.unencrypted.* deny entries jureuma added by hand'
fi
skill_roots="$USER_HOME/.agents/skills"
[ -d "$USER_HOME/.claude" ] && skill_roots="$USER_HOME/.claude/skills $skill_roots"
[ -d "$USER_HOME/.codex" ] && skill_roots="$skill_roots $USER_HOME/.codex/skills"
for r in $skill_roots; do
  [ -d "$r/$SKILL_ID" ] && add R dir "$r/$SKILL_ID" 'managed skill copy'
done
if [ -f "$ROOT/AGENTS.md" ] && grep -qxF "$BEGIN_MARKER" "$ROOT/AGENTS.md"; then
  add R block "$ROOT/AGENTS.md" 'jureuma managed block only; the rest of AGENTS.md is kept'
fi
if [ -f "$ROOT/jureuma.yaml" ]; then
  if [ "$preserve_config" = true ]; then add P file "$ROOT/jureuma.yaml" '--preserve-config'
  else add R file "$ROOT/jureuma.yaml" 'project configuration'; fi
fi
if [ -f "$ROOT/.gitignore" ] && grep -q 'jureuma' "$ROOT/.gitignore"; then
  add P file "$ROOT/.gitignore" 'jureuma block is persistent project data: plaintext files may outlive jureuma'
fi
[ -e "$LAUNCHER" ] && add R file "$LAUNCHER" 'stable launcher'
preserved_in_home=false
remove_keys=false
if [ -d "$JHOME" ]; then
  for p in "$JHOME"/* "$JHOME"/.[!.]* "$JHOME"/..?*; do
    [ -e "$p" ] || [ -L "$p" ] || continue
    name=${p##*/}
    case "$name" in
      keys) continue ;;
      jureuma.global.yaml)
        if [ "$preserve_config" = true ]; then add P path "$p" '--preserve-config'; preserved_in_home=true
        else add R path "$p" 'global configuration'; fi ;;
      state|audit.log)
        if [ "$preserve_data" = true ]; then add P path "$p" '--preserve-data'; preserved_in_home=true
        else add R path "$p" 'persistent data'; fi ;;
      *) add R path "$p" 'installation, cache or disposable state' ;;
    esac
  done
  if [ -e "$JHOME/keys" ]; then
    if [ "$preserve_data" = true ]; then
      add P keys "$JHOME/keys" '--preserve-data'; preserved_in_home=true
    elif ! interactive || is_agent; then
      add P keys "$JHOME/keys" 'private keys are removed only through the human-only confirmation (non-interactive run)'; preserved_in_home=true
    else
      remove_keys=true
      add R keys "$JHOME/keys" 'private keys; requires the human-only confirmation'
    fi
  fi
fi

say 'jureuma uninstall plan'
for label in REMOVE PRESERVE; do
  say "$label"
  tag=R; [ "$label" = PRESERVE ] && tag=P
  lines=$(printf '%s' "$plan" | awk -F "$TAB" -v t="$tag" '$1 == t { printf "  %s  (%s)\n", $3, $4 }')
  if [ -n "$lines" ]; then say "$lines"; else say '  (nothing)'; fi
done
if [ -d "$JHOME/keys" ]; then
  say "back up private keys first with: jureuma key export <keyId>"
fi
if [ "$dry_run" = true ]; then
  say 'dry run: nothing was changed'
  exit 0
fi
if [ "$yes" != true ]; then
  interactive || die 'uninstall needs confirmation: run it in an interactive terminal or pass --yes (nothing was changed)'
  printf 'Type "uninstall" to remove the items listed under REMOVE: ' >&2
  if [ "$use_tty" = true ]; then read -r answer </dev/tty || answer=; else read -r answer || answer=; fi
  [ "$answer" = uninstall ] || die 'uninstall cancelled; nothing was changed'
fi

remove_block() {
  file=$1
  tmp="$file.jureuma-uninstall.$$"
  awk -v b="$BEGIN_MARKER" -v e="$END_MARKER" '
    $0 == b { skip = 1; next }
    skip && $0 == e { skip = 0; next }
    !skip { print }
  ' "$file" > "$tmp" && mv -- "$tmp" "$file"
}

failed=0
printf '%s' "$plan" | awk -F "$TAB" '$1 == "R" && $2 != "keys" { print $2 "\t" $3 }' > "${TMPDIR:-/tmp}/jureuma-uninstall-plan.$$"
while IFS="$TAB" read -r kind path; do
  case "$path" in
    "$JHOME"/?*|"$BIN/jureuma"|"$ROOT/jureuma.yaml"|"$ROOT/AGENTS.md"|*/skills/"$SKILL_ID") ;;
    *) printf 'error: refusing to remove unexpected path %s\n' "$path" >&2; failed=$((failed + 1)); continue ;;
  esac
  if case "$kind" in block) remove_block "$path" ;; *) rm -rf -- "$path" ;; esac; then
    say "removed $path"
  else
    printf 'error: could not remove %s\n' "$path" >&2
    failed=$((failed + 1))
  fi
done < "${TMPDIR:-/tmp}/jureuma-uninstall-plan.$$"
rm -f -- "${TMPDIR:-/tmp}/jureuma-uninstall-plan.$$"

if [ "$remove_keys" = true ]; then
  word="$(od -An -N2 -tu2 /dev/urandom | tr -d ' ')"
  word="keys-$word"
  printf 'Type "%s" to permanently delete the private keys in %s: ' "$word" "$JHOME/keys" >&2
  if [ "$use_tty" = true ]; then read -r answer </dev/tty || answer=; else read -r answer || answer=; fi
  if [ "$answer" = "$word" ]; then
    if rm -rf -- "$JHOME/keys"; then say "removed $JHOME/keys"; else failed=$((failed + 1)); fi
  else
    say "preserved $JHOME/keys (the human-only confirmation was not given)"
    preserved_in_home=true
  fi
fi
if [ "$preserved_in_home" = false ] && [ -d "$JHOME" ]; then
  if rmdir -- "$JHOME" 2>/dev/null; then say "removed $JHOME"; fi
fi
[ "$failed" -eq 0 ] || die "$failed item(s) could not be removed"
say 'jureuma was uninstalled; ~/.guiho, ~/.guiho/bin, ~/.guiho/.temp and PATH were left untouched'
