#### &copy; 2026 [GUIHO](https://guiho.co) as represented by [Cristóvão GUIHO](https://guiho.co/cguiho) All Rights Reserved.

# Handoff — W4: bind the project key pair in `zeze.yaml` (`key:`) — issue #14

Mode: dnd · Role: workhorse (engineer) · Harness: OpenCode native background
subagent (`general`) · Model: `xiaomi/mimo-v2.6-pro` (provider-default
thinking, no variant) · Issue: <https://github.com/CGuiho/zeze/issues/14>

**Dispatch gate:** after the `ZEZE_CONFIG` restore (issue #10) lands; it
owns `internal/config`, `internal/cli` and `test/e2e` before this unit.
Record the base commit at start and confirm `DefaultEnvName` is
`ZEZE_CONFIG`; do not revert that work.

## Mission

CG (2026-10-17): "a project will use a key pair, and the key pair that the
project will use will be specified by the zeze.yaml of the project."
Confirmed file name: `zeze.yaml` — YAML stays, there is no TOML.

Add an optional top-level binding:

```yaml
key: k_5c1e0a9f3b7d2e4c8a6f1b0d9e3c7a25
```

One project, one key, every configured pair. The key ID is public metadata and
safe to commit.

## Contract

1. **Shape.** Top-level `key` is a full key ID (`k_` plus 32 lowercase hex
   characters, `crypt.ValidKeyID`'s shape). Labels are display metadata and
   are not accepted here — a binding must be verifiable on an agent machine
   without the keystore. Empty or malformed values are validation errors
   (exit 4 on load; the JSON Schema carries the same pattern and description).
2. **Agreement.** When `key` is set, every pair's recorded identity must match
   it: the encrypted file's `$zeze.keyId`, or the unencrypted file's
   `$zeze.keyId` when the encrypted file is missing. A disagreement is a
   key error (exit 5) naming the binding and the file, never a silent
   re-encrypt. A pair with no recorded identity (both files absent, or a
   headerless plaintext) is not a disagreement.
3. **One shared resolver.** Enforce agreement in one shared path used by every
   command that resolves pairs (`run`, `set`, `unset`, `encrypt`, `decrypt`,
   `list`, `get`, `status`, `check`, `schema infer`), skipping files that
   cannot be read so `init` can still repair them (their own read reports the
   problem). Do not duplicate the check per command.
4. **`init`.** When `zeze.yaml` names a key:
   - a pair with no recorded identity and no `--key-id` / `--key-label` uses
     the bound key (resolved from the keystore; a missing key is exit 5 with
     the key ID and a pointer to `zeze key list`);
   - `--key-id` / `--key-label` that contradict the binding are a usage error
     (exit 2);
   - a pair whose files record a different key is exit 5;
   - after creating or choosing a key for a pair that had no recorded
     identity, `init` ensures the binding equals that key: absent → add it,
     already equal → no write, different → never overwrite (exit 5).
5. **Byte-preserving write.** Adding or updating `key:` must not reformat
   `zeze.yaml`: splice exactly one top-level line (replace the existing
   `^key:` line, or append one line), keep every comment, blank line,
   ordering and quoting elsewhere byte-identical, write atomically, then
   re-parse and re-validate the result. A file that is missing or does not
   parse is refused with nothing written. Use this same helper from `init`
   and `key rotate`; never call `config.SaveProject` for this change (it
   rewrites the whole document).
6. **`key rotate`.** After a successful rotation, update an existing `key:`
   binding to the new key ID through the same helper (absent stays absent).
   If the update cannot be represented, the command fails after reporting the
   rotation truthfully; it never leaves the binding silently stale.
7. **Docs.** Design document (configuration and key sections), the project
   schema and its description, `README.md` configuration example, the
   `CHANGELOG.md` `## Unreleased` entry, and the relevant xdocs descriptors.
   Historical records under `docs/plans/**`, `docs/questions/**` and
   `docs/reviews/**` stay untouched.

## Do not touch

`internal/pair/**` (the W2 surface is frozen), `internal/jsonc/**`,
`internal/keystore/**` beyond reading existing APIs, `todo.md`,
`docs/plans/**` (except reading this brief), release machinery, version pins,
other projects. Do not change the `ZEZE_CONFIG` default, pair headers,
schema-copy behaviour or single-file flows.

## Validation (capture exact commands and exit codes)

- Full battery: `runx run go-fmt-check`, `runx run go-vet-cross`,
  `runx run go-test`, `runx run test-e2e`, `runx run test-smoke`,
  `runx run docs-check`, `runx run ci-check` — all exit 0.
- `xdocs doctor` 0 errors / 0 warnings.
- Manual proof in a throwaway `ZEZE_USER_HOME` and git project (never the
  real home): (a) `init` with a bound stored key creates a pair using it and
  writes the binding without touching comments; (b) a project whose
  `zeze.yaml` binds key A while `$zeze.keyId` is B fails `run`/`set`
  with exit 5; (c) a headerless/absent pair is not a disagreement; (d)
  `key rotate` updates the binding and the pair still runs; (e) a label or
  malformed value in `key:` is rejected with exit 4.

## Commits

Two or three coherent commits with imperative messages (config + schema;
resolver + init/rotate; tests + docs), through the serialized index:
`flock -w 120 /tmp/opencode/zeze-git.lock -c 'cd /root/swe/zeze && git add <paths> && git commit -m "<message>"'`.
Stage only owned paths; never `git add -A`; never push; leave the worktree
clean.

## Boundaries

- Never leak plaintext, ciphertext values or key material into logs, tests,
  fixtures or error messages; key IDs and labels are public metadata and may
  appear.
- Keep every human-only gate and redaction rule intact.
- DND: never stop to ask a human. Undecided matters: safest reversible
  choice, recorded in `docs/questions/key-identity-headers/2026-10-17-dnd-decisions.md`
  with confidence, action taken and human-review pending.

## Report

Return: base commit verified and the confirmed `ZEZE_CONFIG` default;
commit hashes and subjects; changed files; exact validation commands with
exit codes; new/updated test names; manual-proof transcript summary
(value-free); deviations and why; residual uncertainty. Do not claim a result
you did not capture.
