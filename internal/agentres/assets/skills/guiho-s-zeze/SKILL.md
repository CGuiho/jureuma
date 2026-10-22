---
name: guiho-s-zeze
description: Use whenever a project uses zeze or needs configuration or secrets handled safely by an agent - installing, initializing, upgrading, or uninstalling zeze; reading the shape of configuration with list/status/check; setting or unsetting values without seeing them; running commands with injected configuration (ZEZE_CONFIG); writing or refining config.schema.json(c) JSON Schemas; rotating keys; or reporting zeze bugs and improvements.
metadata:
  version: "0.4.0"
---

#### &copy; 2026 [GUIHO](https://guiho.co) as represented by [Cristóvão GUIHO](https://guiho.co/cguiho) All Rights Reserved.

# guiho-s-zeze

`zeze` is an **agent-safe configuration CLI** (repository: https://github.com/CGuiho/zeze). A human writes configuration and secrets in plain JSON; zeze encrypts every value into a sibling file that is safe to commit and validates the configuration against a JSON Schema. Agents can **set** values and **inject** them into processes, but cannot **read** them through zeze: decrypting and reading are human-only.

## The file model

Every configuration is a **pair** sharing one stem, plus a schema:

```text
config.unencrypted.json   plaintext, human-owned, always gitignored   (or .unencrypted.jsonc)
config.encrypted.json     every value encrypted, keys readable, committed
config.schema.json        JSON Schema draft 2020-12, committed           (or .schema.jsonc)
```

- Default stem: `config` in the project root. `--name <stem>` selects another pair (`--name production`, `--name apps/api/config`). Pairs may be declared in `zeze.yaml` under `pairs:`.
- `--encrypted-file`, `--unencrypted-file`, `--schema-file` override individual paths. Explicit names must still contain `.encrypted.` / `.unencrypted.`.
- The encrypted file mirrors the structure of the plaintext: object keys and array positions stay readable; each leaf is a `zeze:1:<keyId>:...` ciphertext. The `$zeze` key holds metadata (public key, key ID, schema path, sync state). Never edit `$zeze` by hand.
- The filename tells you the state: `.unencrypted.` is plaintext, `.encrypted.` is ciphertext.
- Paths are always **JSON Pointers** (RFC 6901): `/database/url`, `/features/0`, `/a~1b` for key `a/b`, `/a~0b` for key `a~b`.
- Private keys live outside the project: `~/.guiho/zeze/keys/<keyId>.json`, or the environment variables `ZEZE_PRIVATE_KEY_<KEYID>` / `ZEZE_PRIVATE_KEY` in CI. Agents usually do not have them and do not need them for `set`.

## Agent-safe rules (always apply)

1. **Never read plaintext.** Never read, open, `cat`, `grep`, `diff`, `head`, copy, upload, or attach any `*.unencrypted.*` file (`.json` or `.jsonc`), any temporary document zeze creates, or anything under `~/.guiho/zeze/` (keys, `audit.log`, `state/`, `run/`). The single exception is `~/.guiho/zeze/zeze.global.yaml`, which only holds policy settings.
2. **Never bypass protections on your own.** Never pass `--dangerously-bypass-protections` unless the human explicitly asked for it in **this** conversation. Never set or unset agent-marker environment variables to get past the human-only gate, and never drive a pseudo-terminal to answer its confirmation word.
3. **Human-only commands are for humans:** `zeze decrypt`, `zeze get`, `zeze key show --private`, `zeze key export`. They exit `3` when an agent runs them. When a human needs one, tell them the exact command to run themselves.
4. **Discover with metadata only:** `zeze list`, `zeze status`, `zeze check`. Reading `*.encrypted.json` and `*.schema.json(c)` is fine; they contain no plaintext.
5. **Set values without echoing them:** pipe the value through stdin, `printf '%s' "$VALUE" | zeze set /path --value-stdin`, so it stays out of shell history and logs. Use `--json` for anything that is not a string.
6. **Use values only through `zeze run`.** Never try to extract a value from a `run` child: do not print it, write it to a file, encode or transform it, or send it anywhere. Output redaction exists to prevent accidents; deliberate extraction is forbidden.
7. **Never guess secrets.** When a value is missing, invalid, or unknown, ask the human to provide it (they can run `zeze set` themselves or edit the plaintext file and run `zeze encrypt`).
8. **Never commit plaintext.** Do not remove the zeze block from `.gitignore`, do not `git add -f` an unencrypted file, and do not weaken `zeze.yaml` to make a check pass.
9. **Never paste values into prompts, issues, commits, docs, or logs**, including ciphertexts and key material.

## Install

Install zeze by following the install prompt referenced in the README (https://github.com/CGuiho/zeze#install): load the `guiho-p-zeze-install` prompt and follow it. In short, run exactly one command for the host:

```text
Windows (PowerShell):  irm https://raw.githubusercontent.com/CGuiho/zeze/main/devops/install.ps1 | iex
macOS / Linux:         curl -fsSL https://raw.githubusercontent.com/CGuiho/zeze/main/devops/install.sh | sh
```

Add `--version 'X.Y.Z'` or `--channel '<name>'` (PowerShell: `-Version` / `-Channel`) to pin a release. Never install zeze through a package manager (npm, Bun, pip, Homebrew, or similar); no such package exists. Verify with `zeze --version` (it prints a raw SemVer such as `0.3.1`).

## Initialize

Run `zeze init` in the project root after every install, reinstall, or upgrade. It is safe to re-run. It:

- installs or refreshes the `guiho-s-zeze` skill in the global agent-skill directories;
- creates `AGENTS.md` if needed and applies or refreshes zeze's managed instruction block (content outside the block is never touched);
- ensures `~/.guiho/zeze/zeze.global.yaml` and the project `zeze.yaml` exist and are valid, and asks for the `agent.evolution` policy;
- for each pair: creates a key pair and an empty encrypted and unencrypted file when no encrypted file exists (it never replaces the key of an existing encrypted file), adds the `.gitignore` block for `*.unencrypted.json` / `*.unencrypted.jsonc`, creates a starter schema when missing, and offers agent-tool deny rules for `*.unencrypted.*` in Claude Code and OpenCode settings (recommend accepting).

`private key not available (expected on agent and CI machines)` is informational, not a failure. `init` needs an interactive terminal when it must ask questions; when it fails for that reason, ask the human to run it.

## Everyday workflows

**Understand the configuration.**

```sh
zeze list                  # pointers, flattened env names, types (schema-declared without the key)
zeze status                # read-only sync state per path
zeze check                 # schema, gitignore, tracked plaintext, key consistency, env collisions, deny rules
```

**Set a value** (encrypts with the committed public key; no private key needed):

```sh
printf '%s' "$DATABASE_URL" | zeze set /database/url --value-stdin       # string
zeze set /database/pool --json 20                                       # number
zeze set /features/beta --json true                                     # boolean
printf '%s' '{"host":"db","port":5432}' | zeze set /database --json --value-stdin   # object, expanded per leaf
zeze set /features/- --json '"search"'                                  # append to an array
zeze set /optional --json null                                          # stores null (not the same as unset)
```

Values default to strings; if the schema says the path is not a string, `set` fails with exit `4` and suggests `--json`. Missing parent objects are created. `set` writes the encrypted file first, then mirrors the value into an existing unencrypted file (`.json` or `.jsonc`; a `.jsonc` file keeps its comments and formatting outside the written value). It never prints the value. Use `--value-file <path>` only for a file you created yourself for this purpose, never for an `*.unencrypted.*` file.

**Remove a value:** `zeze unset /path` removes a member or subtree (parents remain as `{}`). Without the private key only object members and the **last** array element can be removed; for other array elements, ask the human.

**Run with configuration** (`inject` is an alias of `run`, identical in every way):

```sh
zeze run -- npm start
zeze run -- python -m app
zeze run --name production -- ./server
```

By default the child receives the **whole decrypted document, JSON-stringified, in `ZEZE_CONFIG`**:

```js
const config = JSON.parse(process.env.ZEZE_CONFIG); // Node / Bun
```

```python
config = json.loads(os.environ["ZEZE_CONFIG"])  # Python
```

```go
var cfg Config
err := json.Unmarshal([]byte(os.Getenv("ZEZE_CONFIG")), &cfg) // Go
```

Other mappings:

| Flag | Effect |
| --- | --- |
| `--format json` (default) | Whole document as JSON in `--env-name` (default `ZEZE_CONFIG`). |
| `--format flatten` | One variable per leaf: `/database/url` becomes `DATABASE_URL`; `--prefix APP_` prepends a prefix. Names that collide, and names that load or run code or belong to zeze (`LD_*`, `DYLD_*`, `GIT_*`, `JAVA_*`, `NODE_OPTIONS`, `PYTHONPATH`, `BASH_ENV`, `PAGER`, `EDITOR`, `PATH`, `ZEZE_*`, ...), are refused; map one explicitly with `--env` only when the human asked for it. `null` and empty containers are not set. |
| `--format file` | `--env-name` holds a path to the whole document (a memfd on Linux, a `0600` temp file elsewhere, deleted when the child exits). |
| `--env-name NAME` | Variable name for `json` and `file` formats (always set, replacing an inherited value; denylisted names are refused). |
| `--env NAME=/pointer` | Also inject one value (repeatable). |
| `--override` | Flattened and `--env` values win over variables already in the environment (default: existing variables win). |

`run` removes every `ZEZE_PRIVATE_KEY*` variable from the child, redacts config values (and their common encodings) and private keys in the child's stdout and stderr as `[REDACTED]`, and returns the child's exit code. Exit `125` with a `[ZEZE:...]` prefix means zeze failed before or around the child. Seeing `[REDACTED]` is expected; do not try to recover what was redacted. Make the application read `ZEZE_` (or flattened variables) rather than adding `.env` files.

**Human edit cycle** (tell the human; do not do it yourself): `zeze decrypt` (human-only; deletes `config.encrypted.json`) → edit `config.unencrypted.json` → `zeze encrypt` (deletes `config.unencrypted.json`). Only one side of a pair exists at a time unless `--keep-both` is passed; while a pair is decrypted, `set`, `run` and `list` report the encrypted file as missing, so ask the human to finish with `zeze encrypt`. An agent may run `zeze encrypt` when the human asks, because zeze reads the plaintext internally; it refuses paths in `conflict` unless `--force` is given, and an agent must not pass `--force` without the human's approval.

**Status states:** `in-sync`; `unencrypted-newer` (run `zeze encrypt`); `encrypted-newer` (a human runs `zeze decrypt`); `conflict` (a human resolves, then `encrypt --force` or `decrypt --force`); `unencrypted-missing` and `key-unavailable` (normal on agent and CI machines).

## Command reference

Pair-aware commands accept `--name`, `--encrypted-file`, `--unencrypted-file`, `--schema-file`. Every command accepts `-h/--help`, `--help-docs`, `--help-tree`, `--help-tree-depth`, `--help-tree-global-flags`. `zeze -v` / `zeze --version` prints the raw version. `[P]` = protected, human-only.

| Command | Purpose |
| --- | --- |
| `zeze init` | Initialize or reconcile zeze for the project (see above). |
| `zeze encrypt [--force] [--keep-both]` | Unencrypted file → encrypted file; validates the whole document first; reuses unchanged ciphertexts; deletes the unencrypted file unless `--keep-both`. |
| `zeze decrypt [--force] [--keep-both]` [P] | Encrypted file → unencrypted file; deletes the encrypted file unless `--keep-both`. |
| `zeze set <pointer> [value] [--json] [--value-stdin] [--value-file <path>] [--allow-invalid]` | Encrypt and store one value. |
| `zeze unset <pointer> [--allow-invalid]` | Remove one value or subtree. |
| `zeze get <pointer>` [P] | Print one plaintext value. |
| `zeze run [--format json\|flatten\|file] [--env-name NAME] [--prefix P] [--env NAME=/ptr]... [--override] -- <cmd> [args...]` | Run a command with injected, redacted configuration. `inject` is an alias of `run`. |
| `zeze list` | Pointers, flattened names, and types. Never values. |
| `zeze status` | Read-only per-path sync state. |
| `zeze check` | Full consistency and validation check (needs the key for value validation). |
| `zeze key show` | Public key and key ID. `--private` is [P]. |
| `zeze key export` [P] | Print the private key for backup. |
| `zeze key rotate` | New key pair, re-encrypt every value, retire the old key. Needs the private key. Rotation does not revoke git history. |
| `zeze key retire <keyId>` | Delete a retired key once no ciphertext uses it. |
| `zeze schema infer [--force]` | Generate a starter schema from structure and JSON types (never values). |
| `zeze agent skill install\|uninstall\|upgrade\|list\|show` | Manage the bundled skills (this one is `guiho-s-zeze`). |
| `zeze agent instruction apply\|remove\|upgrade\|show` | Manage zeze's managed block in `AGENTS.md`. |
| `zeze agent prompt list\|show` | Inspect bundled prompts: `guiho-p-zeze` (default), `guiho-p-zeze-install`, `guiho-p-zeze-uninstall`. |
| `zeze upgrade` / `upgrade check` / `upgrade list` | Upgrade the whole release in place (verified, rolled back on failure), check for one, list releases. Refuses while `zeze run` sessions are alive (`--force` stops them gracefully, right before activation). Then run `zeze init` and `zeze --version`. |
| `zeze uninstall [--preserve-config] [--preserve-data] [--dry-run] [--yes]` | Remove zeze. Deleting `keys/` always needs the human-only gate; non-interactive runs preserve keys. Follow the `guiho-p-zeze-uninstall` prompt. |

**Exit codes:** `0` success, `1` failure, `2` usage error, `3` protected operation refused (human-only), `4` validation failed (schema), `5` key not found, key mismatch, or decryption failed. `run` returns the child's code, or `125` for a zeze-side failure. Error messages contain JSON Pointers, schema keywords, and schema parameters only, never values.

**Configuration (`zeze.yaml`, project root):**

```yaml
# yaml-language-server: $schema=https://github.com/CGuiho/zeze/releases/download/v<version>/zeze.schema.json
pairs:
  - name: config
    schema: ./config.schema.json
run:
  format: json
  envName: ZEZE_CONFIG
  prefix: ""
  override: false
agent:
  evolution:
    upgrade: always-ask
    issues: { bugs: always-ask, improvements: always-ask, reviews: always-ask }
```

`protections` (`allowBypass`, `agentEnvMarkers`) exists only in `~/.guiho/zeze/zeze.global.yaml` and is rejected in `zeze.yaml`. Never edit it.

## Writing the schema

The schema is a standard **JSON Schema draft 2020-12** document in `<stem>.schema.json`, or `<stem>.schema.jsonc` when you want comments. It is committed and must never contain configuration values. Nobody writes it from scratch:

1. **Generate:** `zeze schema infer` (add `--name <stem>` for another pair). zeze reads the structure and JSON types internally, never the values, and writes objects with `required` for every present key and `additionalProperties: false`, homogeneous arrays as `items`, mixed arrays as `prefixItems`. Without a plaintext file it infers structure only from the encrypted file and leaves leaf types unconstrained (`{}`). It refuses to overwrite an existing schema; pass `--force` only when replacing the old schema is intended (refinements are lost; a `.schema.jsonc` keeps its comments on surviving keys).
2. **Refine** the generated file by editing it, using only what you know from field names, the code that consumes the configuration, and what the human told you. Never infer constraints by looking at values.
3. **Verify:** `zeze check` (it also warns while the permissive starter schema from `init` is still in use).

What to add while refining:

- `description` on every property: what it configures and who sets it. `title` and `$comment` are also fine.
- Precise `type`s (`string`, `integer`, `number`, `boolean`, `object`, `array`, `null`, or a list such as `["string", "null"]`).
- Constraints: `minimum`, `maximum`, `exclusiveMinimum`, `exclusiveMaximum`, `multipleOf`, `minLength`, `maxLength`, `pattern`, `minItems`, `maxItems`, `uniqueItems`, `minProperties`, `maxProperties`, `required`, `additionalProperties`, `patternProperties`, `propertyNames`, `enum` (non-secret choices such as log levels).
- Composition: `anyOf`, `oneOf`, `allOf`, `if`/`then`/`else`, `$ref` with `$defs` for repeated shapes.
- `format`, only from this list (any other format is an error, so typos never pass silently): `date-time`, `date`, `time`, `email`, `hostname`, `ipv4`, `ipv6`, `uri`, `uuid`.
- `"x-zeze-secret": true` on every secret (passwords, tokens, API keys, private keys, connection strings with credentials, signing secrets). `run` then redacts the value in any form and length. When unsure, mark it secret.
- Unknown keywords are errors; custom annotations must start with `x-`.

Never put real values anywhere in the schema: no real values in `default`, `examples`, `const`, or `enum`, and none of those keywords at all on `x-zeze-secret` paths (`zeze check` warns about them). Placeholders that are obviously not real (`"postgres://user:password@host:5432/db"` as a description example) belong in `description` text, if anywhere.

Example refinement:

```jsonc
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "required": ["database", "logLevel"],
  "additionalProperties": false,
  "properties": {
    "database": {
      "type": "object",
      "description": "Primary PostgreSQL database.",
      "required": ["url", "pool"],
      "additionalProperties": false,
      "properties": {
        "url": {
          "type": "string",
          "format": "uri",
          "description": "Connection URL including credentials. Set by a human.",
          "x-zeze-secret": true
        },
        "pool": {
          "type": "integer",
          "minimum": 1,
          "maximum": 100,
          "description": "Maximum open connections."
        }
      }
    },
    "logLevel": {
      "enum": ["debug", "info", "warn", "error"],
      "description": "Minimum log level."
    }
  }
}
```

When the application adds a setting: add it to the schema first (with `description`, type, constraints, and `x-zeze-secret` when relevant), then set it with `zeze set`, or ask the human to set a secret. Several pairs may share one schema (for example `production` and `staging` both pointing at `config.schema.json` through `zeze.yaml`).

## CLI Evolution and Feedback

While using this CLI, treat observed bugs, useful improvements, and actionable review findings as opportunities to evolve the CLI. Follow the effective agent-evolution policy. When issue creation is authorized, create an issue in the CLI's GitHub repository and provide the resulting issue URL to the user.

- Repository: https://github.com/CGuiho/zeze
- New issue: https://github.com/CGuiho/zeze/issues/new (or `gh issue create --repo CGuiho/zeze`)

**Read the effective policy first.** Before upgrading zeze or creating an issue, read `agent.evolution` from the project `zeze.yaml`; any field it omits is inherited from `~/.guiho/zeze/zeze.global.yaml`; any field still missing is `always-ask`. Fields: `upgrade`, `issues.bugs`, `issues.improvements`, `issues.reviews`. Each takes exactly one value:

- `disabled`: do not perform the action and do not ask for permission to perform it. You may still tell the user what you observed.
- `always-ask`: explain the proposed action, ask the user for permission, and wait for approval before acting. Each action needs a new approval. For example: "I found something that appears to be a bug in zeze. Do you allow me to create an issue in the zeze GitHub repository?"
- `always-proceed`: perform the action without asking again, then report the result.

**Upgrades.** While using zeze, check for a newer release with `zeze upgrade check` (subject to `agent.evolution.upgrade`; skip the check when it is `disabled`). With `always-ask`, tell the user the target version and wait for approval; with `always-proceed`, upgrade. After any upgrade (or reinstall): verify `zeze --version` prints the expected version, rerun `zeze init`, and verify `zeze --version` again. If the upgrade fails, run the reinstall command that `zeze upgrade` printed. `zeze upgrade` refuses while `zeze run` sessions are active; do not pass `--force` unless the user agrees to stop them.

**Recognizing what to report** (zeze-specific):

- **Bugs:** a plaintext value, ciphertext, or key material appears where it should not (output, error, log, audit line); `[REDACTED]` missing for a value that should be redacted; a command crashes or exits with a code that contradicts the documented exit codes; `status` reports a wrong state; `set` loses or corrupts a value; `check` passes on a tracked `*.unencrypted.*` file; the `AGENTS.md` block or `.gitignore` block damages content outside it; a valid JSONC file or valid draft 2020-12 schema is rejected (or an invalid one accepted).
- **Improvements:** a missing schema keyword or `format`, an unclear error message, a workflow that forces a human step that could be safely automated, a missing `--help-docs` example, friction in `run` integration for a language or framework.
- **Reviews:** documentation that contradicts behaviour, inconsistencies between this skill, the instruction, and `--help-docs`, or design concerns about agent safety.

**Secret-safe issue content.** An issue may contain only: JSON Pointer paths, JSON types, schema keywords and schema parameters, exit codes, zeze versions (`zeze --version`), OS and architecture, and command lines **with every value removed** (write `<value>`). An issue must never contain: `zeze run` output, contents of any `*.unencrypted.*` file or of temporary files, ciphertexts or `$zeze` blocks, private key material (anything starting with `zeze-sk-1:`), environment dumps, or anything from `~/.guiho/zeze/`. Report a suspected redaction failure by describing it with the leaked text replaced (for example `<leaked value, 24 chars>`), never by quoting it. When in doubt, leave it out and tell the user.

**After creating an issue,** give the user the direct URL of the created issue. Never claim an issue was created unless creation succeeded and returned a URL.
