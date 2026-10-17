#!/bin/sh
# Smoke test of a built jureuma binary in a throw-away home and project.
#
# usage: devops/smoke.sh [<binary>]   (default: bin/jureuma, built by devops/build.sh)
#
# Uses a temporary JUREUMA_USER_HOME and a temporary git project; never touches
# the real home. Plants marker values and fails if any of them appears in
# command output, the audit log, the encrypted file or the schema. Protected
# commands are only exercised without the bypass flag.
# shellcheck disable=SC2016 # child commands expand variables in the child shell
set -eu
root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
bin=${1:-"$root/bin/jureuma"}
case "$bin" in /*) ;; *) bin="$(pwd)/$bin" ;; esac
[ -x "$bin" ] || { echo "smoke: $bin is not executable (run devops/build.sh first)" >&2; exit 1; }

work=$(mktemp -d "${TMPDIR:-/tmp}/jureuma-smoke.XXXXXX")
trap 'rm -rf -- "$work"' EXIT
export JUREUMA_USER_HOME="$work/home"
export HOME="$work/home"
export JUREUMA_NO_UPDATE_CHECK=1
mkdir -p "$JUREUMA_USER_HOME" "$work/project"
cd "$work/project"
git init -q .

secret1=SMOKE-SECRET-1-$$
secret2=SMOKE-SECRET-2-$$
log="$work/output.log"
n=0
step() { # expected-exit description command...
  want=$1; what=$2; shift 2
  n=$((n + 1))
  set +e
  "$@" >>"$log" 2>&1 </dev/null
  got=$?
  set -e
  if [ "$got" -ne "$want" ]; then
    printf 'not ok %d - %s (exit %d, expected %d)\n' "$n" "$what" "$got" "$want" >&2
    tail -n 20 "$log" >&2
    exit 1
  fi
  printf 'ok %d - %s\n' "$n" "$what"
}

step 0 'init' "$bin" init --agent-evolution always-ask --agent-deny-rules yes
printf '{"database":{"url":"postgres://%s@db","pool":5},"apiKey":"%s","debug":false}\n' "$secret1" "$secret1" > config.unencrypted.json
step 0 'schema infer' "$bin" schema infer
step 0 'encrypt' "$bin" encrypt
step 0 'list' "$bin" list
step 0 'status' "$bin" status
n=$((n + 1))
if printf '%s\n' "$secret2" | "$bin" set /apiKey --value-stdin >>"$log" 2>&1; then
  printf 'ok %d - set --value-stdin\n' "$n"
else
  printf 'not ok %d - set --value-stdin\n' "$n" >&2; tail -n 20 "$log" >&2; exit 1
fi
step 0 'set --json' "$bin" set /database/pool --json 20
step 0 'run (json)' "$bin" run -- sh -c 'printf "%s\n" "$JUREUMA_CONFIG"'
step 0 'run (flatten)' "$bin" run --format flatten -- sh -c 'echo "$DATABASE_URL $APIKEY $DATABASE_POOL"'
step 7 'run passes the child exit code' "$bin" run -- sh -c 'exit 7'
step 3 'get is human-only' "$bin" get /apiKey
step 4 'set rejects a schema violation' "$bin" set /database/pool not-a-number
step 0 'check' "$bin" check
step 0 'key show' "$bin" key show
step 2 'unknown flag is a usage error' "$bin" list --no-such-flag
step 0 'uninstall --dry-run' "$bin" uninstall --dry-run
[ "$("$bin" --version)" != '' ] || { echo 'smoke: --version printed nothing' >&2; exit 1; }

for f in "$log" config.encrypted.json config.schema.json "$JUREUMA_USER_HOME/.guiho/jureuma/audit.log"; do
  [ -f "$f" ] || continue
  if grep -q -e "$secret1" -e "$secret2" "$f"; then
    echo "smoke: a planted value leaked into $f" >&2
    exit 1
  fi
done
printf 'ok %d - no planted value in output, audit log, encrypted file or schema\n' $((n + 1))
echo "smoke: passed ($("$bin" --version))"
