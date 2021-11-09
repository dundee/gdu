# go DiskUsage()

[![Build Status](https://travis-ci.com/dundee/gdu.svg?branch=master)](https://travis-ci.com/dundee/gdu)
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

Head for the [releases](https://github.com/dundee/gdu/releases) and download binary for your system.

Using curl:

    curl -L https://github.com/dundee/gdu/releases/latest/download/gdu_linux_amd64.tgz | tar xz
    chmod +x gdu_linux_amd64
    mv gdu_linux_amd64 /usr/bin/gdu

[Arch Linux](https://aur.archlinux.org/packages/gdu/):

    yay -S gdu

[Debian](https://packages.debian.org/bullseye/gdu):

    apt install gdu

[Ubuntu](https://launchpad.net/~daniel-milde/+archive/ubuntu/gdu)

    add-apt-repository ppa:daniel-milde/gdu
    apt-get update
    apt-get install gdu


[NixOS](https://search.nixos.org/packages?channel=unstable&show=gdu&query=gdu):

    nix-env -iA nixos.gdu

[Homebrew](https://formulae.brew.sh/formula/gdu):

    brew install -f gdu
    brew link --overwrite gdu  # if you have coreutils installed as well

[Snap](https://snapcraft.io/gdu-disk-usage-analyzer):

    snap install gdu-disk-usage-analyzer
    snap connect gdu-disk-usage-analyzer:mount-observe :mount-observe
    snap connect gdu-disk-usage-analyzer:system-backup :system-backup
    snap alias gdu-disk-usage-analyzer.gdu gdu

[Binenv](https://github.com/devops-works/binenv)

    binenv install gdu

[Go](https://pkg.go.dev/github.com/dundee/gdu):

    go install github.com/dundee/gdu/v5/cmd/gdu@latest

## Usage

```
  gdu [flags] [directory_to_scan]

Flags:
  -h, --help                          help for gdu
  -i, --ignore-dirs strings           Absolute paths to ignore (separated by comma) (default [/proc,/dev,/sys,/run])
  -I, --ignore-dirs-pattern strings   Absolute path patterns to ignore (separated by comma)
  -X, --ignore-from string            Read absolute path patterns to ignore from file
  -f, --input-file string             Import analysis from JSON file
  -l, --log-file string               Path to a logfile (default "/dev/null")
  -m, --max-cores int                 Set max cores that GDU will use. 8 cores available (default 8)
  -c, --no-color                      Do not use colorized output
  -x, --no-cross                      Do not cross filesystem boundaries
  -H, --no-hidden                     Ignore hidden directories (beginning with dot)
  -p, --no-progress                   Do not show progress in non-interactive mode
  -n, --non-interactive               Do not run in interactive mode
  -o, --output-file string            Export all info into file as JSON
  -a, --show-apparent-size            Show apparent size
  -d, --show-disks                    Show all mounted disks
  -s, --summarize                     Show only a total in non-interactive mode
  -v, --version                       Print version
```

## Examples

    gdu                                   # analyze current dir
    gdu -a                                # show apparent size instead of disk usage
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
    gdu / > file                          # write stats to file, do not start interactive mode

    gdu -o- / | gzip -c >report.json.gz   # write all info to JSON file for later analysis
    zcat report.json.gz | gdu -f-         # read analysis from file

## Modes

Gdu has three modes: interactive (default), non-interactive and export.

Non-interactive mode is started automtically when TTY is not detected (using [go-isatty](https://github.com/mattn/go-isatty)), for example if the output is being piped to a file, or it can be started explicitly by using a flag.

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

## Running tests

    make test

## Benchmarks

Benchmarks were performed on 50G directory (100k directories, 400k files) on 500 GB SSD using [hyperfine](https://github.com/sharkdp/hyperfine).
See `benchmark` target in [Makefile](Makefile) for more info.

### Cold cache

Filesystem cache was cleared using `sync; echo 3 | sudo tee /proc/sys/vm/drop_caches`.

| Command | Mean [s] | Min [s] | Max [s] | Relative |
|:---|---:|---:|---:|---:|
| `gdu -npc ~` | 5.377 ± 0.479 | 5.132 | 6.719 | 1.07 ± 0.10 |
| `dua ~` | 6.431 ± 0.015 | 6.417 | 6.465 | 1.28 ± 0.01 |
| `duc index ~` | 30.432 ± 2.965 | 29.321 | 38.866 | 6.07 ± 0.59 |
| `ncdu -0 -o /dev/null ~` | 29.435 ± 0.145 | 29.188 | 29.711 | 5.87 ± 0.06 |
| `diskus ~` | 5.013 ± 0.042 | 4.948 | 5.087 | 1.00 |
| `du -hs ~` | 29.445 ± 3.223 | 28.337 | 38.616 | 5.87 ± 0.64 |
| `dust -d0 ~` | 6.673 ± 0.483 | 6.337 | 7.788 | 1.33 ± 0.10 |

### Warm cache

| Command | Mean [ms] | Min [ms] | Max [ms] | Relative |
|:---|---:|---:|---:|---:|
| `gdu -npc ~` | 710.8 ± 11.0 | 692.8 | 730.7 | 1.67 ± 0.10 |
| `dua ~` | 532.0 ± 16.5 | 496.6 | 551.2 | 1.25 ± 0.08 |
| `duc index ~` | 1706.6 ± 23.2 | 1668.1 | 1729.3 | 4.01 ± 0.23 |
| `ncdu -0 -o /dev/null ~` | 2399.4 ± 12.5 | 2388.5 | 2425.2 | 5.64 ± 0.32 |
| `diskus ~` | 425.1 ± 23.7 | 393.3 | 469.7 | 1.00 |
| `du -hs ~` | 1397.0 ± 19.6 | 1378.2 | 1435.5 | 3.29 ± 0.19 |
| `dust -d0 ~` | 663.3 ± 9.8 | 645.0 | 679.5 | 1.56 ± 0.09 |

## Alternatives

* [ncdu](https://dev.yorhel.nl/ncdu) - NCurses based tool written in pure C
* [godu](https://github.com/viktomas/godu) - Analyzer with carousel like user interface
* [dua](https://github.com/Byron/dua-cli) - Tool written in Rust with interface similar to gdu (and ncdu)
* [diskus](https://github.com/sharkdp/diskus) - Very simple but very fast tool written in Rust
* [duc](https://duc.zevv.nl/) - Collection of tools with many possibilities for inspecting and visualising disk usage
* [dust](https://github.com/bootandy/dust) - Tool written in Rust showing tree like structures of disk usage
