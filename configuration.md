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

Show number of items in directory

#### `no-color`

Do not use colorized output

#### `no-mouse`

Do not use mouse

#### `non-interactive`

Do not run in interactive mode

#### `no-progress`

Do not show progress in non-interactive mode

#### `no-cross`

Do not cross filesystem boundaries

#### `no-hidden`

Ignore hidden directories (beginning with dot)

#### `no-delete`

Do not allow deletions

#### `follow-symlinks`

Follow symlinks for files, i.e. show the size of the file to which symlink points to (symlinks to directories are not followed)

#### `profiling`

Enable collection of profiling data and provide it on http://localhost:6060/debug/pprof/
#### `const-gc`

Enable memory garbage collection during analysis with constant level set by GOGC

#### `use-storage`

Use persistent key-value storage for analysis data (experimental)

#### `storage-path`

Path to persistent key-value storage directory (default is /tmp/badger)

#### `read-from-storage`

Read analysis data from persistent key-value storage

#### `summarize`

Show only a total in non-interactive mode

#### `use-si-prefix`

Show sizes with decimal SI prefixes (kB, MB, GB) instead of binary prefixes (KiB, MiB, GiB)

#### `no-prefix`

Show sizes as raw numbers without any prefixes (SI or binary) in non-interactive mode

#### `change-cwd`

Set CWD variable when browsing directories

#### `delete-in-background`

Delete items in the background, not blocking the UI from work

#### `delete-in-parallel`

Delete items in parallel, which might increase the speed of deletion

#### `style.selected-row.text-color`

Color of text for the selected row

#### `style.selected-row.background-color`

Background color for the selected row

#### `style.progress-modal.current-item-path-max-len`

Maximum length of file path for the current item in progress bar.
When the length is reached, the path is shortened with "/.../".

#### `style.use-old-size-bar`

Show size bar without Unicode symbols.

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
* itemCount - number of items in the folder tree
* mtime - modification time

#### `sorting.order`

Set sorting order. Possible values:
* asc - ascending order
* desc - descending order