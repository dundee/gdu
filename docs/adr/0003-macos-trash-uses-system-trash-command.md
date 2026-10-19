# 3. The macOS trash runs the system trash command

Date: 2026-10-19

## Status

Accepted

## Context

The built-in trash targets the FreeDesktop trash, which Finder ignores, so on
darwin it was a stub that returned "not supported".
[ADR 0002 (external trash command)](0002-external-trash-command.md) left macOS
users a workaround (`--trash-command`) and noted that probing for a trash CLI
remained open.

The web UI's cleanup projections make moving many items to the trash a common
action, so it has to work without configuration on the platform most of its
users run.

macOS 15 (Sequoia) and later ship `/usr/bin/trash`, which moves items into the
Finder trash, where they can be put back. It is missing on macOS 14 and older.

## Decision

On darwin, `MoveItemToTrash` runs `/usr/bin/trash <abspath>` directly, without a
shell. `trash` does not understand `--`, but the path is always absolute, so it
can never be read as an option.

As with the trash command, the tree is updated from the filesystem: the item is
dropped only when its path is really gone.

When `/usr/bin/trash` does not exist, the error points the user at
`trash-command`, which still takes precedence when set.

## Consequences

- `D` in the terminal UI and "Move to trash" in the web UI work on macOS out of
  the box, and trashed items can be restored from Finder.
- Older macOS releases without `/usr/bin/trash` keep the previous behavior, an
  error, with a hint towards the workaround.
- No Objective-C bridge or cgo is needed.
