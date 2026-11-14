# 9. One database contract, with a completion marker

Date: 2026-11-14

## Status

Accepted

## Context

`--db` keeps a scan in a badger or an SQLite database, chosen by the file
extension, and `-r` loads it again in a later run. The two backends shared a
method name, `LoadAnalysis`, but not its meaning:

- **No common type.** The session found the database analyzers through a
  private interface of its own, and each backend described its errors in its
  own words. A caller could not tell "no analysis of this path" from "written
  by another gdu" except by the message.
- **No record of how the scan ended.** Neither database recorded whether the
  scan that wrote it had finished. Killed in the middle of a scan:
  - badger kept every directory stored so far, without totals, and `-r` showed
    that partial tree as the analysis;
  - SQLite reported no analysis, as its rows were not committed yet.

  Neither could tell a stopped scan from a finished one. A write that failed during the scan was logged and forgotten, so the
  analysis lacked part of the tree with nothing to tell.
- **No format of the SQLite layout.** Only badger's records carried a format
  version.

## Decision

- `analyze.Database` is the contract of both backends: an `Analyzer` whose
  `AnalyzeDir` replaces the database's analysis with the scan, and whose
  `LoadAnalysis(path)` returns the stored analysis of path. Its failures are
  sentinel errors: `ErrNoAnalysis` (nothing stored for that path),
  `ErrIncompleteAnalysis` and `ErrStoredFormat` (another gdu wrote it).
- Each database records what it holds: the format of its layout, the scanned
  root and a state.
  - The scan records **unfinished** before it changes anything.
  - Once the totals are stored, it records **complete**, or **stopped** when
    the user stopped the scan. SQLite stores the totals as it scans; badger
    when `UpdateStats` computes them on the returned tree.
  - When any write of the scan failed, the state stays unfinished.
- `LoadAnalysis` loads a complete or a stopped analysis, as a stopped scan is
  kept and browsed in memory too. It refuses an unfinished one.
- A database recording nothing was written by an older gdu, and is refused as
  another format; a later scan replaces it.
- The record is part of each backend's layout: a key no path can take in
  badger (`"\x00analysis"`), a `metadata` row in SQLite. Both hold the same
  JSON.

## Consequences

- `-r` never shows a partial tree as the analysis. After gdu was killed it
  tells the user to scan again.
- Databases written before this change, by any released gdu, are refused
  with "written by another version of gdu" and have to be scanned again.
- A program using `pkg/analyze` can tell the failures apart with `errors.Is`,
  and accept any `Database`.
- A badger analysis whose tree never got its totals computed, by a program
  that scans without `UpdateStats`, stays unfinished. Without the totals it
  could not be loaded correctly anyway.
- The open questions of the backends are left as they were: badger still
  opens the database per operation, and SQLite still trades crash safety for
  speed with its pragmas. A crash now leaves an analysis that is refused
  rather than loaded, unless the database itself cannot be read.
