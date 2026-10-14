#!/bin/sh
# Syntax-check every devops shell script with sh -n (and dash/bash when
# present), parse the PowerShell scripts when pwsh is present, and lint with
# the shellcheck tool when it is installed.
set -eu
root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
status=0
for f in "$root"/devops/*.sh; do
  sh -n "$f" || status=1
  if command -v dash >/dev/null 2>&1; then dash -n "$f" || status=1; fi
  if command -v bash >/dev/null 2>&1; then bash -n "$f" || status=1; fi
done
if command -v shellcheck >/dev/null 2>&1; then
  shellcheck -s sh "$root"/devops/*.sh || status=1
else
  echo 'shell-lint: shellcheck not installed; skipped'
fi
if command -v pwsh >/dev/null 2>&1; then
  for f in "$root"/devops/*.ps1; do
    # shellcheck disable=SC2016 # PowerShell code, not shell
    PS1_FILE=$f pwsh -NoProfile -NonInteractive -Command '$e = $null; [void][System.Management.Automation.Language.Parser]::ParseFile($env:PS1_FILE, [ref]$null, [ref]$e); if ($e) { $e | ForEach-Object { $_.ToString() }; exit 1 }' || status=1
  done
else
  echo 'shell-lint: pwsh not installed; PowerShell scripts not parsed'
fi
[ "$status" -eq 0 ] && echo 'shell-lint: ok'
exit "$status"
