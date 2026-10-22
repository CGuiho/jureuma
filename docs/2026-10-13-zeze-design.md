---
name: ZeZé Design
purpose: Define the file model, cryptography, command surface, and agent-safety rules of the zeze CLI before implementation.
description: Design of zeze, an agent-safe configuration CLI that keeps a gitignored plaintext JSON file, a committed encrypted JSON file, and a committed JSON Schema per configuration, lets agents set and inject values without reading them, and reserves decryption and reading for humans.
created: "2026-10-13"
owner: zeze-docs
flags: []
tags:
  - docs
keywords:
  - design
  - agent-safety
  - cryptography
  - run
  - lifecycle
---

#### &copy; 2026 [GUIHO](https://guiho.co) as represented by [Cristóvão GUIHO](https://guiho.co/cguiho) All Rights Reserved.

# ZeZé Design

Status: **implemented** — revision 2 of this design (after a four-lens review: cryptography, agent safety, convention compliance, lifecycle) is implemented in full as of 2026-10-16; see `CHANGELOG.md` for the release state.

## Summary

`zeze` is an agent-safe configuration CLI. A human writes configuration and secrets in plain JSON. `zeze` encrypts every value into a sibling file that is safe to commit, and validates the configuration against a JSON Schema (JSON or JSONC) that agents write or that `zeze schema infer` generates. AI agents can **set** values and **inject** them into scripts, but cannot **read** them through `zeze`: `decrypt` and `get` are human-only, interactive commands, unless the caller passes `--dangerously-bypass-protections`.

```text
config.unencrypted.json   plaintext, human-owned, always gitignored
config.encrypted.json     ciphertext + public key, committed
config.schema.json        JSON Schema (written by agents or inferred), committed
```

The goal is that a human sets raw values once and an agent can run the project with them without ever being shown them.

## Goals

