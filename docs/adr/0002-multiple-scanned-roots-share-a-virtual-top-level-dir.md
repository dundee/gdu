# 2. Multiple scanned roots share a virtual top level directory

Date: 2026-09-02

## Status

Accepted

## Context

gdu accepted exactly one directory to scan (`cobra.MaximumNArgs(1)`). Issue #423
asks for a subset of a large directory's children to be scanned together: with a
hundred subdirectories, scanning all of them takes minutes and scanning them one
at a time defeats the purpose, which is to compare them.

Everything downstream of the analyzer assumes a single-rooted tree. The TUI
anchors navigation on a `topDir`/`topDirPath` pair and stops the `/..` row at it;
`Dir.subtractStats` walks parent pointers to the root; the ncdu-compatible export
format holds exactly one root array. A second root has nowhere to go.

Two shapes were available:

1. Keep N independent trees and teach every consumer to iterate them.
2. Give the N roots a synthetic parent, so there is still one tree.

## Decision

Multiple scanned roots are grouped under a **virtual top level directory**
(`analyze.CreateVirtualRootDir`), and the rest of gdu keeps working on the single
rooted tree it already expects.

The virtual root is a real `analyze.Dir` with no parent and no `BasePath`, named
`(multiple)`. Because `Dir.GetPath` prefers `BasePath` over the parent chain, the
roots below it keep reporting their true absolute path even though they now have
a parent — so rescan, deletion, shell spawning and path display need no special
handling once the user has navigated into a root.

Consequences of the virtual root having no path are handled by refusing the
operation rather than inventing one: `--change-cwd`, spawning a shell, browsing
to a parent (`--browse-parent-dirs`) and JSON export are unavailable while it is
displayed.

Rows directly under the virtual root are **labelled with their absolute path**
rather than their base name (`analyze.ItemDisplayName`). Two roots taken from
different parents routinely share a base name — `gdu /a/data /b/data` would
otherwise show `data` twice — and the absolute path is what the user typed.
Below the roots, ordinary base names resume.

Two related decisions:

- **Nested roots are rejected**, not deduplicated. `gdu /a /a/b` counts `/a/b`
  twice in every total gdu reports, and no silent resolution of that is
  unsurprising. Exact duplicates are dropped, since they have one obvious
  meaning.
- **Export (`-o`) and storage (`--db`) accept one directory only.** Both are
  keyed by a single root, and inventing a multi-root encoding would break
  compatibility with ncdu for a case nobody has asked for yet.

## Consequences

- Roots are scanned sequentially, not concurrently. The analyzer owns one set of
  progress channels per scan and `ResetProgress` replaces them, so a second
  concurrent scan on the same analyzer would close a closed channel. Progress
  therefore restarts per root, and the modal reports `Scanning 2/3...`.
- `ResetProgress` also clears the analyzer's cancelled flag, so a cancel request
  made between roots would be lost. The TUI tracks the request separately in
  `scanCancelRequested` and stops the loop itself, keeping partial results as a
  cancelled single scan does.
- The non-interactive stdout mode gives up its memory-efficient `SimpleDir`
  analyzer for multiple roots: `SimpleDir` deliberately panics on `SetParent` and
  cannot be re-parented. `--top` and `--depth` already make the same trade.
- Labelling roots by path means the directory marker (`/` in the TUI and in
  stdout) has to be suppressed for them, since an absolute path already opens
  with a separator. Filtering at the virtual root matches against the path shown,
  not the base name, so what is typed matches what is on screen.
