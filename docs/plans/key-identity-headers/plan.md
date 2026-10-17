---
name: Key Identity Headers and Single-File Pairs
purpose: Record the train that puts the `$jureuma` identity header on both pair files, makes single-file operation first-class, and adds key labels with stored-key selection (issue #13).
description: "Plan for the key-identity train: format-2 `$jureuma` headers on both files with the always-present `$schema` pointer and the embedded encrypted schema, single-file operation, `init` repair and key selection, labeled keys with a listing and a TTY-safe picker (issue #13)."
created: "2026-10-17"
owner: jureuma-docs-plans-key-identity-headers
flags: []
tags:
  - "planning"
keywords:
  - "key identity"
  - "pair header"
  - "single-file pairs"
  - "key labels"
  - "key selection"
status: stable
---

#### &copy; 2026 [GUIHO](https://guiho.co) as represented by [Cristóvão GUIHO](https://guiho.co/cguiho) All Rights Reserved.

# Key identity headers and single-file pairs

## Context

CG directive (voice, 2026-10-17, issue #13): the public key currently lives
only in the committed encrypted file, so a bare unencrypted file cannot tell
jureuma — or its holder — which key pair it belongs to. jureuma must carry a
`$jureuma` header on **both** files (public key, key reference `keyId`, and
the global directory reference) and must work with **either file alone**:
only-unencrypted or only-encrypted; both present stays allowed but is not
assumed ("having both is risky"). Headerless legacy files are repaired by
re-running `jureuma init`. Every created key pair gets a label (default: the
project/repository name); key listing shows the label plus the short keyId /
public-key prefix, and commands let the user pick from already stored keys.
After implementation, the next minor release (0.3.0) ships this together
with #10, #11 and #12.

Additional directive (2026-10-17, referencing `/root/swe/auth-ui/config.unencrypted.json`
and its jureuma-migration note): the unencrypted file must **always** specify
the path to the local schema — "always, always, always" — and the encrypted
file must also carry a property holding the schema in encrypted form, so each
file stays self-describing. auth-ui grounds this: its schema declares the
optional `$schema` string property ("Always specify it in the unencrypted file
so editors validate and autocomplete") and its note says the unencrypted
document always points at `./config.schema.json`. jureuma makes that pointer
first-class (always written, repaired, preserved, checked) and turns the
top-level `$schema` into reserved metadata — never a value, listing, injection
or encrypted body entry.

## Units

### W1 — key labels, listing and selection (issue #13, part 1)

Keystore labels with a repository-name default, `key list`, label on
`key show`, and a TTY-safe stored-key picker helper. File-disjoint from the
in-flight JSONC writer; dispatched immediately. Brief:
`execution/handoffs/w1-key-labels-and-selection.md`.

### W2 — `$jureuma` identity header on both files (issue #13, part 2)

Format-2 identity block (`format`, `publicKey`, `keyId`, `home`) on the
encrypted and unencrypted files; the always-present `$schema` pointer and the
`$jureuma.schema` encrypted copy; body exclusion of reserved keys; cross-file
consistency checks; typed legacy-header repair pointer; header and pointer
insert/repair through the JSONC lossless engine. Brief:
`execution/handoffs/w2-pair-identity-headers.md`.

### W3 — single-file flows, `init` repair and `init` key selection (issue #13, parts 3–4)

Only-unencrypted and only-encrypted operation as first-class states; keyless
`set` building the encrypted sibling from the plaintext header (embedding the
encrypted schema copy); `init` header and `$schema` repair; stored-key
selection on `init`; schema restore on `decrypt` and refresh on
`schema infer`; status/check reporting across all states. Brief:
`execution/handoffs/w3-single-file-flows-and-init.md`.

### W4 — project key binding in `jureuma.yaml` (issue #14, CG directive 2026-10-17)

An optional top-level `key:` (full key ID) names the one key the project uses;
every pair must agree with it (mismatch exit 5, never a silent re-encrypt).
`init` uses the bound key when a pair records no identity and adds the binding
through a byte-preserving one-line splice; `key rotate` updates it the same
way. Labels stay display metadata and are not accepted in the binding. Brief:
`execution/handoffs/w4-project-key-binding.md`.

## Sequencing

Planned and packet-validated by UltraCode (session
`ses_eb5c1925dffessB1Ia5P7XuL9o`), 2026-10-17. W1 is file-disjoint from the
JSONC write-support unit (issue #12) and runs concurrently with it; W2 and W3
dispatch only after the JSONC worker's commits have landed and been reviewed,
because they share `internal/pair/files.go`, `internal/cli/cmd_init.go`,
`internal/cli/cmd_values.go`, the design document, `CHANGELOG.md` and
`test/e2e/lifecycle_test.go` with it. One serialized Git index throughout
(`flock -w 120 /tmp/opencode/jureuma-git.lock`); all work on `main`; workers
commit owned units and never push.

## Decisions

Confirmed by the orchestrator from the UltraCode packet v1 (safest-reversible
defaults; stated to CG at dispatch time):

- **Format 2.** New headers are written as `format: 2` on both files. Legacy
  is old jureuma **state**, never header absence: a `$jureuma` block with
  `format: 1`, or a present-but-malformed block, on either side. Reads keep
  working; mutations through such files refuse with an explicit
  `run jureuma init to repair` pointer (enforced in the pair mutating entry
  points, so untouched command callers inherit it). A headerless plaintext
  stays fully mutable (encrypt, mirror and both-present flows — the
  untouched suite pins this and W3 keeps it green); the repair pointer for
  missing plaintext identity belongs to the single-file identity paths,
  `check` and `init` repair. (Amended by UltraCode packet v2.1; see the W2
  brief's A4 block.) Old jureuma readers already reject unknown `$jureuma`
  fields; the explicit version makes the break legible instead of a shape
  surprise.
- **Identity-only plaintext header.** The unencrypted header carries
  `format`, `publicKey`, `keyId`, `home` (the `~/.guiho/jureuma/` global
  directory reference); `sync`/`pending` stay encrypted-side only, so the
  plaintext header is stable and a keyless `set` can rebuild the sibling
  from it. The schema pointer lives beside the header as a real top-level
  `$schema` property (editor convention, auth-ui precedent), never inside it.
- **Always-present `$schema` pointer (2026-10-17).** The unencrypted document
  always carries `"$schema"` pointing at the local schema
  (`./config.schema.json` or `.jsonc`): `init` writes it, repair restores it,
  every write preserves it, and it is reserved metadata — excluded from
  values, listings, `run` injection and encrypted bodies; `check` reports it
  when missing; `schema infer` includes the optional `$schema` string
  property in generated schemas.
- **Encrypted schema copy (2026-10-17).** The encrypted header carries
  `$jureuma.schema`: the readable schema path plus the schema document
  encrypted to the pair key, written as `{ path, c }` when the embedded copy
  is present (the historical plain-string path form stays readable in any
  generation and remains the written form when no copy exists — packet
  v2.1), so a key holder restores the schema from the committed file alone
  (a keyless sibling build embeds it too — encryption needs only the public
  key). Additive: the readable local schema file stays as-is; `schema.path`
  mismatch across files and drift between the embedded copy and the local
  file are warnings.
- **Both-present consistency.** `publicKey`/`keyId` must match across files
  and against the stored key where present (mismatch = exit 5, today's
  keyId↔publicKey rule); `home` mismatch is a warning only.
- **Keyless rebuild baseline.** A keyless `set` building the encrypted
  sibling writes `c` per leaf and empty `pending`; `h` stays absent until a
  key holder runs `encrypt`/`decrypt`; `status` never claims `in-sync` with a
  missing `h`.
- **Label default.** Label = basename of the `origin` git remote, falling
  back to the project root directory name; labels are non-secret metadata and
  no listing path ever shows private key material.
- **Selection scope.** The interactive picker (`Which one do you want to
  use?`) runs only in a TTY (human `init`); agents/CI pass `--key-id` /
  `--key-label`, and non-TTY `init` keeps today's create-new default.

## Gates

- Unit gate: full battery (`go-fmt-check`, `go-vet-cross`, `go-test`,
  `test-e2e`, `test-smoke`, `docs-check`, `ci-check`) green; the orchestrator
  reviews actual commits, diffs, checks and readbacks per unit.
- Release gate: the 0.3.0 minor bump and release via Mirror only on CG's
  explicit word, after the whole train (this plan and #10/#11/#12) is green.
