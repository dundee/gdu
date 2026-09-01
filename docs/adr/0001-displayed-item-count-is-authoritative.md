# 1. The displayed item count is authoritative

Date: 2026-09-01

## Status

Accepted

## Context

`Item.GetItemCount()` counts a directory *including itself*: an empty directory
returns 1, and a directory holding three files returns 4.

Three call sites derived a user-facing number from it, and they disagreed:

- `tui/format.go` subtracted 1 for directories before rendering the item count
  column.
- `pkg/fs.ByItemCount` (the comparator behind sorting by item count, used by
  both the TUI and the web UI) compared the raw value.
- `webui/tree.go` serialized the raw value to the browser.

So the terminal sorted by one number and printed another, and the two frontends
printed different numbers for the same directory. The mismatch was invisible in
practice until we set out to draw a *bar* proportional to the count
(issue #599): a bar that contradicts the number printed beside it is obviously
wrong in a way that a subtly misordered list is not.

The raw and displayed values only diverge at the boundary between an empty
directory (raw 1, displayed 0) and a file (raw 1, displayed 1), so the two
orderings differ by at most a small reshuffle — which is precisely why the
inconsistency survived this long.

## Decision

There is one definition of the count, `fs.DisplayedItemCount`, and everything
conforms to it: the comparator, the TUI renderer, and the web UI serializer.

Where display and ordering disagreed, **display wins**. The number the user can
see is the number we sort by.

The file/directory asymmetry is kept: a file reports 1, a directory reports its
contents excluding itself.

## Consequences

- Sorting by item count now places an empty directory below a file. This is a
  behaviour change to existing output in both the TUI and the web UI.
- The web UI's displayed item count for directories drops by one. This was
  outside the scope of the originating issue, but leaving it would have made
  the web UI exhibit the very defect being fixed in the TUI, since the
  comparator is shared. The change is Go-side only — the compiled frontend in
  `webui/dist` is untouched, so no Node build is required.
- Adding a count bar becomes safe: it is drawn from the same value it is
  labelled with.
- The asymmetry is now documented rather than accidental. A uniform
  "items contained" rule (file = 0) was rejected: it would render every file
  row as `0` with an empty bar, which is strictly less informative.
