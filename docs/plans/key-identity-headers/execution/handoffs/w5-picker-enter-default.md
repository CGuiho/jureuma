#### &copy; 2026 [GUIHO](https://guiho.co) as represented by [Cristóvão GUIHO](https://guiho.co/cguiho) All Rights Reserved.

# Handoff — W5: init key picker defaults to "create a new key" (issue #15)

Mode: dnd · Role: workhorse (engineer) · Harness: OpenCode native background
subagent (`general`) · Model: `xiaomi/mimo-v2.6-pro` (provider-default
thinking, no variant) · Issue: <https://github.com/CGuiho/jureuma/issues/15>

**Base commit:** the commit that adds this brief (verify with `git log -1`).

## Mission

CG hit Enter in `jureuma init`'s stored-key picker and it re-prompted
"enter a number between 1 and 3". Enter must select **create a new key**
immediately. The synthetic entry must also render honestly instead of showing
empty key-ID/`active` columns.

## Required behavior

1. **Picker default.** `pickKey` gains a default-index variant (keep the
   existing four-argument `pickKey` behavior and tests intact by delegating to
   it with no default). `init` calls the default variant with the synthetic
   "create a new key" entry (last) as the default.
2. **Enter selects the default.** An empty line (Enter) returns the default
   entry immediately — no second prompt, no number required. EOF (Ctrl-D, no
   line) still cancels with `ErrNoKeySelected`. A garbage line still
   re-prompts; a number, full key ID and short key ID still select as today.
3. **Honest rendering.** The synthetic entry renders as
   `create a new key (default)` (label only; no empty key-ID column, no
   `active` state). Real entries keep the current columns. The prompt names
   the default, e.g. `Which one do you want to use? (Enter = create a new
   key) `.
4. **Prompt loop.** Empty input with a default never prints the "enter a
   number…" hint; it returns. Without a default (generic `pickKey`), today's
   cancel/re-prompt behavior is unchanged.
5. **`init` flow.** With Enter, `chooseInitKey` receives the synthetic entry
   (empty KeyID) and falls through to creating a labeled key exactly as today;
   no extra confirmation, no second picker.

## Files you own

`internal/cli/keysel.go`, `internal/cli/keysel_test.go`,
`internal/cli/cmd_init.go` (picker call only), `CHANGELOG.md` (add an
`## Unreleased` section with one `### Fixed` bullet for this behavior; the
parent renames it at the 0.3.1 release), and any xdocs descriptor text that
describes the picker. Do not change `internal/pair/**`, `internal/jsonc/**`,
`internal/keystore/**`, `todo.md`, `docs/plans/**` (except reading this
brief), release machinery or version pins.

## Tests

- Unit: default selected on Enter; default entry rendered with
  `(default)` and no `—`/`active`; EOF cancels; number/key-ID/garbage paths
  unchanged; non-TTY still refuses without reading; generic `pickKey`
  unchanged.
- e2e or CLI-level if practical; otherwise the unit coverage above is the
  gate.
- Full battery: `runx run go-fmt-check`, `go-vet-cross`, `go-test`,
  `test-e2e`, `test-smoke`, `docs-check`, `ci-check` — all exit 0; `xdocs
  doctor` 0/0.

## Commits and report

One or two coherent commits with imperative messages through
`flock -w 120 /tmp/opencode/jureuma-git.lock`; stage only owned paths; never
`git add -A`; never push; worktree clean. Report commits, changed files, exact
commands + exit codes, test names, a value-free before/after transcript of the
picker (using a pipe is fine), deviations and residual uncertainty. DND: never
ask a human; ledger undecided matters in
`docs/questions/key-identity-headers/2026-10-17-dnd-decisions.md` (Q40+).
