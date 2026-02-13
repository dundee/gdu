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
  gdu [directory_to_scan] [flags]

Flags:
      --archive-browsing              Enable browsing of zip/jar archives
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
    gdu <some_dir_to_analyze>             # analyze given dir
    gdu -d                                # show all mounted disks
    gdu -l ./gdu.log <some_dir>           # write errors to log file
    gdu -i /sys,/proc /                   # ignore some paths
    gdu -I '.*[abc]+'                     # ignore paths by regular pattern
    gdu -X ignore_file /                  # ignore paths by regular patterns from file
    gdu -c /                              # use only white/gray/black colors

    gdu -n /                              # only print stats, do not start interactive mode
    gdu -p /                              # do not show progress, useful when using its output in a script
    gdu -ps /some/dir                     # show only total usage for given dir
    gdu -t 10 /                           # show top 10 largest files
    gdu --reverse-sort -n /               # show files sorted from smallest to largest in non-interactive mode
    gdu / > file                          # write stats to file, do not start interactive mode

    gdu -o- / | gzip -c >report.json.gz   # write all info to JSON file for later analysis
    zcat report.json.gz | gdu -f-         # read analysis from file

    GOGC=10 gdu -g --use-storage /        # use persistent key-value storage for saving analysis data
    gdu -r /                              # read saved analysis data from persistent key-value storage

## Modes

Gdu has three modes: interactive (default), non-interactive and export.

Non-interactive mode is started automatically when TTY is not detected (using [go-isatty](https://github.com/mattn/go-isatty)), for example if the output is being piped to a file, or it can be started explicitly by using a flag.

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

Benchmarks were performed on 50G directory (100k directories, 400k files) on 500 GB SSD using [hyperfine](https://github.com/sharkdp/hyperfine).
See `benchmark` target in [Makefile](Makefile) for more info.

### Cold cache

Filesystem cache was cleared using `sync; echo 3 | sudo tee /proc/sys/vm/drop_caches`.

| Command | Mean [s] | Min [s] | Max [s] | Relative |
|:---|---:|---:|---:|---:|
| `diskus ~` | 3.074 ± 0.010 | 3.056 | 3.094 | 1.00 |
| `gdu -npc ~` | 3.133 ± 0.013 | 3.116 | 3.159 | 1.02 ± 0.01 |
| `gdu -gnpc ~` | 3.157 ± 0.013 | 3.139 | 3.180 | 1.03 ± 0.01 |
| `pdu ~` | 3.772 ± 0.149 | 3.630 | 4.071 | 1.23 ± 0.05 |
| `dust -d0 ~` | 4.001 ± 0.162 | 3.786 | 4.305 | 1.30 ± 0.05 |
| `dua ~` | 5.315 ± 3.210 | 4.068 | 14.447 | 1.73 ± 1.04 |
| `gdu -npc --use-storage ~` | 12.690 ± 0.527 | 11.325 | 13.091 | 4.13 ± 0.17 |
| `du -hs ~` | 14.940 ± 0.064 | 14.852 | 15.048 | 4.86 ± 0.03 |
| `duc index ~` | 15.501 ± 0.136 | 15.386 | 15.849 | 5.04 ± 0.05 |
| `ncdu -0 -o /dev/null ~` | 15.688 ± 0.053 | 15.610 | 15.789 | 5.10 ± 0.02 |

### Warm cache

| Command | Mean [ms] | Min [ms] | Max [ms] | Relative |
|:---|---:|---:|---:|---:|
| `diskus ~` | 211.4 ± 3.7 | 206.4 | 219.3 | 1.00 |
| `pdu ~` | 221.8 ± 2.4 | 219.3 | 226.3 | 1.05 ± 0.02 |
| `dust -d0 ~` | 363.6 ± 5.4 | 357.3 | 373.2 | 1.72 ± 0.04 |
| `gdu -npc ~` | 434.3 ± 3.4 | 426.0 | 437.8 | 2.05 ± 0.04 |
| `dua ~` | 451.2 ± 4.2 | 444.9 | 457.9 | 2.13 ± 0.04 |
| `gdu -gnpc ~` | 521.0 ± 14.0 | 510.9 | 558.5 | 2.46 ± 0.08 |
| `du -hs ~` | 809.4 ± 3.2 | 804.8 | 816.0 | 3.83 ± 0.07 |
| `duc index ~` | 952.3 ± 4.8 | 946.0 | 961.7 | 4.50 ± 0.08 |
| `ncdu -0 -o /dev/null ~` | 1432.8 ± 3.4 | 1428.0 | 1439.0 | 6.78 ± 0.12 |
| `gdu -npc --use-storage ~` | 9950.0 ± 474.1 | 9117.5 | 10647.4 | 47.07 ± 2.39 |

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
