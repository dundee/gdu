# go DiskUsage()

[![Build Status](https://travis-ci.com/dundee/gdu.svg?branch=master)](https://travis-ci.com/dundee/gdu)
[![Coverage Status](https://coveralls.io/repos/github/dundee/gdu/badge.svg?branch=master)](https://coveralls.io/github/dundee/gdu?branch=master)
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

[Debian](https://packages.debian.org/sid/gdu):

    dpkg -i gdu_*_amd64.deb

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

    go get -u github.com/dundee/gdu/v5/cmd/gdu


## Usage

```
  gdu [flags] [directory_to_scan]

Flags:
  -h, --help                          help for gdu
  -i, --ignore-dirs strings           Absolute paths to ignore (separated by comma) (default [/proc,/dev,/sys,/run])
  -I, --ignore-dirs-pattern strings   Absolute path patterns to ignore (separated by comma)
  -l, --log-file string               Path to a logfile (default "/dev/null")
  -m, --max-cores int                 Set max cores that GDU will use. 8 cores available (default 8)
  -c, --no-color                      Do not use colorized output
  -x, --no-cross                      Do not cross filesystem boundaries
  -H, --no-hidden                     Ignore hidden directories (beggining with dot)
  -p, --no-progress                   Do not show progress in non-interactive mode
  -n, --non-interactive               Do not run in interactive mode
  -a, --show-apparent-size            Show apparent size
  -d, --show-disks                    Show all mounted disks
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
    gdu -c /                              # use only white/gray/black colors

    gdu -n /                              # only print stats, do not start interactive mode
    gdu -np /                             # do not show progress, useful when using its output in a script
    gdu / > file                          # write stats to file, do not start interactive mode

Gdu has two modes: interactive (default) and non-interactive.

Non-interactive mode is started automtically when TTY is not detected (using [go-isatty](https://github.com/mattn/go-isatty)), for example if the output is being piped to a file, or it can be started explicitly by using a flag.

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

Benchmarks performed on 50G directory (100k directories, 400k files) on 500 GB SSD using [hyperfine](https://github.com/sharkdp/hyperfine).
See `benchmark` target in [Makefile](Makefile) for more info.

### Cold cache

Filesystem cache was cleared using `sync; echo 3 | sudo tee /proc/sys/vm/drop_caches`.

| Command | Mean [s] | Min [s] | Max [s] | Relative |
|:---|---:|---:|---:|---:|
| `gdu -npc ~` | 3.714 ± 0.036 | 3.685 | 3.809 | 1.00 |
| `dua ~` | 4.703 ± 0.011 | 4.691 | 4.721 | 1.27 ± 0.01 |
| `duc index ~` | 20.776 ± 0.093 | 20.591 | 20.924 | 5.59 ± 0.06 |
| `ncdu -0 -o /dev/null ~` | 20.933 ± 0.113 | 20.757 | 21.073 | 5.64 ± 0.06 |
| `diskus ~` | 3.747 ± 0.027 | 3.707 | 3.779 | 1.01 ± 0.01 |
| `du -hs ~` | 20.096 ± 0.128 | 19.916 | 20.313 | 5.41 ± 0.06 |
| `dust -d0 ~` | 16.281 ± 0.118 | 16.148 | 16.490 | 4.38 ± 0.05 |


### Warm cache

| Command | Mean [ms] | Min [ms] | Max [ms] | Relative |
|:---|---:|---:|---:|---:|
| `gdu -npc ~` | 643.5 ± 11.3 | 623.8 | 659.9 | 1.99 ± 0.12 |
| `dua ~` | 389.7 ± 13.0 | 374.2 | 410.4 | 1.20 ± 0.08 |
| `duc index ~` | 1241.1 ± 19.6 | 1205.4 | 1274.9 | 3.84 ± 0.23 |
| `ncdu -0 -o /dev/null ~` | 1846.7 ± 11.9 | 1823.0 | 1859.9 | 5.71 ± 0.33 |
| `diskus ~` | 323.4 ± 18.8 | 302.1 | 362.2 | 1.00 |
| `du -hs ~` | 1027.7 ± 9.7 | 1009.0 | 1037.5 | 3.18 ± 0.19 |
| `dust -d0 ~` | 8864.4 ± 35.9 | 8798.8 | 8906.6 | 27.41 ± 1.60 |

## Alternatives

* [ncdu](https://dev.yorhel.nl/ncdu) - NCurses based tool written in pure C
* [godu](https://github.com/viktomas/godu) - Analyzer with carousel like user interface
* [dua](https://github.com/Byron/dua-cli) - Tool written in Rust with interface similar to gdu (and ncdu)
* [diskus](https://github.com/sharkdp/diskus) - Very simple but very fast tool written in Rust
* [duc](https://duc.zevv.nl/) - Collection of tools with many possibilities for inspecting and visualising disk usage
* [dust](https://github.com/bootandy/dust) - Tool written in Rust showing tree like structures of disk usage