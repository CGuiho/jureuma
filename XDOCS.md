# Jureuma XDocs Root

This repository uses xdocs for structured, machine-readable documentation.
The root descriptor and every per-directory named descriptor are browsable
with `xdocs tree`.

## Root Descriptor

- [jureuma.xdocs.md](jureuma.xdocs.md): Root descriptor for this repository.

## Top-Level Directories

- [cmd/](cmd/): Container for the Go main packages (executables) built from this module.
- [devops/](devops/): Lifecycle scripts (install, uninstall) and the build, release, smoke and lint automation cataloged in `runx.yaml`.
- [docs/](docs/): Design and implementation-contract documents, plus the planning, question-ledger and review trees of the Mandume lifecycle.
- [internal/](internal/): Internal Go packages (not importable outside the module).
- [test/](test/): Test suites that exercise the built jureuma binary.
