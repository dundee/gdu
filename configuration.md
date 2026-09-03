# YAML file configuration options

Gdu provides an additional set of configuration options to the usual command line options.

You can get the full list of all possible options by running:

```
gdu --write-config
```

This will create file `$HOME/.gdu.yaml` with all the options set to default values.

Let's go through them one by one:

#### `log-file`

Path to a logfile (default "/dev/null")

#### `input-file`

Import analysis from JSON file

#### `output-file`

Export all info into file as JSON

#### `ignore-dirs`

Paths to ignore (separated by comma). Can be absolute (like `/proc`) or relative to the current working directory (like `node_modules`). Default values are [/proc,/dev,/sys,/run].

#### `ignore-dir-patterns`

Path patterns to ignore (separated by comma). Patterns can be absolute or relative to the current working directory.

#### `ignore-from-file`

Read path patterns to ignore from file. Patterns can be absolute or relative to the current working directory.

#### `max-cores`

Set max cores that Gdu will use.

#### `sequential-scanning`

Use sequential scanning (intended for rotating HDDs)

#### `show-apparent-size`

Show apparent size

#### `show-relative-size`

Show relative size

#### `show-item-count`

Show the item count column. For a directory this is the number of items
(files and subdirectories, recursively) it contains, not counting the
directory itself; for a file it is 1.

#### `show-symlink-target`

Show symlink target (`name -> target`) in the file list. Disabled by default.

#### `no-color`

Do not use colorized output

#### `mouse`

Use mouse

#### `non-interactive`

Do not run in interactive mode

#### `interactive`

Force interactive mode even when output is not a TTY

#### `no-progress`

Do not show progress in non-interactive mode

#### `no-cross`

Do not cross filesystem boundaries

#### `no-hidden`

Ignore hidden directories (beginning with dot)

#### `no-delete`

Do not allow deletions

#### `no-view-file`

Do not allow viewing file contents

#### `no-confirm-quit`

Do not ask for confirmation before quitting after a long scan. By default, pressing `q`/`Q` after a scan that took more than a few seconds shows a confirmation dialog so that results are not lost by an accidental key press.

#### `follow-symlinks`

Follow symlinks for files, i.e. show the size of the file to which symlink points to (symlinks to directories are not followed)

#### `profiling`

Enable collection of profiling data and provide it on http://localhost:6060/debug/pprof/

#### `read-from-storage`

Read analysis data from persistent key-value storage

#### `summarize`

Show only a total in non-interactive mode

#### `use-si-prefix`

Show sizes with decimal SI prefixes (kB, MB, GB) instead of binary prefixes (KiB, MiB, GiB)

#### `no-prefix`

Show sizes as raw numbers without any prefixes (SI or binary) in non-interactive mode

#### `reverse-sort`

Reverse sorting order (smallest to largest) in non-interactive mode

#### `change-cwd`

Set CWD variable when browsing directories

#### `delete-in-background`

Delete items in the background, not blocking the UI from work

#### `delete-in-parallel`

Delete items in parallel, which might increase the speed of deletion

#### `trash-command`

Command used by the `D` key to move items to trash, replacing the built-in [FreeDesktop trash](https://specifications.freedesktop.org/trash-spec/latest/) implementation. Useful to trash into a non-default location, or to use a trash tool on systems where the built-in trash is not available (macOS).

The command is evaluated by `/bin/sh` with the absolute path of the item appended as an argument, so shell syntax such as variable expansion works:

```yaml
trash-command: trash-put --trash-dir ~/mytrash
```

##### Using `mv`

Commands taking their destination last would receive the item path in the wrong place. Refer to the path explicitly as `"$1"` and it is not appended:

```yaml
trash-command: mv -f "$1" ~/mytrash/
```

Note the trailing slash on the destination: without it, `mv` renames the item to `~/mytrash` when that directory does not exist, quietly replacing whatever was there. With it, a missing trash dir becomes an error dialog instead.

Unlike a real trash tool, `mv` overwrites an item of the same name that was trashed earlier, and records nothing that would allow restoring the item to its original location.

Further details:

* The path is passed as a positional shell parameter, so item names are never interpreted as shell syntax.
* The path is appended to the command unless the command refers to it itself via `$1`, `$@` or `$*`. An appended path needs the command to end where an argument can follow (`gio trash --` works, `some-command;` does not); otherwise use `"$1"`.
* The absolute path is also exported as the `GDU_TRASH_PATH` environment variable.
* The command must not be interactive. It gets no terminal, the UI keeps running while it executes, and with `delete-in-background` it may even run from a background worker.
* When the command fails, its exit status and standard error output are shown in an error dialog and the item stays in the listing.
* When the command succeeds but the item is still present on disk, the item stays in the listing as well. Only the selected item is refreshed, so an item moved elsewhere by the command is not picked up at its new location.
* `no-delete` disables the `D` key regardless of this option.
* Not supported on Windows, which has no POSIX shell to evaluate the command in.

#### `browse-parent-dirs`

Allow navigating above the launch directory by pressing the left arrow key. When enabled, pressing left at the top-level directory will rescan and open its parent directory. Disabled by default.


#### `web.listen`

Address the web UI (`--web`) listens on, e.g. `localhost:8080`. When empty (the default), Gdu binds to `localhost` on a random free port. Binding to a non-loopback address exposes file names and sizes to other hosts on the network and prints a warning.

#### `web.open-browser`

Whether the web UI opens in the default browser on start. Enabled by default. The URL is always printed regardless of this setting. Can be overridden on the command line with `--web-open=false`.

#### `web.browser`

Override the command used to open the browser (the URL is appended as the final argument), e.g. `firefox --new-window`. When empty (the default), the operating system's default handler is used.


#### `style.selected-row.text-color`

Color of text for the selected row

#### `style.selected-row.background-color`

Background color for the selected row

#### `style.marked.text-color`

Color of text for marked items

#### `style.marked.background-color`

Background color for marked items

#### `style.progress-modal.current-item-path-max-len`

Maximum length of file path for the current item in progress bar.
When the length is reached, the path is shortened with "/.../".

#### `style.use-old-size-bar`

Show size bar without Unicode symbols.

#### `style.show-bar-percentage`

Show the numeric usage percentage (e.g. `61.4%`) next to the size bar in the directory listing.

#### `style.show-item-count-bar`

Show a bar next to the item count, drawn in proportion to the item counts of
the other items in the same directory, in the same way the size bar works for
sizes. Disabled by default.

The bar is only drawn while the item count column itself is visible, so enable
it together with `show-item-count` or toggle the column with the `c` key. Like
the size bar, it is measured against the sum of the items in the listing, or
against the largest one when `show-relative-size` is set.

#### `style.footer.text-color`

Color of text for footer bar

#### `style.footer.background-color`

Background color for footer bar

#### `style.footer.number-color`

Color of numbers displayed in the footer

#### `style.header.text-color`

Color of text for header bar

#### `style.header.background-color`

Background color for header bar

#### `style.header.hidden`

Hide the header bar

#### `style.result-row.number-color`

Color of numbers in result rows

#### `style.result-row.directory-color`

Color of directory names in result rows

#### `sorting.by`

Sort items. Possible values:
* name - name of the item
* size - usage or apparent size
* itemCount - number of items contained in the folder tree (the same number shown by `show-item-count`)
* mtime - modification time

#### `sorting.order`

Set sorting order. Possible values:
* asc - ascending order
* desc - descending order