1. Every value in the committed file is encrypted; field names and structure stay readable so agents understand the shape of the configuration.
2. No `zeze` command an agent can run without the bypass flag **displays** plaintext or key material to the agent: not on stdout, stderr, in errors, logs, or diffs.
3. An agent can set values and inject values into a process.
4. Human-only operations (`decrypt`, `get`, private-key display and export) require an interactive human confirmation.
5. The state of a configuration is obvious from filenames alone: `.unencrypted.` means plaintext, `.encrypted.` means ciphertext.
6. Configuration is validated against a JSON Schema that nobody has to type by hand: `zeze schema infer` generates it and agents refine it, guided by the `guiho-s-zeze` skill.
7. Full compliance with the [GUIHO CLI Convention](../guiho-convention-0001-cli.md), with the deviations listed under [Convention Notes](#convention-notes).

## Non-Goals and Honest Limits

- **Not a security boundary against a malicious agent running as the same OS user.** Anything `zeze` can read (private keys, the unencrypted file), a same-user process can read. Agent safety means *no accidental exposure* and *no plaintext displayed by the sanctioned interface*, not containment.
- **`run` hands plaintext to the child process by design.** An agent chooses that child, so an agent determined to extract a value can (for example by writing it to a file, or transforming it before printing). Output redaction prevents **accidental display**, not deliberate extraction. The agent instruction forbids extraction; `zeze` does not and cannot enforce it.
- Rotating the key does not revoke values already in git history. After a key leak, the underlying secrets themselves must be rotated.
- No `.json5` support yet. No YAML, TOML, or `.env` formats.
- No remote secret store, sync service, or team key server in v1.

## Supported Formats

| Extension | Read | Written | Notes |
| --- | --- | --- | --- |
| `.json` | yes | yes | RFC 8259. |
| `.jsonc` | yes | yes | jsonc.org draft, lenient: `//` and `/* */` comments; trailing commas accepted. Written in place with comments and formatting preserved. Converted to `.json` on encryption. |
| `.json5` | no | no | Future. |

- The encrypted file is always `.json`. Comments in a `.jsonc` plaintext file are **not** carried into the encrypted file.
- `zeze` writes a `.jsonc` file only through a comment-preserving editor: `set`, `unset`, `decrypt` and `schema infer` splice the minimal byte spans their change needs (a replaced value gets exactly its value span; a removal deletes the key-to-value span plus one separator comma; an insertion lands before the closing brace or bracket, matching the sibling indentation and separator style and reusing an existing trailing comma). Every comment, blank line, formatting detail, number text (`1e3`, `-0`) and string-escape spelling outside the edited spans survives byte for byte, and a new trailing comma is never emitted. A change that cannot be represented this way, and a file that no longer parses, are refused with nothing written — never rewritten. `.json` stays the default (new pairs and the starter schema), and the encrypted file is always `.json`.
- If both `<stem>.unencrypted.json` and `<stem>.unencrypted.jsonc` exist, every command on that pair fails with a usage error.
- Duplicate object keys are rejected. A plaintext document may not use the top-level key `$zeze`.
- Strings containing NUL are rejected on input (they cannot be injected into environment variables).

## File Model

### Pairs and schema

Configuration always lives in a **pair** that shares one stem, plus a schema file:

```text
<stem>.unencrypted.json   (or .unencrypted.jsonc)   plaintext, gitignored
<stem>.encrypted.json                               ciphertext, committed
<stem>.schema.json                                  JSON Schema, committed
```

With the default stem the project root contains `config.unencrypted.json`, `config.encrypted.json`, and `config.schema.json`.

- Default stem: `config` in the project root.
- `--name <stem>` selects another pair, e.g. `--name production` → `production.encrypted.json` / `production.unencrypted.json` / `production.schema.json`. A stem may include a directory: `--name apps/api/config`.
- `--encrypted-file <path>` and `--unencrypted-file <path>` override each side. Explicit paths must still contain `.encrypted.` and `.unencrypted.` respectively; `zeze` refuses any other name so the state stays obvious to agents.
- Pairs can be declared once in `zeze.yaml` so commands need no flags.

### Schema resolution

The schema for a pair is resolved in this order; the first source that is set wins:

1. `--schema-file <path>` (relative to the current directory). On mutating commands, a value that disagrees with sources 2–3 is a usage error (exit 2), so an agent cannot validate a write against a looser schema by accident.
2. `pairs[].schema` in `zeze.yaml` (relative to the project root).
3. `$zeze.schema` in the encrypted file (relative to the encrypted file).
4. `<stem>.schema.json` beside the encrypted file.

Several pairs may share one schema (for example `production` and `staging` both use `config.schema.json`). `init` creates a missing schema only at the resolved path, so a pair configured to share a schema never gets its own copy.

`init`, `encrypt` and `schema infer` record the resolved path in `$zeze.schema`, so a pair stays self-describing and the recorded path follows `zeze.yaml` whenever that file names one. To move the schema to another file (for example from `config.schema.json` to `config.schema.jsonc`), name it in `pairs[].schema` or re-run `zeze schema infer` — deleting the recorded file makes `check` fail until then. The unencrypted document's `$schema` pointer and the copy embedded in the encrypted file are kept in step: `check` and `status` report pointer presence, path match and drift between the embedded copy and the local file (all warnings), `schema infer` refreshes both, and `decrypt` restores a missing local schema from the embedded copy — never clobbering an existing file, and warning when it differs.

`config.schema.json` is committed and contains no values. `zeze check` warns when a schema carries `default`, `examples`, `const`, or `enum` on a path marked `x-zeze-secret`, since those could embed secrets.

### Gitignore invariant

Every `*.unencrypted.json` and `*.unencrypted.jsonc` file must be ignored by git.

- `zeze init` adds a bounded block to the project `.gitignore`:

  ```gitignore
  # >>> zeze managed block >>>
  *.unencrypted.json
  *.unencrypted.jsonc
  # <<< zeze managed block <<<
  ```

- Before writing any unencrypted file, `zeze` runs `git check-ignore` on the target. If it would not be ignored, it adds the block and re-checks; if still not ignored (for example a negation rule), it refuses to write.
- `zeze check` fails when an unencrypted file is tracked by git.
- The block is **persistent project data**: `upgrade` and `uninstall` never remove it, because plaintext files may outlive `zeze`. The uninstall plan lists it under `PRESERVE` with that reason.

### Encrypted file format

The encrypted file mirrors the plaintext structure. Object keys and array positions stay plaintext; every leaf value (string, number, boolean, `null`) is replaced by a ciphertext string. One reserved top-level key carries metadata:

```json
{
  "$zeze": {
    "format": 2,
    "publicKey": "zeze-pk-1:BASE64URL",
    "keyId": "k_5c1e0a9f3b7d2e4c8a6f1b0d9e3c7a25",
    "home": "~/.guiho/zeze/",
    "schema": { "path": "./config.schema.json", "c": "zeze:1:k_5c…:BASE64URL" },
    "sync": {
      "/database/url":  { "c": "9f2c…", "h": "41ab…" },
      "/database/pool": { "c": "07de…", "h": "c3f0…" }
    },
    "pending": ["/features/1"]
  },
  "database": {
    "url":  "zeze:1:k_5c1e0a9f3b7d2e4c8a6f1b0d9e3c7a25:BASE64URL",
    "pool": "zeze:1:k_5c1e0a9f3b7d2e4c8a6f1b0d9e3c7a25:BASE64URL"
  },
  "features": ["zeze:1:k_…:BASE64URL", "zeze:1:k_…:BASE64URL"]
}
```

- `$zeze` is the only plaintext value block. `sync` and `pending` are described under [File Lifecycle](#file-lifecycle).
- Empty objects and arrays stay as `{}` and `[]`.
- All values are encrypted, as required. Plaintext opt-outs (like dotenvx `_PLAIN`) are not supported in v1 (Q3).
- An agent can read this file or run `zeze list` to learn which settings exist, their nesting, and array lengths, without learning values or their types.

### Pair identity and schema metadata

Both files of a pair carry the same `$zeze` identity — `format`, `publicKey`, `keyId` and the global directory reference `home` (`~/.guiho/zeze/`) — so a single file is enough to know which key the pair belongs to. This zeze writes `format: 2`; it reads `1` (legacy) and `2`, and every write renders `2`, so no path re-emits a legacy block. The unencrypted header is exactly those four fields; beside it the document always carries a top-level `$schema` pointer at the local schema (`./config.schema.json` or `.jsonc`), which editors use to validate and autocomplete.

Both are **reserved metadata, never values**: `$zeze` and `$schema` are excluded from the body, so they are never encrypted, listed, injected into `run`, or accepted as `set`/`unset` paths. Every write preserves them exactly as they were read; `init` adds and repairs them; `decrypt` writes them into the plaintext it creates. The encrypted `$zeze` block additionally records the readable schema path and the schema document encrypted to the pair key (`$zeze.schema = { path, c }`, or the plain string path while no copy is embedded), so each file is self-describing and a key holder can restore the schema from the committed file alone — a keyless sibling build embeds the copy too, because encryption needs only the public key.

A pair whose files carry old zeze state **refuses every mutation** with a typed, value-free `run zeze init to repair` error. A `$zeze` block with `format: 1` stays readable, and `init` repairs it in place (keylessly) keeping values, `sync` and `pending`; a present-but-malformed block is refused on read with the same typed error, is reported by `init` (never overwritten) and must be fixed or removed by hand — zeze never guesses at broken metadata. A headerless unencrypted file is not legacy: it stays fully mutable in the both-present and only-encrypted flows, while the single-file flows that must rebuild the sibling from it return the same repair pointer. One pair belongs to exactly one key: a `publicKey`/`keyId` disagreement between the two files is a key error (exit 5), `home` and schema-path differences are warnings only.

## Cryptography

dotenvx uses ECIES over secp256k1. Go's standard library has no secp256k1, and the convention prefers the standard library, so `zeze` uses the same ECIES construction over **X25519**, entirely from Go's standard library (`crypto/ecdh`, `crypto/hkdf`, `crypto/aes`, `crypto/cipher`, `crypto/hmac`, `crypto/sha256`).

### Keys and key IDs

- Key pair: X25519. Encodings: `zeze-pk-1:` + unpadded base64url(32 bytes) and `zeze-sk-1:` + unpadded base64url(32 bytes).
- **Key ID is derived, never chosen:** `keyId = "k_" + lowercase_hex(SHA-256("zeze/1/keyid" || pk))[0:32]` (128 bits). It always matches `^k_[0-9a-f]{32}$`.
- Every load of an encrypted file checks that `keyId` matches `publicKey`; a mismatch is exit 5.
- Every command that holds the private key (`encrypt`, `decrypt`, `get`, `run`, `check`, `status`, `key rotate`) derives the public key **from the private key** and refuses (exit 5) when it differs from `$zeze.publicKey`. `encrypt` and `key rotate` never take the recipient key from the file.
- `set`/`unset` without a private key encrypt to `$zeze.publicKey`, after checking it against the local key file for that `keyId` when one exists. `zeze check` and `zeze status` warn when `$zeze.publicKey` differs from the version at git `HEAD`. This catches a swapped recipient key (from a bad merge, a copied `$zeze` block, or a malicious edit) before new secrets are encrypted to it.
- **The project key binding (optional).** `zeze.yaml` may carry a top-level `key: <keyId>` naming the one key pair every configured pair of the project uses — the full key ID (`k_` plus 32 lowercase hex characters, the shape zeze derives); a label is display metadata and is not accepted, so the binding is verifiable on an agent machine without the keystore. When it is set, every pair's recorded identity must match it — the encrypted file's `$zeze.keyId`, or the unencrypted file's `$zeze.keyId` when the encrypted file is missing — and a disagreement is a key error (exit 5) on every command that resolves pairs, never a silent re-encrypt. A pair with no recorded identity (both files absent, or a headerless plaintext) is not a disagreement. An empty or malformed binding is a validation error (exit 4) when the file loads. The binding is written and updated by splicing exactly one top-level line: every comment, blank line, ordering and quoting outside it stays byte-identical.

### Value encryption

For each leaf at JSON Pointer `p` (RFC 6901, escaped form, raw UTF-8 keys, no Unicode normalisation):

1. `plaintext_value` = the value's JSON text in zeze canonical form (below).
2. `framed = uvarint(len(plaintext_value)) || plaintext_value || zero padding` to a bucket: at least 32 bytes, then the next multiple of 32. This hides the difference between `true`/`false`/`null`, digit counts, and exact lengths within a bucket. Decryption verifies the padding is all zeros.
3. Generate an ephemeral X25519 key pair; `shared = X25519(eph_sk, recipient_pk)`.
4. `key = HKDF-SHA256(ikm = shared, salt = eph_pk || recipient_pk, info = "zeze/1/value")`, 32 bytes.
5. Seal `framed` with AES-256-GCM and a random 12-byte nonce.
6. **AAD** = `"zeze/1/aad" || keyId (34 bytes, fixed) || uint32_be(len(p)) || p`. Binding the pointer stops a ciphertext from being silently relocated to another field. (It does not stop deliberate extraction through `run`; see Non-Goals.)
7. Ciphertext string: `zeze:1:<keyId>:` + unpadded base64url(`eph_pk (32) || nonce (12) || sealed`).

Decoding is strict: padded or non-canonical base64url, or decoded input shorter than 32 + 12 + 16 + 32 bytes, is rejected. Every parse or authentication failure maps to exit 5 with no detail beyond the path.

Encryption only needs the public key, so `set` works for agents and CI without the private key. Re-encrypting an unchanged value yields a different ciphertext; to keep diffs small, `encrypt` reuses the existing ciphertext for a path whose plaintext is unchanged.

### Canonical form

- Values are encrypted individually, so canonical form applies to scalars only.
- Numbers: the exact validated source text (`json.Number`), never re-formatted. `1.0`, `1e2` and `100` stay distinct.
- Strings: RFC 8785 (JCS) string escaping, so `"é"` and `"é"` produce the same bytes.
- `true`, `false`, `null` as literals.

### Private key storage

Private keys never live in the project directory.

| Source | Priority | Use |
| --- | --- | --- |
| `ZEZE_PRIVATE_KEY_<KEYID>` (e.g. `ZEZE_PRIVATE_KEY_K_5C1E0A9F3B7D2E4C8A6F1B0D9E3C7A25`) | 1 | CI/production, one variable per key. |
| `ZEZE_PRIVATE_KEY` | 2 | CI/production; used only if its derived `keyId` matches the file. |
| `$HOME/.guiho/zeze/keys/<keyId>.json` | 3 | Developer machines. |
| `$HOME/.guiho/zeze/keys/retired/<keyId>.json` | 4 | Old keys kept after rotation, for migrating stale branches. |

- Naming variables by `keyId` (not by stem) removes stem-normalisation collisions in monorepos.
- After loading a key from any source, `zeze` derives its `keyId` and fails with `key mismatch (source: <NAME>)` when it does not match.
- `keyId` is validated against its grammar **before** it is used in a filename. Key files are opened without following symlinks, must be owned by the current user, and must have mode `0600` on Unix (on Windows, an ACL restricted to the current user). Parse errors on key files never echo their contents.
- OS keychain storage is a later option (Q4).

### Key rotation

`zeze key rotate` (requires the old private key):

1. Without a key binding (and for a bound project that declares no pairs), rotate the selected pair exactly as before. With `key:` set, rotation is **project-wide**: every configured pair that records the bound key rotates to ONE new key, and a pair recording another key refuses with exit 5 — naming the pair and both key IDs — before anything is written (never a silent re-encrypt). Pairs with no recorded identity are skipped with a note; an unreadable file, old zeze state or a missing key fails before any write, so a rotation is all-or-nothing for everything a pair can refuse.
2. Generate one new key pair and write it to `keys/<newKeyId>.json`.
3. Decrypt every pair's values (retired keys still open older ciphertexts), re-encrypt them to the new key, write each encrypted file atomically, rebuild each pair's `sync`/`pending`, and migrate each embedded schema copy. Every recorded identity follows the new key — in project-wide and single-pair rotation alike: a headered unencrypted file (the sibling of an encrypted file, or a pair whose plaintext is its only file) is re-keyed with its body, `$schema` pointer, comments and formatting kept, while a headerless plaintext stays headerless. The two files of a pair never end a rotation disagreeing on the pair key.
4. When `zeze.yaml` binds a key, update the binding to the new key ID **once**, after the pair writes succeed, through the same one-line splice (an absent binding stays absent). When the update cannot be represented, the command fails after reporting the rotation truthfully; the binding is never left silently stale.
5. Move the old key to `keys/retired/` when no pair still uses it. It is deleted only when the human runs `zeze key retire <keyId>` and `zeze check` reports no ciphertexts under it in the current project.
6. Print one line per rotated pair plus a summary (single-pair rotation keeps its original report, adding one line when the plaintext identity was re-keyed), the CI variable names to update (`ZEZE_PRIVATE_KEY_<NEWKEYID>`) and a reminder that rotation does not revoke git history. A sibling identity that cannot be updated fails after reporting that the encrypted side was rotated and the plaintext identity was not — never a silent success.

Because every ciphertext carries its `keyId`, a file merged from an older branch can hold ciphertexts under a retired key; `check` reports them and `encrypt` migrates them using the retired keyring.

## Agent Safety Model

### Who may do what

| Operation | Human | Agent | Needs private key |
| --- | --- | --- | --- |
| `encrypt` | yes | yes | yes |
| `set` | yes | yes | no |
| `unset` (object member, last array element) | yes | yes | no |
| `unset` (other array element) | yes | yes | yes (re-indexes the tail) |
| `run` | yes | yes | yes, in memory only |
| `list`, `status`, `check` | yes | yes | optional; more detail with the key, values never printed |
| `decrypt` | yes | **human-only** | yes |
| `get` | yes | **human-only** | yes |
| `key show --private`, `key export` | yes | **human-only** | yes |
| `uninstall` removing `keys/` | yes | **human-only** | — |

An agent may run `encrypt`: it reads the plaintext file inside `zeze`, not in the agent's context, and the recipient key is derived from the private key, never from the file.

### Human-only gate

A protected operation proceeds only when all of these hold:

1. Standard input **and** standard error are terminals.
2. No known agent marker is present in the environment (for example `CLAUDECODE`, `CODEX_SANDBOX`, `CURSOR_AGENT`, `GEMINI_CLI`; the list is maintained in code, documented, and extended by `protections.agentEnvMarkers`).
3. The user types a randomly generated confirmation word shown on the terminal (for example `type "maple-17" to reveal 1 value`). A plain `y` is not enough, because agents that drive a pseudo-terminal can answer yes/no prompts.

If a check fails, the command exits `3` with `[PROTECTED] <command> is human-only. A human must run it in an interactive terminal.` It does not hint at the bypass flag. An agent that drives a PTY with markers unset and reads the prompt can still pass; that falls under the same-user non-goal.

### `--dangerously-bypass-protections`

- Accepted only by protected operations, and by `run` to disable output redaction of config values. It never disables stripping and redaction of private keys in `run`.
- Prints a warning to stderr and appends an audit line (timestamp, command, paths, caller agent markers, **no values**) to `$HOME/.guiho/zeze/audit.log`. Gate refusals are audited too.
- Can be disabled by `protections.allowBypass: false` in `zeze.global.yaml`.
- `protections.*` exists **only** in the global configuration: the project schema rejects it and the configuration merge never reads it from `zeze.yaml`, so a committed project file cannot loosen protections.
- When `zeze init` (a protected step for this setting) writes the `protections` block, it stores an HMAC of the block in `$HOME/.guiho/zeze/state/protections.json`, keyed by a random local secret. If the block is edited by hand so the HMAC no longer matches, the bypass flag is refused with exit 3. This detects a casual edit; a determined same-user agent can redo the HMAC (non-goal).
- The agent instruction says agents must never pass this flag unless the human explicitly asked for it in the current conversation.

### Injection (`run`)

`zeze run [flags] -- <command> [args...]` (`inject` is an accepted alias of `run`: the same command, flags, injection, redaction and exit codes):

1. Load the encrypted file, load and verify the private key, decrypt in memory, validate against the schema.
2. Build the child environment: start from the inherited environment, **remove `ZEZE_PRIVATE_KEY` and every `ZEZE_PRIVATE_KEY_*` unconditionally**, then add injected values (mapping below).
3. Start the child with inherited stdin; register the `run` process in the CLI instance registry.
4. Redact the child's stdout and stderr (below), streaming and boundary-safe (a value split across two writes is still caught).
5. Return the child's exit code. Failures on zeze's side, before or around the child, exit `125` with a fixed stderr prefix such as `[ZEZE:VALIDATION]`, `[ZEZE:KEY]`, `[ZEZE:PROTECTED]`. Any other exit code from `run` belongs to the child.

**Redaction set** (replaced with `[REDACTED]`; the path is not included, so redaction does not confirm which value matched):

- Every decrypted string value of length ≥ 4, and its base64, base64url, URL-encoded and JSON-escaped forms.
- Every value, of any type and length, at a path the schema marks `"x-zeze-secret": true`.
- Every private key in use, in all its encodings. Always on, even with `--dangerously-bypass-protections`.

Redaction applies whether stdout is a terminal or redirected. It covers only the child's stdout and stderr, not files or sockets the child writes; see Non-Goals.

**Environment mapping:**

By default the application receives the **whole decrypted document as one JSON-stringified environment variable**, `ZEZE_CONFIG` (name configurable with `--env-name` or `run.envName`). `--format` selects another mapping:

| `--format` | Result |
| --- | --- |
| `json` (default) | The full document, JSON-stringified, in `--env-name` (default `ZEZE_CONFIG`). |
| `flatten` | Every leaf becomes one variable (rules below); `--prefix <P>` prepends a prefix. |
| `file` | `--env-name` holds a path to the full document (below). |

`--env NAME=/json/pointer` (repeatable, Cobra `StringArray`, never comma-split) injects selected values in addition to the chosen format.

Flatten rules:

- Name: decode pointer escapes (`~1` → `/`, `~0` → `~`), join segments with `_`, uppercase, map every character outside `[A-Z0-9_]` to `_`, prefix `_` if the result starts with a digit, then prepend `--prefix`. `/database/url` → `DATABASE_URL`; `/features/0` → `FEATURES_0`; `/apiKey` → `APIKEY`.
- Collisions are checked **after** normalisation; any collision is an error (exit 125), reported in advance by `check` and `list`.
- **Denylist:** flatten mode refuses to produce names that load or run code, change how a shell or tool runs commands, or belong to zeze, unless they are mapped explicitly with `--env`: `LD_*`, `DYLD_*`, `GIT_*` (including `GIT_CONFIG_*`), `JAVA_*`, `NPM_CONFIG_*`, `BASH_FUNC_*`, `ZEZE_*`, `NODE_OPTIONS`, `NODE_PATH`, `BASH_ENV`, `ENV`, `PROMPT_COMMAND`, `PS1`, `PS2`, `PS4`, `SHELLOPTS`, `BASHOPTS`, `IFS`, `CDPATH`, `PATH`, `SHELL`, `ZDOTDIR`, `PAGER`, `MANPAGER`, `EDITOR`, `VISUAL`, `BROWSER`, `LESSOPEN`, `LESSCLOSE`, `SSH_ASKPASS`, `SUDO_ASKPASS`, `PYTHONSTARTUP`, `PYTHONPATH`, `PYTHONHOME`, `PERL5OPT`, `PERL5LIB`, `PERLLIB`, `RUBYOPT`, `RUBYLIB`, `_JAVA_OPTIONS`, `JDK_JAVA_OPTIONS`, `RUSTC_WRAPPER` (compared case-insensitively). This stops a key added by `set` (anyone can, with the committed public key) from turning into code execution inside a process that holds every secret. The same denylist applies to `--env-name` and `run.envName` (except the default `ZEZE_CONFIG`) for every format, and `run.envName` is validated against it when `zeze.yaml` is loaded, because a committed `run: {format: file, envName: BASH_ENV}` would make every bash child source the document.
- Values: strings raw; numbers as their source text; `true`/`false`; `null` → variable not set; empty `{}` and `[]` → not set.
- Existing environment variables win over flattened and `--env` values unless `--override` is given (same default as dotenvx). The `json` and `file` formats always set `--env-name`: an inherited `ZEZE_CONFIG` (from the shell or an outer `run`) never silently replaces this run's document.

`--format file`: on Linux the document is passed through `memfd_create` as `/dev/fd/N`, so nothing touches disk. Elsewhere it is written to `$HOME/.guiho/zeze/run/<id>/document.json` (mode `0600`, with a PID owner file) and deleted when the child exits. `run/` is classified as disposable; `zeze` removes stale entries whose owner PID is gone on startup and on uninstall. On non-Linux systems a `SIGKILL` of `zeze` can leave the file until the next sweep; this is documented.

### Error and log hygiene

- Every user-visible message is built by `zeze` from the JSON Pointer, the schema keyword, and schema-side parameters only (`/database/pool: maximum 100`). Third-party validator and parser messages are never passed through, because they commonly embed the instance value.
- JSON/JSONC parse errors report file, line, column and error class only, never token text or a source snippet.
- A test suite plants a sentinel value and asserts it never appears in any output, error path, or the audit log.

### Agent-facing guarantees

- Without the bypass flag, no agent-reachable command displays plaintext or key material (Goal 2).
- `set` never echoes the value. It reads the value from an argument, `--value-stdin`, or `--value-file`; the docs recommend stdin so the value stays out of shell history.
- The agent instruction forbids reading, opening, printing, grepping, or copying any `*.unencrypted.*` file and any file under `$HOME/.guiho/zeze/`.
- `zeze init` offers to add deny rules for `*.unencrypted.*` to supported agent tools' settings (Claude Code `permissions.deny` in `.claude/settings.json`, and OpenCode's ordered `permissions` rules in the project's `opencode.json`, created when absent). Accepting is recommended. Rules are written as a marker-tagged entry and recorded in the project state, so `zeze uninstall` and `zeze agent instruction remove` delete exactly what `zeze` added. `zeze check` warns when they are absent. These rules usually cover file-read tools, not shell commands such as `cat`, which is why the instruction also forbids them.

## File Lifecycle

By default only **one side of a pair exists at a time**, so the state is obvious from the filesystem:

- `zeze encrypt` writes the encrypted file and then **deletes the unencrypted file**.
- `zeze decrypt` writes the unencrypted file and then **deletes the encrypted file**. Before deleting, it saves a copy (ciphertext and `$zeze` metadata only) to `~/.guiho/zeze/state/stash/<hash>.encrypted.json`.
- `--keep-both` on either command keeps both files.
- When the encrypted file is missing, `encrypt` restores the key, sync baseline and unchanged ciphertexts from that saved copy, falling back to the version at git `HEAD`. The saved copy is removed once the encrypted file exists again.
- While a pair is decrypted, `set` and `unset` keep working from the unencrypted file alone: the `publicKey` in its `$zeze` header rebuilds (or refreshes) the encrypted sibling from the document, with no private key required, and the usual mirror writes the value into the plaintext. `run`, `list` and `get` keep needing the encrypted file (`not found (run zeze encrypt …)`); a human finishes editing and runs `zeze encrypt`.
- The two single-file states are the designed ones. When both files exist (for example after such a rebuild), `status` and `check` warn that this is the riskiest state; `encrypt` or `decrypt` returns the pair to a single file. `status` and `check` report all four states (only the unencrypted file, only the encrypted file, both, legacy zeze state) with value-free output.

```text
          human edits                       zeze encrypt
config.unencrypted.json  ───────────────────────────────►  config.encrypted.json
          ▲                                                       │
          │  zeze decrypt (human-only)                            │ zeze set / unset (agent or human)
          └───────────────────────────────────────────────────────┘
```

### Sync state

`$zeze.sync` records, for every leaf at the last sync (`encrypt` or `decrypt`):

- `c`: the first 16 bytes of SHA-256 of the ciphertext string, hex. No key needed, so `set` can keep it meaningful: a ciphertext that no longer matches `c` was changed on the encrypted side.
- `h`: the first 16 bytes of HMAC-SHA256 of the canonical plaintext, keyed by `HKDF-SHA256(sk, info = "zeze/1/sync")`, hex. Only a key holder can compute or check it; it reveals nothing without the key.

`$zeze.pending` is a sorted list of pointers written by keyless `set`/`unset` since the last sync. Both are sorted maps and lists, so concurrent branches merge in git without a single-line conflict.

A keyless sibling rebuild (`set`/`unset` with only the unencrypted file, or `init` building the sibling) writes **no baseline**: `h` needs the private key, and zeze never claims `in-sync` for a path without one. A key holder refreshing the sibling writes full `c`/`h` pairs; `encrypt` and `decrypt` rebuild the whole baseline as before.

### States

`zeze status` is **read-only**. Per path and overall it reports:

| State | Meaning | Action |
| --- | --- | --- |
| `in-sync` | Both sides equal the baseline. | — |
| `unencrypted-newer` | Plaintext differs from `h`; ciphertext matches `c`. | `zeze encrypt` |
| `encrypted-newer` | Ciphertext differs from `c` (or path is pending); plaintext matches `h`. | `zeze decrypt` |
| `conflict` | Both sides changed the same path. | Human resolves; `encrypt --force` or `decrypt --force` |
| `unencrypted-missing` | No plaintext file. | Normal on agent and CI machines |
| `encrypted-missing` | No encrypted file: the unencrypted file is the only side. | `zeze set` rebuilds the sibling; `zeze encrypt` finishes the human flow |
| `unencrypted-only` | Per-path state while the encrypted file is missing. | — |
| `key-unavailable` | No matching private key; only `c`/`pending` checks run. | Normal on agent machines |

### Rules

- **Human workflow:** `zeze decrypt` (removes the encrypted file) → edit `config.unencrypted.json` → `zeze encrypt` (removes the unencrypted file).
- **Agent workflow:** `zeze set /path` writes the encrypted file using the pair's public key (from the encrypted file, or from the unencrypted file's `$zeze` header when it is the only side), records the touched paths, and, if an unencrypted file exists, writes the same value into it (the agent already knows the value it set). The encrypted file is written first, then the unencrypted file. With only the unencrypted file present, the whole encrypted sibling is built or refreshed from that document first.
- `encrypt` refuses to overwrite any path in `conflict` or `encrypted-newer` unless `--force` is given, so an agent's `set` is never silently lost. `--force` resolves conflicts in favour of the plaintext only: a path changed on the encrypted side alone keeps its ciphertext (or stays removed), stays in `pending` and is listed on stderr; one that no longer fits the plaintext structure is listed as discarded. On success `encrypt` refreshes `sync` and clears every other `pending` entry.
- `decrypt` refuses when the plaintext file has paths in `unencrypted-newer` or `conflict` (unsaved human edits), or new empty objects or arrays that the encrypted side lacks, unless `--force` is given.

### Concurrency and atomicity

- Every mutating command takes an exclusive lock for the pair: `$HOME/.guiho/zeze/locks/<sha256(abs encrypted path)[0:16]>.lock`, created atomically with an ownership token and PID; a lock whose PID is gone is recovered.
- Every file write goes to a temporary file in the same directory and is renamed into place. Two-file writes go encrypted first, then unencrypted; a crash between them leaves a state `status` reports as `encrypted-newer`, never a lost value.
- `key rotate` writes the new encrypted file atomically before it moves the old key to `retired/`, so a crash never leaves ciphertexts that no available key can decrypt.

## `set` and `unset`

### Value typing

- By default the value is a **string**: `zeze set /database/url --value-stdin` stores `"…"`.
- `--json` parses the input as a JSON value: `zeze set /database/pool --json 20` stores the number `20`; `--json '{"host":"h","port":5432}'` stores an object.
- If the schema says the path is not a string and `--json` was not given, `set` fails with exit 4 and suggests `--json`. Type is never guessed from the schema.

### Paths

- Object or array values given with `--json` are expanded into per-leaf ciphertexts, as the format requires.
- `$zeze` and `$schema` are reserved metadata: `set` and `unset` refuse them as value paths (exit 2), and they never appear in a body, a listing or an injection.
- Missing intermediate objects are created. A pointer that passes through an existing leaf (setting `/database/url` when `/database` is a leaf) is an error.
- Array indices may address an existing element or `len` (append); `/-` also appends. Indices beyond `len` are an error.

### `unset`

- Removes the value at the pointer; on a container, removes the whole subtree. Parents are not pruned (`{}` remains).
- Without the key, only object members and the **last** array element can be removed, because removing another element shifts later indices and their path-bound ciphertexts must be re-encrypted. Otherwise `unset` asks for the key (exit 5 without it) and suggests editing the plaintext file and running `encrypt`.
- `unset` is mirrored into an existing unencrypted file (`.json` or `.jsonc`), like `set`.
- The `set` mirror never replaces an unsaved human edit. With the private key, it is skipped when a touched path is `unencrypted-newer` or `conflict`; without the key, when the plaintext already holds a value at or under the target (it cannot be compared). An append (`/-`) is also skipped when the two arrays differ in length. A skipped mirror prints a warning and leaves the path pending. The rule is unchanged when only the unencrypted file exists: the edit is compared against the last saved state (the saved copy, then git `HEAD`), and a value the plaintext holds that cannot be proven unchanged is never replaced.
- With only the unencrypted file present, `set` and `unset` rebuild the whole encrypted sibling from the document (values included), so there `unset` may remove any array element — nothing has to be re-indexed ciphertext by ciphertext.
- `set` validates against the readable schema when one is present, and says so explicitly when validation is skipped (no schema file). `/-` appends only to an existing array; it is a usage error under a missing parent.
- `set` refuses pointers deeper than 64 levels, and every write refuses (exit 4) a file larger than the 32 MiB that zeze reads.
- Paths that come from committed files (`pairs[].encrypted`, `pairs[].unencrypted`, `pairs[].schema` in `zeze.yaml`, and `$zeze.schema`) must stay inside the project root, also after resolving symbolic links, and a schema path must end with `.schema.json` or `.schema.jsonc`. Only command-line flags may name files outside the project. A `--schema-file` on a mutating command must agree with the configured schema, or with the default `<stem>.schema.json(c)` when that file exists.
- `set /x --json null` stores `null`; it is not the same as `unset`.

## Schema Validation

`zeze` is pure Go; there is no TypeScript or TypeBox toolchain. The schema is a standard **JSON Schema draft 2020-12** document, stored as `<stem>.schema.json` (or `<stem>.schema.jsonc`, so agents can leave comments in it). Nobody types it by hand:

- **Generate:** `zeze schema infer` derives a schema from the plaintext structure and JSON types (never values). Without a plaintext file it infers structure from the encrypted file (types become unconstrained).
- **Refine:** agents edit the schema to add formats, ranges, enums, descriptions, and `x-zeze-secret`, following the `guiho-s-zeze` skill, which contains a schema authoring guide.
- **Validate:** `zeze` validates in Go with no network access. If the file lacks `$schema`, draft 2020-12 is assumed.

### Supported schema subset

- Supported: the JSON-data vocabulary of draft 2020-12 (`type`, `properties`, `required`, `additionalProperties`, `items`, `prefixItems`, `enum`, `const`, `anyOf`/`oneOf`/`allOf`, `if`/`then`/`else`, `$ref`/`$defs`, numeric and string constraints).
- Unknown keywords: an error, except `x-*` annotations.
- `format`: asserted for `date-time`, `date`, `time`, `email`, `hostname`, `ipv4`, `ipv6`, `uri`, `uuid`; any other format is an error, so a typo is never silently ignored.
- Annotations used by `zeze`: `x-zeze-secret` (always redact in `run`; warn on `default`/`examples`/`const`/`enum`).

### Validation points

| Command | Validation | Needs key |
| --- | --- | --- |
| `encrypt` | Whole plaintext document. Fails before writing. | yes |
| `set`, `unset` | **Structural pass** over the encrypted file (`required`, `additionalProperties`, `unevaluatedProperties`, `propertyNames`, `patternProperties` names, `minItems`/`maxItems`, `minProperties`/`maxProperties`), which needs no values, plus a **provisional value check**: accept if any schema branch that could apply to the path, with sibling values unknown, accepts the value. Without a readable schema nothing is validated and `set`/`unset` warn that validation was skipped. | no |
| `run`, `check` | Whole decrypted document. | yes |

A failed `set`/`unset` validation exits 4 and writes nothing unless `--allow-invalid` is given. Removing a `required` property is allowed only with `--allow-invalid`.

### `schema infer`

Generates a starter schema from the **plaintext file's** structure and JSON types (never values): objects get `required` for every present key and `additionalProperties: false`; homogeneous arrays become `items`; mixed arrays become `prefixItems`; empty arrays become `items: {}`. The generated schema declares the optional top-level `$schema` string property (the pointer the unencrypted document always carries) without requiring it, so editors can validate the whole document. It needs the unencrypted file and refuses to overwrite an existing schema without `--force`. `init` creates a permissive starter schema (`{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object"}`) when none exists, and `check` warns while that starter is still in use. After writing, `schema infer` refreshes the unencrypted file's `$schema` pointer and the copy embedded in the encrypted file.

## Command Tree

Domain commands sit beside the convention's mandatory commands. All flags are long-form only; repeatable flags are Cobra `StringArray`.

```text
zeze
├── init                    convention init + domain steps below (--key-id, --key-label)
├── encrypt                 unencrypted file → encrypted file
├── decrypt          [P]    encrypted file → unencrypted file
├── set <pointer> [value]   encrypt one value (--json, --value-stdin, --value-file, --allow-invalid)
├── unset <pointer>         remove one value (--allow-invalid)
├── get <pointer>    [P]    print one plaintext value
├── run -- <cmd>            inject values into a child process, redacted output (alias: inject)
├── list                    pointers, flattened env names, types (schema-declared without key; actual with key)
├── status                  read-only pair state across its four file states, per path
├── check                   pair state, identity headers, $schema pointer and embedded schema copy, schema, gitignore, tracked plaintext, key/publicKey consistency, env collisions, deny rules
├── key
│   ├── show                public key and key ID
│   │                       --private is [P]
│   ├── export       [P]    print the private key for backup
│   ├── rotate              new key pair, re-encrypt every value
│   └── retire <keyId>      delete a retired key once no ciphertext uses it
├── schema
│   └── infer               starter schema from plaintext structure and types
├── agent …                 convention: skill, instruction, prompt
├── upgrade …               convention: upgrade, check, list
└── uninstall               convention, with the key protections below
```

`[P]` = protected (human-only gate).

Common flags on pair-aware commands: `--name`, `--encrypted-file`, `--unencrypted-file`, `--schema-file`. JSON Pointer is the only path syntax, because it is unambiguous for keys containing dots.

Exit codes for every command except `run`: `0` success, `1` general failure, `2` usage error, `3` protected operation refused, `4` validation failed, `5` key not found, key mismatch, or decryption failed. `run` returns the child's code, or `125` for a zeze-side failure.

### `init` domain steps

These slot into step 14 of the convention's `init` sequence, for each configured pair:

1. If `<stem>.encrypted.json` exists: never generate keys and never re-key. Report `private key not available (expected on agent and CI machines)` when no matching private key is available; this is informational, not a failure. Replacing a key on an existing file is only possible with `zeze key rotate`. Old zeze state is repaired in place — the `$zeze` block becomes a format-2 identity header (adding `home`, keeping every value, `sync` and `pending`), the schema reference is recorded and the embedded schema copy added when the local schema is present. An already-repaired file is left byte-identical, so `init` is idempotent; a `$zeze` block zeze cannot parse at all is reported and never overwritten.
2. If no encrypted file exists and the unencrypted file records the pair key in its `$zeze` header: build the encrypted sibling from that document under that key (no private key needed).
3. If no encrypted file exists and nothing records the identity: choose the pair key — the key bound in `zeze.yaml` (`key:`) resolved from the keystore (a missing key is exit 5, naming the key ID and pointing at `zeze key list`), else a stored key picked in a terminal (`Which one do you want to use?`, label and short key ID each line, plus a `create a new key` choice) or named with `--key-id` / `--key-label`; a new key otherwise (non-TTY default), saved with the repository-name label — then create an empty encrypted file (`{"$zeze": {…}}`) and, after the gitignore check, an empty `<stem>.unencrypted.json`. After creating or choosing a key for such a pair, `init` ensures the `key:` binding names it: an absent binding is added (one spliced line; comments and formatting elsewhere stay byte-identical), an equal binding is never rewritten, a different one is never overwritten (exit 5). `--key-id` / `--key-label` that contradict the binding are a usage error (exit 2); a pair whose files record another key than the binding is a key error (exit 5).
4. Add or repair the unencrypted file's `$zeze` identity header and `$schema` pointer, keeping its body exactly.
5. Ensure the `.gitignore` block exists.
6. Ensure the resolved schema file exists (starter schema if missing).
7. Offer the agent-tool deny rules.
8. Fatal checks (init fails if any fails): gitignore block present, no unencrypted file tracked by git, schema present and supported, `keyId` matches `publicKey` and the `zeze.yaml` key binding when it is set. Informational: key availability, sync state, starter schema still in use.

## Configuration

Per the convention: project `zeze.yaml` and global `$HOME/.guiho/zeze/zeze.global.yaml`, each with its own schema and version-pinned schema reference.

```yaml
# yaml-language-server: $schema=https://github.com/CGuiho/zeze/releases/download/v0.1.0/zeze.schema.json
key: k_5c1e0a9f3b7d2e4c8a6f1b0d9e3c7a25   # the one key pair every pair of this project uses (optional)
pairs:
  - name: config
    schema: ./config.schema.json
  - name: production
    schema: ./config.schema.json
run:
  prefix: ""
  format: json
  envName: ZEZE_CONFIG
  override: false
agent:
  evolution:
    upgrade: always-ask
    issues: { bugs: always-ask, improvements: always-ask, reviews: always-ask }
```

The top-level `key` is the optional project key binding: a full key ID naming the one key pair every configured pair uses. `init` writes it after creating or choosing a key for a pair with no recorded identity (one spliced line; an existing binding is never overwritten), every pair-resolving command enforces it (a disagreement is exit 5, never a silent re-encrypt), and `key rotate` is project-wide with it — every configured pair that records the bound key rotates to one new key — then updates the binding to the new key ID. Labels are not accepted there, and an empty or malformed value is a validation error (exit 4).

```yaml
# yaml-language-server: $schema=https://github.com/CGuiho/zeze/releases/download/v0.1.0/zeze.global.schema.json
protections:                     # global only; rejected in zeze.yaml
  allowBypass: true              # false disables --dangerously-bypass-protections
  agentEnvMarkers: []            # extra environment variable names that identify an agent
agent:
  evolution: { … }
```

## CLI-Owned Paths

| Path | Class | Upgrade / reinstall | Default uninstall |
| --- | --- | --- | --- |
| `$HOME/.guiho/zeze/keys/` (incl. `retired/`) | **persistent data** | preserved | removed only after the human-only gate; preserved automatically when non-interactive |
| `$HOME/.guiho/zeze/audit.log` | persistent data | preserved | removed (kept with `--preserve-data`) |
| `$HOME/.guiho/zeze/state/` (incl. `stash/`, saved encrypted copies) | persistent data | preserved | removed (kept with `--preserve-data`) |
| `$HOME/.guiho/zeze/locks/` | disposable | cleared when unowned | removed |
| `$HOME/.guiho/zeze/run/` | disposable | stale entries removed | removed |
| `$HOME/.guiho/zeze/cache.json` | disposable | reset | removed |
| Project `.gitignore` zeze block | persistent project data | kept | **kept** |
| Project agent-tool deny rules | managed projection | reapplied | removed |

The uninstall plan lists every `keyId` it would remove and how many `*.encrypted.json` files in the current project reference it, and recommends `zeze key export` first.

## Convention Notes

- **Uninstall and keys:** the convention's default uninstall removes all CLI data, and `--yes` skips the prompt. For `keys/` only, `zeze` requires the human-only gate even with `--yes`; a non-interactive uninstall preserves `keys/` and says so. Deleting the only copy of a private key makes every committed secret permanently unreadable, so this deliberately narrows the convention's `--yes` contract (confirmed by the user, Q8).
- **Upgrade and running `run` sessions:** the convention's upgrade kills old payload processes before activation. Killing a `run` parent would stop redaction of the child's remaining output and skip temp-file cleanup. `zeze upgrade` therefore refuses while `run` instances are registered and alive, and lists them; `--force` sends them a graceful termination signal so they can clean up, only right before activation (after the release is downloaded, verified and installed), never for an up-to-date or failed upgrade.
- **Upgrade transaction scope:** `zeze upgrade` performs the synchronous transaction in the payload (lock, staging under `~/.guiho/.temp/zeze-upgrade-*`, SHA-256 verification against `checksums.txt` and `artifacts.json`, `--version` and `__self-test` of the candidate, immutable `versions/<v>/`, skill projections, `installed-artifacts.json`, atomic `current.json` with `previous`, verification through the launcher, `AGENTS.md` block refresh from a project root, rollback of every step). Deviations: beyond the registered `run` instances it also terminates every other current-user process whose symlink-resolved executable path is the superseded payload (see "Process termination on upgrade"); it keeps a transaction journal (`~/.guiho/zeze/state/upgrade-transaction.json`) that distinguishes staging, artifact replacement, activation, verification, rollback and completion and records the write-ahead rollback plan (staged backups under the transaction's staging directory), so the next invocation repairs an interrupted upgrade automatically before dispatching an ordinary command — by completing its rollback to a consistent state, or by keeping the installed state when the staged backups are gone; and it requires an installer-managed installation (`current.json` and the stable launcher).
- **Stable launcher:** `~/.guiho/bin/zeze` (`zeze.exe` on Windows) is the released launcher binary (`zeze-launcher-<os>-<arch>[.exe]` release assets, installed and SHA-256 verified by the installers), not a generated script or a payload copy. It resolves the active payload through `current.json`, starts it with the original arguments and exact exit code, and falls back to the retained previous payload when the active one is missing or cannot start. Ordinary `zeze upgrade` never replaces the running launcher (installer-driven repair may); payloads install only into `versions/<v>/` behind the atomic `current.json` swap. `zeze uninstall` on Windows moves its own running image into `~/.guiho/.temp/zeze-uninstall-*` and schedules its deletion after exit.
- **Process termination on upgrade:** registered `run` instances (see "Upgrade and running `run` sessions"), plus every other process owned by the current user whose symlink-resolved executable path is the superseded `versions/<previous>/zeze[.exe]` payload. Selection is never by filename; the calling process and descendants of it or of registered `run` instances are excluded; termination is graceful first with escalation to a forceful kill and verified death; a verified old-payload process that cannot be terminated fails the upgrade before activation; a process whose path or owner cannot be verified is never terminated. Nothing else is inspected or terminated.
- **Uninstall fallback and deny rules:** when the payload cannot run, `uninstall.sh` and `uninstall.ps1` use a native plan that lists the project's agent-tool deny rules under PRESERVE (they need the payload to be removed exactly); `zeze uninstall` removes them.
- **Prompt IDs:** `guiho-p-zeze` is the default prompt (project setup). Extra prompts use the `guiho-p-zeze-` prefix: `guiho-p-zeze-install` (referenced by the README) and `guiho-p-zeze-uninstall`. Extra skills use the `guiho-s-zeze-` prefix. Confirmed by the user.
- **Bypass and relocated homes:** `--dangerously-bypass-protections` is refused while `ZEZE_USER_HOME` points somewhere other than the user's home (`HOME`/`USERPROFILE`), so one variable cannot swap in an empty global configuration and a different audit log. Tests set both.
- **Main skill, `## CLI Evolution and Feedback`:** besides the convention's mandatory content, it restricts what an issue may contain: JSON Pointer paths, JSON types, exit codes, zeze versions, and command lines with values removed. Never `run` output, contents of `*.unencrypted.*` or temp files, ciphertexts, or key material. A suspected redaction failure is reported with the leaked text replaced.

## Agent Instruction (managed `AGENTS.md` block, outline)

- zeze (https://github.com/CGuiho/zeze) owns project configuration and secrets in `*.encrypted.json` / `*.unencrypted.json` pairs with `*.schema.json` schemas.
- Never read, open, print, grep, or copy any `*.unencrypted.*` file or anything under `~/.guiho/zeze/`.
- Use `zeze list` and `zeze status` to understand configuration; `zeze set --value-stdin` (with `--json` for non-strings) to change it; `zeze run -- <cmd>` to run anything that needs it. Never try to extract values from a `run` child.
- `decrypt`, `get`, `key show --private`, and `key export` are for humans. Never pass `--dangerously-bypass-protections` unless the human asked in this conversation.
- If a value is missing or invalid, ask the human to set it; never guess secrets.
- Exit `3` means human-only; exit `125` from `run` means zeze failed, not the child.
- Deeper guidance: the `guiho-s-zeze` skill, the `guiho-p-zeze` prompt, and `zeze <command> --help-docs`.

## Open Questions

| ID | Question | Recommendation |
| --- | --- | --- |
| Q1 | ~~TypeBox vs Go~~ | Resolved: Go only; schema is JSON Schema generated by `schema infer` and refined by agents. |
| Q2 | X25519 (stdlib) instead of dotenvx's secp256k1? | X25519. No dotenvx file compatibility. |
| Q3 | Allow selected values to be stored as plaintext (like dotenvx `_PLAIN`)? | Not in v1; later via `x-zeze-plain` if needed. |
| Q4 | Store developer private keys in the OS keychain? | v1 files; keychain later. |
| Q5 | Should `set` also update an existing unencrypted file? | Yes (described above). |
| Q7 | Convention names. | Resolved: home `~/.guiho/zeze/`, skill `guiho-s-zeze`, prompt `guiho-p-zeze`, repo `github.com/CGuiho/zeze` (live). |
| Q8 | Narrowed `--yes` contract for uninstalling `keys/`. | Resolved: accepted. |

## References

- [GUIHO CLI Convention](../guiho-convention-0001-cli.md)
- [dotenvx](https://github.com/dotenvx/dotenvx), the model for the encrypt-commit-inject workflow
- [JSONC specification draft](https://jsonc.org/)
- [RFC 6901 — JSON Pointer](https://www.rfc-editor.org/rfc/rfc6901)
- [RFC 8785 — JSON Canonicalization Scheme](https://www.rfc-editor.org/rfc/rfc8785)
- [JSON Schema draft 2020-12](https://json-schema.org/draft/2020-12)
