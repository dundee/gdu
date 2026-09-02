# 2. The external trash command receives the path as a shell argument

Date: 2026-09-02

## Status

Accepted

## Context

The `D` key moves an item into the FreeDesktop trash. That implementation is
fixed: it always targets `$XDG_DATA_HOME/Trash`, and it exists only on
non-macOS Unix — darwin and Windows get a stub that errors out.

Users asked for the escape hatch ncdu offers as `--delete-command`
(issue #640): a custom trash directory via `trash-put --trash-dir DIR`, or a
working `D` key on macOS via `trash`/`gio trash`.

ncdu appends the absolute path to the given string and evaluates the result in
a shell. Appending the path *textually* means a file named `; rm -rf ~` becomes
shell syntax. Not using a shell at all avoids that, but then `~` and `$HOME` in
a command read from `~/.gdu.yaml` are never expanded, which is exactly the
"custom trash dir" case the option exists for.

Appending also assumes the command ends where an argument can follow. That
holds for `trash-put` and `gio trash --`, but not for `mv`, which takes its
destination last. Testing showed the natural reaction is to write
`mv "$1" ~/Trash`, which under an unconditional append receives the path twice.

A second question is when to drop the item from the in-memory tree. gdu has no
per-item refresh; it only knows how to remove a row. The command is arbitrary,
so exit code 0 does not imply the item is gone.

## Decision

The command is evaluated by `/bin/sh -c '<command> "$@"' gdu <abspath>`.

The shell is kept, so expansion and other shell syntax work. The path is passed
as a positional parameter rather than concatenated, so item names cannot be
interpreted as shell syntax. `/bin/sh` is used rather than `$SHELL`, because
`"$@"` must behave predictably.

The `"$@"` suffix is omitted when the command already refers to the path via
`$1`, `$@` or `$*`. The path is then wherever the user put it, which is what
commands taking their destination last need.

The tree is updated from the filesystem, not from the exit code: after a
successful command gdu stats the path and only removes the row if the path is
really gone.

The command runs without a terminal and the UI is not suspended, so interactive
commands are unsupported.

Windows returns "not supported", matching the existing built-in trash stub.

## Consequences

- A command without a placeholder must be a prefix that a path can be appended
  to. `gio trash --` works; a command ending in `;` or a pipeline does not.
  Writing `"$1"` lifts the constraint, so unlike ncdu there is always a way out.
- The placeholder rule is implicit: a command containing `$@` for an unrelated
  reason never receives the path. It then trashes nothing, and because the tree
  is updated from the filesystem the row simply stays — visible, not silent
  corruption.
- A command that succeeds without removing the item (an interactive command
  answered "no", or a typo like `ls`) leaves the row in place silently. The
  tree never claims something was trashed when it wasn't, at the cost of a
  key press that appears to do nothing.
- An item *moved elsewhere* by the command is dropped from the tree rather than
  re-parented, since gdu does not rescan. Same limitation as ncdu.
- macOS users get a working `D` key by pointing the option at a trash CLI. This
  is a user-configured workaround, not autodetection — probing for `trash` or
  `gio` on macOS remains open.
- Windows gains nothing here. A Recycle Bin implementation needs
  `SHFileOperation`, not a command line.
