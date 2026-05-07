# go DiskUsage()

<img src="./gdu.png" alt="Gdu " width="200" align="right">

[![Codecov](https://codecov.io/gh/dundee/gdu/branch/master/graph/badge.svg)](https://codecov.io/gh/dundee/gdu)
[![Go Report Card](https://goreportcard.com/badge/github.com/dundee/gdu)](https://goreportcard.com/report/github.com/dundee/gdu)
[![Maintainability](https://api.codeclimate.com/v1/badges/30d793274607f599e658/maintainability)](https://codeclimate.com/github/dundee/gdu/maintainability)
[![CodeScene Code Health](https://codescene.io/projects/13129/status-badges/code-health)](https://codescene.io/projects/13129)

Pretty fast disk usage analyzer written in Go.

Gdu is intended primarily for SSD disks where it can fully utilize parallel processing.
However HDDs work as well, but the performance gain is not so huge.

[![asciicast](https://asciinema.org/a/382738.svg)](https://asciinema.org/a/382738)

<a href="https://repology.org/project/gdu/versions">
    <img src="https://repology.org/badge/vertical-allrepos/gdu.svg" alt="Packaging status" align="right">
</a>

## Installation

Head for the [releases page](https://github.com/dundee/gdu/releases) and download the binary for your system.

Using curl:

    curl -L https://github.com/dundee/gdu/releases/latest/download/gdu_linux_amd64.tgz | tar xz
    chmod +x gdu_linux_amd64
    mv gdu_linux_amd64 /usr/bin/gdu

See the [installation page](./INSTALL.md) for other ways how to install Gdu to your system.

Or you can use Gdu directly via Docker:

    docker run --rm --init --interactive --tty --privileged --volume /:/mnt/root ghcr.io/dundee/gdu /mnt/root

## Usage

```
  gdu [flags] [directory_to_scan]

Flags:
      --archive-browsing              Enable browsing of zip/jar/tar archives (tar, tar.gz, tar.bz2, tar.xz)
      --collapse-path                 Collapse single-child directory chains
      --config-file string            Read config from file (default is $HOME/.gdu.yaml)
  -D, --db string                     Store analysis in database (*.sqlite for SQLite, *.badger for BadgerDB)
      --depth int                     Show directory structure up to specified depth in non-interactive mode (0 means the flag is ignored)
      --enable-profiling              Enable collection of profiling data and provide it on http://localhost:6060/debug/pprof/
  -E, --exclude-type strings          File types to exclude (e.g., --exclude-type yaml,json)
  -L, --follow-symlinks               Follow symlinks for files, i.e. show the size of the file to which symlink points to (symlinks to directories are not followed)
  -h, --help                          help for gdu
  -i, --ignore-dirs strings           Paths to ignore (separated by comma). Can be absolute or relative to current directory (default [/proc,/dev,/sys,/run])
  -I, --ignore-dirs-pattern strings   Path patterns to ignore (separated by comma)
  -X, --ignore-from string            Read path patterns to ignore from file
  -f, --input-file string             Import analysis from JSON file
      --interactive                   Force interactive mode even when output is not a TTY
  -l, --log-file string               Path to a logfile (default "/dev/null")
      --max-age string                Include files with mtime no older than DURATION (e.g., 7d, 2h30m, 1y2mo)
  -m, --max-cores int                 Set max cores that Gdu will use. 8 cores available (default 8)
      --min-age string                Include files with mtime at least DURATION old (e.g., 30d, 1w)
      --mouse                         Use mouse
  -c, --no-color                      Do not use colorized output
  -x, --no-cross                      Do not cross filesystem boundaries
      --no-delete                     Do not allow deletions
  -H, --no-hidden                     Ignore hidden directories (beginning with dot)
      --no-prefix                     Show sizes as raw numbers without any prefixes (SI or binary) in non-interactive mode
  -p, --no-progress                   Do not show progress in non-interactive mode
      --no-spawn-shell                Do not allow spawning shell
  -u, --no-unicode                    Do not use Unicode symbols (for size bar)
      --no-view-file                  Do not allow viewing file contents
  -n, --non-interactive               Do not run in interactive mode
  -o, --output-file string            Export all info into file as JSON
  -r, --read-from-storage             Use existing database instead of re-scanning
      --reverse-sort                  Reverse sorting order (smallest to largest) in non-interactive mode
      --sequential                    Use sequential scanning (intended for rotating HDDs)
  -A, --show-annexed-size             Use apparent size of git-annex'ed files in case files are not present locally (real usage is zero)
  -a, --show-apparent-size            Show apparent size
  -d, --show-disks                    Show all mounted disks
  -k, --show-in-kib                   Show sizes in KiB (or kB with --si) in non-interactive mode
  -C, --show-item-count               Show number of items in directory
  -M, --show-mtime                    Show latest mtime of items in directory
  -B, --show-relative-size            Show relative size
      --si                            Show sizes with decimal SI prefixes (kB, MB, GB) instead of binary prefixes (KiB, MiB, GiB)
      --since string                  Include files with mtime >= WHEN. WHEN accepts RFC3339 timestamp (e.g., 2025-08-11T01:00:00-07:00) or date only YYYY-MM-DD (calendar-day compare; includes the whole day)
  -s, --summarize                     Show only a total in non-interactive mode
  -t, --top int                       Show only top X largest files in non-interactive mode
  -T, --type strings                  File types to include (e.g., --type yaml,json)
      --until string                  Include files with mtime <= WHEN. WHEN accepts RFC3339 timestamp or date only YYYY-MM-DD
  -v, --version                       Print version
      --write-config                  Write current configuration to file (default is $HOME/.gdu.yaml)

Basic list of actions in interactive mode (show help modal for more):
  ↑ or k                              Move cursor up
  ↓ or j                              Move cursor down
  → or Enter or l                     Go to highlighted directory
  ← or h                              Go to parent directory
  d                                   Delete the selected file or directory
  e                                   Empty the selected directory
  n                                   Sort by name
  s                                   Sort by size
  c                                   Show number of items in directory
  ?                                   Show help modal
```

## Examples

    gdu                                   # analyze current dir
    gdu -a                                # show apparent size instead of disk usage
    gdu --no-delete                       # prevent write operations
    gdu --no-view-file                    # prevent viewing file contents
    gdu <some_dir_to_analyze>             # analyze given dir
    gdu -d                                # show all mounted disks
    gdu -l ./gdu.log <some_dir>           # write errors to log file
    gdu -i /sys,/proc /                   # ignore some paths
    gdu -I '.*[abc]+'                     # ignore paths by regular pattern
    gdu -X ignore_file /                  # ignore paths by regular patterns from file
    gdu -c /                              # use only white/gray/black colors

    gdu -n /                              # only print stats, do not start interactive mode
    gdu --interactive / | tee out.txt     # force interactive mode even when stdout is piped
    gdu -p /                              # do not show progress, useful when using its output in a script
    gdu -ps /some/dir                     # show only total usage for given dir
    gdu -t 10 /                           # show top 10 largest files
    gdu --reverse-sort -n /               # show files sorted from smallest to largest in non-interactive mode
    gdu / > file                          # write stats to file, do not start interactive mode

    gdu -o- / | gzip -c >report.json.gz   # write all info to JSON file for later analysis
    zcat report.json.gz | gdu -f-         # read analysis from file

    gdu --db=tmp.badger /                 # use persistent key-value storage for saving analysis data
    gdu --db=tmp.db /                     # use persistent SQLite storage for saving analysis data
    gdu -r /                              # read saved analysis data from persistent key-value storage

## Modes

Gdu has three modes: interactive (default), non-interactive and export.

Non-interactive mode is started automatically when TTY is not detected (using [go-isatty](https://github.com/mattn/go-isatty)), for example if the output is being piped to a file, or it can be started explicitly by using a flag. Use `--interactive` to disable this automatic fallback and force interactive mode.

In non-interactive mode (and without `--top` and `--depth` flags), gdu uses a memory-efficient analyzer that only tracks top-level directory totals.
This means memory usage stays constant regardless of how large the scanned directory tree is.
When `--top` or `--depth` flags are used, the full directory tree is built in memory as in interactive mode.

Export mode (flag `-o`) outputs all usage data as JSON, which can be later opened using the `-f` flag.

Hard links are counted only once.

## File flags

Files and directories may be prefixed by a one-character
flag with following meaning:

* `!` An error occurred while reading this directory.

* `.` An error occurred while reading a subdirectory, size may be not correct.

* `@` File is symlink or socket.

* `H` Same file was already counted (hard link).

* `e` Directory is empty.

## Configuration file

Gdu can read (and write) YAML configuration file.

`$HOME/.config/gdu/gdu.yaml` and `$HOME/.gdu.yaml` are checked for the presence of the config file by default.

See the [full list of all configuration options](configuration.md).

### Examples

* To configure gdu to permanently run in gray-scale color mode:

```
echo "no-color: true" >> ~/.gdu.yaml
```

* To set default sorting in configuration file:

```
sorting:
    by: name // size, name, itemCount, mtime
    order: desc
```

* To configure gdu to set CWD variable when browsing directories:

```
echo "change-cwd: true" >> ~/.gdu.yaml
```

* To save the current configuration

```
gdu --write-config
```

## Styling

There are wide options for how terminals can be colored.
Some gdu primitives (like basic text) adapt to different color schemas, but the selected/highlighted row does not.

If the default look is not sufficient, it can be changed in configuration file, e.g.:

```
style:
    selected-row:
        text-color: black
        background-color: "#ff0000"
    marked:
        text-color: white
        background-color: "#6600cc"
```

## Deletion in background and in parallel (experimental)

Gdu can delete items in the background, thus not blocking the UI for additional work.
To enable:

```
echo "delete-in-background: true" >> ~/.gdu.yaml
```

Directory items can be also deleted in parallel, which might increase the speed of deletion.
To enable:

```
echo "delete-in-parallel: true" >> ~/.gdu.yaml
```

## Saving analysis data to database

Gdu can store the analysis data to a database file instead of just memory.
This allows you to save and reload analysis results later.
Both SQLite and BadgerDB are supported.

```
gdu --db analysis.sqlite /        # saves analysis data to SQLite database
gdu --db analysis.badger /        # saves analysis data to BadgerDB
gdu -r --db analysis.sqlite /     # reads saved data, does not run analysis again
```

## Running tests

    make install-dev-dependencies
    make test

## Profiling

Gdu can collect profiling data when the `--enable-profiling` flag is set.
The data are provided via embedded http server on URL `http://localhost:6060/debug/pprof/`.

You can then use e.g. `go tool pprof -web http://localhost:6060/debug/pprof/heap`
to open the heap profile as SVG image in your web browser.

## Benchmarks

Benchmarks were performed on 90G directory (100k directories, 400k files) on 500 GB SSD using [hyperfine](https://github.com/sharkdp/hyperfine).
See `benchmark` target in [Makefile](Makefile) for more info.

### Cold cache

Filesystem cache was cleared using `sync; echo 3 | sudo tee /proc/sys/vm/drop_caches`.

| Command | Mean [s] | Min [s] | Max [s] | Relative |
|:---|---:|---:|---:|---:|
| `diskus ~` | 4.489 ± 0.020 | 4.449 | 4.516 | 1.00 |
| `gdu -npc ~` | 4.716 ± 0.342 | 4.109 | 5.337 | 1.05 ± 0.08 |
| `GOMAXPROCS=80 gdu -npc ~` | 4.901 ± 1.953 | 3.627 | 9.993 | 1.09 ± 0.44 |
| `pdu ~` | 5.969 ± 0.492 | 5.567 | 6.640 | 1.33 ± 0.11 |
| `dua ~` | 6.030 ± 0.249 | 5.878 | 6.597 | 1.34 ± 0.06 |
| `dust -d0 ~` | 6.181 ± 0.311 | 6.043 | 7.053 | 1.38 ± 0.07 |
| `gdu -npc --db=tmp.badger ~` | 27.479 ± 3.015 | 25.048 | 32.777 | 6.12 ± 0.67 |
| `du -hs ~` | 30.608 ± 0.221 | 30.136 | 30.794 | 6.82 ± 0.06 |
| `duc index ~` | 32.897 ± 3.168 | 31.524 | 41.865 | 7.33 ± 0.71 |
| `ncdu -0 -o /dev/null ~` | 33.163 ± 3.482 | 31.476 | 42.979 | 7.39 ± 0.78 |
| `gdu -npc --db=tmp.db ~` | 44.989 ± 0.270 | 44.622 | 45.414 | 10.02 ± 0.07 |

### Warm cache

| Command | Mean [ms] | Min [ms] | Max [ms] | Relative |
|:---|---:|---:|---:|---:|
| `diskus ~` | 270.8 ± 8.1 | 262.4 | 291.5 | 1.00 |
| `pdu ~` | 299.1 ± 4.1 | 292.1 | 305.0 | 1.10 ± 0.04 |
| `GOMAXPROCS=100 gdu -npc ~` | 459.1 ± 14.2 | 446.7 | 490.3 | 1.69 ± 0.07 |
| `gdu -npc ~` | 466.1 ± 27.9 | 421.4 | 495.3 | 1.72 ± 0.12 |
| `dua ~` | 590.6 ± 5.9 | 580.5 | 599.7 | 2.18 ± 0.07 |
| `dust -d0 ~` | 578.7 ± 3.7 | 572.2 | 586.3 | 2.14 ± 0.07 |
| `du -hs ~` | 1255.2 ± 7.4 | 1245.1 | 1273.4 | 4.63 ± 0.14 |
| `duc index ~` | 1450.5 ± 6.2 | 1440.6 | 1460.4 | 5.36 ± 0.16 |
| `ncdu -0 -o /dev/null ~` | 2222.4 ± 5.6 | 2215.6 | 2231.0 | 8.21 ± 0.25 |
| `gdu -npc --db=tmp.db ~` | 8246.7 ± 30.9 | 8181.9 | 8288.7 | 30.45 ± 0.92 |
| `gdu -npc --db=tmp.badger ~` | 15608.0 ± 3215.8 | 13960.3 | 22448.0 | 57.63 ± 12.00 |

## Alternatives

* [ncdu](https://dev.yorhel.nl/ncdu) - NCurses based tool written in pure `C` (LTS) or `zig` (Stable)
* [godu](https://github.com/viktomas/godu) - Analyzer with a carousel like user interface
* [dua](https://github.com/Byron/dua-cli) - Tool written in `Rust` with interface similar to gdu (and ncdu)
* [diskus](https://github.com/sharkdp/diskus) - Very simple but very fast tool written in `Rust`
* [duc](https://duc.zevv.nl/) - Collection of tools with many possibilities for inspecting and visualising disk usage
* [dust](https://github.com/bootandy/dust) - Tool written in `Rust` showing tree like structures of disk usage
* [pdu](https://github.com/KSXGitHub/parallel-disk-usage) - Tool written in `Rust` showing tree like structures of disk usage

## Notes

[HDD icon created by Nikita Golubev - Flaticon](https://www.flaticon.com/free-icons/hdd)
