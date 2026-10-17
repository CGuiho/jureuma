---
name: 2026-10-17 DND Decisions
purpose: Record every question the unattended (DND) workers answer themselves during the key-identity-headers train (issue #13).
description: "DND question ledger for the key-identity-headers train: W1 key-list display format, label storage and relabeling, best-effort key-file metadata, picker behavior and label derivation."
created: 2026-10-17
owner: jureuma-docs-questions-key-identity-headers
flags: []
tags:
  - "jureuma"
  - "questions"
  - "dnd"
keywords:
  - "key identity"
  - "key labels"
  - "key list"
  - "key selection"
  - "issue 13"
  - "question ledger"
---

#### &copy; 2026 [GUIHO](https://guiho.co) as represented by [Cristóvão GUIHO](https://guiho.co/cguiho) All Rights Reserved.

# 2026-10-17 DND Decisions

Context: the key-identity-headers train (issue #13) runs unattended (DND):
never stop, never ask CG, answer every question with the safest reversible
choice and record it here. Entries are grouped by unit.

## W1 — key labels, listing and selection

### Q1 — How short are the "short key ID" and the public-key prefix in listings?

**Answer: 12 characters each.** `key list` shows `k_` plus the first 12 hex
characters of the key ID and `jureuma-pk-1:` plus the first 12 base64url
characters of the public key. CG asked for "the beginning of the public key or
the key ID"; 12 characters are unique in practice for one machine's key set
and stay readable in one line. Display-only, so widening or narrowing later
touches no stored data.

### Q2 — `key list` shows short key IDs only. How does a human get the full key ID?

**Answer: `key show` and the file name carry it.** The brief mandates the line
`label  k_shortid  jureuma-pk-1:prefix  active|retired`, so `export <keyId>`
and `retire <keyId>` cannot be fed straight from `list`. The full key ID stays
available from `jureuma key show`, from `check`/`status` output, and from the
key file name `~/.guiho/jureuma/keys/<keyId>.json`. Safest reading of the
mandated format; if CG wants copy-pasteable IDs in the listing, adding a
long-format option is additive.

### Q3 — What happens when a key is saved again with a different label?

**Answer: the label is updated and `created` is preserved.** Silently ignoring
the new label would make `SaveLabeled` a lie; rewriting with a fresh timestamp
would lose history. Labels are non-secret metadata, so an update is cheap and
reversible. An identical key with an identical label leaves the file
untouched, as before.

### Q4 — `key list` needs labels, but `List` used to "not read contents". What is the contract now?

**Answer: `List` reads each file's non-secret metadata (label, public key)
best-effort and never decodes the private key.** Files that cannot be opened,
checked or parsed — or whose inner key ID contradicts the file name — still
list by name with placeholder metadata, so `key retire` state detection keeps
working. The decode target struct simply has no `privateKey` field, so key
material cannot materialise during listing.

### Q5 — What does `key list` print for an empty store?

**Answer: nothing, exit 0.** One line per stored key is the whole output
contract; an empty-state sentence would be indistinguishable from data for
parsers and for agents reading metadata. `key list` stays machine-readable.

### Q6 — What do `key show` and `key list` display when a key has no label?

**Answer: the placeholder `—`.** Covers legacy key files created before
labels existed and env-only keys (`JUREUMA_PRIVATE_KEY*`) that have no file at
all. The placeholder is display-only.

### Q7 — How does the stored-key picker decide it may prompt, and what may it read?

**Answer: character-device stdin means a TTY (`os.Stdin.Stat()`,
`ModeCharDevice`), no new dependency.** In a TTY it prints the `key list`
lines and prompts `Which one do you want to use? `; the answer is a line
number, a full key ID or a short key ID; an empty line or EOF cancels
(`ErrNoKeySelected`). Outside a TTY it never reads the injected reader and
returns the typed `ErrNoInteractiveSelection`, so callers must take an
explicit `--key-id` / `--key-label` (W3 wires `init` to that rule).

### Q8 — Where does the default label come from, and what if git is absent?

**Answer: the `origin` remote's repository name (`.git` suffix stripped,
https/ssh/scp/path remotes all supported), falling back to the project
directory name; an empty result means no label.** The git lookup reads local
configuration only (`git remote get-url origin`), never the network, and never
fails hard: no git, no remote, or any error falls through to the directory
name. The lookup is injectable for tests.
