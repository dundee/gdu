# 7. Items only read the tree

Date: 2026-11-14

## Status

Accepted

## Context

`fs.Item` was the one interface of every item of a scanned tree, with 19
methods:
- reading: the getters and `GetFiles`;
- the totals: `UpdateStats` and `GetItemStats`, the recursion behind it;
- the export: `EncodeJSON`;
- changing the tree: `SetParent`, `AddFile`, `RemoveFile` and
  `RemoveFileByName`.

Most of its users only read: the frontends, cleanup, the hard-link index and
library users walking a tree. Item types that cannot change, or have no
parent, satisfied the rest by panicking. `File` panicked on `AddFile`,
`RemoveFile` and `RemoveFileByName`, and the top-dir analyzer's `SimpleDir` on
nine methods, so whether a call was safe depended on the dynamic type.
`RemoveFileByName` had no caller left.

Outside `pkg/analyze`, the tree was changed in only a few places: the session
replacing a rescanned directory, the deleter taking out what it deleted, and
the trash functions, which removed the item from the tree themselves under a
lock the deleter wrapped around the directory.

## Decision

- `fs.Item` only reads the tree: its 11 getters and `GetFiles`.
- `analyze.Node` is an `fs.Item` that can also compute the totals of its tree
  (`UpdateStats`) and export it (`EncodeJSON`). Analyzers, `LoadAnalysis` and
  the session return a `Node`, so `tree.UpdateStats(...)` and
  `tree.EncodeJSON(...)` stay as library users call them.
- The rest of the model is internal to `pkg/analyze`: an unexported interface
  computes the totals of each entry, and `SetParent`, `AddFile` and
  `RemoveFile` stay methods of the concrete types.
- Other packages change a tree with `analyze.RemoveEntry` and
  `analyze.ReplaceEntry`. These keep the totals and the hard-link index in
  step, and return `ErrNotChangeable` for a directory that cannot change.
- A trash function (`remove.Trash`) only moves an item to the trash and
  reports whether it left its place. The deleter changes the tree, holding
  the session's lock.

## Consequences

- No item type panics. `SimpleDir` has a parent, a type and totals, and is
  exported like any other directory.
- Replacing an emptied file counts it again: emptying a file no longer took
  one off the item count of its directories.
- A `Node` of another package among the entries of a tree computes its own
  totals, so test fakes can wrap one. Any other foreign item keeps the totals
  it reports.
- `fs.Item` and `analyze.Node` are public API. Library users iterating a
  tree change nothing. Code calling `GetItemStats`, `SetParent`, `AddFile`,
  `RemoveFile` or `RemoveFileByName` through `fs.Item`, or the removed
  `fs.Files.FindByName` and `RemoveByName`, does not compile any more.
