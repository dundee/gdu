# 5. The session owns the tree, and one lock guards it

Date: 2026-11-13

## Status

Accepted

## Context

The tree a scan produces is read and changed by many goroutines: the TUI's
event loop, its background deletion workers, the export, a rescan, the web
UI's request handlers and its cleanup classifier.

Each `analyze.Dir` had a mutex of its own, taken by every getter, and every
`fs.Item` offered both `GetFiles`, for callers holding `RLock()`, and
`GetFilesLocked`. That made each call safe, but not the reads made of several
calls: rendering a directory, which reads its entries and theirs, or encoding
the tree, ran while deletion workers removed entries, and the race detector
found the TUI's export reading an entry list a worker was changing. A process-
global mutex guarded the flags of files, every directory carried 24 bytes of
mutex, and sorting a directory took two locks per comparison.

## Decision

After a scan, the tree belongs to the session (`internal/session`), and one
read/write lock guards it, `Session.TreeLock`:

- Whoever reads the tree holds it for reading, around the whole read: the TUI
  around rendering a directory, its info panel and the export; the web UI
  around each request; the classifier around each directory it reads.
- Whoever changes the tree holds it for writing, around the change only. The
  session does for a rescan and for computing totals; the `remove.Deleter`,
  given the lock with `remove.WithTreeLock`, does around `RemoveFile` and
  `AddFile`, after changing the disk, which can take long.
- Fields that never change once an item is in the tree (names, parents, the
  sizes and flags of files, the devices of directories) can be read without it.

During a scan, the walk owns the tree it builds. Only the root and the
attaching of a complete directory to its parent are shared, under a mutex of
the scan that the preview's snapshot takes too.

The per-directory mutex, `fs.Item.RLock` and `GetFilesLocked` are gone.

## Consequences

- A read that holds the lock sees a consistent tree, and "is this safe?" has
  one answer for every frontend.
- `sync.RWMutex` is not reentrant: a goroutine holding the lock for reading
  must not take it again, which would deadlock once a writer waits. Session
  methods that take it (`RootPaths`, `Rescan`, `Snapshot`) say so, and the
  frontends take it in the functions that read, not in the ones they call.
- A long read delays changes. The classifier therefore reads one directory at
  a time, and discards a walk that overlapped a change, as it already did.
- A tree is not safe for concurrent use on its own: code using `pkg/analyze`
  outside the session must serialise its changes and reads itself.
