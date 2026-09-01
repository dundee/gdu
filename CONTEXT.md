# Context

Glossary for gdu. Terms only — no implementation detail, no decisions.
Architectural decisions live in [`docs/adr/`](docs/adr/).

## Item

Anything gdu can put on a row: a file or a directory. Devices are not items —
they are listed by a separate screen with their own vocabulary.

## Apparent size

The size an item claims to be, i.e. the length of its contents.

## Disk usage

The space an item actually occupies on disk, i.e. its allocated blocks. Differs
from apparent size for sparse files, for files smaller than a block, and for
hard-linked files.

Apparent size and disk usage are interchangeable *metrics*: exactly one of them
is displayed at a time, and the user switches between them.

## Displayed item count

The number of items a row reports.

- For a **directory**: the number of items (files and subdirectories,
  recursively) contained in its tree, **not counting the directory itself**.
- For a **file**: 1 — a file counts itself.

The asymmetry is deliberate. A file row reports the one file it stands for; a
directory row reports what is inside it. A consequence is that an empty
directory reports 0 while a file reports 1.

This is the authoritative count: it is what every frontend renders, and what
ordering by item count sorts on, so the number on screen and the ordering
always agree. It is distinct from the raw internal count, which includes the
directory itself.

## Size bar

The horizontal bar drawn on each row to show that item's magnitude relative to
its siblings, measured in the currently displayed size metric (apparent size or
disk usage).

## Count bar

The horizontal bar drawn on each row to show that item's displayed item count
relative to its siblings. Optional, and only meaningful alongside the item
count column.

## Bar denominator

What a bar is measured against. Two modes, applying to every bar alike:

- **Sum of siblings** (default) — each row's bar shows its share of the total,
  so a full row of bars accounts for 100%.
- **Largest sibling** — the biggest sibling fills the bar and everything else
  is drawn in proportion to it.

Rows the user has ignored are excluded from the denominator and draw an empty
bar.
