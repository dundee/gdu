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
      --config-file string            Read config from file (default is $HOME/.gdu.yaml)
  -g, --const-gc                      Enable memory garbage collection during analysis with constant level set by GOGC
      --enable-profiling              Enable collection of profiling data and provide it on http://localhost:6060/debug/pprof/
  -L, --follow-symlinks               Follow symlinks for files, i.e. show the size of the file to which symlink points to (symlinks to directories are not followed)
  -h, --help                          help for gdu
  -i, --ignore-dirs strings           Paths to ignore (separated by comma). Can be absolute or relative to current directory (default [/proc,/dev,/sys,/run])
  -I, --ignore-dirs-pattern strings   Path patterns to ignore (separated by comma)
  -X, --ignore-from string            Read path patterns to ignore from file
  -f, --input-file string             Import analysis from JSON file
  -l, --log-file string               Path to a logfile (default "/dev/null")
  -m, --max-cores int                 Set max cores that Gdu will use. 12 cores available (default 12)
  -c, --no-color                      Do not use colorized output
  -x, --no-cross                      Do not cross filesystem boundaries
      --no-delete                     Do not allow deletions
  -H, --no-hidden                     Ignore hidden directories (beginning with dot)
      --no-mouse                      Do not use mouse
      --no-prefix                     Show sizes as raw numbers without any prefixes (SI or binary) in non-interactive mode
  -p, --no-progress                   Do not show progress in non-interactive mode
  -u, --no-unicode                    Do not use Unicode symbols (for size bar)
  -n, --non-interactive               Do not run in interactive mode
  -o, --output-file string            Export all info into file as JSON
  -r, --read-from-storage             Read analysis data from persistent key-value storage
      --sequential                    Use sequential scanning (intended for rotating HDDs)
  -a, --show-apparent-size            Show apparent size
  -d, --show-disks                    Show all mounted disks
  -C, --show-item-count               Show number of items in directory
  -M, --show-mtime                    Show latest mtime of items in directory
  -B, --show-relative-size            Show relative size
      --si                            Show sizes with decimal SI prefixes (kB, MB, GB) instead of binary prefixes (KiB, MiB, GiB)
      --storage-path string           Path to persistent key-value storage directory (default "/tmp/badger")
  -s, --summarize                     Show only a total in non-interactive mode
  -t, --top int                       Show only top X largest files in non-interactive mode
      --use-storage                   Use persistent key-value storage for analysis data (experimental)
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
    gdu -np /                             # do not show progress, useful when using its output in a script
    gdu -nps /some/dir                    # show only total usage for given dir
    gdu -nt 10 /                          # show top 10 largest files
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

See the [full list of all configuration options](configuration).

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

## Memory usage

### Automatic balancing

Gdu tries to balance performance and memory usage.

When less memory is used by gdu than the total free memory of the host,
then Garbage Collection is disabled during the analysis phase completely to gain maximum speed.

Otherwise GC is enabled.
The more memory is used and the less memory is free, the more often will the GC happen.

### Manual memory usage control

If you want manual control over Garbage Collection, you can use `--const-gc` / `-g` flag.
It will run Garbage Collection during the analysis phase with constant level of aggressiveness.
As a result, the analysis will be about 25% slower and will consume about 30% less memory.
To change the level, you can set the `GOGC` environment variable to specify how often the garbage collection will happen.
Lower value (than 100) means GC will run more often. Higher means less often. Negative number will stop GC.

Example running gdu with constant GC, but not so aggressive as default:

```
GOGC=200 gdu -g /
```

## Saving analysis data to persistent key-value storage (experimental)

Gdu can store the analysis data to persistent key-value storage instead of just memory.
Gdu will run much slower (approx 10x) but it should use much less memory (when using small GOGC as well).
Gdu can also reopen with the saved data.
Currently only BadgerDB is supported as the key-value storage (embedded).

```
GOGC=10 gdu -g --use-storage /    # saves analysis data to key-value storage
gdu -r /                          # reads just saved data, does not run analysis again
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
| `diskus ~` | 3.126 ± 0.020 | 3.087 | 3.155 | 1.00 |
| `gdu -npc ~` | 3.132 ± 0.019 | 3.111 | 3.173 | 1.00 ± 0.01 |
| `gdu -gnpc ~` | 3.136 ± 0.012 | 3.112 | 3.155 | 1.00 ± 0.01 |
| `pdu ~` | 3.657 ± 0.013 | 3.641 | 3.677 | 1.17 ± 0.01 |
| `dust -d0 ~` | 3.933 ± 0.144 | 3.849 | 4.213 | 1.26 ± 0.05 |
| `dua ~` | 3.994 ± 0.073 | 3.827 | 4.134 | 1.28 ± 0.02 |
| `gdu -npc --use-storage ~` | 12.812 ± 0.078 | 12.644 | 12.912 | 4.10 ± 0.04 |
| `du -hs ~` | 14.120 ± 0.213 | 13.969 | 14.703 | 4.52 ± 0.07 |
| `duc index ~` | 14.567 ± 0.080 | 14.385 | 14.657 | 4.66 ± 0.04 |
| `ncdu -0 -o /dev/null ~` | 14.963 ± 0.254 | 14.759 | 15.637 | 4.79 ± 0.09 |

### Warm cache

| Command | Mean [ms] | Min [ms] | Max [ms] | Relative |
|:---|---:|---:|---:|---:|
| `pdu ~` | 226.6 ± 3.7 | 219.6 | 231.2 | 1.00 |
| `diskus ~` | 227.7 ± 5.2 | 221.6 | 239.9 | 1.00 ± 0.03 |
| `dust -d0 ~` | 400.1 ± 7.1 | 386.7 | 409.4 | 1.77 ± 0.04 |
| `dua ~` | 444.9 ± 2.4 | 442.4 | 448.9 | 1.96 ± 0.03 |
| `gdu -npc ~` | 451.3 ± 3.8 | 445.9 | 458.5 | 1.99 ± 0.04 |
| `gdu -gnpc ~` | 516.1 ± 6.7 | 503.1 | 527.5 | 2.28 ± 0.05 |
| `du -hs ~` | 905.0 ± 3.9 | 901.2 | 913.4 | 3.99 ± 0.07 |
| `duc index ~` | 1053.0 ± 5.1 | 1046.2 | 1064.1 | 4.65 ± 0.08 |
| `ncdu -0 -o /dev/null ~` | 1653.9 ± 5.7 | 1645.9 | 1663.0 | 7.30 ± 0.12 |
| `gdu -npc --use-storage ~` | 9754.9 ± 688.7 | 8403.8 | 10427.4 | 43.04 ± 3.12 |

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