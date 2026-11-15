# 6. A small public Go API

Date: 2026-11-13

## Status

Accepted

## Context

Every package of the module outside `internal/` can be imported by other
modules: `pkg/` (analyze, annex, cleanup, device, fs, path, remove, selfupdate,
timefilter), the frontends (`report`, `stdout`, `tui`, `webui`), `cmd/gdu/app`
and `build`. `CONTRIBUTING.md` called all of `pkg/` reusable libraries, but
nothing said which of it was meant to stay stable, and its exported signatures
used types from `internal/common`, which other modules cannot name. Importing
`pkg/analyze` also pulled the terminal UI libraries in through
`internal/common`.

Other projects do use gdu as a library. A search of GitHub in November 2026
(pkg.go.dev has no page for gdu, and deps.dev no dependents) found:

- `pkg/analyze` and `pkg/fs`, all to scan a directory, compute its totals and
  walk the tree: dundee/disk_usage_exporter, vudsen/ai-disk-cleaner,
  ice-blockchain/subzero, brandtg/burrow;
- `cmd/gdu/app` and `pkg/device`, to embed the whole interactive UI:
  nerdneilsfield/Beav;
- `build`, for the version: vudsen/ai-disk-cleaner;
- `cmd/gdu`, blank-imported to pin the tool: pnoguera/tools.

No other package was imported. The API used had already broken within v5
(`GetProgressChan` and the arguments of `EncodeJSON`), and the refactoring of
the analyzers had made `AnalyzeDir` impossible to call from another module, as
its options were a type of `internal/common`.

## Decision

The public Go API is these packages, and every other one lives in `internal/`:

- `pkg/analyze` — scanning a directory into a tree, and its options;
- `pkg/fs` — the tree;
- `pkg/device` — mounted devices, which `cmd/gdu/app` takes;
- `cmd/gdu/app` — running gdu inside another program;
- `build` — the version;
- `cmd/gdu` — the command, for `go install`.

Their exported identifiers use no types of internal packages, so another
module can name everything it is given and build everything it must pass.
`internal/publicapi` lists the public packages and tests both rules: that no
package outside `internal/` is missing from the list, and that no exported
identifier of a listed package uses a type of an internal package.

## Consequences

- Moving the frontends, deletion, cleanup classification, self-update and the
  helpers to `internal/` lets them change freely.
- `pkg/analyze` no longer depends on the terminal UI libraries; a program using
  it links badger and SQLite still, as the analyzers storing their results in a
  database live there.
- The public packages still change while the architecture review is carried
  out (the `fs.Item` interface is due to be split), so changes to them are
  called out in the release notes, with how to migrate.
- `cmd/gdu/app` is public for embedding gdu, and its `Flags` will change with
  the configuration; a program embedding gdu follows those changes.
