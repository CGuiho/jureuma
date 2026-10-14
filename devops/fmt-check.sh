#!/bin/sh
# Fail (exit 1) when a Go file under cmd/ or internal/ is not gofmt-formatted,
# listing the files. Changes nothing.
set -eu
root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"
out=$(gofmt -l cmd internal)
if [ -n "$out" ]; then
  printf 'not gofmt-formatted:\n%s\n' "$out" >&2
  exit 1
fi
echo 'gofmt: ok'
